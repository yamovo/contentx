package auth_test

import (
	"testing"

	"github.com/yamovo/contentx/internal/auth"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"valid password", "MyStr0ng!Pass", false},
		{"too short", "Ab1", true},
		{"no uppercase", "abcdefgh1", true},
		{"no lowercase", "ABCDEFGH1", true},
		{"no digit", "Abcdefghij", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := auth.HashPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && hash == "" {
				t.Error("HashPassword() returned empty hash")
			}
		})
	}
}

func TestCheckPassword(t *testing.T) {
	password := "MyStr0ng!Pass"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	// Correct password.
	if err := auth.CheckPassword(hash, password); err != nil {
		t.Errorf("CheckPassword() should succeed for correct password, got: %v", err)
	}

	// Wrong password.
	if err := auth.CheckPassword(hash, "WrongPass1"); err == nil {
		t.Error("CheckPassword() should fail for wrong password")
	}
}

func TestPasswordStrengthScore(t *testing.T) {
	tests := []struct {
		password string
		minScore int
	}{
		{"abc", 0},
		{"Abcdef1", 2},
		{"Abcdef1!", 4},
		{"MyStr0ng!Pass123", 5},
	}

	for _, tt := range tests {
		score := auth.PasswordStrengthScore(tt.password)
		if score < tt.minScore {
			t.Errorf("PasswordStrengthScore(%q) = %d, want >= %d", tt.password, score, tt.minScore)
		}
	}
}

func TestGenerateRandomToken(t *testing.T) {
	token1, err := auth.GenerateRandomToken(16)
	if err != nil {
		t.Fatalf("GenerateRandomToken() error: %v", err)
	}
	token2, err := auth.GenerateRandomToken(16)
	if err != nil {
		t.Fatalf("GenerateRandomToken() error: %v", err)
	}

	if token1 == token2 {
		t.Error("GenerateRandomToken() should generate unique tokens")
	}
	if len(token1) != 32 { // hex encoding doubles the length
		t.Errorf("GenerateRandomToken(16) length = %d, want 32", len(token1))
	}
}
