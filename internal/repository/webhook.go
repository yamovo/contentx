package repository

import (
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// WebhookRepository defines data-access operations for webhooks and their delivery logs.
type WebhookRepository interface {
	Create(wh *models.Webhook) error
	List() ([]models.Webhook, error)
	GetByID(id uint) (*models.Webhook, error)
	Delete(id uint) (rowsAffected int64, err error)
	ListLogs(webhookID uint, limit int) ([]models.WebhookLog, error)
	CreateLog(log *models.WebhookLog) error
	ListActive() ([]models.Webhook, error)
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
	// Best-effort cleanup of delivery logs (mirrors prior service behaviour).
	r.db.Where("webhook_id = ?", id).Delete(&models.WebhookLog{})
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
