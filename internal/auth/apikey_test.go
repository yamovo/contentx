package auth

import (
	"strings"
	"testing"
	"time"
)

func TestCreateAPIKey(t *testing.T) {
	key, raw, err := CreateAPIKey(42, "ci", "articles.read", nil)
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if !strings.HasPrefix(raw, "vx_") {
		t.Fatalf("raw key should start with vx_, got %q", raw)
	}
	if key.UserID != 42 {
		t.Fatalf("expected userID 42, got %d", key.UserID)
	}
	if key.KeyHash != HashAPIKey(raw) {
		t.Fatal("stored hash must match HashAPIKey(raw)")
	}
	if key.KeyHash == raw {
		t.Fatal("must store hash, not the raw key")
	}
	if key.KeyPrefix != raw[:10] {
		t.Fatalf("prefix mismatch: %q vs %q", key.KeyPrefix, raw[:10])
	}
}

func TestAPIKey_IsExpired(t *testing.T) {
	k := &APIKey{}
	if k.IsExpired() {
		t.Fatal("nil ExpiresAt should never expire")
	}
	past := time.Now().Add(-time.Hour)
	k.ExpiresAt = &past
	if !k.IsExpired() {
		t.Fatal("past ExpiresAt should be expired")
	}
	future := time.Now().Add(time.Hour)
	k.ExpiresAt = &future
	if k.IsExpired() {
		t.Fatal("future ExpiresAt should not be expired")
	}
}

func TestAPIKey_HasScope(t *testing.T) {
	full := &APIKey{Scopes: ""}
	if !full.HasScope("anything") {
		t.Fatal("empty scopes should grant full access")
	}
	wild := &APIKey{Scopes: "*"}
	if !wild.HasScope("articles.read") {
		t.Fatal("wildcard should grant all")
	}
	scoped := &APIKey{Scopes: "articles.read, categories.read"}
	if !scoped.HasScope("articles.read") {
		t.Fatal("should have articles.read")
	}
	if !scoped.HasScope("categories.read") {
		t.Fatal("should have categories.read (space-trimmed)")
	}
	if scoped.HasScope("users.write") {
		t.Fatal("should not have users.write")
	}
}

func TestHashAPIKey_Deterministic(t *testing.T) {
	h1 := HashAPIKey("vx_abc")
	h2 := HashAPIKey("vx_abc")
	if h1 != h2 {
		t.Fatal("hash must be deterministic")
	}
	if HashAPIKey("a") == HashAPIKey("b") {
		t.Fatal("different inputs must hash differently")
	}
}
