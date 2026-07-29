package repository

import (
	"time"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// DeliveryOutcome carries the result of one delivery attempt back to the
// store. A terminal outcome (success/failed/exhausted) sets CompletedAt; a
// retry outcome keeps Status pending and schedules NextRetryAt.
type DeliveryOutcome struct {
	Status       string
	Attempts     int
	NextRetryAt  time.Time
	ResponseCode int
	LastError    string
	CompletedAt  *time.Time
}

// WebhookRepository defines data-access operations for webhooks, their
// delivery logs and the persistent delivery queue.
type WebhookRepository interface {
	Create(wh *models.Webhook) error
	List() ([]models.Webhook, error)
	GetByID(id uint) (*models.Webhook, error)
	Delete(id uint) (rowsAffected int64, err error)
	ListLogs(webhookID uint, limit int) ([]models.WebhookLog, error)
	CreateLog(log *models.WebhookLog) error
	ListActive() ([]models.Webhook, error)

	// ─── Persistent delivery queue ──────────────────────────────────────
	EnqueueDelivery(d *models.WebhookDelivery) error
	ClaimDueDeliveries(now time.Time, limit int) ([]models.WebhookDelivery, error)
	CompleteDelivery(id uint, outcome DeliveryOutcome) error
	RequeueStaleDeliveries() (int64, error)
	CountPendingDeliveries() (int64, error)
}

// gormWebhookRepository implements WebhookRepository with GORM.
type gormWebhookRepository struct {
	db *gorm.DB
}

// NewWebhookRepository builds a GORM-backed WebhookRepository.
func NewWebhookRepository(db *gorm.DB) WebhookRepository {
	return &gormWebhookRepository{db: db}
}

func (r *gormWebhookRepository) Create(wh *models.Webhook) error {
	return r.db.Create(wh).Error
}

func (r *gormWebhookRepository) List() ([]models.Webhook, error) {
	var webhooks []models.Webhook
	if err := r.db.Order("created_at DESC").Find(&webhooks).Error; err != nil {
		return nil, err
	}
	return webhooks, nil
}

func (r *gormWebhookRepository) GetByID(id uint) (*models.Webhook, error) {
	var wh models.Webhook
	if err := r.db.First(&wh, id).Error; err != nil {
		return nil, err
	}
	return &wh, nil
}

func (r *gormWebhookRepository) Delete(id uint) (int64, error) {
	result := r.db.Delete(&models.Webhook{}, id)
	if result.Error != nil {
		return 0, result.Error
	}
	// Best-effort cleanup of delivery logs and queued deliveries (mirrors
	// prior service behaviour).
	r.db.Where("webhook_id = ?", id).Delete(&models.WebhookLog{})
	r.db.Where("webhook_id = ?", id).Delete(&models.WebhookDelivery{})
	return result.RowsAffected, nil
}

func (r *gormWebhookRepository) ListLogs(webhookID uint, limit int) ([]models.WebhookLog, error) {
	if limit <= 0 {
		limit = 50
	}
	var logs []models.WebhookLog
	if err := r.db.Where("webhook_id = ?", webhookID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

func (r *gormWebhookRepository) CreateLog(log *models.WebhookLog) error {
	return r.db.Create(log).Error
}

func (r *gormWebhookRepository) ListActive() ([]models.Webhook, error) {
	var webhooks []models.Webhook
	if err := r.db.Where("is_active = ?", true).Find(&webhooks).Error; err != nil {
		return nil, err
	}
	return webhooks, nil
}

// ─── Persistent delivery queue ──────────────────────────────────────────────

// EnqueueDelivery inserts a new pending delivery row.
func (r *gormWebhookRepository) EnqueueDelivery(d *models.WebhookDelivery) error {
	return r.db.Create(d).Error
}

// ClaimDueDeliveries atomically moves up to limit due pending rows into
// delivering and returns them. The conditional UPDATE ... WHERE status =
// 'pending' guard makes claiming race-safe across workers and instances on
// all supported databases (portable alternative to FOR UPDATE SKIP LOCKED).
func (r *gormWebhookRepository) ClaimDueDeliveries(now time.Time, limit int) ([]models.WebhookDelivery, error) {
	if limit <= 0 {
		return nil, nil
	}
	var claimed []models.WebhookDelivery
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var due []models.WebhookDelivery
		if err := tx.Where("status = ? AND next_retry_at <= ?", models.WebhookDeliveryPending, now).
			Order("next_retry_at").
			Limit(limit).
			Find(&due).Error; err != nil {
			return err
		}
		for _, d := range due {
			res := tx.Model(&models.WebhookDelivery{}).
				Where("id = ? AND status = ?", d.ID, models.WebhookDeliveryPending).
				Updates(map[string]interface{}{
					"status":     models.WebhookDeliveryDelivering,
					"updated_at": now,
				})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				continue // lost the race to another worker; skip
			}
			d.Status = models.WebhookDeliveryDelivering
			claimed = append(claimed, d)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

// CompleteDelivery writes the outcome of an attempt back to the row. Only
// rows still in delivering state are updated; a row reclaimed meanwhile (or
// deleted) is left untouched.
func (r *gormWebhookRepository) CompleteDelivery(id uint, outcome DeliveryOutcome) error {
	updates := map[string]interface{}{
		"status":        outcome.Status,
		"attempts":      outcome.Attempts,
		"next_retry_at": outcome.NextRetryAt,
		"response_code": outcome.ResponseCode,
		"last_error":    outcome.LastError,
		"completed_at":  outcome.CompletedAt,
	}
	return r.db.Model(&models.WebhookDelivery{}).
		Where("id = ? AND status = ?", id, models.WebhookDeliveryDelivering).
		Updates(updates).Error
}

// RequeueStaleDeliveries resets rows stuck in delivering (process crashed or
// was killed mid-attempt) back to pending so they are retried. Their original
// NextRetryAt is already in the past, making them immediately due.
func (r *gormWebhookRepository) RequeueStaleDeliveries() (int64, error) {
	res := r.db.Model(&models.WebhookDelivery{}).
		Where("status = ?", models.WebhookDeliveryDelivering).
		Update("status", models.WebhookDeliveryPending)
	return res.RowsAffected, res.Error
}

// CountPendingDeliveries reports the current queue depth (pending rows).
func (r *gormWebhookRepository) CountPendingDeliveries() (int64, error) {
	var count int64
	err := r.db.Model(&models.WebhookDelivery{}).
		Where("status = ?", models.WebhookDeliveryPending).
		Count(&count).Error
	return count, err
}
