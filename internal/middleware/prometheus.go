package middleware

import (
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// PrometheusCollector 收集 Prometheus 格式的指标，零外部依赖。
// 输出符合 Prometheus exposition format v0.0.4 文本格式。
type PrometheusCollector struct {
	mu sync.RWMutex

	// HTTP 指标
	requestCounts   map[string]int64             // key: "METHOD|PATH|STATUS"
	durationBuckets map[string]*histogramBuckets // key: "METHOD|PATH"

	// 业务指标（通过 Inc/Add 方法更新）
	counters map[string]*promCounter
	gauges   map[string]*promGauge

	startTime time.Time
	snapshot  func()
}

// histogramBuckets 实现简单的直方图分桶。
type histogramBuckets struct {
	counts []int64   // 按 bucket 边界分桶
	bounds []float64 // bucket 上界（秒）
	sum    float64
	total  int64
}

// 默认延迟分桶（秒），覆盖 5ms ~ 10s 范围
var defaultDurationBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
}

type promCounter struct {
	name   string
	value  int64
	help   string
	labels map[string]string
}

type promGauge struct {
	name   string
	value  float64
	help   string
	labels map[string]string
}

// NewPrometheusCollector 创建采集器。
func NewPrometheusCollector() *PrometheusCollector {
	return &PrometheusCollector{
		requestCounts:   make(map[string]int64),
		durationBuckets: make(map[string]*histogramBuckets),
		counters:        make(map[string]*promCounter),
		gauges:          make(map[string]*promGauge),
		startTime:       time.Now(),
	}
}

// PrometheusMiddleware 采集 HTTP 请求指标。
func PrometheusMiddleware(collector *PrometheusCollector) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()

		method := c.Request.Method
		path := normalizePath(c.FullPath())
		status := fmt.Sprintf("%d", c.Writer.Status())

		key := method + "|" + path + "|" + status
		durationKey := method + "|" + path

		collector.mu.Lock()
		collector.requestCounts[key]++

		hist, ok := collector.durationBuckets[durationKey]
		if !ok {
			hist = &histogramBuckets{
				bounds: defaultDurationBuckets,
				counts: make([]int64, len(defaultDurationBuckets)+1), // +1 for +Inf bucket
			}
			collector.durationBuckets[durationKey] = hist
		}
		hist.total++
		hist.sum += duration
		for i, bound := range hist.bounds {
			if duration <= bound {
				hist.counts[i]++
				break
			}
		}
		if duration > hist.bounds[len(hist.bounds)-1] {
			hist.counts[len(hist.counts)-1]++ // +Inf bucket
		}
		collector.mu.Unlock()
	}
}

// IncCounter 增加业务计数器。
func (p *PrometheusCollector) IncCounter(name string, help string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.counters[name]; ok {
		atomic.AddInt64(&c.value, 1)
		return
	}
	p.counters[name] = &promCounter{name: name, value: 1, help: help}
}

// AddCounter 增加业务计数器的值。
func (p *PrometheusCollector) AddCounter(name string, help string, delta int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.counters[name]; ok {
		atomic.AddInt64(&c.value, delta)
		return
	}
	p.counters[name] = &promCounter{name: name, value: delta, help: help}
}

// SetGauge 设置业务 gauge 值。
func (p *PrometheusCollector) SetGauge(name string, help string, value float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if g, ok := p.gauges[name]; ok {
		g.value = value
		return
	}
	p.gauges[name] = &promGauge{name: name, value: value, help: help}
}

// IncCounterWithLabels increments a counter identified by a stable label set.
func (p *PrometheusCollector) IncCounterWithLabels(name, help string, labels map[string]string) {
	key := metricKey(name, labels)
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.counters[key]; ok {
		atomic.AddInt64(&c.value, 1)
		return
	}
	p.counters[key] = &promCounter{name: name, value: 1, help: help, labels: cloneLabels(labels)}
}

// SetGaugeWithLabels sets a gauge identified by a stable label set.
func (p *PrometheusCollector) SetGaugeWithLabels(name, help string, labels map[string]string, value float64) {
	key := metricKey(name, labels)
	p.mu.Lock()
	defer p.mu.Unlock()
	if g, ok := p.gauges[key]; ok {
		g.value = value
		return
	}
	p.gauges[key] = &promGauge{name: name, value: value, help: help, labels: cloneLabels(labels)}
}

// SetSnapshotter registers a callback run immediately before metrics are rendered.
func (p *PrometheusCollector) SetSnapshotter(snapshot func()) {
	p.mu.Lock()
	p.snapshot = snapshot
	p.mu.Unlock()
}

