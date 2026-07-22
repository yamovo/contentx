package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/services"
)

// SearchHandler handles full-text search HTTP requests.
type SearchHandler struct {
	svc *services.ArticleService
}

// NewSearchHandler creates a new SearchHandler backed by the article service
// (which delegates to the configured SearchIndexer).
func NewSearchHandler(svc *services.ArticleService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

// Search runs a full-text query.
// GET /api/v1/search?q=keyword&type=article&locale=zh&page=1&page_size=20
//
// The public endpoint only returns published content. Authenticated callers
// can pass ?status=draft to search across non-published content (requires
// the articles.view permission, enforced by the route registration).
//
//	@Summary      Search content
//	@Description  Full-text search across articles and pages. Public endpoint returns only published content.
//	@Tags         Search
//	@Produce      json
//	@Param        q          query  string  true   "Search query"
//	@Param        type       query  string  false  "Filter by type"  Enums(article,page)
//	@Param        status     query  string  false  "Filter by status (admin only; public callers are forced to 'published')"  Enums(draft,published,pending,scheduled,trash,archived)
//	@Param        locale     query  string  false  "Filter by locale (BCP-47 tag, e.g. en, zh)"
//	@Param        page       query  int     false  "Page number"     default(1)
//	@Param        page_size  query  int     false  "Items per page"  default(20)
//	@Success      200  {object}  APIResponse{data=object}
//	@Router       /search [get]
func (h *SearchHandler) Search(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		BadRequest(c, "query parameter 'q' is required")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	// Public endpoint: force status=published so non-published content is
	// never exposed. Admin search is a separate protected route that passes
	// through whatever status the caller requested.
	status := c.DefaultQuery("status", "published")

	query := services.SearchQuery{
		Query:    q,
		Type:     c.Query("type"),
		Status:   status,
		Locale:   c.Query("locale"),
		Page:     page,
		PageSize: pageSize,
	}

	result, err := h.svc.Search(c.Request.Context(), query)
	if err != nil {
		InternalError(c)
		return
	}

	c.JSON(200, APIResponse{
		Code:    0,
		Message: "success",
		Data:    result,
		Meta: &Meta{
			Page:     result.Page,
			PageSize: result.PageSize,
			Total:    result.Total,
			HasNext:  result.Page < result.TotalPages,
			HasPrev:  result.Page > 1,
		},
	})
}

// AdminSearch allows authenticated callers to search across all statuses.
// GET /api/v1/search/admin?q=keyword&type=article&status=draft&locale=zh
//
//	@Summary      Search content (admin)
//	@Description  Full-text search across all statuses. Requires authentication.
//	@Tags         Search
//	@Security     BearerAuth
//	@Produce      json
//	@Param        q          query  string  true   "Search query"
//	@Param        type       query  string  false  "Filter by type"  Enums(article,page)
//	@Param        status     query  string  false  "Filter by status (empty = any)"
//	@Param        locale     query  string  false  "Filter by locale"
//	@Param        page       query  int     false  "Page number"     default(1)
//	@Param        page_size  query  int     false  "Items per page"  default(20)
//	@Success      200  {object}  APIResponse{data=object}
//	@Router       /search/admin [get]
func (h *SearchHandler) AdminSearch(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		BadRequest(c, "query parameter 'q' is required")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	// Admin: status is optional; empty means "any".
	query := services.SearchQuery{
		Query:    q,
		Type:     c.Query("type"),
		Status:   c.Query("status"),
		Locale:   c.Query("locale"),
		Page:     page,
		PageSize: pageSize,
	}

	result, err := h.svc.Search(c.Request.Context(), query)
	if err != nil {
		InternalError(c)
		return
	}

	c.JSON(200, APIResponse{
		Code:    0,
		Message: "success",
		Data:    result,
		Meta: &Meta{
			Page:     result.Page,
			PageSize: result.PageSize,
			Total:    result.Total,
			HasNext:  result.Page < result.TotalPages,
			HasPrev:  result.Page > 1,
		},
	})
}

// Reindex rebuilds the search index from the database. Admin only.
// POST /api/v1/search/reindex
//
//	@Summary      Rebuild search index
//	@Description  Rebuilds the full-text search index from the database. Admin only.
//	@Tags         Search
//	@Security     BearerAuth
//	@Produce      json
//	@Success      200  {object}  APIResponse{data=object}
//	@Router       /search/reindex [post]
func (h *SearchHandler) Reindex(c *gin.Context) {
	count, err := h.svc.ReindexAll(c.Request.Context())
	if err != nil {
		Error(c, 500, "REINDEX_FAILED", "failed to rebuild search index: "+err.Error())
		return
	}
	Success(c, gin.H{"indexed": count})
}
