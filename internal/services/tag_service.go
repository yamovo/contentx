package services

import (
	"errors"

	"github.com/gosimple/slug"
	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// TagListParams holds query parameters for listing tags.
type TagListParams struct {
	Sort   string
	Limit  int
	Search string
}

// CreateTagRequest holds data for creating a tag.
type CreateTagRequest struct {
	Name  string `json:"name" binding:"required,max=64"`
	Slug  string `json:"slug"`
	Color string `json:"color"`
}

// UpdateTagRequest holds data for updating a tag.
type UpdateTagRequest struct {
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Color string `json:"color"`
}

// TagService handles tag business logic.
type TagService struct {
	db *gorm.DB
}

// NewTagService creates a new TagService.
func NewTagService(db *gorm.DB) *TagService {
	return &TagService{db: db}
}

// List returns tags with optional sorting, limit, and search.
func (s *TagService) List(params TagListParams) ([]models.Tag, int64, error) {
	query := s.db.Model(&models.Tag{})
	if params.Search != "" {
		query = query.Where("name LIKE ?", "%"+params.Search+"%")
	}

	switch params.Sort {
	case "count":
		query = query.Order("count DESC")
	case "newest":
		query = query.Order("created_at DESC")
	default:
		query = query.Order("name ASC")
	}
	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}

	var total int64
	if err := s.db.Model(&models.Tag{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tags []models.Tag
	if err := query.Find(&tags).Error; err != nil {
		return nil, 0, err
	}

	return tags, total, nil
}

// Get returns a single tag by ID.
func (s *TagService) Get(id uint) (*models.Tag, error) {
	var tag models.Tag
	if err := s.db.First(&tag, id).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

// Create creates a new tag.
func (s *TagService) Create(req CreateTagRequest) (*models.Tag, error) {
	tag := models.Tag{Name: req.Name, Color: req.Color}
	if req.Slug != "" {
		tag.Slug = req.Slug
	} else {
		tag.Slug = slug.MakeLang(req.Name, "zh")
		if tag.Slug == "" {
			tag.Slug = slug.Make(req.Name)
		}
	}

	if err := s.db.Create(&tag).Error; err != nil {
		return nil, err
	}

	return &tag, nil
}

// Update updates a tag's fields.
func (s *TagService) Update(id uint, req UpdateTagRequest) error {
	var tag models.Tag
	if err := s.db.First(&tag, id).Error; err != nil {
		return errors.New("tag not found")
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Slug != "" {
		updates["slug"] = req.Slug
	}
	if req.Color != "" {
		updates["color"] = req.Color
	}

	return s.db.Model(&tag).Updates(updates).Error
}

// Delete removes a tag and clears its article associations.
func (s *TagService) Delete(id uint) error {
	var tag models.Tag
	if err := s.db.First(&tag, id).Error; err != nil {
		return errors.New("tag not found")
	}

	// Remove associations.
	s.db.Model(&tag).Association("Articles").Clear()
	return s.db.Delete(&tag).Error
}

// Merge merges source tags into a target tag, optionally deleting the source tags.
func (s *TagService) Merge(sourceIDs []uint, targetID uint, deleteOld bool) error {
	var target models.Tag
	if err := s.db.First(&target, targetID).Error; err != nil {
		return errors.New("target tag not found")
	}

	// Re-point article_tags from sources to target.
	for _, srcID := range sourceIDs {
		if srcID == targetID {
			continue
		}
		s.db.Exec("UPDATE OR IGNORE article_tags SET tag_id = ? WHERE tag_id = ?",
			targetID, srcID)
		s.db.Exec("DELETE FROM article_tags WHERE tag_id = ?", srcID)
	}

	// Recalculate count.
	s.db.Model(&target).Update("count",
		s.db.Model(&models.Tag{}).Select("COUNT(*) FROM article_tags WHERE tag_id = ?", target.ID))

	if deleteOld {
		s.db.Where("id IN ?", sourceIDs).Delete(&models.Tag{})
	}

	return nil
}
