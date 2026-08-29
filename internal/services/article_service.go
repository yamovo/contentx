package services

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/yamovo/contentx/internal/cache"
	"github.com/yamovo/contentx/internal/errs"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/plugin"
	"github.com/yamovo/contentx/internal/repository"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

// ArticleService handles business logic for articles.
type ArticleService struct {
	repo     repository.ArticleRepository
	baseURL  string
	webhook  WebhookDispatcher
	plugins  *plugin.Manager
	search   SearchIndexer // optional; defaults to NoopIndexer when unset
	rag      RAGIndexer    // optional; defaults to noopRAGIndexer when unset
	cache    cache.Driver
	cacheTTL time.Duration
	cacheGen uint64
	flight   singleflight.Group // SEC-9: collapses concurrent cache-miss loads
	audit    AuditLogger        // business-level audit; defaults to NoopAuditLogger
}

const (
	// defaultExcerptLength is the maximum character count for auto-generated excerpts.
	defaultExcerptLength = 200
	// defaultFeedSize is the number of articles included in the RSS feed.
	defaultFeedSize = 20
)

// NewArticleService creates a new ArticleService backed by a GORM repository.
// Kept for backward compatibility with existing callers and tests.
func NewArticleService(db *gorm.DB, baseURL string) *ArticleService {
	return &ArticleService{repo: repository.NewArticleRepository(db), baseURL: baseURL, audit: NoopAuditLogger{}}
}

// NewArticleServiceWithRepo builds an ArticleService with an explicit repository,
// enabling unit tests to inject mocks.
func NewArticleServiceWithRepo(repo repository.ArticleRepository, baseURL string) *ArticleService {
	return &ArticleService{repo: repo, baseURL: baseURL, audit: NoopAuditLogger{}}
}

// SetWebhookDispatcher attaches a webhook dispatcher for event triggering.
func (s *ArticleService) SetWebhookDispatcher(d WebhookDispatcher) { s.webhook = d }

// SetAuditLogger wires the business-level audit logger.
func (s *ArticleService) SetAuditLogger(l AuditLogger) {
	if l != nil {
		s.audit = l
	}
}

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

// SetRAGIndexer injects a RAG indexer so article create/update/delete events
// keep the vector index in sync. Pass nil to disable RAG indexing.
func (s *ArticleService) SetRAGIndexer(idx RAGIndexer) {
	if idx == nil {
		idx = noopRAGIndexer{}
	}
	s.rag = idx
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
	if idx != nil {
		doc := ArticleToSearchDoc(article)
		s.retryIndexOp("search index", article.ID, func() error {
			return idx.Index(context.Background(), doc)
		})
	}
	// RAG vector index (best-effort; failures logged but non-fatal).
	if s.rag != nil {
		s.retryIndexOp("rag index", article.ID, func() error {
			return s.rag.IndexArticle(context.Background(), article)
		})
	}
}

// unindexArticle removes an article from the search index (best-effort).
func (s *ArticleService) unindexArticle(id uint, postType models.PostType, tenantID uint) {
	idx := s.indexer()
	if idx != nil {
		docType := "article"
		if postType == models.PostTypePage {
			docType = "page"
		}
		s.retryIndexOp("search unindex", id, func() error {
			return idx.Delete(context.Background(), id, docType, tenantID)
		})
	}
	// RAG vector index (best-effort).
	if s.rag != nil {
		s.retryIndexOp("rag unindex", id, func() error {
			return s.rag.DeleteArticle(context.Background(), id, tenantID)
		})
	}
}

// retryIndexOp runs a best-effort index write with a small bounded retry so
// transient backend failures self-heal instead of leaving stale index state
// until the next full reindex (the convergence backstop). Permanent policy
// rejections (outbound disabled) are never retried.
func (s *ArticleService) retryIndexOp(op string, articleID uint, fn func() error) {
	delays := []time.Duration{0, 200 * time.Millisecond, 500 * time.Millisecond}
	var err error
	for i, delay := range delays {
		if i > 0 {
			time.Sleep(delay)
		}
		if err = fn(); err == nil {
			return
		}
		if errors.Is(err, ErrAIOutboundDisabled) {
			slog.Warn(op+" refused by outbound policy", "article_id", articleID)
			return
		}
	}
	slog.Warn(op+" failed after retries", "article_id", articleID, "error", err)
}

