package services

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yamovo/contentx/internal/models"
)

// BuiltinIndexer is an in-memory full-text search engine with no external
// dependencies. It is designed to be the default SEARCH_ENGINE=builtin backend
// and matches the project's lightweight philosophy.
//
// Features:
//   - Inverted index keyed by token -> set of doc IDs
//   - Mixed tokenization: whitespace/punctuation split for Latin, bigram
//     (n=2) sliding window for CJK (Chinese/Japanese/Korean) so that
//     substring queries work without a dictionary
//   - BM25 ranking (k1=1.2, b=0.75) with field weighting (title x3)
//   - Snippet highlight: first occurrence window wrapped in <mark>
//   - Thread-safe via a single RWMutex; reads scale, writes serialized
//
// The index lives in process memory and is rebuilt on startup via
// ReindexAll. For multi-instance deployments configure MeiliSearch instead.
type BuiltinIndexer struct {
	mu sync.RWMutex

	// docs stores the canonical document by composite key "type:id".
	docs map[string]SearchDocument

	// index maps token -> set of doc keys with per-field term frequency.
	// postings[token] = {docKey: tf}
	postings map[string]map[string]int

	// titlePostings maps token -> docKey -> title term frequency (boosted).
	titlePostings map[string]map[string]int

	// docLen stores the total token count per doc (for BM25 length norm).
	docLen map[string]int

	// avgDL is the average document length across the corpus.
	avgDL float64
}

// NewBuiltinIndexer returns a ready-to-use in-memory indexer.
func NewBuiltinIndexer() *BuiltinIndexer {
	return &BuiltinIndexer{
		docs:          make(map[string]SearchDocument),
		postings:      make(map[string]map[string]int),
		titlePostings: make(map[string]map[string]int),
		docLen:        make(map[string]int),
	}
}

func (b *BuiltinIndexer) Name() string { return "builtin" }

func (b *BuiltinIndexer) Ping(ctx context.Context) error { return nil }

// docKey returns the composite key for a document.
func docKey(id uint, docType string) string { return docType + ":" + itoa(id) }

func itoa(n uint) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// Index adds or replaces a document in the index. Re-indexing the same ID
// removes the previous version's postings first to keep term frequencies
// accurate.
func (b *BuiltinIndexer) Index(ctx context.Context, doc SearchDocument) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	key := docKey(doc.ID, doc.Type)
	// Remove previous version if present.
	b.removeLocked(key)

	// Tokenize body (content + excerpt) and title separately so the title
	// can be boosted during scoring.
	titleTokens := tokenize(doc.Title)
	bodyTokens := tokenize(doc.Content)
	// Excerpt acts as a small body extension when content is short.
	bodyTokens = append(bodyTokens, tokenize(doc.Excerpt)...)

	b.docs[key] = doc
	b.docLen[key] = len(bodyTokens) + len(titleTokens)

	for tok, tf := range countTokens(bodyTokens) {
		if b.postings[tok] == nil {
			b.postings[tok] = make(map[string]int)
		}
		b.postings[tok][key] = tf
	}
	for tok, tf := range countTokens(titleTokens) {
		if b.titlePostings[tok] == nil {
			b.titlePostings[tok] = make(map[string]int)
		}
		b.titlePostings[tok][key] = tf
	}
	b.recomputeAvgDLLocked()
	return nil
}

// Delete removes a document from the index.
func (b *BuiltinIndexer) Delete(ctx context.Context, id uint, docType string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.removeLocked(docKey(id, docType))
	b.recomputeAvgDLLocked()
	return nil
}

// removeLocked purges a document's postings. Caller holds b.mu.
func (b *BuiltinIndexer) removeLocked(key string) {
	delete(b.docs, key)
	delete(b.docLen, key)
	for tok, set := range b.postings {
		if _, ok := set[key]; ok {
			delete(set, key)
			if len(set) == 0 {
				delete(b.postings, tok)
			}
		}
	}
	for tok, set := range b.titlePostings {
		if _, ok := set[key]; ok {
			delete(set, key)
			if len(set) == 0 {
				delete(b.titlePostings, tok)
			}
		}
	}
}

