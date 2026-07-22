package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/services"
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
//
//	@Summary      List tags
//	@Description  Returns all tags with optional sorting and limit
//	@Tags         Tags
//	@Produce      json
//	@Param        sort    query  string  false  "Sort field (name|count)"  default(name)
//	@Param        limit   query  int     false  "Max items (0 = all)"  default(0)
//	@Param        search  query  string  false  "Search by name"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      401  {object}  APIResponse
//	@Router       /tags [get]
func (h *TagHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))

	params := services.TagListParams{
		Sort:   c.DefaultQuery("sort", "name"),
		Limit:  limit,
		Search: c.Query("search"),
	}

	tags, total, err := h.svc.List(params)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"data": tags, "total": total})
}

// Get returns a single tag.
// GET /api/v1/tags/:id
//
//	@Summary      Get tag
//	@Description  Returns a single tag by ID
//	@Tags         Tags
//	@Produce      json
//	@Param        id   path      int     true  "Tag ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.Tag}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /tags/{id} [get]
func (h *TagHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid tag ID")
		return
	}

	tag, err := h.svc.Get(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, tag)
}

// Create creates a new tag.
// POST /api/v1/tags
//
//	@Summary      Create tag
//	@Description  Creates a new tag (requires tags.manage permission)
//	@Tags         Tags
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.CreateTagRequest  true  "Tag data"
//	@Security     BearerAuth
//	@Success      201   {object}  APIResponse{data=models.Tag}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /tags [post]
func (h *TagHandler) Create(c *gin.Context) {
	var req services.CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	tag, err := h.svc.Create(req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Created(c, tag)
}

// Update updates a tag.
// PUT /api/v1/tags/:id
//
//	@Summary      Update tag
//	@Description  Updates a tag (requires tags.manage permission)
//	@Tags         Tags
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                      true  "Tag ID"
//	@Param        body  body      services.UpdateTagRequest  true  "Tag data"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Failure      404   {object}  APIResponse
//	@Router       /tags/{id} [put]
func (h *TagHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid tag ID")
		return
	}

	var req services.UpdateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if err := h.svc.Update(uint(id), req); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Tag updated"})
}

// Delete removes a tag.
// DELETE /api/v1/tags/:id
//
//	@Summary      Delete tag
//	@Description  Removes a tag (requires tags.manage permission)
//	@Tags         Tags
//	@Produce      json
//	@Param        id   path      int     true  "Tag ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /tags/{id} [delete]
func (h *TagHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid tag ID")
		return
	}

	if err := h.svc.Delete(uint(id)); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Tag deleted"})
}

// Merge merges multiple tags into one target.
// POST /api/v1/tags/merge
//
//	@Summary      Merge tags
//	@Description  Merges multiple source tags into one target tag (requires tags.manage permission)
//	@Tags         Tags
//	@Accept       json
//	@Produce      json
//	@Param        body  body      object  true  "Merge payload {source_ids, target_id, delete_old}"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /tags/merge [post]
func (h *TagHandler) Merge(c *gin.Context) {
	var req struct {
		SourceIDs []uint `json:"source_ids" binding:"required"`
		TargetID  uint   `json:"target_id" binding:"required"`
		DeleteOld bool   `json:"delete_old"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if err := h.svc.Merge(req.SourceIDs, req.TargetID, req.DeleteOld); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Tags merged"})
}
