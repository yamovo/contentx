package migrations

import (
	"github.com/yamovo/contentx/internal/database"
	"gorm.io/gorm"
)

// 005 converges upgraded databases on the canonical permission registry and
// built-in role matrix. It is a forward-only data migration: legacy rows remain
// for one release so old permission slugs can still be recognized, while role
// grants and API-token permissions are copied to canonical slugs.
func init() {
	RegisterMigrations(database.Migration{
		Version:     5,
		Description: "Canonicalize permissions and repair built-in role grants",
		Up: func(tx *gorm.DB) error {
			return database.SyncCanonicalPermissionsAndRoles(tx)
		},
		Down: func(_ *gorm.DB) error {
			// Data canonicalization is intentionally not reversed. On a full
			// rollback migration 001 drops the affected tables.
			return nil
		},
	})
}
