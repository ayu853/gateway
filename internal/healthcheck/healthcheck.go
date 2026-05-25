package healthcheck

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ayu853/gateway/internal/loadbalancer"
)

// Checker performs periodic health checks on backends.
type Checker struct {
	backends           []*loadbalancer.Backend
	interval           time.Duration
	timeout            time.Duration
	path               string
	unhealthyThreshold int
	healthyThreshold   int
	client             *http.Client
	failureCounts      map[string]int
	successCounts      map[string]int
	mu                 sync.Mutex
}

// New creates a new health checker.
func New(backends []*loadbalancer.Backend, interval, timeout time.Duration, path string, unhealthyThreshold, healthyThreshold int) *Checker {
	return &Checker{
		backends:           backends,
		interval:           interval,
		timeout:            timeout,
		path:               path,
		unhealthyThreshold: unhealthyThreshold,
		healthyThreshold:   healthyThreshold,
		client: &http.Client{
			Timeout: timeout,
		},
		failureCounts: make(map[string]int),
		successCounts: make(map[string]int),
	}
}

// Start begins periodic health checking in the background.
// It runs until the context is cancelled.
func (c *Checker) Start(ctx context.Context) {
	log.Printf("[HealthCheck] Starting health checks every %s on path %s", c.interval, c.path)

	// Initial check
	c.checkAll()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[HealthCheck] Stopping health checks")
			return
		case <-ticker.C:
			c.checkAll()
		}
	}
}

func (c *Checker) checkAll() {
	var wg sync.WaitGroup
	for _, backend := range c.backends {
		wg.Add(1)
		go func(b *loadbalancer.Backend) {
			defer wg.Done()
			c.check(b)
		}(backend)
	}
	wg.Wait()
}

func (c *Checker) check(b *loadbalancer.Backend) {
	checkURL := fmt.Sprintf("%s%s", b.URL, c.path)
	resp, err := c.client.Get(checkURL)

	c.mu.Lock()
	defer c.mu.Unlock()

	if err != nil || resp.StatusCode >= 500 {
		c.failureCounts[b.URL]++
		c.successCounts[b.URL] = 0

		if c.failureCounts[b.URL] >= c.unhealthyThreshold && b.IsHealthy() {
			b.MarkUnhealthy()
			log.Printf("[HealthCheck] ❌ Backend %s marked UNHEALTHY (failures: %d)", b.URL, c.failureCounts[b.URL])
		}
	} else {
		c.successCounts[b.URL]++
		c.failureCounts[b.URL] = 0

		if c.successCounts[b.URL] >= c.healthyThreshold && !b.IsHealthy() {
			b.MarkHealthy()
			log.Printf("[HealthCheck] ✅ Backend %s marked HEALTHY", b.URL)
		}
	}

	if resp != nil {
		resp.Body.Close()
	}
}

// GetStatus returns the health status of all backends.
func (c *Checker) GetStatus() []BackendStatus {
	c.mu.Lock()
	defer c.mu.Unlock()

	statuses := make([]BackendStatus, len(c.backends))
	for i, b := range c.backends {
		statuses[i] = BackendStatus{
			URL:               b.URL,
			Healthy:           b.IsHealthy(),
			ActiveConnections: b.GetActiveConnections(),
			FailureCount:      c.failureCounts[b.URL],
			SuccessCount:      c.successCounts[b.URL],
		}
	}
	return statuses
}

// BackendStatus represents the health status of a backend.
type BackendStatus struct {
	URL               string `json:"url"`
	Healthy           bool   `json:"healthy"`
	ActiveConnections int64  `json:"active_connections"`
	FailureCount      int    `json:"failure_count"`
	SuccessCount      int    `json:"success_count"`
}
