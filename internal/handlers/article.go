package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/middleware"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/services"
)

// ArticleHandler handles article-related HTTP requests.
type ArticleHandler struct {
	svc *services.ArticleService
}

// NewArticleHandler creates a new article handler.
func NewArticleHandler(svc *services.ArticleService) *ArticleHandler {
	return &ArticleHandler{svc: svc}
}

// List returns a paginated list of articles.
// GET /api/v1/articles?page=1&page_size=20&status=published&category_id=1&tag=go&sort=newest
//
//	@Summary      List articles
//	@Description  Returns a paginated, filtered list of articles
//	@Tags         Articles
//	@Produce      json
//	@Param        page        query  int     false  "Page number"     default(1)
//	@Param        page_size   query  int     false  "Items per page"  default(20)
//	@Param        status      query  string  false  "Filter by status"  Enums(draft,published,pending,scheduled,trash,archived)
//	@Param        post_type   query  string  false  "Post type"  Enums(post,page)
//	@Param        category_id query  string  false  "Filter by category ID"
//	@Param        tag         query  string  false  "Filter by tag slug"
//	@Param        search      query  string  false  "Search keyword"
//	@Param        sort        query  string  false  "Sort order"  Enums(newest,oldest,most_viewed,most_liked)  default(newest)
//	@Param        author_id   query  string  false  "Filter by author ID"
//	@Param        full        query  bool    false  "Include full article body (content). Default false: list omits content for smaller payloads."
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      401  {object}  APIResponse
//	@Router       /articles [get]
func (h *ArticleHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	filter := services.ListArticlesFilter{
		Page:       page,
		PageSize:   pageSize,
		Status:     c.Query("status"),
		PostType:   c.Query("post_type"),
		CategoryID: c.Query("category_id"),
		TagSlug:    c.Query("tag"),
		Search:     c.Query("search"),
		Sort:       c.DefaultQuery("sort", "newest"),
		AuthorID:   c.Query("author_id"),
		Locale:     c.Query("locale"),
		Full:       c.Query("full") == "true",
	}

	result, err := h.svc.List(filter)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, result)
}

// Get returns a single article by ID.
// GET /api/v1/articles/:id
//
//	@Summary      Get article by ID
//	@Description  Returns a single article by its ID
//	@Tags         Articles
//	@Produce      json
//	@Param        id  path      int  true  "Article ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.Article}
//	@Failure      400  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /articles/{id} [get]
func (h *ArticleHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid article ID")
		return
	}

	article, err := h.svc.Get(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, article)
}

// GetBySlug returns a single article by slug (public endpoint).
// GET /api/v1/articles/slug/:slug
//
//	@Summary      Get article by slug
//	@Description  Returns a single article by its URL slug (public)
//	@Tags         Articles
//	@Produce      json
//	@Param        slug  path      string  true  "Article slug"
//	@Success      200  {object}  APIResponse{data=models.Article}
//	@Failure      404  {object}  APIResponse
//	@Router       /articles/slug/{slug} [get]
func (h *ArticleHandler) GetBySlug(c *gin.Context) {
	article, err := h.svc.GetBySlug(c.Param("slug"))
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, article)
}

// Create creates a new article.
// POST /api/v1/articles
//
//	@Summary      Create article
//	@Description  Create a new article (requires articles.create permission)
//	@Tags         Articles
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.CreateArticleRequest  true  "Article data"
//	@Security     BearerAuth
//	@Success      201   {object}  APIResponse{data=models.Article}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /articles [post]
func (h *ArticleHandler) Create(c *gin.Context) {
	var req services.CreateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	user := getCurrentUser(c)
	if user == nil {
		return
	}

	article, err := h.svc.Create(req, user.ID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Created(c, article)
}

// Update updates an existing article.
// PUT /api/v1/articles/:id
//
//	@Summary      Update article
//	@Description  Update an existing article (requires articles.edit permission)
//	@Tags         Articles
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int                        true  "Article ID"
//	@Param        body  body      services.UpdateArticleRequest  true  "Fields to update"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse{data=models.Article}
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Failure      404   {object}  APIResponse
//	@Router       /articles/{id} [put]
func (h *ArticleHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid article ID")
		return
	}

	var req services.UpdateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	user := getCurrentUser(c)
	if user == nil {
		return
	}

	article, err := h.svc.Update(uint(id), req, user.ID, user.IsEditor())
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, article)
}

