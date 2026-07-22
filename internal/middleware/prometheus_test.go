package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPrometheusMiddleware_RecordsRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	collector := NewPrometheusCollector()

	r := gin.New()
	r.Use(PrometheusMiddleware(collector))
	r.GET("/api/v1/articles", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/api/v1/articles/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
	})

	// 发送 3 个请求到列表 + 2 个请求到详情
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/articles", nil))
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	}
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/articles/42", nil))
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	}

	// 获取 /metrics 输出
	w := httptest.NewRecorder()
	collector.MetricsHandler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	body := w.Body.String()

	// 验证 http_requests_total
	if !strings.Contains(body, "http_requests_total") {
		t.Error("expected http_requests_total in output")
	}
	if !strings.Contains(body, `path="/api/v1/articles"`) {
		t.Error("expected path /api/v1/articles in output")
	}
	if !strings.Contains(body, `path="/api/v1/articles/:param"`) {
		t.Error("expected normalized path with :param in output")
	}

	// 验证计数：列表 3 次，详情 2 次
	if !strings.Contains(body, `http_requests_total{method="GET",path="/api/v1/articles",status="200"} 3`) {
		t.Error("expected 3 requests for articles list")
	}
	if !strings.Contains(body, `http_requests_total{method="GET",path="/api/v1/articles/:param",status="200"} 2`) {
		t.Error("expected 2 requests for article detail")
	}

	// 验证 histogram
	if !strings.Contains(body, "http_request_duration_seconds_bucket") {
		t.Error("expected histogram bucket in output")
	}
	if !strings.Contains(body, "http_request_duration_seconds_sum") {
		t.Error("expected histogram sum in output")
	}
	if !strings.Contains(body, "http_request_duration_seconds_count") {
		t.Error("expected histogram count in output")
	}
}

func TestPrometheusCollector_LabelsAndSnapshot(t *testing.T) {
	collector := NewPrometheusCollector()
	snapshots := 0
	collector.SetSnapshotter(func() {
		snapshots++
		collector.SetGaugeWithLabels("articles_total", "Articles by status", map[string]string{"status": "published"}, 7)
	})
	collector.IncCounterWithLabels("webhook_dispatch_total", "Webhook deliveries", map[string]string{
		"status": "success",
		"event":  "entry.publish",
	})

	w := httptest.NewRecorder()
	collector.MetricsHandler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := w.Body.String()
	if snapshots != 1 {
		t.Fatalf("expected one snapshot, got %d", snapshots)
	}
	if !strings.Contains(body, `articles_total{status="published"} 7`) {
		t.Fatalf("missing labeled article gauge: %s", body)
	}
	if !strings.Contains(body, `webhook_dispatch_total{event="entry.publish",status="success"} 1`) {
		t.Fatalf("missing stable labeled webhook counter: %s", body)
	}
}

func TestPrometheusMiddleware_DifferentStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	collector := NewPrometheusCollector()

	r := gin.New()
	r.Use(PrometheusMiddleware(collector))
	r.GET("/api/v1/error", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail"})
	})
	r.GET("/api/v1/notfound", func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "missing"})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/error", nil))

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/notfound", nil))

	// 404 路由不匹配
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/nonexistent", nil))

	w = httptest.NewRecorder()
	collector.MetricsHandler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	body := w.Body.String()

	if !strings.Contains(body, `status="500"`) {
		t.Error("expected status 500 in output")
	}
	if !strings.Contains(body, `status="404"`) {
		t.Error("expected status 404 in output")
	}
}

func TestPrometheusCollector_CountersAndGauges(t *testing.T) {
	collector := NewPrometheusCollector()

	collector.IncCounter("articles_published_total", "Total published articles")
	collector.IncCounter("articles_published_total", "Total published articles")
	collector.AddCounter("articles_published_total", "Total published articles", 3)

	collector.SetGauge("active_users", "Currently active users", 42)

	w := httptest.NewRecorder()
	collector.MetricsHandler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	body := w.Body.String()

	// Counter 应该是 5 (2 次 Inc + 3 次 Add)
	if !strings.Contains(body, "articles_published_total 5") {
		t.Errorf("expected articles_published_total 5, got body: %s", body)
	}

	// Gauge 应该是 42
	if !strings.Contains(body, "active_users 42") {
		t.Errorf("expected active_users 42, got body: %s", body)
	}

	// 验证 HELP 和 TYPE 注释
	if !strings.Contains(body, "# HELP articles_published_total Total published articles") {
		t.Error("expected HELP comment for articles_published_total")
	}
	if !strings.Contains(body, "# TYPE articles_published_total counter") {
		t.Error("expected TYPE counter for articles_published_total")
	}
	if !strings.Contains(body, "# TYPE active_users gauge") {
		t.Error("expected TYPE gauge for active_users")
	}
}

func TestPrometheusCollector_RuntimeMetrics(t *testing.T) {
	collector := NewPrometheusCollector()

	w := httptest.NewRecorder()
	collector.MetricsHandler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	body := w.Body.String()

	// 验证 Go runtime 指标
	if !strings.Contains(body, "go_goroutines") {
		t.Error("expected go_goroutines in output")
	}
	if !strings.Contains(body, "go_memstats_alloc_bytes") {
		t.Error("expected go_memstats_alloc_bytes in output")
	}
	if !strings.Contains(body, "process_uptime_seconds") {
		t.Error("expected process_uptime_seconds in output")
	}
}

func TestPrometheusCollector_ContentType(t *testing.T) {
	collector := NewPrometheusCollector()

	w := httptest.NewRecorder()
	collector.MetricsHandler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Errorf("expected text/plain content-type, got %s", ct)
	}
	if !strings.Contains(ct, "version=0.0.4") {
		t.Errorf("expected version=0.0.4 in content-type, got %s", ct)
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/api/v1/articles", "/api/v1/articles"},
		{"/api/v1/articles/:id", "/api/v1/articles/:param"},
		{"/api/v1/articles/:id/comments/:cid", "/api/v1/articles/:param/comments/:param"},
		{"/api/v1/articles/:slug", "/api/v1/articles/:param"},
		{"", "/"},
		{"/", "/"},
	}

	for _, tt := range tests {
		got := normalizePath(tt.input)
		if got != tt.expected {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
