package database

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect establishes a database connection based on the configured driver.
func Connect(cfg config.DatabaseConfig) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "postgres":
		dsn := fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
			cfg.Host, cfg.User, cfg.Password, cfg.Name, cfg.Port, cfg.SSLMode, cfg.Timezone,
		)
		dialector = postgres.Open(dsn)
	case "mysql":
		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name, cfg.Charset, cfg.Timezone,
		)
		dialector = mysql.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(cfg.Name + "?_journal_mode=WAL&_busy_timeout=5000")
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", cfg.Driver, err)
	}

	// Connection pool settings.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	slog.Info("database connected", "driver", cfg.Driver, "host", cfg.Host)
	return db, nil
}

// AllModels returns the canonical list of all GORM-registered models in the
// order they should be created. This is shared between AutoMigrate (legacy
// path) and the versioned Migrator (001_initial_schema) so the two paths
// always produce identical schema.
func AllModels() []interface{} {
	return []interface{}{
		// Auth & Users
		&models.Permission{},
		&models.Role{},
		&models.User{},

		// Content
		&models.Category{},
		&models.Tag{},
		&models.Article{},
		&models.Comment{},
		&models.Revision{},
		&models.CustomField{},

		// Media
		&models.Media{},

		// Navigation
		&models.Menu{},
		&models.MenuItem{},

		// Settings & SEO
		&models.SiteSetting{},
		&models.SEOSetting{},
		&models.RedirectRule{},

		// Extensions
		&models.Plugin{},
		&models.ThemeConfig{},

		// Analytics
		&models.PageView{},
		&models.ActivityLog{},

		// API Tokens
		&models.APIToken{},

		// Content Types
		&models.ContentType{},
		&models.ContentField{},
		&models.ContentEntry{},

		// Webhooks
		&models.Webhook{},
		&models.WebhookLog{},
	}
}

// AutoMigrate runs GORM auto-migration for all models.
// This is the legacy migration path used by tests. For production, use
// RunMigrations() which goes through the versioned Migrator instead.
func AutoMigrate(db *gorm.DB) error {
	slog.Info("running auto-migration...")
	if err := db.AutoMigrate(AllModels()...); err != nil {
		return fmt.Errorf("auto-migration failed: %w", err)
	}
	slog.Info("auto-migration completed")
	return nil
}

// Seed populates the database with initial data.
func Seed(db *gorm.DB) error {
	slog.Info("seeding database...")
	if err := SeedAll(db); err != nil {
		return fmt.Errorf("seeding failed: %w", err)
	}
	return nil
}

// RunMigrations creates a Migrator, registers the given migrations, and runs
// Up. This is the production migration path — replaces the legacy AutoMigrate
// call in main.go. The caller is responsible for importing the migrations
// package (which registers migrations via init()) and passing migrations.All().
func RunMigrations(db *gorm.DB, migs []Migration) error {
	m := NewMigrator(db)
	m.Register(migs...)
	return m.Up()
}

// RollbackMigration rolls back the last N applied migrations.
func RollbackMigration(db *gorm.DB, migs []Migration, steps int) error {
	m := NewMigrator(db)
	m.Register(migs...)
	return m.Down(steps)
}

// MigrationStatuses returns the status of all registered migrations.
func MigrationStatuses(db *gorm.DB, migs []Migration) ([]MigrationStatus, error) {
	m := NewMigrator(db)
	m.Register(migs...)
	return m.Status()
}

// Cleanup removes old data (e.g., page views older than retention period).
func Cleanup(db *gorm.DB, retentionDays int) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result := db.Where("created_at < ?", cutoff).Delete(&models.PageView{})
	if result.Error != nil {
		slog.Error("cleanup failed", "error", result.Error)
	} else {
		slog.Info("cleanup completed", "deleted_page_views", result.RowsAffected)
	}
}

// WithTransaction executes fn inside a database transaction.
func WithTransaction(db *gorm.DB, fn func(tx *gorm.DB) error) error {
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}
