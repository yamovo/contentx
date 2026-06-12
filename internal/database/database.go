package database

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/vortexcms/go-cms/internal/config"
	"github.com/vortexcms/go-cms/internal/models"
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
		Logger: logger.Default.LogMode(logger.Silent),
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

// AutoMigrate runs GORM auto-migration for all models.
// This is the legacy migration path. For production, use Migrate().
func AutoMigrate(db *gorm.DB) error {
	slog.Info("running auto-migration...")

	err := db.AutoMigrate(
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

		// Analytics
		&models.PageView{},
		&models.ActivityLog{},

		// API Tokens
		&models.APIToken{},
	)
	if err != nil {
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
