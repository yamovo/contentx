package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gosimple/slug"
	"github.com/yamovo/contentx/internal/errs"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/plugin"
	"github.com/yamovo/contentx/internal/repository"
	"gorm.io/gorm"
)

// ArticleService handles business logic for articles.
type ArticleService struct {
	repo    repository.ArticleRepository
	baseURL string
	webhook WebhookDispatcher
	plugins *plugin.Manager
	search  SearchIndexer // optional; defaults to NoopIndexer when unset
}

// NewArticleService creates a new ArticleService backed by a GORM repository.
// Kept for backward compatibility with existing callers and tests.
func NewArticleService(db *gorm.DB, baseURL string) *ArticleService {
	return &ArticleService{repo: repository.NewArticleRepository(db), baseURL: baseURL}
}

// NewArticleServiceWithRepo builds an ArticleService with an explicit repository,
// enabling unit tests to inject mocks.
func NewArticleServiceWithRepo(repo repository.ArticleRepository, baseURL string) *ArticleService {
	return &ArticleService{repo: repo, baseURL: baseURL}
}

// SetWebhookDispatcher attaches a webhook dispatcher for event triggering.
func (s *ArticleService) SetWebhookDispatcher(d WebhookDispatcher) { s.webhook = d }

// SetPluginManager attaches a plugin manager for hook dispatch.
func (s *ArticleService) SetPluginManager(m *plugin.Manager) { s.plugins = m }

// SetSearchIndexer attaches a full-text search indexer. When nil or unset the
// service uses NoopIndexer so write paths don't need nil checks.
func (s *ArticleService) SetSearchIndexer(idx SearchIndexer) {
	if idx == nil {
		idx = NoopIndexer()
	}
	s.search = idx
}

// indexer returns the configured SearchIndexer (NoopIndexer if unset).
func (s *ArticleService) indexer() SearchIndexer {
	if s.search == nil {
		return NoopIndexer()
	}
	return s.search
}

// indexArticle pushes the article into the search index. Best-effort:
// errors are logged but never returned to the caller, since search is a
// secondary concern and should not break a successful write.
func (s *ArticleService) indexArticle(article *models.Article) {
	idx := s.indexer()
	if idx == nil {
		return
	}
	doc := ArticleToSearchDoc(article)
	if err := idx.Index(context.Background(), doc); err != nil {
		slog.Warn("search index failed", "article_id", article.ID, "error", err)
	}
}

// unindexArticle removes an article from the search index (best-effort).
func (s *ArticleService) unindexArticle(id uint, postType models.PostType) {
	idx := s.indexer()
	if idx == nil {
		return
	}
	docType := "article"
	if postType == models.PostTypePage {
		docType = "page"
	}
	if err := idx.Delete(context.Background(), id, docType); err != nil {
		slog.Warn("search unindex failed", "article_id", id, "error", err)
	}
}

// reindexByID reloads the article with associations preloaded (via GetByID)
// and pushes it into the search index. Used by status-transition paths
// (Publish/Unpublish/Schedule/Archive) where the in-memory article came
// from FindByID (no preloads), so the indexed document would otherwise lose
// author/category/tag metadata.
//
// Skipped entirely when the indexer is NoopIndexer (search disabled) to
// avoid the extra GetByID DB round-trip.
func (s *ArticleService) reindexByID(id uint) {
	idx := s.indexer()
	if idx == nil || idx.Name() == "noop" {
		return
	}
	article, err := s.repo.GetByID(id)
	if err != nil {
		slog.Warn("search reindex: reload failed", "article_id", id, "error", err)
		return
	}
	s.indexArticle(article)
}

// fireAction dispatches an action hook if a plugin manager is attached.
func (s *ArticleService) fireAction(hook string, args map[string]interface{}) {
	if s.plugins != nil {
		s.plugins.ExecuteAction(hook, args)
	}
}

