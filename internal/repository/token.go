package repository

import (
	"errors"
	"time"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// TokenRepository defines data-access operations for API tokens.
type TokenRepository interface {
	List(tenantID uint) ([]models.APIToken, error)
	Create(token *models.APIToken) error
	Delete(id, tenantID uint) (rowsAffected int64, err error)
	FindPrincipalByToken(tokenStr string) (*TokenPrincipalRecord, error)
	UpdateUsage(tokenID uint, lastUsed time.Time) error
}

// TokenPrincipalRecord is the database state that must be revalidated every
// time a long-lived API token is resolved. Membership is nil when the token
// creator no longer belongs to the token's tenant.
type TokenPrincipalRecord struct {
	Token      models.APIToken
	User       models.User
	Tenant     models.Tenant
	Membership *models.TenantMembership
}

// gormTokenRepository implements TokenRepository with GORM.
type gormTokenRepository struct {
	db *gorm.DB
}

// NewTokenRepository builds a GORM-backed TokenRepository.
func NewTokenRepository(db *gorm.DB) TokenRepository {
	return &gormTokenRepository{db: db}
}

func (r *gormTokenRepository) List(tenantID uint) ([]models.APIToken, error) {
	var tokens []models.APIToken
	if err := r.db.Where("tenant_id = ?", tenantID).
		Order("created_at DESC").Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

func (r *gormTokenRepository) Create(token *models.APIToken) error {
	return r.db.Create(token).Error
}

func (r *gormTokenRepository) Delete(id, tenantID uint) (int64, error) {
	result := r.db.Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&models.APIToken{})
	return result.RowsAffected, result.Error
}

func (r *gormTokenRepository) FindPrincipalByToken(tokenStr string) (*TokenPrincipalRecord, error) {
	record := &TokenPrincipalRecord{}
	if err := r.db.Where("token = ? AND is_active = ?", tokenStr, true).
		First(&record.Token).Error; err != nil {
		return nil, err
	}

	// A usable token must be explicitly bound to a concrete tenant. Legacy
	// NULL-tenant tokens fail closed and can be replaced by a tenant-bound token.
	if record.Token.TenantID == nil || *record.Token.TenantID == 0 {
		return record, nil
	}

	if err := r.db.Preload("Role").Preload("Role.Permissions").
		First(&record.User, record.Token.CreatedByID).Error; err != nil {
		return nil, err
	}
	if err := r.db.First(&record.Tenant, *record.Token.TenantID).Error; err != nil {
		return nil, err
	}

	var membership models.TenantMembership
	err := r.db.Where("tenant_id = ? AND user_id = ?", *record.Token.TenantID, record.Token.CreatedByID).
		First(&membership).Error
	if err == nil {
		record.Membership = &membership
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return record, nil
}

func (r *gormTokenRepository) UpdateUsage(tokenID uint, lastUsed time.Time) error {
	return r.db.Model(&models.APIToken{}).Where("id = ?", tokenID).
		Updates(map[string]interface{}{
			"last_used_at": lastUsed,
			"use_count":    gorm.Expr("use_count + 1"),
		}).Error
}
