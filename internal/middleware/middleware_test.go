package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestRouter builds a gin engine with the given middleware and a simple
// /test endpoint for exercising them.
func setupTestRouter(mw ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	for _, m := range mw {
		r.Use(m)
	}
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.OPTIONS("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func doRequest(r *gin.Engine, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	r.ServeHTTP(w, req)
	return w
}

// ---------- RequestID ----------

func TestRequestID_GeneratesUUID(t *testing.T) {
	var ctxID string
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) {
		if v, ok := c.Get("request_id"); ok {
			ctxID, _ = v.(string)
		}
		c.Status(http.StatusOK)
	})

	w := doRequest(r, http.MethodGet, "/test", nil)
	headerID := w.Header().Get("X-Request-ID")
	if headerID == "" {
		t.Fatal("X-Request-ID header not set")
	}
	if _, err := uuid.Parse(headerID); err != nil {
		t.Fatalf("request ID is not a valid UUID: %q", headerID)
	}
	if ctxID != headerID {
		t.Fatal("context request_id should match header")
	}
}

func TestRequestID_PreservesExisting(t *testing.T) {
	r := setupTestRouter(RequestID())
	w := doRequest(r, http.MethodGet, "/test", map[string]string{"X-Request-ID": "my-trace-123"})
	if w.Header().Get("X-Request-ID") != "my-trace-123" {
		t.Fatalf("existing request ID not preserved: %q", w.Header().Get("X-Request-ID"))
	}
}

func TestGenerateRequestID_UniqueUUID(t *testing.T) {
	a, b := generateRequestID(), generateRequestID()
	if a == b {
		t.Fatal("request IDs should be unique")
	}
	if _, err := uuid.Parse(a); err != nil {
		t.Fatalf("not a UUID: %q", a)
	}
}

// ---------- CORS ----------

func testCORSConfig() config.CORSConfig {
	return config.CORSConfig{
		AllowedOrigins:   []string{"https://app.example.com", "*.example.org"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           3600,
	}
}

func TestCORS_AllowedOrigin(t *testing.T) {
	r := setupTestRouter(CORSMiddleware(testCORSConfig()))
	w := doRequest(r, http.MethodGet, "/test", map[string]string{"Origin": "https://app.example.com"})
	if w.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Fatalf("Allow-Origin wrong: %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Methods") != "GET, POST" {
		t.Fatalf("Allow-Methods wrong: %q", w.Header().Get("Access-Control-Allow-Methods"))
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("Allow-Credentials wrong")
	}
}

func TestCORS_WildcardSubdomain(t *testing.T) {
	r := setupTestRouter(CORSMiddleware(testCORSConfig()))
	w := doRequest(r, http.MethodGet, "/test", map[string]string{"Origin": "https://blog.example.org"})
	if w.Header().Get("Access-Control-Allow-Origin") != "https://blog.example.org" {
		t.Fatal("wildcard subdomain should be allowed")
	}
	// Bare domain (no subdomain) should NOT match wildcard.
	w2 := doRequest(r, http.MethodGet, "/test", map[string]string{"Origin": "https://example.org"})
	if w2.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("bare domain should not match wildcard *.example.org")
	}
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	r := setupTestRouter(CORSMiddleware(testCORSConfig()))
	w := doRequest(r, http.MethodGet, "/test", map[string]string{"Origin": "https://evil.com"})
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("disallowed origin should get no CORS headers")
	}
	if w.Code != http.StatusOK {
		t.Fatal("request should still pass through")
	}
}

func TestCORS_Preflight(t *testing.T) {
	r := setupTestRouter(CORSMiddleware(testCORSConfig()))
	w := doRequest(r, http.MethodOptions, "/test", map[string]string{"Origin": "https://app.example.com"})
	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight should return 204, got %d", w.Code)
	}
}

func TestCORS_NoOriginHeader(t *testing.T) {
	r := setupTestRouter(CORSMiddleware(testCORSConfig()))
	w := doRequest(r, http.MethodGet, "/test", nil)
	if w.Code != http.StatusOK {
		t.Fatal("request without Origin should pass")
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("no Origin header → no CORS headers")
	}
}

// ---------- SecurityHeaders ----------

func TestSecurityHeaders_NormalPath(t *testing.T) {
	r := setupTestRouter(SecurityHeaders())
	w := doRequest(r, http.MethodGet, "/test", nil)
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("nosniff missing")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("X-Frame-Options missing")
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'self'") {
		t.Fatal("CSP missing")
	}
	if w.Header().Get("Referrer-Policy") == "" {
		t.Fatal("Referrer-Policy missing")
	}
}

func TestSecurityHeaders_SwaggerSkipsCSP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/swagger/index.html", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := doRequest(r, http.MethodGet, "/swagger/index.html", nil)
	if w.Header().Get("Content-Security-Policy") != "" {
		t.Fatal("CSP should be skipped for swagger")
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("nosniff should still be set for swagger")
	}
}

// ---------- ContentTypeJSON ----------

func TestContentTypeJSON_APIPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ContentTypeJSON())
	r.GET("/api/v1/things", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/page", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := doRequest(r, http.MethodGet, "/api/v1/things", nil)
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("API path should have JSON content type, got %q", ct)
	}
	w2 := doRequest(r, http.MethodGet, "/page", nil)
	if ct := w2.Header().Get("Content-Type"); strings.Contains(ct, "application/json") {
		t.Fatalf("non-API path should not get JSON content type, got %q", ct)
	}
}