// MetricsHandler 输出 Prometheus exposition format 文本。
func (p *PrometheusCollector) MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		p.mu.RLock()
		snapshot := p.snapshot
		p.mu.RUnlock()
		if snapshot != nil {
			snapshot()
		}

		var sb strings.Builder

		// --- HTTP 指标 ---
		p.mu.RLock()
		defer p.mu.RUnlock()

		// 1. http_requests_total
		sb.WriteString("# HELP http_requests_total Total number of HTTP requests.\n")
		sb.WriteString("# TYPE http_requests_total counter\n")
		keys := make([]string, 0, len(p.requestCounts))
		for k := range p.requestCounts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts := strings.SplitN(k, "|", 3)
			fmt.Fprintf(&sb, "http_requests_total{method=%q,path=%q,status=%q} %d\n",
				parts[0], parts[1], parts[2], p.requestCounts[k])
		}

		// 2. http_request_duration_seconds
		sb.WriteString("\n# HELP http_request_duration_seconds HTTP request latency distribution.\n")
		sb.WriteString("# TYPE http_request_duration_seconds histogram\n")
		durKeys := make([]string, 0, len(p.durationBuckets))
		for k := range p.durationBuckets {
			durKeys = append(durKeys, k)
		}
		sort.Strings(durKeys)
		for _, k := range durKeys {
			parts := strings.SplitN(k, "|", 2)
			method, path := parts[0], parts[1]
			hist := p.durationBuckets[k]

			for i, bound := range hist.bounds {
				fmt.Fprintf(&sb, "http_request_duration_seconds_bucket{method=%q,path=%q,le=\"%g\"} %d\n",
					method, path, bound, hist.counts[i])
			}
			fmt.Fprintf(&sb, "http_request_duration_seconds_bucket{method=%q,path=%q,le=\"+Inf\"} %d\n",
				method, path, hist.total)
			fmt.Fprintf(&sb, "http_request_duration_seconds_sum{method=%q,path=%q} %g\n",
				method, path, hist.sum)
			fmt.Fprintf(&sb, "http_request_duration_seconds_count{method=%q,path=%q} %d\n",
				method, path, hist.total)
		}

		// --- 业务计数器 ---
		counterMetadata := make(map[string]struct{})
		for _, c := range p.counters {
			if _, emitted := counterMetadata[c.name]; !emitted {
				fmt.Fprintf(&sb, "\n# HELP %s %s\n", c.name, c.help)
				fmt.Fprintf(&sb, "# TYPE %s counter\n", c.name)
				counterMetadata[c.name] = struct{}{}
			}
			fmt.Fprintf(&sb, "%s%s %d\n", c.name, formatLabels(c.labels), atomic.LoadInt64(&c.value))
		}

		// --- 业务 Gauge ---
		gaugeMetadata := make(map[string]struct{})
		for _, g := range p.gauges {
			if _, emitted := gaugeMetadata[g.name]; !emitted {
				fmt.Fprintf(&sb, "\n# HELP %s %s\n", g.name, g.help)
				fmt.Fprintf(&sb, "# TYPE %s gauge\n", g.name)
				gaugeMetadata[g.name] = struct{}{}
			}
			fmt.Fprintf(&sb, "%s%s %g\n", g.name, formatLabels(g.labels), g.value)
		}

		// --- Go runtime 指标 ---
		sb.WriteString("\n# HELP go_goroutines Number of running goroutines.\n")
		sb.WriteString("# TYPE go_goroutines gauge\n")
		fmt.Fprintf(&sb, "go_goroutines %d\n", runtime.NumGoroutine())

		sb.WriteString("\n# HELP go_memstats_alloc_bytes Number of bytes allocated and still in use.\n")
		sb.WriteString("# TYPE go_memstats_alloc_bytes gauge\n")
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		fmt.Fprintf(&sb, "go_memstats_alloc_bytes %d\n", m.Alloc)

		sb.WriteString("\n# HELP go_memstats_sys_bytes Number of bytes obtained from system.\n")
		sb.WriteString("# TYPE go_memstats_sys_bytes gauge\n")
		fmt.Fprintf(&sb, "go_memstats_sys_bytes %d\n", m.Sys)

		sb.WriteString("\n# HELP process_uptime_seconds Uptime in seconds.\n")
		sb.WriteString("# TYPE process_uptime_seconds gauge\n")
		fmt.Fprintf(&sb, "process_uptime_seconds %d\n", int64(time.Since(p.startTime).Seconds()))

		_, _ = w.Write([]byte(sb.String()))
	}
}

func metricKey(name string, labels map[string]string) string {
	return name + formatLabels(labels)
}

func cloneLabels(labels map[string]string) map[string]string {
	result := make(map[string]string, len(labels))
	for key, value := range labels {
		result[key] = value
	}
	return result
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteByte('{')
	for index, key := range keys {
		if index > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%s=%q", key, labels[key])
	}
	sb.WriteByte('}')
	return sb.String()
}

// normalizePath 将路径中的参数占位符标准化（:id -> :param），
// 避免高基数标签导致 Prometheus 内存膨胀。
func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	// 将 :id, :slug 等参数统一为 :param，降低基数
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, ":") {
			parts[i] = ":param"
		}
	}
	return strings.Join(parts, "/")
}
