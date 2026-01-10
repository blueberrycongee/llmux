// Package resilience provides high-availability patterns for the LLM gateway.
// It includes circuit breaker, rate limiting, and concurrency control.
//
// IMPORTANT: Circuit Breaker Status
// =================================
// The CircuitBreaker implementation in this file is a REFERENCE IMPLEMENTATION
// that is NOT currently integrated into the production router.
//
// The router (routers/base.go) uses a LiteLLM-style failure-rate based cooldown
// mechanism instead, which is more suitable for LLM APIs with bursty error patterns:
//   - Immediate cooldown on 429 (Rate Limit)
//   - Immediate cooldown on 401/404 (Non-retryable)
//   - Failure rate threshold (default 50%, min 5 requests)
//
// This traditional circuit breaker implementation may be integrated in the future
// if more sophisticated patterns (half-open state, gradual recovery) are needed.
// See: routers/base.go ReportFailure() for the currently active logic.
package resilience

import (
	"errors"
	"sync"
	"time"
)

// CircuitState represents the current state of a circuit breaker.
type CircuitState int

const (
	// StateClosed allows requests to pass through normally.
	StateClosed CircuitState = iota
	// StateOpen blocks all requests.
	StateOpen
	// StateHalfOpen allows limited requests to test recovery.
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreakerConfig contains configuration for a circuit breaker.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures before opening the circuit.
	FailureThreshold int
	// SuccessThreshold is the number of successes in half-open state to close.
	SuccessThreshold int
	// Timeout is how long the circuit stays open before transitioning to half-open.
	Timeout time.Duration
	// HalfOpenMaxRequests is the max requests allowed in half-open state.
	HalfOpenMaxRequests int
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             30 * time.Second,
		HalfOpenMaxRequests: 3,
	}
}

// CircuitBreaker implements the circuit breaker pattern.
// It prevents cascading failures by stopping requests to unhealthy services.
type CircuitBreaker struct {
	mu              sync.RWMutex
	name            string
	state           CircuitState
	failureCount    int
	successCount    int
	halfOpenCount   int
	lastFailureTime time.Time
	config          CircuitBreakerConfig
	onStateChange   func(name string, from, to CircuitState)
}

// NewCircuitBreaker creates a new circuit breaker with the given config.
func NewCircuitBreaker(name string, cfg CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		name:   name,
		state:  StateClosed,
		config: cfg,
	}
}

// OnStateChange sets a callback for state transitions.
func (cb *CircuitBreaker) OnStateChange(fn func(name string, from, to CircuitState)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

// Allow checks if a request should be allowed through.
// Returns true if the request can proceed, false if blocked.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if timeout has elapsed
		if time.Since(cb.lastFailureTime) >= cb.config.Timeout {
			cb.transitionTo(StateHalfOpen)
			cb.halfOpenCount = 1
			return true
		}
		return false

	case StateHalfOpen:
		// Allow limited requests in half-open state
		if cb.halfOpenCount < cb.config.HalfOpenMaxRequests {
			cb.halfOpenCount++
			return true
		}
		return false

	default:
		return false
	}
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		cb.failureCount = 0

	case StateHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.transitionTo(StateClosed)
			cb.failureCount = 0
			cb.successCount = 0
		}
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		cb.failureCount++
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.transitionTo(StateOpen)
		}

	case StateHalfOpen:
		// Any failure in half-open state reopens the circuit
		cb.transitionTo(StateOpen)
		cb.successCount = 0
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Name returns the circuit breaker name.
func (cb *CircuitBreaker) Name() string {
	return cb.name
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.transitionTo(StateClosed)
	cb.failureCount = 0
	cb.successCount = 0
	cb.halfOpenCount = 0
}

func (cb *CircuitBreaker) transitionTo(newState CircuitState) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState

	if cb.onStateChange != nil {
		// Call callback without holding lock
		go cb.onStateChange(cb.name, oldState, newState)
	}
}
