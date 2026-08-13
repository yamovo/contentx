package repository

import (
	"fmt"
	"strconv"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// CategoryRepository defines data-access operations for categories, scoped to
// a tenant (RFC-001 §5): queries MUST carry tenantID.
type CategoryRepository interface {
	List(showAll bool, tenantID uint) ([]models.Category, error)
	GetByID(id, tenantID uint) (*models.Category, error) // loads Parent association + Children
	FindByID(id, tenantID uint) (*models.Category, error)
	Create(category *models.Category) error // category.TenantID must be set by the caller
	UpdateFields(id uint, updates map[string]interface{}, tenantID uint) error
	UpdateSortOrder(id uint, sortOrder int, parentID *uint, tenantID uint) error
	Delete(id, tenantID uint) error // nulls articles' category_id and children's parent_id, then deletes
	EnsureUniqueSlug(original string, excludeID, tenantID uint) (string, error)
}

// gormCategoryRepository implements CategoryRepository with GORM.
type gormCategoryRepository struct {
	db *gorm.DB
}

// NewCategoryRepository builds a GORM-backed CategoryRepository.
func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &gormCategoryRepository{db: db}
}

func (r *gormCategoryRepository) List(showAll bool, tenantID uint) ([]models.Category, error) {
	query := r.db.Model(&models.Category{}).Where("tenant_id = ?", tenantID)
	if !showAll {
		query = query.Where("is_active = ?", true)
	}

	var categories []models.Category
	if err := query.Order("sort_order ASC, name ASC").Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *gormCategoryRepository) GetByID(id, tenantID uint) (*models.Category, error) {
	var category models.Category
	if err := r.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&category).Error; err != nil {
		return nil, err
	}

	// Load parent and children (mirrors prior service behaviour).
	if category.ParentID != nil {
		r.db.Where("id = ? AND tenant_id = ?", *category.ParentID, tenantID).First(&category.Parent)
	}
	r.db.Where("parent_id = ? AND tenant_id = ?", category.ID, tenantID).Order("sort_order ASC").Find(&category.Children)

	return &category, nil
}

func (r *gormCategoryRepository) FindByID(id, tenantID uint) (*models.Category, error) {
	var category models.Category
	if err := r.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *gormCategoryRepository) Create(category *models.Category) error {
	return r.db.Create(category).Error
}

func (r *gormCategoryRepository) UpdateFields(id uint, updates map[string]interface{}, tenantID uint) error {
	return r.db.Model(&models.Category{}).Where("id = ? AND tenant_id = ?", id, tenantID).Updates(updates).Error
}

func (r *gormCategoryRepository) UpdateSortOrder(id uint, sortOrder int, parentID *uint, tenantID uint) error {
	updates := map[string]interface{}{"sort_order": sortOrder}
	if parentID != nil {
		updates["parent_id"] = parentID
	}
	return r.db.Model(&models.Category{}).Where("id = ? AND tenant_id = ?", id, tenantID).Updates(updates).Error
}

func (r *gormCategoryRepository) Delete(id, tenantID uint) error {
	var category models.Category
	if err := r.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&category).Error; err != nil {
		return err
	}

	// Move articles to uncategorized (null).
	r.db.Model(&models.Article{}).Where("category_id = ? AND tenant_id = ?", category.ID, tenantID).Update("category_id", nil)
	// Move children to root.
	r.db.Model(&models.Category{}).Where("parent_id = ? AND tenant_id = ?", category.ID, tenantID).Update("parent_id", nil)

	return r.db.Delete(&category).Error
}

// EnsureUniqueSlug generates a unique slug by appending a counter if needed.
// Returns an error if the underlying COUNT query fails or if no unique slug
// can be found within maxSlugAttempts iterations (A-2 fix).
func (r *gormCategoryRepository) EnsureUniqueSlug(original string, excludeID, tenantID uint) (string, error) {
	candidate := original
	for i := 1; i <= maxSlugAttempts; i++ {
		var count int64
		query := r.db.Model(&models.Category{}).Where("slug = ? AND tenant_id = ?", candidate, tenantID)
		if excludeID > 0 {
			query = query.Where("id != ?", excludeID)
		}
		if err := query.Count(&count).Error; err != nil {
			return "", fmt.Errorf("ensure unique slug: %w", err)
		}
		if count == 0 {
			return candidate, nil
		}
		candidate = original + "-" + strconv.Itoa(i)
	}
	return "", fmt.Errorf("ensure unique slug: no unique slug for %q after %d attempts", original, maxSlugAttempts)
}
