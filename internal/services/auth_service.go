package services

import (
	"errors"
	"time"

	"github.com/vortexcms/go-cms/internal/auth"
	"github.com/vortexcms/go-cms/internal/models"
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
	db        *gorm.DB
	jwtMgr    *auth.JWTManager
	blacklist *auth.Blacklist
}

// NewAuthService creates a new AuthService.
func NewAuthService(db *gorm.DB, jwtMgr *auth.JWTManager, blacklist *auth.Blacklist) *AuthService {
	return &AuthService{db: db, jwtMgr: jwtMgr, blacklist: blacklist}
}

// Login authenticates a user by username/email and password, records the login
// event, and returns a token pair together with the sanitized user profile.
func (s *AuthService) Login(username, password, clientIP, userAgent string) (*auth.TokenPair, *SafeUser, error) {
	var user models.User
	if err := s.db.Preload("Role").
		Where("username = ? OR email = ?", username, username).
		First(&user).Error; err != nil {
		return nil, nil, errors.New("invalid credentials")
	}

	if !user.IsActive() {
		return nil, nil, errors.New("account is disabled")
	}

	if err := auth.CheckPassword(user.Password, password); err != nil {
		return nil, nil, errors.New("invalid credentials")
	}

	tokenPair, err := s.jwtMgr.GenerateTokenPair(
		user.ID, user.Username, user.Email, user.Role.Slug, user.DisplayName,
	)
	if err != nil {
		return nil, nil, errors.New("failed to generate token")
	}

	// Record login metadata.
	user.RecordLogin(clientIP)
	s.db.Model(&user).Updates(map[string]interface{}{
		"last_login_at": user.LastLoginAt,
		"last_login_ip": user.LastLoginIP,
		"login_count":   user.LoginCount,
	})

	// Log activity.
	s.db.Create(&models.ActivityLog{
		UserID:    &user.ID,
		Action:    "login",
		Entity:    "user",
		EntityID:  user.ID,
		IP:        clientIP,
		UserAgent: userAgent,
	})

	return tokenPair, SanitizeUser(&user), nil
}

// Register creates a new user account, assigns the default role, generates
// tokens, and returns them together with the sanitized user profile.
func (s *AuthService) Register(req RegisterRequest, clientIP string) (*auth.TokenPair, *SafeUser, error) {
	// Check if registration is enabled.
	var setting models.SiteSetting
	if s.db.Where("key = ?", "enable_registration").First(&setting).Error == nil {
		if setting.Value == "false" {
			return nil, nil, errors.New("registration is currently disabled")
		}
	}

	// Check uniqueness.
	var count int64
	s.db.Model(&models.User{}).
		Where("username = ? OR email = ?", req.Username, req.Email).
		Count(&count)
	if count > 0 {
		return nil, nil, errors.New("username or email already exists")
	}

	// Hash password.
	hashedPw, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, nil, err
	}

	// Find default role.
	var defaultRole models.Role
	if err := s.db.Where("is_default = ?", true).First(&defaultRole).Error; err != nil {
		s.db.Where("slug = ?", "subscriber").First(&defaultRole)
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

	if err := s.db.Create(&user).Error; err != nil {
		return nil, nil, errors.New("failed to create user")
	}

	// Reload with role to generate tokens.
	s.db.Preload("Role").First(&user, user.ID)
	tokenPair, err := s.jwtMgr.GenerateTokenPair(
		user.ID, user.Username, user.Email, user.Role.Slug, user.DisplayName,
	)
	if err != nil {
		return nil, nil, errors.New("user created but token generation failed")
	}

	return tokenPair, SanitizeUser(&user), nil
}

// RefreshToken validates a refresh token and issues a new token pair.
func (s *AuthService) RefreshToken(refreshToken string) (*auth.TokenPair, error) {
	tokenPair, err := s.jwtMgr.RefreshAccessToken(refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}
	return tokenPair, nil
}

// Logout invalidates the given access token by adding it to the blacklist.
func (s *AuthService) Logout(tokenString string, userID uint) error {
	claims, err := s.jwtMgr.ValidateToken(tokenString)
	if err != nil {
		return errors.New("invalid token")
	}

	s.blacklist.Revoke(tokenString, claims.ExpiresAt.Time)
	return nil
}

// Me loads the full user profile (with role and permissions) and returns
// the sanitized user together with the list of permission slugs.
func (s *AuthService) Me(userID uint) (*SafeUser, []string, error) {
	var user models.User
	if err := s.db.Preload("Role").Preload("Role.Permissions").
		First(&user, userID).Error; err != nil {
		return nil, nil, errors.New("user not found")
	}

	permissions := make([]string, len(user.Role.Permissions))
	for i, p := range user.Role.Permissions {
		permissions[i] = p.Slug
	}

	return SanitizeUser(&user), permissions, nil
}

// UpdateProfile applies the supplied field updates to the user and returns
// the refreshed user model. Only display_name, bio, website, and avatar
// are accepted.
func (s *AuthService) UpdateProfile(userID uint, fields map[string]interface{}) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, errors.New("user not found")
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
		s.db.Model(&user).Updates(updates)
	}

	s.db.Preload("Role").First(&user, user.ID)
	return &user, nil
}

// ChangePassword verifies the old password, hashes the new one, and persists
// the change.
func (s *AuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return errors.New("user not found")
	}

	if err := auth.CheckPassword(user.Password, oldPassword); err != nil {
		return errors.New("current password is incorrect")
	}

	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}

	s.db.Model(&user).Update("password", newHash)
	return nil
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
