package services

import (
	"context"
	"testing"

	"github.com/yamovo/contentx/internal/models"
)

// ─── DummyEmbeddingProvider Tests ────────────────────────────────────────────

func TestDummyEmbeddingProvider_Dimension(t *testing.T) {
	p := NewDummyEmbeddingProvider(384)
	if p.Dimension() != 384 {
		t.Fatalf("expected dimension 384, got %d", p.Dimension())
	}
	if p.Name() != "dummy" {
		t.Fatalf("expected name 'dummy', got '%s'", p.Name())
	}
}

func TestDummyEmbeddingProvider_Deterministic(t *testing.T) {
	p := NewDummyEmbeddingProvider(128)
	texts := []string{"hello world", "hello world", "different text"}
	vecs, err := p.Embed(context.Background(), texts)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(vecs) != 3 {
		t.Fatalf("expected 3 vectors, got %d", len(vecs))
	}
	// Same text → same vector.
	for i := range vecs[0] {
		if vecs[0][i] != vecs[1][i] {
			t.Fatal("expected identical vectors for identical text")
		}
	}
	// Different text → different vector.
	diff := false
	for i := range vecs[0] {
		if vecs[0][i] != vecs[2][i] {
			diff = true
			break
		}
	}
	if !diff {
		t.Fatal("expected different vectors for different text")
	}
}

func TestDummyEmbeddingProvider_UnitLength(t *testing.T) {
	p := NewDummyEmbeddingProvider(64)
	vecs, _ := p.Embed(context.Background(), []string{"test"})
	var norm float64
	for _, v := range vecs[0] {
		norm += float64(v) * float64(v)
	}
	if norm < 0.99 || norm > 1.01 {
		t.Fatalf("expected unit length, got norm %f", norm)
	}
}

// ─── MemoryVectorStore Tests ─────────────────────────────────────────────────

