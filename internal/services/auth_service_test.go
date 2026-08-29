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

func TestAuthService_Login_Success(t *testing.T) {
	db := setupTestDB(t)
	jwtMgr := auth.NewJWTManager(config.JWTConfig{
		Secret:          "test-secret-key-for-testing-only!",
		AccessTokenTTL:  15 * time.Minute,   // 15m
		RefreshTokenTTL: 7 * 24 * time.Hour, // 7d
		Issuer:          "test",
	})
	blacklist := auth.NewBlacklist()
	svc := NewAuthService(db, jwtMgr, blacklist, nil)

	// Seed creates an admin user with a random password.
	// We need to create a user with a known password instead.
	user := createTestUser(t, db, "logintest", "subscriber")

	tp, safeUser, err := svc.Login("logintest", "TestPass1", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("Login() error: %v", err)
	}

	if tp.AccessToken == "" {
		t.Error("AccessToken should not be empty")
	}
	if tp.RefreshToken == "" {
		t.Error("RefreshToken should not be empty")
	}
	if safeUser.Username != "logintest" {
		t.Errorf("Username = %q, want %q", safeUser.Username, "logintest")
	}
	_ = user
}

func TestAuthService_Login_SelectsFirstUsableTenant(t *testing.T) {
	db := setupTestDB(t)
	ensureDefaultTestTenant(t, db)
	jwtMgr := newAuthServiceTestJWTManager()
	svc := NewAuthService(db, jwtMgr, auth.NewBlacklist(), nil)
	user := createTestUser(t, db, "login-tenant-selection", "subscriber")

	suspended := createTestTenant(t, db, "Suspended First", "suspended-first", models.TenantStatusSuspended)
	active := createTestTenant(t, db, "Active Second", "active-second", models.TenantStatusActive)
	createTestMembership(t, db, user.ID, suspended.ID)
	createTestMembership(t, db, user.ID, active.ID)

	pair, _, err := svc.Login(user.Username, "TestPass1", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	assertTokenPairTenant(t, jwtMgr, pair, active.ID)

	// A legacy/custom role on the first membership is also skipped instead of
	// poisoning the valid membership that follows it.
	if err := db.Model(&models.TenantMembership{}).
		Where("tenant_id = ? AND user_id = ?", suspended.ID, user.ID).
		Update("role_slug", "reviewer").Error; err != nil {
		t.Fatalf("set legacy membership role: %v", err)
	}
	if err := db.Model(&models.Tenant{}).Where("id = ?", suspended.ID).
		Update("status", models.TenantStatusActive).Error; err != nil {
		t.Fatalf("activate legacy-role tenant: %v", err)
	}
	pair, _, err = svc.Login(user.Username, "TestPass1", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("Login after legacy membership: %v", err)
	}
	assertTokenPairTenant(t, jwtMgr, pair, active.ID)
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	db := setupTestDB(t)
	jwtMgr := auth.NewJWTManager(config.JWTConfig{
		Secret: "test-secret", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 7 * 24 * time.Hour, Issuer: "test",
	})
	blacklist := auth.NewBlacklist()
	svc := NewAuthService(db, jwtMgr, blacklist, nil)

	createTestUser(t, db, "logintest2", "subscriber")

	_, _, err := svc.Login("logintest2", "WrongPass1", "127.0.0.1", "test-agent")
	if err == nil {
		t.Error("Login() should fail with wrong password")
	}
}

func TestAuthService_Login_NonExistentUser(t *testing.T) {
	db := setupTestDB(t)
	jwtMgr := auth.NewJWTManager(config.JWTConfig{
		Secret: "test-secret", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 7 * 24 * time.Hour, Issuer: "test",
	})
	blacklist := auth.NewBlacklist()
	svc := NewAuthService(db, jwtMgr, blacklist, nil)

	_, _, err := svc.Login("nobody", "TestPass1", "127.0.0.1", "test-agent")
	if err == nil {
		t.Error("Login() should fail for non-existent user")
	}
}

func TestAuthService_Register_Success(t *testing.T) {
	db := setupTestDB(t)
	jwtMgr := auth.NewJWTManager(config.JWTConfig{
		Secret: "test-secret", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 7 * 24 * time.Hour, Issuer: "test",
	})
	blacklist := auth.NewBlacklist()
	svc := NewAuthService(db, jwtMgr, blacklist, nil, config.AuthConfig{AllowRegistration: true})

	req := RegisterRequest{
		Username: "newuser",
		Email:    "new@test.com",
		Password: "NewPass123!",
	}

	tp, safeUser, err := svc.Register(req, "127.0.0.1")
	if err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	if tp.AccessToken == "" {
		t.Error("AccessToken should not be empty")
	}
	if safeUser.Username != "newuser" {
		t.Errorf("Username = %q, want %q", safeUser.Username, "newuser")
	}
	if safeUser.Email != "new@test.com" {
		t.Errorf("Email = %q, want %q", safeUser.Email, "new@test.com")
	}
}

func TestAuthService_Register_Duplicate(t *testing.T) {
	db := setupTestDB(t)
	jwtMgr := auth.NewJWTManager(config.JWTConfig{
		Secret: "test-secret", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 7 * 24 * time.Hour, Issuer: "test",
	})
	blacklist := auth.NewBlacklist()
	svc := NewAuthService(db, jwtMgr, blacklist, nil, config.AuthConfig{AllowRegistration: true})

	req := RegisterRequest{
		Username: "dupuser",
		Email:    "dup@test.com",
		Password: "DupPass123!",
	}

	svc.Register(req, "127.0.0.1")

	// Try again with same username.
	_, _, err := svc.Register(req, "127.0.0.1")
	if err == nil {
		t.Error("Register() should fail for duplicate username")
	}
}

func TestAuthService_RefreshToken(t *testing.T) {
	db := setupTestDB(t)
	ensureDefaultTestTenant(t, db)
	jwtMgr := auth.NewJWTManager(config.JWTConfig{
		Secret: "test-secret", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 7 * 24 * time.Hour, Issuer: "test",
	})
	blacklist := auth.NewBlacklist()
	svc := NewAuthService(db, jwtMgr, blacklist, nil)

	user := createTestUser(t, db, "refreshuser", "subscriber")
	createTestMembership(t, db, user.ID, models.DefaultTenantID)
	tp, _, loginErr := svc.Login("refreshuser", "TestPass1", "127.0.0.1", "test-agent")
	if loginErr != nil {
		t.Fatalf("Login() error: %v", loginErr)
	}

	newTP, err := svc.RefreshToken(tp.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken() error: %v", err)
	}
	if newTP.AccessToken == "" {
		t.Error("New AccessToken should not be empty")
	}
	assertTokenPairTenant(t, jwtMgr, newTP, models.DefaultTenantID)
}

func TestAuthService_RefreshToken_PreservesExplicitTenant(t *testing.T) {
	db := setupTestDB(t)
	ensureDefaultTestTenant(t, db)
	jwtMgr := newAuthServiceTestJWTManager()
	svc := NewAuthService(db, jwtMgr, auth.NewBlacklist(), nil)
	user := createTestUser(t, db, "refresh-tenant", "editor")
	tenant := createTestTenant(t, db, "Refresh Tenant", "refresh-tenant", models.TenantStatusActive)
	createTestMembership(t, db, user.ID, tenant.ID)

	pair, err := jwtMgr.GenerateTokenPairWithTenant(
		user.ID, tenant.ID, user.Username, user.Email, user.Role.Slug, user.DisplayName,
	)
	if err != nil {
		t.Fatalf("GenerateTokenPairWithTenant: %v", err)
	}

	refreshed, err := svc.RefreshToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	assertTokenPairTenant(t, jwtMgr, refreshed, tenant.ID)
}

func TestAuthService_RefreshToken_RejectsInvalidTenantState(t *testing.T) {
	tests := []struct {
		name             string
		createTenant     bool
		tenantStatus     string
		createMembership bool
	}{
		{name: "tenant missing"},
		{name: "tenant suspended", createTenant: true, tenantStatus: models.TenantStatusSuspended, createMembership: true},
		{name: "membership removed", createTenant: true, tenantStatus: models.TenantStatusActive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			ensureDefaultTestTenant(t, db)
			jwtMgr := newAuthServiceTestJWTManager()
			svc := NewAuthService(db, jwtMgr, auth.NewBlacklist(), nil)
			user := createTestUser(t, db, "refresh-invalid-tenant", "editor")

			tenantID := uint(999999)
			if tt.createTenant {
				tenant := createTestTenant(t, db, "Invalid Refresh Tenant", "invalid-refresh-tenant", tt.tenantStatus)
				tenantID = tenant.ID
				if tt.createMembership {
					createTestMembership(t, db, user.ID, tenant.ID)
				}
			}

			pair, err := jwtMgr.GenerateTokenPairWithTenant(
				user.ID, tenantID, user.Username, user.Email, user.Role.Slug, user.DisplayName,
			)
			if err != nil {
				t.Fatalf("GenerateTokenPairWithTenant: %v", err)
			}
			_, err = svc.RefreshToken(pair.RefreshToken)
			requireUnauthorized(t, err)
		})
	}
}

