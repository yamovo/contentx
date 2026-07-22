package repository

import (
	"time"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// TokenRepository defines data-access operations for API tokens.
type TokenRepository interface {
	List() ([]models.APIToken, error)
	Create(token *models.APIToken) error
	Delete(id uint) (rowsAffected int64, err error)
	FindActiveByToken(tokenStr string) (*models.APIToken, error)
	UpdateUsage(tokenID uint, lastUsed time.Time) error
}

// gormTokenRepository implements TokenRepository with GORM.
type gormTokenRepository struct {
	db *gorm.DB
}

// NewTokenRepository builds a GORM-backed TokenRepository.
func NewTokenRepository(db *gorm.DB) TokenRepository {
	return &gormTokenRepository{db: db}
}

func (r *gormTokenRepository) List() ([]models.APIToken, error) {
	var tokens []models.APIToken
	if err := r.db.Order("created_at DESC").Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

func (r *gormTokenRepository) Create(token *models.APIToken) error {
	return r.db.Create(token).Error
}

func (r *gormTokenRepository) Delete(id uint) (int64, error) {
	result := r.db.Delete(&models.APIToken{}, id)
	return result.RowsAffected, result.Error
}

func (r *gormTokenRepository) FindActiveByToken(tokenStr string) (*models.APIToken, error) {
	var token models.APIToken
	if err := r.db.Where("token = ? AND is_active = ?", tokenStr, true).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *gormTokenRepository) UpdateUsage(tokenID uint, lastUsed time.Time) error {
	return r.db.Model(&models.APIToken{}).Where("id = ?", tokenID).
		Updates(map[string]interface{}{
			"last_used_at": lastUsed,
			"use_count":    gorm.Expr("use_count + 1"),
		}).Error
}
