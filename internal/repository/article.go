package repository

import (
	"strconv"
	"strings"
	"time"

	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// ArticleListFilter holds query parameters for listing articles.
type ArticleListFilter struct {
	Page       int
	PageSize   int
	Status     string
	PostType   string
	CategoryID string
	TagSlug    string
	Search     string
	Sort       string // newest | oldest | title | views | likes
	AuthorID   string
	Locale     string // i18n: filter by locale (exact match)
}

// ArticleRepository defines data-access operations for articles, including
// the transactional Create/Update/RestoreRevision flows that previously lived
// in the service layer.
type ArticleRepository interface {
	List(filter ArticleListFilter) ([]models.Article, int64, error)
	GetByID(id uint) (*models.Article, error) // preloads Author, Category, Tags, CustomFields
	FindByID(id uint) (*models.Article, error)
	GetPublishedBySlug(slug string) (*models.Article, error) // preloads Author, Category, Tags
	IncrementViewCount(id uint) error
	IncrementLikeCount(id uint) error

	// Create inserts a new article together with its tags, tag/count and
	// category post_count bumps, and the initial revision — all in a single
	// transaction. The article pointer is mutated to hold the final state
	// (with associations preloaded).
	Create(article *models.Article, tagIDs []uint, revisionNote string, userID uint) error

	// Update applies partial updates, optional tag replacement (with tag count
	// recompute), and creates a new revision — all in a single transaction.
	// tagIDs == nil means "do not touch tags". The article pointer is mutated
	// to hold the final state (with associations preloaded).
	Update(article *models.Article, updates map[string]interface{}, tagIDs []uint, revisionNote string, userID uint) error

	Delete(article *models.Article) error

	// UpdateStatus applies a single-article status transition. When non-nil,
	// publishedAt / scheduledAt are written alongside the status so the caller
	// can flip an article to published (recording the time) or to scheduled
	// (recording the planned time) atomically.
	UpdateStatus(id uint, status string, publishedAt, scheduledAt *time.Time) error

	BulkPublish(articleIDs []uint, publishedAt time.Time) (int64, error)
	BulkUpdateStatus(articleIDs []uint, status string) (int64, error)
	BulkDelete(articleIDs []uint) (int64, error)
	BulkMoveCategory(articleIDs []uint, categoryID uint) (int64, error)
	BulkSetPinned(articleIDs []uint, pinned bool) (int64, error)

	ListRevisions(articleID uint) ([]models.Revision, error) // preloads Editor
	FindRevision(revisionID, articleID uint) (*models.Revision, error)
	RestoreRevision(article *models.Article, revision *models.Revision, userID uint) error

	ListPublishedForFeed(limit int) ([]models.Article, error) // preloads Author, Category
	// ListScheduledDue returns scheduled articles whose ScheduledAt is at or
	// before `now`, i.e. due for automatic publication. Preloads Author.
	ListScheduledDue(now time.Time) ([]models.Article, error)
	EnsureUniqueSlug(original string, excludeID uint) string

	// i18n: ListTranslations returns all articles sharing the same
	// translation group (excluding the article itself).
	ListTranslations(groupID, excludeID uint) ([]models.Article, error)
	// FindTranslationInLocale returns the article in the given translation
	// group for the requested locale, or gorm.ErrRecordNotFound.
	FindTranslationInLocale(groupID uint, locale string) (*models.Article, error)
}

// gormArticleRepository implements ArticleRepository with GORM.
type gormArticleRepository struct {
	db *gorm.DB
}

// NewArticleRepository builds a GORM-backed ArticleRepository.
func NewArticleRepository(db *gorm.DB) ArticleRepository {
	return &gormArticleRepository{db: db}
}

func (r *gormArticleRepository) List(filter ArticleListFilter) ([]models.Article, int64, error) {
	query := r.db.Model(&models.Article{}).
		Preload("Author").
		Preload("Category").
		Preload("Tags")

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.PostType != "" {
		query = query.Where("post_type = ?", filter.PostType)
	}
	if filter.CategoryID != "" {
		query = query.Where("category_id = ?", filter.CategoryID)
	}
	if filter.AuthorID != "" {
		query = query.Where("author_id = ?", filter.AuthorID)
	}
	if filter.Locale != "" {
		query = query.Where("locale = ?", filter.Locale)
	}
	if filter.TagSlug != "" {
		query = query.Joins("JOIN article_tags ON article_tags.article_id = articles.id").
			Joins("JOIN tags ON tags.id = article_tags.tag_id").
			Where("tags.slug = ?", filter.TagSlug)
	}
	if filter.Search != "" {
		escaped := strings.NewReplacer("%", "\\%", "_", "\\_").Replace(filter.Search)
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+escaped+"%", "%"+escaped+"%")
	}

	switch filter.Sort {
	case "oldest":
		query = query.Order("articles.created_at ASC")
	case "title":
		query = query.Order("articles.title ASC")
	case "views":
		query = query.Order("articles.view_count DESC")
	case "likes":
		query = query.Order("articles.like_count DESC")
	default: // newest
		query = query.Order("articles.is_pinned DESC, articles.published_at DESC, articles.created_at DESC")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var articles []models.Article
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Offset(offset).Limit(filter.PageSize).Find(&articles).Error; err != nil {
		return nil, 0, err
	}
	return articles, total, nil
}

func (r *gormArticleRepository) GetByID(id uint) (*models.Article, error) {
	var article models.Article
	if err := r.db.
		Preload("Author").
		Preload("Category").
		Preload("Tags").
		Preload("CustomFields").
		First(&article, id).Error; err != nil {
		return nil, err
	}
	return &article, nil
}

func (r *gormArticleRepository) FindByID(id uint) (*models.Article, error) {
	var article models.Article
	if err := r.db.First(&article, id).Error; err != nil {
		return nil, err
	}
	return &article, nil
}

func (r *gormArticleRepository) GetPublishedBySlug(articleSlug string) (*models.Article, error) {
	var article models.Article
	if err := r.db.
		Preload("Author").
		Preload("Category").
		Preload("Tags").
		Where("slug = ? AND status = ?", articleSlug, models.StatusPublished).
		First(&article).Error; err != nil {
		return nil, err
	}
	return &article, nil
}

func (r *gormArticleRepository) IncrementViewCount(id uint) error {
	return r.db.Model(&models.Article{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

func (r *gormArticleRepository) IncrementLikeCount(id uint) error {
	return r.db.Model(&models.Article{}).Where("id = ?", id).
		UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
}

func (r *gormArticleRepository) Create(article *models.Article, tagIDs []uint, revisionNote string, userID uint) error {
	return database.WithTransaction(r.db, func(tx *gorm.DB) error {
		// Resolve tags inside the transaction so associations are created atomically.
		if len(tagIDs) > 0 {
			var tags []models.Tag
			if err := tx.Where("id IN ?", tagIDs).Find(&tags).Error; err != nil {
				return err
			}
			article.Tags = tags
		}

		if err := tx.Create(article).Error; err != nil {
			return err
		}

		// Bump tag counts.
		for _, tag := range article.Tags {
			tx.Model(&models.Tag{}).Where("id = ?", tag.ID).
				UpdateColumn("count", gorm.Expr("count + 1"))
		}

		// Bump category post count.
		if article.CategoryID != nil {
			tx.Model(&models.Category{}).Where("id = ?", *article.CategoryID).
				UpdateColumn("post_count", gorm.Expr("post_count + 1"))
		}

		// Initial revision.
		note := revisionNote
		if note == "" {
			note = "Initial version"
		}
		revision := models.Revision{
			ArticleID: article.ID,
			Title:     article.Title,
			Content:   article.Content,
			Excerpt:   article.Excerpt,
			EditorID:  userID,
			Version:   1,
			Note:      note,
		}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}

		// Reload with associations.
		return tx.Preload("Author").Preload("Category").Preload("Tags").First(article, article.ID).Error
	})
}

func (r *gormArticleRepository) Update(article *models.Article, updates map[string]interface{}, tagIDs []uint, revisionNote string, userID uint) error {
	return database.WithTransaction(r.db, func(tx *gorm.DB) error {
		if len(updates) > 0 {
			if err := tx.Model(article).Updates(updates).Error; err != nil {
				return err
			}
		}

		// nil tagIDs means "do not touch tags".
		if tagIDs != nil {
			var tags []models.Tag
			if err := tx.Where("id IN ?", tagIDs).Find(&tags).Error; err != nil {
				return err
			}
			if err := tx.Model(article).Association("Tags").Replace(tags); err != nil {
				return err
			}
			// Recompute all tag counts (preserves prior behaviour).
			if err := tx.Exec("UPDATE tags SET count = (SELECT COUNT(*) FROM article_tags WHERE tag_id = tags.id)").Error; err != nil {
				return err
			}
		}

		// Reload for revision snapshot.
		if err := tx.First(article, article.ID).Error; err != nil {
			return err
		}

		var version int
		if err := tx.Model(&models.Revision{}).Where("article_id = ?", article.ID).
			Select("COALESCE(MAX(version), 0)").Scan(&version).Error; err != nil {
			return err
		}

		revision := models.Revision{
			ArticleID: article.ID,
			Title:     article.Title,
			Content:   article.Content,
			Excerpt:   article.Excerpt,
			EditorID:  userID,
			Version:   version + 1,
			Note:      revisionNote,
		}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}

		// Reload with associations for the caller.
		return tx.Preload("Author").Preload("Category").Preload("Tags").First(article, article.ID).Error
	})
}

func (r *gormArticleRepository) Delete(article *models.Article) error {
	return r.db.Delete(article).Error
}

func (r *gormArticleRepository) BulkPublish(articleIDs []uint, publishedAt time.Time) (int64, error) {
	result := r.db.Model(&models.Article{}).
		Where("id IN ?", articleIDs).
		Updates(map[string]interface{}{
			"status":       models.StatusPublished,
			"published_at": publishedAt,
		})
	return result.RowsAffected, result.Error
}

func (r *gormArticleRepository) UpdateStatus(id uint, status string, publishedAt, scheduledAt *time.Time) error {
	updates := map[string]interface{}{"status": status}
	if publishedAt != nil {
		updates["published_at"] = *publishedAt
	}
	if scheduledAt != nil {
		updates["scheduled_at"] = *scheduledAt
	}
	return r.db.Model(&models.Article{}).Where("id = ?", id).Updates(updates).Error
}

func (r *gormArticleRepository) BulkUpdateStatus(articleIDs []uint, status string) (int64, error) {
	result := r.db.Model(&models.Article{}).
		Where("id IN ?", articleIDs).
		Update("status", status)
	return result.RowsAffected, result.Error
}

func (r *gormArticleRepository) BulkDelete(articleIDs []uint) (int64, error) {
	result := r.db.Where("id IN ?", articleIDs).Delete(&models.Article{})
	return result.RowsAffected, result.Error
}

func (r *gormArticleRepository) BulkMoveCategory(articleIDs []uint, categoryID uint) (int64, error) {
	result := r.db.Model(&models.Article{}).
		Where("id IN ?", articleIDs).
		Update("category_id", categoryID)
	return result.RowsAffected, result.Error
}

func (r *gormArticleRepository) BulkSetPinned(articleIDs []uint, pinned bool) (int64, error) {
	result := r.db.Model(&models.Article{}).
		Where("id IN ?", articleIDs).
		Update("is_pinned", pinned)
	return result.RowsAffected, result.Error
}

func (r *gormArticleRepository) ListRevisions(articleID uint) ([]models.Revision, error) {
	var revisions []models.Revision
	if err := r.db.
		Preload("Editor").
		Where("article_id = ?", articleID).
		Order("version DESC").
		Find(&revisions).Error; err != nil {
		return nil, err
	}
	return revisions, nil
}

func (r *gormArticleRepository) FindRevision(revisionID, articleID uint) (*models.Revision, error) {
	var revision models.Revision
	if err := r.db.Where("id = ? AND article_id = ?", revisionID, articleID).First(&revision).Error; err != nil {
		return nil, err
	}
	return &revision, nil
}

func (r *gormArticleRepository) RestoreRevision(article *models.Article, revision *models.Revision, userID uint) error {
	return database.WithTransaction(r.db, func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			"title":   revision.Title,
			"content": revision.Content,
			"excerpt": revision.Excerpt,
		}
		if err := tx.Model(article).Updates(updates).Error; err != nil {
			return err
		}

		var maxVersion int
		if err := tx.Model(&models.Revision{}).Where("article_id = ?", article.ID).
			Select("COALESCE(MAX(version), 0)").Scan(&maxVersion).Error; err != nil {
			return err
		}

		newRevision := models.Revision{
			ArticleID: article.ID,
			Title:     revision.Title,
			Content:   revision.Content,
			Excerpt:   revision.Excerpt,
			EditorID:  userID,
			Version:   maxVersion + 1,
			Note:      "Restored from version " + strconv.Itoa(revision.Version),
		}
		return tx.Create(&newRevision).Error
	})
}

