package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/middleware"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/services"
)

// CommentHandler handles comment operations.
type CommentHandler struct {
	svc *services.CommentService
}

func NewCommentHandler(svc *services.CommentService) *CommentHandler {
	return &CommentHandler{svc: svc}
}

// List returns comments with pagination and filters.
// GET /api/v1/comments?status=pending&article_id=1&page=1
//
//	@Summary      List comments
//	@Description  Returns comments with pagination and filters
//	@Tags         Comments
//	@Produce      json
//	@Param        page        query  int     false  "Page number"           default(1)
//	@Param        page_size   query  int     false  "Items per page"        default(20)
//	@Param        status      query  string  false  "Filter by status (pending|approved|spam|trash)"
//	@Param        article_id  query  string  false  "Filter by article ID"
//	@Param        search      query  string  false  "Search content"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      401  {object}  APIResponse
//	@Router       /comments [get]
func (h *CommentHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	params := services.CommentListParams{
		Page:      page,
		PageSize:  pageSize,
		Status:    c.Query("status"),
		ArticleID: c.Query("article_id"),
		Search:    c.Query("search"),
	}

	comments, total, err := h.svc.List(params)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	paginate := paginateFrom(page, pageSize, total)
	Success(c, listResponse(comments, paginate))
}

// Get returns a single comment.
// GET /api/v1/comments/:id
//
//	@Summary      Get comment
//	@Description  Returns a single comment by ID
//	@Tags         Comments
//	@Produce      json
//	@Param        id   path      int     true  "Comment ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=models.Comment}
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /comments/{id} [get]
func (h *CommentHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid comment ID")
		return
	}

	comment, err := h.svc.Get(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, comment)
}

// Create creates a new comment (public or authenticated).
// POST /api/v1/comments
//
//	@Summary      Create comment
//	@Description  Creates a new comment (public or authenticated; rate-limited)
//	@Tags         Comments
//	@Accept       json
//	@Produce      json
//	@Param        body  body      services.CreateCommentRequest  true  "Comment data"
//	@Success      201   {object}  APIResponse{data=models.Comment}
//	@Failure      400   {object}  APIResponse
//	@Failure      429   {object}  APIResponse
//	@Router       /comments [post]
func (h *CommentHandler) Create(c *gin.Context) {
	var req services.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	var userID *uint
	isEditor := false
	if user := middleware.GetCurrentUser(c); user != nil {
		userID = &user.ID
		isEditor = user.IsEditor()
	}

	comment, err := h.svc.Create(req, c.ClientIP(), c.Request.UserAgent(), userID, isEditor)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Created(c, comment)
}

// Update updates a comment's content.
// PUT /api/v1/comments/:id
//
//	@Summary      Update comment
//	@Description  Updates a comment's content
//	@Tags         Comments
//	@Accept       json
//	@Produce      json
//	@Param        id    path      int     true  "Comment ID"
//	@Param        body  body      object  true  "Comment payload {content}"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      404   {object}  APIResponse
//	@Router       /comments/{id} [put]
func (h *CommentHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid comment ID")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	if req.Content == "" {
		BadRequest(c, "Content is required")
		return
	}

	if err := h.svc.Update(uint(id), req.Content); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Comment updated"})
}

// Approve approves a pending comment.
// POST /api/v1/comments/:id/approve
//
//	@Summary      Approve comment
//	@Description  Approves a pending comment (requires comments.moderate permission)
//	@Tags         Comments
//	@Produce      json
//	@Param        id   path      int     true  "Comment ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /comments/{id}/approve [post]
func (h *CommentHandler) Approve(c *gin.Context) {
	h.updateStatus(c, "approved")
}

// Spam marks a comment as spam.
// POST /api/v1/comments/:id/spam
//
//	@Summary      Mark comment as spam
//	@Description  Marks a comment as spam (requires comments.moderate permission)
//	@Tags         Comments
//	@Produce      json
//	@Param        id   path      int     true  "Comment ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /comments/{id}/spam [post]
func (h *CommentHandler) Spam(c *gin.Context) {
	h.updateStatus(c, "spam")
}

// Trash moves a comment to trash.
// POST /api/v1/comments/:id/trash
//
//	@Summary      Trash comment
//	@Description  Moves a comment to trash (requires comments.moderate permission)
//	@Tags         Comments
//	@Produce      json
//	@Param        id   path      int     true  "Comment ID"
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse
//	@Failure      400  {object}  APIResponse
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /comments/{id}/trash [post]
func (h *CommentHandler) Trash(c *gin.Context) {
	h.updateStatus(c, "trash")
}

func (h *CommentHandler) updateStatus(c *gin.Context, status string) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid comment ID")
		return
	}

	if err := h.svc.UpdateStatus(uint(id), status); err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Comment status updated to " + status})
}

// BulkAction handles bulk comment operations.
// POST /api/v1/comments/bulk
//
//	@Summary      Bulk comment action
//	@Description  Performs a bulk action on multiple comments (requires comments.moderate permission)
//	@Tags         Comments
//	@Accept       json
//	@Produce      json
//	@Param        body  body      object  true  "Bulk payload {comment_ids, action}"
//	@Security     BearerAuth
//	@Success      200   {object}  APIResponse
//	@Failure      400   {object}  APIResponse
//	@Failure      401   {object}  APIResponse
//	@Failure      403   {object}  APIResponse
//	@Router       /comments/bulk [post]
func (h *CommentHandler) BulkAction(c *gin.Context) {
	var req struct {
		CommentIDs []uint `json:"comment_ids" binding:"required"`
		Action     string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, sanitizeBindErr(err))
		return
	}

	affected, err := h.svc.BulkAction(req.CommentIDs, req.Action)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, gin.H{"message": "Bulk action completed", "affected": affected})
}

// ArticleComments returns comments for a specific article (public).
// GET /api/v1/articles/:id/comments
//
//	@Summary      Article comments
//	@Description  Returns approved comments for a specific article (public)
//	@Tags         Comments
//	@Produce      json
//	@Param        id   path      int     true  "Article ID"
//	@Success      200  {object}  APIResponse{data=[]models.Comment}
//	@Failure      400  {object}  APIResponse
//	@Failure      404  {object}  APIResponse
//	@Router       /articles/{id}/comments [get]
func (h *CommentHandler) ArticleComments(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		BadRequest(c, "Invalid article ID")
		return
	}

	comments, err := h.svc.ArticleComments(uint(id))
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, comments)
}

// Stats returns comment statistics.
// GET /api/v1/comments/stats
//
//	@Summary      Comment stats
//	@Description  Returns comment statistics (requires comments.view permission)
//	@Tags         Comments
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  APIResponse{data=object}
//	@Failure      401  {object}  APIResponse
//	@Failure      403  {object}  APIResponse
//	@Router       /comments/stats [get]
func (h *CommentHandler) Stats(c *gin.Context) {
	stats, err := h.svc.Stats()
	if err != nil {
		handleServiceError(c, err)
		return
	}

	Success(c, stats)
}

// Pagination helpers used by multiple handlers.
func paginateFrom(page, pageSize int, total int64) models.Paginate {
	return models.Paginate{Page: page, PageSize: pageSize, Total: total}
}

func listResponse(items interface{}, p models.Paginate) gin.H {
	return gin.H{
		"items":       items,
		"page":        p.Page,
		"page_size":   p.PageSize,
		"total":       p.Total,
		"total_pages": p.TotalPages(),
		"has_next":    p.HasNext(),
		"has_prev":    p.HasPrev(),
	}
}
