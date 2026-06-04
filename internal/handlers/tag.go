package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/vortexcms/go-cms/internal/services"
)

// TagHandler handles tag CRUD operations.
type TagHandler struct {
	svc *services.TagService
}

func NewTagHandler(svc *services.TagService) *TagHandler {
	return &TagHandler{svc: svc}
}

// List returns all tags.
// GET /api/v1/tags?sort=count&limit=50
func (h *TagHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))

	params := services.TagListParams{
		Sort:   c.DefaultQuery("sort", "name"),
		Limit:  limit,
		Search: c.Query("search"),
	}

	tags, total, err := h.svc.List(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tags"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tags, "total": total})
}

// Get returns a single tag.
// GET /api/v1/tags/:id
func (h *TagHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}

	tag, err := h.svc.Get(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tag})
}

// Create creates a new tag.
// POST /api/v1/tags
func (h *TagHandler) Create(c *gin.Context) {
	var req services.CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tag, err := h.svc.Create(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Tag already exists"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": tag})
}

// Update updates a tag.
// PUT /api/v1/tags/:id
func (h *TagHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}

	var req services.UpdateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.Update(uint(id), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update tag"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tag updated"})
}

// Delete removes a tag.
// DELETE /api/v1/tags/:id
func (h *TagHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tag ID"})
		return
	}

	if err := h.svc.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete tag"})
		return
	}

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

	if err := h.svc.Merge(req.SourceIDs, req.TargetID, req.DeleteOld); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tags merged"})
}
