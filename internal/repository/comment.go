package repository

import (
	"strings"
	"time"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// CommentListFilter holds query parameters for listing comments.
type CommentListFilter struct {
	Page      int
	PageSize  int
	Status    string
	ArticleID string
	Search    string
}

// CommentStatsData holds raw comment counts returned by the repository.
// The service layer is responsible for mapping these to its public CommentStats DTO.
type CommentStatsData struct {
	Total    int64
	Pending  int64
	Approved int64
	Spam     int64
	Today    int64
}

// CommentRepository defines data-access operations for comments.
type CommentRepository interface {
	List(filter CommentListFilter) ([]models.Comment, int64, error)
	GetByID(id uint) (*models.Comment, error) // preloads User + Children
	FindArticleByID(articleID uint) (*models.Article, error)
	FindCommentByID(id uint) (*models.Comment, error) // returns gorm.ErrRecordNotFound if missing
	Create(comment *models.Comment) error
	UpdateContent(id uint, content string) (rowsAffected int64, err error)
	UpdateStatus(id uint, status string) (rowsAffected int64, err error)
	BulkUpdateStatus(ids []uint, status string) (rowsAffected int64, err error)
	BulkDelete(ids []uint) (rowsAffected int64, err error)
	FindArticleComments(articleID uint) ([]models.Comment, error) // scoped Children preload
	IncrementArticleCommentCount(articleID uint) error
	Stats() (CommentStatsData, error)
	CountToday() (int64, error)
}

// gormCommentRepository implements CommentRepository with GORM.
type gormCommentRepository struct {
	db *gorm.DB
}

// NewCommentRepository builds a GORM-backed CommentRepository.
func NewCommentRepository(db *gorm.DB) CommentRepository {
	return &gormCommentRepository{db: db}
}

func (r *gormCommentRepository) List(filter CommentListFilter) ([]models.Comment, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	query := r.db.Model(&models.Comment{}).Preload("User").Preload("Article")
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.ArticleID != "" {
		query = query.Where("article_id = ?", filter.ArticleID)
	}
	if filter.Search != "" {
		escaped := strings.NewReplacer("%", "\\%", "_", "\\_").Replace(filter.Search)
		query = query.Where("content LIKE ? OR author_name LIKE ?", "%"+escaped+"%", "%"+escaped+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var comments []models.Comment
	if err := query.Order("created_at DESC").
		Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).
		Find(&comments).Error; err != nil {
		return nil, 0, err
	}

	return comments, total, nil
}

func (r *gormCommentRepository) GetByID(id uint) (*models.Comment, error) {
	var comment models.Comment
	if err := r.db.Preload("User").Preload("Children").First(&comment, id).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *gormCommentRepository) FindArticleByID(articleID uint) (*models.Article, error) {
	var article models.Article
	if err := r.db.First(&article, articleID).Error; err != nil {
		return nil, err
	}
	return &article, nil
}

func (r *gormCommentRepository) FindCommentByID(id uint) (*models.Comment, error) {
	var comment models.Comment
	if err := r.db.First(&comment, id).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *gormCommentRepository) Create(comment *models.Comment) error {
	return r.db.Create(comment).Error
}

func (r *gormCommentRepository) UpdateContent(id uint, content string) (int64, error) {
	result := r.db.Model(&models.Comment{}).Where("id = ?", id).Update("content", content)
	return result.RowsAffected, result.Error
}

func (r *gormCommentRepository) UpdateStatus(id uint, status string) (int64, error) {
	result := r.db.Model(&models.Comment{}).Where("id = ?", id).Update("status", status)
	return result.RowsAffected, result.Error
}

func (r *gormCommentRepository) BulkUpdateStatus(ids []uint, status string) (int64, error) {
	result := r.db.Model(&models.Comment{}).Where("id IN ?", ids).Update("status", status)
	return result.RowsAffected, result.Error
}

func (r *gormCommentRepository) BulkDelete(ids []uint) (int64, error) {
	result := r.db.Where("id IN ?", ids).Delete(&models.Comment{})
	return result.RowsAffected, result.Error
}

func (r *gormCommentRepository) FindArticleComments(articleID uint) ([]models.Comment, error) {
	var comments []models.Comment
	err := r.db.Where("article_id = ? AND status = ? AND parent_id IS NULL", articleID, "approved").
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

func (r *gormCommentRepository) IncrementArticleCommentCount(articleID uint) error {
	return r.db.Model(&models.Article{}).Where("id = ?", articleID).
		UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error
}

func (r *gormCommentRepository) CountToday() (int64, error) {
	var count int64
	if err := r.db.Model(&models.Comment{}).Where("DATE(created_at) = DATE(?)", time.Now()).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Stats runs the same five COUNT queries as the original service implementation.
// Individual query errors are ignored to preserve prior behaviour (best-effort stats).
func (r *gormCommentRepository) Stats() (CommentStatsData, error) {
	var stats CommentStatsData

	if err := r.db.Model(&models.Comment{}).Count(&stats.Total).Error; err != nil {
		return stats, err
	}
	if err := r.db.Model(&models.Comment{}).Where("status = ?", "pending").Count(&stats.Pending).Error; err != nil {
		return stats, err
	}
	if err := r.db.Model(&models.Comment{}).Where("status = ?", "approved").Count(&stats.Approved).Error; err != nil {
		return stats, err
	}
	if err := r.db.Model(&models.Comment{}).Where("status = ?", "spam").Count(&stats.Spam).Error; err != nil {
		return stats, err
	}
	today, err := r.CountToday()
	if err != nil {
		return stats, err
	}
	stats.Today = today

	return stats, nil
}
