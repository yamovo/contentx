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

// TagRepository defines data-access operations for tags.
type TagRepository interface {
	List(filter TagListFilter) ([]models.Tag, int64, error)
	GetByID(id uint) (*models.Tag, error)
	FindByID(id uint) (*models.Tag, error) // returns gorm.ErrRecordNotFound if missing
	Create(tag *models.Tag) error
	UpdateFields(id uint, updates map[string]interface{}) error
	Delete(tag *models.Tag) error
	ClearArticleAssociations(tagID uint) error
	MergeTags(srcID, targetID uint) error // re-points article_tags from src to target, then deletes src rows
	CountArticleAssociations(tagID uint) (int64, error)
	UpdateCount(tagID uint, count int64) error
	DeleteByIDs(ids []uint) (rowsAffected int64, err error)
}

// gormTagRepository implements TagRepository with GORM.
type gormTagRepository struct {
	db *gorm.DB
}

// NewTagRepository builds a GORM-backed TagRepository.
func NewTagRepository(db *gorm.DB) TagRepository {
	return &gormTagRepository{db: db}
}

func (r *gormTagRepository) List(filter TagListFilter) ([]models.Tag, int64, error) {
	query := r.db.Model(&models.Tag{})
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

func (r *gormTagRepository) GetByID(id uint) (*models.Tag, error) {
	var tag models.Tag
	if err := r.db.First(&tag, id).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *gormTagRepository) FindByID(id uint) (*models.Tag, error) {
	var tag models.Tag
	if err := r.db.First(&tag, id).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *gormTagRepository) Create(tag *models.Tag) error {
	return r.db.Create(tag).Error
}

func (r *gormTagRepository) UpdateFields(id uint, updates map[string]interface{}) error {
	return r.db.Model(&models.Tag{}).Where("id = ?", id).Updates(updates).Error
}

func (r *gormTagRepository) Delete(tag *models.Tag) error {
	return r.db.Delete(tag).Error
}

func (r *gormTagRepository) ClearArticleAssociations(tagID uint) error {
	tag := models.Tag{}
	tag.ID = tagID
	return r.db.Model(&tag).Association("Articles").Clear()
}

// MergeTags re-points article_tags rows from srcID to targetID using
// SQLite's UPDATE OR IGNORE to avoid duplicate-key errors, then deletes
// any leftover src rows (the IGNORE may skip rows that would collide).
func (r *gormTagRepository) MergeTags(srcID, targetID uint) error {
	if err := r.db.Exec("UPDATE OR IGNORE article_tags SET tag_id = ? WHERE tag_id = ?", targetID, srcID).Error; err != nil {
		return err
	}
	return r.db.Exec("DELETE FROM article_tags WHERE tag_id = ?", srcID).Error
}

func (r *gormTagRepository) CountArticleAssociations(tagID uint) (int64, error) {
	var count int64
	if err := r.db.Table("article_tags").Where("tag_id = ?", tagID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *gormTagRepository) UpdateCount(tagID uint, count int64) error {
	return r.db.Model(&models.Tag{}).Where("id = ?", tagID).Update("count", count).Error
}

func (r *gormTagRepository) DeleteByIDs(ids []uint) (int64, error) {
	result := r.db.Where("id IN ?", ids).Delete(&models.Tag{})
	return result.RowsAffected, result.Error
}