func TestAuthService_RefreshToken_RejectsInvalidMembershipRole(t *testing.T) {
	tests := []struct {
		name   string
		role   string
		legacy bool
	}{
		{name: "explicit tenant empty role", role: ""},
		{name: "explicit tenant unknown role", role: "owner"},
		{name: "legacy token unknown role", role: "owner", legacy: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			ensureDefaultTestTenant(t, db)
			jwtMgr := newAuthServiceTestJWTManager()
			svc := NewAuthService(db, jwtMgr, auth.NewBlacklist(), nil)
			user := createTestUser(t, db, "refresh-invalid-membership-role", "editor")
			tenant := createTestTenant(t, db, "Invalid Membership Role", "invalid-membership-role", models.TenantStatusActive)
			membership := createTestMembership(t, db, user.ID, tenant.ID)
			if err := db.Model(membership).Update("role_slug", tt.role).Error; err != nil {
				t.Fatalf("set membership role %q: %v", tt.role, err)
			}

			var pair *auth.TokenPair
			var err error
			if tt.legacy {
				pair, err = jwtMgr.GenerateTokenPair(
					user.ID, user.Username, user.Email, user.Role.Slug, user.DisplayName,
				)
			} else {
				pair, err = jwtMgr.GenerateTokenPairWithTenant(
					user.ID, tenant.ID, user.Username, user.Email, user.Role.Slug, user.DisplayName,
				)
			}
			if err != nil {
				t.Fatalf("generate token pair: %v", err)
			}

			_, err = svc.RefreshToken(pair.RefreshToken)
			requireUnauthorized(t, err)
		})
	}
}

