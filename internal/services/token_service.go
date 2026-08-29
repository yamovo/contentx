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

// TokenPrincipal is the fully revalidated identity carried by a long-lived API
// token. Permissions are the effective intersection of the stored token grants,
// the creator's current global role, and the current tenant membership role.
type TokenPrincipal struct {
	TokenID         uint
	UserID          uint
	TenantID        uint
	Permissions     []string
	IsPlatformAdmin bool
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

// List returns API tokens for one tenant (without the secret).
func (s *TokenService) List(tenantID uint) ([]models.APIToken, error) {
	if tenantID == 0 {
		return nil, errs.ErrBadRequest.WithMessage("tenant is required")
	}
	tokens, err := s.repo.List(tenantID)
	if err != nil {
		return nil, err
	}
	for i := range tokens {
		canonical, _ := permissions.CanonicalizeList([]string(tokens[i].Permissions))
		tokens[i].Permissions = models.StringSlice(canonical)
	}
	return tokens, nil
}

// Create generates a new API token bound to the given tenant.
func (s *TokenService) Create(req CreateTokenRequest, createdBy, tenantID uint) (*TokenCreatedResponse, error) {
	if tenantID == 0 {
		return nil, errs.ErrBadRequest.WithMessage("tenant is required")
	}
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
		TenantID:    &tenantID,
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

// Delete removes an API token by ID within one tenant.
func (s *TokenService) Delete(id, tenantID uint) error {
	if tenantID == 0 {
		return errs.ErrBadRequest.WithMessage("tenant is required")
	}
	rowsAffected, err := s.repo.Delete(id, tenantID)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errs.ErrNotFound.WithMessage("token not found")
	}
	return nil
}

// Resolve returns a fully revalidated principal for an active, non-expired API
// token. Every call reloads the creator, global role permissions, tenant, and
// tenant membership so disabling any of them immediately revokes access.
func (s *TokenService) Resolve(tokenStr string) (*TokenPrincipal, error) {
	record, err := s.repo.FindPrincipalByToken(tokenStr)
	if err != nil {
		return nil, errors.New("invalid token")
	}
	if record == nil || record.Token.TenantID == nil || *record.Token.TenantID == 0 {
		return nil, errors.New("invalid token principal")
	}

	// Check expiry.
	if record.Token.ExpiresAt != nil && record.Token.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("token expired")
	}
	if !record.User.IsActive() || record.Tenant.Status != models.TenantStatusActive || record.Membership == nil {
		return nil, errors.New("invalid token principal")
	}

	effective, valid := permissions.EffectiveForTenant(
		&record.User,
		[]string(record.Token.Permissions),
		record.Membership.RoleSlug,
	)
	if !valid {
		return nil, errors.New("invalid token permissions")
	}

	// Update last used (best-effort; ignore error).
	_ = s.repo.UpdateUsage(record.Token.ID, time.Now())
	return &TokenPrincipal{
		TokenID:         record.Token.ID,
		UserID:          record.User.ID,
		TenantID:        *record.Token.TenantID,
		Permissions:     effective,
		IsPlatformAdmin: record.User.Role.Slug == "admin",
	}, nil
}

// Validate checks if a token string is valid and has the required permission.
func (s *TokenService) Validate(tokenStr string, requiredPerm string) (bool, uint, error) {
	principal, err := s.Resolve(tokenStr)
	if err != nil {
		return false, 0, err
	}

	// An empty requirement checks authentication only; an empty token grant set
	// still carries no action permissions.
	if requiredPerm == "" {
		return true, principal.UserID, nil
	}
	if permissions.Grants(principal.Permissions, requiredPerm) {
		return true, principal.UserID, nil
	}

	return false, principal.UserID, errors.New("insufficient token permissions")
}
