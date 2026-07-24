package services

import (
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/errs"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// newTestJWTManager 构建一个用于测试的 JWTManager。
func newTestJWTManager() *auth.JWTManager {
	return auth.NewJWTManager(config.JWTConfig{
		Secret:          "test-secret-key-for-unit-tests",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		Issuer:          "contentx-test",
	})
}

// hashTestPassword 使用真实 bcrypt 哈希一个测试密码。
func hashTestPassword(t *testing.T, password string) string {
	t.Helper()
	hashed, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	return hashed
}

func TestMockAuth_NewWithRepo(t *testing.T) {
	repo := &MockAuthRepository{}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

// ---------- Logout ----------

func TestMockAuth_Logout_Success(t *testing.T) {
	jwtMgr := newTestJWTManager()
	bl := auth.NewBlacklist()
	repo := &MockAuthRepository{}
	svc := NewAuthServiceWithRepo(repo, jwtMgr, bl, nil)

	pair, err := jwtMgr.GenerateTokenPair(1, "alice", "alice@example.com", "editor", "Alice")
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	if err := svc.Logout(pair.AccessToken, ""); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}
	if !bl.IsRevoked(pair.AccessToken) {
		t.Error("expected token to be blacklisted")
	}
}

func TestMockAuth_Logout_BlacklistsRefreshToken(t *testing.T) {
	jwtMgr := newTestJWTManager()
	bl := auth.NewBlacklist()
	repo := &MockAuthRepository{}
	svc := NewAuthServiceWithRepo(repo, jwtMgr, bl, nil)

	pair, err := jwtMgr.GenerateTokenPair(1, "alice", "alice@example.com", "editor", "Alice")
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	if err := svc.Logout(pair.AccessToken, pair.RefreshToken); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}
	if !bl.IsRevoked(pair.AccessToken) {
		t.Error("expected access token to be blacklisted")
	}
	if !bl.IsRevoked(pair.RefreshToken) {
		t.Error("expected refresh token to be blacklisted (A-3 fix)")
	}
}

func TestMockAuth_Logout_InvalidToken(t *testing.T) {
	repo := &MockAuthRepository{}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	err := svc.Logout("not-a-valid-token", "")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	appErr, ok := err.(*errs.AppError)
	if !ok {
		t.Fatalf("expected *errs.AppError, got %T", err)
	}
	if appErr.Code != "UNAUTHORIZED" {
		t.Errorf("expected code UNAUTHORIZED, got %s", appErr.Code)
	}
}

// ---------- UpdateProfile ----------