func TestAuthService_RefreshToken_PlatformAdminBypassesMembershipOnly(t *testing.T) {
	t.Run("active tenant without membership", func(t *testing.T) {
		db := setupTestDB(t)
		ensureDefaultTestTenant(t, db)
		jwtMgr := newAuthServiceTestJWTManager()
		svc := NewAuthService(db, jwtMgr, auth.NewBlacklist(), nil)
		user := createTestUser(t, db, "refresh-platform-admin", "admin")
		tenant := createTestTenant(t, db, "Admin Target", "admin-target", models.TenantStatusActive)

		pair, err := jwtMgr.GenerateTokenPairWithTenant(
			user.ID, tenant.ID, user.Username, user.Email, user.Role.Slug, user.DisplayName,
		)
		if err != nil {
			t.Fatalf("GenerateTokenPairWithTenant: %v", err)
		}
		refreshed, err := svc.RefreshToken(pair.RefreshToken)
		if err != nil {
			t.Fatalf("RefreshToken: %v", err)
		}
		assertTokenPairTenant(t, jwtMgr, refreshed, tenant.ID)
	})

	t.Run("suspended tenant", func(t *testing.T) {
		db := setupTestDB(t)
		ensureDefaultTestTenant(t, db)
		jwtMgr := newAuthServiceTestJWTManager()
		svc := NewAuthService(db, jwtMgr, auth.NewBlacklist(), nil)
		user := createTestUser(t, db, "refresh-platform-admin-suspended", "admin")
		tenant := createTestTenant(t, db, "Suspended Admin Target", "suspended-admin-target", models.TenantStatusSuspended)

		pair, err := jwtMgr.GenerateTokenPairWithTenant(
			user.ID, tenant.ID, user.Username, user.Email, user.Role.Slug, user.DisplayName,
		)
		if err != nil {
			t.Fatalf("GenerateTokenPairWithTenant: %v", err)
		}
		_, err = svc.RefreshToken(pair.RefreshToken)
		requireUnauthorized(t, err)
	})
}

