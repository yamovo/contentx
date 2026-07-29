package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/yamovo/contentx/internal/cache"
	"github.com/yamovo/contentx/internal/models"
)

// WithCache attaches a cache driver and TTL to the ArticleService, enabling
// caching for List and Get(by ID). Write operations automatically invalidate
// the cache via generation bumps (lists) and per-key deletes (details).
// Returns the service for chaining. Not calling WithCache means no caching.
func (s *ArticleService) WithCache(d cache.Driver, ttl time.Duration) *ArticleService {
	s.cache = d
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	s.cacheTTL = ttl
	return s
}

// listCacheKey builds a generation-prefixed key for a list query. When cacheGen
// increments (on any write), all prior list keys become unreachable (natural
// invalidation without needing prefix-based delete).
func (s *ArticleService) listCacheKey(filter ListArticlesFilter) string {
	gen := atomic.LoadUint64(&s.cacheGen)
	data, _ := json.Marshal(filter)
	h := sha256.Sum256(data)
	return fmt.Sprintf("articles:list:v%d:%s", gen, hex.EncodeToString(h[:8]))
}

// cacheGetList reads a cached list response. Returns false on miss.
func (s *ArticleService) cacheGetList(key string) (models.ListResponse, bool) {
	if s.cache == nil {
		return models.ListResponse{}, false
	}
	data, err := s.cache.Get(context.Background(), key)
	if err != nil {
		return models.ListResponse{}, false
	}
	var cr cachedListResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return models.ListResponse{}, false
	}
	return models.ListResponse{
		Items:      cr.Items,
		Page:       cr.Page,
		PageSize:   cr.PageSize,
		Total:      cr.Total,
		TotalPages: cr.TotalPages,
		HasNext:    cr.HasNext,
		HasPrev:    cr.HasPrev,
	}, true
}

// cacheSetList stores a list response in cache.
func (s *ArticleService) cacheSetList(key string, resp models.ListResponse) {
	if s.cache == nil {
		return
	}
	articles, ok := resp.Items.([]models.Article)
	if !ok {
		return
	}
	cr := cachedListResponse{
		Items:      articles,
		Page:       resp.Page,
		PageSize:   resp.PageSize,
		Total:      resp.Total,
		TotalPages: resp.TotalPages,
		HasNext:    resp.HasNext,
		HasPrev:    resp.HasPrev,
	}
	data, err := json.Marshal(cr)
	if err != nil {
		return
	}
	_ = s.cache.Set(context.Background(), key, data, s.cacheTTL)
}

// cachedListResponse is the typed form of ListResponse for cache serialization.
// Items must be concrete []models.Article (not interface{}) for unmarshal.
type cachedListResponse struct {
	Items      []models.Article `json:"items"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	Total      int64            `json:"total"`
	TotalPages int              `json:"total_pages"`
	HasNext    bool             `json:"has_next"`
	HasPrev    bool             `json:"has_prev"`
}

// articleCacheKey builds the per-ID detail cache key. Shared by the cache
// helpers and the single-flight group so both sides agree on the key.
func articleCacheKey(id uint) string {
	return fmt.Sprintf("articles:id:%d", id)
}

// cacheGetArticle reads a single article from cache. Returns nil on miss.
func (s *ArticleService) cacheGetArticle(id uint) *models.Article {
	if s.cache == nil {
		return nil
	}
	key := articleCacheKey(id)
	data, err := s.cache.Get(context.Background(), key)
	if err != nil {
		return nil
	}
	var a models.Article
	if err := json.Unmarshal(data, &a); err != nil {
		return nil
	}
	return &a
}

// cacheSetArticle stores a single article in cache.
func (s *ArticleService) cacheSetArticle(a *models.Article) {
	if s.cache == nil || a == nil {
		return
	}
	key := articleCacheKey(a.ID)
	data, err := json.Marshal(a)
	if err != nil {
		return
	}
	_ = s.cache.Set(context.Background(), key, data, s.cacheTTL)
}

// invalidateArticle increments the generation counter (invalidating all cached
// list responses) and deletes the per-ID detail caches for the given articles.
func (s *ArticleService) invalidateArticle(ids ...uint) {
	if s.cache == nil {
		return
	}
	atomic.AddUint64(&s.cacheGen, 1)
	ctx := context.Background()
	for _, id := range ids {
		_ = s.cache.Delete(ctx, articleCacheKey(id))
	}
}
