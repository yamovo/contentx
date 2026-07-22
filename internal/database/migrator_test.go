package database

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestDB creates an in-memory SQLite database for migrator tests.
// It does NOT run AutoMigrate — the migrator is responsible for schema.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return db
}

// testMigrations returns a small set of migrations for unit testing the
// Migrator itself (independent of the real 001/002 migrations).
func testMigrations() []Migration {
	return []Migration{
		{
			Version:     1,
			Description: "Create test_users table",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&models.User{})
			},
			Down: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable("users")
			},
		},
		{
			Version:     2,
			Description: "Create test_articles table",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(&models.Article{})
			},
			Down: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable("articles")
			},
		},
	}
}

func TestMigrator_Up_CreatesSchemaMigrationsTable(t *testing.T) {
	db := newTestDB(t)
	m := NewMigrator(db)

	// schema_migrations table should exist after NewMigrator.
	if !db.Migrator().HasTable("schema_migrations") {
		t.Error("schema_migrations table should exist after NewMigrator")
	}
	_ = m
}

func TestMigrator_Up_AppliesPendingMigrations(t *testing.T) {
	db := newTestDB(t)
	m := NewMigrator(db)
	m.Register(testMigrations()...)

	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	// Both tables should exist.
	if !db.Migrator().HasTable("users") {
		t.Error("users table should exist after Up()")
	}
	if !db.Migrator().HasTable("articles") {
		t.Error("articles table should exist after Up()")
	}
}

func TestMigrator_Up_Idempotent(t *testing.T) {
	db := newTestDB(t)
	m := NewMigrator(db)
	m.Register(testMigrations()...)

	// First run.
	if err := m.Up(); err != nil {
		t.Fatalf("first Up() error: %v", err)
	}
	// Second run should be a no-op (no error).
	if err := m.Up(); err != nil {
		t.Fatalf("second Up() error: %v", err)
	}
}

func TestMigrator_Status(t *testing.T) {
	db := newTestDB(t)
	m := NewMigrator(db)
	m.Register(testMigrations()...)

	// Before Up: none applied.
	statuses, err := m.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	for _, s := range statuses {
		if s.Applied {
			t.Errorf("version %d should not be applied before Up()", s.Version)
		}
	}

	// After Up: both applied.
	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}
	statuses, err = m.Status()
	if err != nil {
		t.Fatalf("Status() after Up() error: %v", err)
	}
	for _, s := range statuses {
		if !s.Applied {
			t.Errorf("version %d should be applied after Up()", s.Version)
		}
	}
}

func TestMigrator_Down_RollsBackMigrations(t *testing.T) {
	db := newTestDB(t)
	m := NewMigrator(db)
	m.Register(testMigrations()...)

	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	// Roll back 1 migration (version 2 — articles).
	if err := m.Down(1); err != nil {
		t.Fatalf("Down(1) error: %v", err)
	}

	// articles table should be gone, users should remain.
	if db.Migrator().HasTable("articles") {
		t.Error("articles table should be dropped after Down(1)")
	}
	if !db.Migrator().HasTable("users") {
		t.Error("users table should still exist after Down(1)")
	}

	// Status should show v1 applied, v2 not.
	statuses, _ := m.Status()
	if !statuses[0].Applied {
		t.Error("v1 should be applied")
	}
	if statuses[1].Applied {
		t.Error("v2 should not be applied after Down(1)")
	}
}

func TestMigrator_Down_All(t *testing.T) {
	db := newTestDB(t)
	m := NewMigrator(db)
	m.Register(testMigrations()...)

	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	// Roll back all migrations.
	if err := m.Down(2); err != nil {
		t.Fatalf("Down(2) error: %v", err)
	}

	if db.Migrator().HasTable("users") {
		t.Error("users table should be dropped after Down(2)")
	}
	if db.Migrator().HasTable("articles") {
		t.Error("articles table should be dropped after Down(2)")
	}
}

func TestMigrator_Up_ReplayAfterDown(t *testing.T) {
	db := newTestDB(t)
	m := NewMigrator(db)
	m.Register(testMigrations()...)

	// Up → Down → Up should work.
	if err := m.Up(); err != nil {
		t.Fatalf("first Up() error: %v", err)
	}
	if err := m.Down(2); err != nil {
		t.Fatalf("Down(2) error: %v", err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("second Up() error: %v", err)
	}

	if !db.Migrator().HasTable("users") {
		t.Error("users table should exist after replay")
	}
	if !db.Migrator().HasTable("articles") {
		t.Error("articles table should exist after replay")
	}
}

func TestRunMigrations_WithAllModels(t *testing.T) {
	db := newTestDB(t)

	// Use the real 001 migration (via AllModels) to verify it creates all
	// tables. We simulate the migrations package by constructing the same
	// migration inline.
	migs := []Migration{
		{
			Version:     1,
			Description: "Create initial schema",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(AllModels()...)
			},
			Down: func(tx *gorm.DB) error {
				models := AllModels()
				for i := len(models) - 1; i >= 0; i-- {
					tx.Migrator().DropTable(models[i])
				}
				return nil
			},
		},
	}

	if err := RunMigrations(db, migs); err != nil {
		t.Fatalf("RunMigrations() error: %v", err)
	}

	// Verify a sampling of tables exists.
	for _, table := range []string{"users", "articles", "tags", "api_tokens", "content_types", "webhooks"} {
		if !db.Migrator().HasTable(table) {
			t.Errorf("table %q should exist after RunMigrations", table)
		}
	}
}
