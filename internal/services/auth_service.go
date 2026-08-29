package services

import (
	"fmt"
	"sort"
	"time"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/errs"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/permissions"
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
	// TOTPCode is required when the account has two-factor authentication
	// enabled; a one-time backup code is also accepted.
	TOTPCode string `json:"totp_code"`
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
	repo              repository.AuthRepository
	jwtMgr            *auth.JWTManager
	blacklist         auth.TokenStore
	guard             auth.LoginLimiter
	totp              TOTPVerifier // optional second-factor hook
	allowRegistration bool
	audit             AuditLogger // business-level audit; defaults to NoopAuditLogger
}

// TOTPVerifier is the login-time second-factor hook. Implemented by
// *TOTPService; kept as an interface so AuthService unit tests don't need a
// database and existing constructors stay unchanged.
type TOTPVerifier interface {
	// Required reports whether the user must supply a TOTP code.
	Required(userID uint) (bool, error)
	// VerifyLogin validates a TOTP or backup code for the user.
	VerifyLogin(userID uint, code string) error
}

// SetTOTPVerifier wires the optional TOTP second-factor check into login.
func (s *AuthService) SetTOTPVerifier(v TOTPVerifier) {
	s.totp = v
}

// NewAuthService creates a new AuthService backed by a GORM repository.
// blacklist 可为 *auth.Blacklist（内存版）或 *auth.RedisTokenStore（Redis 版）。
// guard 可为 *auth.LoginGuard（内存版）或 *auth.RedisLoginGuard（多实例共享）。
func NewAuthService(db *gorm.DB, jwtMgr *auth.JWTManager, blacklist auth.TokenStore, guard auth.LoginLimiter, authCfg ...config.AuthConfig) *AuthService {
	return &AuthService{
		repo:              repository.NewAuthRepository(db),
		jwtMgr:            jwtMgr,
		blacklist:         blacklist,
		guard:             guard,
		allowRegistration: registrationEnabled(authCfg),
		audit:             NoopAuditLogger{},
	}
}

// NewAuthServiceWithRepo builds an AuthService with an explicit repository.
func NewAuthServiceWithRepo(repo repository.AuthRepository, jwtMgr *auth.JWTManager, blacklist auth.TokenStore, guard auth.LoginLimiter, authCfg ...config.AuthConfig) *AuthService {
	return &AuthService{
		repo:              repo,
		jwtMgr:            jwtMgr,
		blacklist:         blacklist,
		guard:             guard,
		allowRegistration: registrationEnabled(authCfg),
		audit:             NoopAuditLogger{},
	}
}

// SetAuditLogger wires the business-level audit logger. Must be called before
// the service handles requests; defaults to NoopAuditLogger when unset.
func (s *AuthService) SetAuditLogger(l AuditLogger) {
	if l != nil {
		s.audit = l
	}
}

func registrationEnabled(authCfg []config.AuthConfig) bool {
	return len(authCfg) > 0 && authCfg[0].AllowRegistration
}

// Login authenticates a user by username/email and password, records the login
// event, and returns a token pair together with the sanitized user profile.
// Accounts with TOTP enabled must use LoginWithTOTP (Login passes no code and
// will fail for them with ErrTOTPRequired).
func (s *AuthService) Login(username, password, clientIP, userAgent string) (*auth.TokenPair, *SafeUser, error) {
	return s.LoginWithTOTP(username, password, "", clientIP, userAgent)
}

