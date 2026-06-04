package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/vortexcms/go-cms/internal/middleware"
	"github.com/vortexcms/go-cms/internal/services"
)

// MediaHandler handles media upload and management.
type MediaHandler struct {
	svc *services.MediaService
}

func NewMediaHandler(svc *services.MediaService) *MediaHandler {
	return &MediaHandler{svc: svc}
}

// List returns media files with pagination and filters.
// GET /api/v1/media?type=image&folder=/&search=test&page=1
func (h *MediaHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	params := services.MediaListParams{
		Page:     page,
		PageSize: pageSize,
		MimeType: c.Query("type"),
		Folder:   c.DefaultQuery("folder", ""),
		Search:   c.Query("search"),
		Sort:     c.DefaultQuery("sort", "newest"),
	}

	media, total, err := h.svc.List(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch media"})
		return
	}

	paginate := paginateFrom(page, pageSize, total)
	c.JSON(http.StatusOK, listResponse(media, paginate))
}

// Upload handles file upload.
// POST /api/v1/media/upload
func (h *MediaHandler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	defer file.Close()

	user := middleware.GetCurrentUser(c)

	media, err := h.svc.Upload(file, header,
		c.PostForm("folder"),
		c.PostForm("alt"),
		c.PostForm("title"),
		c.PostForm("caption"),
		c.PostForm("description"),
		user.ID,
	)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": media})
}

// Get returns a single media item.
// GET /api/v1/media/:id
func (h *MediaHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid media ID"})
		return
	}

	media, err := h.svc.Get(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Media not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": media})
}

// Update updates media metadata (alt, title, caption, etc.).
// PUT /api/v1/media/:id
func (h *MediaHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid media ID"})
		return
	}

	var req services.UpdateMediaRequest
	c.ShouldBindJSON(&req)

	if err := h.svc.Update(uint(id), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update media"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Media updated"})
}

// Delete removes a media file.
// DELETE /api/v1/media/:id
func (h *MediaHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid media ID"})
		return
	}

	if err := h.svc.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete media"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Media deleted"})
}

// BulkDelete removes multiple media files.
// POST /api/v1/media/bulk-delete
func (h *MediaHandler) BulkDelete(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	affected, err := h.svc.BulkDelete(req.IDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete media"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted", "affected": affected})
}

// Folders returns all unique folders.
// GET /api/v1/media/folders
func (h *MediaHandler) Folders(c *gin.Context) {
	folders, err := h.svc.Folders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch folders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": folders})
}

// Stats returns media library statistics.
// GET /api/v1/media/stats
func (h *MediaHandler) Stats(c *gin.Context) {
	stats, err := h.svc.Stats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
}
