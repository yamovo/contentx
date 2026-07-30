package backup

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/yamovo/contentx/internal/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPostgresDumpArgsArePortable(t *testing.T) {
	args := postgresDumpArgs(config.DatabaseConfig{
		Host: "db", Port: 5432, User: "contentx", Name: "contentx",
	}, "/backups/contentx.sql")

	for _, required := range []string{"--clean", "--if-exists", "--no-owner", "--no-privileges"} {
		if !slices.Contains(args, required) {
			t.Fatalf("pg_dump args missing %s: %v", required, args)
		}
	}
}

func TestPostgresRestoreArgsFailFastAndAreAtomic(t *testing.T) {
	args := postgresRestoreArgs(config.DatabaseConfig{
		Host: "db", Port: 5432, User: "contentx", Name: "contentx",
	}, "/backups/contentx.sql")

	for _, required := range []string{"ON_ERROR_STOP=1", "--single-transaction"} {
		if !slices.Contains(args, required) {
			t.Fatalf("psql args missing %s: %v", required, args)
		}
	}
}

func TestValidateSchemaRejectsUnidentifiedSQLBackupWhenTargetIsEmpty(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	mgr := NewManager(config.BackupConfig{}, config.DatabaseConfig{Driver: "postgres"}, "", db)
	path := filepath.Join(t.TempDir(), "unknown.sql")
	if err := os.WriteFile(path, []byte("CREATE TABLE items (id integer);\n"), 0o600); err != nil {
		t.Fatalf("write SQL: %v", err)
	}

	if err := mgr.validateSchema(path); !errors.Is(err, ErrSchemaMismatch) {
		t.Fatalf("expected ErrSchemaMismatch, got %v", err)
	}
}

func TestCanonicalModelTablesIncludeJoinAndLatestMigrationTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	mgr := NewManager(config.BackupConfig{}, config.DatabaseConfig{Driver: "postgres"}, "", db)
	tables, err := mgr.canonicalModelTables()
	if err != nil {
		t.Fatalf("canonicalModelTables: %v", err)
	}

	for _, required := range []string{
		"articles", "article_tags", "role_permissions", "schema_migrations",
		"webhook_deliveries",
	} {
		if required == "schema_migrations" {
			if slices.Contains(tables, required) {
				t.Fatalf("schema_migrations should be validated separately, got %v", tables)
			}
			continue
		}
		if !slices.Contains(tables, required) {
			t.Fatalf("canonical table set missing %s: %v", required, tables)
		}
	}
}
