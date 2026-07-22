package auth

import (
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
