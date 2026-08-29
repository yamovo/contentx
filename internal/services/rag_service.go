package services

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/yamovo/contentx/internal/errs"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// ─── RAGIndexer interface (injected into ArticleService) ─────────────────────

// RAGIndexer is the minimal interface that ArticleService calls to keep the
// RAG vector index in sync. It mirrors the SearchIndexer injection pattern.
type RAGIndexer interface {
	IndexArticle(ctx context.Context, article *models.Article) error
	DeleteArticle(ctx context.Context, id uint, tenantID uint) error
}

// noopRAGIndexer is a no-op implementation used when RAG is disabled.
type noopRAGIndexer struct{}

func (noopRAGIndexer) IndexArticle(context.Context, *models.Article) error { return nil }
func (noopRAGIndexer) DeleteArticle(context.Context, uint, uint) error     { return nil }

// ─── LLM interface (optional, for answer synthesis) ─────────────────────────

// LLMProvider generates text completions. Used by RAGService.Ask to synthesise
// an answer from retrieved context. When no LLM is configured, Ask returns
// only the retrieved context chunks.
type LLMProvider interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	Name() string
	// External reports whether the provider calls an external (out-of-process)
	// API. The RAG outbound policy (AI_ALLOW_OUTBOUND=false) refuses answer
	// synthesis on external providers so content never leaves the host.
	External() bool
}

// ─── dummyLLM ────────────────────────────────────────────────────────────────

type dummyLLM struct{}

func NewDummyLLM() LLMProvider { return dummyLLM{} }
func (dummyLLM) Name() string  { return "dummy" }
func (dummyLLM) External() bool {
	return false
}
func (dummyLLM) Complete(_ context.Context, _, userPrompt string) (string, error) {
	return fmt.Sprintf("[dummy LLM] I would answer based on this question: %s", userPrompt), nil
}

// ─── Response types ──────────────────────────────────────────────────────────

// SemanticHit is a single semantic search result.
type SemanticHit struct {
	DocID      uint    `json:"doc_id"`
	DocType    string  `json:"doc_type"`
	ChunkIndex int     `json:"chunk_index"`
	Title      string  `json:"title"`
	Slug       string  `json:"slug"`
	Excerpt    string  `json:"excerpt"` // first ~200 chars of the chunk
	Score      float64 `json:"score"`   // cosine similarity 0..1
	Locale     string  `json:"locale"`
}

// SemanticSearchResult is the response for a semantic search query.
type SemanticSearchResult struct {
	Query string        `json:"query"`
	Hits  []SemanticHit `json:"hits"`
	Total int           `json:"total"`
	Took  time.Duration `json:"took_ms"`
}