// recomputeAvgDLLocked recomputes the average document length.
func (b *BuiltinIndexer) recomputeAvgDLLocked() {
	if len(b.docLen) == 0 {
		b.avgDL = 0
		return
	}
	total := 0
	for _, n := range b.docLen {
		total += n
	}
	b.avgDL = float64(total) / float64(len(b.docLen))
}

// Search runs a BM25-ranked query against the in-memory index.
func (b *BuiltinIndexer) Search(ctx context.Context, q SearchQuery) (*SearchResult, error) {
	start := time.Now()
	if q.Query == "" {
		return &SearchResult{Hits: []SearchHit{}, Took: time.Since(start).String()}, nil
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	// Query tokens are AND-combined: a doc must match every query token
	// (in either body or title) to qualify. This keeps precision high for
	// the common multi-keyword case.
	queryTokens := tokenize(q.Query)
	if len(queryTokens) == 0 {
		return &SearchResult{Hits: []SearchHit{}, Took: time.Since(start).String()}, nil
	}

	// Candidate docKeys: intersection of postings across all query tokens.
	var candidates map[string]struct{}
	for _, qt := range queryTokens {
		hits := b.docsMatchingToken(qt)
		if candidates == nil {
			candidates = make(map[string]struct{}, len(hits))
			for k := range hits {
				candidates[k] = struct{}{}
			}
		} else {
			for k := range candidates {
				if _, ok := hits[k]; !ok {
					delete(candidates, k)
				}
			}
		}
		if len(candidates) == 0 {
			break
		}
	}

	// Apply status/locale/type filters and score remaining candidates.
	type scored struct {
		key   string
		score float64
	}
	results := make([]scored, 0, len(candidates))
	for key := range candidates {
		doc := b.docs[key]
		if q.Status != "" && doc.Status != q.Status {
			continue
		}
		if q.Locale != "" && doc.Locale != q.Locale && doc.Locale != "" {
			continue
		}
		if q.Type != "" && doc.Type != q.Type {
			continue
		}
		s := b.bm25Locked(key, queryTokens)
		results = append(results, scored{key: key, score: s})
	}

	// Sort by score desc, then by published_at desc as a tiebreaker.
	sort.Slice(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		di := b.docs[results[i].key]
		dj := b.docs[results[j].key]
		var ti, tj time.Time
		if di.PublishedAt != nil {
			ti = *di.PublishedAt
		} else {
			ti = di.UpdatedAt
		}
		if dj.PublishedAt != nil {
			tj = *dj.PublishedAt
		} else {
			tj = dj.UpdatedAt
		}
		return ti.After(tj)
	})

	total := int64(len(results))
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	}
	startIdx := (q.Page - 1) * q.PageSize
	endIdx := startIdx + q.PageSize
	if startIdx > len(results) {
		startIdx = len(results)
	}
	if endIdx > len(results) {
		endIdx = len(results)
	}

	hits := make([]SearchHit, 0, endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		doc := b.docs[results[i].key]
		hits = append(hits, SearchHit{
			ID:          doc.ID,
			Type:        doc.Type,
			Title:       doc.Title,
			Excerpt:     doc.Excerpt,
			Slug:        doc.Slug,
			Score:       roundScore(results[i].score),
			Highlight:   buildHighlight(doc.Content, queryTokens),
			Locale:      doc.Locale,
			AuthorID:    doc.AuthorID,
			AuthorName:  doc.AuthorName,
			CategoryID:  doc.CategoryID,
			PublishedAt: doc.PublishedAt,
		})
	}

	return &SearchResult{
		Hits:       hits,
		Total:      total,
		Page:       q.Page,
		PageSize:   q.PageSize,
		TotalPages: totalPages,
		Took:       time.Since(start).String(),
	}, nil
}

