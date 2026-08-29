package migrations

import (
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// 013 adds the versioned audit envelope columns (RESEARCH-001 §4) to
// activity_logs: first-class request/trace/span correlation plus source,
// actor type, and outcome provenance. Legacy rows keep empty strings, which
// every reader must treat as "unknown provenance" rather than filtering out.
//
// The columns are additive and nullable-free (empty string default) so the
// upgrade is a plain ADD COLUMN on SQLite, PostgreSQL, and MySQL alike. The
// correlation indexes only serve new writes; building them is part of this
// migration so large existing tables are indexed exactly once.
func init() {
	RegisterMigrations(database.Migration{
		Version:     13,
		Description: "Add versioned audit envelope columns to activity_logs",
		Up: func(tx *gorm.DB) error {
			for _, column := range []string{
				"EventID", "RequestID", "TraceID", "SpanID", "Source", "ActorType", "Outcome",
			} {
				if !tx.Migrator().HasColumn(&models.ActivityLog{}, column) {
					if err := tx.Migrator().AddColumn(&models.ActivityLog{}, column); err != nil {
						return err
					}
				}
			}
			correlationIndexes := map[string]string{
				"idx_activity_logs_request_id": "request_id",
				"idx_activity_logs_trace_id":   "trace_id",
			}
			for index, column := range correlationIndexes {
				if !tx.Migrator().HasIndex(&models.ActivityLog{}, index) {
					if err := tx.Exec("CREATE INDEX " + index + " ON activity_logs (" + column + ")").Error; err != nil {
						return err
					}
				}
			}
			return nil
		},
		Down: func(tx *gorm.DB) error {
			// Audit provenance is deliberately not destroyed on downgrade:
			// dropping the columns would erase correlation evidence for events
			// written while this migration was active. The columns are inert
			// for older application versions, so Down is a no-op.
			return nil
		},
	})
}
