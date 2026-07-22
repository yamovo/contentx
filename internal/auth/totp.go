package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

const (
	// TOTP digits and period.
	totpDigits = 6
	totpPeriod = 30 // seconds
	totpSkew   = 1  // allow 1 period skew
	issuerName = "ContentX"
)

// TOTPConfig holds 2FA configuration for a user.
type TOTPConfig struct {
	Secret      string `json:"-" gorm:"size:32"`
	Enabled     bool   `json:"enabled"`
	BackupCodes string `json:"-" gorm:"type:text"` // comma-separated hashed codes
}

// GenerateTOTPSecret creates a new TOTP secret.
func GenerateTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// GenerateTOTPURI returns the otpauth:// URI for QR code generation.
func GenerateTOTPURI(secret, account string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&digits=%d&period=%d",
		issuerName, account, secret, issuerName, totpDigits, totpPeriod)
}

// ValidateTOTP validates a TOTP code against a secret.
func ValidateTOTP(secret, code string) bool {
	// Try current time and +/- skew periods.
	for i := -totpSkew; i <= totpSkew; i++ {
		t := time.Now().Add(time.Duration(i) * totpPeriod * time.Second)
		if generateTOTP(secret, t) == code {
			return true
		}
	}
	return false
}

// generateTOTP generates a TOTP code for the given time.
func generateTOTP(secret string, t time.Time) string {
	// Decode base32 secret.
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}

	// Calculate time counter.
	counter := uint64(t.Unix()) / uint64(totpPeriod)

	// Convert counter to bytes.
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	// HMAC-SHA1.
	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)

	// Dynamic truncation.
	offset := sum[len(sum)-1] & 0x0F
	code := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7FFFFFFF

	// Format to digits.
	code = code % pow10(totpDigits)

	return fmt.Sprintf("%0*d", totpDigits, code)
}

func pow10(n int) uint32 {
	result := uint32(1)
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}

// GenerateBackupCodes creates N random backup codes.
func GenerateBackupCodes(count int) ([]string, error) {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		codes[i] = fmt.Sprintf("%08x", b)
	}
	return codes, nil
}

// HashBackupCode hashes a backup code for storage.
func HashBackupCode(code string) string {
	h := sha1.Sum([]byte(code))
	return fmt.Sprintf("%x", h)
}