// reindexByID re-indexes a single article by ID (used after scheduled publish).
// reindexByID reloads the article with associations preloaded (via GetByID)
// and pushes it into the search index. Used by status-transition paths
// (Publish/Unpublish/Schedule/Archive) where the in-memory article came
// from FindByID (no preloads), so the indexed document would otherwise lose
// author/category/tag metadata.
//
// Skipped entirely when both the search indexer and RAG indexer are no-op
// to avoid the extra GetByID DB round-trip.
func (s *ArticleService) reindexByID(id, tenantID uint) {
	idx := s.indexer()
	ragActive := s.rag != nil
	if (idx == nil || idx.Name() == "noop") && !ragActive {
		return
	}
	article, err := s.repo.GetByID(id, tenantID)
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
	Status        string     `json:"-"`
	PostType      string     `json:"post_type"`
	Format        string     `json:"format"`
	Visibility    string     `json:"visibility"`
	Password      string     `json:"password"`
	IsPinned      bool       `json:"is_pinned"`
	IsFeatured    bool       `json:"is_featured"`
	AllowComment  *bool      `json:"allow_comment"`
	PublishedAt   *time.Time `json:"-"`
	ScheduledAt   *time.Time `json:"-"`
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
	Title           *string    `json:"title"`
	Slug            *string    `json:"slug"`
	Content         *string    `json:"content"`
	Excerpt         *string    `json:"excerpt"`
	CategoryID      *uint      `json:"category_id"`
	TagIDs          []uint     `json:"tag_ids"`
	FeaturedImage   *string    `json:"featured_image"`
	Status          *string    `json:"-"`
	PostType        *string    `json:"-"`
	Format          *string    `json:"format"`
	Visibility      *string    `json:"visibility"`
	Password        *string    `json:"password"`
	IsPinned        *bool      `json:"is_pinned"`
	IsFeatured      *bool      `json:"is_featured"`
	AllowComment    *bool      `json:"allow_comment"`
	PublishedAt     *time.Time `json:"-"`
	ScheduledAt     *time.Time `json:"-"`
	MetaTitle       *string    `json:"meta_title"`
	MetaDesc        *string    `json:"meta_desc"`
	MetaKeywords    *string    `json:"meta_keywords"`
	CanonicalURL    *string    `json:"canonical_url"`
	RobotsIndex     *bool      `json:"robots_index"`
	RobotsFollow    *bool      `json:"robots_follow"`
	OGImage         *string    `json:"og_image"`
	Template        *string    `json:"template"`
	RevisionNote    string     `json:"revision_note"`
	ExpectedVersion *int       `json:"expected_version" binding:"required,min=1"` // 乐观锁：客户端读取时的 version，不匹配返回 409
}

// ListArticlesFilter holds query parameters for listing articles.
type ListArticlesFilter struct {
	Page       int
	PageSize   int
	Status     string
	Visibility string
	PostType   string
	CategoryID string
	TagSlug    string
	Search     string
	Sort       string
	AuthorID   string
	Locale     string // i18n: filter by locale (exact match)
	Full       bool   // when true the response includes the full Content body
}

// BulkActionRequest is the payload for bulk operations on articles.
type BulkActionRequest struct {
	ArticleIDs []uint `json:"article_ids" binding:"required"`
	Action     string `json:"action" binding:"required"`
	Status     string `json:"status"`
	CategoryID *uint  `json:"category_id"`
}

// ---------- Service Methods ----------

// List returns a paginated list of articles matching the given filters,
// scoped to the request tenant (RFC-001 §5).
func (s *ArticleService) List(filter ListArticlesFilter, tenantID uint) (models.ListResponse, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	if filter.Sort == "" {
		filter.Sort = "newest"
	}

	// Cache check (keys are tenant-scoped, RFC-001 §6).
	if s.cache == nil {
		return s.listUncached(filter, "", tenantID)
	}
	cacheKey := s.listCacheKey(filter, tenantID)
	if cached, hit := s.cacheGetList(cacheKey); hit {
		return cached, nil
	}
	// SEC-9: single-flight — concurrent misses on the same key share one
	// repo query instead of stampeding the database (cache breakdown).
	v, err, _ := s.flight.Do(cacheKey, func() (interface{}, error) {
		// Re-check inside the flight: an earlier winner may have filled it.
		if cached, hit := s.cacheGetList(cacheKey); hit {
			return cached, nil
		}
		return s.listUncached(filter, cacheKey, tenantID)
	})
	if err != nil {
		return models.ListResponse{}, err
	}
	resp, ok := v.(models.ListResponse)
	if !ok {
		return models.ListResponse{}, fmt.Errorf("article service: unexpected singleflight type %T", v)
	}
	return resp, nil
}

