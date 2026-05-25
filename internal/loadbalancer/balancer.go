package loadbalancer

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
)

// Backend represents a backend server.
type Backend struct {
	URL               string
	Weight            int
	Healthy           atomic.Bool
	ActiveConnections atomic.Int64
	mu                sync.RWMutex
}

// NewBackend creates a new backend with the given URL and weight.
func NewBackend(url string, weight int) *Backend {
	b := &Backend{
		URL:    url,
		Weight: weight,
	}
	b.Healthy.Store(true)
	return b
}

// IsHealthy returns whether the backend is healthy.
func (b *Backend) IsHealthy() bool {
	return b.Healthy.Load()
}

// MarkHealthy marks the backend as healthy.
func (b *Backend) MarkHealthy() {
	b.Healthy.Store(true)
}

// MarkUnhealthy marks the backend as unhealthy.
func (b *Backend) MarkUnhealthy() {
	b.Healthy.Store(false)
}

// IncrementConnections increments the active connection count.
func (b *Backend) IncrementConnections() {
	b.ActiveConnections.Add(1)
}

// DecrementConnections decrements the active connection count.
func (b *Backend) DecrementConnections() {
	b.ActiveConnections.Add(-1)
}

// GetActiveConnections returns the current active connection count.
func (b *Backend) GetActiveConnections() int64 {
	return b.ActiveConnections.Load()
}

// Balancer is the interface for load balancing algorithms.
type Balancer interface {
	// Next selects the next healthy backend for the given request.
	Next(r *http.Request) *Backend
	// GetBackends returns all backends.
	GetBackends() []*Backend
}

// New creates a new load balancer with the given algorithm and backends.
func New(algorithm string, backends []*Backend) (Balancer, error) {
	switch algorithm {
	case "round-robin":
		return NewRoundRobin(backends), nil
	case "least-conn":
		return NewLeastConn(backends), nil
	case "weighted-rr":
		return NewWeightedRR(backends), nil
	case "ip-hash":
		return NewIPHash(backends), nil
	default:
		return nil, fmt.Errorf("unknown balancing algorithm: %s", algorithm)
	}
}
