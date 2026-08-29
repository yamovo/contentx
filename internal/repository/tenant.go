package repository

import (
	"time"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// TenantMember is a flat projection of a membership joined with its user, for
// member-management listings.
type TenantMember struct {
	TenantID    uint
	UserID      uint
	RoleSlug    string
	JoinedAt    time.Time
	Username    string
	Email       string
	DisplayName string
	UserStatus  string
}

// TenantRepository defines data-access operations for tenant administration
// (RFC-001 PR-5): deployment-wide tenant CRUD plus membership management.
// These operate on the identity plane and are guarded by platform permissions.
type TenantRepository interface {
	List() ([]models.Tenant, error)
	GetByID(id uint) (*models.Tenant, error)
	GetBySlug(slug string) (*models.Tenant, error)
	Create(tenant *models.Tenant) error
	Update(tenant *models.Tenant) error

	ListMembers(tenantID uint) ([]TenantMember, error)
	AddMember(m *models.TenantMembership) error
	UpdateMemberRole(tenantID, userID uint, roleSlug string) error
	RemoveMember(tenantID, userID uint) error
	HasMembership(tenantID, userID uint) (bool, error)
	CountAdmins(tenantID uint) (int64, error)
	UserExists(userID uint) (bool, error)
}

type gormTenantRepository struct {
	db *gorm.DB
}

// NewTenantRepository builds a GORM-backed TenantRepository.
func NewTenantRepository(db *gorm.DB) TenantRepository {
	return &gormTenantRepository{db: db}
}

func (r *gormTenantRepository) List() ([]models.Tenant, error) {
	var tenants []models.Tenant
	if err := r.db.Order("id ASC").Find(&tenants).Error; err != nil {
		return nil, err
	}
	return tenants, nil
}

func (r *gormTenantRepository) GetByID(id uint) (*models.Tenant, error) {
	var tenant models.Tenant
	if err := r.db.First(&tenant, id).Error; err != nil {
		return nil, err
	}
	return &tenant, nil
}

func (r *gormTenantRepository) GetBySlug(slug string) (*models.Tenant, error) {
	var tenant models.Tenant
	if err := r.db.Where("slug = ?", slug).First(&tenant).Error; err != nil {
		return nil, err
	}
	return &tenant, nil
}

func (r *gormTenantRepository) Create(tenant *models.Tenant) error {
	return r.db.Create(tenant).Error
}

func (r *gormTenantRepository) Update(tenant *models.Tenant) error {
	return r.db.Save(tenant).Error
}

func (r *gormTenantRepository) ListMembers(tenantID uint) ([]TenantMember, error) {
	var members []TenantMember
	// The raw Table() join bypasses GORM's soft-delete scoping, so the live
	// filter is applied explicitly: removed members must not appear here, or
	// last-admin accounting in the service layer double-counts them.
	err := r.db.Table("tenant_memberships").
		Select("tenant_memberships.tenant_id, tenant_memberships.user_id, tenant_memberships.role_slug, "+
			"tenant_memberships.created_at AS joined_at, users.username, users.email, users.display_name, users.status AS user_status").
		Joins("JOIN users ON users.id = tenant_memberships.user_id").
		Where("tenant_memberships.tenant_id = ? AND tenant_memberships.deleted_at IS NULL", tenantID).
		Order("tenant_memberships.created_at ASC, tenant_memberships.user_id ASC").
		Scan(&members).Error
	return members, err
}

func (r *gormTenantRepository) AddMember(m *models.TenantMembership) error {
	return r.db.Create(m).Error
}

func (r *gormTenantRepository) UpdateMemberRole(tenantID, userID uint, roleSlug string) error {
	return r.db.Model(&models.TenantMembership{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Update("role_slug", roleSlug).Error
}

func (r *gormTenantRepository) RemoveMember(tenantID, userID uint) error {
	return r.db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Delete(&models.TenantMembership{}).Error
}

func (r *gormTenantRepository) HasMembership(tenantID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.TenantMembership{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *gormTenantRepository) CountAdmins(tenantID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.TenantMembership{}).
		Where("tenant_id = ? AND role_slug = ?", tenantID, models.TenantRoleAdmin).
		Count(&count).Error
	return count, err
}

func (r *gormTenantRepository) UserExists(userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("id = ?", userID).Count(&count).Error
	return count > 0, err
}