// docsMatchingToken returns the union of doc keys whose body or title
// contains the token (or whose token is a prefix of an indexed token, for
// partial-match convenience on short queries).
func (b *BuiltinIndexer) docsMatchingToken(token string) map[string]struct{} {
	out := make(map[string]struct{})
	if set, ok := b.postings[token]; ok {
		for k := range set {
			out[k] = struct{}{}
		}
	}
	if set, ok := b.titlePostings[token]; ok {
		for k := range set {
			out[k] = struct{}{}
		}
	}
	// Prefix expansion: for tokens shorter than 4 chars (common in CJK
	// bigram queries where the user typed a single char), expand to any
	// indexed token that starts with it. This raises recall at the cost
	// of a slightly larger candidate set, which BM25 then reranks.
	if utf8.RuneCountInString(token) <= 2 {
		prefix := token
		for tok, set := range b.postings {
			if strings.HasPrefix(tok, prefix) {
				for k := range set {
					out[k] = struct{}{}
				}
			}
		}
		for tok, set := range b.titlePostings {
			if strings.HasPrefix(tok, prefix) {
				for k := range set {
					out[k] = struct{}{}
				}
			}
		}
	}
	return out
}

// bm25Locked computes the BM25 score for a document across the query tokens.
// Title hits are boosted by a field weight of 3.
func (b *BuiltinIndexer) bm25Locked(key string, queryTokens []string) float64 {
	const (
		k1     = 1.2
		bParam = 0.75 // BM25 length-normalization bias
		titleW = 3.0
		bodyW  = 1.0
	)
	N := len(b.docs)
	if N == 0 {
		return 0
	}
	dl := float64(b.docLen[key])
	var score float64
	idfSeen := make(map[string]bool, len(queryTokens))
	for _, qt := range queryTokens {
		if idfSeen[qt] {
			continue
		}
		idfSeen[qt] = true

		// Document frequency = union of body+title postings.
		df := len(b.docsMatchingToken(qt))
		if df == 0 {
			continue
		}
		// BM25 IDF with +1 smoothing to keep it non-negative.
		idf := math.Log(1 + (float64(N)-float64(df)+0.5)/(float64(df)+0.5))

		tfBody := b.postings[qt][key]
		tfTitle := b.titlePostings[qt][key]
		if tfBody == 0 && tfTitle == 0 {
			continue
		}
		norm := 1 - bParam + bParam*(dl/maxAvgDL(b.avgDL))
		if tfBody > 0 {
			score += bodyW * idf * (float64(tfBody) * (k1 + 1)) / (float64(tfBody) + k1*norm)
		}
		if tfTitle > 0 {
			score += titleW * idf * (float64(tfTitle) * (k1 + 1)) / (float64(tfTitle) + k1*norm)
		}
	}
	return score
}

func maxAvgDL(avg float64) float64 {
	if avg <= 0 {
		return 1
	}
	return avg
}

// ReindexAll replaces the entire index with the provided articles.
func (b *BuiltinIndexer) ReindexAll(ctx context.Context, articles []models.Article) error {
	b.mu.Lock()
	b.docs = make(map[string]SearchDocument, len(articles))
	b.postings = make(map[string]map[string]int)
	b.titlePostings = make(map[string]map[string]int)
	b.docLen = make(map[string]int)
	b.avgDL = 0
	b.mu.Unlock()

	for i := range articles {
		a := &articles[i]
		doc := ArticleToSearchDoc(a)
		if err := b.Index(ctx, doc); err != nil {
			return err
		}
	}
	return nil
}

// ---------- Tokenization ----------

// tokenize splits text into normalized lowercased tokens. It handles two
// scripts differently:
//
//  1. Latin / Cyrillic / digits: split on whitespace and punctuation, then
//     lowercase. Single-char tokens are dropped to keep the index small.
//  2. CJK (Chinese/Japanese/Korean Han, Hiragana, Katakana, Hangul): emit
//     a bigram sliding window so substring search works without a
//     dictionary. A single CJK char produces no token (too noisy); runs of
//     length >= 2 produce len-1 bigrams. This is the same trick used by
//     SQLite's FTS unicode61 tokenizer in bigram mode.
//
// Punctuation and whitespace between scripts act as hard boundaries so a
// Latin word never merges with a CJK run.
func tokenize(s string) []string {
	if s == "" {
		return nil
	}
	var tokens []string
	var cjkRun []rune

	flushCJK := func() {
		if len(cjkRun) < 2 {
			cjkRun = cjkRun[:0]
			return
		}
		for i := 0; i+1 < len(cjkRun); i++ {
			tokens = append(tokens, string(cjkRun[i:i+2]))
		}
		cjkRun = cjkRun[:0]
	}
	flushLatin := func(buf *strings.Builder) {
		if buf.Len() == 0 {
			return
		}
		tok := strings.ToLower(buf.String())
		buf.Reset()
		if utf8.RuneCountInString(tok) >= 2 {
			tokens = append(tokens, tok)
		}
	}

	var latin strings.Builder
	for _, r := range s {
		switch {
		case isCJK(r):
			flushLatin(&latin)
			cjkRun = append(cjkRun, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushCJK()
			latin.WriteRune(r)
		default:
			flushLatin(&latin)
			flushCJK()
		}
	}
	flushLatin(&latin)
	flushCJK()
	return tokens
}

// isCJK reports whether r is a CJK ideograph or kana that benefits from
// bigram tokenization. Ranges cover common CJK blocks; rare extensions are
// intentionally omitted to keep the check fast.
func isCJK(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF: // CJK Unified Ideographs
		return true
	case r >= 0x3400 && r <= 0x4DBF: // CJK Extension A
		return true
	case r >= 0x3040 && r <= 0x309F: // Hiragana
		return true
	case r >= 0x30A0 && r <= 0x30FF: // Katakana
		return true
	case r >= 0xAC00 && r <= 0xD7AF: // Hangul Syllables
		return true
	case r >= 0xF900 && r <= 0xFAFF: // CJK Compatibility Ideographs
		return true
	}
	return false
}

