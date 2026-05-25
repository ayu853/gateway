package loadbalancer

import (
	"net/http"
	"sync"
)

// LeastConn implements a least-connections load balancer.
// It routes traffic to the backend with the fewest active connections.
type LeastConn struct {
	backends []*Backend
	mu       sync.Mutex
}

// NewLeastConn creates a new least-connections balancer.
func NewLeastConn(backends []*Backend) *LeastConn {
	return &LeastConn{backends: backends}
}

// Next selects the healthy backend with the fewest active connections.
func (lc *LeastConn) Next(r *http.Request) *Backend {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	var selected *Backend
	minConns := int64(^uint64(0) >> 1) // max int64

	for _, b := range lc.backends {
		if !b.IsHealthy() {
			continue
		}
		conns := b.GetActiveConnections()
		if conns < minConns {
			minConns = conns
			selected = b
		}
	}
	return selected
}

// GetBackends returns all backends.
func (lc *LeastConn) GetBackends() []*Backend {
	return lc.backends
}
