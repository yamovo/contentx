package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode"
)

// EmbeddingProvider generates dense vector embeddings from text. Implementations
// must be safe for concurrent use.
type EmbeddingProvider interface {
	// Embed converts the input texts into embedding vectors. The returned slice
	// has the same length and order as the input.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dimension returns the vector dimension produced by this provider.
	Dimension() int
	// Name returns a human-readable identifier (e.g. "openai", "dummy").
	Name() string
}

// ─── dummyProvider (TF-IDF with feature hashing) ─────────────────────────────

// dummyProvider generates TF-IDF style vectors using the feature hashing trick.
// It tokenises text into English words and Chinese character bigrams, applies
// sublinear TF scaling, removes stopwords, and L2-normalises the result.
// No external API or pre-built vocabulary is required, making it suitable for
// development, testing, and as a fallback when no embedding API is configured.
type dummyProvider struct {
	dim int
}

func NewDummyEmbeddingProvider(dim int) EmbeddingProvider {
	if dim <= 0 {
		dim = 256
	}
	return &dummyProvider{dim: dim}
}

func (d *dummyProvider) Name() string   { return "dummy" }
func (d *dummyProvider) Dimension() int { return d.dim }

func (d *dummyProvider) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = d.tfidfVector(text)
	}
	return out, nil
}

// tfidfVector builds a fixed-size dense vector from text using sublinear TF
// and feature hashing. Each token is hashed to a dimension; its TF weight is
// accumulated with a random sign to reduce collision bias. The vector is
// L2-normalised so cosine similarity is meaningful.
func (d *dummyProvider) tfidfVector(text string) []float32 {
	tokens := tokenizeForEmbedding(text)
	if len(tokens) == 0 {
		return d.hashToVector(text)
	}

	vec := make([]float32, d.dim)

	// Term frequency with sublinear scaling: tf' = 1 + log(count).
	tf := make(map[string]float64)
	for _, tok := range tokens {
		tf[tok]++
	}
	for tok, count := range tf {
		tf[tok] = 1 + math.Log(count)
	}

	// Feature hashing: map each token to a dimension with signed accumulation.
	for tok, weight := range tf {
		h := fnv1aHash(tok)
		idx := int(h % uint64(d.dim))
		sign := 1.0
		if h&1 == 1 {
			sign = -1.0
		}
		vec[idx] += float32(sign * weight)
	}

	// L2 normalise.
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm > 0 {
		inv := 1.0 / math.Sqrt(norm)
		for i := range vec {
			vec[i] = float32(float64(vec[i]) * inv)
		}
	}
	return vec
}

// hashToVector is the fallback for text that yields no tokens (e.g. empty or
// pure punctuation). It produces a deterministic unit vector via SHA-256.
func (d *dummyProvider) hashToVector(text string) []float32 {
	vec := make([]float32, d.dim)
	seed := []byte(text)
	if len(seed) == 0 {
		seed = []byte("\x00")
	}
	var buf []byte
	counter := 0
	for len(buf) < d.dim*4 {
		h := sha256.New()
		h.Write(seed)
		h.Write([]byte(fmt.Sprintf("%d", counter)))
		buf = append(buf, h.Sum(nil)...)
		counter++
	}
	var norm float64
	for i := 0; i < d.dim; i++ {
		u := uint32(buf[i*4])<<24 | uint32(buf[i*4+1])<<16 | uint32(buf[i*4+2])<<8 | uint32(buf[i*4+3])
		v := float64(u)/float64(math.MaxUint32)*2.0 - 1.0
		vec[i] = float32(v)
		norm += v * v
	}
	if norm > 0 {
		inv := 1.0 / math.Sqrt(norm)
		for i := range vec {
			vec[i] = float32(float64(vec[i]) * inv)
		}
	}
	return vec
}

// ─── Tokenisation helpers ────────────────────────────────────────────────────

