package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// CommentHandler handles comment operations.
type CommentHandler struct{ db *gorm.DB }

func NewCommentHandler(db *gorm.DB) *CommentHandler { return &CommentHandler{db: db} }

// List returns comments with pagination and filters.
// GET /api/v1/comments?status=pending&article_id=1&page=1
func (h *CommentHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	articleID := c.Query("article_id")
	search := c.Query("search")

	if page < 1 { page = 1 }
	if pageSize < 1 || pageSize > 100 { pageSize = 20 }

	query := h.db.Model(&models.Comment{}).Preload("User").Preload("Article")
	if status != "" { query = query.Where("status = ?", status) }
	if articleID != "" { query = query.Where("article_id = ?", articleID) }
	if search != "" {
		query = query.Where("content LIKE ? OR author_name LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var comments []models.Comment
	query.Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&comments)

	paginate := models.Paginate{Page: page, PageSize: pageSize, Total: total}
	c.JSON(http.StatusOK, models.NewListResponse(comments, paginate))
}

// Get returns a single comment.
// GET /api/v1/comments/:id
func (h *CommentHandler) Get(c *gin.Context) {
	var comment models.Comment
	if err := h.db.Preload("User").Preload("Children").
		First(&comment, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comment not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": comment})
}

// Create creates a new comment (public or authenticated).
// POST /api/v1/comments
func (h *CommentHandler) Create(c *gin.Context) {
	var req struct {
		ArticleID  uint   `json:"article_id" binding:"required"`
		ParentID   *uint  `json:"parent_id"`
		Content    string `json:"content" binding:"required"`
		AuthorName string `json:"author_name"`
		AuthorEmail string `json:"author_email"`
		AuthorURL  string `json:"author_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if article exists and allows comments.
	var article models.Article
	if err := h.db.First(&article, req.ArticleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
		return
	}
	if !article.AllowComment {
		c.JSON(http.StatusForbidden, gin.H{"error": "Comments are disabled for this article"})
		return
	}

	comment := models.Comment{
		ArticleID:   req.ArticleID,
		ParentID:    req.ParentID,
		Content:     req.Content,
		AuthorName:  req.AuthorName,
		AuthorEmail: req.AuthorEmail,
		AuthorURL:   req.AuthorURL,
		AuthorIP:    c.ClientIP(),
		Agent:       c.Request.UserAgent(),
		Status:      "pending", // Default to pending for moderation
	}

	// If authenticated, link to user.
	if user := getCurrentUserFromDB(h.db, c); user != nil {
		comment.UserID = &user.ID
		comment.AuthorName = user.DisplayName
		comment.AuthorEmail = user.Email
		// Auto-approve for editors+
		if user.IsEditor() {
			comment.Status = "approved"
		}
	}

	// Calculate depth.
	if req.ParentID != nil {
		var parent models.Comment
		if h.db.First(&parent, *req.ParentID).Error == nil {
			comment.Depth = parent.Depth + 1
		}
	}

	if err := h.db.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create comment"})
		return
	}

	// Update article comment count.
	h.db.Model(&article).UpdateColumn("comment_count", gorm.Expr("comment_count + 1"))

	c.JSON(http.StatusCreated, gin.H{"data": comment})
}

// Update updates a comment's content.
// PUT /api/v1/comments/:id
func (h *CommentHandler) Update(c *gin.Context) {
	var comment models.Comment
	if err := h.db.First(&comment, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comment not found"})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	c.ShouldBindJSON(&req)
	if req.Content != "" {
		h.db.Model(&comment).Update("content", req.Content)
	}

	c.JSON(http.StatusOK, gin.H{"data": comment})
}

// Approve approves a pending comment.
// POST /api/v1/comments/:id/approve
func (h *CommentHandler) Approve(c *gin.Context) {
	h.updateCommentStatus(c, "approved")
}

// Spam marks a comment as spam.
// POST /api/v1/comments/:id/spam
func (h *CommentHandler) Spam(c *gin.Context) {
	h.updateCommentStatus(c, "spam")
}

// Trash moves a comment to trash.
// POST /api/v1/comments/:id/trash
func (h *CommentHandler) Trash(c *gin.Context) {
	h.updateCommentStatus(c, "trash")
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

	var affected int64
	switch req.Action {
	case "approve":
		affected = h.db.Model(&models.Comment{}).Where("id IN ?", req.CommentIDs).
			Update("status", "approved").RowsAffected
	case "spam":
		affected = h.db.Model(&models.Comment{}).Where("id IN ?", req.CommentIDs).
			Update("status", "spam").RowsAffected
	case "trash":
		affected = h.db.Model(&models.Comment{}).Where("id IN ?", req.CommentIDs).
			Update("status", "trash").RowsAffected
	case "delete":
		affected = h.db.Where("id IN ?", req.CommentIDs).Delete(&models.Comment{}).RowsAffected
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown action"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Bulk action completed", "affected": affected})
}

// ArticleComments returns comments for a specific article (public).
// GET /api/v1/articles/:id/comments
func (h *CommentHandler) ArticleComments(c *gin.Context) {
	articleID := c.Param("id")

	var comments []models.Comment
	h.db.Where("article_id = ? AND status = ? AND parent_id IS NULL", articleID, "approved").
		Preload("User").
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			return db.Where("status = ?", "approved").Order("created_at ASC")
		}).
		Preload("Children.User").
		Order("is_sticky DESC, created_at DESC").
		Find(&comments)

	c.JSON(http.StatusOK, gin.H{"data": comments})
}

// Stats returns comment statistics.
// GET /api/v1/comments/stats
func (h *CommentHandler) Stats(c *gin.Context) {
	var stats struct {
		Total    int64 `json:"total"`
		Pending  int64 `json:"pending"`
		Approved int64 `json:"approved"`
		Spam     int64 `json:"spam"`
		Today    int64 `json:"today"`
	}
	h.db.Model(&models.Comment{}).Count(&stats.Total)
	h.db.Model(&models.Comment{}).Where("status = ?", "pending").Count(&stats.Pending)
	h.db.Model(&models.Comment{}).Where("status = ?", "approved").Count(&stats.Approved)
	h.db.Model(&models.Comment{}).Where("status = ?", "spam").Count(&stats.Spam)
	h.db.Model(&models.Comment{}).Where("DATE(created_at) = DATE(?)", time.Now()).Count(&stats.Today)

	c.JSON(http.StatusOK, gin.H{"data": stats})
}

func (h *CommentHandler) updateCommentStatus(c *gin.Context, status string) {
	var comment models.Comment
	if err := h.db.First(&comment, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Comment not found"})
		return
	}
	h.db.Model(&comment).Update("status", status)
	c.JSON(http.StatusOK, gin.H{"message": "Comment status updated to " + status})
}

func getCurrentUserFromDB(db *gorm.DB, c *gin.Context) *models.User {
	// Reuse middleware helper if available.
	if user, exists := c.Get("currentUser"); exists {
		return user.(*models.User)
	}
	return nil
}