// applyContentFilter runs the article.filterContent filter hook to allow
// plugins to transform the content before it is saved.
func (s *ArticleService) applyContentFilter(content string) string {
	if s.plugins == nil {
		return content
	}
	v, err := s.plugins.ApplyFilter("article.filterContent", content, nil)
	if err != nil {
		slog.Error("plugin filter failed", "hook", "article.filterContent", "error", err)
		return content
	}
	if s, ok := v.(string); ok {
		return s
	}
	return content
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
	Locale        string     `json:"locale"` // i18n: BCP-47 tag, defaults to "en"
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
	Locale     string // i18n: filter by locale (exact match)
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

	articles, total, err := s.repo.List(repository.ArticleListFilter{
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		Status:     filter.Status,
		PostType:   filter.PostType,
		CategoryID: filter.CategoryID,
		TagSlug:    filter.TagSlug,
		Search:     filter.Search,
		Sort:       filter.Sort,
		AuthorID:   filter.AuthorID,
		Locale:     filter.Locale,
	})
	if err != nil {
		return models.ListResponse{}, err
	}

	paginate := models.Paginate{Page: filter.Page, PageSize: filter.PageSize, Total: total}
	return models.NewListResponse(articles, paginate), nil
}

// Search runs a full-text query against the configured SearchIndexer and
// returns ranked hits. Public callers should pass Status="published" so the
// search surface only exposes published content; admin callers may omit it
// to search across all statuses.
func (s *ArticleService) Search(ctx context.Context, q SearchQuery) (*SearchResult, error) {
	return s.indexer().Search(ctx, q)
}

// ReindexAll rebuilds the search index from scratch using all articles in
// the database. Intended for startup warm-up or admin-triggered reindex.
func (s *ArticleService) ReindexAll(ctx context.Context) (int, error) {
	// Pull all articles with associations preloaded in batches, then hand
	// the full slice to the indexer's ReindexAll (which atomically clears
	// and rebuilds). Collecting in memory is fine for typical CMS scale
	// (< 100k articles); a streaming approach would be needed beyond that.
	// List deliberately caps public page sizes at 100. Keep the reindex batch
	// within that contract so a larger requested size is not silently reset to
	// the default of 20 and mistaken for the final page.
	const batchSize = 100
	var all []models.Article
	page := 1
	for {
		resp, err := s.List(ListArticlesFilter{Page: page, PageSize: batchSize, Sort: "oldest"})
		if err != nil {
			return 0, err
		}
		articles, ok := resp.Items.([]models.Article)
		if !ok || len(articles) == 0 {
			break
		}
		all = append(all, articles...)
		if len(articles) < batchSize {
			break
		}
		page++
	}
	if len(all) == 0 {
		return 0, nil
	}
	if err := s.indexer().ReindexAll(ctx, all); err != nil {
		return 0, err
	}
	return len(all), nil
}

// Get returns a single article by ID.
func (s *ArticleService) Get(id uint) (*models.Article, error) {
	return s.repo.GetByID(id)
}

// GetBySlug returns a single published article by slug and increments its view count.
func (s *ArticleService) GetBySlug(articleSlug string) (*models.Article, error) {
	article, err := s.repo.GetPublishedBySlug(articleSlug)
	if err != nil {
		return nil, err
	}

	// Increment view count (best-effort, preserves prior behaviour).
	_ = s.repo.IncrementViewCount(article.ID)
	article.ViewCount++

	return article, nil
}