func TestAuthService_RefreshToken_LegacyTenantResolution(t *testing.T) {
	t.Run("chooses lowest active membership deterministically", func(t *testing.T) {
		db := setupTestDB(t)
		ensureDefaultTestTenant(t, db)
		jwtMgr := newAuthServiceTestJWTManager()
		svc := NewAuthService(db, jwtMgr, auth.NewBlacklist(), nil)
		user := createTestUser(t, db, "refresh-legacy", "editor")
		suspended := createTestTenant(t, db, "Legacy Suspended", "legacy-suspended", models.TenantStatusSuspended)
		activeFirst := createTestTenant(t, db, "Legacy Active First", "legacy-active-first", models.TenantStatusActive)
		activeSecond := createTestTenant(t, db, "Legacy Active Second", "legacy-active-second", models.TenantStatusActive)

		// Insert memberships in reverse order. Resolution must not depend on the
		// repository's row order, and it must skip the lower suspended tenant.
		createTestMembership(t, db, user.ID, activeSecond.ID)
		createTestMembership(t, db, user.ID, suspended.ID)
		createTestMembership(t, db, user.ID, activeFirst.ID)

		pair, err := jwtMgr.GenerateTokenPair(
			user.ID, user.Username, user.Email, user.Role.Slug, user.DisplayName,
		)
		if err != nil {
			t.Fatalf("GenerateTokenPair: %v", err)
		}
		legacyClaims, err := jwtMgr.ValidateRefreshToken(pair.RefreshToken)
		if err != nil {
			t.Fatalf("ValidateRefreshToken: %v", err)
		}
		if legacyClaims.TenantID != 0 {
			t.Fatalf("legacy TenantID = %d, want 0", legacyClaims.TenantID)
		}

		refreshed, err := svc.RefreshToken(pair.RefreshToken)
		if err != nil {
			t.Fatalf("RefreshToken: %v", err)
		}
		assertTokenPairTenant(t, jwtMgr, refreshed, activeFirst.ID)
	})

	t.Run("rejects non-admin without membership", func(t *testing.T) {
		db := setupTestDB(t)
		ensureDefaultTestTenant(t, db)
		jwtMgr := newAuthServiceTestJWTManager()
		svc := NewAuthService(db, jwtMgr, auth.NewBlacklist(), nil)
		user := createTestUser(t, db, "refresh-legacy-no-membership", "editor")

		pair, err := jwtMgr.GenerateTokenPair(
			user.ID, user.Username, user.Email, user.Role.Slug, user.DisplayName,
		)
		if err != nil {
			t.Fatalf("GenerateTokenPair: %v", err)
		}
		_, err = svc.RefreshToken(pair.RefreshToken)
		requireUnauthorized(t, err)
	})

	t.Run("platform admin uses active default tenant", func(t *testing.T) {
		db := setupTestDB(t)
		ensureDefaultTestTenant(t, db)
		jwtMgr := newAuthServiceTestJWTManager()
		svc := NewAuthService(db, jwtMgr, auth.NewBlacklist(), nil)
		user := createTestUser(t, db, "refresh-legacy-admin", "admin")

		pair, err := jwtMgr.GenerateTokenPair(
			user.ID, user.Username, user.Email, user.Role.Slug, user.DisplayName,
		)
		if err != nil {
			t.Fatalf("GenerateTokenPair: %v", err)
		}
		refreshed, err := svc.RefreshToken(pair.RefreshToken)
		if err != nil {
			t.Fatalf("RefreshToken: %v", err)
		}
		assertTokenPairTenant(t, jwtMgr, refreshed, models.DefaultTenantID)
	})
}

