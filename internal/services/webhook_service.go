package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
	"gorm.io/gorm"
)

// WebhookDispatcher is the contract other services use to trigger webhooks.
// WebhookService implements it; a nil dispatcher means webhooks are disabled.
type WebhookDispatcher interface {
	Dispatch(event string, data interface{})
}

// WebhookService manages webhooks and dispatches events.
type WebhookService struct {
	repo   repository.WebhookRepository
	client *http.Client
}

// NewWebhookService creates a new WebhookService backed by a GORM repository.
// Kept for backward compatibility with existing callers and tests.
func NewWebhookService(db *gorm.DB) *WebhookService {
	return &WebhookService{
		repo:   repository.NewWebhookRepository(db),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// NewWebhookServiceWithRepo builds a WebhookService with an explicit repository,
// enabling unit tests to inject mocks.
func NewWebhookServiceWithRepo(repo repository.WebhookRepository) *WebhookService {
	return &WebhookService{
		repo:   repo,
		client: &http.Client{Timeout: 10 * time.Second},
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

// Create creates a new webhook.
func (s *WebhookService) Create(req CreateWebhookRequest) (*models.Webhook, error) {
	wh := models.Webhook{
		Name:     req.Name,
		URL:      req.URL,
		Events:   req.Events,
		Headers:  req.Headers,
		Secret:   req.Secret,
		IsActive: true,
	}
	if err := s.repo.Create(&wh); err != nil {
		return nil, errors.New("failed to create webhook")
	}
	return &wh, nil
}

// List returns all webhooks.
func (s *WebhookService) List() ([]models.Webhook, error) {
	return s.repo.List()
}

// Get returns a webhook by ID.
func (s *WebhookService) Get(id uint) (*models.Webhook, error) {
	wh, err := s.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("webhook not found")
	}
	return wh, nil
}

// Delete deletes a webhook.
func (s *WebhookService) Delete(id uint) error {
	rowsAffected, err := s.repo.Delete(id)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("webhook not found")
	}
	return nil
}

// GetLogs returns delivery logs for a webhook.
func (s *WebhookService) GetLogs(webhookID uint, limit int) ([]models.WebhookLog, error) {
	return s.repo.ListLogs(webhookID, limit)
}

// ─── Dispatch ───────────────────────────────────────────────────────────────

// WebhookPayload is the JSON body sent to webhook endpoints.
type WebhookPayload struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// Dispatch sends an event to all matching webhooks (async).
func (s *WebhookService) Dispatch(event string, data interface{}) {
	webhooks, err := s.repo.ListActive()
	if err != nil {
		slog.Error("webhook list active failed", "event", event, "error", err)
		return
	}

	payload := WebhookPayload{
		Event:     event,
		Timestamp: time.Now(),
		Data:      data,
	}

	for _, wh := range webhooks {
		if !wh.Events.Has(event) {
			continue
		}
		go s.deliver(wh, payload)
	}
}

func (s *WebhookService) deliver(wh models.Webhook, payload WebhookPayload) {
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("webhook marshal failed", "webhook_id", wh.ID, "error", err)
		return
	}

	req, err := http.NewRequest("POST", wh.URL, bytes.NewReader(body))
	if err != nil {
		slog.Error("webhook request failed", "webhook_id", wh.ID, "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ContentX-Event", payload.Event)

	// HMAC signature if secret is set.
	if wh.Secret != "" {
		sig := hmacSign([]byte(wh.Secret), body)
		req.Header.Set("X-ContentX-Signature", "sha256="+sig)
	}

	start := time.Now()
	resp, err := s.client.Do(req)
	duration := int(time.Since(start).Milliseconds())

	log := models.WebhookLog{
		WebhookID: wh.ID,
		Event:     payload.Event,
		Payload:   string(body),
		Duration:  duration,
	}

	if err != nil {
		log.Success = false
		log.Error = err.Error()
		slog.Warn("webhook delivery failed", "webhook_id", wh.ID, "url", wh.URL, "error", err)
	} else {
		log.Response = resp.StatusCode
		log.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
		resp.Body.Close()
		if !log.Success {
			slog.Warn("webhook returned non-2xx", "webhook_id", wh.ID, "status", resp.StatusCode)
		}
	}

	_ = s.repo.CreateLog(&log)
}

func hmacSign(secret, data []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
