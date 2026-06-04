package handlers

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vortexcms/go-cms/internal/config"
	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// MediaHandler handles media upload and management.
type MediaHandler struct {
	db  *gorm.DB
	cfg config.UploadConfig
}

func NewMediaHandler(db *gorm.DB, cfg config.UploadConfig) *MediaHandler {
	return &MediaHandler{db: db, cfg: cfg}
}

// List returns media files with pagination and filters.
// GET /api/v1/media?type=image&folder=/&search=test&page=1
func (h *MediaHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	mimeType := c.Query("type") // image, video, audio, application
	folder := c.DefaultQuery("folder", "")
	search := c.Query("search")
	sort := c.DefaultQuery("sort", "newest")

	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }

	query := h.db.Model(&models.Media{}).Preload("Uploader")
	if mimeType != "" {
		query = query.Where("mime_type LIKE ?", mimeType+"%")
	}
	if folder != "" {
		query = query.Where("folder = ?", folder)
	}
	if search != "" {
		query = query.Where("filename LIKE ? OR original_name LIKE ? OR alt LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	switch sort {
	case "oldest": query = query.Order("created_at ASC")
	case "name": query = query.Order("filename ASC")
	case "size": query = query.Order("file_size DESC")
	default: query = query.Order("created_at DESC")
	}

	var total int64
	query.Count(&total)

	var media []models.Media
	query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&media)

	paginate := models.Paginate{Page: page, PageSize: pageSize, Total: total}
	c.JSON(http.StatusOK, models.NewListResponse(media, paginate))
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

	// Validate size.
	if header.Size > h.cfg.MaxSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("File too large. Max size: %d bytes", h.cfg.MaxSize),
		})
		return
	}

	// Validate type.
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	mimeType := http.DetectContentType(buf[:n])
	file.Seek(0, 0)

	allowed := false
	for _, t := range h.cfg.AllowedTypes {
		if strings.HasPrefix(mimeType, t) || t == mimeType {
			allowed = true
			break
		}
	}
	if !allowed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File type not allowed: " + mimeType})
		return
	}

	// Generate unique filename.
	ext := filepath.Ext(header.Filename)
	folder := c.PostForm("folder")
	if folder == "" {
		folder = "/" + time.Now().Format("2006/01")
	}
	// Sanitize folder path to prevent directory traversal.
	folder = filepath.Clean("/" + folder)
	if strings.Contains(folder, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid folder path"})
		return
	}

	// Create directory.
	uploadDir := filepath.Join(h.cfg.StoragePath, folder)
	os.MkdirAll(uploadDir, 0755)

	// Generate unique name.
	hash := md5.New()
	fmt.Fprintf(hash, "%s-%d-%s", header.Filename, time.Now().UnixNano(), header.Filename)
	filename := fmt.Sprintf("%x%s", hash.Sum(nil), ext)
	filePath := filepath.Join(uploadDir, filename)

	// Save file.
	dst, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	defer dst.Close()

	size, err := io.Copy(dst, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write file"})
		return
	}

	// Calculate checksum.
	file.Seek(0, 0)
	checksum := md5.New()
	io.Copy(checksum, file)

	url := h.cfg.URLPrefix + "/" + folder + "/" + filename

	user := getCurrentUserFromDB(h.db, c)

	media := models.Media{
		Filename:     filename,
		OriginalName: header.Filename,
		FilePath:     filePath,
		URL:          url,
		MimeType:     mimeType,
		FileSize:     size,
		Folder:       folder,
		UploaderID:   user.ID,
		Checksum:     fmt.Sprintf("%x", checksum.Sum(nil)),
		Alt:          c.PostForm("alt"),
		Title:        c.PostForm("title"),
		Caption:      c.PostForm("caption"),
		Description:  c.PostForm("description"),
	}

	// Try to detect image dimensions.
	if strings.HasPrefix(mimeType, "image/") {
		// Image dimension detection would go here with imaging library
		// For now, we skip this.
	}

	if err := h.db.Create(&media).Error; err != nil {
		// Cleanup file on DB error.
		os.Remove(filePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save media record"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": media})
}

// Get returns a single media item.
// GET /api/v1/media/:id
func (h *MediaHandler) Get(c *gin.Context) {
	var media models.Media
	if err := h.db.Preload("Uploader").First(&media, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Media not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": media})
}

// Update updates media metadata (alt, title, caption, etc.).
// PUT /api/v1/media/:id
func (h *MediaHandler) Update(c *gin.Context) {
	var media models.Media
	if err := h.db.First(&media, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Media not found"})
		return
	}

	var req struct {
		Alt         string `json:"alt"`
		Title       string `json:"title"`
		Caption     string `json:"caption"`
		Description string `json:"description"`
		Folder      string `json:"folder"`
	}
	c.ShouldBindJSON(&req)

	updates := map[string]interface{}{
		"alt":         req.Alt,
		"title":       req.Title,
		"caption":     req.Caption,
		"description": req.Description,
	}
	if req.Folder != "" {
		updates["folder"] = req.Folder
	}

	h.db.Model(&media).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"data": media})
}

// Delete removes a media file.
// DELETE /api/v1/media/:id
func (h *MediaHandler) Delete(c *gin.Context) {
	var media models.Media
	if err := h.db.First(&media, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Media not found"})
		return
	}

	// Delete file from disk.
	os.Remove(media.FilePath)
	if media.ThumbnailURL != "" {
		thumbPath := strings.Replace(media.ThumbnailURL, h.cfg.URLPrefix, h.cfg.StoragePath, 1)
		os.Remove(thumbPath)
	}

	h.db.Delete(&media)
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

	var media []models.Media
	h.db.Where("id IN ?", req.IDs).Find(&media)

	for _, m := range media {
		os.Remove(m.FilePath)
	}

	result := h.db.Where("id IN ?", req.IDs).Delete(&models.Media{})
	c.JSON(http.StatusOK, gin.H{"message": "Deleted", "affected": result.RowsAffected})
}

// Folders returns all unique folders.
// GET /api/v1/media/folders
func (h *MediaHandler) Folders(c *gin.Context) {
	var folders []string
	h.db.Model(&models.Media{}).Distinct().Pluck("folder", &folders)
	c.JSON(http.StatusOK, gin.H{"data": folders})
}

// Stats returns media library statistics.
// GET /api/v1/media/stats
func (h *MediaHandler) Stats(c *gin.Context) {
	var stats struct {
		TotalFiles int64 `json:"total_files"`
		TotalSize  int64 `json:"total_size"`
		Images     int64 `json:"images"`
		Videos     int64 `json:"videos"`
		Documents  int64 `json:"documents"`
	}
	h.db.Model(&models.Media{}).Count(&stats.TotalFiles)
	h.db.Model(&models.Media{}).Select("COALESCE(SUM(file_size), 0)").Scan(&stats.TotalSize)
	h.db.Model(&models.Media{}).Where("mime_type LIKE ?", "image%").Count(&stats.Images)
	h.db.Model(&models.Media{}).Where("mime_type LIKE ?", "video%").Count(&stats.Videos)
	h.db.Model(&models.Media{}).Where("mime_type LIKE ?", "application%").Count(&stats.Documents)
	c.JSON(http.StatusOK, gin.H{"data": stats})
}
