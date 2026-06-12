package services

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gosimple/slug"
	"github.com/vortexcms/go-cms/internal/database"
	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// ArticleService handles business logic for articles.
type ArticleService struct {
	db      *gorm.DB
	baseURL string
}

// NewArticleService creates a new ArticleService.
func NewArticleService(db *gorm.DB, baseURL string) *ArticleService {
	return &ArticleService{db: db, baseURL: baseURL}
}

// ---------- Request/Response DTOs ----------

// CreateArticleRequest is the payload for creating an article.
type CreateArticleRequest struct {
	Title         string     `json:"title" binding:"required,max=512"`
	Slug          string     `json:"slug"`
	Content       string     `json:"content"`
	Excerpt       string     `json:"excerpt"`
	CategoryID    *uint      `json:"category_id"`
	TagIDs        []uint     `json:"tag_ids"`
	FeaturedImage string     `json:"featured_image"`
	Status        string     `json:"status"`
	PostType      string     `json:"post_type"`
	Format        string     `json:"format"`
	Visibility    string     `json:"visibility"`
	Password      string     `json:"password"`
	IsPinned      bool       `json:"is_pinned"`
	IsFeatured    bool       `json:"is_featured"`
	AllowComment  *bool      `json:"allow_comment"`
	PublishedAt   *time.Time `json:"published_at"`
	ScheduledAt   *time.Time `json:"scheduled_at"`
	MetaTitle     string     `json:"meta_title"`
	MetaDesc      string     `json:"meta_desc"`
	MetaKeywords  string     `json:"meta_keywords"`
	CanonicalURL  string     `json:"canonical_url"`
	RobotsIndex   *bool      `json:"robots_index"`
	RobotsFollow  *bool      `json:"robots_follow"`
	OGImage       string     `json:"og_image"`
	Template      string     `json:"template"`
	RevisionNote  string     `json:"revision_note"`
}

// UpdateArticleRequest is the payload for updating an article.
type UpdateArticleRequest struct {
	Title         *string    `json:"title"`
	Slug          *string    `json:"slug"`
	Content       *string    `json:"content"`
	Excerpt       *string    `json:"excerpt"`
	CategoryID    *uint      `json:"category_id"`
	TagIDs        []uint     `json:"tag_ids"`
	FeaturedImage *string    `json:"featured_image"`
	Status        *string    `json:"status"`
	PostType      *string    `json:"post_type"`
	Format        *string    `json:"format"`
	Visibility    *string    `json:"visibility"`
	Password      *string    `json:"password"`
	IsPinned      *bool      `json:"is_pinned"`
	IsFeatured    *bool      `json:"is_featured"`
	AllowComment  *bool      `json:"allow_comment"`
	PublishedAt   *time.Time `json:"published_at"`
	ScheduledAt   *time.Time `json:"scheduled_at"`
	MetaTitle     *string    `json:"meta_title"`
	MetaDesc      *string    `json:"meta_desc"`
	MetaKeywords  *string    `json:"meta_keywords"`
	CanonicalURL  *string    `json:"canonical_url"`
	RobotsIndex   *bool      `json:"robots_index"`
	RobotsFollow  *bool      `json:"robots_follow"`
	OGImage       *string    `json:"og_image"`
	Template      *string    `json:"template"`
	RevisionNote  string     `json:"revision_note"`
}

// ListArticlesFilter holds query parameters for listing articles.
type ListArticlesFilter struct {
	Page       int
	PageSize   int
	Status     string
	PostType   string
	CategoryID string
	TagSlug    string
	Search     string
	Sort       string
	AuthorID   string
}

// BulkActionRequest is the payload for bulk operations on articles.
type BulkActionRequest struct {
	ArticleIDs []uint `json:"article_ids" binding:"required"`
	Action     string `json:"action" binding:"required"`
	Status     string `json:"status"`
	CategoryID *uint  `json:"category_id"`
}

// ---------- Service Methods ----------

