package verifier

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHealthMonitor_ClosedByDefault(t *testing.T) {
	h := NewHealthMonitor(3, 2, 100*time.Millisecond)
	assert.Equal(t, CircuitClosed, h.State())
	assert.True(t, h.AllowRequest())
	assert.True(t, h.IsHealthy())
}

func TestHealthMonitor_OpensAfterFailures(t *testing.T) {
	h := NewHealthMonitor(3, 2, 100*time.Millisecond)
	h.RecordFailure()
	h.RecordFailure()
	assert.Equal(t, CircuitClosed, h.State(), "should still be closed at threshold-1")
	h.RecordFailure()
	assert.Equal(t, CircuitOpen, h.State(), "should open at threshold")
	assert.False(t, h.AllowRequest(), "should block requests when open")
}

func TestHealthMonitor_HalfOpenAfterTimeout(t *testing.T) {
	h := NewHealthMonitor(2, 1, 50*time.Millisecond)
	h.RecordFailure()
	h.RecordFailure()
	assert.Equal(t, CircuitOpen, h.State())
	time.Sleep(60 * time.Millisecond)
	assert.True(t, h.AllowRequest(), "should allow probe after half-open timeout")
}

func TestHealthMonitor_ClosesAfterSuccesses(t *testing.T) {
	h := NewHealthMonitor(2, 2, 50*time.Millisecond)
	h.RecordFailure()
	h.RecordFailure()
	assert.Equal(t, CircuitOpen, h.State())

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)
	h.RecordSuccess()
	assert.Equal(t, CircuitHalfOpen, h.State())
	h.RecordSuccess()
	assert.Equal(t, CircuitClosed, h.State())
	assert.True(t, h.AllowRequest())
}

func TestHealthMonitor_HalfOpenFailureReopens(t *testing.T) {
	h := NewHealthMonitor(2, 2, 50*time.Millisecond)
	h.RecordFailure()
	h.RecordFailure()
	assert.Equal(t, CircuitOpen, h.State())

	time.Sleep(60 * time.Millisecond)
	h.RecordSuccess()
	assert.Equal(t, CircuitHalfOpen, h.State())
	h.RecordFailure()
	assert.Equal(t, CircuitOpen, h.State())
}

func TestHealthMonitor_Defaults(t *testing.T) {
	h := NewHealthMonitor(0, 0, 0)
	assert.Equal(t, 5, h.failureThreshold)
	assert.Equal(t, 3, h.recoveryThreshold)
	assert.Equal(t, 60*time.Second, h.halfOpenTimeout)
}