// Create creates a new article and its initial revision.
func (s *ArticleService) Create(req CreateArticleRequest, userID uint) (*models.Article, error) {
	article := models.Article{
		Title:         req.Title,
		Content:       req.Content,
		Excerpt:       req.Excerpt,
		AuthorID:      userID,
		CategoryID:    req.CategoryID,
		FeaturedImage: req.FeaturedImage,
		Format:        req.Format,
		Visibility:    models.Visibility(req.Visibility),
		Password:      req.Password,
		IsPinned:      req.IsPinned,
		IsFeatured:    req.IsFeatured,
		PublishedAt:   req.PublishedAt,
		ScheduledAt:   req.ScheduledAt,
		MetaTitle:     req.MetaTitle,
		MetaDesc:      req.MetaDesc,
		MetaKeywords:  req.MetaKeywords,
		CanonicalURL:  req.CanonicalURL,
		OGImage:       req.OGImage,
		Template:      req.Template,
	}

	// Defaults.
	if req.PostType != "" {
		article.PostType = models.PostType(req.PostType)
	} else {
		article.PostType = models.PostTypePost
	}
	if req.Locale != "" {
		article.Locale = req.Locale
	} else {
		article.Locale = "en"
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
	article.Slug = s.repo.EnsureUniqueSlug(article.Slug, 0)

	// Allow plugins to transform the content before reading-time calculation.
	article.Content = s.applyContentFilter(article.Content)

	// Calculate reading time & excerpt.
	article.CalcReadingTime()
	article.MakeExcerpt(200)

	// Set publish time if publishing.
	if article.Status == models.StatusPublished && article.PublishedAt == nil {
		now := time.Now()
		article.PublishedAt = &now
	}

	if err := s.repo.Create(&article, req.TagIDs, req.RevisionNote, userID); err != nil {
		return nil, err
	}

	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventEntryCreate, &article)
	}

	s.indexArticle(&article)

	s.fireAction("article.afterCreate", map[string]interface{}{
		"article": &article,
		"title":   article.Title,
		"content": article.Content,
		"user_id": userID,
	})

	return &article, nil
}

// ─── i18n: translation helpers ──────────────────────────────────────────────

// effectiveGroupID returns the translation group id for an article. When the
// article was created without an explicit group (the common case for the first
// locale), its own ID serves as the group root.
func effectiveGroupID(a *models.Article) uint {
	if a.TranslationGroupID != nil {
		return *a.TranslationGroupID
	}
	return a.ID
}

// ListTranslations returns all sibling translations of the given article
// (excluding the article itself). The article's own locale is not included.
func (s *ArticleService) ListTranslations(articleID uint) ([]models.Article, error) {
	article, err := s.repo.FindByID(articleID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListTranslations(effectiveGroupID(article), articleID)
}

// CreateTranslation creates a new article as a translation of an existing one.
// The new article inherits the source's category, tags, and translation group,
// but gets its own title/content/slug and the requested locale.
func (s *ArticleService) CreateTranslation(sourceID uint, locale string, req CreateArticleRequest, userID uint) (*models.Article, error) {
	if locale == "" {
		return nil, errs.ErrBadRequest.WithMessage("locale is required for translation")
	}
	source, err := s.repo.FindByID(sourceID)
	if err != nil {
		return nil, err
	}
	// Refuse duplicate locale within the same group.
	if existing, err := s.repo.FindTranslationInLocale(effectiveGroupID(source), locale); err == nil && existing != nil {
		return nil, errs.ErrConflict.WithMessage("translation already exists for this locale")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Build the translated article from the source's metadata.
	article := models.Article{
		Title:              req.Title,
		Content:            req.Content,
		Excerpt:            req.Excerpt,
		AuthorID:           userID,
		CategoryID:         source.CategoryID,
		FeaturedImage:      source.FeaturedImage,
		Format:             source.Format,
		Visibility:         source.Visibility,
		IsPinned:           source.IsPinned,
		IsFeatured:         source.IsFeatured,
		MetaTitle:          req.MetaTitle,
		MetaDesc:           req.MetaDesc,
		MetaKeywords:       req.MetaKeywords,
		CanonicalURL:       req.CanonicalURL,
		OGImage:            source.OGImage,
		Template:           source.Template,
		PostType:           source.PostType,
		Locale:             locale,
		TranslationGroupID: new(uint),
	}
	*article.TranslationGroupID = effectiveGroupID(source)

	// Status defaults to draft; published_at only if explicitly published.
	if req.Status != "" {
		article.Status = models.ArticleStatus(req.Status)
	} else {
		article.Status = models.StatusDraft
	}
	if article.Status == models.StatusPublished && article.PublishedAt == nil {
		now := time.Now()
		article.PublishedAt = &now
	}
	if req.PostType != "" {
		article.PostType = models.PostType(req.PostType)
	}
	if req.Visibility != "" {
		article.Visibility = models.Visibility(req.Visibility)
	}
	article.AllowComment = source.AllowComment
	article.RobotsIndex = source.RobotsIndex
	article.RobotsFollow = source.RobotsFollow

	// Slug: use provided, else derive from title; ensure unique.
	if req.Slug != "" {
		article.Slug = req.Slug
	} else {
		article.Slug = slug.MakeLang(req.Title, "zh")
		if article.Slug == "" {
			article.Slug = slug.Make(req.Title)
		}
	}
	article.Slug = s.repo.EnsureUniqueSlug(article.Slug, 0)

	article.CalcReadingTime()
	article.MakeExcerpt(200)

	// Inherit tags from source unless overridden.
	tagIDs := req.TagIDs
	if tagIDs == nil {
		tagIDs = make([]uint, 0, len(source.Tags))
		for _, t := range source.Tags {
			tagIDs = append(tagIDs, t.ID)
		}
	}

	if err := s.repo.Create(&article, tagIDs, req.RevisionNote, userID); err != nil {
		return nil, err
	}

	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventEntryCreate, &article)
	}
	s.indexArticle(&article)
	return &article, nil
}

