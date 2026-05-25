package loadbalancer

import (
	"net/http"
	"sync"
)

// WeightedRR implements the smooth weighted round-robin algorithm (NGINX-style).
// Backends with higher weights receive proportionally more traffic.
type WeightedRR struct {
	backends       []*Backend
	currentWeights []int
	mu             sync.Mutex
}

// NewWeightedRR creates a new weighted round-robin balancer.
func NewWeightedRR(backends []*Backend) *WeightedRR {
	return &WeightedRR{
		backends:       backends,
		currentWeights: make([]int, len(backends)),
	}
}

// Next selects the next backend using smooth weighted round-robin.
func (wrr *WeightedRR) Next(r *http.Request) *Backend {
	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	totalWeight := 0
	bestIdx := -1
	bestWeight := -1 << 31 // min int

	for i, b := range wrr.backends {
		if !b.IsHealthy() {
			continue
		}

		wrr.currentWeights[i] += b.Weight
		totalWeight += b.Weight

		if wrr.currentWeights[i] > bestWeight {
			bestWeight = wrr.currentWeights[i]
			bestIdx = i
		}
	}

	if bestIdx == -1 {
		return nil
	}

	wrr.currentWeights[bestIdx] -= totalWeight
	return wrr.backends[bestIdx]
}

// GetBackends returns all backends.
func (wrr *WeightedRR) GetBackends() []*Backend {
	return wrr.backends
}
