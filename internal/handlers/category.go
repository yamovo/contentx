package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gosimple/slug"
	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// CategoryHandler handles category CRUD operations.
type CategoryHandler struct {
	db *gorm.DB
}

func NewCategoryHandler(db *gorm.DB) *CategoryHandler {
	return &CategoryHandler{db: db}
}

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

// List returns all categories in a tree structure.
// GET /api/v1/categories
func (h *CategoryHandler) List(c *gin.Context) {
	var categories []models.Category
	query := h.db.Model(&models.Category{})

	if show := c.Query("all"); show != "true" {
		query = query.Where("is_active = ?", true)
	}

	query.Order("sort_order ASC, name ASC").Find(&categories)

	// Build tree.
	tree := buildCategoryTree(categories, nil)
	c.JSON(http.StatusOK, gin.H{"data": tree})
}

// Get returns a single category.
// GET /api/v1/categories/:id
func (h *CategoryHandler) Get(c *gin.Context) {
	var category models.Category
	if err := h.db.First(&category, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	// Load parent and children.
	h.db.Model(&category).Association("Parent")
	h.db.Where("parent_id = ?", category.ID).Order("sort_order ASC").Find(&category.Children)

	c.JSON(http.StatusOK, gin.H{"data": category})
}

// Create creates a new category.
// POST /api/v1/categories
func (h *CategoryHandler) Create(c *gin.Context) {
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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

	// Ensure unique slug.
	category.Slug = ensureUniqueCategorySlug(h.db, category.Slug, 0)

	if err := h.db.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": category})
}

// Update updates an existing category.
// PUT /api/v1/categories/:id
func (h *CategoryHandler) Update(c *gin.Context) {
	var category models.Category
	if err := h.db.First(&category, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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
		updates["slug"] = ensureUniqueCategorySlug(h.db, req.Slug, category.ID)
	} else if req.Name != category.Name {
		newSlug := slug.MakeLang(req.Name, "zh")
		if newSlug == "" {
			newSlug = slug.Make(req.Name)
		}
		updates["slug"] = ensureUniqueCategorySlug(h.db, newSlug, category.ID)
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	h.db.Model(&category).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"data": category})
}

// Delete removes a category.
// DELETE /api/v1/categories/:id
func (h *CategoryHandler) Delete(c *gin.Context) {
	var category models.Category
	if err := h.db.First(&category, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	// Move articles to uncategorized (null).
	h.db.Model(&models.Article{}).Where("category_id = ?", category.ID).Update("category_id", nil)
	// Move children to root.
	h.db.Model(&models.Category{}).Where("parent_id = ?", category.ID).Update("parent_id", nil)

	h.db.Delete(&category)
	c.JSON(http.StatusOK, gin.H{"message": "Category deleted"})
}

// Reorder updates sort order for multiple categories.
// PUT /api/v1/categories/reorder
func (h *CategoryHandler) Reorder(c *gin.Context) {
	var req struct {
		Items []struct {
			ID        uint `json:"id"`
			SortOrder int  `json:"sort_order"`
			ParentID  *uint `json:"parent_id"`
		} `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, item := range req.Items {
		updates := map[string]interface{}{"sort_order": item.SortOrder}
		if item.ParentID != nil {
			updates["parent_id"] = item.ParentID
		}
		h.db.Model(&models.Category{}).Where("id = ?", item.ID).Updates(updates)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Categories reordered"})
}

// Helper: build a tree from flat category list.
type CategoryTree struct {
	models.Category
	Children []CategoryTree `json:"children"`
}

func buildCategoryTree(categories []models.Category, parentID *uint) []CategoryTree {
	var tree []CategoryTree
	for _, cat := range categories {
		if (parentID == nil && cat.ParentID == nil) ||
			(parentID != nil && cat.ParentID != nil && *parentID == *cat.ParentID) {
			node := CategoryTree{Category: cat}
			node.Children = buildCategoryTree(categories, &cat.ID)
			tree = append(tree, node)
		}
	}
	return tree
}

func ensureUniqueCategorySlug(db *gorm.DB, s string, excludeID uint) string {
	original := s
	for i := 1; ; i++ {
		var count int64
		query := db.Model(&models.Category{}).Where("slug = ?", s)
		if excludeID > 0 {
			query = query.Where("id != ?", excludeID)
		}
		query.Count(&count)
		if count == 0 {
			return s
		}
		s = original + "-" + strconv.Itoa(i)
	}
}
