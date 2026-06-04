package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     int           // requests per interval
	interval time.Duration // time window
	burst    int           // max burst size
}

type visitor struct {
	tokens   int
	lastSeen time.Time
	mu       sync.Mutex
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(ratePerMinute int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     ratePerMinute,
		interval: time.Minute,
		burst:    ratePerMinute,
	}
	// Cleanup stale visitors every 3 minutes.
	go rl.cleanup()
	return rl
}

// RateLimitMiddleware returns a gin middleware that rate limits by IP.
func RateLimitMiddleware(ratePerMinute int) gin.HandlerFunc {
	limiter := NewRateLimiter(ratePerMinute)
	return func(c *gin.Context) {
		// Only rate-limit API routes, skip static files.
		path := c.Request.URL.Path
		if len(path) < 4 || path[:4] != "/api" {
			c.Next()
			return
		}

		if !limiter.allow(c.ClientIP()) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "请求过于频繁",
				"retry_after": "60s",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	v, exists := rl.visitors[key]
	if !exists {
		v = &visitor{tokens: rl.burst, lastSeen: time.Now()}
		rl.visitors[key] = v
	}
	rl.mu.Unlock()

	v.mu.Lock()
	defer v.mu.Unlock()

	// Refill tokens based on elapsed time.
	elapsed := time.Since(v.lastSeen)
	v.lastSeen = time.Now()
	newTokens := int(elapsed.Seconds() * float64(rl.rate) / rl.interval.Seconds())
	v.tokens += newTokens
	if v.tokens > rl.burst {
		v.tokens = rl.burst
	}

	if v.tokens <= 0 {
		return false
	}

	v.tokens--
	return true
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(3 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		for key, v := range rl.visitors {
			if time.Since(v.lastSeen) > 5*time.Minute {
				delete(rl.visitors, key)
			}
		}
		rl.mu.Unlock()
	}
}

// IPRateLimit uses separate limits for different endpoint groups.
type IPRateLimit struct {
	limiters map[string]*RateLimiter
	mu       sync.RWMutex
}

// NewIPRateLimit creates a multi-endpoint rate limiter.
func NewIPRateLimit() *IPRateLimit {
	return &IPRateLimit{
		limiters: make(map[string]*RateLimiter),
	}
}

// Add registers a rate limit for a specific group.
func (r *IPRateLimit) Add(group string, ratePerMinute int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limiters[group] = NewRateLimiter(ratePerMinute)
}

// Allow checks if a request from ip for the given group is allowed.
func (r *IPRateLimit) Allow(group, ip string) bool {
	r.mu.RLock()
	limiter, ok := r.limiters[group]
	r.mu.RUnlock()
	if !ok {
		return true
	}
	return limiter.allow(ip)
}

// GroupRateLimit creates a middleware for a specific rate-limit group.
func GroupRateLimit(rl *IPRateLimit, group string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.Allow(group, c.ClientIP()) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"group":       group,
				"retry_after": "60s",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
