package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/yamovo/contentx/internal/models" // swag annotation resolution
	"github.com/yamovo/contentx/internal/services"
)

// ContentTypeHandler manages content types and entries.
type ContentTypeHandler struct {
	svc *services.ContentTypeService
}

// NewContentTypeHandler creates a new content type handler.
func NewContentTypeHandler(svc *services.ContentTypeService) *ContentTypeHandler {
	return &ContentTypeHandler{svc: svc}
}

// ─── Content Type endpoints ─────────────────────────────────────────────────

// ListTypes returns all content types.
// GET /api/v1/content-types
//
//	@Summary      List content types
//	@Description  Returns all user-defined content types with field definitions
//	@Tags         Content Types
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=[]models.ContentType}
//	@Failure      401  {object}  APIResponse
//	@Router       /content-types [get]
func (h *ContentTypeHandler) ListTypes(c *gin.Context) {
	types, err := h.svc.ListContentTypes()
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, types)
}

// GetType returns a single content type by UID.
// GET /api/v1/content-types/:uid
//
//	@Summary      Get content type
//	@Description  Returns a content type with its field definitions
//	@Tags         Content Types
//	@Produce      json
//	@Param        uid  path      string  true  "Content Type UID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.ContentType}
//	@Failure      404  {object}  APIResponse
//	@Router       /content-types/{uid} [get]
func (h *ContentTypeHandler) GetType(c *gin.Context) {
	ct, err := h.svc.GetContentType(c.Param("uid"))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, ct)
}

// CreateType creates a new content type.
// POST /api/v1/content-types
//
//	@Summary      Create content type
//	@Description  Create a new content type with field definitions
//	@Tags         Content Types
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.CreateContentTypeRequest  true  "Content type definition"
//	@Security     BearerAuth
//	@Success      201   {object}  APIResponse{data=models.ContentType}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /content-types [post]
func (h *ContentTypeHandler) CreateType(c *gin.Context) {
	var req services.CreateContentTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	ct, err := h.svc.CreateContentType(req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Created(c, ct)
}

// DeleteType deletes a content type and all its entries.
// DELETE /api/v1/content-types/:uid
//
//	@Summary      Delete content type
//	@Description  Deletes a content type and all its entries
//	@Tags         Content Types
//	@Produce      json
//	@Param        uid  path      string  true  "Content Type UID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /content-types/{uid} [delete]
func (h *ContentTypeHandler) DeleteType(c *gin.Context) {
	if err := h.svc.DeleteContentType(c.Param("uid")); err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, gin.H{"message": "Content type deleted"})
}

// ─── Content Entry endpoints ────────────────────────────────────────────────

// ListEntries returns entries of a content type.
// GET /api/v1/content/:uid
//
//	@Summary      List entries
//	@Description  Returns paginated entries of a content type
//	@Tags         Content Entries
//	@Produce      json
//	@Param        uid        path      string  true   "Content Type UID"
//	@Param        page       query     int     false  "Page number"  default(1)
//	@Param        page_size  query     int     false  "Items per page"  default(20)
//	@Param        status     query     string  false  "Filter by status"  Enums(draft,published)
//	@Param        search     query     string  false  "Search keyword"
//	@Param        sort       query     string  false  "Sort order"  Enums(newest,oldest,updated)  default(newest)
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.ListResponse}
//	@Failure      401  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /content/{uid} [get]
func (h *ContentTypeHandler) ListEntries(c *gin.Context) {
	uid := c.Param("uid")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	params := services.ListEntriesParams{
		Page:     page,
		PageSize: pageSize,
		Status:   c.Query("status"),
		Search:   c.Query("search"),
		Sort:     c.Query("sort"),
		Locale:   c.Query("locale"),
		Filters:  make(map[string]string),
	}

	// Parse field filters from query params (e.g., ?color=red&size=large).
	for key, values := range c.Request.URL.Query() {
		if !isReservedParam(key) && len(values) > 0 {
			params.Filters[key] = values[0]
		}
	}

	result, err := h.svc.ListEntries(uid, params)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, result)
}

