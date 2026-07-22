package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/config"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// BenchmarkGenerateRequestID benchmarks UUID-based request ID generation.
func BenchmarkGenerateRequestID(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = generateRequestID()
		}
	})
}

// BenchmarkRateLimitMiddleware benchmarks the sharded rate limiter under concurrent load.
func BenchmarkRateLimitMiddleware(b *testing.B) {
	router := gin.New()
	router.Use(RateLimitMiddleware(10000))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}

// BenchmarkRateLimitMiddleware_MultiIP benchmarks rate limiter with multiple distinct IPs.
func BenchmarkRateLimitMiddleware_MultiIP(b *testing.B) {
	router := gin.New()
	router.Use(RateLimitMiddleware(10000))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	ips := []string{
		"10.0.0.1:1234", "10.0.0.2:1234", "10.0.0.3:1234", "10.0.0.4:1234",
		"10.0.1.1:1234", "10.0.1.2:1234", "10.0.1.3:1234", "10.0.1.4:1234",
		"172.16.0.1:1234", "172.16.0.2:1234", "172.16.0.3:1234", "172.16.0.4:1234",
		"192.168.0.1:1234", "192.168.0.2:1234", "192.168.0.3:1234", "192.168.0.4:1234",
	}

	var counter uint64
	var mu sync.Mutex

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			idx := counter % uint64(len(ips))
			counter++
			mu.Unlock()

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = ips[idx]
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}

// BenchmarkGroupRateLimit benchmarks the per-group sharded rate limiter.
func BenchmarkGroupRateLimit(b *testing.B) {
	rl := NewIPRateLimit()
	rl.Add("api", 10000)

	router := gin.New()
	router.Use(GroupRateLimit(rl, "api"))
	router.GET("/api/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}

// BenchmarkCORSMiddleware benchmarks CORS middleware processing.
func BenchmarkCORSMiddleware(b *testing.B) {
	cfg := config.CORSConfig{
		AllowedOrigins:   []string{"http://localhost:3000", "https://example.com"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           86400,
	}

	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", "http://localhost:3000")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}

// BenchmarkRequestIDMiddleware benchmarks the request ID middleware.
func BenchmarkRequestIDMiddleware(b *testing.B) {
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}

// BenchmarkRequestIDMiddleware_ExistingHeader benchmarks when X-Request-ID already exists.
func BenchmarkRequestIDMiddleware_ExistingHeader(b *testing.B) {
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Request-ID", "existing-request-id-12345")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}