// List returns a paginated list of articles matching the given filters.
func (s *ArticleService) List(filter ListArticlesFilter) (models.ListResponse, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	if filter.Sort == "" {
		filter.Sort = "newest"
	}

	query := s.db.Model(&models.Article{}).
		Preload("Author").
		Preload("Category").
		Preload("Tags")

	// Filters.
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
	if filter.TagSlug != "" {
		query = query.Joins("JOIN article_tags ON article_tags.article_id = articles.id").
			Joins("JOIN tags ON tags.id = article_tags.tag_id").
			Where("tags.slug = ?", filter.TagSlug)
	}
	if filter.Search != "" {
		escaped := strings.NewReplacer("%", "\\%", "_", "\\_").Replace(filter.Search)
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+escaped+"%", "%"+escaped+"%")
	}

	// Sorting.
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

	// Count.
	var total int64
	query.Count(&total)

	// Fetch.
	offset := (filter.Page - 1) * filter.PageSize
	var articles []models.Article
	if err := query.Offset(offset).Limit(filter.PageSize).Find(&articles).Error; err != nil {
		return models.ListResponse{}, err
	}

	paginate := models.Paginate{Page: filter.Page, PageSize: filter.PageSize, Total: total}
	return models.NewListResponse(articles, paginate), nil
}

// Get returns a single article by ID.
func (s *ArticleService) Get(id uint) (*models.Article, error) {
	var article models.Article
	if err := s.db.
		Preload("Author").
		Preload("Category").
		Preload("Tags").
		Preload("CustomFields").
		First(&article, id).Error; err != nil {
		return nil, err
	}
	return &article, nil
}

// GetBySlug returns a single published article by slug and increments its view count.
func (s *ArticleService) GetBySlug(articleSlug string) (*models.Article, error) {
	var article models.Article
	if err := s.db.
		Preload("Author").
		Preload("Category").
		Preload("Tags").
		Where("slug = ? AND status = ?", articleSlug, models.StatusPublished).
		First(&article).Error; err != nil {
		return nil, err
	}

	// Increment view count.
	s.db.Model(&article).UpdateColumn("view_count", gorm.Expr("view_count + 1"))

	return &article, nil
}

// Create creates a new article and its initial revision.
func (s *ArticleService) Create(req CreateArticleRequest, userID uint) (*models.Article, error) {
	article := models.Article{
		Title:          req.Title,
		Content:        req.Content,
		Excerpt:        req.Excerpt,
		AuthorID:       userID,
		CategoryID:     req.CategoryID,
		FeaturedImage:  req.FeaturedImage,
		Format:         req.Format,
		Visibility:     models.Visibility(req.Visibility),
		Password:       req.Password,
		IsPinned:       req.IsPinned,
		IsFeatured:     req.IsFeatured,
		PublishedAt:    req.PublishedAt,
		ScheduledAt:    req.ScheduledAt,
		MetaTitle:      req.MetaTitle,
		MetaDesc:       req.MetaDesc,
		MetaKeywords:   req.MetaKeywords,
		CanonicalURL:   req.CanonicalURL,
		OGImage:        req.OGImage,
		Template:       req.Template,
	}

	// Defaults.
	if req.PostType != "" {
		article.PostType = models.PostType(req.PostType)
	} else {
		article.PostType = models.PostTypePost
	}
	if req.Status != "" {
		article.Status = models.ArticleStatus(req.Status)
	} else {
		article.Status = models.StatusDraft
	}
	if req.Visibility == "" {
		article.Visibility = models.VisibilityPublic
	}
	if req.AllowComment != nil {
		article.AllowComment = *req.AllowComment
	} else {
		article.AllowComment = true
	}
	if req.RobotsIndex != nil {
		article.RobotsIndex = *req.RobotsIndex
	} else {
		article.RobotsIndex = true
	}
	if req.RobotsFollow != nil {
		article.RobotsFollow = *req.RobotsFollow
	} else {
		article.RobotsFollow = true
	}

	// Generate slug.
	if req.Slug != "" {
		article.Slug = req.Slug
	} else {
		article.Slug = slug.MakeLang(req.Title, "zh")
		if article.Slug == "" {
			article.Slug = slug.Make(req.Title)
		}
	}
	// Ensure unique slug.
	article.Slug = s.ensureUniqueSlug(article.Slug, 0)

	// Calculate reading time & excerpt.
	article.CalcReadingTime()
	article.MakeExcerpt(200)

	// Set publish time if publishing.
	if article.Status == models.StatusPublished && article.PublishedAt == nil {
		now := time.Now()
		article.PublishedAt = &now
	}

	// Tags.
	if len(req.TagIDs) > 0 {
		var tags []models.Tag
		s.db.Where("id IN ?", req.TagIDs).Find(&tags)
		article.Tags = tags
	}

	// Create in transaction.
	err := database.WithTransaction(s.db, func(tx *gorm.DB) error {
		if err := tx.Create(&article).Error; err != nil {
			return err
		}

		// Update tag counts.
		if len(article.Tags) > 0 {
			for _, tag := range article.Tags {
				tx.Model(&models.Tag{}).Where("id = ?", tag.ID).
					UpdateColumn("count", gorm.Expr("count + 1"))
			}
		}

		// Update category post count.
		if article.CategoryID != nil {
			tx.Model(&models.Category{}).Where("id = ?", *article.CategoryID).
				UpdateColumn("post_count", gorm.Expr("post_count + 1"))
		}

		// Create initial revision.
		revision := models.Revision{
			ArticleID: article.ID,
			Title:     article.Title,
			Content:   article.Content,
			Excerpt:   article.Excerpt,
			EditorID:  userID,
			Version:   1,
			Note:      req.RevisionNote,
		}
		if revision.Note == "" {
			revision.Note = "Initial version"
		}
		return tx.Create(&revision).Error
	})

	if err != nil {
		return nil, err
	}

	// Reload with associations.
	s.db.Preload("Author").Preload("Category").Preload("Tags").First(&article, article.ID)

	return &article, nil
}

