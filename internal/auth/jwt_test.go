package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/config"
)

func testJWTManager(accessTTL time.Duration) *JWTManager {
	return NewJWTManager(config.JWTConfig{
		Secret:          "test-secret-key-at-least-16-chars",
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: time.Hour,
		Issuer:          "contentx-test",
	})
}

func TestGenerateAndValidateToken(t *testing.T) {
	m := testJWTManager(time.Hour)
	pair, err := m.GenerateTokenPair(7, "alice", "a@x.com", "admin", "Alice")
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("tokens must not be empty")
	}
	if pair.TokenType != "Bearer" {
		t.Fatalf("expected Bearer, got %q", pair.TokenType)
	}

	claims, err := m.ValidateToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.UserID != 7 || claims.Username != "alice" || claims.RoleSlug != "admin" {
		t.Fatalf("claims mismatch: %+v", claims)
	}
	if claims.TokenUse != TokenUseAccess || claims.ID == "" {
		t.Fatalf("access token missing token_use/JTI: %+v", claims)
	}
	refreshClaims, err := m.ValidateRefreshToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if refreshClaims.TokenUse != TokenUseRefresh || refreshClaims.ID == "" || refreshClaims.ID == claims.ID {
		t.Fatalf("refresh token missing distinct token_use/JTI: %+v", refreshClaims)
	}
	if refreshClaims.UserID != claims.UserID || refreshClaims.Username != claims.Username || refreshClaims.RoleSlug != claims.RoleSlug {
		t.Fatalf("refresh token identity claims mismatch: access=%+v refresh=%+v", claims, refreshClaims)
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	m := testJWTManager(time.Hour)
	if _, err := m.ValidateToken("not-a-real-token"); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	m1 := testJWTManager(time.Hour)
	pair, _ := m1.GenerateTokenPair(1, "u", "e", "r", "d")

	m2 := NewJWTManager(config.JWTConfig{
		Secret:          "a-completely-different-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "x",
	})
	if _, err := m2.ValidateToken(pair.AccessToken); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for wrong secret, got %v", err)
	}
}

func TestValidateToken_Expired(t *testing.T) {
	m := testJWTManager(-time.Minute) // issued already expired
	pair, _ := m.GenerateTokenPair(1, "u", "e", "r", "d")
	if _, err := m.ValidateToken(pair.AccessToken); err != ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestRefreshAccessToken(t *testing.T) {
	m := testJWTManager(time.Hour)
	pair, _ := m.GenerateTokenPair(9, "bob", "b@x.com", "editor", "Bob")
	newPair, err := m.RefreshAccessToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshAccessToken: %v", err)
	}
	claims, err := m.ValidateToken(newPair.AccessToken)
	if err != nil {
		t.Fatalf("validate refreshed token: %v", err)
	}
	if claims.UserID != 9 {
		t.Fatalf("expected userID 9, got %d", claims.UserID)
	}
}

func TestRefreshAccessToken_PreservesTenantID(t *testing.T) {
	m := testJWTManager(time.Hour)
	pair, err := m.GenerateTokenPairWithTenant(9, 42, "bob", "b@x.com", "editor", "Bob")
	if err != nil {
		t.Fatalf("GenerateTokenPairWithTenant: %v", err)
	}

	refreshClaims, err := m.ValidateRefreshToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if refreshClaims.TenantID != 42 {
		t.Fatalf("refresh claims TenantID = %d, want 42", refreshClaims.TenantID)
	}

	newPair, err := m.RefreshAccessToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshAccessToken: %v", err)
	}
	for name, token := range map[string]string{
		"access":  newPair.AccessToken,
		"refresh": newPair.RefreshToken,
	} {
		claims, err := m.ValidateToken(token)
		if err != nil {
			t.Fatalf("ValidateToken(%s): %v", name, err)
		}
		if claims.TenantID != 42 {
			t.Errorf("%s claims TenantID = %d, want 42", name, claims.TenantID)
		}
	}
}

func TestRefreshAccessToken_RejectsAccessToken(t *testing.T) {
	m := testJWTManager(time.Hour)
	pair, _ := m.GenerateTokenPair(9, "bob", "b@x.com", "editor", "Bob")
	if _, err := m.RefreshAccessToken(pair.AccessToken); !errors.Is(err, ErrWrongTokenUse) {
		t.Fatalf("expected ErrWrongTokenUse, got %v", err)
	}
}

func TestValidateAccessToken_RejectsRefreshToken(t *testing.T) {
	m := testJWTManager(time.Hour)
	pair, _ := m.GenerateTokenPair(9, "bob", "b@x.com", "editor", "Bob")
	if _, err := m.ValidateAccessToken(pair.RefreshToken); err != ErrWrongTokenUse {
		t.Fatalf("expected ErrWrongTokenUse, got %v", err)
	}
}

func TestBlacklist_RevokeAndCheck(t *testing.T) {
	b := NewBlacklist()
	tok := "token-abc"
	if b.IsRevoked(tok) {
		t.Fatal("fresh token should not be revoked")
	}
	b.Revoke(tok, time.Now().Add(time.Hour))
	if !b.IsRevoked(tok) {
		t.Fatal("token should be revoked")
	}
}

func TestBlacklist_ExpiredRevocationCleared(t *testing.T) {
	b := NewBlacklist()
	tok := "token-exp"
	b.Revoke(tok, time.Now().Add(-time.Minute)) // already past
	if b.IsRevoked(tok) {
		t.Fatal("expired revocation should be treated as not revoked")
	}
}

func TestBlacklist_Cleanup(t *testing.T) {
	b := NewBlacklist()
	b.Revoke("live", time.Now().Add(time.Hour))
	b.Revoke("dead", time.Now().Add(-time.Hour))
	b.Cleanup()
	if !b.IsRevoked("live") {
		t.Fatal("live token should remain revoked")
	}
	if b.IsRevoked("dead") {
		t.Fatal("dead token should have been cleaned")
	}
}

func TestBlacklist_ConsumeIsAtomic(t *testing.T) {
	b := NewBlacklist()
	const callers = 32
	results := make(chan bool, callers)
	for i := 0; i < callers; i++ {
		go func() {
			ok, err := b.Consume("one-time-refresh", time.Now().Add(time.Hour))
			if err != nil {
				t.Errorf("Consume: %v", err)
			}
			results <- ok
		}()
	}
	successes := 0
	for i := 0; i < callers; i++ {
		if <-results {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one successful consume, got %d", successes)
	}
}

func TestGenerateTokenPairWithTenant_EmbedsTenantID(t *testing.T) {
	m := testJWTManager(time.Hour)

	pair, err := m.GenerateTokenPairWithTenant(1, 42, "u", "e@x.com", "admin", "Admin")
	if err != nil {
		t.Fatalf("GenerateTokenPairWithTenant: %v", err)
	}
	claims, err := m.ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if claims.TenantID != 42 {
		t.Errorf("claims.TenantID = %d, want 42", claims.TenantID)
	}
	refreshClaims, err := m.ValidateRefreshToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if refreshClaims.TenantID != 42 {
		t.Errorf("refresh claims.TenantID = %d, want 42", refreshClaims.TenantID)
	}

	// Legacy path stays at 0 (resolved to default tenant by middleware).
	pair2, err := m.GenerateTokenPair(1, "u", "e@x.com", "admin", "Admin")
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}
	claims2, err := m.ValidateAccessToken(pair2.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if claims2.TenantID != 0 {
		t.Errorf("legacy claims.TenantID = %d, want 0", claims2.TenantID)
	}
}