// LoginWithTOTP is Login plus an optional TOTP/backup code. The second factor
// is checked after the password so the code can never be probed on its own,
// and before token generation so no session exists until both factors pass.
func (s *AuthService) LoginWithTOTP(username, password, totpCode, clientIP, userAgent string) (*auth.TokenPair, *SafeUser, error) {
	// Check if account is locked.
	if s.guard != nil {
		locked, remaining := s.guard.Check(username)
		if locked {
			s.audit.Log(AuditEvent{
				Action: "login.locked", Entity: "user", EntityID: 0,
				Details:   map[string]any{"username": username, "remaining_attempts": remaining},
				IP:        clientIP,
				UserAgent: userAgent,
			})
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
		s.audit.Log(AuditEvent{
			Action: "login.failed", Entity: "user", EntityID: 0,
			Details:   map[string]any{"username": username, "reason": "user_not_found"},
			IP:        clientIP,
			UserAgent: userAgent,
		})
		return nil, nil, errs.ErrInvalidCreds
	}

	if !user.IsActive() {
		s.audit.Log(AuditEvent{
			UserID: &user.ID,
			Action: "login.failed", Entity: "user", EntityID: user.ID,
			Details:   map[string]any{"username": username, "reason": "account_disabled"},
			IP:        clientIP,
			UserAgent: userAgent,
		})
		return nil, nil, errs.ErrAccountDisabled
	}

	if err := auth.CheckPassword(user.Password, password); err != nil {
		// Record failed attempt.
		if s.guard != nil {
			locked, _ := s.guard.RecordFailed(username)
			if locked {
				s.audit.Log(AuditEvent{
					UserID: &user.ID,
					Action: "login.locked", Entity: "user", EntityID: user.ID,
					Details:   map[string]any{"username": username, "reason": "max_password_attempts"},
					IP:        clientIP,
					UserAgent: userAgent,
				})
				return nil, nil, errs.ErrAccountLocked.WithMessage(fmt.Sprintf("account locked after %d failed attempts", s.guard.MaxAttempts()))
			}
		}
		s.audit.Log(AuditEvent{
			UserID: &user.ID,
			Action: "login.failed", Entity: "user", EntityID: user.ID,
			Details:   map[string]any{"username": username, "reason": "invalid_password"},
			IP:        clientIP,
			UserAgent: userAgent,
		})
		return nil, nil, errs.ErrInvalidCreds
	}

	// Login successful — reset guard.
	if s.guard != nil {
		s.guard.RecordSuccess(username)
	}

	// Second factor: checked after the password (and guard reset) so lockout
	// semantics for the first factor are unchanged, but before any token is
	// issued. Failed codes count as failed attempts to block TOTP brute force.
	if s.totp != nil {
		required, err := s.totp.Required(user.ID)
		if err != nil {
			return nil, nil, errs.ErrServiceUnavailable.Wrap(err)
		}
		if required {
			if totpCode == "" {
				s.audit.Log(AuditEvent{
					UserID: &user.ID,
					Action: "login.failed", Entity: "user", EntityID: user.ID,
					Details:   map[string]any{"username": username, "reason": "totp_required"},
					IP:        clientIP,
					UserAgent: userAgent,
				})
				return nil, nil, errs.ErrTOTPRequired
			}
			if err := s.totp.VerifyLogin(user.ID, totpCode); err != nil {
				if s.guard != nil {
					locked, _ := s.guard.RecordFailed(username)
					if locked {
						s.audit.Log(AuditEvent{
							UserID: &user.ID,
							Action: "login.locked", Entity: "user", EntityID: user.ID,
							Details:   map[string]any{"username": username, "reason": "max_totp_attempts"},
							IP:        clientIP,
							UserAgent: userAgent,
						})
						return nil, nil, errs.ErrAccountLocked.WithMessage(fmt.Sprintf("account locked after %d failed attempts", s.guard.MaxAttempts()))
					}
				}
				s.audit.Log(AuditEvent{
					UserID: &user.ID,
					Action: "login.failed", Entity: "user", EntityID: user.ID,
					Details:   map[string]any{"username": username, "reason": "invalid_totp"},
					IP:        clientIP,
					UserAgent: userAgent,
				})
				return nil, nil, err
			}
		}
	}

	tenantID := s.resolveUserTenant(user)
	tokenPair, err := s.jwtMgr.GenerateTokenPairWithTenant(
		user.ID, tenantID, user.Username, user.Email, user.Role.Slug, user.DisplayName,
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
		TenantID:  &tenantID,
		Action:    "login",
		Entity:    "user",
		EntityID:  user.ID,
		IP:        clientIP,
		UserAgent: userAgent,
	})
	s.audit.Log(AuditEvent{
		UserID: &user.ID,
		Action: "login.success", Entity: "user", EntityID: user.ID,
		Details:   map[string]any{"username": username},
		IP:        clientIP,
		UserAgent: userAgent,
	})

	return tokenPair, SanitizeUser(user), nil
}