func TestMockAuth_UpdateProfile_Success(t *testing.T) {
	user := &models.User{BaseModel: models.BaseModel{ID: 1}, Username: "alice"}
	repo := &MockAuthRepository{
		UserByID:         user,
		UserByIDWithRole: user,
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	result, err := svc.UpdateProfile(1, map[string]interface{}{
		"display_name": "Alice Updated",
		"bio":          "new bio",
		"website":      "https://alice.example",
		"avatar":       "https://cdn.example/a.png",
		"password":     "should-be-ignored", // not in allowed list
	})
	if err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(repo.UpdatedUserFields) != 1 {
		t.Fatalf("expected 1 UpdateUserFields call, got %d", len(repo.UpdatedUserFields))
	}
	updates := repo.UpdatedUserFields[0]
	for _, allowed := range []string{"display_name", "bio", "website", "avatar"} {
		if _, ok := updates[allowed]; !ok {
			t.Errorf("expected allowed field %q in updates", allowed)
		}
	}
	if _, ok := updates["password"]; ok {
		t.Error("password should not be in updates")
	}
}

func TestMockAuth_UpdateProfile_UserNotFound(t *testing.T) {
	repo := &MockAuthRepository{
		FindUserByIDErr: gorm.ErrRecordNotFound,
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	_, err := svc.UpdateProfile(99, map[string]interface{}{"bio": "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	appErr, ok := err.(*errs.AppError)
	if !ok || appErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND, got %v", err)
	}
}

func TestMockAuth_UpdateProfile_ReloadNotFound(t *testing.T) {
	repo := &MockAuthRepository{
		UserByID:                &models.User{BaseModel: models.BaseModel{ID: 1}},
		FindUserByIDWithRoleErr: gorm.ErrRecordNotFound,
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	_, err := svc.UpdateProfile(1, map[string]interface{}{"bio": "x"})
	if err == nil {
		t.Fatal("expected error on reload")
	}
}

func TestMockAuth_UpdateProfile_NoAllowedFields(t *testing.T) {
	user := &models.User{BaseModel: models.BaseModel{ID: 1}}
	repo := &MockAuthRepository{
		UserByID:         user,
		UserByIDWithRole: user,
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	// All fields disallowed → updates empty, no UpdateUserFields call.
	_, err := svc.UpdateProfile(1, map[string]interface{}{"password": "x", "role_id": 2})
	if err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}
	if len(repo.UpdatedUserFields) != 0 {
		t.Errorf("expected 0 UpdateUserFields calls, got %d", len(repo.UpdatedUserFields))
	}
}

// ---------- Login ----------

func TestMockAuth_Login_Locked(t *testing.T) {
	guard := auth.NewLoginGuard(auth.WithMaxAttempts(1), auth.WithLockDuration(time.Hour))
	guard.RecordFailed("alice") // trigger lock with 1 attempt

	repo := &MockAuthRepository{}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), guard)

	_, _, err := svc.Login("alice", "Password1", "1.2.3.4", "ua")
	if err != errs.ErrAccountLocked {
		t.Errorf("expected ErrAccountLocked, got %v", err)
	}
}

func TestMockAuth_Login_UserNotFound(t *testing.T) {
	guard := auth.NewLoginGuard()
	repo := &MockAuthRepository{
		FindUserByUsernameOrEmailErr: gorm.ErrRecordNotFound,
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), guard)

	_, _, err := svc.Login("ghost", "Password1", "1.2.3.4", "ua")
	if err != errs.ErrInvalidCreds {
		t.Errorf("expected ErrInvalidCreds, got %v", err)
	}
}

func TestMockAuth_Login_InactiveUser(t *testing.T) {
	repo := &MockAuthRepository{
		UserByUsernameOrEmail: &models.User{
			BaseModel: models.BaseModel{ID: 1},
			Username:  "alice",
			Status:    models.UserStatusInactive,
			Password:  hashTestPassword(t, "Password1"),
		},
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	_, _, err := svc.Login("alice", "Password1", "1.2.3.4", "ua")
	if err != errs.ErrAccountDisabled {
		t.Errorf("expected ErrAccountDisabled, got %v", err)
	}
}

func TestMockAuth_Login_WrongPassword_Lockout(t *testing.T) {
	guard := auth.NewLoginGuard(auth.WithMaxAttempts(1), auth.WithLockDuration(time.Hour))
	repo := &MockAuthRepository{
		UserByUsernameOrEmail: &models.User{
			BaseModel: models.BaseModel{ID: 1},
			Username:  "alice",
			Status:    models.UserStatusActive,
			Password:  hashTestPassword(t, "Password1"),
			Role:      models.Role{Slug: "editor"},
		},
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), guard)

	_, _, err := svc.Login("alice", "WrongPassword1", "1.2.3.4", "ua")
	if err == nil {
		t.Fatal("expected error")
	}
	appErr, ok := err.(*errs.AppError)
	if !ok || appErr.Code != "ACCOUNT_LOCKED" {
		t.Errorf("expected ACCOUNT_LOCKED, got %v", err)
	}
}

func TestMockAuth_Login_Success(t *testing.T) {
	repo := &MockAuthRepository{
		UserByUsernameOrEmail: &models.User{
			BaseModel:   models.BaseModel{ID: 1},
			Username:    "alice",
			Email:       "alice@example.com",
			DisplayName: "Alice",
			Status:      models.UserStatusActive,
			Password:    hashTestPassword(t, "Password1"),
			Role:        models.Role{Slug: "editor"},
		},
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	pair, safe, err := svc.Login("alice", "Password1", "1.2.3.4", "ua")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if pair == nil || safe == nil {
		t.Fatal("expected non-nil pair and safe user")
	}
	if safe.Username != "alice" {
		t.Errorf("expected username alice, got %s", safe.Username)
	}
	if len(repo.CreatedActivityLogs) != 1 {
		t.Errorf("expected 1 activity log, got %d", len(repo.CreatedActivityLogs))
	}
	if len(repo.UpdatedUserFields) != 1 {
		t.Errorf("expected 1 UpdateUserFields call, got %d", len(repo.UpdatedUserFields))
	}
}

// ---------- Register ----------

func TestMockAuth_Register_Disabled(t *testing.T) {
	repo := &MockAuthRepository{
		Setting: &models.SiteSetting{Key: "enable_registration", Value: "false"},
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	_, _, err := svc.Register(RegisterRequest{
		Username: "newuser", Email: "new@example.com", Password: "Password1",
	}, "1.2.3.4")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMockAuth_Register_DuplicateUser(t *testing.T) {
	repo := &MockAuthRepository{
		FindSettingErr:         gorm.ErrRecordNotFound, // registration enabled (setting not configured)
		CountByUsernameOrEmail: 1,
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	_, _, err := svc.Register(RegisterRequest{
		Username: "alice", Email: "alice@example.com", Password: "Password1",
	}, "1.2.3.4")
	if err != errs.ErrDuplicateUser {
		t.Errorf("expected ErrDuplicateUser, got %v", err)
	}
}

func TestMockAuth_Register_DefaultRoleFallback(t *testing.T) {
	repo := &MockAuthRepository{
		FindSettingErr:     gorm.ErrRecordNotFound,
		FindDefaultRoleErr: gorm.ErrRecordNotFound,
		RoleBySlug:         &models.Role{BaseModel: models.BaseModel{ID: 5}, Slug: "subscriber"},
		UserByIDWithRole: &models.User{
			BaseModel:   models.BaseModel{ID: 1},
			Username:    "newuser",
			Email:       "new@example.com",
			DisplayName: "newuser",
			Role:        models.Role{Slug: "subscriber"},
		},
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	pair, safe, err := svc.Register(RegisterRequest{
		Username: "newuser", Email: "new@example.com", Password: "Password1",
	}, "1.2.3.4")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if pair == nil || safe == nil {
		t.Fatal("expected non-nil pair and safe user")
	}
	if len(repo.CreatedUsers) != 1 {
		t.Errorf("expected 1 created user, got %d", len(repo.CreatedUsers))
	}
}

func TestMockAuth_Register_NoDefaultRole(t *testing.T) {
	repo := &MockAuthRepository{
		FindSettingErr:     gorm.ErrRecordNotFound,
		FindDefaultRoleErr: gorm.ErrRecordNotFound,
		FindRoleBySlugErr:  gorm.ErrRecordNotFound,
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	_, _, err := svc.Register(RegisterRequest{
		Username: "newuser", Email: "new@example.com", Password: "Password1",
	}, "1.2.3.4")
	if err == nil {
		t.Fatal("expected error")
	}
	appErr, ok := err.(*errs.AppError)
	if !ok || appErr.Code != "INTERNAL_ERROR" {
		t.Errorf("expected INTERNAL_ERROR, got %v", err)
	}
}

func TestMockAuth_Register_CreateUserError(t *testing.T) {
	repo := &MockAuthRepository{
		FindSettingErr: gorm.ErrRecordNotFound,
		DefaultRole:    &models.Role{BaseModel: models.BaseModel{ID: 5}, Slug: "subscriber"},
		CreateUserErr:  gorm.ErrInvalidDB,
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	_, _, err := svc.Register(RegisterRequest{
		Username: "newuser", Email: "new@example.com", Password: "Password1",
	}, "1.2.3.4")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMockAuth_Register_Success(t *testing.T) {
	repo := &MockAuthRepository{
		FindSettingErr: gorm.ErrRecordNotFound,
		DefaultRole:    &models.Role{BaseModel: models.BaseModel{ID: 5}, Slug: "subscriber"},
		UserByIDWithRole: &models.User{
			BaseModel:   models.BaseModel{ID: 1},
			Username:    "newuser",
			Email:       "new@example.com",
			DisplayName: "New User",
			Role:        models.Role{Slug: "subscriber"},
		},
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	pair, safe, err := svc.Register(RegisterRequest{
		Username: "newuser", Email: "new@example.com", Password: "Password1", DisplayName: "New User",
	}, "1.2.3.4")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if pair == nil || safe == nil {
		t.Fatal("expected non-nil pair and safe user")
	}
}

// ---------- Me ----------

func TestMockAuth_Me_UserNotFound(t *testing.T) {
	repo := &MockAuthRepository{
		FindUserByIDWithPermsErr: gorm.ErrRecordNotFound,
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	_, _, err := svc.Me(99)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMockAuth_Me_Success(t *testing.T) {
	repo := &MockAuthRepository{
		UserByIDWithPerms: &models.User{
			BaseModel:   models.BaseModel{ID: 1},
			Username:    "alice",
			Email:       "alice@example.com",
			DisplayName: "Alice",
			Status:      models.UserStatusActive,
			Role: models.Role{
				Slug: "editor",
				Permissions: []models.Permission{
					{BaseModel: models.BaseModel{ID: 1}, Slug: "articles.edit"},
					{BaseModel: models.BaseModel{ID: 2}, Slug: "articles.delete"},
				},
			},
		},
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	safe, perms, err := svc.Me(1)
	if err != nil {
		t.Fatalf("Me failed: %v", err)
	}
	if safe == nil {
		t.Fatal("expected non-nil safe user")
	}
	if len(perms) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(perms))
	}
	if perms[0] != "articles.edit" {
		t.Errorf("expected first perm articles.edit, got %s", perms[0])
	}
}

// ---------- ChangePassword ----------

func TestMockAuth_ChangePassword_UserNotFound(t *testing.T) {
	repo := &MockAuthRepository{
		FindUserByIDErr: gorm.ErrRecordNotFound,
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	err := svc.ChangePassword(99, "OldPass1", "NewPass1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMockAuth_ChangePassword_WrongOldPassword(t *testing.T) {
	repo := &MockAuthRepository{
		UserByID: &models.User{
			BaseModel: models.BaseModel{ID: 1},
			Password:  hashTestPassword(t, "CorrectPass1"),
		},
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	err := svc.ChangePassword(1, "WrongOldPass1", "NewPass1")
	if err == nil {
		t.Fatal("expected error for wrong old password")
	}
}

func TestMockAuth_ChangePassword_Success(t *testing.T) {
	repo := &MockAuthRepository{
		UserByID: &models.User{
			BaseModel: models.BaseModel{ID: 1},
			Password:  hashTestPassword(t, "OldPass1"),
		},
	}
	svc := NewAuthServiceWithRepo(repo, newTestJWTManager(), auth.NewBlacklist(), nil)

	if err := svc.ChangePassword(1, "OldPass1", "NewPass1"); err != nil {
		t.Fatalf("ChangePassword failed: %v", err)
	}
	if len(repo.UpdatedPasswords) != 1 {
		t.Fatalf("expected 1 password update, got %d", len(repo.UpdatedPasswords))
	}
	if repo.UpdatedPasswords[0].ID != 1 {
		t.Errorf("expected user ID 1, got %d", repo.UpdatedPasswords[0].ID)
	}
}

// ---------- RefreshToken ----------

func TestMockAuth_RefreshToken_Invalid(t *testing.T) {
	svc := NewAuthServiceWithRepo(&MockAuthRepository{}, newTestJWTManager(), auth.NewBlacklist(), nil)

	_, err := svc.RefreshToken("invalid-refresh-token")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMockAuth_RefreshToken_Success(t *testing.T) {
	jwtMgr := newTestJWTManager()
	repo := &MockAuthRepository{
		UserByIDWithRole: &models.User{
			BaseModel:   models.BaseModel{ID: 1},
			Username:    "alice",
			Email:       "alice@example.com",
			DisplayName: "Alice",
			Status:      models.UserStatusActive,
			Role:        models.Role{Slug: "editor"},
		},
	}
	svc := NewAuthServiceWithRepo(repo, jwtMgr, auth.NewBlacklist(), nil)

	pair, err := jwtMgr.GenerateTokenPair(1, "alice", "alice@example.com", "editor", "Alice")
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	newPair, err := svc.RefreshToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}
	if newPair == nil {
		t.Fatal("expected non-nil pair")
	}
}

// TestMockAuth_RefreshToken_RoleChanged verifies that a role change takes
// effect on the next refresh. Previously the refresh reused stale claims,
// so a demoted user kept their old role until the refresh token expired.
func TestMockAuth_RefreshToken_RoleChanged(t *testing.T) {
	jwtMgr := newTestJWTManager()
	// Issue a refresh token that carries the "admin" role.
	pair, err := jwtMgr.GenerateTokenPair(1, "alice", "alice@example.com", "admin", "Alice")
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	// The database now reflects a demotion to "editor".
	repo := &MockAuthRepository{
		UserByIDWithRole: &models.User{
			BaseModel:   models.BaseModel{ID: 1},
			Username:    "alice",
			Email:       "alice@example.com",
			DisplayName: "Alice",
			Status:      models.UserStatusActive,
			Role:        models.Role{Slug: "editor"},
		},
	}
	svc := NewAuthServiceWithRepo(repo, jwtMgr, auth.NewBlacklist(), nil)

	newPair, err := svc.RefreshToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}

	claims, err := jwtMgr.ValidateToken(newPair.AccessToken)
	if err != nil {
		t.Fatalf("validate refreshed token: %v", err)
	}
	if claims.RoleSlug != "editor" {
		t.Fatalf("expected refreshed role editor (loaded from DB), got %q", claims.RoleSlug)
	}
}

// TestMockAuth_RefreshToken_InactiveUser verifies that a disabled user cannot
// refresh their token. Previously the refresh succeeded because it never
// consulted the database.
func TestMockAuth_RefreshToken_InactiveUser(t *testing.T) {
	jwtMgr := newTestJWTManager()
	pair, err := jwtMgr.GenerateTokenPair(1, "alice", "alice@example.com", "editor", "Alice")
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	repo := &MockAuthRepository{
		UserByIDWithRole: &models.User{
			BaseModel: models.BaseModel{ID: 1},
			Username:  "alice",
			Status:    models.UserStatusInactive,
			Role:      models.Role{Slug: "editor"},
		},
	}
	svc := NewAuthServiceWithRepo(repo, jwtMgr, auth.NewBlacklist(), nil)

	_, err = svc.RefreshToken(pair.RefreshToken)
	if err == nil {
		t.Fatal("expected error for inactive user")
	}
	appErr, ok := err.(*errs.AppError)
	if !ok || appErr.Code != "UNAUTHORIZED" {
		t.Errorf("expected UNAUTHORIZED, got %v", err)
	}
}

// TestMockAuth_RefreshToken_UserNotFound verifies that a deleted user cannot
// refresh their token. Previously the refresh succeeded because it never
// consulted the database.
func TestMockAuth_RefreshToken_UserNotFound(t *testing.T) {
	jwtMgr := newTestJWTManager()
	pair, err := jwtMgr.GenerateTokenPair(99, "ghost", "ghost@example.com", "editor", "Ghost")
	if err != nil {
		t.Fatalf("GenerateTokenPair failed: %v", err)
	}

	repo := &MockAuthRepository{
		FindUserByIDWithRoleErr: gorm.ErrRecordNotFound,
	}
	svc := NewAuthServiceWithRepo(repo, jwtMgr, auth.NewBlacklist(), nil)

	_, err = svc.RefreshToken(pair.RefreshToken)
	if err == nil {
		t.Fatal("expected error for deleted user")
	}
	appErr, ok := err.(*errs.AppError)
	if !ok || appErr.Code != "UNAUTHORIZED" {
		t.Errorf("expected UNAUTHORIZED, got %v", err)
	}
}
