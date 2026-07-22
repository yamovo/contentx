package services

import (
	"errors"
	"fmt"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
	"gorm.io/gorm"
)

// Sentinel errors returned by the service layer.
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUsernameExists    = errors.New("username or email already exists")
	ErrCannotDeleteAdmin = errors.New("cannot delete admin user")
	ErrRoleNotFound      = errors.New("role not found")
	ErrRoleExists        = errors.New("role already exists")
	ErrSystemRole        = errors.New("cannot modify or delete system roles")
	ErrRoleInUse         = errors.New("role is still assigned to users")
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
	repo    repository.UserRepository
	webhook WebhookDispatcher
}

// NewUserService creates a new UserService backed by a GORM repository.
// Kept for backward compatibility with existing callers and tests.
func NewUserService(db *gorm.DB) *UserService {
	return &UserService{repo: repository.NewUserRepository(db)}
}

// NewUserServiceWithRepo builds a UserService with an explicit repository,
// enabling unit tests to inject mocks.
func NewUserServiceWithRepo(repo repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

// SetWebhookDispatcher attaches a webhook dispatcher for event triggering.
func (s *UserService) SetWebhookDispatcher(d WebhookDispatcher) { s.webhook = d }

// List returns a paginated, filtered list of users and the total count.
func (s *UserService) List(params UserListParams) ([]models.User, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}

	users, total, err := s.repo.List(repository.UserListFilter{
		Page:     params.Page,
		PageSize: params.PageSize,
		Role:     params.Role,
		Status:   params.Status,
		Search:   params.Search,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	return users, total, nil
}

// Get returns a single user by ID, preloading the Role.
func (s *UserService) Get(id uint) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
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

	if err := s.repo.Create(&user); err != nil {
		return nil, ErrUsernameExists
	}

	// Reload with associations (best-effort; preserve prior behaviour of ignoring reload error).
	var result *models.User
	if reloaded, err := s.repo.GetByID(user.ID); err == nil && reloaded != nil {
		result = reloaded
	} else {
		result = &user
	}

	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventUserCreate, result)
	}
	return result, nil
}

// Update applies partial updates to a user and returns the refreshed record.
func (s *UserService) Update(id uint, req UpdateUserRequest) (*models.User, error) {
	if _, err := s.repo.FindByID(id); err != nil {
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
		if err := s.repo.UpdateFields(id, updates); err != nil {
			return nil, fmt.Errorf("update user: %w", err)
		}
	}

	// Reload with Role preloaded (best-effort).
	user, err := s.repo.GetByID(id)
	if err != nil {
		// Fall back to a bare record if preload fails for any reason.
		if bare, ferr := s.repo.FindByID(id); ferr == nil {
			return bare, nil
		}
		return nil, fmt.Errorf("reload user: %w", err)
	}
	return user, nil
}

// Delete soft-deletes a user. Returns an error if the user is an admin.
func (s *UserService) Delete(id uint) error {
	user, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("find user: %w", err)
	}

	if user.IsAdmin() {
		return ErrCannotDeleteAdmin
	}

	if err := s.repo.SoftDelete(user); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// ResetPassword hashes the new password and updates the user record.
func (s *UserService) ResetPassword(id uint, newPassword string) error {
	if _, err := s.repo.FindByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("find user: %w", err)
	}

	hashedPw, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.repo.UpdatePassword(id, hashedPw); err != nil {
		return fmt.Errorf("reset password: %w", err)
	}
	return nil
}

// ---------- RoleService ----------

// RoleService provides role-related business logic.
type RoleService struct {
	repo repository.RoleRepository
}

// NewRoleService creates a new RoleService backed by a GORM repository.
// Kept for backward compatibility with existing callers and tests.
func NewRoleService(db *gorm.DB) *RoleService {
	return &RoleService{repo: repository.NewRoleRepository(db)}
}

// NewRoleServiceWithRepo builds a RoleService with an explicit repository,
// enabling unit tests to inject mocks.
func NewRoleServiceWithRepo(repo repository.RoleRepository) *RoleService {
	return &RoleService{repo: repo}
}

// List returns all roles with permissions preloaded and user counts populated.
func (s *RoleService) List() ([]models.Role, error) {
	roles, err := s.repo.List()
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}

	for i := range roles {
		// Best-effort user-count enrichment (mirrors prior behaviour: errors ignored).
		count, _ := s.repo.CountUsersByRoleID(roles[i].ID)
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

	if err := s.repo.Create(&role); err != nil {
		return nil, ErrRoleExists
	}

	if len(req.PermissionIDs) > 0 {
		perms, _ := s.repo.ListPermissionsByIDs(req.PermissionIDs)
		_ = s.repo.ReplacePermissions(role.ID, perms)
	}

	// Reload with permissions preloaded (best-effort).
	reloaded, err := s.repo.GetByID(role.ID)
	if err != nil {
		return &role, nil
	}
	return reloaded, nil
}

// Update modifies a role. System roles cannot be updated.
func (s *RoleService) Update(id uint, req UpdateRoleRequest) (*models.Role, error) {
	role, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("find role: %w", err)
	}

	if role.IsSystem {
		return nil, ErrSystemRole
	}

	// Preserve prior behaviour: errors from individual field updates are ignored.
	if req.Name != nil {
		_ = s.repo.UpdateField(id, "name", *req.Name)
	}
	if req.Description != nil {
		_ = s.repo.UpdateField(id, "description", *req.Description)
	}
	if req.PermissionIDs != nil {
		perms, _ := s.repo.ListPermissionsByIDs(req.PermissionIDs)
		_ = s.repo.ReplacePermissions(role.ID, perms)
	}

	// Reload with permissions preloaded (best-effort).
	reloaded, err := s.repo.GetByID(id)
	if err != nil {
		return role, nil
	}
	return reloaded, nil
}

// Delete removes a role after checking it is not a system role and not in use.
func (s *RoleService) Delete(id uint) error {
	role, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("find role: %w", err)
	}

	if role.IsSystem {
		return ErrSystemRole
	}

	count, err := s.repo.CountUsersByRoleID(role.ID)
	if err != nil {
		return fmt.Errorf("count role users: %w", err)
	}
	if count > 0 {
		return ErrRoleInUse
	}

	_ = s.repo.ClearPermissions(role.ID)
	if err := s.repo.Delete(role); err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	return nil
}

// Permissions returns all permissions as a flat list and grouped by module.
func (s *RoleService) Permissions() ([]models.Permission, map[string][]models.Permission, error) {
	perms, err := s.repo.ListAllPermissions()
	if err != nil {
		return nil, nil, fmt.Errorf("list permissions: %w", err)
	}

	grouped := make(map[string][]models.Permission)
	for _, p := range perms {
		grouped[p.Module] = append(grouped[p.Module], p)
	}

	return perms, grouped, nil
}
