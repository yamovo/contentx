package migrations

import (
	"github.com/yamovo/contentx/internal/database"
	"gorm.io/gorm"
)

// 002 adds a composite index on activity_logs(entity, created_at) to speed up
// the admin activity-feed query that filters by entity type and orders by date.
//
// This is an example of an incremental migration: it modifies existing schema
// without recreating tables. GORM's Migrator().CreateIndex is idempotent —
// it will not fail if the index already exists.
func init() {
	RegisterMigrations(
		database.Migration{
			Version:     2,
			Description: "Add composite index on activity_logs(entity, created_at)",
			Up: func(tx *gorm.DB) error {
				return tx.Exec(
					"CREATE INDEX IF NOT EXISTS idx_activity_logs_entity_created ON activity_logs(entity, created_at)",
				).Error
			},
			Down: func(tx *gorm.DB) error {
				return tx.Exec("DROP INDEX IF EXISTS idx_activity_logs_entity_created").Error
			},
		},
	)
}