// listUncached queries the repository directly and (when cacheKey is
// non-empty) stores the result in cache.
func (s *ArticleService) listUncached(filter ListArticlesFilter, cacheKey string, tenantID uint) (models.ListResponse, error) {
	articles, total, err := s.repo.List(repository.ArticleListFilter{
		Page:       filter.Page,
		PageSize:   filter.PageSize,
		Status:     filter.Status,
		Visibility: filter.Visibility,
		PostType:   filter.PostType,
		CategoryID: filter.CategoryID,
		TagSlug:    filter.TagSlug,
		Search:     filter.Search,
		Sort:       filter.Sort,
		AuthorID:   filter.AuthorID,
		Locale:     filter.Locale,
		Full:       filter.Full,
	}, tenantID)
	if err != nil {
		return models.ListResponse{}, err
	}

	paginate := models.Paginate{Page: filter.Page, PageSize: filter.PageSize, Total: total}
	resp := models.NewListResponse(articles, paginate)
	if cacheKey != "" {
		s.cacheSetList(cacheKey, resp)
	}
	return resp, nil
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
//
// Multi-tenancy note: currently rebuilds the default tenant only. A
// tenant-by-tenant pass lands with search-index tenant scoping (RFC-001 §6,
// PR-4); behaviour is unchanged while all data lives in the default tenant.
func (s *ArticleService) ReindexAll(ctx context.Context) (int, error) {
	// Pull all articles directly from the repository in batches, then hand the
	// full slice to the indexer's ReindexAll (which atomically clears and
	// rebuilds). Reindexing must bypass the service cache: after a database
	// restore Redis may still contain list results from the pre-restore state.
	// Collecting in memory is fine for typical CMS scale (< 100k articles); a
	// streaming approach would be needed beyond that.
	// List deliberately caps public page sizes at 100. Keep the reindex batch
	// within that contract so a larger requested size is not silently reset to
	// the default of 20 and mistaken for the final page.
	const batchSize = 100
	var all []models.Article
	page := 1
	for {
		articles, _, err := s.repo.List(repository.ArticleListFilter{
			Page: page, PageSize: batchSize, Sort: "oldest", Full: true,
		}, models.DefaultTenantID)
		if err != nil {
			return 0, err
		}
		if len(articles) == 0 {
			break
		}
		all = append(all, articles...)
		if len(articles) < batchSize {
			break
		}
		page++
	}
	if err := s.indexer().ReindexAll(ctx, all); err != nil {
		return 0, err
	}
	return len(all), nil
}

// Get returns a single article by ID within the request tenant.
func (s *ArticleService) Get(id, tenantID uint) (*models.Article, error) {
	if a := s.cacheGetArticle(id, tenantID); a != nil {
		return a, nil
	}
	// SEC-9: single-flight — concurrent misses on the same article share one
	// DB load instead of stampeding the database.
	v, err, _ := s.flight.Do(articleCacheKey(id, tenantID), func() (interface{}, error) {
		if a := s.cacheGetArticle(id, tenantID); a != nil {
			return a, nil
		}
		a, err := s.repo.GetByID(id, tenantID)
		if err != nil {
			return nil, err
		}
		s.cacheSetArticle(a)
		return a, nil
	})
	if err != nil {
		return nil, err
	}
	article, ok := v.(*models.Article)
	if !ok {
		return nil, fmt.Errorf("article service: unexpected singleflight type %T", v)
	}
	return article, nil
}

// GetBySlug returns a single published article by slug within the request
// tenant and increments its view count.
func (s *ArticleService) GetBySlug(articleSlug string, tenantID uint) (*models.Article, error) {
	article, err := s.repo.GetPublishedBySlug(articleSlug, tenantID)
	if err != nil {
		return nil, err
	}

	// Increment view count (best-effort, preserves prior behaviour).
	_ = s.repo.IncrementViewCount(article.ID, tenantID)
	article.ViewCount++

	return article, nil
}

// Create creates a new article and its initial revision within the tenant.
func (s *ArticleService) Create(req CreateArticleRequest, tenantID, userID uint) (*models.Article, error) {
	article := models.Article{
		TenantID:      tenantID, // RFC-001 §5: created rows always carry the request tenant
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
	// Creation always produces a draft. Publishing and scheduling are
	// privileged lifecycle operations exposed through dedicated endpoints.
	article.Status = models.StatusDraft
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
		article.Slug = models.GenerateSlug(req.Title)
	}
	// Ensure unique slug (tenant-scoped uniqueness, RFC-001 §4.4).
	uniqueSlug, err := s.repo.EnsureUniqueSlug(article.Slug, 0, tenantID)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	article.Slug = uniqueSlug

	// Allow plugins to transform the content before reading-time calculation.
	article.Content = s.applyContentFilter(article.Content)

	// Calculate reading time & excerpt.
	article.CalcReadingTime()
	article.MakeExcerpt(defaultExcerptLength)

	if err := s.repo.Create(&article, req.TagIDs, req.RevisionNote, userID); err != nil {
		return nil, err
	}

	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventEntryCreate, &article, tenantID)
	}

	s.indexArticle(&article)

	s.fireAction("article.afterCreate", map[string]interface{}{
		"article": &article,
		"title":   article.Title,
		"content": article.Content,
		"user_id": userID,
	})

	s.invalidateArticle(tenantID, article.ID)

	uid := userID
	tid := tenantID
	s.audit.Log(AuditEvent{
		UserID: &uid, TenantID: &tid, Action: "article.create", Entity: "article", EntityID: article.ID,
		Details: map[string]any{
			"title": article.Title, "slug": article.Slug,
			"post_type": string(article.PostType), "status": string(article.Status),
		},
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
// (excluding the article itself), scoped to the tenant.
func (s *ArticleService) ListTranslations(articleID, tenantID uint) ([]models.Article, error) {
	article, err := s.repo.FindByID(articleID, tenantID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListTranslations(effectiveGroupID(article), articleID, tenantID)
}

// CreateTranslation creates a new article as a translation of an existing one.
// The new article inherits the source's category, tags, and translation group,
// but gets its own title/content/slug and the requested locale.
func (s *ArticleService) CreateTranslation(sourceID uint, locale string, req CreateArticleRequest, tenantID, userID uint) (*models.Article, error) {
	if locale == "" {
		return nil, errs.ErrBadRequest.WithMessage("locale is required for translation")
	}
	source, err := s.repo.FindByID(sourceID, tenantID)
	if err != nil {
		return nil, err
	}
	// Refuse duplicate locale within the same group.
	if existing, err := s.repo.FindTranslationInLocale(effectiveGroupID(source), locale, tenantID); err == nil && existing != nil {
		return nil, errs.ErrConflict.WithMessage("translation already exists for this locale")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Build the translated article from the source's metadata.
	article := models.Article{
		Title:              req.Title,
		TenantID:           tenantID,
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
		article.Slug = models.GenerateSlug(req.Title)
	}
	uniqueSlug, err := s.repo.EnsureUniqueSlug(article.Slug, 0, tenantID)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	article.Slug = uniqueSlug

	article.CalcReadingTime()
	article.MakeExcerpt(defaultExcerptLength)

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
		s.webhook.Dispatch(models.WebhookEventEntryCreate, &article, tenantID)
	}
	uid := userID
	tid := tenantID
	s.audit.Log(AuditEvent{
		UserID: &uid, TenantID: &tid, Action: "article.create", Entity: "article", EntityID: article.ID,
		Details: map[string]any{
			"title": article.Title, "slug": article.Slug,
			"post_type": string(article.PostType), "status": string(article.Status),
		},
	})
	s.indexArticle(&article)
	return &article, nil
}

// Update updates an existing article within the tenant. The caller must
// verify ownership or editor status.
func (s *ArticleService) Update(id uint, req UpdateArticleRequest, tenantID, userID uint, isEditor bool) (*models.Article, error) {
	article, err := s.repo.FindByID(id, tenantID)
	if err != nil {
		return nil, err
	}

	// Check ownership or admin/editor.
	if article.AuthorID != userID && !isEditor {
		return nil, errs.ErrForbidden.WithMessage("Not authorized to edit this article")
	}

	updates, tagIDs, err := s.buildUpdateMap(article, req, tenantID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Update(article, updates, tagIDs, req.RevisionNote, userID, req.ExpectedVersion, tenantID); err != nil {
		if errors.Is(err, repository.ErrConcurrentModification) {
			return nil, errs.ErrConcurrentModification
		}
		return nil, err
	}

	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventEntryUpdate, article, tenantID)
	}

	s.indexArticle(article)

	s.fireAction("article.afterUpdate", map[string]interface{}{
		"article": article,
		"user_id": userID,
	})

	uid := userID
	tid := tenantID
	s.audit.Log(AuditEvent{
		UserID: &uid, TenantID: &tid, Action: "article.update", Entity: "article", EntityID: article.ID,
		Details: map[string]any{
			"title": article.Title, "slug": article.Slug, "version": article.Version,
		},
	})
	s.invalidateArticle(tenantID, article.ID)
	return article, nil
}

// buildUpdateMap translates an UpdateArticleRequest into a GORM partial-update
// map, validating status transitions and slug uniqueness along the way. Simple
// nullable fields are handled generically via setIf; fields with special logic
// (slug, status) are handled explicitly. Returns (updates, tagIDs, err) where
// tagIDs is nil when the request does not touch tags.
func (s *ArticleService) buildUpdateMap(article *models.Article, req UpdateArticleRequest, tenantID uint) (map[string]interface{}, []uint, error) {
	updates := map[string]interface{}{}

	// Fields with special logic.
	if req.Slug != nil {
		uniqueSlug, err := s.repo.EnsureUniqueSlug(*req.Slug, article.ID, tenantID)
		if err != nil {
			return nil, nil, errs.ErrInternal.Wrap(err)
		}
		updates["slug"] = uniqueSlug
	}
	// Simple nullable fields — copy through when present.
	setIf(updates, "title", req.Title)
	setIf(updates, "content", req.Content)
	setIf(updates, "excerpt", req.Excerpt)
	setIf(updates, "category_id", req.CategoryID)
	setIf(updates, "featured_image", req.FeaturedImage)
	setIf(updates, "format", req.Format)
	setIf(updates, "visibility", req.Visibility)
	setIf(updates, "password", req.Password)
	setIf(updates, "is_pinned", req.IsPinned)
	setIf(updates, "is_featured", req.IsFeatured)
	setIf(updates, "allow_comment", req.AllowComment)
	setIf(updates, "meta_title", req.MetaTitle)
	setIf(updates, "meta_desc", req.MetaDesc)
	setIf(updates, "meta_keywords", req.MetaKeywords)
	setIf(updates, "canonical_url", req.CanonicalURL)
	setIf(updates, "robots_index", req.RobotsIndex)
	setIf(updates, "robots_follow", req.RobotsFollow)
	setIf(updates, "og_image", req.OGImage)
	setIf(updates, "template", req.Template)

	// tagIDs == nil means "do not touch tags"; an empty slice means "clear tags".
	var tagIDs []uint
	if req.TagIDs != nil {
		tagIDs = req.TagIDs
	}
	return updates, tagIDs, nil
}

// setIf assigns *v to updates[key] when v is non-nil. Generic helper used by
// buildUpdateMap to avoid 20+ repetitive nil-check blocks.
func setIf[T any](updates map[string]interface{}, key string, v *T) {
	if v != nil {
		updates[key] = *v
	}
}

// Delete soft-deletes an article within the tenant. The caller must verify
// ownership or editor status.
func (s *ArticleService) Delete(id, tenantID, userID uint, isEditor bool) error {
	article, err := s.repo.FindByID(id, tenantID)
	if err != nil {
		return err
	}

	// Check ownership or admin/editor.
	if article.AuthorID != userID && !isEditor {
		return errs.ErrForbidden.WithMessage("Not authorized")
	}

	if err := s.repo.Delete(article, tenantID); err != nil {
		return err
	}

	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventEntryDelete, article, tenantID)
	}

	s.unindexArticle(article.ID, article.PostType, tenantID)

	s.fireAction("article.afterDelete", map[string]interface{}{
		"article_id": id,
		"user_id":    userID,
	})
	uid := userID
	tid := tenantID
	s.audit.Log(AuditEvent{
		UserID: &uid, TenantID: &tid,
		Action: "article.delete", Entity: "article", EntityID: id,
		Details: map[string]any{"title": article.Title, "slug": article.Slug},
	})
	s.invalidateArticle(tenantID, id)
	return nil
}

