package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// CreateTokenRequest is the payload for creating an API token.
type CreateTokenRequest struct {
	Name        string   `json:"name" binding:"required,max=128"`
	Permissions []string `json:"permissions"`
	ExpiresAt   string   `json:"expires_at"` // RFC3339 or empty for no expiry
}

// TokenCreatedResponse is returned once after token creation.
type TokenCreatedResponse struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Token       string    `json:"token"` // only shown once
	Permissions []string  `json:"permissions"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// TokenService manages API tokens.
type TokenService struct {
	db *gorm.DB
}

// NewTokenService creates a new TokenService.
func NewTokenService(db *gorm.DB) *TokenService {
	return &TokenService{db: db}
}

// List returns all API tokens (without the secret).
func (s *TokenService) List() ([]models.APIToken, error) {
	var tokens []models.APIToken
	if err := s.db.Order("created_at DESC").Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

// Create generates a new API token.
func (s *TokenService) Create(req CreateTokenRequest, createdBy uint) (*TokenCreatedResponse, error) {
	// Generate random token (vc_live_ + 32 hex chars).
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, errors.New("failed to generate token")
	}
	tokenStr := "vc_live_" + hex.EncodeToString(raw)

	// Parse expiry.
	var expiresAt *time.Time
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			return nil, errors.New("invalid expires_at format, use RFC3339")
		}
		expiresAt = &t
	}

	token := models.APIToken{
		Name:        req.Name,
		Token:       tokenStr,
		Permissions: req.Permissions,
		ExpiresAt:   expiresAt,
		CreatedByID: createdBy,
		IsActive:    true,
	}

	if err := s.db.Create(&token).Error; err != nil {
		return nil, errors.New("failed to create token")
	}

	return &TokenCreatedResponse{
		ID:          token.ID,
		Name:        token.Name,
		Token:       tokenStr,
		Permissions: token.Permissions,
		ExpiresAt:   token.ExpiresAt,
		CreatedAt:   token.CreatedAt,
	}, nil
}

// Delete removes an API token by ID.
func (s *TokenService) Delete(id uint) error {
	result := s.db.Delete(&models.APIToken{}, id)
	if result.RowsAffected == 0 {
		return errors.New("token not found")
	}
	return result.Error
}

// Validate checks if a token string is valid and has the required permission.
func (s *TokenService) Validate(tokenStr string, requiredPerm string) (bool, uint, error) {
	var token models.APIToken
	if err := s.db.Where("token = ? AND is_active = ?", tokenStr, true).First(&token).Error; err != nil {
		return false, 0, errors.New("invalid token")
	}

	// Check expiry.
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		return false, 0, errors.New("token expired")
	}

	// Update last used.
	s.db.Model(&token).Updates(map[string]interface{}{
		"last_used_at": time.Now(),
		"use_count":    gorm.Expr("use_count + 1"),
	})

	// Check permission (empty = full access).
	if requiredPerm == "" {
		return true, token.CreatedByID, nil
	}
	for _, p := range token.Permissions {
		if p == "*" || p == requiredPerm {
			return true, token.CreatedByID, nil
		}
	}

	return false, token.CreatedByID, errors.New("insufficient token permissions")
}
