package migrations

import (
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// 002 adds a composite index on activity_logs(entity, created_at) to speed up
// the admin activity-feed query that filters by entity type and orders by date.
//
// This is an example of an incremental migration: it modifies existing schema
// without recreating tables. It uses the GORM migrator's HasIndex/DropIndex so
// it stays portable across SQLite, PostgreSQL and MySQL — MySQL supports
// neither "CREATE INDEX IF NOT EXISTS" nor "DROP INDEX IF EXISTS".
const activityLogEntityCreatedIdx = "idx_activity_logs_entity_created"

func init() {
	RegisterMigrations(
		database.Migration{
			Version:     2,
			Description: "Add composite index on activity_logs(entity, created_at)",
			Up: func(tx *gorm.DB) error {
				if tx.Migrator().HasIndex(&models.ActivityLog{}, activityLogEntityCreatedIdx) {
					return nil
				}
				return tx.Exec(
					"CREATE INDEX " + activityLogEntityCreatedIdx + " ON activity_logs(entity, created_at)",
				).Error
			},
			Down: func(tx *gorm.DB) error {
				if !tx.Migrator().HasIndex(&models.ActivityLog{}, activityLogEntityCreatedIdx) {
					return nil
				}
				return tx.Migrator().DropIndex(&models.ActivityLog{}, activityLogEntityCreatedIdx)
			},
		},
	)
}
