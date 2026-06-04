package services

import (
	"errors"
	"fmt"

	"github.com/vortexcms/go-cms/internal/auth"
	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// Sentinel errors returned by the service layer.
var (
	ErrUserNotFound    = errors.New("user not found")
	ErrUsernameExists  = errors.New("username or email already exists")
	ErrCannotDeleteAdmin = errors.New("cannot delete admin user")
	ErrRoleNotFound    = errors.New("role not found")
	ErrRoleExists      = errors.New("role already exists")
	ErrSystemRole      = errors.New("cannot modify or delete system roles")
	ErrRoleInUse       = errors.New("role is still assigned to users")
)

// ---------- Request / param structs ----------

// UserListParams holds query parameters for listing users.
type UserListParams struct {
	Page     int
	PageSize int
	Role     string // role slug filter
	Status   string // user status filter
	Search   string // matches username, email, or display_name
}

// CreateUserRequest is the payload for creating a user.
type CreateUserRequest struct {
	Username    string `json:"username" binding:"required,min=3,max=64"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name"`
	RoleID      uint   `json:"role_id" binding:"required"`
	Status      string `json:"status"`
}

// UpdateUserRequest is the payload for updating a user. Pointer fields indicate "set if non-nil".
type UpdateUserRequest struct {
	Email       *string `json:"email"`
	DisplayName *string `json:"display_name"`
	RoleID      *uint   `json:"role_id"`
	Status      *string `json:"status"`
	Bio         *string `json:"bio"`
	Website     *string `json:"website"`
	Avatar      *string `json:"avatar"`
}

// CreateRoleRequest is the payload for creating a role.
type CreateRoleRequest struct {
	Name          string `json:"name" binding:"required"`
	Slug          string `json:"slug" binding:"required"`
	Description   string `json:"description"`
	PermissionIDs []uint `json:"permission_ids"`
}

// UpdateRoleRequest is the payload for updating a role.
type UpdateRoleRequest struct {
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	PermissionIDs []uint  `json:"permission_ids"`
}

// ---------- UserService ----------

// UserService provides user-related business logic.
type UserService struct {
	db *gorm.DB
}

// NewUserService creates a new UserService.
func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// List returns a paginated, filtered list of users and the total count.
func (s *UserService) List(params UserListParams) ([]models.User, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}

	query := s.db.Model(&models.User{}).Preload("Role")

	if params.Role != "" {
		query = query.Joins("JOIN roles ON roles.id = users.role_id").Where("roles.slug = ?", params.Role)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.Search != "" {
		like := "%" + params.Search + "%"
		query = query.Where("username LIKE ? OR email LIKE ? OR display_name LIKE ?", like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	var users []models.User
	if err := query.Order("created_at DESC").
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	return users, total, nil
}

// Get returns a single user by ID, preloading the Role.
func (s *UserService) Get(id uint) (*models.User, error) {
	var user models.User
	if err := s.db.Preload("Role").First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &user, nil
}

// Create inserts a new user after hashing the password and applying defaults.
func (s *UserService) Create(req CreateUserRequest) (*models.User, error) {
	hashedPw, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := models.User{
		Username:    req.Username,
		Email:       req.Email,
		Password:    hashedPw,
		DisplayName: req.DisplayName,
		RoleID:      req.RoleID,
		Status:      models.UserStatus(req.Status),
	}
	if user.Status == "" {
		user.Status = models.UserStatusActive
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, ErrUsernameExists
	}

	// Reload with associations.
	s.db.Preload("Role").First(&user, user.ID)
	return &user, nil
}

// Update applies partial updates to a user and returns the refreshed record.
func (s *UserService) Update(id uint, req UpdateUserRequest) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("find user: %w", err)
	}

	updates := map[string]interface{}{}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.RoleID != nil {
		updates["role_id"] = *req.RoleID
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Bio != nil {
		updates["bio"] = *req.Bio
	}
	if req.Website != nil {
		updates["website"] = *req.Website
	}
	if req.Avatar != nil {
		updates["avatar"] = *req.Avatar
	}

	if len(updates) > 0 {
		if err := s.db.Model(&user).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("update user: %w", err)
		}
	}

	s.db.Preload("Role").First(&user, user.ID)
	return &user, nil
}

// Delete soft-deletes a user. Returns an error if the user is an admin.
func (s *UserService) Delete(id uint) error {
	var user models.User
	if err := s.db.Preload("Role").First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("find user: %w", err)
	}

	if user.IsAdmin() {
		return ErrCannotDeleteAdmin
	}

	if err := s.db.Delete(&user).Error; err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// ResetPassword hashes the new password and updates the user record.
func (s *UserService) ResetPassword(id uint, newPassword string) error {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("find user: %w", err)
	}

	hashedPw, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.db.Model(&user).Update("password", hashedPw).Error; err != nil {
		return fmt.Errorf("reset password: %w", err)
	}
	return nil
}

