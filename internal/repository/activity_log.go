package repository

import (
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// ActivityLogRepository consolidates write access to models.ActivityLog.
// Previously the only write path lived on AuthRepository.CreateActivityLog,
// which made audit writes scatter across unrelated repositories. This
// interface is the single entry point for service-layer audit logging.
type ActivityLogRepository interface {
	Create(log *models.ActivityLog) error
}

type gormActivityLogRepository struct {
	db *gorm.DB
}

// NewActivityLogRepository returns a GORM-backed ActivityLogRepository.
func NewActivityLogRepository(db *gorm.DB) ActivityLogRepository {
	return &gormActivityLogRepository{db: db}
}

func (r *gormActivityLogRepository) Create(log *models.ActivityLog) error {
	return r.db.Create(log).Error
}
