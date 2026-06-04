package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost is the cost factor for bcrypt hashing.
	BcryptCost = 12
	// MinPasswordLength is the minimum password length.
	MinPasswordLength = 8
	// MaxPasswordLength is the maximum password length (bcrypt limit).
	MaxPasswordLength = 72
)

var (
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong  = errors.New("password must be no more than 72 characters")
	ErrPasswordTooWeak  = errors.New("password must contain at least one uppercase, one lowercase, and one digit")
	ErrPasswordMismatch = errors.New("password does not match")
)

var (
	hasUpper = regexp.MustCompile(`[A-Z]`)
	hasLower = regexp.MustCompile(`[a-z]`)
	hasDigit = regexp.MustCompile(`[0-9]`)
)

// HashPassword hashes a plaintext password using bcrypt.
func HashPassword(password string) (string, error) {
	if err := ValidatePasswordStrength(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword verifies a password against a bcrypt hash.
func CheckPassword(hashedPassword, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		return ErrPasswordMismatch
	}
	return nil
}

// ValidatePasswordStrength checks if a password meets complexity requirements.
func ValidatePasswordStrength(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if len(password) > MaxPasswordLength {
		return ErrPasswordTooLong
	}

	var hasUpperChar, hasLowerChar, hasDigitChar bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpperChar = true
		case unicode.IsLower(c):
			hasLowerChar = true
		case unicode.IsDigit(c):
			hasDigitChar = true
		}
	}

	if !hasUpperChar || !hasLowerChar || !hasDigitChar {
		return ErrPasswordTooWeak
	}

	return nil
}

// GenerateRandomToken generates a cryptographically secure random hex string.
func GenerateRandomToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateAPIKey generates a 32-byte random API key.
func GenerateAPIKey() (string, error) {
	return GenerateRandomToken(32)
}

// PasswordStrengthScore returns a score from 0-4 indicating password strength.
func PasswordStrengthScore(password string) int {
	score := 0

	if len(password) >= 8 {
		score++
	}
	if len(password) >= 12 {
		score++
	}
	if hasUpper.MatchString(password) && hasLower.MatchString(password) {
		score++
	}
	if hasDigit.MatchString(password) {
		score++
	}
	if regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password) {
		score++
	}

	return score
}
