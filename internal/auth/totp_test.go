package auth

import (
	"strings"
	"testing"
	"time"
)

// ─── TOTP ────────────────────────────────────────────────────────────────────

func TestGenerateTOTPSecret(t *testing.T) {
	s1, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	if len(s1) == 0 {
		t.Fatal("secret should not be empty")
	}
	// base32 无 padding：只含 A-Z2-7。
	if strings.ContainsAny(s1, "=") {
		t.Errorf("secret should have no padding, got %q", s1)
	}
	s2, _ := GenerateTOTPSecret()
	if s1 == s2 {
		t.Error("two generated secrets should differ")
	}
}

func TestGenerateTOTPURI(t *testing.T) {
	uri := GenerateTOTPURI("SECRET234567", "alice@example.com")
	for _, want := range []string{
		"otpauth://totp/",
		"secret=SECRET234567",
		"issuer=ContentX",
		"digits=6",
		"period=30",
		"alice@example.com",
	} {
		if !strings.Contains(uri, want) {
			t.Errorf("URI %q missing %q", uri, want)
		}
	}
}

func TestValidateTOTP_CurrentCode(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	code := generateTOTP(secret, time.Now())
	if len(code) != 6 {
		t.Fatalf("code length = %d, want 6", len(code))
	}
	if !ValidateTOTP(secret, code) {
		t.Error("current code should validate")
	}
}

func TestValidateTOTP_SkewWindow(t *testing.T) {
	secret, _ := GenerateTOTPSecret()

	// 上一周期的 code 在 ±1 skew 内应通过。
	prev := generateTOTP(secret, time.Now().Add(-30*time.Second))
	if !ValidateTOTP(secret, prev) {
		t.Error("previous-period code should validate within skew")
	}

	// 远超 skew 的 code 应被拒。
	stale := generateTOTP(secret, time.Now().Add(-10*time.Minute))
	if ValidateTOTP(secret, stale) {
		t.Error("code from 10 minutes ago should not validate")
	}
}

func TestValidateTOTP_WrongCode(t *testing.T) {
	secret, _ := GenerateTOTPSecret()
	if ValidateTOTP(secret, "000000") && ValidateTOTP(secret, "999999") {
		t.Error("at least one of two fixed codes must fail")
	}
	if ValidateTOTP(secret, "not-a-code") {
		t.Error("non-numeric code should not validate")
	}
}

func TestGenerateTOTP_InvalidSecret(t *testing.T) {
	// 非法 base32 → 空串（ValidateTOTP 随之失败）。
	if got := generateTOTP("!!!invalid!!!", time.Now()); got != "" {
		t.Errorf("invalid secret should yield empty code, got %q", got)
	}
}

func TestGenerateBackupCodes(t *testing.T) {
	codes, err := GenerateBackupCodes(10)
	if err != nil {
		t.Fatalf("GenerateBackupCodes: %v", err)
	}
	if len(codes) != 10 {
		t.Fatalf("got %d codes, want 10", len(codes))
	}
	seen := make(map[string]bool)
	for _, c := range codes {
		if len(c) != 8 {
			t.Errorf("code %q length = %d, want 8", c, len(c))
		}
		if seen[c] {
			t.Errorf("duplicate backup code %q", c)
		}
		seen[c] = true
	}
}

func TestHashBackupCode(t *testing.T) {
	h1 := HashBackupCode("abcd1234")
	h2 := HashBackupCode("abcd1234")
	h3 := HashBackupCode("ffff0000")
	if h1 != h2 {
		t.Error("same code must hash identically")
	}
	if h1 == h3 {
		t.Error("different codes must hash differently")
	}
	if len(h1) != 40 { // SHA-1 hex
		t.Errorf("hash length = %d, want 40", len(h1))
	}
}