func (r *gormArticleRepository) ListPublishedForFeed(limit int) ([]models.Article, error) {
	var articles []models.Article
	if err := r.db.Where("status = ?", models.StatusPublished).
		Preload("Author").
		Preload("Category").
		Order("published_at DESC").
		Limit(limit).
		Find(&articles).Error; err != nil {
		return nil, err
	}
	return articles, nil
}

func (r *gormArticleRepository) ListScheduledDue(now time.Time) ([]models.Article, error) {
	var articles []models.Article
	if err := r.db.Where("status = ? AND scheduled_at <= ?", models.StatusScheduled, now).
		Preload("Author").
		Order("scheduled_at ASC").
		Find(&articles).Error; err != nil {
		return nil, err
	}
	return articles, nil
}

// EnsureUniqueSlug generates a unique slug by appending a counter if needed.
// Errors from the underlying COUNT query are silently ignored (preserves prior behaviour).
func (r *gormArticleRepository) EnsureUniqueSlug(original string, excludeID uint) string {
	candidate := original
	for i := 1; ; i++ {
		var count int64
		query := r.db.Model(&models.Article{}).Where("slug = ?", candidate)
		if excludeID > 0 {
			query = query.Where("id != ?", excludeID)
		}
		query.Count(&count)
		if count == 0 {
			return candidate
		}
		candidate = original + "-" + strconv.Itoa(i)
	}
}

// ─── i18n: translation queries ──────────────────────────────────────────────

func (r *gormArticleRepository) ListTranslations(groupID, excludeID uint) ([]models.Article, error) {
	var articles []models.Article
	// The group root has translation_group_id = NULL; its own id is the
	// group id. Match it via (id = groupID) so siblings see the root too.
	if err := r.db.Where(
		"(translation_group_id = ? OR (translation_group_id IS NULL AND id = ?)) AND id != ?",
		groupID, groupID, excludeID,
	).Order("locale ASC").Find(&articles).Error; err != nil {
		return nil, err
	}
	return articles, nil
}

func (r *gormArticleRepository) FindTranslationInLocale(groupID uint, locale string) (*models.Article, error) {
	var article models.Article
	if err := r.db.Where(
		"(translation_group_id = ? OR (translation_group_id IS NULL AND id = ?)) AND locale = ?",
		groupID, groupID, locale,
	).First(&article).Error; err != nil {
		return nil, err
	}
	return &article, nil
}
