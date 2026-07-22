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
	// Seed roles.
	if err := seedRoles(db); err != nil {
		return err
	}

	// Seed permissions.
	if err := seedPermissions(db); err != nil {
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

func seedRoles(db *gorm.DB) error {
	var count int64
	db.Model(&models.Role{}).Count(&count)
	if count > 0 {
		return nil
	}

	roles := []models.Role{
		{Name: "admin", Slug: "admin", Description: "Administrator with full access", IsDefault: false},
		{Name: "editor", Slug: "editor", Description: "Can edit and publish content", IsDefault: false},
		{Name: "author", Slug: "author", Description: "Can create and manage own content", IsDefault: true},
		{Name: "subscriber", Slug: "subscriber", Description: "Can read and comment", IsDefault: false},
	}

	return db.Create(&roles).Error
}

func seedPermissions(db *gorm.DB) error {
	var count int64
	db.Model(&models.Permission{}).Count(&count)
	if count > 0 {
		return nil
	}

	permissions := []models.Permission{
		// Article permissions
		{Name: "articles.create", Slug: "articles.create", Module: "articles", Description: "Create articles"},
		{Name: "articles.read", Slug: "articles.read", Module: "articles", Description: "Read articles"},
		{Name: "articles.update", Slug: "articles.update", Module: "articles", Description: "Update articles"},
		{Name: "articles.delete", Slug: "articles.delete", Module: "articles", Description: "Delete articles"},
		{Name: "articles.publish", Slug: "articles.publish", Module: "articles", Description: "Publish articles"},

		// Comment permissions
		{Name: "comments.create", Slug: "comments.create", Module: "comments", Description: "Create comments"},
		{Name: "comments.read", Slug: "comments.read", Module: "comments", Description: "Read comments"},
		{Name: "comments.moderate", Slug: "comments.moderate", Module: "comments", Description: "Moderate comments"},
		{Name: "comments.delete", Slug: "comments.delete", Module: "comments", Description: "Delete comments"},

		// Media permissions
		{Name: "media.upload", Slug: "media.upload", Module: "media", Description: "Upload media"},
		{Name: "media.read", Slug: "media.read", Module: "media", Description: "View media"},
		{Name: "media.delete", Slug: "media.delete", Module: "media", Description: "Delete media"},

		// User permissions
		{Name: "users.create", Slug: "users.create", Module: "users", Description: "Create users"},
		{Name: "users.read", Slug: "users.read", Module: "users", Description: "View users"},
		{Name: "users.update", Slug: "users.update", Module: "users", Description: "Update users"},
		{Name: "users.delete", Slug: "users.delete", Module: "users", Description: "Delete users"},

		// Settings permissions
		{Name: "settings.read", Slug: "settings.read", Module: "settings", Description: "View settings"},
		{Name: "settings.update", Slug: "settings.update", Module: "settings", Description: "Update settings"},

		// Category permissions
		{Name: "categories.create", Slug: "categories.create", Module: "categories", Description: "Create categories"},
		{Name: "categories.read", Slug: "categories.read", Module: "categories", Description: "View categories"},
		{Name: "categories.update", Slug: "categories.update", Module: "categories", Description: "Update categories"},
		{Name: "categories.delete", Slug: "categories.delete", Module: "categories", Description: "Delete categories"},

		// Tag permissions
		{Name: "tags.create", Slug: "tags.create", Module: "tags", Description: "Create tags"},
		{Name: "tags.read", Slug: "tags.read", Module: "tags", Description: "View tags"},
		{Name: "tags.update", Slug: "tags.update", Module: "tags", Description: "Update tags"},
		{Name: "tags.delete", Slug: "tags.delete", Module: "tags", Description: "Delete tags"},

		// Menu permissions
		{Name: "menus.create", Slug: "menus.create", Module: "menus", Description: "Create menus"},
		{Name: "menus.read", Slug: "menus.read", Module: "menus", Description: "View menus"},
		{Name: "menus.update", Slug: "menus.update", Module: "menus", Description: "Update menus"},
		{Name: "menus.delete", Slug: "menus.delete", Module: "menus", Description: "Delete menus"},

		// Analytics permissions
		{Name: "analytics.read", Slug: "analytics.read", Module: "analytics", Description: "View analytics"},

		// SEO permissions
		{Name: "seo.read", Slug: "seo.read", Module: "seo", Description: "View SEO settings"},
		{Name: "seo.update", Slug: "seo.update", Module: "seo", Description: "Update SEO settings"},
	}

	if err := db.Create(&permissions).Error; err != nil {
		return err
	}

	// Assign all permissions to admin role.
	var adminRole models.Role
	if err := db.Where("name = ?", "admin").First(&adminRole).Error; err != nil {
		return err
	}

	var allPerms []models.Permission
	db.Find(&allPerms)
	if err := db.Model(&adminRole).Association("Permissions").Append(allPerms); err != nil {
		return err
	}

	// Assign content permissions to editor role.
	var editorRole models.Role
	if err := db.Where("name = ?", "editor").First(&editorRole).Error; err != nil {
		return err
	}

	editorPerms := []string{
		"articles.create", "articles.read", "articles.update", "articles.delete", "articles.publish",
		"comments.create", "comments.read", "comments.moderate", "comments.delete",
		"media.upload", "media.read", "media.delete",
		"categories.create", "categories.read", "categories.update", "categories.delete",
		"tags.create", "tags.read", "tags.update", "tags.delete",
	}

	var editorPermModels []models.Permission
	db.Where("name IN ?", editorPerms).Find(&editorPermModels)
	if err := db.Model(&editorRole).Association("Permissions").Append(editorPermModels); err != nil {
		return err
	}

	// Assign basic permissions to author role.
	var authorRole models.Role
	if err := db.Where("name = ?", "author").First(&authorRole).Error; err != nil {
		return err
	}

	authorPerms := []string{
		"articles.create", "articles.read", "articles.update",
		"comments.create", "comments.read",
		"media.upload", "media.read",
		"categories.read", "tags.read",
	}

	var authorPermModels []models.Permission
	db.Where("name IN ?", authorPerms).Find(&authorPermModels)
	if err := db.Model(&authorRole).Association("Permissions").Append(authorPermModels); err != nil {
		return err
	}

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
		{Key: "site_name", Value: "ContentX", Type: "string", Group: "general", IsPublic: true, SortOrder: 1},
		{Key: "site_description", Value: "A modern content management system", Type: "string", Group: "general", IsPublic: true, SortOrder: 2},
		{Key: "site_url", Value: "http://localhost:8080", Type: "string", Group: "general", IsPublic: true, SortOrder: 3},
		{Key: "site_logo", Value: "", Type: "string", Group: "general", IsPublic: true, SortOrder: 4},
		{Key: "site_favicon", Value: "", Type: "string", Group: "general", IsPublic: true, SortOrder: 5},
		{Key: "site_language", Value: "zh-CN", Type: "string", Group: "general", IsPublic: true, SortOrder: 6},
		{Key: "site_timezone", Value: "Asia/Shanghai", Type: "string", Group: "general", IsPublic: false, SortOrder: 7},
		{Key: "posts_per_page", Value: "10", Type: "int", Group: "reading", IsPublic: true, SortOrder: 1},
		{Key: "default_category", Value: "1", Type: "int", Group: "writing", IsPublic: false, SortOrder: 1},
		{Key: "enable_comments", Value: "true", Type: "bool", Group: "discussion", IsPublic: true, SortOrder: 1},
		{Key: "moderate_comments", Value: "true", Type: "bool", Group: "discussion", IsPublic: false, SortOrder: 2},
		{Key: "allow_registration", Value: "true", Type: "bool", Group: "users", IsPublic: true, SortOrder: 1},
		{Key: "default_role", Value: "subscriber", Type: "string", Group: "users", IsPublic: false, SortOrder: 2},
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
