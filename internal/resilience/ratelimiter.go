package resilience

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
// It allows bursting while maintaining a long-term rate limit.
type RateLimiter struct {
	mu           sync.Mutex
	rate         float64   // tokens per second
	burst        int       // maximum bucket size
	tokens       float64   // current tokens
	lastRefill   time.Time // last refill time
}

// NewRateLimiter creates a new rate limiter.
// rate: requests per second allowed
// burst: maximum burst size (bucket capacity)
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	return &RateLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst), // Start with full bucket
		lastRefill: time.Now(),
	}
}

// Allow checks if a request should be allowed.
// Returns true if allowed, false if rate limited.
func (rl *RateLimiter) Allow() bool {
	return rl.AllowN(1)
}

// AllowN checks if n requests should be allowed.
func (rl *RateLimiter) AllowN(n int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()

	if rl.tokens >= float64(n) {
		rl.tokens -= float64(n)
		return true
	}
	return false
}

// refill adds tokens based on elapsed time.
func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.lastRefill = now

	// Add tokens based on elapsed time
	rl.tokens += elapsed * rl.rate

	// Cap at burst size
	if rl.tokens > float64(rl.burst) {
		rl.tokens = float64(rl.burst)
	}
}

// Tokens returns the current number of available tokens.
func (rl *RateLimiter) Tokens() float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.refill()
	return rl.tokens
}

// Rate returns the rate limit (tokens per second).
func (rl *RateLimiter) Rate() float64 {
	return rl.rate
}

// Burst returns the burst size.
func (rl *RateLimiter) Burst() int {
	return rl.burst
}

// SetRate updates the rate limit.
func (rl *RateLimiter) SetRate(rate float64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.rate = rate
}

// SetBurst updates the burst size.
func (rl *RateLimiter) SetBurst(burst int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.burst = burst
	if rl.tokens > float64(burst) {
		rl.tokens = float64(burst)
	}
}
