package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Float32Slice is a JSON-serialised []float32 suitable for GORM TEXT columns.
// It stores embedding vectors in databases that lack a native vector type
// (SQLite, MySQL). For PostgreSQL with pgvector, a future migration can add a
// native vector column and bypass this type.
type Float32Slice []float32

// Value implements driver.Valuer.
func (f Float32Slice) Value() (driver.Value, error) {
	if f == nil {
		return "[]", nil
	}
	b, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner.
func (f *Float32Slice) Scan(value interface{}) error {
	if value == nil {
		*f = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("failed to scan Float32Slice: %v", value)
	}
	return json.Unmarshal(bytes, f)
}

// DocumentEmbedding stores a text chunk and its embedding vector. A single
// article may produce multiple rows (one per chunk). All queries are scoped
// by tenant_id per RFC-001 §4.3.
type DocumentEmbedding struct {
	BaseModel
	TenantID   uint         `gorm:"index;not null;default:1" json:"tenant_id"`
	DocType    string       `gorm:"type:varchar(20);not null" json:"doc_type"` // "article" | "page" | "content_entry"
	DocID      uint         `gorm:"not null;index:idx_emb_doc" json:"doc_id"`  // article/entry ID
	ChunkIndex int          `gorm:"not null;default:0;index:idx_emb_doc" json:"chunk_index"`
	Content    string       `gorm:"type:text" json:"content"`       // the text that was embedded
	Embedding  Float32Slice `gorm:"type:text" json:"-"`             // JSON-serialised vector (never exposed via API)
	Model      string       `gorm:"type:varchar(100)" json:"model"` // embedding model name
	Locale     string       `gorm:"type:varchar(10)" json:"locale"`

	// Denormalised metadata to avoid joins at query time.
	Title  string `gorm:"type:varchar(500)" json:"title"`
	Slug   string `gorm:"type:varchar(500)" json:"slug"`
	Status string `gorm:"type:varchar(20)" json:"status"`
}

// TableName overrides the default table name.
func (DocumentEmbedding) TableName() string { return "document_embeddings" }
