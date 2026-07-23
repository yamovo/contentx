package repository

import (
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database with all migrations applied
// and minimal seed data (roles + permissions). Reused across all repository tests.
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
	database.Seed(db)
	return db
}

// createTestUser creates a user with the given role slug. The password is a
// dummy hash — repository tests don't exercise auth verification.
func createTestUser(t *testing.T, db *gorm.DB, username, roleSlug string) *models.User {
	t.Helper()
	var role models.Role
	if err := db.Where("slug = ?", roleSlug).First(&role).Error; err != nil {
		t.Fatalf("role %q not found: %v", roleSlug, err)
	}
	user := models.User{
		Username:    username,
		Email:       username + "@test.com",
		Password:    "$2a$10$dummypasswordhashnotforauth",
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

// createTestTag creates a tag with a unique slug.
func createTestTag(t *testing.T, db *gorm.DB, name, slug string) *models.Tag {
	t.Helper()
	tag := models.Tag{Name: name, Slug: slug}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatalf("failed to create test tag: %v", err)
	}
	return &tag
}

// createTestCategory creates a category.
func createTestCategory(t *testing.T, db *gorm.DB, name, slug string) *models.Category {
	t.Helper()
	cat := models.Category{Name: name, Slug: slug}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatalf("failed to create test category: %v", err)
	}
	return &cat
}

// createTestArticleDirect inserts an article directly via GORM (bypassing the
// repository Create method) for setup that doesn't depend on the method under test.
func createTestArticleDirect(t *testing.T, db *gorm.DB, authorID uint, title, slug string) *models.Article {
	t.Helper()
	now := time.Now()
	article := models.Article{
		Title:       title,
		Slug:        slug,
		Content:     "<p>" + title + "</p>",
		AuthorID:    authorID,
		Status:      models.StatusPublished,
		PublishedAt: &now,
	}
	if err := db.Create(&article).Error; err != nil {
		t.Fatalf("failed to create test article: %v", err)
	}
	return &article
}
