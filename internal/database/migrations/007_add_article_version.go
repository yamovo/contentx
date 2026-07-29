package migrations

import (
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// 007 adds the optimistic-lock version to existing article tables. Fresh
// installations already receive the column through migration 001's AllModels
// snapshot, so both directions must be safe when the column already exists.
func init() {
	RegisterMigrations(database.Migration{
		Version:     7,
		Description: "Add article version for optimistic locking",
		Up: func(tx *gorm.DB) error {
			if !tx.Migrator().HasColumn(&models.Article{}, "Version") {
				if err := tx.Migrator().AddColumn(&models.Article{}, "Version"); err != nil {
					return err
				}
			}
			return tx.Table("articles").
				Where("version IS NULL OR version < ?", 1).
				UpdateColumn("version", 1).Error
		},
		Down: func(tx *gorm.DB) error {
			if !tx.Migrator().HasColumn(&models.Article{}, "Version") {
				return nil
			}
			return tx.Migrator().DropColumn(&models.Article{}, "Version")
		},
	})
}
