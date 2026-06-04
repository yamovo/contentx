package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gosimple/slug"
	"github.com/vortexcms/go-cms/internal/database"
	"github.com/vortexcms/go-cms/internal/middleware"
	"github.com/vortexcms/go-cms/internal/models"
	"gorm.io/gorm"
)

// ArticleHandler handles article-related HTTP requests.
type ArticleHandler struct {
	db *gorm.DB
}

// NewArticleHandler creates a new article handler.
func NewArticleHandler(db *gorm.DB) *ArticleHandler {
	return &ArticleHandler{db: db}
}

// ---------- Request/Response DTOs ----------

type CreateArticleRequest struct {
	Title        string   `json:"title" binding:"required,max=512"`
	Slug         string   `json:"slug"`
	Content      string   `json:"content"`
	Excerpt      string   `json:"excerpt"`
	CategoryID   *uint    `json:"category_id"`
	TagIDs       []uint   `json:"tag_ids"`
	FeaturedImage string  `json:"featured_image"`
	Status       string   `json:"status"`
	PostType     string   `json:"post_type"`
	Format       string   `json:"format"`
	Visibility   string   `json:"visibility"`
	Password     string   `json:"password"`
	IsPinned     bool     `json:"is_pinned"`
	IsFeatured   bool     `json:"is_featured"`
	AllowComment *bool    `json:"allow_comment"`
	PublishedAt  *time.Time `json:"published_at"`
	ScheduledAt  *time.Time `json:"scheduled_at"`
	MetaTitle    string   `json:"meta_title"`
	MetaDesc     string   `json:"meta_desc"`
	MetaKeywords string   `json:"meta_keywords"`
	CanonicalURL string   `json:"canonical_url"`
	RobotsIndex  *bool    `json:"robots_index"`
	RobotsFollow *bool    `json:"robots_follow"`
	OGImage      string   `json:"og_image"`
	Template     string   `json:"template"`
	RevisionNote string   `json:"revision_note"`
}

type UpdateArticleRequest struct {
	Title        *string   `json:"title"`
	Slug         *string   `json:"slug"`
	Content      *string   `json:"content"`
	Excerpt      *string   `json:"excerpt"`
	CategoryID   *uint     `json:"category_id"`
	TagIDs       []uint    `json:"tag_ids"`
	FeaturedImage *string  `json:"featured_image"`
	Status       *string   `json:"status"`
	PostType     *string   `json:"post_type"`
	Format       *string   `json:"format"`
	Visibility   *string   `json:"visibility"`
	Password     *string   `json:"password"`
	IsPinned     *bool     `json:"is_pinned"`
	IsFeatured   *bool     `json:"is_featured"`
	AllowComment *bool     `json:"allow_comment"`
	PublishedAt  *time.Time `json:"published_at"`
	ScheduledAt  *time.Time `json:"scheduled_at"`
	MetaTitle    *string   `json:"meta_title"`
	MetaDesc     *string   `json:"meta_desc"`
	MetaKeywords *string   `json:"meta_keywords"`
	CanonicalURL *string   `json:"canonical_url"`
	RobotsIndex  *bool     `json:"robots_index"`
	RobotsFollow *bool     `json:"robots_follow"`
	OGImage      *string   `json:"og_image"`
	Template     *string   `json:"template"`
	RevisionNote string    `json:"revision_note"`
}

// ---------- Handlers ----------

// List returns a paginated list of articles.
// GET /api/v1/articles?page=1&page_size=20&status=published&category_id=1&tag=go&sort=newest
func (h *ArticleHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	postType := c.Query("post_type")
	categoryID := c.Query("category_id")
	tagSlug := c.Query("tag")
	search := c.Query("search")
	sort := c.DefaultQuery("sort", "newest")
	authorID := c.Query("author_id")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query := h.db.Model(&models.Article{}).
		Preload("Author").
		Preload("Category").
		Preload("Tags")

	// Filters.
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if postType != "" {
		query = query.Where("post_type = ?", postType)
	}
	if categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}
	if authorID != "" {
		query = query.Where("author_id = ?", authorID)
	}
	if tagSlug != "" {
		query = query.Joins("JOIN article_tags ON article_tags.article_id = articles.id").
			Joins("JOIN tags ON tags.id = article_tags.tag_id").
			Where("tags.slug = ?", tagSlug)
	}
	if search != "" {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Sorting.
	switch sort {
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
	offset := (page - 1) * pageSize
	var articles []models.Article
	if err := query.Offset(offset).Limit(pageSize).Find(&articles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch articles"})
		return
	}

	paginate := models.Paginate{Page: page, PageSize: pageSize, Total: total}
	c.JSON(http.StatusOK, models.NewListResponse(articles, paginate))
}

// Get returns a single article by ID.
// GET /api/v1/articles/:id
func (h *ArticleHandler) Get(c *gin.Context) {
	id := c.Param("id")

	var article models.Article
	if err := h.db.
		Preload("Author").
		Preload("Category").
		Preload("Tags").
		Preload("CustomFields").
		First(&article, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch article"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": article})
}

