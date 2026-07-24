package services

import (
	"fmt"
	"time"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/errs"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Request DTOs
// ---------------------------------------------------------------------------

// LoginRequest is the payload for user login.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest is the payload for user registration.
type RegisterRequest struct {
	Username    string `json:"username" binding:"required,min=3,max=64"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name"`
}

// ChangePasswordRequest is the payload for changing a password.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// RefreshRequest is the payload for refreshing an access token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

// SafeUser is the sanitized user representation (no password or secrets).
type SafeUser struct {
	ID          uint                   `json:"id"`
	Username    string                 `json:"username"`
	Email       string                 `json:"email"`
	DisplayName string                 `json:"display_name"`
	Avatar      string                 `json:"avatar"`
	Bio         string                 `json:"bio"`
	Website     string                 `json:"website"`
	Role        models.Role            `json:"role"`
	Status      models.UserStatus      `json:"status"`
	LastLoginAt *time.Time             `json:"last_login_at"`
	LoginCount  int                    `json:"login_count"`
	Preferences models.UserPreferences `json:"preferences"`
	CreatedAt   time.Time              `json:"created_at"`
}

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// AuthService handles authentication business logic.
type AuthService struct {
	repo      repository.AuthRepository
	jwtMgr    *auth.JWTManager
	blacklist auth.TokenStore
	guard     *auth.LoginGuard
}

// NewAuthService creates a new AuthService backed by a GORM repository.
// blacklist 可为 *auth.Blacklist（内存版）或 *auth.RedisTokenStore（Redis 版）。
func NewAuthService(db *gorm.DB, jwtMgr *auth.JWTManager, blacklist auth.TokenStore, guard *auth.LoginGuard) *AuthService {
	return &AuthService{
		repo:      repository.NewAuthRepository(db),
		jwtMgr:    jwtMgr,
		blacklist: blacklist,
		guard:     guard,
	}
}

// NewAuthServiceWithRepo builds an AuthService with an explicit repository.
func NewAuthServiceWithRepo(repo repository.AuthRepository, jwtMgr *auth.JWTManager, blacklist auth.TokenStore, guard *auth.LoginGuard) *AuthService {
	return &AuthService{
		repo:      repo,
		jwtMgr:    jwtMgr,
		blacklist: blacklist,
		guard:     guard,
	}
}

// Login authenticates a user by username/email and password, records the login
// event, and returns a token pair together with the sanitized user profile.
func (s *AuthService) Login(username, password, clientIP, userAgent string) (*auth.TokenPair, *SafeUser, error) {
	// Check if account is locked.
	if s.guard != nil {
		locked, remaining := s.guard.Check(username)
		if locked {
			return nil, nil, errs.ErrAccountLocked
		}
		_ = remaining
	}

	user, err := s.repo.FindUserByUsernameOrEmail(username)
	if err != nil {
		// Record failed attempt even for non-existent users (prevent enumeration).
		if s.guard != nil {
			s.guard.RecordFailed(username)
		}
		return nil, nil, errs.ErrInvalidCreds
	}

	if !user.IsActive() {
		return nil, nil, errs.ErrAccountDisabled
	}

	if err := auth.CheckPassword(user.Password, password); err != nil {
		// Record failed attempt.
		if s.guard != nil {
			locked, _ := s.guard.RecordFailed(username)
			if locked {
				return nil, nil, errs.ErrAccountLocked.WithMessage(fmt.Sprintf("account locked after %d failed attempts", 5))
			}
		}
		return nil, nil, errs.ErrInvalidCreds
	}

	// Login successful — reset guard.
	if s.guard != nil {
		s.guard.RecordSuccess(username)
	}

	tokenPair, err := s.jwtMgr.GenerateTokenPair(
		user.ID, user.Username, user.Email, user.Role.Slug, user.DisplayName,
	)
	if err != nil {
		return nil, nil, errs.ErrInternal.Wrap(err)
	}

	// Record login metadata.
	user.RecordLogin(clientIP)
	_ = s.repo.UpdateUserFields(user.ID, map[string]interface{}{
		"last_login_at": user.LastLoginAt,
		"last_login_ip": user.LastLoginIP,
		"login_count":   user.LoginCount,
	})

	// Log activity (best-effort).
	_ = s.repo.CreateActivityLog(&models.ActivityLog{
		UserID:    &user.ID,
		Action:    "login",
		Entity:    "user",
		EntityID:  user.ID,
		IP:        clientIP,
		UserAgent: userAgent,
	})

	return tokenPair, SanitizeUser(user), nil
}

// Register creates a new user account, assigns the default role, generates
// tokens, and returns them together with the sanitized user profile.
func (s *AuthService) Register(req RegisterRequest, clientIP string) (*auth.TokenPair, *SafeUser, error) {
	// Check if registration is enabled.
	if setting, err := s.repo.FindSetting("enable_registration"); err == nil {
		if setting.Value == "false" {
			return nil, nil, errs.ErrForbidden.WithMessage("registration is currently disabled")
		}
	}

	// Check uniqueness.
	count, err := s.repo.CountUsersByUsernameOrEmail(req.Username, req.Email)
	if err != nil {
		return nil, nil, errs.ErrInternal.Wrap(err)
	}
	if count > 0 {
		return nil, nil, errs.ErrDuplicateUser
	}

	// Hash password.
	hashedPw, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, nil, err
	}

	// Find default role (fallback to subscriber slug).
	defaultRole, err := s.repo.FindDefaultRole()
	if err != nil {
		defaultRole, _ = s.repo.FindRoleBySlug("subscriber")
	}
	if defaultRole == nil {
		return nil, nil, errs.ErrInternal.WithMessage("no default role configured")
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Username
	}

	user := models.User{
		Username:    req.Username,
		Email:       req.Email,
		Password:    hashedPw,
		DisplayName: displayName,
		RoleID:      defaultRole.ID,
		Status:      models.UserStatusActive,
	}

	if err := s.repo.CreateUser(&user); err != nil {
		return nil, nil, errs.ErrInternal.Wrap(err)
	}

	// Reload with role to generate tokens.
	userWithRole, err := s.repo.FindUserByIDWithRole(user.ID)
	if err != nil {
		return nil, nil, errs.ErrInternal.Wrap(fmt.Errorf("user created but role reload failed: %w", err))
	}
	tokenPair, err := s.jwtMgr.GenerateTokenPair(
		userWithRole.ID, userWithRole.Username, userWithRole.Email, userWithRole.Role.Slug, userWithRole.DisplayName,
	)
	if err != nil {
		return nil, nil, errs.ErrInternal.Wrap(fmt.Errorf("user created but token generation failed: %w", err))
	}

	return tokenPair, SanitizeUser(userWithRole), nil
}

