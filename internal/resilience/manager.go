package resilience

import (
	"context"
	"sync"
	"time"
)

// Manager coordinates resilience components for multiple providers/deployments.
type Manager struct {
	mu              sync.RWMutex
	circuitBreakers map[string]*CircuitBreaker
	rateLimiters    map[string]*RateLimiter
	semaphores      map[string]*Semaphore
	cbConfig        CircuitBreakerConfig
	defaultRate     float64
	defaultBurst    int
}

// ManagerConfig contains configuration for the resilience manager.
type ManagerConfig struct {
	CircuitBreaker CircuitBreakerConfig
	DefaultRate    float64 // Default rate limit (requests/sec)
	DefaultBurst   int     // Default burst size
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		CircuitBreaker: DefaultCircuitBreakerConfig(),
		DefaultRate:    100, // 100 req/sec per provider
		DefaultBurst:   50,  // Allow bursts of 50
	}
}

// NewManager creates a new resilience manager.
func NewManager(cfg ManagerConfig) *Manager {
	return &Manager{
		circuitBreakers: make(map[string]*CircuitBreaker),
		rateLimiters:    make(map[string]*RateLimiter),
		semaphores:      make(map[string]*Semaphore),
		cbConfig:        cfg.CircuitBreaker,
		defaultRate:     cfg.DefaultRate,
		defaultBurst:    cfg.DefaultBurst,
	}
}

// GetCircuitBreaker returns or creates a circuit breaker for the given key.
func (m *Manager) GetCircuitBreaker(key string) *CircuitBreaker {
	m.mu.RLock()
	cb, ok := m.circuitBreakers[key]
	m.mu.RUnlock()

	if ok {
		return cb
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, ok = m.circuitBreakers[key]; ok {
		return cb
	}

	cb = NewCircuitBreaker(key, m.cbConfig)
	m.circuitBreakers[key] = cb
	return cb
}

// GetRateLimiter returns or creates a rate limiter for the given key.
func (m *Manager) GetRateLimiter(key string) *RateLimiter {
	m.mu.RLock()
	rl, ok := m.rateLimiters[key]
	m.mu.RUnlock()

	if ok {
		return rl
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if rl, ok = m.rateLimiters[key]; ok {
		return rl
	}

	rl = NewRateLimiter(m.defaultRate, m.defaultBurst)
	m.rateLimiters[key] = rl
	return rl
}

// GetSemaphore returns or creates a semaphore for the given key.
func (m *Manager) GetSemaphore(key string, capacity int) *Semaphore {
	m.mu.RLock()
	s, ok := m.semaphores[key]
	m.mu.RUnlock()

	if ok {
		return s
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok = m.semaphores[key]; ok {
		return s
	}

	s = NewSemaphore(capacity)
	m.semaphores[key] = s
	return s
}

// SetRateLimiter sets a custom rate limiter for a key.
func (m *Manager) SetRateLimiter(key string, rate float64, burst int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rateLimiters[key] = NewRateLimiter(rate, burst)
}

// SetSemaphore sets a custom semaphore for a key.
func (m *Manager) SetSemaphore(key string, capacity int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.semaphores[key] = NewSemaphore(capacity)
}

// CheckAndAcquire performs all resilience checks and acquires resources.
// Returns nil if allowed, or an error describing why the request was blocked.
func (m *Manager) CheckAndAcquire(ctx context.Context, key string, maxConcurrent int) error {
	// Check circuit breaker
	cb := m.GetCircuitBreaker(key)
	if !cb.Allow() {
		return ErrCircuitOpen
	}

	// Check rate limiter
	rl := m.GetRateLimiter(key)
	if !rl.Allow() {
		return ErrRateLimited
	}

	// Acquire semaphore (with context for timeout)
	if maxConcurrent > 0 {
		s := m.GetSemaphore(key, maxConcurrent)
		if err := s.Acquire(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Release releases acquired resources after a request completes.
func (m *Manager) Release(key string, maxConcurrent int) {
	if maxConcurrent > 0 {
		m.mu.RLock()
		s, ok := m.semaphores[key]
		m.mu.RUnlock()
		if ok {
			s.Release()
		}
	}
}

// RecordSuccess records a successful request.
func (m *Manager) RecordSuccess(key string) {
	cb := m.GetCircuitBreaker(key)
	cb.RecordSuccess()
}

// RecordFailure records a failed request.
func (m *Manager) RecordFailure(key string) {
	cb := m.GetCircuitBreaker(key)
	cb.RecordFailure()
}

// Stats returns current statistics for a key.
func (m *Manager) Stats(key string) ResilienceStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ResilienceStats{Key: key}

	if cb, ok := m.circuitBreakers[key]; ok {
		stats.CircuitState = cb.State().String()
	}

	if rl, ok := m.rateLimiters[key]; ok {
		stats.RateLimitTokens = rl.Tokens()
	}

	if s, ok := m.semaphores[key]; ok {
		stats.ConcurrentCurrent = s.Current()
		stats.ConcurrentCapacity = s.Capacity()
	}

	return stats
}

// ResilienceStats contains current resilience statistics.
type ResilienceStats struct {
	Key                string
	CircuitState       string
	RateLimitTokens    float64
	ConcurrentCurrent  int
	ConcurrentCapacity int
}

// ErrRateLimited is returned when rate limit is exceeded.
var ErrRateLimited = &RateLimitError{}

// RateLimitError indicates a rate limit was exceeded.
type RateLimitError struct{}

func (e *RateLimitError) Error() string {
	return "rate limit exceeded"
}

// RetryAfter returns a suggested retry delay.
func (e *RateLimitError) RetryAfter() time.Duration {
	return time.Second
}
