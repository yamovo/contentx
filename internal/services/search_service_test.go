package services

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/models"
)

// ---------- MockSearchIndexer ----------
//
// 记录所有调用以便测试断言 Index/Delete/Search/ReindexAll 是否被触发。

type MockSearchIndexer struct {
	mu sync.Mutex

	IndexedDocs   []SearchDocument
	DeletedIDs    []uint
	DeletedTypes  []string
	SearchCalls   []SearchQuery
	Reindexed     []models.Article
	SearchResult  *SearchResult
	SearchErr     error
	IndexErr      error
}

func (m *MockSearchIndexer) Index(_ context.Context, doc SearchDocument) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.IndexedDocs = append(m.IndexedDocs, doc)
	return m.IndexErr
}

func (m *MockSearchIndexer) Delete(_ context.Context, id uint, docType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeletedIDs = append(m.DeletedIDs, id)
	m.DeletedTypes = append(m.DeletedTypes, docType)
	return nil
}

func (m *MockSearchIndexer) Search(_ context.Context, q SearchQuery) (*SearchResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SearchCalls = append(m.SearchCalls, q)
	if m.SearchResult == nil {
		return &SearchResult{Hits: []SearchHit{}}, m.SearchErr
	}
	return m.SearchResult, m.SearchErr
}

func (m *MockSearchIndexer) ReindexAll(_ context.Context, articles []models.Article) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Reindexed = articles
	return nil
}

func (m *MockSearchIndexer) Ping(_ context.Context) error { return nil }
func (m *MockSearchIndexer) Name() string                 { return "mock" }

func (m *MockSearchIndexer) IndexCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.IndexedDocs)
}

func (m *MockSearchIndexer) DeleteCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.DeletedIDs)
}

// ---------- BuiltinIndexer unit tests ----------

