package auth

import (
	"strings"
	"testing"
)

func TestHashAndCheckPassword(t *testing.T) {
	pw := "Str0ngPass"
	hash, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == pw {
		t.Fatal("hash should not equal plaintext")
	}
	if err := CheckPassword(hash, pw); err != nil {
		t.Fatalf("CheckPassword valid: %v", err)
	}
	if err := CheckPassword(hash, "Wr0ngPass"); err != ErrPasswordMismatch {
		t.Fatalf("expected ErrPasswordMismatch, got %v", err)
	}
}

func TestHashPassword_RejectsWeak(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("expected error for weak password")
	}
}

func TestValidatePasswordStrength(t *testing.T) {
	cases := []struct {
		name string
		pw   string
		want error
	}{
		{"too short", "Ab1", ErrPasswordTooShort},
		{"too long", strings.Repeat("Aa1", 30), ErrPasswordTooLong},
		{"no upper", "lowercase1", ErrPasswordTooWeak},
		{"no lower", "UPPERCASE1", ErrPasswordTooWeak},
		{"no digit", "NoDigitsHere", ErrPasswordTooWeak},
		{"valid", "Str0ngPass", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidatePasswordStrength(tc.pw); got != tc.want {
				t.Fatalf("want %v, got %v", tc.want, got)
			}
		})
	}
}

func TestPasswordStrengthScore(t *testing.T) {
	weak := PasswordStrengthScore("abc")
	strong := PasswordStrengthScore("Str0ng!Password")
	if strong <= weak {
		t.Fatalf("expected strong score (%d) > weak score (%d)", strong, weak)
	}
	if strong > 5 {
		t.Fatalf("score should cap at 5, got %d", strong)
	}
}

func TestGenerateRandomToken(t *testing.T) {
	tok, err := GenerateRandomToken(16)
	if err != nil {
		t.Fatalf("GenerateRandomToken: %v", err)
	}
	if len(tok) != 32 { // hex of 16 bytes
		t.Fatalf("expected 32 hex chars, got %d", len(tok))
	}
	tok2, _ := GenerateRandomToken(16)
	if tok == tok2 {
		t.Fatal("tokens should be unique")
	}
}