// Update updates an existing article. The caller must verify ownership or editor status.
func (s *ArticleService) Update(id uint, req UpdateArticleRequest, userID uint, isEditor bool) (*models.Article, error) {
	var article models.Article
	if err := s.db.First(&article, id).Error; err != nil {
		return nil, err
	}

	// Check ownership or admin/editor.
	if article.AuthorID != userID && !isEditor {
		return nil, &ForbiddenError{Message: "Not authorized to edit this article"}
	}

	// Apply partial updates.
	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Slug != nil {
		updates["slug"] = s.ensureUniqueSlug(*req.Slug, article.ID)
	}
	if req.Content != nil {
		updates["content"] = *req.Content
	}
	if req.Excerpt != nil {
		updates["excerpt"] = *req.Excerpt
	}
	if req.CategoryID != nil {
		updates["category_id"] = *req.CategoryID
	}
	if req.FeaturedImage != nil {
		updates["featured_image"] = *req.FeaturedImage
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.PostType != nil {
		updates["post_type"] = *req.PostType
	}
	if req.Format != nil {
		updates["format"] = *req.Format
	}
	if req.Visibility != nil {
		updates["visibility"] = *req.Visibility
	}
	if req.Password != nil {
		updates["password"] = *req.Password
	}
	if req.IsPinned != nil {
		updates["is_pinned"] = *req.IsPinned
	}
	if req.IsFeatured != nil {
		updates["is_featured"] = *req.IsFeatured
	}
	if req.AllowComment != nil {
		updates["allow_comment"] = *req.AllowComment
	}
	if req.PublishedAt != nil {
		updates["published_at"] = *req.PublishedAt
	}
	if req.ScheduledAt != nil {
		updates["scheduled_at"] = *req.ScheduledAt
	}
	if req.MetaTitle != nil {
		updates["meta_title"] = *req.MetaTitle
	}
	if req.MetaDesc != nil {
		updates["meta_desc"] = *req.MetaDesc
	}
	if req.MetaKeywords != nil {
		updates["meta_keywords"] = *req.MetaKeywords
	}
	if req.CanonicalURL != nil {
		updates["canonical_url"] = *req.CanonicalURL
	}
	if req.RobotsIndex != nil {
		updates["robots_index"] = *req.RobotsIndex
	}
	if req.RobotsFollow != nil {
		updates["robots_follow"] = *req.RobotsFollow
	}
	if req.OGImage != nil {
		updates["og_image"] = *req.OGImage
	}
	if req.Template != nil {
		updates["template"] = *req.Template
	}

	err := database.WithTransaction(s.db, func(tx *gorm.DB) error {
		if len(updates) > 0 {
			if err := tx.Model(&article).Updates(updates).Error; err != nil {
				return err
			}
		}

		// Update tags if provided.
		if req.TagIDs != nil {
			var tags []models.Tag
			tx.Where("id IN ?", req.TagIDs).Find(&tags)
			if err := tx.Model(&article).Association("Tags").Replace(tags); err != nil {
				return err
			}
			// Recalculate tag counts.
			tx.Exec("UPDATE tags SET count = (SELECT COUNT(*) FROM article_tags WHERE tag_id = tags.id)")
		}

		// Create revision.
		tx.First(&article, article.ID)
		var version int
		tx.Model(&models.Revision{}).Where("article_id = ?", article.ID).
			Select("COALESCE(MAX(version), 0)").Scan(&version)
		revision := models.Revision{
			ArticleID: article.ID,
			Title:     article.Title,
			Content:   article.Content,
			Excerpt:   article.Excerpt,
			EditorID:  userID,
			Version:   version + 1,
			Note:      req.RevisionNote,
		}
		return tx.Create(&revision).Error
	})

	if err != nil {
		return nil, err
	}

	s.db.Preload("Author").Preload("Category").Preload("Tags").First(&article, article.ID)
	return &article, nil
}

// Delete soft-deletes an article. The caller must verify ownership or editor status.
func (s *ArticleService) Delete(id uint, userID uint, isEditor bool) error {
	var article models.Article
	if err := s.db.First(&article, id).Error; err != nil {
		return err
	}

	// Check ownership or admin/editor.
	if article.AuthorID != userID && !isEditor {
		return &ForbiddenError{Message: "Not authorized"}
	}

	return s.db.Delete(&article).Error
}

// BulkAction performs a bulk operation on a set of articles. Requires editor privileges.
func (s *ArticleService) BulkAction(req BulkActionRequest) (int64, error) {
	var affected int64

	switch req.Action {
	case "publish":
		result := s.db.Model(&models.Article{}).
			Where("id IN ?", req.ArticleIDs).
			Updates(map[string]interface{}{
				"status":       models.StatusPublished,
				"published_at": time.Now(),
			})
		affected = result.RowsAffected
		if result.Error != nil {
			return 0, result.Error
		}
	case "draft":
		result := s.db.Model(&models.Article{}).
			Where("id IN ?", req.ArticleIDs).
			Update("status", models.StatusDraft)
		affected = result.RowsAffected
		if result.Error != nil {
			return 0, result.Error
		}
	case "trash":
		result := s.db.Model(&models.Article{}).
			Where("id IN ?", req.ArticleIDs).
			Update("status", models.StatusTrash)
		affected = result.RowsAffected
		if result.Error != nil {
			return 0, result.Error
		}
	case "delete":
		result := s.db.Where("id IN ?", req.ArticleIDs).Delete(&models.Article{})
		affected = result.RowsAffected
		if result.Error != nil {
			return 0, result.Error
		}
	case "move":
		if req.CategoryID == nil {
			return 0, &BadRequestError{Message: "category_id required for move action"}
		}
		result := s.db.Model(&models.Article{}).
			Where("id IN ?", req.ArticleIDs).
			Update("category_id", *req.CategoryID)
		affected = result.RowsAffected
		if result.Error != nil {
			return 0, result.Error
		}
	case "pin":
		result := s.db.Model(&models.Article{}).
			Where("id IN ?", req.ArticleIDs).
			Update("is_pinned", true)
		affected = result.RowsAffected
		if result.Error != nil {
			return 0, result.Error
		}
	case "unpin":
		result := s.db.Model(&models.Article{}).
			Where("id IN ?", req.ArticleIDs).
			Update("is_pinned", false)
		affected = result.RowsAffected
		if result.Error != nil {
			return 0, result.Error
		}
	default:
		return 0, &BadRequestError{Message: "Unknown action"}
	}

	return affected, nil
}

// Revisions returns the revision history for an article.
func (s *ArticleService) Revisions(articleID uint) ([]models.Revision, error) {
	var revisions []models.Revision
	if err := s.db.
		Preload("Editor").
		Where("article_id = ?", articleID).
		Order("version DESC").
		Find(&revisions).Error; err != nil {
		return nil, err
	}
	return revisions, nil
}

// RestoreRevision restores an article to a specific revision and creates a new revision recording the restore.
func (s *ArticleService) RestoreRevision(articleID uint, revisionID uint, userID uint) error {
	var revision models.Revision
	if err := s.db.Where("id = ? AND article_id = ?", revisionID, articleID).First(&revision).Error; err != nil {
		return err
	}

	var article models.Article
	if err := s.db.First(&article, articleID).Error; err != nil {
		return err
	}

	return database.WithTransaction(s.db, func(tx *gorm.DB) error {
		// Restore content.
		updates := map[string]interface{}{
			"title":   revision.Title,
			"content": revision.Content,
			"excerpt": revision.Excerpt,
		}
		if err := tx.Model(&article).Updates(updates).Error; err != nil {
			return err
		}

		// Create a new revision recording the restore.
		var maxVersion int
		tx.Model(&models.Revision{}).Where("article_id = ?", article.ID).
			Select("COALESCE(MAX(version), 0)").Scan(&maxVersion)
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

// LikeArticle increments the like count for an article.
func (s *ArticleService) LikeArticle(id uint) error {
	return s.db.Model(&models.Article{}).Where("id = ?", id).
		UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
}

// GenerateFeed produces an RSS 2.0 XML string of the latest published articles.
func (s *ArticleService) GenerateFeed() (string, error) {
	var articles []models.Article
	if err := s.db.Where("status = ?", models.StatusPublished).
		Preload("Author").
		Preload("Category").
		Order("published_at DESC").
		Limit(20).
		Find(&articles).Error; err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
<channel>
<title>ContentX Feed</title>
<link>` + s.baseURL + `</link>
<description>Latest articles from ContentX</description>
<language>zh-cn</language>
`)
	for _, a := range articles {
		articleURL := s.baseURL + "/articles/" + a.Slug
		sb.WriteString("<item>\n")
		sb.WriteString("  <title>" + xmlEscape(a.Title) + "</title>\n")
		sb.WriteString("  <link>" + articleURL + "</link>\n")
		sb.WriteString("  <pubDate>" + a.PublishedAt.Format(time.RFC1123Z) + "</pubDate>\n")
		sb.WriteString("  <description>" + xmlEscape(a.Excerpt) + "</description>\n")
		if a.Author.DisplayName != "" {
			sb.WriteString("  <author>" + xmlEscape(a.Author.Email) + " (" + xmlEscape(a.Author.DisplayName) + ")</author>\n")
		}
		sb.WriteString("  <guid>" + articleURL + "</guid>\n")
		sb.WriteString("</item>\n")
	}
	sb.WriteString("</channel>\n</rss>")

	return sb.String(), nil
}

// ---------- Private Helpers ----------

// ensureUniqueSlug appends a numeric suffix to slug s until it is unique.
// excludeID should be 0 when creating, or the article's own ID when updating.
func (s *ArticleService) ensureUniqueSlug(s2 string, excludeID uint) string {
	original := s2
	for i := 1; ; i++ {
		var count int64
		query := s.db.Model(&models.Article{}).Where("slug = ?", s2)
		if excludeID > 0 {
			query = query.Where("id != ?", excludeID)
		}
		query.Count(&count)
		if count == 0 {
			return s2
		}
		s2 = original + "-" + strconv.Itoa(i)
	}
}

// xmlEscape escapes special XML characters in a string.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// ---------- Custom Errors ----------

// ForbiddenError indicates the user lacks permission for the requested action.
type ForbiddenError struct {
	Message string
}

func (e *ForbiddenError) Error() string {
	return e.Message
}

// StatusCode returns HTTP 403.
func (e *ForbiddenError) StatusCode() int {
	return http.StatusForbidden
}

// BadRequestError indicates an invalid request.
type BadRequestError struct {
	Message string
}

func (e *BadRequestError) Error() string {
	return e.Message
}

// StatusCode returns HTTP 400.
func (e *BadRequestError) StatusCode() int {
	return http.StatusBadRequest
}