// BulkAction performs a bulk operation on a set of articles within the
// tenant. Requires editor privileges.
func (s *ArticleService) BulkAction(req BulkActionRequest, tenantID uint) (int64, error) {
	var event string
	switch req.Action {
	case "publish":
		event = models.WebhookEventEntryPublish
	case "delete":
		event = models.WebhookEventEntryDelete
	}

	n, err := s.bulkActionRepo(req, tenantID)
	if err != nil {
		return n, err
	}

	if s.webhook != nil && event != "" && n > 0 {
		s.webhook.Dispatch(event, map[string]interface{}{
			"ids":    req.ArticleIDs,
			"action": req.Action,
			"count":  n,
		}, tenantID)
	}
	s.syncBulkRAG(req.Action, req.ArticleIDs, tenantID)
	s.invalidateArticle(tenantID, req.ArticleIDs...)
	return n, nil
}

// syncBulkRAG keeps the RAG vector index in sync after a bulk action.
// Publish re-indexes articles (IndexArticle only indexes published status);
// draft/trash/delete removes them from the vector index.
func (s *ArticleService) syncBulkRAG(action string, ids []uint, tenantID uint) {
	if s.rag == nil {
		return
	}
	switch action {
	case "publish":
		for _, id := range ids {
			s.reindexByID(id, tenantID)
		}
	case "draft", "trash", "delete":
		for _, id := range ids {
			s.unindexArticle(id, models.PostTypePost, tenantID)
		}
	}
}

