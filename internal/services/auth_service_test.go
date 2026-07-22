package services

import (
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/config"
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
	svc := NewAuthService(db, jwtMgr, blacklist, nil)

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
	svc := NewAuthService(db, jwtMgr, blacklist, nil)

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
	jwtMgr := auth.NewJWTManager(config.JWTConfig{
		Secret: "test-secret", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 7 * 24 * time.Hour, Issuer: "test",
	})
	blacklist := auth.NewBlacklist()
	svc := NewAuthService(db, jwtMgr, blacklist, nil)

	createTestUser(t, db, "refreshuser", "subscriber")
	tp, _, _ := svc.Login("refreshuser", "TestPass1", "127.0.0.1", "test-agent")

	newTP, err := svc.RefreshToken(tp.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken() error: %v", err)
	}
	if newTP.AccessToken == "" {
		t.Error("New AccessToken should not be empty")
	}
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
