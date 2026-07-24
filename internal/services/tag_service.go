package services

import (
	"errors"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
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
	repo repository.TagRepository
}

// NewTagService creates a new TagService backed by a GORM repository.
// Kept for backward compatibility with existing callers and tests.
func NewTagService(db *gorm.DB) *TagService {
	return &TagService{repo: repository.NewTagRepository(db)}
}

// NewTagServiceWithRepo builds a TagService with an explicit repository,
// enabling unit tests to inject mocks.
func NewTagServiceWithRepo(repo repository.TagRepository) *TagService {
	return &TagService{repo: repo}
}

// List returns tags with optional sorting, limit, and search.
func (s *TagService) List(params TagListParams) ([]models.Tag, int64, error) {
	return s.repo.List(repository.TagListFilter{
		Sort:   params.Sort,
		Limit:  params.Limit,
		Search: params.Search,
	})
}

// Get returns a single tag by ID.
func (s *TagService) Get(id uint) (*models.Tag, error) {
	return s.repo.GetByID(id)
}

// Create creates a new tag.
func (s *TagService) Create(req CreateTagRequest) (*models.Tag, error) {
	tag := models.Tag{Name: req.Name, Color: req.Color}
	if req.Slug != "" {
		tag.Slug = req.Slug
	} else {
		tag.Slug = models.GenerateSlug(req.Name)
	}

	if err := s.repo.Create(&tag); err != nil {
		return nil, err
	}

	return &tag, nil
}

// Update updates a tag's fields.
func (s *TagService) Update(id uint, req UpdateTagRequest) error {
	if _, err := s.repo.FindByID(id); err != nil {
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

	return s.repo.UpdateFields(id, updates)
}

// Delete removes a tag and clears its article associations.
func (s *TagService) Delete(id uint) error {
	tag, err := s.repo.FindByID(id)
	if err != nil {
		return errors.New("tag not found")
	}

	// Remove associations (best-effort, mirrors prior behaviour).
	_ = s.repo.ClearArticleAssociations(tag.ID)
	return s.repo.Delete(tag)
}

// Merge merges source tags into a target tag, optionally deleting the source tags.
func (s *TagService) Merge(sourceIDs []uint, targetID uint, deleteOld bool) error {
	if _, err := s.repo.FindByID(targetID); err != nil {
		return errors.New("target tag not found")
	}

	// Re-point article_tags from sources to target (best-effort, mirrors prior behaviour).
	for _, srcID := range sourceIDs {
		if srcID == targetID {
			continue
		}
		_ = s.repo.MergeTags(srcID, targetID)
	}

	// Recalculate count using subquery.
	count, err := s.repo.CountArticleAssociations(targetID)
	if err != nil {
		return err
	}
	_ = s.repo.UpdateCount(targetID, count)

	if deleteOld {
		_, _ = s.repo.DeleteByIDs(sourceIDs)
	}

	return nil
}