// Delete soft-deletes an article.
// DELETE /api/v1/articles/:id
//
//	@Summary      Delete article
//	@Description  Soft-delete an article (requires articles.delete permission)
//	@Tags         Articles
//	@Produce      json
//	@Param        id  path      int  true  "Article ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /articles/{id} [delete]
func (h *ArticleHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid article ID")
		return
	}

	user := getCurrentUser(c)
	if user == nil {
		return
	}

	if err := h.svc.Delete(uint(id), user.ID, user.IsEditor()); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Article deleted successfully"})
}

// BulkAction handles bulk operations on articles.
// POST /api/v1/articles/bulk
func (h *ArticleHandler) BulkAction(c *gin.Context) {
	var req services.BulkActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	user := getCurrentUser(c)
	if user == nil {
		return
	}
	if !user.IsEditor() {
		Forbidden(c, "Insufficient permissions")
		return
	}

	affected, err := h.svc.BulkAction(req)
	if err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	Success(c, gin.H{
		"message":  "Bulk action completed",
		"action":   req.Action,
		"affected": affected,
	})
}

// Revisions returns the revision history for an article.
// GET /api/v1/articles/:id/revisions
//
//	@Summary      List article revisions
//	@Description  Returns the revision history for an article
//	@Tags         Articles
//	@Produce      json
//	@Param        id  path  int  true  "Article ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /articles/{id}/revisions [get]
func (h *ArticleHandler) Revisions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid article ID")
		return
	}

	revisions, err := h.svc.Revisions(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, revisions)
}

// RestoreRevision restores an article to a specific revision.
// POST /api/v1/articles/:id/revisions/:revision_id/restore
//
//	@Summary      Restore article revision
//	@Description  Restores an article to a specific revision
//	@Tags         Articles
//	@Produce      json
//	@Param        id            path  int  true  "Article ID"
//	@Param        revision_id   path  int  true  "Revision ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /articles/{id}/revisions/{revision_id}/restore [post]
func (h *ArticleHandler) RestoreRevision(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid article ID")
		return
	}
	revisionID, err := strconv.ParseUint(c.Param("revision_id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid revision ID")
		return
	}

	user := getCurrentUser(c)
	if user == nil {
		return
	}

	if err := h.svc.RestoreRevision(uint(id), uint(revisionID), user.ID); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Revision restored successfully"})
}

// LikeArticle increments the like count.
// POST /api/v1/articles/:id/like
func (h *ArticleHandler) LikeArticle(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid article ID")
		return
	}

	if err := h.svc.LikeArticle(uint(id)); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Article liked"})
}

// ──────────────────────────────────────────────────────────────────────────────
// Publication workflow handlers (P2-3)
// ──────────────────────────────────────────────────────────────────────────────

// ScheduleRequest is the JSON body for the /schedule endpoint.
type ScheduleRequest struct {
	ScheduledAt string `json:"scheduled_at" binding:"required"` // RFC3339
}

// Publish flips an article to published status.
// POST /api/v1/articles/:id/publish
//
//	@Summary      Publish article
//	@Description  Publishes an article (status transition to published). Requires articles.edit permission.
//	@Tags         Articles
//	@Produce      json
//	@Param        id  path  int  true  "Article ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.Article}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Failure      409  {object}  APIResponse
//	@Router       /articles/{id}/publish [post]
func (h *ArticleHandler) Publish(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid article ID")
		return
	}
	article, err := h.svc.Publish(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, article)
}

// Unpublish reverts a published/scheduled article back to draft.
// POST /api/v1/articles/:id/unpublish
//
//	@Summary      Unpublish article
//	@Description  Reverts a published/scheduled article back to draft. Requires articles.edit permission.
//	@Tags         Articles
//	@Produce      json
//	@Param        id  path  int  true  "Article ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.Article}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Failure      409  {object}  APIResponse
//	@Router       /articles/{id}/unpublish [post]
func (h *ArticleHandler) Unpublish(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid article ID")
		return
	}
	article, err := h.svc.Unpublish(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, article)
}

