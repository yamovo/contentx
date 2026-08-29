package handlers

import (
	"testing"

	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/database/migrations"
	"gorm.io/gorm"
)

// prepareHandlerTestDB keeps SQLite's in-memory schema on one connection and
// applies the same tenant-scoped indexes as the versioned production migrations.
func prepareHandlerTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get test database connection: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := migrations.ApplyTenantScopedIndexes(db); err != nil {
		t.Fatalf("apply tenant indexes: %v", err)
	}
}
