package services

import (
	"errors"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
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
	repo    repository.CommentRepository
	webhook WebhookDispatcher
}

// NewCommentService creates a new CommentService backed by a GORM repository.
// Kept for backward compatibility with existing callers and tests.
func NewCommentService(db *gorm.DB) *CommentService {
	return &CommentService{repo: repository.NewCommentRepository(db)}
}

// NewCommentServiceWithRepo builds a CommentService with an explicit repository,
// enabling unit tests to inject mocks.
func NewCommentServiceWithRepo(repo repository.CommentRepository) *CommentService {
	return &CommentService{repo: repo}
}

// SetWebhookDispatcher attaches a webhook dispatcher for event triggering.
func (s *CommentService) SetWebhookDispatcher(d WebhookDispatcher) { s.webhook = d }

// List returns comments with pagination and filters.
func (s *CommentService) List(params CommentListParams) ([]models.Comment, int64, error) {
	return s.repo.List(repository.CommentListFilter{
		Page:      params.Page,
		PageSize:  params.PageSize,
		Status:    params.Status,
		ArticleID: params.ArticleID,
		Search:    params.Search,
	})
}

// Get returns a single comment by ID.
func (s *CommentService) Get(id uint) (*models.Comment, error) {
	return s.repo.GetByID(id)
}

// Create creates a new comment.
func (s *CommentService) Create(req CreateCommentRequest, clientIP, userAgent string, userID *uint, isEditor bool) (*models.Comment, error) {
	// Verify article exists and allows comments.
	article, err := s.repo.FindArticleByID(req.ArticleID)
	if err != nil {
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

	// Calculate depth from parent (best-effort, mirrors prior behaviour: silently ignore missing parent).
	if req.ParentID != nil {
		if parent, err := s.repo.FindCommentByID(*req.ParentID); err == nil {
			comment.Depth = parent.Depth + 1
		}
	}

	if err := s.repo.Create(&comment); err != nil {
		return nil, err
	}

	// Update article comment count (best-effort).
	_ = s.repo.IncrementArticleCommentCount(article.ID)

	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventCommentCreate, &comment)
	}

	return &comment, nil
}

// Update updates a comment's content.
func (s *CommentService) Update(id uint, content string) error {
	rowsAffected, err := s.repo.UpdateContent(id, content)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("comment not found")
	}
	return nil
}

// UpdateStatus updates a comment's status.
func (s *CommentService) UpdateStatus(id uint, status string) error {
	rowsAffected, err := s.repo.UpdateStatus(id, status)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("comment not found")
	}
	return nil
}

// BulkAction performs a bulk action on comments by IDs.
func (s *CommentService) BulkAction(ids []uint, action string) (int64, error) {
	switch action {
	case "approve":
		return s.repo.BulkUpdateStatus(ids, "approved")
	case "spam":
		return s.repo.BulkUpdateStatus(ids, "spam")
	case "trash":
		return s.repo.BulkUpdateStatus(ids, "trash")
	case "delete":
		return s.repo.BulkDelete(ids)
	default:
		return 0, errors.New("unknown action")
	}
}

// ArticleComments returns approved top-level comments for an article with nested children.
func (s *CommentService) ArticleComments(articleID uint) ([]models.Comment, error) {
	return s.repo.FindArticleComments(articleID)
}

// Stats returns aggregated comment statistics.
func (s *CommentService) Stats() (CommentStats, error) {
	data, err := s.repo.Stats()
	if err != nil {
		// Preserve prior behaviour: the original implementation never returned an error
		// from Stats() because each Count call's error was ignored. We mirror that by
		// returning a partially-populated (or zero) stats on error.
		return CommentStats{}, nil
	}
	return CommentStats{
		Total:    data.Total,
		Pending:  data.Pending,
		Approved: data.Approved,
		Spam:     data.Spam,
		Today:    data.Today,
	}, nil
}
