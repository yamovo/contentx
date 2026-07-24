package services

import (
	"errors"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
	"gorm.io/gorm"
)

// CreateCategoryRequest holds data for creating or updating a category.
type CreateCategoryRequest struct {
	Name        string `json:"name" binding:"required,max=128"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	ParentID    *uint  `json:"parent_id"`
	Image       string `json:"image"`
	Color       string `json:"color"`
	SortOrder   int    `json:"sort_order"`
	IsActive    *bool  `json:"is_active"`
	MetaTitle   string `json:"meta_title"`
	MetaDesc    string `json:"meta_desc"`
}

// ReorderItem holds a single item in a reorder request.
type ReorderItem struct {
	ID        uint  `json:"id"`
	SortOrder int   `json:"sort_order"`
	ParentID  *uint `json:"parent_id"`
}

// CategoryTree wraps a Category with nested children for tree output.
type CategoryTree struct {
	models.Category
	Children []CategoryTree `json:"children"`
}

// CategoryService handles category business logic.
type CategoryService struct {
	repo repository.CategoryRepository
}

// NewCategoryService creates a new CategoryService backed by a GORM repository.
// Kept for backward compatibility with existing callers and tests.
func NewCategoryService(db *gorm.DB) *CategoryService {
	return &CategoryService{repo: repository.NewCategoryRepository(db)}
}

// NewCategoryServiceWithRepo builds a CategoryService with an explicit repository,
// enabling unit tests to inject mocks.
func NewCategoryServiceWithRepo(repo repository.CategoryRepository) *CategoryService {
	return &CategoryService{repo: repo}
}

// List returns all categories in a tree structure.
func (s *CategoryService) List(showAll bool) ([]models.Category, error) {
	return s.repo.List(showAll)
}

// Get returns a single category by ID with parent and children loaded.
func (s *CategoryService) Get(id uint) (*models.Category, error) {
	return s.repo.GetByID(id)
}

// Create creates a new category.
func (s *CategoryService) Create(req CreateCategoryRequest) (*models.Category, error) {
	category := models.Category{
		Name:        req.Name,
		Description: req.Description,
		ParentID:    req.ParentID,
		Image:       req.Image,
		Color:       req.Color,
		SortOrder:   req.SortOrder,
		MetaTitle:   req.MetaTitle,
		MetaDesc:    req.MetaDesc,
		IsActive:    true,
	}
	if req.IsActive != nil {
		category.IsActive = *req.IsActive
	}

	if req.Slug != "" {
		category.Slug = req.Slug
	} else {
		category.Slug = models.GenerateSlug(req.Name)
	}

	uniqueSlug, err := s.repo.EnsureUniqueSlug(category.Slug, 0)
	if err != nil {
		return nil, err
	}
	category.Slug = uniqueSlug

	if err := s.repo.Create(&category); err != nil {
		return nil, err
	}

	return &category, nil
}

// Update updates an existing category.
func (s *CategoryService) Update(id uint, req CreateCategoryRequest) error {
	category, err := s.repo.FindByID(id)
	if err != nil {
		return errors.New("category not found")
	}

	updates := map[string]interface{}{
		"name":        req.Name,
		"description": req.Description,
		"parent_id":   req.ParentID,
		"image":       req.Image,
		"color":       req.Color,
		"sort_order":  req.SortOrder,
		"meta_title":  req.MetaTitle,
		"meta_desc":   req.MetaDesc,
	}

	if req.Slug != "" {
		uniqueSlug, err := s.repo.EnsureUniqueSlug(req.Slug, category.ID)
		if err != nil {
			return err
		}
		updates["slug"] = uniqueSlug
	} else if req.Name != category.Name {
		newSlug := models.GenerateSlug(req.Name)
		uniqueSlug, err := s.repo.EnsureUniqueSlug(newSlug, category.ID)
		if err != nil {
			return err
		}
		updates["slug"] = uniqueSlug
	}

	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	return s.repo.UpdateFields(id, updates)
}

// Delete removes a category, moving its articles and children.
func (s *CategoryService) Delete(id uint) error {
	return s.repo.Delete(id)
}

// Reorder updates sort order (and optionally parent) for multiple categories.
func (s *CategoryService) Reorder(items []ReorderItem) error {
	for _, item := range items {
		if err := s.repo.UpdateSortOrder(item.ID, item.SortOrder, item.ParentID); err != nil {
			return err
		}
	}
	return nil
}

// BuildCategoryTree converts a flat category list into a nested tree structure.
func BuildCategoryTree(categories []models.Category, parentID *uint) []CategoryTree {
	var tree []CategoryTree
	for _, cat := range categories {
		if (parentID == nil && cat.ParentID == nil) ||
			(parentID != nil && cat.ParentID != nil && *parentID == *cat.ParentID) {
			node := CategoryTree{Category: cat}
			node.Children = BuildCategoryTree(categories, &cat.ID)
			tree = append(tree, node)
		}
	}
	return tree
}