// Update updates an existing article. The caller must verify ownership or editor status.
func (s *ArticleService) Update(id uint, req UpdateArticleRequest, userID uint, isEditor bool) (*models.Article, error) {
	article, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	// Check ownership or admin/editor.
	if article.AuthorID != userID && !isEditor {
		return nil, errs.ErrForbidden.WithMessage("Not authorized to edit this article")
	}

	// Apply partial updates.
	updates := map[string]interface{}{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Slug != nil {
		updates["slug"] = s.repo.EnsureUniqueSlug(*req.Slug, article.ID)
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
		target := models.ArticleStatus(*req.Status)
		if !models.AllowedTransition(article.Status, target) {
			return nil, errs.ErrBadRequest.WithMessage(
				fmt.Sprintf("illegal status transition: %s → %s", article.Status, target))
		}
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

	// tagIDs == nil means "do not touch tags"; an empty slice means "clear tags".
	var tagIDs []uint
	if req.TagIDs != nil {
		tagIDs = req.TagIDs
	}

	if err := s.repo.Update(article, updates, tagIDs, req.RevisionNote, userID); err != nil {
		return nil, err
	}

	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventEntryUpdate, article)
	}

	s.indexArticle(article)

	s.fireAction("article.afterUpdate", map[string]interface{}{
		"article": article,
		"user_id": userID,
	})

	return article, nil
}

// Delete soft-deletes an article. The caller must verify ownership or editor status.
func (s *ArticleService) Delete(id uint, userID uint, isEditor bool) error {
	article, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	// Check ownership or admin/editor.
	if article.AuthorID != userID && !isEditor {
		return errs.ErrForbidden.WithMessage("Not authorized")
	}

	if err := s.repo.Delete(article); err != nil {
		return err
	}

	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventEntryDelete, article)
	}

	s.unindexArticle(article.ID, article.PostType)

	s.fireAction("article.afterDelete", map[string]interface{}{
		"article_id": id,
		"user_id":    userID,
	})
	return nil
}

// BulkAction performs a bulk operation on a set of articles. Requires editor privileges.
func (s *ArticleService) BulkAction(req BulkActionRequest) (int64, error) {
	var event string
	switch req.Action {
	case "publish":
		event = models.WebhookEventEntryPublish
	case "delete":
		event = models.WebhookEventEntryDelete
	}

	n, err := s.bulkActionRepo(req)
	if err != nil {
		return n, err
	}

	if s.webhook != nil && event != "" && n > 0 {
		s.webhook.Dispatch(event, map[string]interface{}{
			"ids":    req.ArticleIDs,
			"action": req.Action,
			"count":  n,
		})
	}
	return n, nil
}