// RAGContextChunk is a single retrieved context chunk for RAG.
type RAGContextChunk struct {
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// RAGAnswer is the response for a RAG query.
type RAGAnswer struct {
	Query   string            `json:"query"`
	Answer  string            `json:"answer"`  // LLM-synthesised answer (empty if no LLM)
	Context []RAGContextChunk `json:"context"` // retrieved chunks
	Took    time.Duration     `json:"took_ms"`
}

// ─── RAGService ──────────────────────────────────────────────────────────────

// RAGService manages the RAG pipeline: text chunking, embedding generation,
// vector indexing, semantic search, and optional LLM answer synthesis.
// It implements RAGIndexer so ArticleService can keep it in sync.
type RAGService struct {
	db            *gorm.DB
	embedder      EmbeddingProvider
	store         VectorStore
	llm           LLMProvider
	chunkSize     int
	chunkOverlap  int
	defaultTopK   int
	minScore      float64 // minimum cosine similarity to include in results
	allowOutbound bool    // when false, refuses calls to external embedding/LLM APIs
	mu            sync.Mutex
	indexing      bool // prevents concurrent ReindexAll
}

// ErrAIOutboundDisabled is returned by RAG operations that would call an
// external embedding/LLM API while the outbound policy is disabled
// (AI_ALLOW_OUTBOUND=false). Local (dummy) providers remain usable.
var ErrAIOutboundDisabled = errs.ErrForbidden.WithMessage("outbound AI API calls are disabled (AI_ALLOW_OUTBOUND=false)")

// NewRAGService creates a new RAG service. The embedder and store must be
// non-nil; llm may be nil (Ask will return context only).
func NewRAGService(db *gorm.DB, embedder EmbeddingProvider, store VectorStore, llm LLMProvider, chunkSize, chunkOverlap, topK int, minScore float64) *RAGService {
	if chunkSize <= 0 {
		chunkSize = 1000
	}
	if chunkOverlap < 0 || chunkOverlap >= chunkSize {
		chunkOverlap = 200
	}
	if topK <= 0 {
		topK = 5
	}
	return &RAGService{
		db:            db,
		embedder:      embedder,
		store:         store,
		llm:           llm,
		chunkSize:     chunkSize,
		chunkOverlap:  chunkOverlap,
		defaultTopK:   topK,
		minScore:      minScore,
		allowOutbound: true, // secure default is opt-in per deployment via SetAllowOutbound
	}
}

// SetAllowOutbound applies the deployment's outbound policy. When false, any
// operation that would call an external embedding/LLM API fails with
// ErrAIOutboundDisabled; local (dummy) providers keep working. Both the REST
// and MCP wiring call this so the boundary is enforced at the service layer
// instead of per transport.
func (s *RAGService) SetAllowOutbound(allow bool) { s.allowOutbound = allow }

// embed generates embeddings through the configured provider after enforcing
// the outbound policy. All internal embedding call sites funnel through here
// so indexing, search, and retrieval share one guard.
func (s *RAGService) embed(ctx context.Context, texts []string) ([][]float32, error) {
	if !s.allowOutbound && s.embedder.External() {
		return nil, ErrAIOutboundDisabled
	}
	return s.embedder.Embed(ctx, texts)
}

// Embedder returns the configured embedding provider (for health checks).
func (s *RAGService) Embedder() EmbeddingProvider { return s.embedder }

// Store returns the configured vector store (for health checks).
func (s *RAGService) Store() VectorStore { return s.store }

// LLM returns the configured LLM provider (for health checks). May be nil.
func (s *RAGService) LLM() LLMProvider { return s.llm }

// ─── Text chunking ───────────────────────────────────────────────────────────

var (
	htmlTagRe    = regexp.MustCompile(`<[^>]*>`)
	whitespaceRe = regexp.MustCompile(`[ \t]+`)
)

// stripHTML removes HTML tags and normalises whitespace.
func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	s = whitespaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// chunkText splits text into overlapping chunks of approximately chunkSize
// characters. It tries to break at paragraph boundaries first, then at
// sentence boundaries, to avoid splitting mid-word.
func (s *RAGService) chunkText(text string) []string {
	clean := stripHTML(text)
	if clean == "" {
		return nil
	}
	if len(clean) <= s.chunkSize {
		return []string{clean}
	}

	// Split by paragraphs (double newline).
	paragraphs := strings.Split(clean, "\n")
	var chunks []string
	var current strings.Builder
	currentLen := 0

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		// If adding this paragraph stays within budget, append it.
		if currentLen+len(para)+1 <= s.chunkSize {
			if currentLen > 0 {
				current.WriteString("\n")
				currentLen++
			}
			current.WriteString(para)
			currentLen += len(para)
		} else {
			// Flush current chunk if non-empty.
			if currentLen > 0 {
				chunks = append(chunks, current.String())
				// Start next chunk with overlap.
				overlap := s.extractOverlap(current.String(), s.chunkOverlap)
				current.Reset()
				current.WriteString(overlap)
				currentLen = len(overlap)
			}
			// If the paragraph itself exceeds chunkSize, split by sentences.
			if len(para) > s.chunkSize {
				sentences := splitSentences(para)
				for _, sent := range sentences {
					if currentLen+len(sent)+1 <= s.chunkSize {
						if currentLen > 0 {
							current.WriteString(" ")
							currentLen++
						}
						current.WriteString(sent)
						currentLen += len(sent)
					} else {
						if currentLen > 0 {
							chunks = append(chunks, current.String())
							overlap := s.extractOverlap(current.String(), s.chunkOverlap)
							current.Reset()
							current.WriteString(overlap)
							currentLen = len(overlap)
						}
						current.WriteString(sent)
						currentLen += len(sent)
					}
				}
			} else {
				current.WriteString(para)
				currentLen += len(para)
			}
		}
	}
	if currentLen > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

// extractOverlap returns the last `n` characters of `s`, adjusted to start
// at a word boundary.
func (s *RAGService) extractOverlap(text string, n int) string {
	if n <= 0 || len(text) <= n {
		return text
	}
	start := len(text) - n
	// Move start forward to the next space.
	for start < len(text) && text[start] != ' ' && text[start] != '\n' {
		start++
	}
	for start < len(text) && (text[start] == ' ' || text[start] == '\n') {
		start++
	}
	return text[start:]
}

// splitSentences splits text at sentence boundaries (. ! ? followed by space).
func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder
	for i, r := range text {
		current.WriteRune(r)
		if (r == '.' || r == '!' || r == '?') && i+1 < len(text) && (text[i+1] == ' ' || text[i+1] == '\n') {
			sentences = append(sentences, strings.TrimSpace(current.String()))
			current.Reset()
		}
	}
	if current.Len() > 0 {
		sentences = append(sentences, strings.TrimSpace(current.String()))
	}
	return sentences
}

