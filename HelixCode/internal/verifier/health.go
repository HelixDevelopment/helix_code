package verifier

import (
	"sync"
	"time"
)

// CircuitState represents the circuit breaker state.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // requests allowed
	CircuitHalfOpen                      // probing
	CircuitOpen                          // requests blocked
)

// HealthMonitor tracks verifier service health with a circuit breaker.
type HealthMonitor struct {
	mu                sync.RWMutex
	state             CircuitState
	failures          int
	successes         int
	lastFailureTime   time.Time
	failureThreshold  int
	recoveryThreshold int
	halfOpenTimeout   time.Duration
}

// NewHealthMonitor creates a health monitor with circuit breaker logic.
func NewHealthMonitor(failureThreshold, recoveryThreshold int, halfOpenTimeout time.Duration) *HealthMonitor {
	if failureThreshold <= 0 {
		failureThreshold = 5
	}
	if recoveryThreshold <= 0 {
		recoveryThreshold = 3
	}
	if halfOpenTimeout <= 0 {
		halfOpenTimeout = 60 * time.Second
	}
	return &HealthMonitor{
		state:             CircuitClosed,
		failureThreshold:  failureThreshold,
		recoveryThreshold: recoveryThreshold,
		halfOpenTimeout:   halfOpenTimeout,
	}
}

// AllowRequest returns true if requests should be allowed through.
func (h *HealthMonitor) AllowRequest() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	switch h.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(h.lastFailureTime) > h.halfOpenTimeout {
			return true // allow one probe
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	return true
}

// RecordSuccess marks a successful request.
func (h *HealthMonitor) RecordSuccess() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.successes++
	h.failures = 0

	if h.state == CircuitHalfOpen && h.successes >= h.recoveryThreshold {
		h.state = CircuitClosed
		h.successes = 0
	} else if h.state == CircuitOpen {
		h.state = CircuitHalfOpen
		h.successes = 1
	}
}

// RecordFailure marks a failed request.
func (h *HealthMonitor) RecordFailure() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.failures++
	h.successes = 0
	h.lastFailureTime = time.Now()

	if h.state == CircuitClosed && h.failures >= h.failureThreshold {
		h.state = CircuitOpen
	} else if h.state == CircuitHalfOpen {
		h.state = CircuitOpen
	}
}

// State returns the current circuit breaker state.
func (h *HealthMonitor) State() CircuitState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.state
}

// IsHealthy returns true if the circuit is closed.
func (h *HealthMonitor) IsHealthy() bool {
	return h.State() == CircuitClosed
}