// GetEntry returns a single entry.
// GET /api/v1/content/:uid/:documentId
//
//	@Summary      Get entry
//	@Description  Returns a single entry by document ID
//	@Tags         Content Entries
//	@Produce      json
//	@Param        uid         path      string  true  "Content Type UID"
//	@Param        documentId  path      string  true  "Document ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.ContentEntry}
//	@Failure      404  {object}  APIResponse
//	@Router       /content/{uid}/{documentId} [get]
func (h *ContentTypeHandler) GetEntry(c *gin.Context) {
	entry, err := h.svc.GetEntry(c.Param("uid"), c.Param("documentId"))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, entry)
}

// CreateEntry creates a new entry.
// POST /api/v1/content/:uid
//
//	@Summary      Create entry
//	@Description  Create a new entry for a content type
//	@Tags         Content Entries
//	@Accept       json
//	@Produce      json
//	@Param        uid   path      string                      true  "Content Type UID"
//	@Param        body  body      services.CreateEntryRequest  true  "Entry data"
//	@Security     BearerAuth
//	@Success      201   {object}  APIResponse{data=models.ContentEntry}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      404   {object}  APIResponse
//	@Router       /content/{uid} [post]
func (h *ContentTypeHandler) CreateEntry(c *gin.Context) {
	var req services.CreateEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	user := getCurrentUser(c)
	if user == nil {
		return
	}

	entry, err := h.svc.CreateEntry(c.Param("uid"), req, user.ID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Created(c, entry)
}

// UpdateEntry updates an existing entry.
// PUT /api/v1/content/:uid/:documentId
//
//	@Summary      Update entry
//	@Description  Update an existing entry
//	@Tags         Content Entries
//	@Accept       json
//	@Produce      json
//	@Param        uid         path      string                      true  "Content Type UID"
//	@Param        documentId  path      string                      true  "Document ID"
//	@Param        body        body      services.UpdateEntryRequest  true  "Fields to update"
//	@Security     BearerAuth
//	@Success      200         {object}  APIResponse{data=models.ContentEntry}
//	@Failure      400         {object}  APIResponse
//	@Failure      404         {object}  APIResponse
//	@Router       /content/{uid}/{documentId} [put]
func (h *ContentTypeHandler) UpdateEntry(c *gin.Context) {
	var req services.UpdateEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	user := getCurrentUser(c)
	if user == nil {
		return
	}

	entry, err := h.svc.UpdateEntry(c.Param("uid"), c.Param("documentId"), req, user.ID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, entry)
}

// DeleteEntry deletes an entry.
// DELETE /api/v1/content/:uid/:documentId
//
//	@Summary      Delete entry
//	@Description  Delete an entry by document ID
//	@Tags         Content Entries
//	@Produce      json
//	@Param        uid         path      string  true  "Content Type UID"
//	@Param        documentId  path      string  true  "Document ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /content/{uid}/{documentId} [delete]
func (h *ContentTypeHandler) DeleteEntry(c *gin.Context) {
	if err := h.svc.DeleteEntry(c.Param("uid"), c.Param("documentId")); err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, gin.H{"message": "Entry deleted"})
}

// PublishEntry publishes a draft entry.
// POST /api/v1/content/:uid/:documentId/publish
//
//	@Summary      Publish entry
//	@Description  Publish a draft entry
//	@Tags         Content Entries
//	@Produce      json
//	@Param        uid         path      string  true  "Content Type UID"
//	@Param        documentId  path      string  true  "Document ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.ContentEntry}
//	@Failure      404  {object}  APIResponse
//	@Router       /content/{uid}/{documentId}/publish [post]
func (h *ContentTypeHandler) PublishEntry(c *gin.Context) {
	user := getCurrentUser(c)
	if user == nil {
		return
	}

	entry, err := h.svc.PublishEntry(c.Param("uid"), c.Param("documentId"), user.ID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, entry)
}

// UnpublishEntry reverts a published entry to draft.
// POST /api/v1/content/:uid/:documentId/unpublish
//
//	@Summary      Unpublish entry
//	@Description  Revert a published entry to draft
//	@Tags         Content Entries
//	@Produce      json
//	@Param        uid         path      string  true  "Content Type UID"
//	@Param        documentId  path      string  true  "Document ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.ContentEntry}
//	@Failure      404  {object}  APIResponse
//	@Router       /content/{uid}/{documentId}/unpublish [post]
func (h *ContentTypeHandler) UnpublishEntry(c *gin.Context) {
	user := getCurrentUser(c)
	if user == nil {
		return
	}

	entry, err := h.svc.UnpublishEntry(c.Param("uid"), c.Param("documentId"), user.ID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, entry)
}