func TestAuthService_ChangePassword(t *testing.T) {
	db := setupTestDB(t)
	jwtMgr := auth.NewJWTManager(config.JWTConfig{
		Secret: "test-secret", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 7 * 24 * time.Hour, Issuer: "test",
	})
	blacklist := auth.NewBlacklist()
	svc := NewAuthService(db, jwtMgr, blacklist, nil)

	user := createTestUser(t, db, "pwuser", "subscriber")

	err := svc.ChangePassword(user.ID, "TestPass1", "NewPass123!")
	if err != nil {
		t.Fatalf("ChangePassword() error: %v", err)
	}

	// Old password should no longer work.
	_, _, loginErr := svc.Login("pwuser", "TestPass1", "127.0.0.1", "test-agent")
	if loginErr == nil {
		t.Error("Login with old password should fail")
	}

	// New password should work.
	_, _, loginErr = svc.Login("pwuser", "NewPass123!", "127.0.0.1", "test-agent")
	if loginErr != nil {
		t.Error("Login with new password should succeed")
	}
}

func TestAuthService_Me(t *testing.T) {
	db := setupTestDB(t)
	jwtMgr := auth.NewJWTManager(config.JWTConfig{
		Secret: "test-secret", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 7 * 24 * time.Hour, Issuer: "test",
	})
	blacklist := auth.NewBlacklist()
	svc := NewAuthService(db, jwtMgr, blacklist, nil)

	user := createTestUser(t, db, "meuser", "admin")

	safeUser, perms, err := svc.Me(user.ID)
	if err != nil {
		t.Fatalf("Me() error: %v", err)
	}

	if safeUser.Username != "meuser" {
		t.Errorf("Username = %q, want %q", safeUser.Username, "meuser")
	}
	if len(perms) == 0 {
		t.Error("Admin should have permissions")
	}
}

func TestSanitizeUser(t *testing.T) {
	// SanitizeUser is tested implicitly through Login/Register tests above.
	// It requires a full models.User with Role preloaded.
}

func newAuthServiceTestJWTManager() *auth.JWTManager {
	return auth.NewJWTManager(config.JWTConfig{
		Secret:          "test-secret-key-for-refresh-tenant-tests",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          "test",
	})
}

func createTestTenant(t *testing.T, db *gorm.DB, name, slug, status string) *models.Tenant {
	t.Helper()
	tenant := &models.Tenant{Name: name, Slug: slug, Status: status}
	if err := db.Create(tenant).Error; err != nil {
		t.Fatalf("create tenant %q: %v", slug, err)
	}
	return tenant
}

func ensureDefaultTestTenant(t *testing.T, db *gorm.DB) *models.Tenant {
	t.Helper()
	tenant := &models.Tenant{
		BaseModel: models.BaseModel{ID: models.DefaultTenantID},
		Name:      "Default",
		Slug:      "default",
		Status:    models.TenantStatusActive,
	}
	if err := db.FirstOrCreate(tenant, models.DefaultTenantID).Error; err != nil {
		t.Fatalf("ensure default tenant: %v", err)
	}
	return tenant
}

func createTestMembership(t *testing.T, db *gorm.DB, userID, tenantID uint) *models.TenantMembership {
	t.Helper()
	membership := &models.TenantMembership{
		TenantID: tenantID,
		UserID:   userID,
		RoleSlug: models.TenantRoleMember,
	}
	if err := db.Create(membership).Error; err != nil {
		t.Fatalf("create membership user=%d tenant=%d: %v", userID, tenantID, err)
	}
	return membership
}

func assertTokenPairTenant(t *testing.T, jwtMgr *auth.JWTManager, pair *auth.TokenPair, want uint) {
	t.Helper()
	for name, token := range map[string]string{
		"access":  pair.AccessToken,
		"refresh": pair.RefreshToken,
	} {
		claims, err := jwtMgr.ValidateToken(token)
		if err != nil {
			t.Fatalf("ValidateToken(%s): %v", name, err)
		}
		if claims.TenantID != want {
			t.Errorf("%s TenantID = %d, want %d", name, claims.TenantID, want)
		}
	}
}

func requireUnauthorized(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
	appErr, ok := err.(*errs.AppError)
	if !ok || appErr.Code != "UNAUTHORIZED" {
		t.Fatalf("expected UNAUTHORIZED, got %v", err)
	}
}