// bulkActionRepo dispatches to the repository without webhook side-effects.
func (s *ArticleService) bulkActionRepo(req BulkActionRequest, tenantID uint) (int64, error) {
	switch req.Action {
	case "publish":
		return s.repo.BulkPublish(req.ArticleIDs, time.Now(), tenantID)
	case "draft":
		return s.repo.BulkUpdateStatus(req.ArticleIDs, string(models.StatusDraft), tenantID)
	case "trash":
		return s.repo.BulkUpdateStatus(req.ArticleIDs, string(models.StatusTrash), tenantID)
	case "delete":
		return s.repo.BulkDelete(req.ArticleIDs, tenantID)
	case "move":
		if req.CategoryID == nil {
			return 0, errs.ErrBadRequest.WithMessage("category_id required for move action")
		}
		return s.repo.BulkMoveCategory(req.ArticleIDs, *req.CategoryID, tenantID)
	case "pin":
		return s.repo.BulkSetPinned(req.ArticleIDs, true, tenantID)
	case "unpin":
		return s.repo.BulkSetPinned(req.ArticleIDs, false, tenantID)
	default:
		return 0, errs.ErrBadRequest.WithMessage("Unknown action")
	}
}

// Revisions returns the revision history for an article within the tenant.
func (s *ArticleService) Revisions(articleID, tenantID uint) ([]models.Revision, error) {
	return s.repo.ListRevisions(articleID, tenantID)
}

