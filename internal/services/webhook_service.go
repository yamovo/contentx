package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"time"

	"github.com/yamovo/contentx/internal/errs"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
	"gorm.io/gorm"
)

// WebhookDispatcher is the contract other services use to trigger webhooks.
// WebhookService implements it; a nil dispatcher means webhooks are disabled.
// The tenantID scopes which webhooks receive the event (RFC-001 §6).
type WebhookDispatcher interface {
	Dispatch(event string, data interface{}, tenantID uint)
}

// WebhookService manages webhooks and enqueues delivery jobs. Actual HTTP
// delivery is performed asynchronously by WebhookWorker draining the
// persistent webhook_deliveries queue.
type WebhookService struct {
	repo  repository.WebhookRepository
	audit AuditLogger
}

// NewWebhookService creates a new WebhookService backed by a GORM repository.
// Kept for backward compatibility with existing callers and tests.
func NewWebhookService(db *gorm.DB) *WebhookService {
	return &WebhookService{
		repo:  repository.NewWebhookRepository(db),
		audit: NoopAuditLogger{},
	}
}

// NewWebhookServiceWithRepo builds a WebhookService with an explicit repository,
// enabling unit tests to inject mocks.
func NewWebhookServiceWithRepo(repo repository.WebhookRepository) *WebhookService {
	return &WebhookService{
		repo:  repo,
		audit: NoopAuditLogger{},
	}
}

// SetAuditLogger wires the business-level audit logger.
func (s *WebhookService) SetAuditLogger(l AuditLogger) {
	if l != nil {
		s.audit = l
	}
}

// ─── CRUD ───────────────────────────────────────────────────────────────────

// CreateWebhookRequest is the payload for creating a webhook.
type CreateWebhookRequest struct {
	Name    string   `json:"name" binding:"required,max=128"`
	URL     string   `json:"url" binding:"required,url"`
	Events  []string `json:"events" binding:"required,min=1"`
	Headers []string `json:"headers"`
	Secret  string   `json:"secret"`
}

// Create creates a new webhook scoped to the given tenant.
func (s *WebhookService) Create(req CreateWebhookRequest, tenantID uint, actorIDs ...uint) (*models.Webhook, error) {
	// SEC-1: reject unsafe target URLs early (scheme + literal internal IPs).
	if err := validateWebhookURL(req.URL); err != nil {
		return nil, errs.ErrBadRequest.WithMessage(err.Error())
	}
	wh := models.Webhook{
		Name:     req.Name,
		URL:      req.URL,
		Events:   req.Events,
		Headers:  req.Headers,
		Secret:   req.Secret,
		IsActive: true,
		TenantID: tenantID,
	}
	if err := s.repo.Create(&wh); err != nil {
		return nil, errors.New("failed to create webhook")
	}
	tid := tenantID
	s.audit.Log(AuditEvent{
		UserID: auditActor(actorIDs), TenantID: &tid, Action: "webhook.create", Entity: "webhook", EntityID: wh.ID,
		Details: map[string]any{"name": wh.Name, "url": auditWebhookURL(wh.URL), "events": wh.Events},
	})
	return &wh, nil
}

// List returns all webhooks for a tenant.
func (s *WebhookService) List(tenantID uint) ([]models.Webhook, error) {
	return s.repo.List(tenantID)
}

// Get returns a webhook by ID within the tenant.
func (s *WebhookService) Get(id, tenantID uint) (*models.Webhook, error) {
	wh, err := s.repo.GetByID(id, tenantID)
	if err != nil {
		return nil, errors.New("webhook not found")
	}
	return wh, nil
}

// Delete deletes a webhook within the tenant.
func (s *WebhookService) Delete(id, tenantID uint, actorIDs ...uint) error {
	wh, _ := s.repo.GetByID(id, tenantID) // best-effort for audit details
	rowsAffected, err := s.repo.Delete(id, tenantID)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("webhook not found")
	}
	details := map[string]any{}
	if wh != nil {
		details["name"] = wh.Name
		details["url"] = auditWebhookURL(wh.URL)
	}
	tid := tenantID
	s.audit.Log(AuditEvent{
		UserID: auditActor(actorIDs), TenantID: &tid, Action: "webhook.delete", Entity: "webhook", EntityID: id, Details: details,
	})
	return nil
}

func auditWebhookURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "[invalid URL]"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	return parsed.String()
}

// GetLogs returns delivery logs for a webhook within the tenant.
func (s *WebhookService) GetLogs(webhookID uint, limit int, tenantID uint) ([]models.WebhookLog, error) {
	return s.repo.ListLogs(webhookID, limit, tenantID)
}

// ─── Dispatch ───────────────────────────────────────────────────────────────

// WebhookPayload is the JSON body sent to webhook endpoints.
type WebhookPayload struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// Dispatch enqueues an event for all matching webhooks in the given tenant.
// Deliveries are persisted as pending webhook_deliveries rows and drained
// asynchronously by WebhookWorker, so dispatch is fast, bounded, and survives
// restarts. Only webhooks belonging to tenantID are considered, preventing
// cross-tenant event leakage (RFC-001 §6).
func (s *WebhookService) Dispatch(event string, data interface{}, tenantID uint) {
	webhooks, err := s.repo.ListActive(tenantID)
	if err != nil {
		slog.Error("webhook list active failed", "event", event, "tenant_id", tenantID, "error", err)
		return
	}

	body, err := json.Marshal(WebhookPayload{
		Event:     event,
		Timestamp: time.Now(),
		Data:      data,
	})
	if err != nil {
		slog.Error("webhook marshal failed", "event", event, "error", err)
		return
	}

	now := time.Now()
	for _, wh := range webhooks {
		if !wh.Events.Has(event) {
			continue
		}
		d := models.WebhookDelivery{
			WebhookID:   wh.ID,
			TenantID:    tenantID,
			Event:       event,
			Payload:     string(body),
			Status:      models.WebhookDeliveryPending,
			NextRetryAt: now, // immediately due for the next worker tick
		}
		if err := s.repo.EnqueueDelivery(&d); err != nil {
			// Best-effort: a failed enqueue must not break the business
			// operation that triggered the event (same contract as before).
			slog.Error("webhook enqueue failed", "webhook_id", wh.ID, "event", event, "error", err)
		}
	}
}

func hmacSign(secret, data []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