func TestMemoryVectorStore_UpsertAndSearch(t *testing.T) {
	store := NewMemoryVectorStore()
	ctx := context.Background()

	// Insert two vectors in tenant 1.
	entries := []VectorEntry{
		{TenantID: 1, DocType: "article", DocID: 1, ChunkIndex: 0, Content: "Go is great", Embedding: []float32{1, 0, 0}, Status: "published", Title: "Go"},
		{TenantID: 1, DocType: "article", DocID: 2, ChunkIndex: 0, Content: "Rust is fast", Embedding: []float32{0, 1, 0}, Status: "published", Title: "Rust"},
	}
	if err := store.Upsert(ctx, entries); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Search for something closest to entry 1.
	results, err := store.Search(ctx, []float32{0.9, 0.1, 0}, VectorSearchOpts{TenantID: 1, TopK: 2, Status: "published"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].DocID != 1 {
		t.Fatalf("expected top result DocID=1, got %d", results[0].DocID)
	}
}

func TestMemoryVectorStore_TenantIsolation(t *testing.T) {
	store := NewMemoryVectorStore()
	ctx := context.Background()

	// Same content in two tenants.
	store.Upsert(ctx, []VectorEntry{
		{TenantID: 1, DocType: "article", DocID: 10, Embedding: []float32{1, 0}, Status: "published", Title: "tenant 1"},
		{TenantID: 2, DocType: "article", DocID: 10, Embedding: []float32{1, 0}, Status: "published", Title: "tenant 2"},
	})

	// Tenant 1 search should only see its own.
	res, _ := store.Search(ctx, []float32{1, 0}, VectorSearchOpts{TenantID: 1, TopK: 10, Status: "published"})
	if len(res) != 1 {
		t.Fatalf("tenant 1: expected 1 result, got %d", len(res))
	}
	if res[0].Title != "tenant 1" {
		t.Fatalf("expected 'tenant 1', got '%s'", res[0].Title)
	}

	// Tenant 2 search should only see its own.
	res, _ = store.Search(ctx, []float32{1, 0}, VectorSearchOpts{TenantID: 2, TopK: 10, Status: "published"})
	if len(res) != 1 {
		t.Fatalf("tenant 2: expected 1 result, got %d", len(res))
	}
	if res[0].Title != "tenant 2" {
		t.Fatalf("expected 'tenant 2', got '%s'", res[0].Title)
	}
}

func TestMemoryVectorStore_Delete(t *testing.T) {
	store := NewMemoryVectorStore()
	ctx := context.Background()

	store.Upsert(ctx, []VectorEntry{
		{TenantID: 1, DocType: "article", DocID: 5, Embedding: []float32{1, 0}, Status: "published"},
	})
	store.Delete(ctx, "article", 5, 1)

	res, _ := store.Search(ctx, []float32{1, 0}, VectorSearchOpts{TenantID: 1, TopK: 10})
	if len(res) != 0 {
		t.Fatalf("expected 0 results after delete, got %d", len(res))
	}
}

func TestMemoryVectorStore_DeleteDoesNotCrossTenant(t *testing.T) {
	store := NewMemoryVectorStore()
	ctx := context.Background()

	store.Upsert(ctx, []VectorEntry{
		{TenantID: 1, DocType: "article", DocID: 7, Embedding: []float32{1, 0}, Status: "published"},
		{TenantID: 2, DocType: "article", DocID: 7, Embedding: []float32{1, 0}, Status: "published"},
	})

	// Delete from tenant 1 only.
	store.Delete(ctx, "article", 7, 1)

	// Tenant 1 should be empty.
	res, _ := store.Search(ctx, []float32{1, 0}, VectorSearchOpts{TenantID: 1, TopK: 10, Status: "published"})
	if len(res) != 0 {
		t.Fatalf("tenant 1: expected 0 results, got %d", len(res))
	}
	// Tenant 2 should still have 1.
	res, _ = store.Search(ctx, []float32{1, 0}, VectorSearchOpts{TenantID: 2, TopK: 10, Status: "published"})
	if len(res) != 1 {
		t.Fatalf("tenant 2: expected 1 result, got %d", len(res))
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		a, b   []float32
		expect float64
	}{
		{[]float32{1, 0, 0}, []float32{1, 0, 0}, 1.0},
		{[]float32{1, 0, 0}, []float32{0, 1, 0}, 0.0},
		{[]float32{1, 0, 0}, []float32{-1, 0, 0}, -1.0},
		{[]float32{}, []float32{}, 0.0},
		{[]float32{1, 0}, []float32{1, 0, 0}, 0.0}, // dimension mismatch
	}
	for _, tt := range tests {
		got := cosineSimilarity(tt.a, tt.b)
		if abs(got-tt.expect) > 0.001 {
			t.Errorf("cosineSimilarity(%v, %v) = %f, want %f", tt.a, tt.b, got, tt.expect)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// ─── RAGService Tests (with DB) ──────────────────────────────────────────────

func TestRAGService_IndexAndSearch(t *testing.T) {
	db := setupTestDB(t)

	embedder := NewDummyEmbeddingProvider(64)
	store := NewMemoryVectorStore()
	rag := NewRAGService(db, embedder, store, NewDummyLLM(), 500, 100, 5, 0.0)

	// Create a published article.
	article := &models.Article{
		TenantID: 1,
		Title:    "Getting Started with Go",
		Slug:     "getting-started-go",
		Content:  "Go is a statically typed, compiled programming language designed at Google. It is simple, efficient, and has excellent concurrency support.",
		Status:   models.StatusPublished,
		PostType: models.PostTypePost,
		Locale:   "en",
	}
	if err := db.Create(article).Error; err != nil {
		t.Fatalf("create article: %v", err)
	}

	// Index the article.
	if err := rag.IndexArticle(context.Background(), article); err != nil {
		t.Fatalf("index article: %v", err)
	}

	// Verify vectors are in the store.
	if count := store.Count(1); count == 0 {
		t.Fatal("expected non-zero vector count after indexing")
	}

	// Verify DB persistence.
	var dbCount int64
	db.Model(&models.DocumentEmbedding{}).Where("tenant_id = ?", 1).Count(&dbCount)
	if dbCount == 0 {
		t.Fatal("expected non-zero DB embedding count")
	}

	// Semantic search should return results.
	result, err := rag.Search(context.Background(), "Go programming language", 1, 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Hits) == 0 {
		t.Fatal("expected search hits")
	}
	if result.Hits[0].Title != "Getting Started with Go" {
		t.Fatalf("expected title 'Getting Started with Go', got '%s'", result.Hits[0].Title)
	}
}

func TestRAGService_TenantIsolation(t *testing.T) {
	db := setupTestDB(t)

	embedder := NewDummyEmbeddingProvider(64)
	store := NewMemoryVectorStore()
	rag := NewRAGService(db, embedder, store, NewDummyLLM(), 500, 100, 5, 0.0)

	// Create articles in two tenants with identical content.
	for _, tenantID := range []uint{1, 2} {
		a := &models.Article{
			TenantID: tenantID,
			Title:    "Shared Topic",
			Slug:     "shared-topic",
			Content:  "This is content about a shared topic that should be isolated per tenant.",
			Status:   models.StatusPublished,
			PostType: models.PostTypePost,
			Locale:   "en",
		}
		if err := db.Create(a).Error; err != nil {
			t.Fatalf("create article tenant %d: %v", tenantID, err)
		}
		if err := rag.IndexArticle(context.Background(), a); err != nil {
			t.Fatalf("index article tenant %d: %v", tenantID, err)
		}
	}

	// Tenant 1 search should only return tenant 1's article.
	res, err := rag.Search(context.Background(), "shared topic", 1, 5)
	if err != nil {
		t.Fatalf("search tenant 1: %v", err)
	}
	if len(res.Hits) != 1 {
		t.Fatalf("tenant 1: expected 1 hit, got %d", len(res.Hits))
	}

	// Tenant 2 search should only return tenant 2's article.
	res, err = rag.Search(context.Background(), "shared topic", 2, 5)
	if err != nil {
		t.Fatalf("search tenant 2: %v", err)
	}
	if len(res.Hits) != 1 {
		t.Fatalf("tenant 2: expected 1 hit, got %d", len(res.Hits))
	}
}

func TestRAGService_DeleteArticle(t *testing.T) {
	db := setupTestDB(t)

	embedder := NewDummyEmbeddingProvider(32)
	store := NewMemoryVectorStore()
	rag := NewRAGService(db, embedder, store, NewDummyLLM(), 500, 100, 5, 0.0)

	article := &models.Article{
		TenantID: 1,
		Title:    "To Delete",
		Slug:     "to-delete",
		Content:  "This article will be deleted from the RAG index.",
		Status:   models.StatusPublished,
		PostType: models.PostTypePost,
	}
	db.Create(article)
	rag.IndexArticle(context.Background(), article)

	// Delete.
	if err := rag.DeleteArticle(context.Background(), article.ID, 1); err != nil {
		t.Fatalf("delete article: %v", err)
	}

	// Vector store should be empty.
	if count := store.Count(1); count != 0 {
		t.Fatalf("expected 0 vectors after delete, got %d", count)
	}

	// DB should have no embeddings.
	var dbCount int64
	db.Model(&models.DocumentEmbedding{}).Where("doc_id = ? AND tenant_id = ?", article.ID, 1).Count(&dbCount)
	if dbCount != 0 {
		t.Fatalf("expected 0 DB embeddings, got %d", dbCount)
	}
}

func TestRAGService_ChunkText(t *testing.T) {
	rag := &RAGService{chunkSize: 100, chunkOverlap: 20}

	// Short text → single chunk.
	chunks := rag.chunkText("Hello world")
	if len(chunks) != 1 {
		t.Fatalf("short text: expected 1 chunk, got %d", len(chunks))
	}

	// Long text → multiple chunks.
	long := ""
	for i := 0; i < 20; i++ {
		long += "This is paragraph " + string(rune('A'+i)) + ". It contains enough text to span multiple chunks. "
	}
	chunks = rag.chunkText(long)
	if len(chunks) < 2 {
		t.Fatalf("long text: expected >= 2 chunks, got %d", len(chunks))
	}

	// Empty text → no chunks.
	chunks = rag.chunkText("")
	if len(chunks) != 0 {
		t.Fatalf("empty text: expected 0 chunks, got %d", len(chunks))
	}

	// HTML tags should be stripped.
	chunks = rag.chunkText("<p>Hello <b>world</b></p>")
	if len(chunks) != 1 {
		t.Fatalf("html text: expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != "Hello world" {
		t.Fatalf("expected 'Hello world', got '%s'", chunks[0])
	}
}

func TestRAGService_Ask(t *testing.T) {
	db := setupTestDB(t)

	embedder := NewDummyEmbeddingProvider(64)
	store := NewMemoryVectorStore()
	rag := NewRAGService(db, embedder, store, NewDummyLLM(), 500, 100, 5, 0.0)

	article := &models.Article{
		TenantID: 1,
		Title:    "Go Concurrency",
		Slug:     "go-concurrency",
		Content:  "Go provides goroutines and channels for concurrent programming.",
		Status:   models.StatusPublished,
		PostType: models.PostTypePost,
	}
	db.Create(article)
	rag.IndexArticle(context.Background(), article)

	answer, err := rag.Ask(context.Background(), "How does Go handle concurrency?", 1, 5)
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if len(answer.Context) == 0 {
		t.Fatal("expected non-empty context")
	}
	if answer.Answer == "" {
		t.Fatal("expected non-empty answer from dummy LLM")
	}
}
