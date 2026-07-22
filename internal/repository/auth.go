package repository

import (
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// AuthRepository consolidates auth-related data access across the User, Role,
// ActivityLog, and SiteSetting aggregates. Keeping these operations in one
// repository avoids forcing AuthService to depend on four separate interfaces
// for a single login/register flow.
type AuthRepository interface {
	// User lookups
	FindUserByUsernameOrEmail(identifier string) (*models.User, error) // preloads Role
	FindUserByIDWithRole(id uint) (*models.User, error)                // preloads Role
	FindUserByIDWithPermissions(id uint) (*models.User, error)         // preloads Role.Permissions
	FindUserByID(id uint) (*models.User, error)                        // no preload
	CountUsersByUsernameOrEmail(username, email string) (int64, error)
	CreateUser(user *models.User) error
	UpdateUserFields(id uint, updates map[string]interface{}) error
	UpdateUserPassword(id uint, hashedPassword string) error

	// Role lookups
	FindDefaultRole() (*models.Role, error)
	FindRoleBySlug(slug string) (*models.Role, error)

	// Activity log
	CreateActivityLog(log *models.ActivityLog) error

	// Settings
	FindSetting(key string) (*models.SiteSetting, error)
}

// gormAuthRepository implements AuthRepository with GORM.
type gormAuthRepository struct {
	db *gorm.DB
}

// NewAuthRepository builds a GORM-backed AuthRepository.
func NewAuthRepository(db *gorm.DB) AuthRepository {
	return &gormAuthRepository{db: db}
}

func (r *gormAuthRepository) FindUserByUsernameOrEmail(identifier string) (*models.User, error) {
	var user models.User
	if err := r.db.Preload("Role").
		Where("username = ? OR email = ?", identifier, identifier).
		First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormAuthRepository) FindUserByIDWithRole(id uint) (*models.User, error) {
	var user models.User
	if err := r.db.Preload("Role").First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormAuthRepository) FindUserByIDWithPermissions(id uint) (*models.User, error) {
	var user models.User
	if err := r.db.Preload("Role").Preload("Role.Permissions").First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormAuthRepository) FindUserByID(id uint) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormAuthRepository) CountUsersByUsernameOrEmail(username, email string) (int64, error) {
	var count int64
	if err := r.db.Model(&models.User{}).
		Where("username = ? OR email = ?", username, email).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *gormAuthRepository) CreateUser(user *models.User) error {
	return r.db.Create(user).Error
}

func (r *gormAuthRepository) UpdateUserFields(id uint, updates map[string]interface{}) error {
	return r.db.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error
}

func (r *gormAuthRepository) UpdateUserPassword(id uint, hashedPassword string) error {
	return r.db.Model(&models.User{}).Where("id = ?", id).Update("password", hashedPassword).Error
}

func (r *gormAuthRepository) FindDefaultRole() (*models.Role, error) {
	var role models.Role
	if err := r.db.Where("is_default = ?", true).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *gormAuthRepository) FindRoleBySlug(slug string) (*models.Role, error) {
	var role models.Role
	if err := r.db.Where("slug = ?", slug).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *gormAuthRepository) CreateActivityLog(log *models.ActivityLog) error {
	return r.db.Create(log).Error
}

func (r *gormAuthRepository) FindSetting(key string) (*models.SiteSetting, error) {
	var setting models.SiteSetting
	if err := r.db.Where("key = ?", key).First(&setting).Error; err != nil {
		return nil, err
	}
	return &setting, nil
}
