package metrics

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics collects and exposes Prometheus-compatible metrics.
type Metrics struct {
	requestsTotal    atomic.Int64
	requestsActive   atomic.Int64
	requestDurations []float64
	statusCounts     map[int]*atomic.Int64
	backendCounts    map[string]*atomic.Int64
	mu               sync.RWMutex
	startTime        time.Time
}

// New creates a new Metrics collector.
func New() *Metrics {
	return &Metrics{
		requestDurations: make([]float64, 0, 10000),
		statusCounts:     make(map[int]*atomic.Int64),
		backendCounts:    make(map[string]*atomic.Int64),
		startTime:        time.Now(),
	}
}

// Middleware returns an HTTP middleware that records metrics.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.requestsActive.Add(1)
		defer m.requestsActive.Add(-1)

		start := time.Now()

		wrapped := &metricsResponseWriter{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()

		m.requestsTotal.Add(1)
		m.recordDuration(duration)
		m.recordStatus(wrapped.statusCode)

		if backend := w.Header().Get("X-Backend-Server"); backend != "" {
			m.recordBackend(backend)
		}
	})
}

type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *metricsResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (m *Metrics) recordDuration(d float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.requestDurations) >= 10000 {
		m.requestDurations = m.requestDurations[1:]
	}
	m.requestDurations = append(m.requestDurations, d)
}

func (m *Metrics) recordStatus(code int) {
	m.mu.Lock()
	counter, exists := m.statusCounts[code]
	if !exists {
		counter = &atomic.Int64{}
		m.statusCounts[code] = counter
	}
	m.mu.Unlock()
	counter.Add(1)
}

func (m *Metrics) recordBackend(backend string) {
	m.mu.Lock()
	counter, exists := m.backendCounts[backend]
	if !exists {
		counter = &atomic.Int64{}
		m.backendCounts[backend] = counter
	}
	m.mu.Unlock()
	counter.Add(1)
}

// ServeHTTP exposes metrics in Prometheus text exposition format.
func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	// Basic gateway metrics
	fmt.Fprintf(w, "# HELP gateway_requests_total Total number of requests processed.\n")
	fmt.Fprintf(w, "# TYPE gateway_requests_total counter\n")
	fmt.Fprintf(w, "gateway_requests_total %d\n\n", m.requestsTotal.Load())

	fmt.Fprintf(w, "# HELP gateway_requests_active Current number of active requests.\n")
	fmt.Fprintf(w, "# TYPE gateway_requests_active gauge\n")
	fmt.Fprintf(w, "gateway_requests_active %d\n\n", m.requestsActive.Load())

	fmt.Fprintf(w, "# HELP gateway_uptime_seconds Gateway uptime in seconds.\n")
	fmt.Fprintf(w, "# TYPE gateway_uptime_seconds gauge\n")
	fmt.Fprintf(w, "gateway_uptime_seconds %.2f\n\n", time.Since(m.startTime).Seconds())

	// Status code distribution
	fmt.Fprintf(w, "# HELP gateway_responses_total Total responses by status code.\n")
	fmt.Fprintf(w, "# TYPE gateway_responses_total counter\n")
	m.mu.RLock()
	for code, counter := range m.statusCounts {
		fmt.Fprintf(w, "gateway_responses_total{code=\"%d\"} %d\n", code, counter.Load())
	}
	m.mu.RUnlock()
	fmt.Fprintln(w)

	// Backend distribution
	fmt.Fprintf(w, "# HELP gateway_backend_requests_total Requests per backend.\n")
	fmt.Fprintf(w, "# TYPE gateway_backend_requests_total counter\n")
	m.mu.RLock()
	for backend, counter := range m.backendCounts {
		fmt.Fprintf(w, "gateway_backend_requests_total{backend=\"%s\"} %d\n", backend, counter.Load())
	}
	m.mu.RUnlock()
	fmt.Fprintln(w)

	// Request duration summary
	m.mu.RLock()
	if len(m.requestDurations) > 0 {
		var sum float64
		for _, d := range m.requestDurations {
			sum += d
		}
		avg := sum / float64(len(m.requestDurations))
		fmt.Fprintf(w, "# HELP gateway_request_duration_avg_seconds Average request duration.\n")
		fmt.Fprintf(w, "# TYPE gateway_request_duration_avg_seconds gauge\n")
		fmt.Fprintf(w, "gateway_request_duration_avg_seconds %.6f\n", avg)
	}
	m.mu.RUnlock()
}