// ─── Indexing (implements RAGIndexer) ────────────────────────────────────────

// IndexArticle chunks the article content, generates embeddings, and stores
// them in both the database (for persistence) and the vector store (for search).
// Non-published articles are removed from the index to ensure only published
// content is ever retrievable.
func (s *RAGService) IndexArticle(ctx context.Context, article *models.Article) error {
	// Only index published content. Unpublished/draft/trashed articles are
	// removed from the index to prevent information leakage.
	if article.Status != models.StatusPublished {
		return s.DeleteArticle(ctx, article.ID, article.TenantID)
	}

	// Combine title + content for embedding.
	text := article.Title + "\n\n" + article.Content
	chunks := s.chunkText(text)
	if len(chunks) == 0 {
		return s.DeleteArticle(ctx, article.ID, article.TenantID)
	}

	// Generate embeddings in a single batch.
	embeddings, err := s.embed(ctx, chunks)
	if err != nil {
		return fmt.Errorf("generate embeddings: %w", err)
	}
	if len(embeddings) != len(chunks) {
		return fmt.Errorf("embedding count mismatch: %d chunks, %d embeddings", len(chunks), len(embeddings))
	}

	// Delete existing embeddings for this article.
	if err := s.DeleteArticle(ctx, article.ID, article.TenantID); err != nil {
		return fmt.Errorf("delete old embeddings: %w", err)
	}

	// Prepare DB records and vector entries.
	docType := "article"
	if article.PostType == models.PostTypePage {
		docType = "page"
	}
	dbRecords := make([]models.DocumentEmbedding, len(chunks))
	vecEntries := make([]VectorEntry, len(chunks))
	for i, chunk := range chunks {
		dbRecords[i] = models.DocumentEmbedding{
			TenantID:   article.TenantID,
			DocType:    docType,
			DocID:      article.ID,
			ChunkIndex: i,
			Content:    chunk,
			Embedding:  models.Float32Slice(embeddings[i]),
			Model:      s.embedder.Name(),
			Locale:     article.Locale,
			Title:      article.Title,
			Slug:       article.Slug,
			Status:     string(article.Status),
		}
		vecEntries[i] = VectorEntry{
			TenantID:   article.TenantID,
			DocType:    docType,
			DocID:      article.ID,
			ChunkIndex: i,
			Content:    chunk,
			Embedding:  embeddings[i],
			Model:      s.embedder.Name(),
			Locale:     article.Locale,
			Title:      article.Title,
			Slug:       article.Slug,
			Status:     string(article.Status),
		}
	}

	// Persist to database.
	if err := s.db.CreateInBatches(dbRecords, 100).Error; err != nil {
		return fmt.Errorf("save embeddings to DB: %w", err)
	}

	// Update vector store.
	if err := s.store.Upsert(ctx, vecEntries); err != nil {
		return fmt.Errorf("upsert vector store: %w", err)
	}

	slog.Debug("rag: indexed article", "article_id", article.ID, "tenant_id", article.TenantID, "chunks", len(chunks))
	return nil
}

// DeleteArticle removes all embeddings for the given article from both the
// database and the vector store.
func (s *RAGService) DeleteArticle(_ context.Context, id uint, tenantID uint) error {
	docTypes := []string{"article", "page"}
	for _, dt := range docTypes {
		if err := s.db.Where("doc_type = ? AND doc_id = ? AND tenant_id = ?", dt, id, tenantID).
			Delete(&models.DocumentEmbedding{}).Error; err != nil {
			return fmt.Errorf("delete embeddings from DB: %w", err)
		}
		_ = s.store.Delete(context.Background(), dt, id, tenantID)
	}
	return nil
}

// ─── Search & Retrieve ───────────────────────────────────────────────────────

