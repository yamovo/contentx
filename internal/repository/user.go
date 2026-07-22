package repository

import (
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// UserListFilter holds query parameters for listing users.
type UserListFilter struct {
	Page     int
	PageSize int
	Role     string // role slug filter
	Status   string // user status filter
	Search   string // matches username, email, or display_name
}

// UserRepository defines data-access operations for users.
type UserRepository interface {
	List(filter UserListFilter) ([]models.User, int64, error)
	GetByID(id uint) (*models.User, error)  // preloads Role
	FindByID(id uint) (*models.User, error) // no preload; returns gorm.ErrRecordNotFound if missing
	Create(user *models.User) error
	UpdateFields(id uint, updates map[string]interface{}) error
	UpdatePassword(id uint, hashedPassword string) error
	SoftDelete(user *models.User) error
}

// gormUserRepository implements UserRepository with GORM.
type gormUserRepository struct {
	db *gorm.DB
}

// NewUserRepository builds a GORM-backed UserRepository.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &gormUserRepository{db: db}
}

func (r *gormUserRepository) List(filter UserListFilter) ([]models.User, int64, error) {
	query := r.db.Model(&models.User{}).Preload("Role")

	if filter.Role != "" {
		query = query.Joins("JOIN roles ON roles.id = users.role_id").Where("roles.slug = ?", filter.Role)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Search != "" {
		like := "%" + filter.Search + "%"
		query = query.Where("username LIKE ? OR email LIKE ? OR display_name LIKE ?", like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []models.User
	if err := query.Order("created_at DESC").
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *gormUserRepository) GetByID(id uint) (*models.User, error) {
	var user models.User
	if err := r.db.Preload("Role").First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormUserRepository) FindByID(id uint) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormUserRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *gormUserRepository) UpdateFields(id uint, updates map[string]interface{}) error {
	return r.db.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error
}

func (r *gormUserRepository) UpdatePassword(id uint, hashedPassword string) error {
	return r.db.Model(&models.User{}).Where("id = ?", id).Update("password", hashedPassword).Error
}

func (r *gormUserRepository) SoftDelete(user *models.User) error {
	return r.db.Delete(user).Error
}

// ─── Role ───────────────────────────────────────────────────────────────────

// RoleRepository defines data-access operations for roles and permissions.
type RoleRepository interface {
	List() ([]models.Role, error)           // preloads Permissions, ordered by id ASC
	GetByID(id uint) (*models.Role, error)  // preloads Permissions
	FindByID(id uint) (*models.Role, error) // no preload; returns gorm.ErrRecordNotFound if missing
	Create(role *models.Role) error
	UpdateField(id uint, field string, value interface{}) error
	ReplacePermissions(roleID uint, perms []models.Permission) error
	ClearPermissions(roleID uint) error
	Delete(role *models.Role) error
	ListPermissionsByIDs(ids []uint) ([]models.Permission, error)
	ListAllPermissions() ([]models.Permission, error) // ordered by module, name ASC
	CountUsersByRoleID(roleID uint) (int64, error)    // cross-aggregate read for List() enrichment
}

// gormRoleRepository implements RoleRepository with GORM.
type gormRoleRepository struct {
	db *gorm.DB
}

// NewRoleRepository builds a GORM-backed RoleRepository.
func NewRoleRepository(db *gorm.DB) RoleRepository {
	return &gormRoleRepository{db: db}
}

func (r *gormRoleRepository) List() ([]models.Role, error) {
	var roles []models.Role
	if err := r.db.Preload("Permissions").Order("id ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *gormRoleRepository) GetByID(id uint) (*models.Role, error) {
	var role models.Role
	if err := r.db.Preload("Permissions").First(&role, id).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *gormRoleRepository) FindByID(id uint) (*models.Role, error) {
	var role models.Role
	if err := r.db.First(&role, id).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *gormRoleRepository) Create(role *models.Role) error {
	return r.db.Create(role).Error
}

func (r *gormRoleRepository) UpdateField(id uint, field string, value interface{}) error {
	return r.db.Model(&models.Role{}).Where("id = ?", id).Update(field, value).Error
}

func (r *gormRoleRepository) ReplacePermissions(roleID uint, perms []models.Permission) error {
	role := models.Role{}
	role.ID = roleID
	return r.db.Model(&role).Association("Permissions").Replace(perms)
}

func (r *gormRoleRepository) ClearPermissions(roleID uint) error {
	role := models.Role{}
	role.ID = roleID
	return r.db.Model(&role).Association("Permissions").Clear()
}

func (r *gormRoleRepository) Delete(role *models.Role) error {
	return r.db.Delete(role).Error
}

func (r *gormRoleRepository) ListPermissionsByIDs(ids []uint) ([]models.Permission, error) {
	var perms []models.Permission
	if err := r.db.Where("id IN ?", ids).Find(&perms).Error; err != nil {
		return nil, err
	}
	return perms, nil
}

func (r *gormRoleRepository) ListAllPermissions() ([]models.Permission, error) {
	var perms []models.Permission
	if err := r.db.Order("module ASC, name ASC").Find(&perms).Error; err != nil {
		return nil, err
	}
	return perms, nil
}

func (r *gormRoleRepository) CountUsersByRoleID(roleID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&models.User{}).Where("role_id = ?", roleID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