// bulkActionRepo dispatches to the repository without webhook side-effects.
func (s *ArticleService) bulkActionRepo(req BulkActionRequest) (int64, error) {
	switch req.Action {
	case "publish":
		return s.repo.BulkPublish(req.ArticleIDs, time.Now())
	case "draft":
		return s.repo.BulkUpdateStatus(req.ArticleIDs, string(models.StatusDraft))
	case "trash":
		return s.repo.BulkUpdateStatus(req.ArticleIDs, string(models.StatusTrash))
	case "delete":
		return s.repo.BulkDelete(req.ArticleIDs)
	case "move":
		if req.CategoryID == nil {
			return 0, errs.ErrBadRequest.WithMessage("category_id required for move action")
		}
		return s.repo.BulkMoveCategory(req.ArticleIDs, *req.CategoryID)
	case "pin":
		return s.repo.BulkSetPinned(req.ArticleIDs, true)
	case "unpin":
		return s.repo.BulkSetPinned(req.ArticleIDs, false)
	default:
		return 0, errs.ErrBadRequest.WithMessage("Unknown action")
	}
}

// Revisions returns the revision history for an article.
func (s *ArticleService) Revisions(articleID uint) ([]models.Revision, error) {
	return s.repo.ListRevisions(articleID)
}

// RestoreRevision restores an article to a specific revision and creates a new revision recording the restore.
func (s *ArticleService) RestoreRevision(articleID uint, revisionID uint, userID uint) error {
	revision, err := s.repo.FindRevision(revisionID, articleID)
	if err != nil {
		return err
	}

	article, err := s.repo.FindByID(articleID)
	if err != nil {
		return err
	}

	if err := s.repo.RestoreRevision(article, revision, userID); err != nil {
		return err
	}
	// Content changed: re-index with full preloaded metadata.
	s.reindexByID(articleID)
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Publication workflow (P2-3)
//
// 以下方法封装文章的状态机流转：草稿→审核→发布、定时发布、归档、取消发布。
// 每个方法都校验状态流转合法性（models.AllowedTransition），通过后委托
// repo.UpdateStatus 原子更新，并触发对应的 webhook 事件。
// ──────────────────────────────────────────────────────────────────────────────

// transitionTo validates and applies a status transition for a single article.
// It loads the article, checks the state machine, applies the update via the
// repository, and returns the reloaded article. The caller is responsible for
// webhook dispatch (so it can choose the right event name).
func (s *ArticleService) transitionTo(id uint, target models.ArticleStatus, publishedAt, scheduledAt *time.Time) (*models.Article, error) {
	article, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if !models.AllowedTransition(article.Status, target) {
		return nil, errs.ErrBadRequest.WithMessage(
			fmt.Sprintf("illegal status transition: %s → %s", article.Status, target))
	}
	if err := s.repo.UpdateStatus(id, string(target), publishedAt, scheduledAt); err != nil {
		return nil, err
	}
	// Reload to reflect the persisted state (FindByID does not preload; for
	// webhook payloads the bare fields are sufficient).
	updated, err := s.repo.FindByID(id)
	if err != nil {
		return article, nil // best-effort: return pre-update snapshot
	}
	// Status changes affect search visibility (e.g. draft→published makes the
	// article publicly searchable). Re-index with full preloaded metadata.
	s.reindexByID(id)
	return updated, nil
}

// Publish flips an article to published status, recording the publish time if
// it has none. Triggers the entry.publish webhook event.
func (s *ArticleService) Publish(id uint) (*models.Article, error) {
	var publishedAt *time.Time
	// Only set PublishedAt if the article doesn't already have one. We need
	// to inspect the current article to decide, so load it first.
	current, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if current.PublishedAt == nil {
		now := time.Now()
		publishedAt = &now
	}
	// Allow publishing from draft/pending/scheduled/trash.
	if !models.AllowedTransition(current.Status, models.StatusPublished) {
		return nil, errs.ErrBadRequest.WithMessage(
			fmt.Sprintf("illegal status transition: %s → published", current.Status))
	}
	if err := s.repo.UpdateStatus(id, string(models.StatusPublished), publishedAt, nil); err != nil {
		return nil, err
	}
	updated, err := s.repo.FindByID(id)
	if err != nil {
		return current, nil
	}
	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventEntryPublish, updated)
	}
	s.reindexByID(id)
	return updated, nil
}

