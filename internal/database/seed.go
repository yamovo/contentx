package database

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SeedAll populates the database with initial data.
func SeedAll(db *gorm.DB) error {
	// Keep permissions and built-in roles converged on both new and existing
	// databases. Unlike the old count-based seed, this also repairs upgrades.
	if err := SyncCanonicalPermissionsAndRoles(db); err != nil {
		return err
	}

	// Seed admin user.
	if err := seedAdminUser(db); err != nil {
		return err
	}

	// Seed default settings.
	if err := seedSettings(db); err != nil {
		return err
	}

	// Seed default categories.
	if err := seedCategories(db); err != nil {
		return err
	}

	slog.Info("seeding completed")
	return nil
}

func seedAdminUser(db *gorm.DB) error {
	var count int64
	db.Model(&models.User{}).Count(&count)
	if count > 0 {
		return nil
	}

	// Get admin role.
	var adminRole models.Role
	if err := db.Where("name = ?", "admin").First(&adminRole).Error; err != nil {
		return err
	}

	// Get admin password from environment or generate random one.
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		if os.Getenv("SERVER_MODE") == "release" {
			return fmt.Errorf("ADMIN_PASSWORD must be set in production mode")
		}
		pw, pwErr := generateRandomPasswordSeed(16)
		if pwErr != nil {
			return pwErr
		}
		adminPassword = pw
		slog.Warn("no ADMIN_PASSWORD set — auto-generated one-time password", "password", adminPassword)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPassword), auth.BcryptCost)
	if err != nil {
		return err
	}

	admin := models.User{
		Username:    "admin",
		Email:       "admin@contentx.local",
		Password:    string(hashedPassword),
		DisplayName: "Administrator",
		Status:      models.UserStatusActive,
		RoleID:      adminRole.ID,
		Preferences: models.UserPreferences{
			Language:     "zh-CN",
			Theme:        "light",
			ItemsPerPage: 20,
		},
	}

	if err := db.Create(&admin).Error; err != nil {
		return err
	}
	// Multi-tenancy (RFC-001): the seeded admin belongs to the default tenant
	// as its admin. Kept in sync with migration 008's backfill so a fresh
	// database (migrations → seed) ends up with the same memberships as an
	// upgraded one.
	if err := db.Create(&models.TenantMembership{
		TenantID: models.DefaultTenantID,
		UserID:   admin.ID,
		RoleSlug: models.TenantRoleAdmin,
	}).Error; err != nil {
		return err
	}
	slog.Info("created admin user")
	return nil
}

// generateRandomPasswordSeed creates a cryptographically secure random password.
func generateRandomPasswordSeed(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		slog.Error("failed to generate random password", "error", err)
		return "", fmt.Errorf("failed to generate random password: %w", err)
	}
	return hex.EncodeToString(bytes)[:length], nil
}

func seedSettings(db *gorm.DB) error {
	var count int64
	db.Model(&models.SiteSetting{}).Count(&count)
	if count > 0 {
		return nil
	}

	settings := []models.SiteSetting{
		{Key: "site_name", Value: "ContentX", Type: "string", Group: "general", Label: "站点名称", IsPublic: true, SortOrder: 1},
		{Key: "site_description", Value: "A modern content management system", Type: "string", Group: "general", Label: "站点描述", IsPublic: true, SortOrder: 2},
		{Key: "site_url", Value: "http://localhost:8080", Type: "string", Group: "general", Label: "站点地址", IsPublic: true, SortOrder: 3},
		{Key: "site_logo", Value: "", Type: "string", Group: "general", Label: "站点 Logo", IsPublic: true, SortOrder: 4},
		{Key: "site_favicon", Value: "", Type: "string", Group: "general", Label: "站点图标", IsPublic: true, SortOrder: 5},
		{Key: "site_language", Value: "zh-CN", Type: "string", Group: "general", Label: "站点语言", IsPublic: true, SortOrder: 6},
		{Key: "site_timezone", Value: "Asia/Shanghai", Type: "string", Group: "general", Label: "时区", IsPublic: false, SortOrder: 7},
		{Key: "posts_per_page", Value: "10", Type: "int", Group: "reading", Label: "每页文章数", IsPublic: true, SortOrder: 1},
		{Key: "default_category", Value: "1", Type: "int", Group: "writing", Label: "默认分类", IsPublic: false, SortOrder: 1},
		{Key: "enable_comments", Value: "true", Type: "bool", Group: "discussion", Label: "开启评论", IsPublic: true, SortOrder: 1},
		{Key: "moderate_comments", Value: "true", Type: "bool", Group: "discussion", Label: "评论先审后发", IsPublic: false, SortOrder: 2},
		{Key: "allow_registration", Value: "true", Type: "bool", Group: "users", Label: "允许注册", IsPublic: true, SortOrder: 1},
		{Key: "default_role", Value: "subscriber", Type: "string", Group: "users", Label: "新用户默认角色", IsPublic: false, SortOrder: 2},
	}

	return db.Create(&settings).Error
}

func seedCategories(db *gorm.DB) error {
	var count int64
	db.Model(&models.Category{}).Count(&count)
	if count > 0 {
		return nil
	}

	categories := []models.Category{
		{Name: "Uncategorized", Slug: "uncategorized", Description: "Default category", SortOrder: 0, IsActive: true},
		{Name: "Technology", Slug: "technology", Description: "Technology related posts", SortOrder: 1, IsActive: true},
		{Name: "News", Slug: "news", Description: "Latest news and updates", SortOrder: 2, IsActive: true},
		{Name: "Tutorials", Slug: "tutorials", Description: "How-to guides and tutorials", SortOrder: 3, IsActive: true},
	}

	return db.Create(&categories).Error
}
