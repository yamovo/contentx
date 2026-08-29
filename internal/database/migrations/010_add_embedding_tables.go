package migrations

import (
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// 010 adds the document_embeddings table for RAG / vector search. Fresh
// installations already receive the table through migration 001's AllModels
// snapshot, so Up must be idempotent when the table already exists.
func init() {
	RegisterMigrations(database.Migration{
		Version:     10,
		Description: "Add document_embeddings table for RAG / vector search",
		Up: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&models.DocumentEmbedding{})
		},
		Down: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(&models.DocumentEmbedding{})
		},
	})
}
