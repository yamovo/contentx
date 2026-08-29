package migrations

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	mysqlconfig "github.com/go-sql-driver/mysql"
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/services"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	settingsPostgresTestDSN = "CONTENTX_SETTINGS_POSTGRES_DSN"
	settingsMySQLTestDSN    = "CONTENTX_SETTINGS_MYSQL_DSN"
)

func TestSettingsIntegration_Postgres(t *testing.T) {
	dsn := os.Getenv(settingsPostgresTestDSN)
	if dsn == "" {
		t.Skip(settingsPostgresTestDSN + " not set; skipping live PostgreSQL Settings test")
	}
	runSettingsIntegration(t, openIsolatedPostgresSettingsDB(t, dsn), "postgres")
}

func TestSettingsIntegration_MySQL(t *testing.T) {
	dsn := os.Getenv(settingsMySQLTestDSN)
	if dsn == "" {
		t.Skip(settingsMySQLTestDSN + " not set; skipping live MySQL Settings test")
	}
	runSettingsIntegration(t, openIsolatedMySQLSettingsDB(t, dsn), "mysql")
}

func TestSettingsIntegration_PostgresDownWithoutTenantData(t *testing.T) {
	dsn := os.Getenv(settingsPostgresTestDSN)
	if dsn == "" {
		t.Skip(settingsPostgresTestDSN + " not set; skipping live PostgreSQL migration Down test")
	}
	db := openIsolatedPostgresSettingsDB(t, dsn)
	for version := 1; version <= 9; version++ {
		if err := findMigration(t, version).Up(db); err != nil {
			t.Fatalf("PostgreSQL migration %03d Up(): %v", version, err)
		}
	}
	if err := findMigration(t, 9).Down(db); err != nil {
		t.Fatalf("PostgreSQL migration 009 Down() without tenant data: %v", err)
	}
	if db.Migrator().HasColumn(&models.SiteSetting{}, "TenantID") {
		t.Fatal("PostgreSQL migration 009 Down() should remove site_settings.tenant_id")
	}
	if !db.Migrator().HasIndex("site_settings", "idx_site_settings_key") {
		t.Fatal("PostgreSQL migration 009 Down() should restore idx_site_settings_key")
	}
}

func runSettingsIntegration(t *testing.T, db *gorm.DB, dialect string) {
	t.Helper()
	migrator := database.NewMigrator(db)
	migrator.Register(All()...)
	if err := migrator.Up(); err != nil {
		t.Fatalf("%s migrations Up(): %v", dialect, err)
	}

	switch dialect {
	case "postgres":
		if !db.Migrator().HasIndex("site_settings", siteSettingGlobalKeyIndex) {
			t.Fatalf("%s missing %s", dialect, siteSettingGlobalKeyIndex)
		}
	case "mysql":
		if !db.Migrator().HasColumn("site_settings", siteSettingScopeColumn) {
			t.Fatalf("%s missing generated %s", dialect, siteSettingScopeColumn)
		}
		if !db.Migrator().HasIndex("site_settings", siteSettingScopeKeyIndex) {
			t.Fatalf("%s missing %s", dialect, siteSettingScopeKeyIndex)
		}
	}
	if dialect == "mysql" {
		err := findMigration(t, 9).Down(db)
		if err == nil || !strings.Contains(err.Error(), "disabled on MySQL") {
			t.Fatalf("MySQL migration 009 Down error = %v, want fail-closed guidance", err)
		}
		if !db.Migrator().HasColumn(&models.SiteSetting{}, "TenantID") {
			t.Fatal("MySQL fail-closed rollback removed site_settings.tenant_id")
		}
	}

	for _, setting := range []models.SiteSetting{
		{Key: "integration_atomic_a", Value: "global-a", Type: "string", Group: "general", IsPublic: true},
		{Key: "integration_atomic_b", Value: "global-b", Type: "string", Group: "general"},
	} {
		if err := db.Create(&setting).Error; err != nil {
			t.Fatalf("%s create global %s: %v", dialect, setting.Key, err)
		}
	}
	duplicate := models.SiteSetting{Key: "integration_atomic_a", Value: "duplicate", Type: "string", Group: "general"}
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatalf("%s accepted a duplicate global key", dialect)
	}

	tenantID := uint(41)
	svc := services.NewSettingsService(db)
	if err := svc.Update(map[string]interface{}{
		"integration_atomic_a": "tenant-a",
		"integration_atomic_b": "tenant-b",
	}, tenantID); err != nil {
		t.Fatalf("%s create tenant overrides: %v", dialect, err)
	}
	assertSettingValue(t, svc, dialect, "integration_atomic_a", tenantID, "tenant-a")
	assertSettingValue(t, svc, dialect, "integration_atomic_a", tenantID+1, "global-a")

	settings, _, err := svc.List("general", tenantID)
	if err != nil {
		t.Fatalf("%s list effective settings: %v", dialect, err)
	}
	matches := 0
	for _, setting := range settings {
		if setting.Key == "integration_atomic_a" {
			matches++
			if setting.Value != "tenant-a" {
				t.Fatalf("%s effective list value = %q, want tenant-a", dialect, setting.Value)
			}
		}
	}
	if matches != 1 {
		t.Fatalf("%s effective list has %d integration_atomic_a rows, want 1", dialect, matches)
	}

	if err := addSettingsFailureConstraint(db, dialect); err != nil {
		t.Fatalf("%s add batch failure constraint: %v", dialect, err)
	}
	if err := svc.Update(map[string]interface{}{
		"integration_atomic_a": "changed-before-failure",
		"integration_atomic_b": "forbidden",
	}, tenantID); err == nil {
		t.Fatalf("%s batch update should fail", dialect)
	}
	assertSettingValue(t, svc, dialect, "integration_atomic_a", tenantID, "tenant-a")
	assertSettingValue(t, svc, dialect, "integration_atomic_b", tenantID, "tenant-b")

	if err := findMigration(t, 12).Down(db); err != nil {
		t.Fatalf("%s migration 012 Down(): %v", dialect, err)
	}
	if dialect == "mysql" {
		if db.Migrator().HasIndex("site_settings", siteSettingScopeKeyIndex) {
			t.Fatalf("MySQL migration 012 Down() left %s", siteSettingScopeKeyIndex)
		}
		if db.Migrator().HasColumn("site_settings", siteSettingScopeColumn) {
			t.Fatalf("MySQL migration 012 Down() left %s", siteSettingScopeColumn)
		}
	}
	err = findMigration(t, 9).Down(db)
	if err == nil || !strings.Contains(err.Error(), "scope would be lost") {
		t.Fatalf("%s migration 009 Down error = %v, want tenant-scope refusal", dialect, err)
	}
	if !db.Migrator().HasColumn(&models.SiteSetting{}, "TenantID") {
		t.Fatalf("%s unsafe rollback attempt removed site_settings.tenant_id", dialect)
	}
}