// SubmitForReview moves a draft into the pending (review) queue.
// POST /api/v1/articles/:id/submit-review
//
//	@Summary      Submit article for review
//	@Description  Moves a draft article into the pending (review) queue. Requires articles.edit permission.
//	@Tags         Articles
//	@Produce      json
//	@Param        id  path  int  true  "Article ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.Article}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Failure      409  {object}  APIResponse
//	@Router       /articles/{id}/submit-review [post]
func (h *ArticleHandler) SubmitForReview(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid article ID")
		return
	}
	article, err := h.svc.SubmitForReview(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, article)
}

// Approve marks a pending article as published.
// POST /api/v1/articles/:id/approve
//
//	@Summary      Approve article
//	@Description  Approves a pending article, transitioning it to published. Requires editor role.
//	@Tags         Articles
//	@Produce      json
//	@Param        id  path  int  true  "Article ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.Article}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Failure      409  {object}  APIResponse
//	@Router       /articles/{id}/approve [post]
func (h *ArticleHandler) Approve(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid article ID")
		return
	}
	article, err := h.svc.Approve(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, article)
}

// Schedule marks an article for automatic publication at the given time.
// POST /api/v1/articles/:id/schedule
//
//	@Summary      Schedule article publication
//	@Description  Marks an article for automatic publication at the given time. Requires articles.edit permission.
//	@Tags         Articles
//	@Accept       json
//	@Produce      json
//	@Param        id    path  int              true  "Article ID"
//	@Param        body  body  ScheduleRequest  true  "Schedule payload"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.Article}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Failure      409  {object}  APIResponse
//	@Router       /articles/{id}/schedule [post]
func (h *ArticleHandler) Schedule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid article ID")
		return
	}
	var req ScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}
	at, err := parseScheduleTime(req.ScheduledAt)
	if err != nil {
		BadRequest(c, "Invalid scheduled_at: "+err.Error())
		return
	}
	article, err := h.svc.Schedule(uint(id), at)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, article)
}

// Archive moves an article out of the active lifecycle.
// POST /api/v1/articles/:id/archive
func (h *ArticleHandler) Archive(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid article ID")
		return
	}
	article, err := h.svc.Archive(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, article)
}

// parseScheduleTime parses a schedule timestamp, accepting RFC3339 or the
// looser layout "2006-01-02 15:04:05" for convenience.
func parseScheduleTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02 15:04", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("expected RFC3339 or YYYY-MM-DD HH:MM[:SS]")
}

// Feed returns articles as RSS/XML.
// GET /api/v1/feed
func (h *ArticleHandler) Feed(c *gin.Context) {
	xml, err := h.svc.GenerateFeed()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	c.Data(200, "application/rss+xml; charset=utf-8", []byte(xml))
}

// ─── i18n: translation endpoints ────────────────────────────────────────────

// ListTranslations returns all sibling translations of an article.
// GET /api/v1/articles/:id/translations
//
//	@Summary      List article translations
//	@Description  Returns all sibling translations of an article (same translation group)
//	@Tags         Articles
//	@Produce      json
//	@Param        id  path  int  true  "Article ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /articles/{id}/translations [get]
func (h *ArticleHandler) ListTranslations(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid article ID")
		return
	}
	translations, err := h.svc.ListTranslations(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Success(c, translations)
}

// CreateTranslation creates a new article as a translation of an existing one.
// POST /api/v1/articles/:id/translations?locale=zh
//
//	@Summary      Create article translation
//	@Description  Creates a new article as a translation of an existing one. Requires articles.create permission.
//	@Tags         Articles
//	@Accept       json
//	@Produce      json
//	@Param        id      path  int                          true  "Article ID"
//	@Param        locale  query string                       true  "Target locale (BCP-47 tag, e.g. en, zh)"
//	@Param        body    body  services.CreateArticleRequest  true  "Article translation payload"
//	@Security     BearerAuth
//	@Success      201  {object}  APIResponse{data=object}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /articles/{id}/translations [post]
func (h *ArticleHandler) CreateTranslation(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid article ID")
		return
	}
	locale := c.Query("locale")
	if locale == "" {
		BadRequest(c, "locale query parameter is required")
		return
	}
	var req services.CreateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}
	user := getCurrentUser(c)
	if user == nil {
		return
	}
	article, err := h.svc.CreateTranslation(uint(id), locale, req, user.ID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	Created(c, article)
}

// getCurrentUser returns the authenticated user or sends a 401 response.
func getCurrentUser(c *gin.Context) *models.User {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		Unauthorized(c, "Not authenticated")
	}
	return user
}
