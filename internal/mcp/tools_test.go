package mcp

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/services"
)

// newTestDeps builds MCP Deps backed by an in-memory SQLite database with one
// published and one draft article, plus a warmed-up builtin search index.
// Returns the Deps and the two article IDs.
func newTestDeps(t *testing.T, includeDrafts bool) (Deps, uint, uint) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.Seed(db)

	var role models.Role
	if err := db.Where("slug = ?", "admin").First(&role).Error; err != nil {
		t.Fatalf("admin role not found: %v", err)
	}
	author := models.User{
		Username:    "mcp-author",
		Email:       "mcp-author@test.com",
		Password:    "$2a$10$dummyhashnotforauth",
		DisplayName: "MCP Author",
		RoleID:      role.ID,
		Status:      models.UserStatusActive,
	}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}

	now := time.Now()
	pub := models.Article{
		Title: "Published One", Slug: "published-one",
		Content: "hello world alphaunique", Excerpt: "published excerpt",
		AuthorID: author.ID, Status: models.StatusPublished, PublishedAt: &now,
	}
	draft := models.Article{
		Title: "Draft Two", Slug: "draft-two",
		Content: "secret draft betaunique", Excerpt: "draft excerpt",
		AuthorID: author.ID, Status: models.StatusDraft,
	}
	if err := db.Create(&pub).Error; err != nil {
		t.Fatalf("create published: %v", err)
	}
	if err := db.Create(&draft).Error; err != nil {
		t.Fatalf("create draft: %v", err)
	}

	articleSvc := services.NewArticleService(db, "http://localhost:8080")
	articleSvc.SetSearchIndexer(services.NewBuiltinIndexer())
	if _, err := articleSvc.ReindexAll(context.Background()); err != nil {
		t.Fatalf("reindex: %v", err)
	}

	return Deps{
		Article:       articleSvc,
		ContentType:   services.NewContentTypeService(db),
		BaseURL:       "http://localhost:8080",
		IncludeDrafts: includeDrafts,
	}, pub.ID, draft.ID
}

// newTestToolset wraps newTestDeps for the direct-call handler tests.
func newTestToolset(t *testing.T, includeDrafts bool) (*toolset, uint, uint) {
	deps, pub, draft := newTestDeps(t, includeDrafts)
	return &toolset{deps: deps}, pub, draft
}

// slugSet collects the slugs from a list_articles result.
func slugSet(items []articleSummary) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, it := range items {
		set[it.Slug] = true
	}
	return set
}

func TestListArticles_PublishedOnlyByDefault(t *testing.T) {
	ts, _, _ := newTestToolset(t, false)
	_, out, err := ts.listArticles(context.Background(), nil, listArticlesInput{})
	if err != nil {
		t.Fatalf("listArticles: %v", err)
	}
	slugs := slugSet(out.Items)
	if !slugs["published-one"] {
		t.Error("expected published article in list")
	}
	if slugs["draft-two"] {
		t.Error("draft must not appear when IncludeDrafts is false")
	}
	// URLs should be absolute against the configured base URL.
	for _, it := range out.Items {
		if it.URL == "" || it.URL[:4] != "http" {
			t.Errorf("expected absolute URL, got %q", it.URL)
		}
		if it.Version < 1 {
			t.Errorf("expected optimistic-lock version in list output, got %d", it.Version)
		}
	}
}

func TestListArticles_IncludeDraftsShowsDraft(t *testing.T) {
	ts, _, _ := newTestToolset(t, true)
	_, out, err := ts.listArticles(context.Background(), nil, listArticlesInput{})
	if err != nil {
		t.Fatalf("listArticles: %v", err)
	}
	if !slugSet(out.Items)["draft-two"] {
		t.Error("draft should appear when IncludeDrafts is true")
	}
}

func TestGetArticle_PublishedReturnsContent(t *testing.T) {
	ts, pubID, _ := newTestToolset(t, false)
	_, out, err := ts.getArticle(context.Background(), nil, getArticleInput{ID: pubID})
	if err != nil {
		t.Fatalf("getArticle: %v", err)
	}
	if out.Content != "hello world alphaunique" {
		t.Errorf("content = %q, want the published body", out.Content)
	}
	if out.Author != "MCP Author" {
		t.Errorf("author = %q, want display name", out.Author)
	}
	if out.Version < 1 {
		t.Errorf("expected optimistic-lock version in detail output, got %d", out.Version)
	}
}

func TestGetArticle_DraftHiddenByDefault(t *testing.T) {
	ts, _, draftID := newTestToolset(t, false)
	if _, _, err := ts.getArticle(context.Background(), nil, getArticleInput{ID: draftID}); err == nil {
		t.Error("expected error fetching a draft when IncludeDrafts is false")
	}
}

func TestGetArticle_MissingID(t *testing.T) {
	ts, _, _ := newTestToolset(t, false)
	if _, _, err := ts.getArticle(context.Background(), nil, getArticleInput{ID: 0}); err == nil {
		t.Error("expected error when id is missing")
	}
}

func TestSearchContent_RequiresQuery(t *testing.T) {
	ts, _, _ := newTestToolset(t, false)
	if _, _, err := ts.searchContent(context.Background(), nil, searchInput{Query: "  "}); err == nil {
		t.Error("expected error for empty query")
	}
}

func TestSearchContent_FindsPublishedExcludesDraft(t *testing.T) {
	ts, _, _ := newTestToolset(t, false)

	// A term unique to the published article should be found.
	_, out, err := ts.searchContent(context.Background(), nil, searchInput{Query: "alphaunique"})
	if err != nil {
		t.Fatalf("searchContent: %v", err)
	}
	found := false
	for _, h := range out.Hits {
		if h.Slug == "published-one" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find the published article by its unique term")
	}

	// A term unique to the draft must not surface on the published-only surface.
	_, out2, err := ts.searchContent(context.Background(), nil, searchInput{Query: "betaunique"})
	if err != nil {
		t.Fatalf("searchContent(draft term): %v", err)
	}
	for _, h := range out2.Hits {
		if h.Slug == "draft-two" {
			t.Error("draft must not be searchable when IncludeDrafts is false")
		}
	}
}

func TestListContentTypes_OK(t *testing.T) {
	ts, _, _ := newTestToolset(t, false)
	_, out, err := ts.listContentTypes(context.Background(), nil, emptyInput{})
	if err != nil {
		t.Fatalf("listContentTypes: %v", err)
	}
	// No content types are seeded; the tool should return an empty (non-nil) list.
	if out.ContentTypes == nil {
		t.Error("expected non-nil content_types slice")
	}
}