func TestBuiltinIndexer_IndexAndSearch(t *testing.T) {
	idx := NewBuiltinIndexer()
	ctx := context.Background()

	docs := []SearchDocument{
		{ID: 1, Type: "article", Title: "Go programming language", Content: "Go is a statically typed, compiled language designed at Google.", Status: "published", Slug: "go-intro"},
		{ID: 2, Type: "article", Title: "Rust vs Go", Content: "Rust offers memory safety without garbage collection, unlike Go.", Status: "published", Slug: "rust-vs-go"},
		{ID: 3, Type: "article", Title: "Python tutorial", Content: "Python is a popular dynamic language for data science.", Status: "draft", Slug: "python-tutorial"},
	}
	for _, d := range docs {
		if err := idx.Index(ctx, d); err != nil {
			t.Fatalf("Index(%d): %v", d.ID, err)
		}
	}

	// Search for "go" — should match docs 1 and 2 (both have "go" in title/body).
	res, err := idx.Search(ctx, SearchQuery{Query: "go", Status: "published"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 2 {
		t.Fatalf("expected 2 hits for 'go' in published, got %d", res.Total)
	}
	// Doc 1 has "go" in title (boosted) and body; doc 2 has "go" in title and body too.
	// Title boost should rank doc with more title matches higher.
	if res.Hits[0].ID != 1 && res.Hits[0].ID != 2 {
		t.Fatalf("top hit should be doc 1 or 2, got %d", res.Hits[0].ID)
	}
}

func TestBuiltinIndexer_SearchFiltersByStatus(t *testing.T) {
	idx := NewBuiltinIndexer()
	ctx := context.Background()

	_ = idx.Index(ctx, SearchDocument{ID: 1, Type: "article", Title: "Published post", Content: "content", Status: "published"})
	_ = idx.Index(ctx, SearchDocument{ID: 2, Type: "article", Title: "Draft post", Content: "content", Status: "draft"})

	// Public search: status=published → only doc 1.
	res, _ := idx.Search(ctx, SearchQuery{Query: "post", Status: "published"})
	if res.Total != 1 {
		t.Fatalf("published search: expected 1 hit, got %d", res.Total)
	}
	if res.Hits[0].ID != 1 {
		t.Fatalf("expected doc 1, got %d", res.Hits[0].ID)
	}

	// Admin search: no status filter → both docs.
	res, _ = idx.Search(ctx, SearchQuery{Query: "post"})
	if res.Total != 2 {
		t.Fatalf("unfiltered search: expected 2 hits, got %d", res.Total)
	}
}

func TestBuiltinIndexer_SearchFiltersByLocale(t *testing.T) {
	idx := NewBuiltinIndexer()
	ctx := context.Background()

	_ = idx.Index(ctx, SearchDocument{ID: 1, Type: "article", Title: "Hello world", Content: "greeting", Status: "published", Locale: "en"})
	_ = idx.Index(ctx, SearchDocument{ID: 2, Type: "article", Title: "Hello world 中文", Content: "问候", Status: "published", Locale: "zh"})

	res, _ := idx.Search(ctx, SearchQuery{Query: "hello", Status: "published", Locale: "zh"})
	if res.Total != 1 {
		t.Fatalf("locale=zh: expected 1 hit, got %d", res.Total)
	}
	if res.Hits[0].ID != 2 {
		t.Fatalf("expected doc 2 (zh), got %d", res.Hits[0].ID)
	}
}

func TestBuiltinIndexer_SearchFiltersByType(t *testing.T) {
	idx := NewBuiltinIndexer()
	ctx := context.Background()

	_ = idx.Index(ctx, SearchDocument{ID: 1, Type: "article", Title: "Go guide", Content: "learn go", Status: "published"})
	_ = idx.Index(ctx, SearchDocument{ID: 2, Type: "page", Title: "About Go", Content: "about page", Status: "published"})

	res, _ := idx.Search(ctx, SearchQuery{Query: "go", Status: "published", Type: "page"})
	if res.Total != 1 {
		t.Fatalf("type=page: expected 1 hit, got %d", res.Total)
	}
	if res.Hits[0].ID != 2 {
		t.Fatalf("expected doc 2 (page), got %d", res.Hits[0].ID)
	}
}

func TestBuiltinIndexer_Delete(t *testing.T) {
	idx := NewBuiltinIndexer()
	ctx := context.Background()

	_ = idx.Index(ctx, SearchDocument{ID: 1, Type: "article", Title: "Temp", Content: "temporary", Status: "published"})
	res, _ := idx.Search(ctx, SearchQuery{Query: "temp", Status: "published"})
	if res.Total != 1 {
		t.Fatalf("before delete: expected 1 hit, got %d", res.Total)
	}

	_ = idx.Delete(ctx, 1, "article")
	res, _ = idx.Search(ctx, SearchQuery{Query: "temp", Status: "published"})
	if res.Total != 0 {
		t.Fatalf("after delete: expected 0 hits, got %d", res.Total)
	}
}

func TestBuiltinIndexer_ReindexAll(t *testing.T) {
	idx := NewBuiltinIndexer()
	ctx := context.Background()

	// Seed with an initial doc.
	_ = idx.Index(ctx, SearchDocument{ID: 1, Type: "article", Title: "old", Content: "old", Status: "published"})

	// ReindexAll should wipe and replace.
	articles := []models.Article{
		{BaseModel: models.BaseModel{ID: 10}, Title: "new alpha", Content: "alpha content", Status: models.StatusPublished, Slug: "new-alpha"},
		{BaseModel: models.BaseModel{ID: 11}, Title: "new beta", Content: "beta content", Status: models.StatusPublished, Slug: "new-beta"},
	}
	if err := idx.ReindexAll(ctx, articles); err != nil {
		t.Fatalf("ReindexAll: %v", err)
	}

	// Old doc should be gone.
	res, _ := idx.Search(ctx, SearchQuery{Query: "old", Status: "published"})
	if res.Total != 0 {
		t.Fatalf("old doc should be wiped, got %d hits", res.Total)
	}

	// New docs should be present.
	res, _ = idx.Search(ctx, SearchQuery{Query: "alpha", Status: "published"})
	if res.Total != 1 {
		t.Fatalf("new doc alpha: expected 1 hit, got %d", res.Total)
	}
}

func TestBuiltinIndexer_Pagination(t *testing.T) {
	idx := NewBuiltinIndexer()
	ctx := context.Background()

	// Index 5 docs that all match the query "golang".
	for i := uint(1); i <= 5; i++ {
		_ = idx.Index(ctx, SearchDocument{
			ID:      i,
			Type:    "article",
			Title:   "golang tutorial",
			Content: "learn golang",
			Status:  "published",
			Slug:    "golang-" + itoa(i),
		})
	}

	// Page 1, size 2 → 2 hits, total 5, totalPages 3.
	res, _ := idx.Search(ctx, SearchQuery{Query: "golang", Status: "published", Page: 1, PageSize: 2})
	if res.Total != 5 {
		t.Fatalf("total: expected 5, got %d", res.Total)
	}
	if len(res.Hits) != 2 {
		t.Fatalf("page 1 hits: expected 2, got %d", len(res.Hits))
	}
	if res.TotalPages != 3 {
		t.Fatalf("totalPages: expected 3, got %d", res.TotalPages)
	}

	// Page 3, size 2 → 1 hit.
	res, _ = idx.Search(ctx, SearchQuery{Query: "golang", Status: "published", Page: 3, PageSize: 2})
	if len(res.Hits) != 1 {
		t.Fatalf("page 3 hits: expected 1, got %d", len(res.Hits))
	}
}

func TestBuiltinIndexer_EmptyQuery(t *testing.T) {
	idx := NewBuiltinIndexer()
	ctx := context.Background()
	_ = idx.Index(ctx, SearchDocument{ID: 1, Type: "article", Title: "test", Content: "test", Status: "published"})

	res, err := idx.Search(ctx, SearchQuery{Query: "", Status: "published"})
	if err != nil {
		t.Fatalf("empty query: %v", err)
	}
	if res.Total != 0 {
		t.Fatalf("empty query should return 0 hits, got %d", res.Total)
	}
}

func TestBuiltinIndexer_NoMatch(t *testing.T) {
	idx := NewBuiltinIndexer()
	ctx := context.Background()
	_ = idx.Index(ctx, SearchDocument{ID: 1, Type: "article", Title: "hello", Content: "world", Status: "published"})

	res, _ := idx.Search(ctx, SearchQuery{Query: "nonexistent", Status: "published"})
	if res.Total != 0 {
		t.Fatalf("no match: expected 0 hits, got %d", res.Total)
	}
}

func TestBuiltinIndexer_ReplacesOnReindex(t *testing.T) {
	idx := NewBuiltinIndexer()
	ctx := context.Background()

	// Index doc 1 with title "old title".
	_ = idx.Index(ctx, SearchDocument{ID: 1, Type: "article", Title: "old title", Content: "content", Status: "published"})
	// Re-index doc 1 with title "new title".
	_ = idx.Index(ctx, SearchDocument{ID: 1, Type: "article", Title: "new title", Content: "content", Status: "published"})

	// "old" should no longer match; "new" should.
	res, _ := idx.Search(ctx, SearchQuery{Query: "old", Status: "published"})
	if res.Total != 0 {
		t.Fatalf("old title should be replaced, got %d hits", res.Total)
	}
	res, _ = idx.Search(ctx, SearchQuery{Query: "new", Status: "published"})
	if res.Total != 1 {
		t.Fatalf("new title: expected 1 hit, got %d", res.Total)
	}
}

// ---------- CJK tokenization tests ----------

func TestBuiltinIndexer_ChineseSearch(t *testing.T) {
	idx := NewBuiltinIndexer()
	ctx := context.Background()

	docs := []SearchDocument{
		{ID: 1, Type: "article", Title: "Go 语言编程指南", Content: "本文介绍如何使用 Go 语言进行后端开发，包括并发编程和网络服务。", Status: "published", Locale: "zh", Slug: "go-lang-guide"},
		{ID: 2, Type: "article", Title: "Python 数据分析", Content: "使用 Python 进行数据分析的基础教程。", Status: "published", Locale: "zh", Slug: "python-data"},
		{ID: 3, Type: "article", Title: "Rust 内存安全", Content: "Rust 语言的内存安全特性详解。", Status: "published", Locale: "zh", Slug: "rust-memory"},
	}
	for _, d := range docs {
		_ = idx.Index(ctx, d)
	}

	// Search for "语言" (language) — should match docs 1 and 3 (both contain "语言").
	res, _ := idx.Search(ctx, SearchQuery{Query: "语言", Status: "published"})
	if res.Total < 2 {
		t.Fatalf("search '语言': expected >= 2 hits, got %d", res.Total)
	}

	// Search for "Go" — should match doc 1.
	res, _ = idx.Search(ctx, SearchQuery{Query: "Go", Status: "published"})
	if res.Total != 1 {
		t.Fatalf("search 'Go': expected 1 hit, got %d", res.Total)
	}
	if res.Hits[0].ID != 1 {
		t.Fatalf("expected doc 1, got %d", res.Hits[0].ID)
	}
}

func TestBuiltinIndexer_ChineseHighlight(t *testing.T) {
	idx := NewBuiltinIndexer()
	ctx := context.Background()

	content := "本文介绍 Go 语言的并发编程模型，包括 goroutine 和 channel。"
	_ = idx.Index(ctx, SearchDocument{ID: 1, Type: "article", Title: "Go 语言", Content: content, Status: "published"})

	res, _ := idx.Search(ctx, SearchQuery{Query: "语言", Status: "published"})
	if res.Total != 1 {
		t.Fatalf("expected 1 hit, got %d", res.Total)
	}
	if !strings.Contains(res.Hits[0].Highlight, "<mark>") {
		t.Fatalf("highlight should contain <mark>, got: %s", res.Hits[0].Highlight)
	}
	if !strings.Contains(res.Hits[0].Highlight, "语言") {
		t.Fatalf("highlight should contain '语言', got: %s", res.Hits[0].Highlight)
	}
}

// ---------- Tokenization unit tests ----------

func TestTokenize_Latin(t *testing.T) {
	tokens := tokenize("Hello, World! Go programming.")
	// Tokens >= 2 chars survive; single-char tokens are dropped.
	expected := map[string]bool{"hello": true, "world": true, "go": true, "programming": true}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for _, tok := range tokens {
		if !expected[tok] {
			t.Fatalf("unexpected token: %s", tok)
		}
	}
}

func TestTokenize_CJKBigram(t *testing.T) {
	// "语言编程" → bigrams: "语言", "言编", "编程"
	tokens := tokenize("语言编程")
	expected := map[string]bool{"语言": true, "言编": true, "编程": true}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d bigrams, got %d: %v", len(expected), len(tokens), tokens)
	}
	for _, tok := range tokens {
		if !expected[tok] {
			t.Fatalf("unexpected bigram: %s", tok)
		}
	}
}

func TestTokenize_MixedScript(t *testing.T) {
	// "Go 语言" → "go" (latin, but dropped: single char < 2... wait, "go" is 2 chars)
	// Actually "Go" is 2 chars so it survives. Then "语言" is a bigram.
	tokens := tokenize("Go 语言")
	expected := map[string]bool{"go": true, "语言": true}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for _, tok := range tokens {
		if !expected[tok] {
			t.Fatalf("unexpected token: %s", tok)
		}
	}
}

func TestTokenize_SingleCJKCharDropped(t *testing.T) {
	// A single CJK char produces no bigram (too noisy).
	tokens := tokenize("语")
	if len(tokens) != 0 {
		t.Fatalf("single CJK char should produce 0 tokens, got %d: %v", len(tokens), tokens)
	}
}

func TestTokenize_Empty(t *testing.T) {
	if tokens := tokenize(""); len(tokens) != 0 {
		t.Fatalf("empty string: expected 0 tokens, got %d", len(tokens))
	}
}

// ---------- Highlight unit tests ----------

func TestBuildHighlight_Match(t *testing.T) {
	hl := buildHighlight("This is a test of the highlighting function.", []string{"test"})
	if !strings.Contains(hl, "<mark>test</mark>") {
		t.Fatalf("expected <mark>test</mark>, got: %s", hl)
	}
}

func TestBuildHighlight_NoMatch(t *testing.T) {
	hl := buildHighlight("This is a test.", []string{"nonexistent"})
	// No match → leading excerpt, no <mark>.
	if strings.Contains(hl, "<mark>") {
		t.Fatalf("should not contain <mark> on no match, got: %s", hl)
	}
}

func TestBuildHighlight_MultipleTokens(t *testing.T) {
	hl := buildHighlight("Go is a programming language", []string{"go", "programming"})
	if !strings.Contains(hl, "<mark>Go</mark>") {
		t.Fatalf("expected <mark>Go</mark>, got: %s", hl)
	}
	if !strings.Contains(hl, "<mark>programming</mark>") {
		t.Fatalf("expected <mark>programming</mark>, got: %s", hl)
	}
}

func TestBuildHighlight_CJK(t *testing.T) {
	hl := buildHighlight("本文介绍 Go 语言的并发编程", []string{"语言"})
	if !strings.Contains(hl, "<mark>语言</mark>") {
		t.Fatalf("expected <mark>语言</mark>, got: %s", hl)
	}
}

// ---------- ArticleToSearchDoc tests ----------

func TestArticleToSearchDoc(t *testing.T) {
	now := time.Now()
	catID := uint(5)
	a := &models.Article{
		BaseModel:   models.BaseModel{ID: 42, UpdatedAt: now},
		Title:       "Test Article",
		Content:     "Body content here",
		Excerpt:     "Short excerpt",
		Slug:        "test-article",
		Status:      models.StatusPublished,
		PostType:    models.PostTypePost,
		Locale:      "en",
		AuthorID:    7,
		CategoryID:  &catID,
		PublishedAt: &now,
		Author: models.User{
			BaseModel:     models.BaseModel{ID: 7},
			Username:      "alice",
			DisplayName:   "Alice",
		},
		Category: &models.Category{BaseModel: models.BaseModel{ID: 5}, Name: "Tech"},
		Tags: []models.Tag{
			{BaseModel: models.BaseModel{ID: 1}, Slug: "go"},
			{BaseModel: models.BaseModel{ID: 2}, Slug: "testing"},
		},
	}

	doc := ArticleToSearchDoc(a)
	if doc.ID != 42 {
		t.Fatalf("ID: expected 42, got %d", doc.ID)
	}
	if doc.Type != "article" {
		t.Fatalf("Type: expected 'article', got '%s'", doc.Type)
	}
	if doc.Title != "Test Article" {
		t.Fatalf("Title: expected 'Test Article', got '%s'", doc.Title)
	}
	if doc.AuthorName != "Alice" {
		t.Fatalf("AuthorName: expected 'Alice', got '%s'", doc.AuthorName)
	}
	if doc.CategoryName != "Tech" {
		t.Fatalf("CategoryName: expected 'Tech', got '%s'", doc.CategoryName)
	}
	if len(doc.TagSlugs) != 2 || doc.TagSlugs[0] != "go" {
		t.Fatalf("TagSlugs: expected [go testing], got %v", doc.TagSlugs)
	}
}

func TestArticleToSearchDoc_PageType(t *testing.T) {
	a := &models.Article{
		BaseModel: models.BaseModel{ID: 1},
		Title:     "About",
		PostType:  models.PostTypePage,
		Status:    models.StatusPublished,
	}
	doc := ArticleToSearchDoc(a)
	if doc.Type != "page" {
		t.Fatalf("PostType=page: expected doc.Type='page', got '%s'", doc.Type)
	}
}

func TestArticleToSearchDoc_AuthorNameFallback(t *testing.T) {
	a := &models.Article{
		BaseModel: models.BaseModel{ID: 1},
		Title:     "Test",
		Author:    models.User{BaseModel: models.BaseModel{ID: 2}, Username: "bob"}, // no DisplayName
	}
	doc := ArticleToSearchDoc(a)
	if doc.AuthorName != "bob" {
		t.Fatalf("AuthorName fallback: expected 'bob', got '%s'", doc.AuthorName)
	}
}

// ---------- NoopIndexer tests ----------

func TestNoopIndexer(t *testing.T) {
	idx := NoopIndexer()
	ctx := context.Background()

	if idx.Name() != "noop" {
		t.Fatalf("Name: expected 'noop', got '%s'", idx.Name())
	}
	if err := idx.Index(ctx, SearchDocument{}); err != nil {
		t.Fatalf("Index: %v", err)
	}
	if err := idx.Delete(ctx, 1, "article"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	res, err := idx.Search(ctx, SearchQuery{Query: "test"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 0 {
		t.Fatalf("Search total: expected 0, got %d", res.Total)
	}
	if err := idx.ReindexAll(ctx, nil); err != nil {
		t.Fatalf("ReindexAll: %v", err)
	}
	if err := idx.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

// ---------- ArticleService integration with SearchIndexer ----------

func TestArticleService_Create_TriggersIndex(t *testing.T) {
	repo := &MockArticleRepository{
		Articles: map[uint]*models.Article{},
	}

	svc := NewArticleServiceWithRepo(repo, "http://localhost:8080")
	mockIdx := &MockSearchIndexer{}
	svc.SetSearchIndexer(mockIdx)

	_, err := svc.Create(CreateArticleRequest{
		Title:   "Test Article",
		Content: "Body",
		Slug:    "test-article",
	}, 1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if mockIdx.IndexCount() != 1 {
		t.Fatalf("expected 1 Index call, got %d", mockIdx.IndexCount())
	}
}

func TestArticleService_Delete_TriggersUnindex(t *testing.T) {
	repo := &MockArticleRepository{
		Articles: map[uint]*models.Article{},
	}
	repo.Articles[1] = &models.Article{
		BaseModel: models.BaseModel{ID: 1},
		Title:     "Test",
		Slug:      "test",
		Status:    models.StatusDraft,
		PostType:  models.PostTypePost,
		AuthorID:  1,
	}

	svc := NewArticleServiceWithRepo(repo, "http://localhost:8080")
	mockIdx := &MockSearchIndexer{}
	svc.SetSearchIndexer(mockIdx)

	if err := svc.Delete(1, 1, true); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if mockIdx.DeleteCount() != 1 {
		t.Fatalf("expected 1 Delete call, got %d", mockIdx.DeleteCount())
	}
	if mockIdx.DeletedIDs[0] != 1 {
		t.Fatalf("expected delete id 1, got %d", mockIdx.DeletedIDs[0])
	}
}

func TestArticleService_Delete_PageType(t *testing.T) {
	repo := &MockArticleRepository{
		Articles: map[uint]*models.Article{},
	}
	repo.Articles[1] = &models.Article{
		BaseModel: models.BaseModel{ID: 1},
		Title:     "About",
		Slug:      "about",
		Status:    models.StatusDraft,
		PostType:  models.PostTypePage,
		AuthorID:  1,
	}

	svc := NewArticleServiceWithRepo(repo, "http://localhost:8080")
	mockIdx := &MockSearchIndexer{}
	svc.SetSearchIndexer(mockIdx)

	if err := svc.Delete(1, 1, true); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if mockIdx.DeleteCount() != 1 {
		t.Fatalf("expected 1 Delete call, got %d", mockIdx.DeleteCount())
	}
	if mockIdx.DeletedTypes[0] != "page" {
		t.Fatalf("expected delete type 'page', got '%s'", mockIdx.DeletedTypes[0])
	}
}

func TestArticleService_Publish_TriggersReindex(t *testing.T) {
	repo := &MockArticleRepository{
		Articles: map[uint]*models.Article{},
	}
	// FindByID returns this (no preloads). GetByID returns full article.
	repo.Articles[1] = &models.Article{
		BaseModel:    models.BaseModel{ID: 1},
		Title:        "Test",
		Slug:         "test",
		Status:       models.StatusDraft,
		PostType:     models.PostTypePost,
		AuthorID:     1,
		PublishedAt:  nil,
	}

	svc := NewArticleServiceWithRepo(repo, "http://localhost:8080")
	mockIdx := &MockSearchIndexer{}
	svc.SetSearchIndexer(mockIdx)

	_, err := svc.Publish(1)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	// Publish calls reindexByID → GetByID → indexArticle → Index.
	// That's 1 Index call (from reindexByID). The initial Publish path
	// does not call indexArticle directly, only reindexByID.
	if mockIdx.IndexCount() < 1 {
		t.Fatalf("expected >= 1 Index call after Publish, got %d", mockIdx.IndexCount())
	}
}

func TestArticleService_Search_Delegates(t *testing.T) {
	svc := NewArticleServiceWithRepo(&MockArticleRepository{}, "http://localhost:8080")
	mockIdx := &MockSearchIndexer{
		SearchResult: &SearchResult{
			Hits:  []SearchHit{{ID: 1, Title: "Hit 1"}},
			Total: 1,
		},
	}
	svc.SetSearchIndexer(mockIdx)

	res, err := svc.Search(context.Background(), SearchQuery{Query: "test"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 1 {
		t.Fatalf("expected total 1, got %d", res.Total)
	}
	if len(mockIdx.SearchCalls) != 1 {
		t.Fatalf("expected 1 Search call, got %d", len(mockIdx.SearchCalls))
	}
	if mockIdx.SearchCalls[0].Query != "test" {
		t.Fatalf("expected query 'test', got '%s'", mockIdx.SearchCalls[0].Query)
	}
}

func TestArticleService_Search_UsesNoopWhenUnset(t *testing.T) {
	svc := NewArticleServiceWithRepo(&MockArticleRepository{}, "http://localhost:8080")
	// Don't call SetSearchIndexer — should use NoopIndexer.
	res, err := svc.Search(context.Background(), SearchQuery{Query: "test"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Total != 0 {
		t.Fatalf("noop: expected total 0, got %d", res.Total)
	}
}

func TestArticleService_SetSearchIndexer_NilUsesNoop(t *testing.T) {
	svc := NewArticleServiceWithRepo(&MockArticleRepository{}, "http://localhost:8080")
	svc.SetSearchIndexer(nil) // should not panic
	res, _ := svc.Search(context.Background(), SearchQuery{Query: "test"})
	if res.Total != 0 {
		t.Fatalf("nil indexer: expected total 0, got %d", res.Total)
	}
}

// ---------- Concurrency test ----------

func TestBuiltinIndexer_ConcurrentAccess(t *testing.T) {
	idx := NewBuiltinIndexer()
	ctx := context.Background()

	// Concurrent writes + reads to verify the RWMutex prevents races.
	var wg sync.WaitGroup
	for i := uint(1); i <= 20; i++ {
		wg.Add(1)
		go func(id uint) {
			defer wg.Done()
			_ = idx.Index(ctx, SearchDocument{
				ID:      id,
				Type:    "article",
				Title:   "concurrent test",
				Content: "concurrent content",
				Status:  "published",
				Slug:    "concurrent-" + itoa(id),
			})
		}(i)
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = idx.Search(ctx, SearchQuery{Query: "concurrent", Status: "published"})
		}()
	}
	wg.Wait()

	res, _ := idx.Search(ctx, SearchQuery{Query: "concurrent", Status: "published"})
	if res.Total != 20 {
		t.Fatalf("after concurrent writes: expected 20 hits, got %d", res.Total)
	}
}
