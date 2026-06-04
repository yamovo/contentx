package services

import (
	"errors"
	"strconv"

	"github.com/gosimple/slug"
	"github.com/vortexcms/go-cms/internal/models"
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
	db *gorm.DB
}

// NewCategoryService creates a new CategoryService.
func NewCategoryService(db *gorm.DB) *CategoryService {
	return &CategoryService{db: db}
}

// List returns all categories in a tree structure.
func (s *CategoryService) List(showAll bool) ([]models.Category, error) {
	query := s.db.Model(&models.Category{})
	if !showAll {
		query = query.Where("is_active = ?", true)
	}

	var categories []models.Category
	if err := query.Order("sort_order ASC, name ASC").Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

// Get returns a single category by ID with parent and children loaded.
func (s *CategoryService) Get(id uint) (*models.Category, error) {
	var category models.Category
	if err := s.db.First(&category, id).Error; err != nil {
		return nil, err
	}

	// Load parent and children.
	s.db.Model(&category).Association("Parent")
	s.db.Where("parent_id = ?", category.ID).Order("sort_order ASC").Find(&category.Children)

	return &category, nil
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
		category.Slug = slug.MakeLang(req.Name, "zh")
		if category.Slug == "" {
			category.Slug = slug.Make(req.Name)
		}
	}

	category.Slug = s.ensureUniqueSlug(category.Slug, 0)

	if err := s.db.Create(&category).Error; err != nil {
		return nil, err
	}

	return &category, nil
}

// Update updates an existing category.
func (s *CategoryService) Update(id uint, req CreateCategoryRequest) error {
	var category models.Category
	if err := s.db.First(&category, id).Error; err != nil {
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
		updates["slug"] = s.ensureUniqueSlug(req.Slug, category.ID)
	} else if req.Name != category.Name {
		newSlug := slug.MakeLang(req.Name, "zh")
		if newSlug == "" {
			newSlug = slug.Make(req.Name)
		}
		updates["slug"] = s.ensureUniqueSlug(newSlug, category.ID)
	}

	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	return s.db.Model(&category).Updates(updates).Error
}

// Delete removes a category, moving its articles and children.
func (s *CategoryService) Delete(id uint) error {
	var category models.Category
	if err := s.db.First(&category, id).Error; err != nil {
		return errors.New("category not found")
	}

	// Move articles to uncategorized (null).
	s.db.Model(&models.Article{}).Where("category_id = ?", category.ID).Update("category_id", nil)
	// Move children to root.
	s.db.Model(&models.Category{}).Where("parent_id = ?", category.ID).Update("parent_id", nil)

	return s.db.Delete(&category).Error
}

// Reorder updates sort order (and optionally parent) for multiple categories.
func (s *CategoryService) Reorder(items []ReorderItem) error {
	for _, item := range items {
		updates := map[string]interface{}{"sort_order": item.SortOrder}
		if item.ParentID != nil {
			updates["parent_id"] = item.ParentID
		}
		if err := s.db.Model(&models.Category{}).Where("id = ?", item.ID).Updates(updates).Error; err != nil {
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

// ensureUniqueSlug generates a unique slug by appending a counter if needed.
func (s *CategoryService) ensureUniqueSlug(original string, excludeID uint) string {
	candidate := original
	for i := 1; ; i++ {
		var count int64
		query := s.db.Model(&models.Category{}).Where("slug = ?", candidate)
		if excludeID > 0 {
			query = query.Where("id != ?", excludeID)
		}
		query.Count(&count)
		if count == 0 {
			return candidate
		}
		candidate = original + "-" + strconv.Itoa(i)
	}
}
