package repository

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

func TestUserRepository_CreateAndGetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)

	var authorRole models.Role
	db.Where("slug = ?", "author").First(&authorRole)

	user := &models.User{
		Username:    "newuser",
		Email:       "newuser@test.com",
		Password:    "hash",
		DisplayName: "New User",
		RoleID:      authorRole.ID,
		Status:      models.UserStatusActive,
	}
	if err := repo.Create(user); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if user.ID == 0 {
		t.Fatal("ID should be set")
	}

	got, err := repo.GetByID(user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Username != "newuser" {
		t.Fatalf("unexpected username: %q", got.Username)
	}
	if got.Role.ID != authorRole.ID {
		t.Fatalf("Role should be preloaded: got ID %d, want %d", got.Role.ID, authorRole.ID)
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	_, err := repo.GetByID(99999)
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestUserRepository_List_Filters(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	createTestUser(t, db, "alice", "author")
	createTestUser(t, db, "bob", "editor")

	// Filter by role=author.
	users, total, err := repo.List(UserListFilter{
		Page: 1, PageSize: 10, Role: "author",
	})
	if err != nil {
		t.Fatalf("List by role: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total=1 for author role, got %d", total)
	}
	if len(users) != 1 || users[0].Username != "alice" {
		t.Fatalf("expected alice, got %+v", users)
	}

	// Filter by search.
	users, total, err = repo.List(UserListFilter{
		Page: 1, PageSize: 10, Search: "bob",
	})
	if err != nil {
		t.Fatalf("List by search: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total=1 for search 'bob', got %d", total)
	}
	if users[0].Username != "bob" {
		t.Fatalf("expected bob, got %q", users[0].Username)
	}
}

func TestUserRepository_UpdateFields(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	user := createTestUser(t, db, "updateuser", "author")

	if err := repo.UpdateFields(user.ID, map[string]interface{}{
		"display_name": "Updated Name",
		"status":       models.UserStatusInactive,
	}); err != nil {
		t.Fatalf("UpdateFields: %v", err)
	}

	var got models.User
	db.First(&got, user.ID)
	if got.DisplayName != "Updated Name" {
		t.Fatalf("display_name should be updated, got %q", got.DisplayName)
	}
	if got.Status != models.UserStatusInactive {
		t.Fatalf("status should be inactive, got %q", got.Status)
	}
}

func TestUserRepository_UpdatePassword(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	user := createTestUser(t, db, "pwduser", "author")

	if err := repo.UpdatePassword(user.ID, "newhash"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}

	var got models.User
	db.First(&got, user.ID)
	if got.Password != "newhash" {
		t.Fatalf("password should be updated, got %q", got.Password)
	}
}

func TestUserRepository_SoftDelete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	user := createTestUser(t, db, "deleteuser", "author")

	if err := repo.SoftDelete(user); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	// Soft-deleted records should not be found by default.
	_, err := repo.FindByID(user.ID)
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("expected ErrRecordNotFound after soft delete, got %v", err)
	}
}

func TestRoleRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRoleRepository(db)

	roles, err := repo.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Seed creates at least admin and author roles.
	if len(roles) < 2 {
		t.Fatalf("expected at least 2 roles, got %d", len(roles))
	}
	// Each role should have permissions preloaded.
	for _, role := range roles {
		if role.Permissions == nil {
			t.Fatalf("role %q should have permissions preloaded", role.Name)
		}
	}
}

func TestRoleRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRoleRepository(db)
	roles, _ := repo.List()
	if len(roles) == 0 {
		t.Fatal("no roles found")
	}

	got, err := repo.GetByID(roles[0].ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != roles[0].ID {
		t.Fatalf("unexpected role ID: got %d, want %d", got.ID, roles[0].ID)
	}
}

func TestRoleRepository_ReplaceAndClearPermissions(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRoleRepository(db)
	allPerms, _ := repo.ListAllPermissions()
	if len(allPerms) < 2 {
		t.Fatal("need at least 2 permissions for this test")
	}

	// Create a test role.
	role := &models.Role{Name: "Test", Slug: "test-role"}
	if err := repo.Create(role); err != nil {
		t.Fatalf("Create role: %v", err)
	}

	// Replace permissions with the first 2.
	if err := repo.ReplacePermissions(role.ID, allPerms[:2]); err != nil {
		t.Fatalf("ReplacePermissions: %v", err)
	}
	got, _ := repo.GetByID(role.ID)
	if len(got.Permissions) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(got.Permissions))
	}

	// Clear permissions.
	if err := repo.ClearPermissions(role.ID); err != nil {
		t.Fatalf("ClearPermissions: %v", err)
	}
	got, _ = repo.GetByID(role.ID)
	if len(got.Permissions) != 0 {
		t.Fatalf("expected 0 permissions after clear, got %d", len(got.Permissions))
	}
}

func TestRoleRepository_CountUsersByRoleID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRoleRepository(db)
	createTestUser(t, db, "count1", "author")
	createTestUser(t, db, "count2", "author")

	var authorRole models.Role
	db.Where("slug = ?", "author").First(&authorRole)

	count, err := repo.CountUsersByRoleID(authorRole.ID)
	if err != nil {
		t.Fatalf("CountUsersByRoleID: %v", err)
	}
	if count < 2 {
		t.Fatalf("expected at least 2 users with author role, got %d", count)
	}
}
