package services

import (
	"errors"
	"testing"

	"github.com/yamovo/contentx/internal/models"
)

// ─── UserService Tests ──────────────────────────────────────────────────────

func TestUserService_List_Empty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewUserService(db)

	// Seed creates an admin user, so not truly empty.
	users, total, err := svc.List(UserListParams{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 seeded user, got %d", total)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
}

func TestUserService_List_WithSearch(t *testing.T) {
	db := setupTestDB(t)
	svc := NewUserService(db)

	createTestUser(t, db, "alice", "author")
	createTestUser(t, db, "bob", "editor")

	users, total, err := svc.List(UserListParams{Page: 1, PageSize: 10, Search: "ali"})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 user matching 'ali', got %d", total)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
}

func TestUserService_Get_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewUserService(db)
	user := createTestUser(t, db, "getuser", "admin")

	found, err := svc.Get(user.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if found.ID != user.ID {
		t.Fatalf("expected ID %d, got %d", user.ID, found.ID)
	}
	if found.Role.ID == 0 {
		t.Fatal("expected role to be preloaded")
	}
}

func TestUserService_Get_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewUserService(db)

	_, err := svc.Get(99999)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserService_Create_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewUserService(db)

	var role models.Role
	db.Where("slug = ?", "author").First(&role)

	user, err := svc.Create(CreateUserRequest{
		Username:    "newuser",
		Email:       "new@test.com",
		Password:    "SecurePass123",
		DisplayName: "New User",
		RoleID:      role.ID,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if user.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if user.Role.ID == 0 {
		t.Fatal("expected role to be preloaded")
	}
	if user.Status != models.UserStatusActive {
		t.Fatalf("expected default status active, got %s", user.Status)
	}
}

func TestUserService_Create_DuplicateUsername(t *testing.T) {
	db := setupTestDB(t)
	svc := NewUserService(db)
	createTestUser(t, db, "dupuser", "admin")

	var role models.Role
	db.Where("slug = ?", "admin").First(&role)

	_, err := svc.Create(CreateUserRequest{
		Username: "dupuser",
		Email:    "another@test.com",
		Password: "SecurePass123",
		RoleID:   role.ID,
	})
	if !errors.Is(err, ErrUsernameExists) {
		t.Fatalf("expected ErrUsernameExists, got %v", err)
	}
}

func TestUserService_Update_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewUserService(db)
	user := createTestUser(t, db, "updateuser", "author")

	newName := "Updated Name"
	newBio := "Updated bio"
	updated, err := svc.Update(user.ID, UpdateUserRequest{
		DisplayName: &newName,
		Bio:         &newBio,
	})
	if err != nil {
		t.Fatalf("update user: %v", err)
	}
	if updated.DisplayName != newName {
		t.Fatalf("expected display_name '%s', got '%s'", newName, updated.DisplayName)
	}
	if updated.Bio != newBio {
		t.Fatalf("expected bio '%s', got '%s'", newBio, updated.Bio)
	}
}

func TestUserService_Update_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewUserService(db)

	name := "x"
	_, err := svc.Update(99999, UpdateUserRequest{DisplayName: &name})
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserService_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewUserService(db)
	user := createTestUser(t, db, "deleteuser", "author")

	if err := svc.Delete(user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	var count int64
	db.Model(&models.User{}).Where("id = ?", user.ID).Count(&count)
	if count != 0 {
		t.Fatal("user should be soft-deleted")
	}
}

func TestUserService_Delete_AdminProtected(t *testing.T) {
	db := setupTestDB(t)
	svc := NewUserService(db)

	// Seed creates an admin user (ID=1).
	err := svc.Delete(1)
	if !errors.Is(err, ErrCannotDeleteAdmin) {
		t.Fatalf("expected ErrCannotDeleteAdmin, got %v", err)
	}
}

func TestUserService_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewUserService(db)

	err := svc.Delete(99999)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUserService_ResetPassword(t *testing.T) {
	db := setupTestDB(t)
	svc := NewUserService(db)
	user := createTestUser(t, db, "resetuser", "admin")

	if err := svc.ResetPassword(user.ID, "NewPassword123"); err != nil {
		t.Fatalf("reset password: %v", err)
	}

	var refreshed models.User
	db.First(&refreshed, user.ID)
	if refreshed.Password == user.Password {
		t.Fatal("password should have changed")
	}
}

// ─── RoleService Tests ──────────────────────────────────────────────────────

func TestRoleService_List(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRoleService(db)

	roles, err := svc.List()
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	// Seed creates 4 roles: admin, editor, author, subscriber.
	if len(roles) != 4 {
		t.Fatalf("expected 4 roles, got %d", len(roles))
	}
	// Admin role should have permissions preloaded.
	for _, r := range roles {
		if r.Slug == "admin" && len(r.Permissions) == 0 {
			t.Fatal("admin role should have permissions preloaded")
		}
	}
}

func TestRoleService_Create_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRoleService(db)

	var perm models.Permission
	db.Where("slug = ?", "articles.read").First(&perm)

	role, err := svc.Create(CreateRoleRequest{
		Name:          "Content Manager",
		Slug:          "content_manager",
		Description:   "Manages content",
		PermissionIDs: []uint{perm.ID},
	})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	if role.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if len(role.Permissions) != 1 {
		t.Fatalf("expected 1 permission, got %d", len(role.Permissions))
	}
}