// GetBySlug returns a single article by slug (public endpoint).
// GET /api/v1/articles/slug/:slug
func (h *ArticleHandler) GetBySlug(c *gin.Context) {
	articleSlug := c.Param("slug")

	var article models.Article
	if err := h.db.
		Preload("Author").
		Preload("Category").
		Preload("Tags").
		Where("slug = ? AND status = ?", articleSlug, models.StatusPublished).
		First(&article).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch article"})
		return
	}

	// Increment view count (async).
	go func() {
		h.db.Model(&article).UpdateColumn("view_count", gorm.Expr("view_count + 1"))
	}()

	c.JSON(http.StatusOK, gin.H{"data": article})
}

// Create creates a new article.
// POST /api/v1/articles
func (h *ArticleHandler) Create(c *gin.Context) {
	var req CreateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := middleware.GetCurrentUser(c)

	article := models.Article{
		Title:          req.Title,
		Content:        req.Content,
		Excerpt:        req.Excerpt,
		AuthorID:       user.ID,
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
	article.Slug = h.ensureUniqueSlug(article.Slug, 0)

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
		h.db.Where("id IN ?", req.TagIDs).Find(&tags)
		article.Tags = tags
	}

	// Create in transaction.
	err := database.WithTransaction(h.db, func(tx *gorm.DB) error {
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
			EditorID:  user.ID,
			Version:   1,
			Note:      req.RevisionNote,
		}
		if revision.Note == "" {
			revision.Note = "Initial version"
		}
		return tx.Create(&revision).Error
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create article"})
		return
	}

	// Reload with associations.
	h.db.Preload("Author").Preload("Category").Preload("Tags").First(&article, article.ID)

	c.JSON(http.StatusCreated, gin.H{"data": article})
}

// Update updates an existing article.
// PUT /api/v1/articles/:id
func (h *ArticleHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var article models.Article
	if err := h.db.First(&article, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch article"})
		return
	}

	// Check ownership or admin.
	user := middleware.GetCurrentUser(c)
	if article.AuthorID != user.ID && !user.IsEditor() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to edit this article"})
		return
	}

	var req UpdateArticleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Apply partial updates.
	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Slug != nil {
		updates["slug"] = h.ensureUniqueSlug(*req.Slug, article.ID)
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

	err := database.WithTransaction(h.db, func(tx *gorm.DB) error {
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
			EditorID:  user.ID,
			Version:   version + 1,
			Note:      req.RevisionNote,
		}
		return tx.Create(&revision).Error
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update article"})
		return
	}

	h.db.Preload("Author").Preload("Category").Preload("Tags").First(&article, article.ID)
	c.JSON(http.StatusOK, gin.H{"data": article})
}

// Delete soft-deletes an article.
// DELETE /api/v1/articles/:id
func (h *ArticleHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	var article models.Article
	if err := h.db.First(&article, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
		return
	}

	user := middleware.GetCurrentUser(c)
	if article.AuthorID != user.ID && !user.IsEditor() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	if err := h.db.Delete(&article).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete article"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Article deleted successfully"})
}

