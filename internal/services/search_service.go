package services

import (
	"context"
	"time"

	"github.com/yamovo/contentx/internal/models"
)

// SearchDocument is the canonical shape pushed into the search index. It is
// the storage-agnostic projection of an article (or content entry) so that
// every SearchIndexer implementation works with the same payload regardless
// of backend (in-memory, MeiliSearch, Elasticsearch).
type SearchDocument struct {
	ID           uint
	Type         string // "article" | "page" | "content_entry"
	Title        string
	Content      string // raw body, used for indexing; never returned to clients
	Excerpt      string
	Slug         string
	Status       string
	Locale       string
	AuthorID     uint
	AuthorName   string
	CategoryID   *uint
	CategoryName string
	TagSlugs     []string
	PublishedAt  *time.Time
	UpdatedAt    time.Time
}

// SearchQuery holds filter/pagination for a search request.
type SearchQuery struct {
	Query    string
	Type     string // empty = any
	Status   string // empty = any; public surface should pass "published"
	Locale   string // empty = any
	Page     int
	PageSize int
}

// SearchHit is one result row.
type SearchHit struct {
	ID          uint      `json:"id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Excerpt     string    `json:"excerpt"`
	Slug        string    `json:"slug"`
	Score       float64   `json:"score"`
	Highlight   string    `json:"highlight,omitempty"`
	Locale      string    `json:"locale,omitempty"`
	AuthorID    uint      `json:"author_id,omitempty"`
	AuthorName  string    `json:"author_name,omitempty"`
	CategoryID  *uint     `json:"category_id,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

// SearchResult bundles a search response.
type SearchResult struct {
	Hits       []SearchHit `json:"hits"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
	Took       string      `json:"took"` // human-readable duration
}

// SearchIndexer is the storage-agnostic full-text search backend.
// The builtin in-memory implementation is the default; an optional MeiliSearch
// driver can be wired in when SEARCH_ENGINE=meilisearch.
type SearchIndexer interface {
	Index(ctx context.Context, doc SearchDocument) error
	Delete(ctx context.Context, id uint, docType string) error
	Search(ctx context.Context, q SearchQuery) (*SearchResult, error)
	ReindexAll(ctx context.Context, articles []models.Article) error
	Ping(ctx context.Context) error
	Name() string
}

// noopIndexer is the default zero-cost indexer used when search is disabled
// or no indexer is injected. All operations are no-ops so services don't need
// nil checks on every write path.
type noopIndexer struct{}

func (noopIndexer) Index(context.Context, SearchDocument) error                  { return nil }
func (noopIndexer) Delete(context.Context, uint, string) error                   { return nil }
func (noopIndexer) Search(context.Context, SearchQuery) (*SearchResult, error) {
	return &SearchResult{Hits: []SearchHit{}}, nil
}
func (noopIndexer) ReindexAll(context.Context, []models.Article) error { return nil }
func (noopIndexer) Ping(context.Context) error                         { return nil }
func (noopIndexer) Name() string                                       { return "noop" }

// NoopIndexer returns a SearchIndexer whose methods are all no-ops. It is
// safe to share a single instance.
func NoopIndexer() SearchIndexer { return noopIndexer{} }

// ArticleToSearchDoc projects a models.Article (with associations preloaded)
// into a SearchDocument ready for indexing.
func ArticleToSearchDoc(a *models.Article) SearchDocument {
	doc := SearchDocument{
		ID:          a.ID,
		Type:        "article",
		Title:       a.Title,
		Content:     a.Content,
		Excerpt:     a.Excerpt,
		Slug:        a.Slug,
		Status:      string(a.Status),
		Locale:      a.Locale,
		AuthorID:    a.AuthorID,
		CategoryID:  a.CategoryID,
		PublishedAt: a.PublishedAt,
		UpdatedAt:   a.UpdatedAt,
		TagSlugs:    make([]string, 0, len(a.Tags)),
	}
	if a.PostType == models.PostTypePage {
		doc.Type = "page"
	}
	if a.Author.ID != 0 {
		doc.AuthorName = a.Author.DisplayName
		if doc.AuthorName == "" {
			doc.AuthorName = a.Author.Username
		}
	}
	if a.Category != nil {
		doc.CategoryName = a.Category.Name
	}
	for _, t := range a.Tags {
		doc.TagSlugs = append(doc.TagSlugs, t.Slug)
	}
	return doc
}
