package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/vortexcms/go-cms/internal/middleware"
	"github.com/vortexcms/go-cms/internal/models"
	"github.com/vortexcms/go-cms/internal/services"
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch comments"})
		return
	}

	paginate := paginateFrom(page, pageSize, total)
	c.JSON(http.StatusOK, listResponse(comments, paginate))
}

// Get returns a single comment.
// GET /api/v1/comments/:id
func (h *CommentHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid comment ID"})
		return
	}

	comment, err := h.svc.Get(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comment not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": comment})
}

// Create creates a new comment (public or authenticated).
// POST /api/v1/comments
func (h *CommentHandler) Create(c *gin.Context) {
	var req services.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	c.JSON(http.StatusCreated, gin.H{"data": comment})
}

// Update updates a comment's content.
// PUT /api/v1/comments/:id
func (h *CommentHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid comment ID"})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	c.ShouldBindJSON(&req)

	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Content is required"})
		return
	}

	if err := h.svc.Update(uint(id), req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update comment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Comment updated"})
}

// Approve approves a pending comment.
// POST /api/v1/comments/:id/approve
func (h *CommentHandler) Approve(c *gin.Context) {
	h.updateStatus(c, "approved")
}

// Spam marks a comment as spam.
// POST /api/v1/comments/:id/spam
func (h *CommentHandler) Spam(c *gin.Context) {
	h.updateStatus(c, "spam")
}

// Trash moves a comment to trash.
// POST /api/v1/comments/:id/trash
func (h *CommentHandler) Trash(c *gin.Context) {
	h.updateStatus(c, "trash")
}

func (h *CommentHandler) updateStatus(c *gin.Context, status string) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid comment ID"})
		return
	}

	if err := h.svc.UpdateStatus(uint(id), status); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comment not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Comment status updated to " + status})
}

// BulkAction handles bulk comment operations.
// POST /api/v1/comments/bulk
func (h *CommentHandler) BulkAction(c *gin.Context) {
	var req struct {
		CommentIDs []uint `json:"comment_ids" binding:"required"`
		Action     string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	affected, err := h.svc.BulkAction(req.CommentIDs, req.Action)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Bulk action completed", "affected": affected})
}

// ArticleComments returns comments for a specific article (public).
// GET /api/v1/articles/:id/comments
func (h *CommentHandler) ArticleComments(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid article ID"})
		return
	}

	comments, err := h.svc.ArticleComments(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch comments"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": comments})
}

// Stats returns comment statistics.
// GET /api/v1/comments/stats
func (h *CommentHandler) Stats(c *gin.Context) {
	stats, err := h.svc.Stats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
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
