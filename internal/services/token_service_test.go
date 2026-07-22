package services

import (
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/models"
)

// ─── TokenService Tests ─────────────────────────────────────────────────────

func TestTokenService_Create_Success(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "tokenuser", "admin")
	svc := NewTokenService(db)

	resp, err := svc.Create(CreateTokenRequest{
		Name:        "test-token",
		Permissions: []string{"articles.read", "articles.create"},
	}, user.ID)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	if resp.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if resp.Name != "test-token" {
		t.Fatalf("expected name 'test-token', got '%s'", resp.Name)
	}
	if resp.Token == "" {
		t.Fatal("expected non-empty token string")
	}
	if len(resp.Permissions) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(resp.Permissions))
	}
}

func TestTokenService_Create_WithExpiry(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "expiryuser", "admin")
	svc := NewTokenService(db)

	expiry := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	resp, err := svc.Create(CreateTokenRequest{
		Name:      "expiring",
		ExpiresAt: expiry,
	}, user.ID)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if resp.ExpiresAt == nil {
		t.Fatal("expected non-nil ExpiresAt")
	}
}

func TestTokenService_Create_InvalidExpiry(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "badexpiryuser", "admin")
	svc := NewTokenService(db)

	_, err := svc.Create(CreateTokenRequest{
		Name:      "bad",
		ExpiresAt: "not-a-date",
	}, user.ID)
	if err == nil {
		t.Fatal("expected error for invalid date format")
	}
}

func TestTokenService_List(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "listuser", "admin")
	svc := NewTokenService(db)

	svc.Create(CreateTokenRequest{Name: "token1"}, user.ID)
	svc.Create(CreateTokenRequest{Name: "token2"}, user.ID)

	tokens, err := svc.List()
	if err != nil {
		t.Fatalf("list tokens: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
}

func TestTokenService_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "deluser", "admin")
	svc := NewTokenService(db)

	resp, _ := svc.Create(CreateTokenRequest{Name: "to-delete"}, user.ID)
	if err := svc.Delete(resp.ID); err != nil {
		t.Fatalf("delete token: %v", err)
	}

	var count int64
	db.Model(&models.APIToken{}).Where("id = ?", resp.ID).Count(&count)
	if count != 0 {
		t.Fatal("token should be deleted")
	}
}

func TestTokenService_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewTokenService(db)

	err := svc.Delete(99999)
	if err == nil {
		t.Fatal("expected error for non-existent token")
	}
}

func TestTokenService_Validate_Success(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "validateuser", "admin")
	svc := NewTokenService(db)

	resp, _ := svc.Create(CreateTokenRequest{
		Name:        "valid-token",
		Permissions: []string{"articles.read"},
	}, user.ID)

	valid, userID, err := svc.Validate(resp.Token, "articles.read")
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if !valid {
		t.Fatal("expected token to be valid")
	}
	if userID != user.ID {
		t.Fatalf("expected user ID %d, got %d", user.ID, userID)
	}
}

func TestTokenService_Validate_InvalidToken(t *testing.T) {
	db := setupTestDB(t)
	svc := NewTokenService(db)

	valid, _, err := svc.Validate("vc_live_invalid", "articles.read")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if valid {
		t.Fatal("expected invalid")
	}
}

func TestTokenService_Validate_ExpiredToken(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "expireduser", "admin")
	svc := NewTokenService(db)

	past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	resp, _ := svc.Create(CreateTokenRequest{
		Name:      "expired",
		ExpiresAt: past,
	}, user.ID)

	valid, _, err := svc.Validate(resp.Token, "")
	if err == nil {
		t.Fatal("expected error for expired token")
	}
	if valid {
		t.Fatal("expected expired token to be invalid")
	}
}

func TestTokenService_Validate_WildcardPermission(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "wildcarduser", "admin")
	svc := NewTokenService(db)

	resp, _ := svc.Create(CreateTokenRequest{
		Name:        "wildcard",
		Permissions: []string{"*"},
	}, user.ID)

	valid, _, err := svc.Validate(resp.Token, "any.permission")
	if err != nil {
		t.Fatalf("validate wildcard: %v", err)
	}
	if !valid {
		t.Fatal("expected wildcard to match any permission")
	}
}

func TestTokenService_Validate_InsufficientPermission(t *testing.T) {
	db := setupTestDB(t)
	user := createTestUser(t, db, "insufficientuser", "admin")
	svc := NewTokenService(db)

	resp, _ := svc.Create(CreateTokenRequest{
		Name:        "limited",
		Permissions: []string{"articles.read"},
	}, user.ID)

	valid, _, err := svc.Validate(resp.Token, "articles.delete")
	if err == nil {
		t.Fatal("expected error for insufficient permission")
	}
	if valid {
		t.Fatal("expected invalid for insufficient permission")
	}
}
