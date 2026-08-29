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

// newRAGTestToolset builds a toolset backed by an in-memory DB with two
// published articles and a warmed-up RAG index (using a dummy embedding
// provider). Returns the toolset and the two article IDs.
func newRAGTestToolset(t *testing.T) (*toolset, uint, uint) {
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
		Username:    "rag-author",
		Email:       "rag-author@test.com",
		Password:    "$2a$10$dummyhashnotforauth",
		DisplayName: "RAG Author",
		RoleID:      role.ID,
		Status:      models.UserStatusActive,
	}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}

	now := time.Now()
	art1 := models.Article{
		Title: "Go Programming Guide", Slug: "go-programming-guide",
		Content:  "Go is a statically typed, compiled programming language designed at Google. It is simple, efficient, and has excellent concurrency support with goroutines and channels.",
		AuthorID: author.ID, Status: models.StatusPublished, PublishedAt: &now,
		TenantID: models.DefaultTenantID,
	}
	art2 := models.Article{
		Title: "Rust Memory Safety", Slug: "rust-memory-safety",
		Content:  "Rust provides memory safety without a garbage collector using ownership, borrowing, and lifetime rules. This prevents null pointer dereferences and data races at compile time.",
		AuthorID: author.ID, Status: models.StatusPublished, PublishedAt: &now,
		TenantID: models.DefaultTenantID,
	}
	if err := db.Create(&art1).Error; err != nil {
		t.Fatalf("create art1: %v", err)
	}
	if err := db.Create(&art2).Error; err != nil {
		t.Fatalf("create art2: %v", err)
	}

	articleSvc := services.NewArticleService(db, "http://localhost:8080")
	articleSvc.SetSearchIndexer(services.NewBuiltinIndexer())

	// Build RAG service with a dummy embedding provider (128 dims).
	embedder := services.NewDummyEmbeddingProvider(128)
	vecStore := services.NewMemoryVectorStore()
	ragSvc := services.NewRAGService(db, embedder, vecStore, services.NewDummyLLM(), 500, 100, 5, 0.01)
	articleSvc.SetRAGIndexer(ragSvc)

	// Index both articles.
	if err := ragSvc.IndexArticle(context.Background(), &art1); err != nil {
		t.Fatalf("index art1: %v", err)
	}
	if err := ragSvc.IndexArticle(context.Background(), &art2); err != nil {
		t.Fatalf("index art2: %v", err)
	}

	deps := Deps{
		Article:       articleSvc,
		ContentType:   services.NewContentTypeService(db),
		RAG:           ragSvc,
		BaseURL:       "http://localhost:8080",
		IncludeDrafts: false,
	}
	return &toolset{deps: deps}, art1.ID, art2.ID
}

func TestRAGSearch_RequiresQuery(t *testing.T) {
	ts, _, _ := newRAGTestToolset(t)
	if _, _, err := ts.ragSearch(context.Background(), nil, ragSearchInput{Query: "  "}); err == nil {
		t.Error("expected error for empty query")
	}
}

func TestRAGSearch_ReturnsResults(t *testing.T) {
	ts, _, _ := newRAGTestToolset(t)
	_, out, err := ts.ragSearch(context.Background(), nil, ragSearchInput{Query: "Go programming language"})
	if err != nil {
		t.Fatalf("ragSearch: %v", err)
	}
	if out.Total == 0 {
		t.Fatal("expected at least one search result")
	}
	// Every hit must have a title, slug, and a positive score.
	for _, h := range out.Hits {
		if h.Title == "" {
			t.Error("expected non-empty title")
		}
		if h.Slug == "" {
			t.Error("expected non-empty slug")
		}
		if h.Score <= 0 {
			t.Errorf("expected positive score, got %f", h.Score)
		}
		if h.URL == "" || h.URL[:4] != "http" {
			t.Errorf("expected absolute URL, got %q", h.URL)
		}
	}
}

func TestRAGSearch_RespectsTopK(t *testing.T) {
	ts, _, _ := newRAGTestToolset(t)
	_, out, err := ts.ragSearch(context.Background(), nil, ragSearchInput{Query: "programming", TopK: 1})
	if err != nil {
		t.Fatalf("ragSearch: %v", err)
	}
	if len(out.Hits) > 1 {
		t.Errorf("expected at most 1 result with TopK=1, got %d", len(out.Hits))
	}
}

func TestRAGAsk_RequiresQuery(t *testing.T) {
	ts, _, _ := newRAGTestToolset(t)
	if _, _, err := ts.ragAsk(context.Background(), nil, ragAskInput{Query: ""}); err == nil {
		t.Error("expected error for empty query")
	}
}

func TestRAGAsk_ReturnsAnswerAndContext(t *testing.T) {
	ts, _, _ := newRAGTestToolset(t)
	_, out, err := ts.ragAsk(context.Background(), nil, ragAskInput{Query: "What is Go?"})
	if err != nil {
		t.Fatalf("ragAsk: %v", err)
	}
	if out.Query != "What is Go?" {
		t.Errorf("expected query echo, got %q", out.Query)
	}
	// The dummy LLM always produces a non-empty answer.
	if out.Answer == "" {
		t.Error("expected non-empty answer from dummy LLM")
	}
	// Context should contain at least one chunk.
	if len(out.Context) == 0 {
		t.Fatal("expected at least one context chunk")
	}
	for _, c := range out.Context {
		if c.Title == "" {
			t.Error("expected non-empty context title")
		}
		if c.Content == "" {
			t.Error("expected non-empty context content")
		}
	}
}

func TestRAGAsk_RespectsTopK(t *testing.T) {
	ts, _, _ := newRAGTestToolset(t)
	_, out, err := ts.ragAsk(context.Background(), nil, ragAskInput{Query: "memory safety", TopK: 1})
	if err != nil {
		t.Fatalf("ragAsk: %v", err)
	}
	if len(out.Context) > 1 {
		t.Errorf("expected at most 1 context chunk with TopK=1, got %d", len(out.Context))
	}
}