// Search performs a semantic search: embeds the query text and finds the most
// similar content chunks within the specified tenant.
func (s *RAGService) Search(ctx context.Context, query string, tenantID uint, topK int) (*SemanticSearchResult, error) {
	start := time.Now()
	if topK <= 0 {
		topK = s.defaultTopK
	}
	embeddings, err := s.embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("empty embedding for query")
	}

	results, err := s.store.Search(ctx, embeddings[0], VectorSearchOpts{
		TenantID: tenantID,
		TopK:     topK,
		Status:   string(models.StatusPublished),
		MinScore: s.minScore,
	})
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	hits := make([]SemanticHit, len(results))
	for i, r := range results {
		hits[i] = SemanticHit{
			DocID:      r.DocID,
			DocType:    r.DocType,
			ChunkIndex: r.ChunkIndex,
			Title:      r.Title,
			Slug:       r.Slug,
			Excerpt:    truncate(r.Content, 200),
			Score:      r.Score,
			Locale:     r.Locale,
		}
	}

	return &SemanticSearchResult{
		Query: query,
		Hits:  hits,
		Total: len(hits),
		Took:  time.Since(start),
	}, nil
}

// Retrieve returns the top-K context chunks for a RAG query. Unlike Search,
// it returns full chunk content (not just excerpts) for LLM context assembly.
func (s *RAGService) Retrieve(ctx context.Context, query string, tenantID uint, topK int) ([]RAGContextChunk, error) {
	if topK <= 0 {
		topK = s.defaultTopK
	}
	embeddings, err := s.embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("empty embedding for query")
	}

	results, err := s.store.Search(ctx, embeddings[0], VectorSearchOpts{
		TenantID: tenantID,
		TopK:     topK,
		Status:   string(models.StatusPublished),
		MinScore: s.minScore,
	})
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	chunks := make([]RAGContextChunk, len(results))
	for i, r := range results {
		chunks[i] = RAGContextChunk{
			Title:   r.Title,
			Content: r.Content,
			Score:   r.Score,
		}
	}
	return chunks, nil
}

// Ask performs a full RAG query: retrieves relevant context chunks and
// optionally synthesises an answer using the configured LLM. If no LLM is
// configured, only the retrieved context is returned.
func (s *RAGService) Ask(ctx context.Context, query string, tenantID uint, topK int) (*RAGAnswer, error) {
	// Refuse deterministically when answer synthesis would require an external
	// LLM while outbound calls are disabled — identical behavior for REST and
	// MCP callers instead of a transport-specific check.
	if s.llm != nil && s.llm.External() && !s.allowOutbound {
		return nil, ErrAIOutboundDisabled
	}
	start := time.Now()
	chunks, err := s.Retrieve(ctx, query, tenantID, topK)
	if err != nil {
		return nil, err
	}

	answer := ""
	if s.llm != nil && len(chunks) > 0 {
		contextParts := make([]string, len(chunks))
		for i, c := range chunks {
			contextParts[i] = fmt.Sprintf("[%d] %s\n%s", i+1, c.Title, c.Content)
		}
		systemPrompt := "You are a helpful assistant for a Headless CMS. Answer the user's question based on the provided context. If the context doesn't contain relevant information, say so."
		userPrompt := fmt.Sprintf("Context:\n%s\n\nQuestion: %s", strings.Join(contextParts, "\n\n---\n"), query)
		answer, err = s.llm.Complete(ctx, systemPrompt, userPrompt)
		if err != nil {
			slog.Warn("rag: LLM completion failed", "error", err)
			answer = ""
		}
	}

	ctxChunks := make([]RAGContextChunk, len(chunks))
	copy(ctxChunks, chunks)

	return &RAGAnswer{
		Query:   query,
		Answer:  answer,
		Context: ctxChunks,
		Took:    time.Since(start),
	}, nil
}

// ─── Reindex ─────────────────────────────────────────────────────────────────

// ReindexAll rebuilds the entire vector index from the articles table. It
// loads all published articles, generates embeddings in batches, and replaces
// the entire vector store. This is safe to call on startup or via the admin API.
func (s *RAGService) ReindexAll(ctx context.Context) (int, error) {
	return s.reindex(ctx, 0)
}

// ReindexTenant rebuilds the vector index for a single tenant. Only published
// articles within the specified tenant are indexed.
func (s *RAGService) ReindexTenant(ctx context.Context, tenantID uint) (int, error) {
	return s.reindex(ctx, tenantID)
}

