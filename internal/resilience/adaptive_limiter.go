package resilience

import (
	"context"
	"math"
	"sync"
	"time"
)

// AdaptiveLimiter implements an adaptive concurrency limit algorithm
// inspired by Netflix's concurrency-limits. It adjusts the maximum
// concurrency based on latency (RTT) variations.
type AdaptiveLimiter struct {
	mu sync.Mutex

	// Config
	minLimit float64
	maxLimit float64
	alpha    float64 // Smoothing factor for limit updates

	// State
	limit    float64
	minRTT   time.Duration
	inflight int

	// Window tracking
	lastReset      time.Time
	rttSamples     []time.Duration
	maxSamples     int
	resetInterval  time.Duration
}

// NewAdaptiveLimiter creates a new AdaptiveLimiter with default settings.
func NewAdaptiveLimiter(minLimit, maxLimit float64) *AdaptiveLimiter {
	if minLimit < 1 {
		minLimit = 1
	}
	if maxLimit < minLimit {
		maxLimit = minLimit
	}
	return &AdaptiveLimiter{
		minLimit:      minLimit,
		maxLimit:      maxLimit,
		limit:         minLimit,
		alpha:         0.1,
		maxSamples:    10,
		rttSamples:    make([]time.Duration, 0, 10),
		lastReset:     time.Now(),
		resetInterval: 5 * time.Minute,
	}
}

// TryAcquire attempts to acquire a permit.
// Returns true if acquired, false if at limit.
func (l *AdaptiveLimiter) TryAcquire() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if float64(l.inflight) >= math.Ceil(l.limit) {
		return false
	}

	l.inflight++
	return true
}

// Acquire blocks until a permit is available or context is canceled.
// Note: In highly loaded systems, blocking might not be desired.
// Most adaptive limiters use TryAcquire and return 429 immediately.
func (l *AdaptiveLimiter) Acquire(ctx context.Context) error {
	// For simplicity and following the gateway pattern, we'll mostly use TryAcquire.
	// But if we want to block:
	for {
		if l.TryAcquire() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			// Busy wait/poll. In a real system, use a cond var or channel.
			// But for adaptive limiters, usually we want to fail fast.
		}
	}
}

// Release releases a permit and updates the limit based on the observed RTT.
func (l *AdaptiveLimiter) Release(rtt time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.inflight--
	if l.inflight < 0 {
		l.inflight = 0
	}

	if rtt <= 0 {
		return
	}

	// Periodically reset minRTT to allow it to drift upwards if network conditions change
	if time.Since(l.lastReset) > l.resetInterval {
		l.minRTT = rtt
		l.lastReset = time.Now()
	} else if l.minRTT <= 0 || rtt < l.minRTT {
		l.minRTT = rtt
	}

	// Add sample
	l.rttSamples = append(l.rttSamples, rtt)
	if len(l.rttSamples) >= l.maxSamples {
		l.updateLimit()
		l.rttSamples = l.rttSamples[:0]
	}
}

// updateLimit implements the Gradient algorithm.
// NewLimit = CurrentLimit * (MinRTT / ActualRTT) + Buffer
func (l *AdaptiveLimiter) updateLimit() {
	if len(l.rttSamples) == 0 || l.minRTT <= 0 {
		return
	}

	// Calculate average RTT in this window
	var sum time.Duration
	for _, r := range l.rttSamples {
		sum += r
	}
	avgRTT := sum / time.Duration(len(l.rttSamples))

	// Gradient = minRTT / avgRTT
	gradient := float64(l.minRTT) / float64(avgRTT)

	// To avoid being too aggressive, we can use a buffer
	// Netflix uses sqrt(limit) as a buffer
	buffer := math.Sqrt(l.limit)
	newLimit := l.limit*gradient + buffer

	// Smoothing
	l.limit = l.limit*(1-l.alpha) + newLimit*l.alpha

	// Bound the limit
	if l.limit < l.minLimit {
		l.limit = l.minLimit
	}
	if l.limit > l.maxLimit {
		l.limit = l.maxLimit
	}
}

// Limit returns the current concurrency limit.
func (l *AdaptiveLimiter) Limit() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return int(math.Ceil(l.limit))
}

// Inflight returns the current number of in-flight requests.
func (l *AdaptiveLimiter) Inflight() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inflight
}