// Register creates a new user account, assigns the default role, generates
// tokens, and returns them together with the sanitized user profile.
func (s *AuthService) Register(req RegisterRequest, clientIP string) (*auth.TokenPair, *SafeUser, error) {
	if !s.allowRegistration {
		return nil, nil, errs.ErrRegistrationDisabled
	}

	// Check if registration is enabled.
	if setting, err := s.repo.FindSetting("enable_registration", models.DefaultTenantID); err == nil {
		if setting.Value == "false" {
			return nil, nil, errs.ErrRegistrationDisabled
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

	// Create default-tenant membership so the new user can access content.
	_ = s.repo.CreateMembership(&models.TenantMembership{
		TenantID: models.DefaultTenantID,
		UserID:   user.ID,
		RoleSlug: models.TenantRoleMember,
	})

	// Reload with role to generate tokens.
	userWithRole, err := s.repo.FindUserByIDWithRole(user.ID)
	if err != nil {
		return nil, nil, errs.ErrInternal.Wrap(fmt.Errorf("user created but role reload failed: %w", err))
	}
	tokenPair, err := s.jwtMgr.GenerateTokenPairWithTenant(
		userWithRole.ID, models.DefaultTenantID, userWithRole.Username, userWithRole.Email, userWithRole.Role.Slug, userWithRole.DisplayName,
	)
	if err != nil {
		return nil, nil, errs.ErrInternal.Wrap(fmt.Errorf("user created but token generation failed: %w", err))
	}

	s.audit.Log(AuditEvent{
		UserID: &userWithRole.ID,
		Action: "user.register", Entity: "user", EntityID: userWithRole.ID,
		Details: map[string]any{"username": userWithRole.Username, "email": userWithRole.Email},
		IP:      clientIP,
	})

	return tokenPair, SanitizeUser(userWithRole), nil
}

// RefreshToken validates a refresh token, loads the user's current state from
// the database, and issues a new token pair.
//
// Loading the user and tenant authorization on every refresh ensures role
// changes, disablement, tenant suspension, or membership removal take effect
// immediately. Previously refresh reused stale claims and did not preserve or
// revalidate the tenant bound to the session (A-1 security fix).
func (s *AuthService) RefreshToken(refreshToken string) (*auth.TokenPair, error) {
	claims, err := s.jwtMgr.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, errs.ErrUnauthorized.WithMessage("invalid refresh token")
	}
	if s.blacklist == nil {
		return nil, errs.ErrServiceUnavailable.WithMessage("token revocation store unavailable")
	}
	consumed, err := s.blacklist.Consume(refreshToken, claims.ExpiresAt.Time)
	if err != nil {
		return nil, errs.ErrServiceUnavailable.Wrap(err)
	}
	if !consumed {
		return nil, errs.ErrTokenRevoked.WithMessage("refresh token has already been used or revoked")
	}

	user, err := s.repo.FindUserByIDWithRole(claims.UserID)
	if err != nil {
		return nil, errs.ErrUnauthorized.WithMessage("user not found")
	}

	if user.Status != models.UserStatusActive {
		return nil, errs.ErrUnauthorized.WithMessage("user is not active")
	}

	tenantID, err := s.resolveRefreshTenant(user, claims.TenantID)
	if err != nil {
		return nil, err
	}
	return s.jwtMgr.GenerateTokenPairWithTenant(user.ID, tenantID, user.Username, user.Email, user.Role.Slug, user.DisplayName)
}

// resolveRefreshTenant validates an explicit tenant carried by a refresh token,
// or deterministically resolves a legacy TenantID=0 token. Tenant existence and
// active status are always rechecked. Non-platform-admin users must still have
// a membership for the selected tenant.
func (s *AuthService) resolveRefreshTenant(user *models.User, requestedTenantID uint) (uint, error) {
	if requestedTenantID == 0 {
		return s.resolveLegacyRefreshTenant(user)
	}

	tenant, err := s.repo.FindTenantByID(requestedTenantID)
	if err != nil || tenant.Status != models.TenantStatusActive {
		return 0, errs.ErrUnauthorized.WithMessage("tenant is unavailable")
	}
	if user.Role.Slug == "admin" {
		return requestedTenantID, nil
	}

	memberships, err := s.repo.FindUserMemberships(user.ID)
	if err != nil {
		return 0, errs.ErrUnauthorized.WithMessage("tenant access is unavailable")
	}
	for _, membership := range memberships {
		if membership.TenantID == requestedTenantID {
			if _, ok := permissions.NormalizeTenantRole(membership.RoleSlug); !ok {
				return 0, errs.ErrUnauthorized.WithMessage("tenant membership role is invalid")
			}
			return requestedTenantID, nil
		}
	}
	return 0, errs.ErrUnauthorized.WithMessage("tenant membership is required")
}

// resolveLegacyRefreshTenant selects a tenant for pre-multi-tenancy refresh
// tokens. Platform admins prefer the active default tenant. Other users, and
// admins whose default tenant is unavailable, select the first active tenant
// after memberships are ordered deterministically (default first, then ID).
func (s *AuthService) resolveLegacyRefreshTenant(user *models.User) (uint, error) {
	if user.Role.Slug == "admin" {
		if tenant, err := s.repo.FindTenantByID(models.DefaultTenantID); err == nil && tenant.Status == models.TenantStatusActive {
			return models.DefaultTenantID, nil
		}
	}

	memberships, err := s.repo.FindUserMemberships(user.ID)
	if err != nil {
		return 0, errs.ErrUnauthorized.WithMessage("tenant access is unavailable")
	}
	if user.Role.Slug != "admin" {
		for _, membership := range memberships {
			if _, ok := permissions.NormalizeTenantRole(membership.RoleSlug); !ok {
				return 0, errs.ErrUnauthorized.WithMessage("tenant membership role is invalid")
			}
		}
	}
	orderMemberships(memberships)
	for _, membership := range memberships {
		if membership.TenantID == 0 {
			continue
		}
		tenant, err := s.repo.FindTenantByID(membership.TenantID)
		if err == nil && tenant.Status == models.TenantStatusActive {
			return membership.TenantID, nil
		}
	}
	return 0, errs.ErrUnauthorized.WithMessage("active tenant membership is required")
}

// resolveUserTenant binds login tokens to the first usable tenant in a stable
// order. Suspended/missing tenants and unrecognized membership roles are
// skipped so one stale membership cannot make an otherwise valid account mint
// an immediately unusable session. The default fallback keeps self-service and
// platform-only accounts able to authenticate; TenantGuard still denies them
// from tenant-scoped routes until a valid membership exists.
func (s *AuthService) resolveUserTenant(user *models.User) uint {
	if user == nil {
		return models.DefaultTenantID
	}
	if user.Role.Slug == "admin" {
		if tenant, err := s.repo.FindTenantByID(models.DefaultTenantID); err == nil && tenant.Status == models.TenantStatusActive {
			return models.DefaultTenantID
		}
	}

	memberships, err := s.repo.FindUserMemberships(user.ID)
	if err != nil || len(memberships) == 0 {
		return models.DefaultTenantID
	}
	orderMemberships(memberships)
	for _, membership := range memberships {
		if membership.TenantID == 0 {
			continue
		}
		if user.Role.Slug != "admin" {
			if _, ok := permissions.NormalizeTenantRole(membership.RoleSlug); !ok {
				continue
			}
		}
		tenant, tenantErr := s.repo.FindTenantByID(membership.TenantID)
		if tenantErr == nil && tenant.Status == models.TenantStatusActive {
			return membership.TenantID
		}
	}
	return models.DefaultTenantID
}

func orderMemberships(memberships []models.TenantMembership) {
	sort.SliceStable(memberships, func(i, j int) bool {
		left := memberships[i].TenantID
		right := memberships[j].TenantID
		if left == models.DefaultTenantID && right != models.DefaultTenantID {
			return true
		}
		if right == models.DefaultTenantID && left != models.DefaultTenantID {
			return false
		}
		if left == right {
			return memberships[i].ID < memberships[j].ID
		}
		return left < right
	})
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
	claims, err := s.jwtMgr.ValidateAccessToken(accessToken)
	if err != nil {
		return errs.ErrUnauthorized.WithMessage("invalid token")
	}

	if s.blacklist != nil {
		s.blacklist.Revoke(accessToken, claims.ExpiresAt.Time)
	}

	if refreshToken != "" && s.blacklist != nil {
		if rClaims, err := s.jwtMgr.ValidateRefreshToken(refreshToken); err == nil {
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

	if err := s.repo.UpdateUserPassword(userID, newHash); err != nil {
		return err
	}

	uid := userID
	s.audit.Log(AuditEvent{
		UserID: &uid,
		Action: "user.password_change", Entity: "user", EntityID: userID,
		Details: map[string]any{"username": user.Username},
	})
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
