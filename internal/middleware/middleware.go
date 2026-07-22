package middleware

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// RecoverMiddleware recovers from panics and logs the error.
func RecoverMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"error", err,
					"stack", string(debug.Stack()),
					"path", c.Request.URL.Path,
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

// RequestID generates a unique request ID for each request using UUID.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

// LoggerMiddleware logs HTTP requests using structured logging.
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		attrs := []slog.Attr{
			slog.Int("status", status),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("query", query),
			slog.String("ip", c.ClientIP()),
			slog.Duration("latency", latency),
			slog.Int("bytes", c.Writer.Size()),
		}

		if requestID, exists := c.Get("request_id"); exists {
			attrs = append(attrs, slog.String("request_id", requestID.(string)))
		}

		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		slog.LogAttrs(c.Request.Context(), level, "HTTP request", attrs...)
	}
}

// CORSMiddleware handles Cross-Origin Resource Sharing.
func CORSMiddleware(cfg config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		allowed := false
		for _, o := range cfg.AllowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
			// Wildcard subdomain matching.
			if len(o) > 2 && o[:2] == "*." {
				domain := o[2:]
				if len(origin) > len(domain)+1 && origin[len(origin)-len(domain)-1:] == "."+domain {
					allowed = true
					break
				}
			}
		}

		if !allowed {
			c.Next()
			return
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", joinStrings(cfg.AllowedMethods))
		c.Header("Access-Control-Allow-Headers", joinStrings(cfg.AllowedHeaders))
		c.Header("Access-Control-Allow-Credentials", fmt.Sprintf("%v", cfg.AllowCredentials))
		c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.MaxAge))

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// SecurityHeaders adds security-related HTTP headers.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip CSP for Swagger UI (needs inline scripts).
		if len(path) >= 9 && path[:9] == "/swagger/" {
			c.Header("X-Content-Type-Options", "nosniff")
			c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
			c.Next()
			return
		}

		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:;")

		if gin.Mode() == gin.ReleaseMode {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}

// ContentTypeJSON sets the Content-Type header to JSON for API routes.
func ContentTypeJSON() gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(c.Request.URL.Path) > 4 && c.Request.URL.Path[:4] == "/api" {
			c.Header("Content-Type", "application/json; charset=utf-8")
		}
		c.Next()
	}
}

// ActivityLogger logs mutation requests (POST, PUT, DELETE) to the activity log.
func ActivityLogger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		method := c.Request.Method
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			return
		}

		// Only log successful mutations.
		if c.Writer.Status() >= 400 {
			return
		}

		user, exists := c.Get(ContextKeyUser)
		if !exists {
			return
		}

		u := user.(*models.User)
		log := models.ActivityLog{
			UserID:    &u.ID,
			Action:    method,
			Entity:    c.FullPath(),
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		}
		db.Create(&log)
	}
}

// RateLimitMiddleware provides global rate limiting using sharded locks for better concurrency.
func RateLimitMiddleware(requestsPerMinute int) gin.HandlerFunc {
	const numShards = 16

	type bucket struct {
		tokens    int
		lastReset time.Time
		lastSeen  time.Time
	}

	type shard struct {
		buckets map[string]*bucket
		mu      sync.Mutex
	}

	shards := make([]*shard, numShards)
	for i := range shards {
		shards[i] = &shard{buckets: make(map[string]*bucket)}
	}

	getShard := func(ip string) *shard {
		h := sha256.Sum256([]byte(ip))
		return shards[h[0]%numShards]
	}

	// Cleanup goroutine: remove stale entries every 5 minutes.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			for _, s := range shards {
				s.mu.Lock()
				for ip, b := range s.buckets {
					if now.Sub(b.lastSeen) > 10*time.Minute {
						delete(s.buckets, ip)
					}
				}
				s.mu.Unlock()
			}
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		s := getShard(ip)

		s.mu.Lock()
		b, exists := s.buckets[ip]
		if !exists {
			b = &bucket{tokens: requestsPerMinute, lastReset: time.Now(), lastSeen: time.Now()}
			s.buckets[ip] = b
		}
		b.lastSeen = time.Now()

		if time.Since(b.lastReset) > time.Minute {
			b.tokens = requestsPerMinute
			b.lastReset = time.Now()
		}

		if b.tokens <= 0 {
			s.mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			return
		}

		b.tokens--
		s.mu.Unlock()
		c.Next()
	}
}

// IPRateLimit tracks per-group rate limits using sharded locks for better concurrency.
type IPRateLimit struct {
	groups map[string]*rateGroup
	mu     sync.RWMutex
}

type rateGroup struct {
	requests int
	shards   [16]*rateShard
}

type rateShard struct {
	buckets map[string]*bucketRL
	mu      sync.Mutex
}

type bucketRL struct {
	count     int
	resetTime time.Time
	lastSeen  time.Time
}

func getRateShard(rg *rateGroup, ip string) *rateShard {
	h := sha256.Sum256([]byte(ip))
	return rg.shards[h[0]%16]
}

// NewIPRateLimit creates a new IP-based rate limiter with background cleanup.
func NewIPRateLimit() *IPRateLimit {
	rl := &IPRateLimit{groups: make(map[string]*rateGroup)}

	// Cleanup goroutine: remove stale buckets every 5 minutes.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.mu.RLock()
			now := time.Now()
			for _, g := range rl.groups {
				for _, s := range g.shards {
					s.mu.Lock()
					for ip, b := range s.buckets {
						if now.Sub(b.lastSeen) > 10*time.Minute {
							delete(s.buckets, ip)
						}
					}
					s.mu.Unlock()
				}
			}
			rl.mu.RUnlock()
		}
	}()

	return rl
}

// Add registers a new rate limit group.
func (rl *IPRateLimit) Add(group string, requestsPerMinute int) {
	rg := &rateGroup{requests: requestsPerMinute}
	for i := range rg.shards {
		rg.shards[i] = &rateShard{buckets: make(map[string]*bucketRL)}
	}
	rl.groups[group] = rg
}

// Shutdown cleans up resources.
func (rl *IPRateLimit) Shutdown() {
	// No-op for in-memory implementation.
}

// GroupRateLimit creates middleware for a specific rate limit group.
func GroupRateLimit(rl *IPRateLimit, group string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rl.mu.RLock()
		g, exists := rl.groups[group]
		rl.mu.RUnlock()
		if !exists {
			c.Next()
			return
		}

		ip := c.ClientIP()
		s := getRateShard(g, ip)

		s.mu.Lock()
		b, exists := s.buckets[ip]
		if !exists {
			b = &bucketRL{count: g.requests, resetTime: time.Now(), lastSeen: time.Now()}
			s.buckets[ip] = b
		}
		b.lastSeen = time.Now()

		if time.Since(b.resetTime) > time.Minute {
			b.count = g.requests
			b.resetTime = time.Now()
		}

		if b.count <= 0 {
			s.mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			return
		}

		b.count--
		s.mu.Unlock()
		c.Next()
	}
}

// Helper functions.

func generateRequestID() string {
	return uuid.New().String()
}

func joinStrings(strs []string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