// RestoreRevision restores an article to a specific revision and creates a
// new revision recording the restore, scoped to the tenant.
func (s *ArticleService) RestoreRevision(articleID, revisionID, tenantID, userID uint, isEditor bool) error {
	revision, err := s.repo.FindRevision(revisionID, articleID, tenantID)
	if err != nil {
		return err
	}

	article, err := s.repo.FindByID(articleID, tenantID)
	if err != nil {
		return err
	}

	// Check ownership or admin/editor (SEC-3: prevent IDOR on revision restore).
	if article.AuthorID != userID && !isEditor {
		return errs.ErrForbidden.WithMessage("Not authorized to restore revisions of this article")
	}

	if err := s.repo.RestoreRevision(article, revision, userID, tenantID); err != nil {
		return err
	}
	// Content changed: re-index with full preloaded metadata.
	s.reindexByID(articleID, tenantID)
	s.invalidateArticle(tenantID, articleID)
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
// transitionAuditAction maps a target status to the audit action recorded for
// the transition. All lifecycle transitions go through transitionTo, so each
// operation produces exactly one business audit event (RESEARCH-001 §4).
func transitionAuditAction(target models.ArticleStatus) string {
	switch target {
	case models.StatusPublished:
		return "article.publish"
	case models.StatusDraft:
		return "article.unpublish"
	case models.StatusPending:
		return "article.submit_review"
	case models.StatusScheduled:
		return "article.schedule"
	case models.StatusArchived:
		return "article.archive"
	case models.StatusTrash:
		return "article.trash"
	default:
		return "article.status_change"
	}
}

func (s *ArticleService) transitionTo(id uint, target models.ArticleStatus, publishedAt, scheduledAt *time.Time, tenantID uint, userID *uint) (*models.Article, error) {
	article, err := s.repo.FindByID(id, tenantID)
	if err != nil {
		return nil, err
	}
	if !models.AllowedTransition(article.Status, target) {
		return nil, errs.ErrBadRequest.WithMessage(
			fmt.Sprintf("illegal status transition: %s → %s", article.Status, target))
	}
	// The status change and its audit record commit atomically: a transition
	// cannot land without its audit trail (fail-closed, RESEARCH-001 §4).
	// There is no HTTP request context at the service layer, so the event
	// carries no request/trace IDs; the ActivityLogger middleware separately
	// records the same request with full correlation.
	actorType := ActorUser
	if userID == nil {
		actorType = ActorSystem
	}
	tenant := tenantID
	auditLog := buildLog(AuditEvent{
		UserID:   userID,
		TenantID: &tenant,
		Action:   transitionAuditAction(target),
		Entity:   "article",
		EntityID: id,
		Details: map[string]any{
			"title": article.Title, "slug": article.Slug,
			"from": string(article.Status), "to": string(target),
		},
		Source:    SourceREST,
		ActorType: actorType,
		Outcome:   OutcomeSuccess,
	})
	if err := s.repo.UpdateStatusWithAudit(id, string(target), publishedAt, scheduledAt, tenantID, auditLog); err != nil {
		return nil, err
	}
	// Reload to reflect the persisted state (FindByID does not preload; for
	// webhook payloads the bare fields are sufficient).
	updated, err := s.repo.FindByID(id, tenantID)
	if err != nil {
		slog.Warn("transitionTo: reload after status update failed, returning pre-update snapshot",
			"article_id", id, "target_status", target, "error", err)
		return article, nil // best-effort: return pre-update snapshot
	}
	// Status changes affect search visibility (e.g. draft→published makes the
	// article publicly searchable). Re-index with full preloaded metadata.
	s.reindexByID(id, tenantID)
	s.invalidateArticle(tenantID, id)
	return updated, nil
}

// Publish flips an article to published status, recording the publish time if
// it has none. Triggers the entry.publish webhook event. Default-tenant
// variant for internal/scheduler use; prefer PublishAs for request paths.
func (s *ArticleService) Publish(id uint) (*models.Article, error) {
	return s.publish(id, nil, models.DefaultTenantID)
}

// PublishAs publishes an article within the tenant and records the
// authenticated actor.
func (s *ArticleService) PublishAs(id, tenantID, userID uint) (*models.Article, error) {
	return s.publish(id, &userID, tenantID)
}

func (s *ArticleService) publish(id uint, userID *uint, tenantID uint) (*models.Article, error) {
	// Only set PublishedAt if the article doesn't already have one. We need
	// to inspect the current article to decide, so load it first.
	current, err := s.repo.FindByID(id, tenantID)
	if err != nil {
		return nil, err
	}
	var publishedAt *time.Time
	if current.PublishedAt == nil {
		now := time.Now()
		publishedAt = &now
	}
	updated, err := s.transitionTo(id, models.StatusPublished, publishedAt, nil, tenantID, userID)
	if err != nil {
		return nil, err
	}
	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventEntryPublish, updated, tenantID)
	}
	return updated, nil
}

