package auth_test

import (
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/config"
)

func newTestJWTManager() *auth.JWTManager {
	return auth.NewJWTManager(config.JWTConfig{
		Secret:          "test-secret-key-for-unit-tests",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          "test-contentx",
	})
}

func TestGenerateAndValidateTokenPair(t *testing.T) {
	mgr := newTestJWTManager()

	pair, err := mgr.GenerateTokenPair(1, "testuser", "test@example.com", "admin", "Test User")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error: %v", err)
	}

	if pair.AccessToken == "" {
		t.Error("AccessToken should not be empty")
	}
	if pair.RefreshToken == "" {
		t.Error("RefreshToken should not be empty")
	}
	if pair.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want 'Bearer'", pair.TokenType)
	}
	if pair.ExpiresIn != 900 { // 15 minutes
		t.Errorf("ExpiresIn = %d, want 900", pair.ExpiresIn)
	}

	// Validate access token.
	claims, err := mgr.ValidateToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken() error: %v", err)
	}
	if claims.UserID != 1 {
		t.Errorf("UserID = %d, want 1", claims.UserID)
	}
	if claims.Username != "testuser" {
		t.Errorf("Username = %q, want 'testuser'", claims.Username)
	}
	if claims.RoleSlug != "admin" {
		t.Errorf("RoleSlug = %q, want 'admin'", claims.RoleSlug)
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	mgr := newTestJWTManager()

	_, err := mgr.ValidateToken("invalid.token.string")
	if err == nil {
		t.Error("ValidateToken() should fail for invalid token")
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	mgr1 := auth.NewJWTManager(config.JWTConfig{
		Secret:         "secret-1",
		AccessTokenTTL: 15 * time.Minute,
		Issuer:         "test",
	})
	mgr2 := auth.NewJWTManager(config.JWTConfig{
		Secret:         "secret-2",
		AccessTokenTTL: 15 * time.Minute,
		Issuer:         "test",
	})

	pair, _ := mgr1.GenerateTokenPair(1, "user", "e@e.com", "admin", "User")
	_, err := mgr2.ValidateToken(pair.AccessToken)
	if err == nil {
		t.Error("ValidateToken() should fail with wrong secret")
	}
}

func TestRefreshAccessToken(t *testing.T) {
	mgr := newTestJWTManager()

	pair, _ := mgr.GenerateTokenPair(1, "testuser", "test@example.com", "admin", "Test User")
	//nolint:staticcheck // SA1019: intentionally testing the deprecated JWT-layer method for backward compatibility; AuthService.RefreshToken is covered by integration tests.
	newPair, err := mgr.RefreshAccessToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshAccessToken() error: %v", err)
	}

	if newPair.AccessToken == "" {
		t.Error("New access token should not be empty")
	}
	if newPair.AccessToken == pair.AccessToken {
		t.Error("New access token should differ from original")
	}
}

func TestBlacklist(t *testing.T) {
	bl := auth.NewBlacklist()

	token := "some.jwt.token"
	if bl.IsRevoked(token) {
		t.Error("Token should not be revoked initially")
	}

	bl.Revoke(token, time.Now().Add(1*time.Hour))
	if !bl.IsRevoked(token) {
		t.Error("Token should be revoked after Revoke()")
	}
}

func TestBlacklist_Expiry(t *testing.T) {
	bl := auth.NewBlacklist()

	token := "expired.jwt.token"
	bl.Revoke(token, time.Now().Add(-1*time.Hour)) // Already expired.
	if bl.IsRevoked(token) {
		t.Error("Expired token should not be considered revoked")
	}
}