func assertSettingValue(t *testing.T, svc *services.SettingsService, dialect, key string, tenantID uint, want string) {
	t.Helper()
	setting, err := svc.Get(key, tenantID)
	if err != nil {
		t.Fatalf("%s get %s for tenant %d: %v", dialect, key, tenantID, err)
	}
	if setting.Value != want {
		t.Fatalf("%s %s for tenant %d = %q, want %q", dialect, key, tenantID, setting.Value, want)
	}
}

func addSettingsFailureConstraint(db *gorm.DB, dialect string) error {
	valueColumn := `"value"`
	if dialect == "mysql" {
		valueColumn = "`value`"
	}
	return db.Exec(
		"ALTER TABLE site_settings ADD CONSTRAINT chk_settings_batch_value CHECK (" + valueColumn + " <> 'forbidden')",
	).Error
}

func openIsolatedPostgresSettingsDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	admin := openPostgresSettingsDB(t, dsn)
	schema := settingsTestNamespace("schema")
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		closeGormDB(t, admin)
		t.Fatalf("create PostgreSQL test schema %s: %v", schema, err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Errorf("drop PostgreSQL test schema %s: %v", schema, err)
		}
		closeGormDB(t, admin)
	})

	isolatedDSN, err := postgresDSNWithSearchPath(dsn, schema)
	if err != nil {
		t.Fatalf("build PostgreSQL isolated DSN: %v", err)
	}
	db := openPostgresSettingsDB(t, isolatedDSN)
	t.Cleanup(func() {
		closeGormDB(t, db)
	})
	return db
}

func openIsolatedMySQLSettingsDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	admin := openMySQLSettingsDB(t, dsn)
	databaseName := settingsTestNamespace("db")
	if err := admin.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error; err != nil {
		closeGormDB(t, admin)
		t.Fatalf("create MySQL test database %s: %v", databaseName, err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP DATABASE `" + databaseName + "`").Error; err != nil {
			t.Errorf("drop MySQL test database %s: %v", databaseName, err)
		}
		closeGormDB(t, admin)
	})

	config, err := mysqlconfig.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse MySQL DSN: %v", err)
	}
	config.DBName = databaseName
	config.ParseTime = true
	db := openMySQLSettingsDB(t, config.FormatDSN())
	t.Cleanup(func() {
		closeGormDB(t, db)
	})
	return db
}

func openPostgresSettingsDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(dsn), settingsIntegrationGormConfig())
	if err != nil {
		t.Fatalf("open PostgreSQL Settings test database: %v", err)
	}
	return db
}

func openMySQLSettingsDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gormmysql.Open(dsn), settingsIntegrationGormConfig())
	if err != nil {
		t.Fatalf("open MySQL Settings test database: %v", err)
	}
	return db
}

func settingsIntegrationGormConfig() *gorm.Config {
	return &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	}
}

func postgresDSNWithSearchPath(dsn, schema string) (string, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			return "", err
		}
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		return parsed.String(), nil
	}
	return strings.TrimSpace(dsn) + " search_path=" + schema, nil
}

func settingsTestNamespace(kind string) string {
	return fmt.Sprintf("contentx_settings_%s_%d", kind, time.Now().UnixNano())
}

func closeGormDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Errorf("get SQL database for cleanup: %v", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		t.Errorf("close SQL database: %v", err)
	}
}
