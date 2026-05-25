package analytics

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Collector tracks request analytics and metrics.
type Collector struct {
	mu              sync.RWMutex
	totalRequests   int64
	totalLatency    time.Duration
	statusCodeDist  map[int]int64
	backendRequests map[string]int64
	pathRequests    map[string]int64
	methodRequests  map[string]int64
	recentLatencies []time.Duration
	startTime       time.Time
}

// New creates a new analytics collector.
func New() *Collector {
	return &Collector{
		statusCodeDist:  make(map[int]int64),
		backendRequests: make(map[string]int64),
		pathRequests:    make(map[string]int64),
		methodRequests:  make(map[string]int64),
		recentLatencies: make([]time.Duration, 0, 1000),
		startTime:       time.Now(),
	}
}

// Record records a request's analytics data.
func (c *Collector) Record(method, path, backend string, statusCode int, latency time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalRequests++
	c.totalLatency += latency
	c.statusCodeDist[statusCode]++
	c.backendRequests[backend]++
	c.pathRequests[path]++
	c.methodRequests[method]++

	// Keep last 1000 latencies for percentile calculations
	if len(c.recentLatencies) >= 1000 {
		c.recentLatencies = c.recentLatencies[1:]
	}
	c.recentLatencies = append(c.recentLatencies, latency)
}

// Middleware returns an HTTP middleware that records analytics.
func (c *Collector) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &analyticsResponseWriter{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(wrapped, r)

		latency := time.Since(start)
		backend := w.Header().Get("X-Backend-Server")
		c.Record(r.Method, r.URL.Path, backend, wrapped.statusCode, latency)
	})
}

type analyticsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *analyticsResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// GetStats returns the current analytics snapshot.
func (c *Collector) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	avgLatency := time.Duration(0)
	if c.totalRequests > 0 {
		avgLatency = c.totalLatency / time.Duration(c.totalRequests)
	}

	// Calculate p99 latency
	p99 := time.Duration(0)
	if len(c.recentLatencies) > 0 {
		sorted := make([]time.Duration, len(c.recentLatencies))
		copy(sorted, c.recentLatencies)
		sortDurations(sorted)
		idx := int(float64(len(sorted)) * 0.99)
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		p99 = sorted[idx]
	}

	return Stats{
		Uptime:          time.Since(c.startTime).String(),
		TotalRequests:   c.totalRequests,
		AvgLatency:      avgLatency.String(),
		P99Latency:      p99.String(),
		StatusCodes:     copyMapInt(c.statusCodeDist),
		BackendRequests: copyMapStr(c.backendRequests),
		PathRequests:    copyMapStr(c.pathRequests),
		MethodRequests:  copyMapStr(c.methodRequests),
	}
}

// ServeHTTP handles the analytics API endpoint.
func (c *Collector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c.GetStats())
}

// Stats holds analytics data.
type Stats struct {
	Uptime          string           `json:"uptime"`
	TotalRequests   int64            `json:"total_requests"`
	AvgLatency      string           `json:"avg_latency"`
	P99Latency      string           `json:"p99_latency"`
	StatusCodes     map[int]int64    `json:"status_codes"`
	BackendRequests map[string]int64 `json:"backend_requests"`
	PathRequests    map[string]int64 `json:"path_requests"`
	MethodRequests  map[string]int64 `json:"method_requests"`
}

func sortDurations(d []time.Duration) {
	// Simple insertion sort for small slices
	for i := 1; i < len(d); i++ {
		key := d[i]
		j := i - 1
		for j >= 0 && d[j] > key {
			d[j+1] = d[j]
			j--
		}
		d[j+1] = key
	}
}

func copyMapInt(src map[int]int64) map[int]int64 {
	dst := make(map[int]int64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyMapStr(src map[string]int64) map[string]int64 {
	dst := make(map[string]int64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
