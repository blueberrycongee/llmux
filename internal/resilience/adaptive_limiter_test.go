package resilience

import (
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestAdaptiveLimiter_Basic(t *testing.T) {
	limiter := NewAdaptiveLimiter(10, 100)

	// Initially limit should be minLimit
	if limiter.Limit() != 10 {
		t.Errorf("expected initial limit 10, got %d", limiter.Limit())
	}

	// Try to acquire up to the limit
	for i := 0; i < 10; i++ {
		if !limiter.TryAcquire() {
			t.Errorf("failed to acquire permit %d", i)
		}
	}

	// Next one should fail
	if limiter.TryAcquire() {
		t.Error("should not have been able to acquire 11th permit")
	}

	// Release all with good RTT
	for i := 0; i < 10; i++ {
		limiter.Release(10 * time.Millisecond)
	}

	// After some samples, limit should increase
	// We need 10 samples to trigger updateLimit in our implementation
	for i := 0; i < 20; i++ {
		if limiter.TryAcquire() {
			limiter.Release(10 * time.Millisecond)
		}
	}

	if limiter.Limit() <= 10 {
		t.Errorf("limit should have increased, got %d", limiter.Limit())
	}
}

func TestAdaptiveLimiter_Stress(t *testing.T) {
	minLim, maxLim := 5.0, 50.0
	limiter := NewAdaptiveLimiter(minLim, maxLim)

	// Simulation parameters
	concurrentClients := 100
	iterations := 50

	var wg sync.WaitGroup
	wg.Add(concurrentClients)

	// Success and rejection counters
	var successCount, rejectCount int64
	var mu sync.Mutex

	baseLatency := 20 * time.Millisecond

	for i := 0; i < concurrentClients; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				start := time.Now()
				if limiter.TryAcquire() {
					// Simulate work with latency that increases with in-flight count
					// to simulate system saturation
					inflight := limiter.Inflight()
					extraLatency := time.Duration(inflight) * 2 * time.Millisecond
					time.Sleep(baseLatency + extraLatency + time.Duration(rand.Intn(5))*time.Millisecond)

					limiter.Release(time.Since(start))

					mu.Lock()
					successCount++
					mu.Unlock()
				} else {
					mu.Lock()
					rejectCount++
					mu.Unlock()
					time.Sleep(10 * time.Millisecond) // Wait before retry
				}
			}
		}()
	}

	wg.Wait()
	t.Logf("Success: %d, Rejected: %d, Final Limit: %d", successCount, rejectCount, limiter.Limit())

	if successCount == 0 {
		t.Error("No requests succeeded")
	}
}

func TestAdaptiveLimiter_LatencyIncrease(t *testing.T) {
	limiter := NewAdaptiveLimiter(10, 100)

	// 1. Train with low latency
	for i := 0; i < 50; i++ {
		if limiter.TryAcquire() {
			limiter.Release(10 * time.Millisecond)
		}
	}
	highLimit := limiter.Limit()
	t.Logf("Limit after low latency: %d", highLimit)

	// 2. Introduce high latency
	for i := 0; i < 50; i++ {
		if limiter.TryAcquire() {
			limiter.Release(500 * time.Millisecond)
		}
	}
	lowLimit := limiter.Limit()
	t.Logf("Limit after high latency: %d", lowLimit)

	if lowLimit >= highLimit {
		t.Errorf("Limit should have decreased: %d -> %d", highLimit, lowLimit)
	}
}
