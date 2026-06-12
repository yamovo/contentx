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

	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// WebhookService manages webhooks and dispatches events.
type WebhookService struct {
	db     *gorm.DB
	client *http.Client
}

// NewWebhookService creates a new WebhookService.
func NewWebhookService(db *gorm.DB) *WebhookService {
	return &WebhookService{
		db:     db,
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
	if err := s.db.Create(&wh).Error; err != nil {
		return nil, errors.New("failed to create webhook")
	}
	return &wh, nil
}

// List returns all webhooks.
func (s *WebhookService) List() ([]models.Webhook, error) {
	var webhooks []models.Webhook
	if err := s.db.Order("created_at DESC").Find(&webhooks).Error; err != nil {
		return nil, err
	}
	return webhooks, nil
}

// Get returns a webhook by ID.
func (s *WebhookService) Get(id uint) (*models.Webhook, error) {
	var wh models.Webhook
	if err := s.db.First(&wh, id).Error; err != nil {
		return nil, errors.New("webhook not found")
	}
	return &wh, nil
}

// Delete deletes a webhook.
func (s *WebhookService) Delete(id uint) error {
	result := s.db.Delete(&models.Webhook{}, id)
	if result.RowsAffected == 0 {
		return errors.New("webhook not found")
	}
	s.db.Where("webhook_id = ?", id).Delete(&models.WebhookLog{})
	return result.Error
}

// GetLogs returns delivery logs for a webhook.
func (s *WebhookService) GetLogs(webhookID uint, limit int) ([]models.WebhookLog, error) {
	if limit <= 0 {
		limit = 50
	}
	var logs []models.WebhookLog
	s.db.Where("webhook_id = ?", webhookID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs)
	return logs, nil
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
	var webhooks []models.Webhook
	s.db.Where("is_active = ?", true).Find(&webhooks)

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

	s.db.Create(&log)
}

func hmacSign(secret, data []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
