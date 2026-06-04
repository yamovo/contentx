package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gosimple/slug"
	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// TagHandler handles tag CRUD operations.
type TagHandler struct{ db *gorm.DB }

func NewTagHandler(db *gorm.DB) *TagHandler { return &TagHandler{db: db} }

// List returns all tags.
// GET /api/v1/tags?sort=count&limit=50
func (h *TagHandler) List(c *gin.Context) {
	sort := c.DefaultQuery("sort", "name")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))
	search := c.Query("search")

	query := h.db.Model(&models.Tag{})
	if search != "" {
		query = query.Where("name LIKE ?", "%"+search+"%")
	}

	switch sort {
	case "count":
		query = query.Order("count DESC")
	case "newest":
		query = query.Order("created_at DESC")
	default:
		query = query.Order("name ASC")
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	var tags []models.Tag
	query.Find(&tags)

	var total int64
	h.db.Model(&models.Tag{}).Count(&total)

	c.JSON(http.StatusOK, gin.H{"data": tags, "total": total})
}

// Get returns a single tag.
// GET /api/v1/tags/:id
func (h *TagHandler) Get(c *gin.Context) {
	var tag models.Tag
	if err := h.db.First(&tag, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tag})
}

// Create creates a new tag.
// POST /api/v1/tags
func (h *TagHandler) Create(c *gin.Context) {
	var req struct {
		Name  string `json:"name" binding:"required,max=64"`
		Slug  string `json:"slug"`
		Color string `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tag := models.Tag{Name: req.Name, Color: req.Color}
	if req.Slug != "" {
		tag.Slug = req.Slug
	} else {
		tag.Slug = slug.MakeLang(req.Name, "zh")
		if tag.Slug == "" {
			tag.Slug = slug.Make(req.Name)
		}
	}

	if err := h.db.Create(&tag).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Tag already exists"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": tag})
}

// Update updates a tag.
// PUT /api/v1/tags/:id
func (h *TagHandler) Update(c *gin.Context) {
	var tag models.Tag
	if err := h.db.First(&tag, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}

	var req struct {
		Name  string `json:"name"`
		Slug  string `json:"slug"`
		Color string `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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
	h.db.Model(&tag).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"data": tag})
}

// Delete removes a tag.
// DELETE /api/v1/tags/:id
func (h *TagHandler) Delete(c *gin.Context) {
	var tag models.Tag
	if err := h.db.First(&tag, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}
	// Remove associations.
	h.db.Model(&tag).Association("Articles").Clear()
	h.db.Delete(&tag)
	c.JSON(http.StatusOK, gin.H{"message": "Tag deleted"})
}

// Merge merges multiple tags into one target.
// POST /api/v1/tags/merge
func (h *TagHandler) Merge(c *gin.Context) {
	var req struct {
		SourceIDs []uint `json:"source_ids" binding:"required"`
		TargetID  uint   `json:"target_id" binding:"required"`
		DeleteOld bool   `json:"delete_old"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var target models.Tag
	if err := h.db.First(&target, req.TargetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Target tag not found"})
		return
	}

	// Re-point article_tags.
	for _, srcID := range req.SourceIDs {
		if srcID == req.TargetID {
			continue
		}
		h.db.Exec("UPDATE OR IGNORE article_tags SET tag_id = ? WHERE tag_id = ?",
			req.TargetID, srcID)
		h.db.Exec("DELETE FROM article_tags WHERE tag_id = ?", srcID)
	}

	// Recalculate count.
	h.db.Model(&target).Update("count",
		h.db.Model(&models.Tag{}).Select("COUNT(*) FROM article_tags WHERE tag_id = ?", target.ID))

	if req.DeleteOld {
		h.db.Where("id IN ?", req.SourceIDs).Delete(&models.Tag{})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tags merged"})
}
