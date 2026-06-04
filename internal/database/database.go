package database

import (
	"fmt"
	"log"
	"time"

	"github.com/vortexcms/go-cms/internal/config"
	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB holds the global database connection.
var DB *gorm.DB

// Connect initializes the database connection.
func Connect(cfg config.DatabaseConfig) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "postgres":
		dialector = postgres.Open(cfg.DSN())
	case "mysql":
		dialector = mysql.Open(cfg.DSN())
	case "sqlite":
		dialector = sqlite.Open(cfg.DSN())
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	logLevel := logger.Warn
	gormCfg := &gorm.Config{
		Logger:                                   logger.Default.LogMode(logLevel),
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
		PrepareStmt:                              true,
	}

	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Verify connection.
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	DB = db
	log.Printf("[DB] Connected to %s successfully", cfg.Driver)
	return db, nil
}

// AutoMigrate runs automatic schema migration for all models.
func AutoMigrate(db *gorm.DB) error {
	log.Println("[DB] Running auto-migration...")

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
		&models.ThemeConfig{},

		// Analytics
		&models.PageView{},
		&models.SitemapEntry{},

		// System
		&models.Notification{},
		&models.ActivityLog{},
	)
	if err != nil {
		return fmt.Errorf("auto-migration failed: %w", err)
	}

	log.Println("[DB] Auto-migration completed")
	return nil
}

// Seed populates the database with initial data.
func Seed(db *gorm.DB) error {
	log.Println("[DB] Seeding database...")
	return SeedAll(db)
}

// Health checks the database connection.
func Health(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// Stats returns database connection pool stats.
func Stats(db *gorm.DB) map[string]interface{} {
	sqlDB, err := db.DB()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	stats := sqlDB.Stats()
	return map[string]interface{}{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":              stats.InUse,
		"idle":                stats.Idle,
		"wait_count":          stats.WaitCount,
		"wait_duration":       stats.WaitDuration.String(),
		"max_idle_closed":     stats.MaxIdleClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
		"max_lifetime_closed": stats.MaxLifetimeClosed,
	}
}

// WithTransaction executes fn inside a database transaction.
func WithTransaction(db *gorm.DB, fn func(tx *gorm.DB) error) error {
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback().Error; rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}
	return nil
}

// CleanupPageViews removes old page views beyond retention period.
func CleanupPageViews(db *gorm.DB, retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result := db.Where("created_at < ?", cutoff).Delete(&models.PageView{})
	if result.Error != nil {
		return result.Error
	}
	log.Printf("[DB] Cleaned up %d old page views", result.RowsAffected)
	return nil
}
