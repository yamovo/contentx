package services

import (
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database with all migrations applied.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Seed minimal data: roles and permissions.
	database.Seed(db)

	return db
}

// createTestUser creates a user with the given role slug for testing.
func createTestUser(t *testing.T, db *gorm.DB, username, roleSlug string) *models.User {
	t.Helper()

	var role models.Role
	if err := db.Where("slug = ?", roleSlug).First(&role).Error; err != nil {
		t.Fatalf("role %q not found: %v", roleSlug, err)
	}

	hash, err := auth.HashPassword("TestPass1")
	if err != nil {
		t.Fatalf("failed to hash test password: %v", err)
	}

	user := models.User{
		Username:    username,
		Email:       username + "@test.com",
		Password:    hash,
		DisplayName: username,
		RoleID:      role.ID,
		Status:      models.UserStatusActive,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	db.Preload("Role").First(&user, user.ID)
	return &user
}

// createTestArticle creates a published article with initial revision for testing.
func createTestArticle(t *testing.T, db *gorm.DB, authorID uint, title string) *models.Article {
	t.Helper()

	now := time.Now()
	article := models.Article{
		Title:       title,
		Slug:        title,
		Content:     "<p>Test content for " + title + "</p>",
		AuthorID:    authorID,
		Status:      models.StatusPublished,
		PublishedAt: &now,
	}
	if err := db.Create(&article).Error; err != nil {
		t.Fatalf("failed to create test article: %v", err)
	}

	// Create initial revision (mirrors what the service does).
	revision := models.Revision{
		ArticleID: article.ID,
		Title:     article.Title,
		Content:   article.Content,
		EditorID:  authorID,
		Version:   1,
		Note:      "Initial version",
	}
	db.Create(&revision)

	return &article
}
