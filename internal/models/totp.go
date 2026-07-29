package models

import "time"

// UserTOTP stores per-user TOTP (two-factor authentication) state in a
// dedicated table so the users table stays untouched. Secret and backup-code
// hashes are never serialized to JSON.
type UserTOTP struct {
	ID     uint `gorm:"primarykey" json:"id"`
	UserID uint `gorm:"uniqueIndex;not null" json:"user_id"`
	// Secret is the base32-encoded TOTP seed. Present but Enabled=false while
	// the user is in the setup phase (scanned but not yet confirmed).
	Secret  string `gorm:"size:64;not null" json:"-"`
	Enabled bool   `gorm:"default:false" json:"enabled"`
	// BackupCodes holds comma-separated SHA-1 hashes of one-time recovery
	// codes; each is removed after use.
	BackupCodes string    `gorm:"type:text" json:"-"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
