package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/yamovo/contentx/internal/errs"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/permissions"
	"github.com/yamovo/contentx/internal/repository"
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
	ID          uint       `json:"id"`
	Name        string     `json:"name"`
	Token       string     `json:"token"` // only shown once
	Permissions []string   `json:"permissions"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// TokenService manages API tokens.
type TokenService struct {
	repo repository.TokenRepository
}

// NewTokenService creates a TokenService backed by a GORM repository.
// This constructor is kept for backward compatibility with existing callers
// and tests; new code should prefer NewTokenServiceWithRepo.
func NewTokenService(db *gorm.DB) *TokenService {
	return &TokenService{repo: repository.NewTokenRepository(db)}
}

// NewTokenServiceWithRepo builds a TokenService with an explicit repository,
// enabling unit tests to inject mocks.
func NewTokenServiceWithRepo(repo repository.TokenRepository) *TokenService {
	return &TokenService{repo: repo}
}

// List returns all API tokens (without the secret).
func (s *TokenService) List() ([]models.APIToken, error) {
	tokens, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	for i := range tokens {
		canonical, _ := permissions.CanonicalizeList([]string(tokens[i].Permissions))
		tokens[i].Permissions = models.StringSlice(canonical)
	}
	return tokens, nil
}

// Create generates a new API token.
func (s *TokenService) Create(req CreateTokenRequest, createdBy uint) (*TokenCreatedResponse, error) {
	canonicalPermissions, valid := permissions.CanonicalizeList(req.Permissions)
	if !valid {
		return nil, errs.ErrBadRequest.WithMessage("permissions contain an unknown slug")
	}

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
		Permissions: models.StringSlice(canonicalPermissions),
		ExpiresAt:   expiresAt,
		CreatedByID: createdBy,
		IsActive:    true,
	}

	if err := s.repo.Create(&token); err != nil {
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
	rowsAffected, err := s.repo.Delete(id)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errs.ErrNotFound.WithMessage("token not found")
	}
	return nil
}

// Resolve returns the active, non-expired API token for the given raw token
// string, updating its last-used timestamp. Callers that need the token's full
// permission set and owner (e.g. the MCP HTTP write tools) use this directly.
func (s *TokenService) Resolve(tokenStr string) (*models.APIToken, error) {
	token, err := s.repo.FindActiveByToken(tokenStr)
	if err != nil {
		return nil, errors.New("invalid token")
	}

	// Check expiry.
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("token expired")
	}

	// Update last used (best-effort; ignore error).
	_ = s.repo.UpdateUsage(token.ID, time.Now())
	canonical, _ := permissions.CanonicalizeList([]string(token.Permissions))
	token.Permissions = models.StringSlice(canonical)
	return token, nil
}

// Validate checks if a token string is valid and has the required permission.
func (s *TokenService) Validate(tokenStr string, requiredPerm string) (bool, uint, error) {
	token, err := s.Resolve(tokenStr)
	if err != nil {
		return false, 0, err
	}

	// Check permission (empty = full access).
	if requiredPerm == "" {
		return true, token.CreatedByID, nil
	}
	if permissions.Grants([]string(token.Permissions), requiredPerm) {
		return true, token.CreatedByID, nil
	}

	return false, token.CreatedByID, errors.New("insufficient token permissions")
}
