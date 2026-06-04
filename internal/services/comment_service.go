package services

import (
	"errors"
	"time"

	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// CommentListParams holds query parameters for listing comments.
type CommentListParams struct {
	Page      int
	PageSize  int
	Status    string
	ArticleID string
	Search    string
}

// CreateCommentRequest holds data for creating a comment.
type CreateCommentRequest struct {
	ArticleID   uint   `json:"article_id" binding:"required"`
	ParentID    *uint  `json:"parent_id"`
	Content     string `json:"content" binding:"required"`
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email"`
	AuthorURL   string `json:"author_url"`
}

// CommentStats holds aggregated comment statistics.
type CommentStats struct {
	Total    int64 `json:"total"`
	Pending  int64 `json:"pending"`
	Approved int64 `json:"approved"`
	Spam     int64 `json:"spam"`
	Today    int64 `json:"today"`
}

// CommentService handles comment business logic.
type CommentService struct {
	db *gorm.DB
}

// NewCommentService creates a new CommentService.
func NewCommentService(db *gorm.DB) *CommentService {
	return &CommentService{db: db}
}

// List returns comments with pagination and filters.
func (s *CommentService) List(params CommentListParams) ([]models.Comment, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}

	query := s.db.Model(&models.Comment{}).Preload("User").Preload("Article")
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.ArticleID != "" {
		query = query.Where("article_id = ?", params.ArticleID)
	}
	if params.Search != "" {
		query = query.Where("content LIKE ? OR author_name LIKE ?", "%"+params.Search+"%", "%"+params.Search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var comments []models.Comment
	err := query.Order("created_at DESC").
		Offset((params.Page - 1) * params.PageSize).Limit(params.PageSize).
		Find(&comments).Error
	if err != nil {
		return nil, 0, err
	}

	return comments, total, nil
}

// Get returns a single comment by ID.
func (s *CommentService) Get(id uint) (*models.Comment, error) {
	var comment models.Comment
	if err := s.db.Preload("User").Preload("Children").First(&comment, id).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

// Create creates a new comment.
func (s *CommentService) Create(req CreateCommentRequest, clientIP, userAgent string, userID *uint, isEditor bool) (*models.Comment, error) {
	// Verify article exists and allows comments.
	var article models.Article
	if err := s.db.First(&article, req.ArticleID).Error; err != nil {
		return nil, errors.New("article not found")
	}
	if !article.AllowComment {
		return nil, errors.New("comments are disabled for this article")
	}

	comment := models.Comment{
		ArticleID:   req.ArticleID,
		ParentID:    req.ParentID,
		Content:     req.Content,
		AuthorName:  req.AuthorName,
		AuthorEmail: req.AuthorEmail,
		AuthorURL:   req.AuthorURL,
		AuthorIP:    clientIP,
		Agent:       userAgent,
		Status:      "pending",
	}

	// If authenticated, link to user.
	if userID != nil {
		comment.UserID = userID
		// Auto-approve for editors+.
		if isEditor {
			comment.Status = "approved"
		}
	}

	// Calculate depth from parent.
	if req.ParentID != nil {
		var parent models.Comment
		if s.db.First(&parent, *req.ParentID).Error == nil {
			comment.Depth = parent.Depth + 1
		}
	}

	if err := s.db.Create(&comment).Error; err != nil {
		return nil, err
	}

	// Update article comment count.
	s.db.Model(&article).UpdateColumn("comment_count", gorm.Expr("comment_count + 1"))

	return &comment, nil
}

// Update updates a comment's content.
func (s *CommentService) Update(id uint, content string) error {
	result := s.db.Model(&models.Comment{}).Where("id = ?", id).Update("content", content)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("comment not found")
	}
	return nil
}

// UpdateStatus updates a comment's status.
func (s *CommentService) UpdateStatus(id uint, status string) error {
	result := s.db.Model(&models.Comment{}).Where("id = ?", id).Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("comment not found")
	}
	return nil
}

// BulkAction performs a bulk action on comments by IDs.
func (s *CommentService) BulkAction(ids []uint, action string) (int64, error) {
	var affected int64
	switch action {
	case "approve":
		affected = s.db.Model(&models.Comment{}).Where("id IN ?", ids).
			Update("status", "approved").RowsAffected
	case "spam":
		affected = s.db.Model(&models.Comment{}).Where("id IN ?", ids).
			Update("status", "spam").RowsAffected
	case "trash":
		affected = s.db.Model(&models.Comment{}).Where("id IN ?", ids).
			Update("status", "trash").RowsAffected
	case "delete":
		affected = s.db.Where("id IN ?", ids).Delete(&models.Comment{}).RowsAffected
	default:
		return 0, errors.New("unknown action")
	}
	return affected, nil
}

// ArticleComments returns approved top-level comments for an article with nested children.
func (s *CommentService) ArticleComments(articleID uint) ([]models.Comment, error) {
	var comments []models.Comment
	err := s.db.Where("article_id = ? AND status = ? AND parent_id IS NULL", articleID, "approved").
		Preload("User").
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			return db.Where("status = ?", "approved").Order("created_at ASC")
		}).
		Preload("Children.User").
		Order("is_sticky DESC, created_at DESC").
		Find(&comments).Error
	if err != nil {
		return nil, err
	}
	return comments, nil
}

// Stats returns aggregated comment statistics.
func (s *CommentService) Stats() (CommentStats, error) {
	var stats CommentStats
	s.db.Model(&models.Comment{}).Count(&stats.Total)
	s.db.Model(&models.Comment{}).Where("status = ?", "pending").Count(&stats.Pending)
	s.db.Model(&models.Comment{}).Where("status = ?", "approved").Count(&stats.Approved)
	s.db.Model(&models.Comment{}).Where("status = ?", "spam").Count(&stats.Spam)
	s.db.Model(&models.Comment{}).Where("DATE(created_at) = DATE(?)", time.Now()).Count(&stats.Today)
	return stats, nil
}
