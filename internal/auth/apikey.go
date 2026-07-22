package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// APIKey represents an API key for programmatic access.
type APIKey struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	UserID    uint       `json:"user_id" gorm:"index"`
	Name      string     `json:"name" gorm:"size:128"`
	KeyHash   string     `json:"-" gorm:"size:64;uniqueIndex"`
	KeyPrefix string     `json:"key_prefix" gorm:"size:8"`
	Scopes    string     `json:"scopes" gorm:"type:text"` // comma-separated scopes
	ExpiresAt *time.Time `json:"expires_at"`
	LastUsed  *time.Time `json:"last_used"`
	CreatedAt time.Time  `json:"created_at"`
}

// TableName returns the table name for APIKey.
func (APIKey) TableName() string {
	return "api_keys"
}

// GenerateAPIKey creates a new API key and returns the raw key + model.
// The raw key is shown once; only the hash is stored.
func CreateAPIKey(userID uint, name, scopes string, expiresAt *time.Time) (*APIKey, string, error) {
	// Generate 32 random bytes.
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, "", fmt.Errorf("failed to generate key: %w", err)
	}

	rawKey := "vx_" + hex.EncodeToString(b)
	hash := sha256.Sum256([]byte(rawKey))

	apiKey := &APIKey{
		UserID:    userID,
		Name:      name,
		KeyHash:   hex.EncodeToString(hash[:]),
		KeyPrefix: rawKey[:10],
		Scopes:    scopes,
		ExpiresAt: expiresAt,
	}

	return apiKey, rawKey, nil
}

// HashAPIKey returns the SHA-256 hash of a raw API key.
func HashAPIKey(rawKey string) string {
	hash := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(hash[:])
}

// IsExpired checks if the API key has expired.
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// HasScope checks if the API key has a specific scope.
func (k *APIKey) HasScope(scope string) bool {
	if k.Scopes == "" {
		return true // No scopes = full access
	}
	for _, s := range splitScopes(k.Scopes) {
		if s == scope || s == "*" {
			return true
		}
	}
	return false
}

func splitScopes(scopes string) []string {
	var result []string
	current := ""
	for _, c := range scopes {
		if c == ',' {
			if current != "" {
				result = append(result, current)
			}
			current = ""
		} else if c != ' ' {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
