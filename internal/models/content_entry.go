package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// ContentEntry represents a single record of a content type.
type ContentEntry struct {
	ID            uint         `gorm:"primarykey" json:"id"`
	ContentTypeID uint         `gorm:"index;not null" json:"content_type_id"`
	ContentType   *ContentType `gorm:"foreignKey:ContentTypeID" json:"content_type,omitempty"`
	DocumentID    string       `gorm:"uniqueIndex;size:36;not null" json:"document_id"`      // UUID
	Status        string       `gorm:"size:20;not null;default:'draft';index" json:"status"` // draft, published
	Data          JSONMap      `gorm:"type:text" json:"data"`                                // field values as JSON
	CreatedByID   uint         `gorm:"index" json:"created_by_id"`
	UpdatedByID   uint         `gorm:"index" json:"updated_by_id"`
	PublishedAt   *time.Time   `json:"published_at"`
	// i18n: Locale is the BCP-47 language tag. TranslationGroupID links
	// translations of the same logical entry across locales.
	Locale             string    `gorm:"size:10;not null;default:'en';index" json:"locale"`
	TranslationGroupID *uint     `gorm:"index" json:"translation_group_id,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// EntryStatus constants.
const (
	EntryStatusDraft     = "draft"
	EntryStatusPublished = "published"
)

// JSONMap is a map[string]interface{} with GORM JSON serialization.
type JSONMap map[string]interface{}

// Scan implements sql.Scanner.
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONMap)
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		*j = make(JSONMap)
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// Value implements driver.Valuer.
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	b, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}
