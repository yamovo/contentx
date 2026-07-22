package migrations

import (
	"github.com/yamovo/contentx/internal/database"
	"gorm.io/gorm"
)

func init() {
	RegisterMigrations(
		database.Migration{
			Version:     1,
			Description: "Create initial schema (all models through 2026-07)",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(database.AllModels()...)
			},
			Down: func(tx *gorm.DB) error {
				// Drop in reverse dependency order.
				models := database.AllModels()
				for i := len(models) - 1; i >= 0; i-- {
					if err := tx.Migrator().DropTable(models[i]); err != nil {
						return err
					}
				}
				// Also drop the join table for role_permissions (many2many).
				return tx.Migrator().DropTable("role_permissions")
			},
		},
	)
}