// ---------- RecoverMiddleware ----------

func TestRecoverMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RecoverMiddleware())
	r.GET("/panic", func(c *gin.Context) { panic("boom") })

	w := doRequest(r, http.MethodGet, "/panic", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Internal server error") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

// ---------- Rate limiting ----------

func TestRateLimitMiddleware_BlocksAfterLimit(t *testing.T) {
	r := setupTestRouter(RateLimitMiddleware(2))

	for i := 0; i < 2; i++ {
		if w := doRequest(r, http.MethodGet, "/test", nil); w.Code != http.StatusOK {
			t.Fatalf("request %d should pass, got %d", i+1, w.Code)
		}
	}
	if w := doRequest(r, http.MethodGet, "/test", nil); w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after limit, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_PerIPIsolation(t *testing.T) {
	r := setupTestRouter(RateLimitMiddleware(1))

	code := func(ip string) int {
		return doRequest(r, http.MethodGet, "/test", map[string]string{"X-Forwarded-For": ip}).Code
	}

	if code("1.1.1.1") != http.StatusOK {
		t.Fatal("first request from 1.1.1.1 should pass")
	}
	if code("2.2.2.2") != http.StatusOK {
		t.Fatal("first request from different IP should have its own bucket")
	}
	if code("1.1.1.1") != http.StatusTooManyRequests {
		t.Fatal("second request from 1.1.1.1 should be limited")
	}
}

func TestGroupRateLimit(t *testing.T) {
	rl := NewIPRateLimit()
	rl.Add("login", 1)
	r := setupTestRouter(GroupRateLimit(rl, "login"))

	if w := doRequest(r, http.MethodGet, "/test", nil); w.Code != http.StatusOK {
		t.Fatal("first request should pass")
	}
	if w := doRequest(r, http.MethodGet, "/test", nil); w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for group limit, got %d", w.Code)
	}
}

func TestGroupRateLimit_UnknownGroup(t *testing.T) {
	rl := NewIPRateLimit()
	r := setupTestRouter(GroupRateLimit(rl, "nonexistent"))
	for i := 0; i < 5; i++ {
		if w := doRequest(r, http.MethodGet, "/test", nil); w.Code != http.StatusOK {
			t.Fatal("unknown group should not limit")
		}
	}
}

func TestIPRateLimit_Shutdown(t *testing.T) {
	rl := NewIPRateLimit()
	rl.Shutdown() // must not panic
}

// ---------- LoggerMiddleware ----------

func TestLoggerMiddleware(t *testing.T) {
	r := setupTestRouter(RequestID(), LoggerMiddleware())
	if w := doRequest(r, http.MethodGet, "/test", nil); w.Code != http.StatusOK {
		t.Fatalf("logger middleware broke the request: %d", w.Code)
	}
}

// ---------- ActivityLogger ----------

func setupActivityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.ActivityLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestActivityLogger_LogsMutation(t *testing.T) {
	db := setupActivityTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ActivityLogger(db))
	r.POST("/api/v1/articles", func(c *gin.Context) {
		c.Set(ContextKeyUser, &models.User{BaseModel: models.BaseModel{ID: 42}})
		c.Status(http.StatusCreated)
	})

	if w := doRequest(r, http.MethodPost, "/api/v1/articles", nil); w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var count int64
	db.Model(&models.ActivityLog{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 activity log, got %d", count)
	}
	var log models.ActivityLog
	db.First(&log)
	if log.UserID == nil || *log.UserID != 42 {
		t.Fatalf("wrong user ID: %+v", log.UserID)
	}
	if log.Action != "POST" {
		t.Fatalf("wrong action: %q", log.Action)
	}
}

func TestActivityLogger_SkipsGet(t *testing.T) {
	db := setupActivityTestDB(t)
	r := setupTestRouter(ActivityLogger(db))
	doRequest(r, http.MethodGet, "/test", nil)

	var count int64
	db.Model(&models.ActivityLog{}).Count(&count)
	if count != 0 {
		t.Fatalf("GET should not be logged, got %d", count)
	}
}

func TestActivityLogger_SkipsAnonymousAndErrors(t *testing.T) {
	db := setupActivityTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ActivityLogger(db))
	// No user in context.
	r.POST("/anon", func(c *gin.Context) { c.Status(http.StatusOK) })
	// User present but request fails.
	r.POST("/fail", func(c *gin.Context) {
		c.Set(ContextKeyUser, &models.User{BaseModel: models.BaseModel{ID: 1}})
		c.Status(http.StatusBadRequest)
	})

	doRequest(r, http.MethodPost, "/anon", nil)
	doRequest(r, http.MethodPost, "/fail", nil)

	var count int64
	db.Model(&models.ActivityLog{}).Count(&count)
	if count != 0 {
		t.Fatalf("anonymous/error requests should not be logged, got %d", count)
	}
}

// ---------- Helpers ----------

func TestJoinStrings(t *testing.T) {
	if got := joinStrings(nil); got != "" {
		t.Fatalf("empty should be '', got %q", got)
	}
	if got := joinStrings([]string{"a"}); got != "a" {
		t.Fatalf("single: %q", got)
	}
	if got := joinStrings([]string{"a", "b", "c"}); got != "a, b, c" {
		t.Fatalf("multi: %q", got)
	}
}