// ExportEntries exports all entries of a content type as JSON.
// GET /api/v1/content/:uid/export
//
//	@Summary      Export entries
//	@Description  Export all entries as JSON
//	@Tags         Content Entries
//	@Produce      json
//	@Param        uid  path      string  true  "Content Type UID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=string}
//	@Failure      404  {object}  APIResponse
//	@Router       /content/{uid}/export [get]
func (h *ContentTypeHandler) ExportEntries(c *gin.Context) {
	data, err := h.svc.ExportEntries(c.Param("uid"))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, gin.H{"json": data})
}

// ImportEntries imports entries from JSON.
// POST /api/v1/content/:uid/import
//
//	@Summary      Import entries
//	@Description  Import entries from JSON data
//	@Tags         Content Entries
//	@Accept       json
//	@Produce      json
//	@Param        uid   path      string  true   "Content Type UID"
//	@Param        body  body      object  true   "JSON data"  Schema({"type":"object","properties":{"json":{"type":"string"}}})
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse{data=object{imported=int}}
//	@Failure      400   {object}  APIResponse
//	@Failure      404   {object}  APIResponse
//	@Router       /content/{uid}/import [post]
func (h *ContentTypeHandler) ImportEntries(c *gin.Context) {
	var body struct {
		JSON string `json:"json" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	user := getCurrentUser(c)
	if user == nil {
		return
	}

	count, err := h.svc.ImportEntries(c.Param("uid"), body.JSON, user.ID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"imported": count})
}

// helper to skip reserved query params when parsing field filters.
func isReservedParam(key string) bool {
	reserved := map[string]bool{
		"page": true, "page_size": true, "status": true,
		"search": true, "sort": true, "populate": true,
		"locale": true, // i18n
	}
	return reserved[key]
}

// ─── i18n: content entry translation endpoints ──────────────────────────────

// ListEntryTranslations returns sibling translations of an entry.
// GET /api/v1/content/:uid/:documentId/translations
//
//	@Summary      List entry translations
//	@Description  Returns sibling translations of an entry (same translation group)
//	@Tags         Content Entries
//	@Produce      json
//	@Param        uid         path  string  true  "Content Type UID"
//	@Param        documentId  path  string  true  "Document ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      404  {object}  APIResponse
//	@Router       /content/{uid}/{documentId}/translations [get]
func (h *ContentTypeHandler) ListEntryTranslations(c *gin.Context) {
	translations, err := h.svc.ListEntryTranslations(c.Param("uid"), c.Param("documentId"))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, translations)
}

// CreateEntryTranslation creates a new entry as a translation.
// POST /api/v1/content/:uid/:documentId/translations?locale=zh
//
//	@Summary      Create entry translation
//	@Description  Creates a new entry as a translation of an existing one. Requires content.create permission.
//	@Tags         Content Entries
//	@Accept       json
//	@Produce      json
//	@Param        uid         path  string                     true  "Content Type UID"
//	@Param        documentId  path  string                     true  "Document ID"
//	@Param        locale      query string                     true  "Target locale (BCP-47 tag, e.g. en, zh)"
//	@Param        body        body  services.CreateEntryRequest  true  "Entry translation payload"
//	@Security     BearerAuth
//	@Success      201  {object}  APIResponse{data=models.ContentEntry}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /content/{uid}/{documentId}/translations [post]
func (h *ContentTypeHandler) CreateEntryTranslation(c *gin.Context) {
	locale := c.Query("locale")
	if locale == "" {
		BadRequest(c, "locale query parameter is required")
		return
	}
	var req services.CreateEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}
	user := getCurrentUser(c)
	if user == nil {
		return
	}
	entry, err := h.svc.CreateEntryTranslation(c.Param("uid"), c.Param("documentId"), locale, req, user.ID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Created(c, entry)
}
