package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
)

// VectorEntry is a single indexed vector with its metadata.
type VectorEntry struct {
	ID         uint // DocumentEmbedding.ID
	TenantID   uint
	DocType    string // "article" | "page" | "content_entry"
	DocID      uint
	ChunkIndex int
	Content    string
	Embedding  []float32
	Model      string
	Locale     string
	Title      string
	Slug       string
	Status     string
}

// VectorSearchOpts controls a vector similarity search.
type VectorSearchOpts struct {
	TenantID uint    // required: tenant-scoped search
	TopK     int     // max results
	DocType  string  // empty = any
	Status   string  // empty = any (e.g. "published")
	MinScore float64 // minimum cosine similarity (0..1)
}

// VectorSearchResult is a single search hit.
type VectorSearchResult struct {
	VectorEntry
	Score float64 // cosine similarity (0..1)
}

// VectorStore is the storage-agnostic vector similarity search backend.
// Implementations must be safe for concurrent use and partition by TenantID.
type VectorStore interface {
	Upsert(ctx context.Context, entries []VectorEntry) error
	Delete(ctx context.Context, docType string, docID uint, tenantID uint) error
	Search(ctx context.Context, query []float32, opts VectorSearchOpts) ([]VectorSearchResult, error)
	Count(tenantID uint) int
	LoadAll(ctx context.Context, entries []VectorEntry) error
	// Prune removes entries whose document key (docType:docID) is not in keep
	// and returns how many entries were removed. Used by the cleanup-style
	// reindex to drop orphaned vectors. When tenantID > 0 pruning is scoped to
	// that tenant; zero prunes all tenants.
	Prune(ctx context.Context, tenantID uint, keep map[string]bool) (int, error)
	Name() string
}

// vectorDocKey builds the document identity used by Prune keep-sets.
func vectorDocKey(docType string, docID uint) string {
	return fmt.Sprintf("%s:%d", docType, docID)
}

// ─── memoryVectorStore ───────────────────────────────────────────────────────

// memoryVectorStore is an in-process vector store with brute-force cosine
// similarity. It is partitioned by tenant: each tenant's vectors live in a
// separate slice so searches never cross tenant boundaries.
//
// For small to medium datasets (< 100k vectors per tenant) brute-force cosine
// is fast enough (< 10ms). For larger datasets a pgvector or Qdrant backend
// can be added behind the same interface.
type memoryVectorStore struct {
	mu sync.RWMutex
	// tenantID → []VectorEntry
	data map[uint][]VectorEntry
}

// NewMemoryVectorStore creates a new in-memory vector store.
func NewMemoryVectorStore() VectorStore {
	return &memoryVectorStore{
		data: make(map[uint][]VectorEntry),
	}
}

func (m *memoryVectorStore) Name() string { return "memory" }

func (m *memoryVectorStore) Upsert(_ context.Context, entries []VectorEntry) error {
	if len(entries) == 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := make(map[string]bool)
	for _, e := range entries {
		key := fmt.Sprintf("%s:%d:%d", e.DocType, e.DocID, e.TenantID)
		if !seen[key] {
			m.removeLocked(e.DocType, e.DocID, e.TenantID)
			seen[key] = true
		}
		m.data[e.TenantID] = append(m.data[e.TenantID], e)
	}
	return nil
}

func (m *memoryVectorStore) Delete(_ context.Context, docType string, docID uint, tenantID uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeLocked(docType, docID, tenantID)
	return nil
}

func (m *memoryVectorStore) removeLocked(docType string, docID, tenantID uint) {
	slice := m.data[tenantID]
	filtered := slice[:0]
	for _, e := range slice {
		if e.DocType != docType || e.DocID != docID {
			filtered = append(filtered, e)
		}
	}
	m.data[tenantID] = filtered
}

func (m *memoryVectorStore) Search(_ context.Context, query []float32, opts VectorSearchOpts) ([]VectorSearchResult, error) {
	if len(query) == 0 {
		return nil, nil
	}
	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	candidates := m.data[opts.TenantID]
	if len(candidates) == 0 {
		return nil, nil
	}

	results := make([]VectorSearchResult, 0, len(candidates))
	for _, e := range candidates {
		if opts.DocType != "" && e.DocType != opts.DocType {
			continue
		}
		if opts.Status != "" && e.Status != opts.Status {
			continue
		}
		score := cosineSimilarity(query, e.Embedding)
		if opts.MinScore > 0 && score < opts.MinScore {
			continue
		}
		results = append(results, VectorSearchResult{
			VectorEntry: e,
			Score:       score,
		})
	}

	// Sort by score descending.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func (m *memoryVectorStore) Count(tenantID uint) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data[tenantID])
}

// LoadAll replaces the entire in-memory index with the provided entries.
// Used on startup to warm the cache from the database.
func (m *memoryVectorStore) LoadAll(_ context.Context, entries []VectorEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[uint][]VectorEntry)
	for _, e := range entries {
		m.data[e.TenantID] = append(m.data[e.TenantID], e)
	}
	return nil
}

func (m *memoryVectorStore) Prune(_ context.Context, tenantID uint, keep map[string]bool) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	removed := 0
	if tenantID > 0 {
		removed = m.pruneTenantLocked(tenantID, keep)
		return removed, nil
	}
	for tid := range m.data {
		removed += m.pruneTenantLocked(tid, keep)
	}
	return removed, nil
}

// pruneTenantLocked drops non-keep entries from one tenant partition and
// returns how many entries were removed.
func (m *memoryVectorStore) pruneTenantLocked(tenantID uint, keep map[string]bool) int {
	slice := m.data[tenantID]
	kept := slice[:0]
	removed := 0
	for _, e := range slice {
		if keep[vectorDocKey(e.DocType, e.DocID)] {
			kept = append(kept, e)
		} else {
			removed++
		}
	}
	m.data[tenantID] = kept
	return removed
}

// cosineSimilarity computes the cosine similarity between two vectors.
// Returns 0 if either vector has zero magnitude.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		af := float64(a[i])
		bf := float64(b[i])
		dot += af * bf
		na += af * af
		nb += bf * bf
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
