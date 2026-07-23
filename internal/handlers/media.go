package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/middleware"
	_ "github.com/yamovo/contentx/internal/models" // swag annotation resolution
	"github.com/yamovo/contentx/internal/services"
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
//
//	@Summary      List media
//	@Description  Returns media files with pagination and filters
//	@Tags         Media
//	@Produce      json
//	@Param        page       query  int     false  "Page number"             default(1)
//	@Param        page_size  query  int     false  "Items per page"          default(20)
//	@Param        type       query  string  false  "Filter by MIME type"
//	@Param        folder     query  string  false  "Filter by folder"
//	@Param        search     query  string  false  "Search by filename"
//	@Param        sort       query  string  false  "Sort order (newest|oldest|name)"  default(newest)
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      401  {object}  APIResponse
//	@Router       /media [get]
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
		handleServiceError(c, err)
		return
	}

	paginate := paginateFrom(page, pageSize, total)
	Success(c, listResponse(media, paginate))
}

// Upload handles file upload.
// POST /api/v1/media/upload
//
//	@Summary      Upload media
//	@Description  Uploads a file (requires media.upload permission)
//	@Tags         Media
//	@Accept       multipart/form-data
//	@Produce      json
//	@Param        file        formData  file    true  "File to upload"
//	@Param        folder      formData  string  false  "Destination folder"
//	@Param        alt         formData  string  false  "Alt text"
//	@Param        title       formData  string  false  "Title"
//	@Param        caption     formData  string  false  "Caption"
//	@Param        description  formData  string  false  "Description"
//	@Security     BearerAuth
//	@Success      201  {object}  APIResponse{data=models.Media}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /media/upload [post]
func (h *MediaHandler) Upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		BadRequest(c, "No file provided")
		return
	}
	defer func() { _ = file.Close() }()

	user := middleware.GetCurrentUser(c)
	if user == nil {
		Unauthorized(c, "Not authenticated")
		return
	}

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

	Created(c, media)
}

// Get returns a single media item.
// GET /api/v1/media/:id
//
//	@Summary      Get media
//	@Description  Returns a single media item by ID
//	@Tags         Media
//	@Produce      json
//	@Param        id   path      int     true  "Media ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.Media}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /media/{id} [get]
func (h *MediaHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid media ID")
		return
	}

	media, err := h.svc.Get(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, media)
}

// Update updates media metadata (alt, title, caption, etc.).
// PUT /api/v1/media/:id
//
//	@Summary      Update media
//	@Description  Updates media metadata (alt, title, caption, etc.)
//	@Tags         Media
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                          true  "Media ID"
//	@Param        body  body      services.UpdateMediaRequest  true  "Media metadata"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      404   {object}  APIResponse
//	@Router       /media/{id} [put]
func (h *MediaHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid media ID")
		return
	}

	var req services.UpdateMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if err := h.svc.Update(uint(id), req); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Media updated"})
}

// Delete removes a media file.
// DELETE /api/v1/media/:id
//
//	@Summary      Delete media
//	@Description  Removes a media file (requires media.delete permission)
//	@Tags         Media
//	@Produce      json
//	@Param        id   path      int     true  "Media ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /media/{id} [delete]
func (h *MediaHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid media ID")
		return
	}

	if err := h.svc.Delete(uint(id)); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Media deleted"})
}

// BulkDelete removes multiple media files.
// POST /api/v1/media/bulk-delete
//
//	@Summary      Bulk delete media
//	@Description  Removes multiple media files (requires media.delete permission)
//	@Tags         Media
//	@Accept       json
//	@Produce      json
//	@Param        body  body      object  true  "Bulk payload {ids}"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /media/bulk-delete [post]
func (h *MediaHandler) BulkDelete(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	affected, err := h.svc.BulkDelete(req.IDs)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Deleted", "affected": affected})
}

// Folders returns all unique folders.
// GET /api/v1/media/folders
//
//	@Summary      List media folders
//	@Description  Returns all unique media folders
//	@Tags         Media
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=[]string}
//	@Failure      401  {object}  APIResponse
//	@Router       /media/folders [get]
func (h *MediaHandler) Folders(c *gin.Context) {
	folders, err := h.svc.Folders()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, folders)
}

// Stats returns media library statistics.
// GET /api/v1/media/stats
//
//	@Summary      Media stats
//	@Description  Returns media library statistics
//	@Tags         Media
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      401  {object}  APIResponse
//	@Router       /media/stats [get]
func (h *MediaHandler) Stats(c *gin.Context) {
	stats, err := h.svc.Stats()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, stats)
}