func TestRoleService_Create_DuplicateSlug(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRoleService(db)

	_, err := svc.Create(CreateRoleRequest{Name: "Admin2", Slug: "admin"})
	if !errors.Is(err, ErrRoleExists) {
		t.Fatalf("expected ErrRoleExists, got %v", err)
	}
}

func TestRoleService_Update_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRoleService(db)

	role, _ := svc.Create(CreateRoleRequest{Name: "Original", Slug: "original_role"})

	newName := "Updated Role"
	updated, err := svc.Update(role.ID, UpdateRoleRequest{Name: &newName})
	if err != nil {
		t.Fatalf("update role: %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("expected name '%s', got '%s'", newName, updated.Name)
	}
}

func TestRoleService_Update_SystemRole(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRoleService(db)

	// Create a system role directly (seed doesn't set IsSystem).
	systemRole := models.Role{Name: "System", Slug: "system_role", IsSystem: true}
	db.Create(&systemRole)

	name := "Hacked"
	_, err := svc.Update(systemRole.ID, UpdateRoleRequest{Name: &name})
	if !errors.Is(err, ErrSystemRole) {
		t.Fatalf("expected ErrSystemRole, got %v", err)
	}
}

func TestRoleService_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRoleService(db)

	role, _ := svc.Create(CreateRoleRequest{Name: "ToDelete", Slug: "to_delete"})

	if err := svc.Delete(role.ID); err != nil {
		t.Fatalf("delete role: %v", err)
	}

	var count int64
	db.Model(&models.Role{}).Where("id = ?", role.ID).Count(&count)
	if count != 0 {
		t.Fatal("role should be deleted")
	}
}

func TestRoleService_Delete_SystemRole(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRoleService(db)

	// Create a system role directly (seed doesn't set IsSystem).
	systemRole := models.Role{Name: "System2", Slug: "sys_del", IsSystem: true}
	db.Create(&systemRole)

	err := svc.Delete(systemRole.ID)
	if !errors.Is(err, ErrSystemRole) {
		t.Fatalf("expected ErrSystemRole, got %v", err)
	}
}

func TestRoleService_Delete_RoleInUse(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRoleService(db)

	// Create a non-system role and assign a user to it.
	role, _ := svc.Create(CreateRoleRequest{Name: "InUse", Slug: "in_use"})
	createTestUser(t, db, "inuseuser", "in_use")

	err := svc.Delete(role.ID)
	if !errors.Is(err, ErrRoleInUse) {
		t.Fatalf("expected ErrRoleInUse, got %v", err)
	}
}

func TestRoleService_Permissions(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRoleService(db)

	perms, grouped, err := svc.Permissions()
	if err != nil {
		t.Fatalf("permissions: %v", err)
	}
	if len(perms) == 0 {
		t.Fatal("expected non-empty permissions list")
	}
	if len(grouped) == 0 {
		t.Fatal("expected non-empty grouped map")
	}
	// Should have 'articles' module.
	if _, ok := grouped["articles"]; !ok {
		t.Fatal("expected 'articles' module in grouped permissions")
	}
}