// tokenize splits text into meaningful tokens for TF-IDF vectorisation.
// English text is split on whitespace/punctuation into lowercase words.
// Chinese text is segmented into character bigrams (which approximate words
// without a dictionary). Stopwords are removed.
func tokenizeForEmbedding(text string) []string {
	text = strings.ToLower(text)
	runes := []rune(text)
	var tokens []string
	var wordBuf []rune

	flushWord := func() {
		if len(wordBuf) > 1 {
			w := string(wordBuf)
			if !stopwords[w] {
				tokens = append(tokens, w)
			}
		}
		wordBuf = wordBuf[:0]
	}

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if isChineseChar(r) {
			flushWord()
			// Chinese bigram: pair this character with the next Chinese character.
			if i+1 < len(runes) && isChineseChar(runes[i+1]) {
				bigram := string(runes[i : i+2])
				if !stopwords[bigram] {
					tokens = append(tokens, bigram)
				}
			}
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			wordBuf = append(wordBuf, r)
		} else {
			flushWord()
		}
	}
	flushWord()
	return tokens
}

// isChineseChar reports whether r is a CJK Unified Ideograph.
func isChineseChar(r rune) bool {
	return unicode.Is(unicode.Han, r)
}

// fnv1aHash computes the 64-bit FNV-1a hash of s. Fast and deterministic.
func fnv1aHash(s string) uint64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

// stopwords filters out common terms that carry little semantic value.
var stopwords = func() map[string]bool {
	words := []string{
		// English
		"the", "and", "for", "are", "but", "not", "you", "all", "can", "her",
		"was", "one", "our", "out", "day", "get", "has", "him", "his", "how",
		"its", "may", "new", "now", "old", "see", "two", "way", "who", "boy",
		"did", "man", "men", "put", "say", "she", "too", "use", "any", "back",
		"come", "made", "from", "make", "than", "them", "were", "what", "when",
		"will", "your", "this", "that", "with", "have", "been", "they", "would",
		"there", "their", "about", "which", "could", "other", "these", "into",
		"more", "some", "such", "only", "very", "also", "just", "like", "even",
		"well", "over", "does", "done", "must", "should", "shall", "through",
		"after", "before", "where", "while", "being", "those", "each", "both",
		// Chinese bigrams (common functional words)
		"的", "了", "在", "是", "和", "就", "不", "都", "一", "上",
		"也", "很", "到", "要", "去", "会", "着", "没", "看", "好",
		"这", "那", "里", "把", "让", "与", "或", "但", "而", "如",
		"果", "为", "以", "及", "等", "可", "以", "对", "从", "被",
		"使", "由", "于", "按", "向", "给", "跟", "同", "据", "除",
	}
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}()

// ─── openaiProvider ──────────────────────────────────────────────────────────

// openaiProvider calls an OpenAI-compatible /v1/embeddings endpoint. This
// works with OpenAI, Ollama (/api/embeddings), vLLM, LocalAI, and any other
// server that mimics the OpenAI Embedding API shape.
type openaiProvider struct {
	apiKey  string
	baseURL string
	model   string
	dim     int
	client  *http.Client
}

// NewOpenAIEmbeddingProvider creates a provider that calls an OpenAI-compatible
// embedding API. If baseURL is empty it defaults to "https://api.openai.com/v1".
func NewOpenAIEmbeddingProvider(apiKey, baseURL, model string, dim int) EmbeddingProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	// Trim trailing slash for consistent URL joining.
	if baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}
	return &openaiProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		dim:     dim,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (o *openaiProvider) Name() string   { return "openai" }
func (o *openaiProvider) Dimension() int { return o.dim }

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (o *openaiProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(embeddingRequest{Model: o.model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding API call: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API returned %d: %s", resp.StatusCode, string(raw))
	}
	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(result.Data) != len(texts) {
		return nil, fmt.Errorf("embedding count mismatch: requested %d, got %d", len(texts), len(result.Data))
	}
	out := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		out[i] = d.Embedding
	}
	return out, nil
}

// NewEmbeddingProvider is the factory used by routes.go to construct the
// provider from configuration. It never returns nil: when no valid provider
// is configured it falls back to dummy so the system still boots.
func NewEmbeddingProvider(provider, apiKey, baseURL, model string, dim int) EmbeddingProvider {
	switch provider {
	case "openai", "ollama", "localai", "vllm":
		if apiKey == "" && provider == "openai" {
			break
		}
		return NewOpenAIEmbeddingProvider(apiKey, baseURL, model, dim)
	case "", "dummy", "tfidf":
		return NewDummyEmbeddingProvider(dim)
	}
	return NewDummyEmbeddingProvider(dim)
}
