package repository

import (
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// TagListFilter holds query parameters for listing tags.
type TagListFilter struct {
	Sort   string
	Limit  int
	Search string
}

// TagRepository defines data-access operations for tags, scoped to a tenant
// (RFC-001 §5): queries MUST carry tenantID.
type TagRepository interface {
	List(filter TagListFilter, tenantID uint) ([]models.Tag, int64, error)
	GetByID(id, tenantID uint) (*models.Tag, error)
	FindByID(id, tenantID uint) (*models.Tag, error) // returns gorm.ErrRecordNotFound if missing
	Create(tag *models.Tag) error                    // tag.TenantID must be set by the caller
	UpdateFields(id uint, updates map[string]interface{}, tenantID uint) error
	Delete(tag *models.Tag, tenantID uint) error
	ClearArticleAssociations(tagID, tenantID uint) error
	MergeTags(srcID, targetID, tenantID uint) error // re-points article_tags from src to target, then deletes src rows
	CountArticleAssociations(tagID, tenantID uint) (int64, error)
	UpdateCount(tagID uint, count int64, tenantID uint) error
	DeleteByIDs(ids []uint, tenantID uint) (rowsAffected int64, err error)
}

// gormTagRepository implements TagRepository with GORM.
type gormTagRepository struct {
	db *gorm.DB
}

// NewTagRepository builds a GORM-backed TagRepository.
func NewTagRepository(db *gorm.DB) TagRepository {
	return &gormTagRepository{db: db}
}

func (r *gormTagRepository) List(filter TagListFilter, tenantID uint) ([]models.Tag, int64, error) {
	query := r.db.Model(&models.Tag{}).Where("tenant_id = ?", tenantID)
	if filter.Search != "" {
		query = query.Where("name LIKE ?", "%"+filter.Search+"%")
	}

	switch filter.Sort {
	case "count":
		query = query.Order("count DESC")
	case "newest":
		query = query.Order("created_at DESC")
	default:
		query = query.Order("name ASC")
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tags []models.Tag
	if err := query.Find(&tags).Error; err != nil {
		return nil, 0, err
	}

	return tags, total, nil
}

func (r *gormTagRepository) GetByID(id, tenantID uint) (*models.Tag, error) {
	var tag models.Tag
	if err := r.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&tag).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *gormTagRepository) FindByID(id, tenantID uint) (*models.Tag, error) {
	var tag models.Tag
	if err := r.db.Where("id = ? AND tenant_id = ?", id, tenantID).First(&tag).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *gormTagRepository) Create(tag *models.Tag) error {
	return r.db.Create(tag).Error
}

func (r *gormTagRepository) UpdateFields(id uint, updates map[string]interface{}, tenantID uint) error {
	return r.db.Model(&models.Tag{}).Where("id = ? AND tenant_id = ?", id, tenantID).Updates(updates).Error
}

func (r *gormTagRepository) Delete(tag *models.Tag, tenantID uint) error {
	return r.db.Where("id = ? AND tenant_id = ?", tag.ID, tenantID).Delete(&models.Tag{}).Error
}

// ClearArticleAssociations clears article_tags rows for a tag. article_tags
// carries no tenant column (implicitly scoped through the article, RFC-001
// §12); the tag itself is verified to belong to the tenant first.
func (r *gormTagRepository) ClearArticleAssociations(tagID, tenantID uint) error {
	var tag models.Tag
	if err := r.db.Where("id = ? AND tenant_id = ?", tagID, tenantID).First(&tag).Error; err != nil {
		return err
	}
	return r.db.Model(&tag).Association("Articles").Clear()
}

// MergeTags re-points article_tags rows from srcID to targetID using
// SQLite's UPDATE OR IGNORE to avoid duplicate-key errors, then deletes
// any leftover src rows (the IGNORE may skip rows that would collide).
// Both tags are verified to belong to the tenant first.
func (r *gormTagRepository) MergeTags(srcID, targetID, tenantID uint) error {
	var src, target models.Tag
	if err := r.db.Where("id = ? AND tenant_id = ?", srcID, tenantID).First(&src).Error; err != nil {
		return err
	}
	if err := r.db.Where("id = ? AND tenant_id = ?", targetID, tenantID).First(&target).Error; err != nil {
		return err
	}
	if err := r.db.Exec("UPDATE OR IGNORE article_tags SET tag_id = ? WHERE tag_id = ?", targetID, srcID).Error; err != nil {
		return err
	}
	return r.db.Exec("DELETE FROM article_tags WHERE tag_id = ?", srcID).Error
}

func (r *gormTagRepository) CountArticleAssociations(tagID, tenantID uint) (int64, error) {
	var tag models.Tag
	if err := r.db.Where("id = ? AND tenant_id = ?", tagID, tenantID).First(&tag).Error; err != nil {
		return 0, err
	}
	var count int64
	if err := r.db.Table("article_tags").Where("tag_id = ?", tagID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *gormTagRepository) UpdateCount(tagID uint, count int64, tenantID uint) error {
	return r.db.Model(&models.Tag{}).Where("id = ? AND tenant_id = ?", tagID, tenantID).Update("count", count).Error
}

func (r *gormTagRepository) DeleteByIDs(ids []uint, tenantID uint) (int64, error) {
	result := r.db.Where("id IN ? AND tenant_id = ?", ids, tenantID).Delete(&models.Tag{})
	return result.RowsAffected, result.Error
}
