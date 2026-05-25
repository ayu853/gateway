package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// CircuitState represents the state of the circuit breaker.
type CircuitState int

const (
	StateClosed   CircuitState = iota // Normal operation
	StateOpen                         // Failing, reject requests
	StateHalfOpen                     // Testing recovery
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF-OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker implements the circuit breaker pattern to prevent cascading failures.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            CircuitState
	failureCount     int
	successCount     int
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	lastFailure      time.Time
}

// NewCircuitBreaker creates a new circuit breaker.
// failureThreshold: number of failures before opening the circuit.
// successThreshold: number of successes in half-open state before closing.
// timeout: how long to wait before transitioning from open to half-open.
func NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
	}
}

// Middleware returns an HTTP middleware implementing the circuit breaker pattern.
func (cb *CircuitBreaker) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !cb.allowRequest() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "circuit breaker open",
				"message": "Service temporarily unavailable. Circuit breaker is open.",
				"state":   cb.GetState().String(),
			})
			return
		}

		wrapped := newResponseWriter(w)
		next.ServeHTTP(wrapped, r)

		// Record result
		if wrapped.statusCode >= 500 {
			cb.recordFailure()
		} else {
			cb.recordSuccess()
		}
	})
}

func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if timeout has elapsed
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.state = StateHalfOpen
			cb.successCount = 0
			log.Printf("[CircuitBreaker] State: OPEN → HALF-OPEN")
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.state = StateClosed
			cb.failureCount = 0
			log.Printf("[CircuitBreaker] State: HALF-OPEN → CLOSED")
		}
	} else {
		cb.failureCount = 0
	}
}

func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailure = time.Now()

	if cb.state == StateHalfOpen {
		cb.state = StateOpen
		log.Printf("[CircuitBreaker] State: HALF-OPEN → OPEN (failure in recovery)")
	} else if cb.failureCount >= cb.failureThreshold {
		cb.state = StateOpen
		log.Printf("[CircuitBreaker] State: CLOSED → OPEN (failures: %d)", cb.failureCount)
	}
}

// GetState returns the current circuit breaker state.
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