// ---------- RoleService ----------

// RoleService provides role-related business logic.
type RoleService struct {
	db *gorm.DB
}

// NewRoleService creates a new RoleService.
func NewRoleService(db *gorm.DB) *RoleService {
	return &RoleService{db: db}
}

// List returns all roles with permissions preloaded and user counts populated.
func (s *RoleService) List() ([]models.Role, error) {
	var roles []models.Role
	if err := s.db.Preload("Permissions").Order("id ASC").Find(&roles).Error; err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}

	for i := range roles {
		var count int64
		s.db.Model(&models.User{}).Where("role_id = ?", roles[i].ID).Count(&count)
		roles[i].UserCount = int(count)
	}

	return roles, nil
}

// Create inserts a new role and assigns the given permission IDs.
func (s *RoleService) Create(req CreateRoleRequest) (*models.Role, error) {
	role := models.Role{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
	}

	if err := s.db.Create(&role).Error; err != nil {
		return nil, ErrRoleExists
	}

	if len(req.PermissionIDs) > 0 {
		var perms []models.Permission
		s.db.Where("id IN ?", req.PermissionIDs).Find(&perms)
		s.db.Model(&role).Association("Permissions").Replace(perms)
	}

	s.db.Preload("Permissions").First(&role, role.ID)
	return &role, nil
}

// Update modifies a role. System roles cannot be updated.
func (s *RoleService) Update(id uint, req UpdateRoleRequest) (*models.Role, error) {
	var role models.Role
	if err := s.db.First(&role, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("find role: %w", err)
	}

	if role.IsSystem {
		return nil, ErrSystemRole
	}

	if req.Name != nil {
		s.db.Model(&role).Update("name", *req.Name)
	}
	if req.Description != nil {
		s.db.Model(&role).Update("description", *req.Description)
	}
	if req.PermissionIDs != nil {
		var perms []models.Permission
		s.db.Where("id IN ?", req.PermissionIDs).Find(&perms)
		s.db.Model(&role).Association("Permissions").Replace(perms)
	}

	s.db.Preload("Permissions").First(&role, role.ID)
	return &role, nil
}

// Delete removes a role after checking it is not a system role and not in use.
func (s *RoleService) Delete(id uint) error {
	var role models.Role
	if err := s.db.First(&role, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("find role: %w", err)
	}

	if role.IsSystem {
		return ErrSystemRole
	}

	var count int64
	s.db.Model(&models.User{}).Where("role_id = ?", role.ID).Count(&count)
	if count > 0 {
		return ErrRoleInUse
	}

	s.db.Model(&role).Association("Permissions").Clear()
	if err := s.db.Delete(&role).Error; err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	return nil
}

// Permissions returns all permissions as a flat list and grouped by module.
func (s *RoleService) Permissions() ([]models.Permission, map[string][]models.Permission, error) {
	var perms []models.Permission
	if err := s.db.Order("module ASC, name ASC").Find(&perms).Error; err != nil {
		return nil, nil, fmt.Errorf("list permissions: %w", err)
	}

	grouped := make(map[string][]models.Permission)
	for _, p := range perms {
		grouped[p.Module] = append(grouped[p.Module], p)
	}

	return perms, grouped, nil
}
