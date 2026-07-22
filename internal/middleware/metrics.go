package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// MetricsCollector collects HTTP request metrics in memory.
// For production, use prometheus/client_golang.
type MetricsCollector struct {
	mu            sync.RWMutex
	requestCounts map[string]int64
	latencySum    map[string]time.Duration
	latencyCount  map[string]int64
	statusCounts  map[string]int64
	startTime     time.Time
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		requestCounts: make(map[string]int64),
		latencySum:    make(map[string]time.Duration),
		latencyCount:  make(map[string]int64),
		statusCounts:  make(map[string]int64),
		startTime:     time.Now(),
	}
}

// MetricsMiddleware records request metrics.
func MetricsMiddleware(collector *MetricsCollector) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		key := fmt.Sprintf("%s %s", c.Request.Method, c.FullPath())
		statusKey := fmt.Sprintf("%d", c.Writer.Status())

		collector.mu.Lock()
		collector.requestCounts[key]++
		collector.latencySum[key] += latency
		collector.latencyCount[key]++
		collector.statusCounts[statusKey]++
		collector.mu.Unlock()
	}
}

// MetricsResponse is the metrics data structure.
type MetricsResponse struct {
	Uptime        string            `json:"uptime"`
	TotalRequests int64             `json:"total_requests"`
	Endpoints     []EndpointMetrics `json:"endpoints"`
	StatusCodes   map[string]int64  `json:"status_codes"`
}

// EndpointMetrics holds metrics for a single endpoint.
type EndpointMetrics struct {
	Endpoint     string  `json:"endpoint"`
	Requests     int64   `json:"requests"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// GetMetrics returns current metrics snapshot.
func (m *MetricsCollector) GetMetrics() MetricsResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var total int64
	var endpoints []EndpointMetrics
	for key, count := range m.requestCounts {
		total += count
		avgMs := float64(0)
		if m.latencyCount[key] > 0 {
			avgMs = float64(m.latencySum[key].Microseconds()) / float64(m.latencyCount[key]) / 1000.0
		}
		endpoints = append(endpoints, EndpointMetrics{
			Endpoint:     key,
			Requests:     count,
			AvgLatencyMs: avgMs,
		})
	}

	// Copy status counts.
	statusCodes := make(map[string]int64, len(m.statusCounts))
	for k, v := range m.statusCounts {
		statusCodes[k] = v
	}

	return MetricsResponse{
		Uptime:        time.Since(m.startTime).Round(time.Second).String(),
		TotalRequests: total,
		Endpoints:     endpoints,
		StatusCodes:   statusCodes,
	}
}