// BulkAction handles bulk operations on articles.
// POST /api/v1/articles/bulk
func (h *ArticleHandler) BulkAction(c *gin.Context) {
	var req struct {
		ArticleIDs []uint `json:"article_ids" binding:"required"`
		Action     string `json:"action" binding:"required"`
		Status     string `json:"status"`
		CategoryID *uint  `json:"category_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := middleware.GetCurrentUser(c)
	if !user.IsEditor() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	var affected int64
	switch req.Action {
	case "publish":
		result := h.db.Model(&models.Article{}).
			Where("id IN ?", req.ArticleIDs).
			Updates(map[string]interface{}{
				"status":        models.StatusPublished,
				"published_at":  time.Now(),
			})
		affected = result.RowsAffected
	case "draft":
		result := h.db.Model(&models.Article{}).
			Where("id IN ?", req.ArticleIDs).
			Update("status", models.StatusDraft)
		affected = result.RowsAffected
	case "trash":
		result := h.db.Model(&models.Article{}).
			Where("id IN ?", req.ArticleIDs).
			Update("status", models.StatusTrash)
		affected = result.RowsAffected
	case "delete":
		result := h.db.Where("id IN ?", req.ArticleIDs).Delete(&models.Article{})
		affected = result.RowsAffected
	case "move":
		if req.CategoryID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category_id required for move action"})
			return
		}
		result := h.db.Model(&models.Article{}).
			Where("id IN ?", req.ArticleIDs).
			Update("category_id", *req.CategoryID)
		affected = result.RowsAffected
	case "pin":
		result := h.db.Model(&models.Article{}).
			Where("id IN ?", req.ArticleIDs).
			Update("is_pinned", true)
		affected = result.RowsAffected
	case "unpin":
		result := h.db.Model(&models.Article{}).
			Where("id IN ?", req.ArticleIDs).
			Update("is_pinned", false)
		affected = result.RowsAffected
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown action"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Bulk action completed",
		"action":       req.Action,
		"affected":     affected,
	})
}

// Revisions returns the revision history for an article.
// GET /api/v1/articles/:id/revisions
func (h *ArticleHandler) Revisions(c *gin.Context) {
	id := c.Param("id")

	var revisions []models.Revision
	if err := h.db.
		Preload("Editor").
		Where("article_id = ?", id).
		Order("version DESC").
		Find(&revisions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch revisions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": revisions})
}

// RestoreRevision restores an article to a specific revision.
// POST /api/v1/articles/:id/revisions/:revision_id/restore
func (h *ArticleHandler) RestoreRevision(c *gin.Context) {
	id := c.Param("id")
	revisionID := c.Param("revision_id")

	var revision models.Revision
	if err := h.db.Where("id = ? AND article_id = ?", revisionID, id).First(&revision).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Revision not found"})
		return
	}

	var article models.Article
	if err := h.db.First(&article, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
		return
	}

	user := middleware.GetCurrentUser(c)

	err := database.WithTransaction(h.db, func(tx *gorm.DB) error {
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
			EditorID:  user.ID,
			Version:   maxVersion + 1,
			Note:      "Restored from version " + strconv.Itoa(revision.Version),
		}
		return tx.Create(&newRevision).Error
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore revision"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Revision restored successfully"})
}

// LikeArticle increments the like count.
// POST /api/v1/articles/:id/like
func (h *ArticleHandler) LikeArticle(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Model(&models.Article{}).Where("id = ?", id).
		UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to like article"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Article liked"})
}

// Helper methods.

func (h *ArticleHandler) ensureUniqueSlug(s string, excludeID uint) string {
	original := s
	for i := 1; ; i++ {
		var count int64
		query := h.db.Model(&models.Article{}).Where("slug = ?", s)
		if excludeID > 0 {
			query = query.Where("id != ?", excludeID)
		}
		query.Count(&count)
		if count == 0 {
			return s
		}
		s = original + "-" + strconv.Itoa(i)
	}
}

// ---------- Public (front-end) handlers ----------

// Feed returns articles as RSS/XML.
// GET /api/v1/feed
func (h *ArticleHandler) Feed(c *gin.Context) {
	var articles []models.Article
	h.db.Where("status = ?", models.StatusPublished).
		Preload("Author").
		Preload("Category").
		Order("published_at DESC").
		Limit(20).
		Find(&articles)

	// Simple RSS 2.0 generation.
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
<channel>
<title>VortexCMS Feed</title>
<link>http://localhost:8080</link>
<description>Latest articles from VortexCMS</description>
<language>zh-cn</language>
`)
	for _, a := range articles {
		sb.WriteString("<item>\n")
		sb.WriteString("  <title>" + xmlEscape(a.Title) + "</title>\n")
		sb.WriteString("  <link>http://localhost:8080/articles/" + a.Slug + "</link>\n")
		sb.WriteString("  <pubDate>" + a.PublishedAt.Format(time.RFC1123Z) + "</pubDate>\n")
		sb.WriteString("  <description>" + xmlEscape(a.Excerpt) + "</description>\n")
		if a.Author.DisplayName != "" {
			sb.WriteString("  <author>" + xmlEscape(a.Author.Email) + " (" + xmlEscape(a.Author.DisplayName) + ")</author>\n")
		}
		sb.WriteString("  <guid>http://localhost:8080/articles/" + a.Slug + "</guid>\n")
		sb.WriteString("</item>\n")
	}
	sb.WriteString("</channel>\n</rss>")

	c.Data(http.StatusOK, "application/rss+xml; charset=utf-8", []byte(sb.String()))
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