// Unpublish reverts a published/scheduled article back to draft. Triggers the
// entry.unpublish webhook event.
func (s *ArticleService) Unpublish(id uint) (*models.Article, error) {
	updated, err := s.transitionTo(id, models.StatusDraft, nil, nil)
	if err != nil {
		return nil, err
	}
	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventEntryUnpublish, updated)
	}
	return updated, nil
}

// SubmitForReview moves a draft into the pending (review) queue.
func (s *ArticleService) SubmitForReview(id uint) (*models.Article, error) {
	return s.transitionTo(id, models.StatusPending, nil, nil)
}

// Approve marks a pending article as published, recording the publish time if
// it has none. Triggers the entry.publish webhook event.
func (s *ArticleService) Approve(id uint) (*models.Article, error) {
	return s.Publish(id) // pending → published reuses the Publish path
}

// Schedule marks an article for automatic publication at the given time. The
// article stays non-public (status=scheduled) until the PublishScheduler flips
// it. Triggers the entry.schedule webhook event.
func (s *ArticleService) Schedule(id uint, at time.Time) (*models.Article, error) {
	if at.IsZero() {
		return nil, errs.ErrBadRequest.WithMessage("scheduled_at is required")
	}
	current, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if !models.AllowedTransition(current.Status, models.StatusScheduled) {
		return nil, errs.ErrBadRequest.WithMessage(
			fmt.Sprintf("illegal status transition: %s → scheduled", current.Status))
	}
	if err := s.repo.UpdateStatus(id, string(models.StatusScheduled), nil, &at); err != nil {
		return nil, err
	}
	updated, err := s.repo.FindByID(id)
	if err != nil {
		return current, nil
	}
	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventEntrySchedule, updated)
	}
	s.reindexByID(id)
	return updated, nil
}

// Archive moves an article out of the active lifecycle.
func (s *ArticleService) Archive(id uint) (*models.Article, error) {
	return s.transitionTo(id, models.StatusArchived, nil, nil)
}

// PublishDueScheduled publishes all scheduled articles whose ScheduledAt is at
// or before now. Returns the number of articles flipped. Used by the
// PublishScheduler worker.
func (s *ArticleService) PublishDueScheduled(now time.Time) (int, error) {
	due, err := s.repo.ListScheduledDue(now)
	if err != nil {
		return 0, err
	}
	if len(due) == 0 {
		return 0, nil
	}
	ids := make([]uint, 0, len(due))
	for _, a := range due {
		ids = append(ids, a.ID)
	}
	n, err := s.repo.BulkPublish(ids, now)
	if err != nil {
		return 0, err
	}
	if s.webhook != nil && n > 0 {
		s.webhook.Dispatch(models.WebhookEventEntryPublish, map[string]interface{}{
			"ids":   ids,
			"count": n,
			"mode":  "scheduled",
		})
	}
	// Re-index auto-published articles so they become publicly searchable.
	for _, id := range ids {
		s.reindexByID(id)
	}
	return int(n), nil
}

// LikeArticle increments the like count for an article.
func (s *ArticleService) LikeArticle(id uint) error {
	return s.repo.IncrementLikeCount(id)
}

// GenerateFeed produces an RSS 2.0 XML string of the latest published articles.
func (s *ArticleService) GenerateFeed() (string, error) {
	articles, err := s.repo.ListPublishedForFeed(20)
	if err != nil {
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
//
// ArticleService 原有的 ForbiddenError / BadRequestError 已移除，统一使用
// errs.ErrForbidden / errs.ErrBadRequest 的 WithMessage 副本。这使所有错误
// 走 handleServiceError 的 AppError 分支，前端可通过 err_code 字段获得
// 稳定的错误码（FORBIDDEN / BAD_REQUEST）。