// RefreshToken validates a refresh token, loads the user's current state from
// the database, and issues a new token pair.
//
// Loading the user on every refresh ensures role changes, disablement, or
// deletion take effect immediately. Previously the refresh reused stale
// claims from the refresh token, which meant a user whose role was changed
// or who was disabled could keep obtaining access tokens with the old
// privileges until the refresh token expired (A-1 security fix).
func (s *AuthService) RefreshToken(refreshToken string) (*auth.TokenPair, error) {
	claims, err := s.jwtMgr.ValidateToken(refreshToken)
	if err != nil {
		return nil, errs.ErrUnauthorized.WithMessage("invalid refresh token")
	}

	user, err := s.repo.FindUserByIDWithRole(claims.UserID)
	if err != nil {
		return nil, errs.ErrUnauthorized.WithMessage("user not found")
	}

	if user.Status != models.UserStatusActive {
		return nil, errs.ErrUnauthorized.WithMessage("user is not active")
	}

	return s.jwtMgr.GenerateTokenPair(user.ID, user.Username, user.Email, user.Role.Slug, user.DisplayName)
}

// LogoutRequest is the optional payload for logout. refresh_token, when
// provided, is also blacklisted so it can no longer be used to mint new
// access tokens (A-3 fix).
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Logout invalidates the given access token (and optional refresh token) by
// adding them to the blacklist. The refresh token is blacklisted
// best-effort: an invalid/expired refresh token is silently ignored so a
// client with a stale token can still log out.
func (s *AuthService) Logout(accessToken, refreshToken string) error {
	claims, err := s.jwtMgr.ValidateToken(accessToken)
	if err != nil {
		return errs.ErrUnauthorized.WithMessage("invalid token")
	}

	s.blacklist.Revoke(accessToken, claims.ExpiresAt.Time)

	if refreshToken != "" {
		if rClaims, err := s.jwtMgr.ValidateToken(refreshToken); err == nil {
			s.blacklist.Revoke(refreshToken, rClaims.ExpiresAt.Time)
		}
	}
	return nil
}

// Me loads the full user profile (with role and permissions) and returns
// the sanitized user together with the list of permission slugs.
func (s *AuthService) Me(userID uint) (*SafeUser, []string, error) {
	user, err := s.repo.FindUserByIDWithPermissions(userID)
	if err != nil {
		return nil, nil, errs.ErrNotFound.WithMessage("user not found")
	}

	permissions := make([]string, len(user.Role.Permissions))
	for i, p := range user.Role.Permissions {
		permissions[i] = p.Slug
	}

	return SanitizeUser(user), permissions, nil
}

// UpdateProfile applies the supplied field updates to the user and returns
// the refreshed user model. Only display_name, bio, website, and avatar
// are accepted.
func (s *AuthService) UpdateProfile(userID uint, fields map[string]interface{}) (*models.User, error) {
	if _, err := s.repo.FindUserByID(userID); err != nil {
		return nil, errs.ErrNotFound.WithMessage("user not found")
	}

	allowed := map[string]bool{
		"display_name": true,
		"bio":          true,
		"website":      true,
		"avatar":       true,
	}

	updates := make(map[string]interface{})
	for k, v := range fields {
		if allowed[k] {
			updates[k] = v
		}
	}

	if len(updates) > 0 {
		_ = s.repo.UpdateUserFields(userID, updates)
	}

	// Reload with role.
	user, err := s.repo.FindUserByIDWithRole(userID)
	if err != nil {
		return nil, errs.ErrNotFound.WithMessage("user not found")
	}
	return user, nil
}

// ChangePassword verifies the old password, hashes the new one, and persists
// the change.
func (s *AuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return errs.ErrNotFound.WithMessage("user not found")
	}

	if err := auth.CheckPassword(user.Password, oldPassword); err != nil {
		return errs.ErrInvalidCreds.WithMessage("current password is incorrect")
	}

	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}

	return s.repo.UpdateUserPassword(userID, newHash)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// SanitizeUser strips sensitive fields and returns a SafeUser.
func SanitizeUser(u *models.User) *SafeUser {
	return &SafeUser{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Avatar:      u.AvatarURL(),
		Bio:         u.Bio,
		Website:     u.Website,
		Role:        u.Role,
		Status:      u.Status,
		LastLoginAt: u.LastLoginAt,
		LoginCount:  u.LoginCount,
		Preferences: u.Preferences,
		CreatedAt:   u.CreatedAt,
	}
}