func (s *RAGService) reindex(ctx context.Context, tenantID uint) (int, error) {
	s.mu.Lock()
	if s.indexing {
		s.mu.Unlock()
		return 0, fmt.Errorf("reindex already in progress")
	}
	s.indexing = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.indexing = false
		s.mu.Unlock()
	}()

	const batchSize = 100
	var totalIndexed int
	var offset int
	// Every published article seen in this run. Anything absent from this set
	// is an orphan (deleted/unpublished outside the sync paths, or a previous
	// failure) and gets pruned from the store and the database below.
	keepDocs := make(map[string]bool)
	keepArticleIDs := make([]uint, 0, 256)

	for {
		var articles []models.Article
		q := s.db.Select("id, tenant_id, title, slug, content, status, post_type, locale").
			Where("deleted_at IS NULL AND status = ?", string(models.StatusPublished)).
			Order("id ASC").
			Offset(offset).
			Limit(batchSize)
		if tenantID > 0 {
			q = q.Where("tenant_id = ?", tenantID)
		}
		if err := q.Find(&articles).Error; err != nil {
			return totalIndexed, fmt.Errorf("load articles batch: %w", err)
		}
		if len(articles) == 0 {
			break
		}

		for i := range articles {
			a := &articles[i]
			if err := s.IndexArticle(ctx, a); err != nil {
				slog.Warn("rag: failed to index article", "article_id", a.ID, "error", err)
				continue
			}
			keepDocs[vectorDocKey("article", a.ID)] = true
			keepArticleIDs = append(keepArticleIDs, a.ID)
			totalIndexed++
		}

		offset += batchSize
		if len(articles) < batchSize {
			break
		}
	}

	// Cleanup-style rebuild: drop in-store and on-disk vectors that are not
	// backed by a published article in scope. A stale vector is worse than a
	// missing one, so articles whose indexing failed are pruned as well.
	if pruned, err := s.store.Prune(ctx, tenantID, keepDocs); err != nil {
		slog.Warn("rag: vector store prune failed", "tenant_id", tenantID, "error", err)
	} else if pruned > 0 {
		slog.Info("rag: pruned orphaned store vectors", "count", pruned, "tenant_id", tenantID)
	}
	if prunedDB, err := s.pruneArticleEmbeddingRows(tenantID, keepArticleIDs); err != nil {
		slog.Warn("rag: embedding row prune failed", "tenant_id", tenantID, "error", err)
	} else if prunedDB > 0 {
		slog.Info("rag: pruned orphaned embedding rows", "count", prunedDB, "tenant_id", tenantID)
	}

	slog.Info("rag: reindex complete", "indexed", totalIndexed, "tenant_id", tenantID)
	return totalIndexed, nil
}

// pruneArticleEmbeddingRows deletes article embedding rows that are not
// backed by a currently published article in scope (deleted articles,
// unpublished content, or rows left behind by failed indexing runs).
func (s *RAGService) pruneArticleEmbeddingRows(tenantID uint, keepArticleIDs []uint) (int64, error) {
	q := s.db.Where("doc_type = ?", "article")
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if len(keepArticleIDs) > 0 {
		q = q.Where("doc_id NOT IN ?", keepArticleIDs)
	}
	// Empty keep-set: every article embedding row in scope is an orphan, so
	// the doc_type filter alone applies.
	res := q.Delete(&models.DocumentEmbedding{})
	return res.RowsAffected, res.Error
}

// WarmUp loads all existing published-content embeddings from the database
// into the vector store. Called on startup to populate the in-memory index.
// Only published-status records are loaded to ensure deleted or unpublished
// content is not retrievable after a restart.
func (s *RAGService) WarmUp(ctx context.Context) (int, error) {
	var allRecords []models.DocumentEmbedding
	if err := s.db.Where("status = ?", string(models.StatusPublished)).Find(&allRecords).Error; err != nil {
		return 0, fmt.Errorf("load all embeddings: %w", err)
	}
	entries := make([]VectorEntry, len(allRecords))
	for i, r := range allRecords {
		entries[i] = VectorEntry{
			ID:         r.ID,
			TenantID:   r.TenantID,
			DocType:    r.DocType,
			DocID:      r.DocID,
			ChunkIndex: r.ChunkIndex,
			Content:    r.Content,
			Embedding:  []float32(r.Embedding),
			Model:      r.Model,
			Locale:     r.Locale,
			Title:      r.Title,
			Slug:       r.Slug,
			Status:     r.Status,
		}
	}
	if err := s.store.LoadAll(ctx, entries); err != nil {
		return 0, fmt.Errorf("load all into vector store: %w", err)
	}

	total := len(entries)
	slog.Info("rag: warm-up complete", "vectors", total)
	return total, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