// countTokens returns a frequency map of tokens.
func countTokens(tokens []string) map[string]int {
	out := make(map[string]int, len(tokens))
	for _, t := range tokens {
		out[t]++
	}
	return out
}

// ---------- Highlight ----------

// buildHighlight returns a snippet of content centered on the first occurrence
// of any query token, with every matched token wrapped in <mark>…</mark>.
// When no match is found it falls back to a leading excerpt.
func buildHighlight(content string, queryTokens []string) string {
	if content == "" {
		return ""
	}
	tokens := dedupNonEmpty(queryTokens)
	if len(tokens) == 0 {
		r := []rune(content)
		if len(r) > 160 {
			return string(r[:160]) + "…"
		}
		return content
	}
	lower := strings.ToLower(content)

	// Find the earliest occurrence of any query token (byte offset).
	bestIdx := -1
	for _, qt := range tokens {
		if idx := strings.Index(lower, qt); idx >= 0 && (bestIdx == -1 || idx < bestIdx) {
			bestIdx = idx
		}
	}

	const window = 160
	r := []rune(content)
	if bestIdx == -1 {
		if len(r) > window {
			return string(r[:window]) + "…"
		}
		return content
	}

	// Center the window on the first match (rune indices).
	matchRuneIdx := utf8.RuneCountInString(content[:bestIdx])
	start := matchRuneIdx - window/2
	if start < 0 {
		start = 0
	}
	end := start + window
	if end > len(r) {
		end = len(r)
	}
	snippet := string(r[start:end])
	lowerSnippet := strings.ToLower(snippet)

	// Scan the snippet left-to-right. At each byte position check whether any
	// query token matches here (case-insensitive). If so emit the original-
	// case slice wrapped in <mark>…</mark> and advance past it; otherwise
	// copy one byte. This naturally avoids overlapping replacements.
	var b strings.Builder
	snippetBytes := []byte(snippet)
	lowerBytes := []byte(lowerSnippet)
	i := 0
	for i < len(snippetBytes) {
		matched := false
		for _, qt := range tokens {
			l := len(qt)
			if l == 0 || i+l > len(lowerBytes) {
				continue
			}
			if string(lowerBytes[i:i+l]) == qt {
				b.WriteString("<mark>")
				b.Write(snippetBytes[i : i+l])
				b.WriteString("</mark>")
				i += l
				matched = true
				break
			}
		}
		if !matched {
			b.WriteByte(snippetBytes[i])
			i++
		}
	}
	out := b.String()
	if start > 0 {
		out = "…" + out
	}
	if end < len(r) {
		out = out + "…"
	}
	return out
}

// dedupNonEmpty removes empty strings and duplicates from a token slice,
// preserving first-seen order.
func dedupNonEmpty(tokens []string) []string {
	seen := make(map[string]bool, len(tokens))
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

// roundScore trims floating noise for JSON output.
func roundScore(s float64) float64 {
	if s <= 0 {
		return 0
	}
	return math.Round(s*1000) / 1000
}