// Unpublish reverts a published/scheduled article back to draft. Triggers the
// entry.unpublish webhook event.
func (s *ArticleService) Unpublish(id uint) (*models.Article, error) {
	return s.unpublish(id, nil, models.DefaultTenantID)
}

// UnpublishAs unpublishes an article within the tenant and records the
// authenticated actor.
func (s *ArticleService) UnpublishAs(id, tenantID, userID uint) (*models.Article, error) {
	return s.unpublish(id, &userID, tenantID)
}

func (s *ArticleService) unpublish(id uint, userID *uint, tenantID uint) (*models.Article, error) {
	updated, err := s.transitionTo(id, models.StatusDraft, nil, nil, tenantID, userID)
	if err != nil {
		return nil, err
	}
	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventEntryUnpublish, updated, tenantID)
	}
	return updated, nil
}

// SubmitForReview moves a draft into the pending (review) queue.
func (s *ArticleService) SubmitForReview(id uint) (*models.Article, error) {
	return s.submitForReview(id, nil, models.DefaultTenantID)
}

// SubmitForReviewAs records who submitted the article for review.
func (s *ArticleService) SubmitForReviewAs(id, tenantID, userID uint) (*models.Article, error) {
	return s.submitForReview(id, &userID, tenantID)
}

func (s *ArticleService) submitForReview(id uint, userID *uint, tenantID uint) (*models.Article, error) {
	updated, err := s.transitionTo(id, models.StatusPending, nil, nil, tenantID, userID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Approve marks a pending article as published, recording the publish time if
// it has none. Triggers the entry.publish webhook event. Default-tenant
// variant; prefer ApproveAs for request paths.
func (s *ArticleService) Approve(id uint) (*models.Article, error) {
	return s.Publish(id)
}

// ApproveAs approves an article within the tenant and records the reviewer.
func (s *ArticleService) ApproveAs(id, tenantID, userID uint) (*models.Article, error) {
	return s.PublishAs(id, tenantID, userID)
}

// Schedule marks an article for automatic publication at the given time. The
// article stays non-public (status=scheduled) until the PublishScheduler flips
// it. Triggers the entry.schedule webhook event.
func (s *ArticleService) Schedule(id uint, at time.Time) (*models.Article, error) {
	return s.schedule(id, at, nil, models.DefaultTenantID)
}

// ScheduleAs schedules an article within the tenant and records the actor.
func (s *ArticleService) ScheduleAs(id uint, at time.Time, tenantID, userID uint) (*models.Article, error) {
	return s.schedule(id, at, &userID, tenantID)
}

func (s *ArticleService) schedule(id uint, at time.Time, userID *uint, tenantID uint) (*models.Article, error) {
	if at.IsZero() {
		return nil, errs.ErrBadRequest.WithMessage("scheduled_at is required")
	}
	updated, err := s.transitionTo(id, models.StatusScheduled, nil, &at, tenantID, userID)
	if err != nil {
		return nil, err
	}
	if s.webhook != nil {
		s.webhook.Dispatch(models.WebhookEventEntrySchedule, updated, tenantID)
	}
	return updated, nil
}

// Archive moves an article out of the active lifecycle.
func (s *ArticleService) Archive(id uint) (*models.Article, error) {
	return s.archive(id, nil, models.DefaultTenantID)
}

// ArchiveAs archives an article within the tenant and records the actor.
func (s *ArticleService) ArchiveAs(id, tenantID, userID uint) (*models.Article, error) {
	return s.archive(id, &userID, tenantID)
}

func (s *ArticleService) archive(id uint, userID *uint, tenantID uint) (*models.Article, error) {
	updated, err := s.transitionTo(id, models.StatusArchived, nil, nil, tenantID, userID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// PublishDueScheduled publishes all scheduled articles whose ScheduledAt is at
// or before now. Returns the number of articles flipped. Used by the
// PublishScheduler worker.
//
// Multi-tenancy: scans all tenants so scheduled articles in every tenant are
// published by the background scheduler sweep.
func (s *ArticleService) PublishDueScheduled(now time.Time) (int, error) {
	due, err := s.repo.ListScheduledDueAllTenants(now)
	if err != nil {
		return 0, err
	}
	if len(due) == 0 {
		return 0, nil
	}
	// Group article IDs by tenant so BulkPublish stays within-tenant.
	byTenant := make(map[uint][]uint)
	for _, a := range due {
		byTenant[a.TenantID] = append(byTenant[a.TenantID], a.ID)
	}
	var totalPublished int64
	for tenantID, ids := range byTenant {
		n, err := s.repo.BulkPublish(ids, now, tenantID)
		if err != nil {
			return int(totalPublished), err
		}
		if n > 0 {
			// Background high-risk write: the scheduler flips content public
			// without a request context, so the event carries no request/trace
			// IDs but is explicitly attributed to the background source.
			s.audit.Log(AuditEvent{
				TenantID:  &tenantID,
				Action:    "article.scheduled_publish",
				Entity:    "article",
				Details:   map[string]any{"ids": ids, "count": n, "mode": "scheduled"},
				Source:    SourceBackground,
				ActorType: ActorSystem,
				Outcome:   OutcomeSuccess,
			})
		}
		if s.webhook != nil && n > 0 {
			s.webhook.Dispatch(models.WebhookEventEntryPublish, map[string]interface{}{
				"ids":   ids,
				"count": n,
				"mode":  "scheduled",
			}, tenantID)
		}
		// Re-index auto-published articles so they become publicly searchable.
		for _, id := range ids {
			s.reindexByID(id, tenantID)
		}
		s.invalidateArticle(tenantID, ids...)
		totalPublished += n
	}
	return int(totalPublished), nil
}

// LikeArticle increments the like count for an article within the tenant.
func (s *ArticleService) LikeArticle(id, tenantID uint) error {
	return s.repo.IncrementLikeCount(id, tenantID)
}

// GenerateFeed produces an RSS 2.0 XML string of the latest published
// articles within the tenant.
func (s *ArticleService) GenerateFeed(tenantID uint) (string, error) {
	articles, err := s.repo.ListPublishedForFeed(defaultFeedSize, tenantID)
	if err != nil {
		return "", err
	}

	type rssItem struct {
		XMLName     xml.Name `xml:"item"`
		Title       string   `xml:"title"`
		Link        string   `xml:"link"`
		PubDate     string   `xml:"pubDate"`
		Description string   `xml:"description"`
		Author      string   `xml:"author,omitempty"`
		GUID        string   `xml:"guid"`
	}

	type rssChannel struct {
		XMLName     xml.Name  `xml:"channel"`
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Language    string    `xml:"language"`
		Items       []rssItem `xml:"item"`
	}

	type rssFeed struct {
		XMLName xml.Name   `xml:"rss"`
		Version string     `xml:"version,attr"`
		Channel rssChannel `xml:"channel"`
	}

	items := make([]rssItem, 0, len(articles))
	for _, a := range articles {
		articleURL := s.baseURL + "/articles/" + a.Slug
		item := rssItem{
			Title:       a.Title,
			Link:        articleURL,
			Description: a.Excerpt,
			GUID:        articleURL,
		}
		if a.PublishedAt != nil {
			item.PubDate = a.PublishedAt.Format(time.RFC1123Z)
		}
		if a.Author.DisplayName != "" {
			item.Author = fmt.Sprintf("%s (%s)", a.Author.Email, a.Author.DisplayName)
		}
		items = append(items, item)
	}

	feed := rssFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:       "ContentX Feed",
			Link:        s.baseURL,
			Description: "Latest articles from ContentX",
			Language:    "zh-cn",
			Items:       items,
		},
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	if err := xml.NewEncoder(&buf).Encode(&feed); err != nil {
		return "", fmt.Errorf("encode rss feed: %w", err)
	}
	return buf.String(), nil
}

// ---------- Custom Errors ----------
//
// ArticleService 原有的 ForbiddenError / BadRequestError 已移除，统一使用
// errs.ErrForbidden / errs.ErrBadRequest 的 WithMessage 副本。这使所有错误
// 走 handleServiceError 的 AppError 分支，前端可通过 err_code 字段获得
// 稳定的错误码（FORBIDDEN / BAD_REQUEST）。
