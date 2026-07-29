package migrations

import (
	"testing"

	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newMigrationTestDB opens an empty in-memory SQLite database. Schema is
// expected to come exclusively from the registered migrations.
func newMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return db
}

// findMigration returns the registered migration with the given version.
func findMigration(t *testing.T, version int) database.Migration {
	t.Helper()
	for _, mig := range All() {
		if mig.Version == version {
			return mig
		}
	}
	t.Fatalf("migration version %d not registered", version)
	return database.Migration{}
}

func TestAll_VersionsSequentialFromOne(t *testing.T) {
	migs := All()
	if len(migs) == 0 {
		t.Fatal("no migrations registered")
	}
	for i, mig := range migs {
		want := i + 1
		if mig.Version != want {
			t.Errorf("migration at position %d has version %d, want %d (versions must be consecutive)", i, mig.Version, want)
		}
		if mig.Description == "" {
			t.Errorf("migration %d has empty description", mig.Version)
		}
		if mig.Up == nil || mig.Down == nil {
			t.Errorf("migration %d must define both Up and Down", mig.Version)
		}
	}
}

func TestRealMigrations_UpCreatesSchemaAndIndexes(t *testing.T) {
	db := newMigrationTestDB(t)
	m := database.NewMigrator(db)
	m.Register(All()...)

	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	// 001: core tables from AllModels.
	for _, table := range []string{"users", "articles", "activity_logs", "content_types", "webhooks"} {
		if !db.Migrator().HasTable(table) {
			t.Errorf("table %q should exist after Up()", table)
		}
	}
	// 002: composite index on activity_logs.
	if !db.Migrator().HasIndex(&models.ActivityLog{}, "idx_activity_logs_entity_created") {
		t.Error("idx_activity_logs_entity_created should exist after Up()")
	}
	// 003: composite list-sort index on articles.
	if !db.Migrator().HasIndex(&models.Article{}, "idx_articles_list_sort") {
		t.Error("idx_articles_list_sort should exist after Up()")
	}
}

func TestRealMigrations_DownRollsBackEverything(t *testing.T) {
	db := newMigrationTestDB(t)
	m := database.NewMigrator(db)
	m.Register(All()...)

	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}
	if err := m.Down(len(All())); err != nil {
		t.Fatalf("Down(%d) error: %v", len(All()), err)
	}

	for _, table := range []string{"users", "articles", "activity_logs"} {
		if db.Migrator().HasTable(table) {
			t.Errorf("table %q should be dropped after full rollback", table)
		}
	}

	// Status must show nothing applied.
	statuses, err := m.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	for _, s := range statuses {
		if s.Applied {
			t.Errorf("migration %d should not be applied after full rollback", s.Version)
		}
	}
}

func TestRealMigrations_ReplayAfterFullRollback(t *testing.T) {
	db := newMigrationTestDB(t)
	m := database.NewMigrator(db)
	m.Register(All()...)

	if err := m.Up(); err != nil {
		t.Fatalf("first Up() error: %v", err)
	}
	if err := m.Down(len(All())); err != nil {
		t.Fatalf("Down() error: %v", err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("replay Up() error: %v", err)
	}

	if !db.Migrator().HasTable("articles") {
		t.Error("articles table should exist after replay")
	}
	if !db.Migrator().HasIndex(&models.Article{}, "idx_articles_list_sort") {
		t.Error("idx_articles_list_sort should exist after replay")
	}
}

func TestIndexMigrations_UpIdempotent(t *testing.T) {
	db := newMigrationTestDB(t)
	m := database.NewMigrator(db)
	m.Register(All()...)
	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	// Running the index migrations again must be a no-op (HasIndex guard),
	// not a "index already exists" failure.
	for _, version := range []int{2, 3} {
		mig := findMigration(t, version)
		if err := mig.Up(db); err != nil {
			t.Errorf("migration %d Up() should be idempotent, got: %v", version, err)
		}
	}
}

func TestIndexMigrations_DownIdempotent(t *testing.T) {
	db := newMigrationTestDB(t)
	m := database.NewMigrator(db)
	m.Register(All()...)
	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	for _, version := range []int{2, 3} {
		mig := findMigration(t, version)
		if err := mig.Down(db); err != nil {
			t.Fatalf("migration %d Down() error: %v", version, err)
		}
		// Second Down must be a no-op thanks to the HasIndex guard.
		if err := mig.Down(db); err != nil {
			t.Errorf("migration %d Down() should be idempotent, got: %v", version, err)
		}
	}
}
