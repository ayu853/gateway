package loadbalancer

import (
	"net/http"
	"sync/atomic"
)

// RoundRobin implements a thread-safe round-robin load balancer.
type RoundRobin struct {
	backends []*Backend
	current   atomic.Uint64
}

// NewRoundRobin creates a new round-robin balancer.
func NewRoundRobin(backends []*Backend) *RoundRobin {
	return &RoundRobin{backends: backends}
}

// Next selects the next healthy backend using round-robin.
func (rr *RoundRobin) Next(r *http.Request) *Backend {
	n := uint64(len(rr.backends))
	if n == 0 {
		return nil
	}

	// Try all backends starting from current position
	for i := uint64(0); i < n; i++ {
		idx := rr.current.Add(1) % n
		backend := rr.backends[idx]
		if backend.IsHealthy() {
			return backend
		}
	}
	return nil
}

// GetBackends returns all backends.
func (rr *RoundRobin) GetBackends() []*Backend {
	return rr.backends
}
