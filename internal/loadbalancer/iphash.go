package loadbalancer

import (
	"hash/fnv"
	"net"
	"net/http"
)

// IPHash implements an IP hash load balancer for session affinity.
// Clients with the same IP are always routed to the same backend.
type IPHash struct {
	backends []*Backend
}

// NewIPHash creates a new IP hash balancer.
func NewIPHash(backends []*Backend) *IPHash {
	return &IPHash{backends: backends}
}

// Next selects a backend based on the client's IP address hash.
func (ih *IPHash) Next(r *http.Request) *Backend {
	n := len(ih.backends)
	if n == 0 {
		return nil
	}

	clientIP := extractClientIP(r)
	hash := hashIP(clientIP)

	// Try the hashed backend first, then scan for a healthy one
	for i := 0; i < n; i++ {
		idx := (int(hash) + i) % n
		if ih.backends[idx].IsHealthy() {
			return ih.backends[idx]
		}
	}
	return nil
}

// GetBackends returns all backends.
func (ih *IPHash) GetBackends() []*Backend {
	return ih.backends
}

func extractClientIP(r *http.Request) string {
	// Check X-Forwarded-For first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func hashIP(ip string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(ip))
	return h.Sum32()
}
