package resilience

import (
	"sync"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10, 5)

	if rl.Rate() != 10 {
		t.Errorf("Rate() = %v, want 10", rl.Rate())
	}
	if rl.Burst() != 5 {
		t.Errorf("Burst() = %v, want 5", rl.Burst())
	}
	// Should start with full bucket
	if rl.Tokens() < 4.9 {
		t.Errorf("Tokens() = %v, want ~5", rl.Tokens())
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(10, 5)

	// Should allow up to burst size
	for i := 0; i < 5; i++ {
		if !rl.Allow() {
			t.Errorf("Allow() should return true for request %d", i)
		}
	}

	// Should deny when bucket is empty
	if rl.Allow() {
		t.Error("Allow() should return false when bucket is empty")
	}
}

func TestRateLimiter_AllowN(t *testing.T) {
	rl := NewRateLimiter(10, 10)

	// Should allow 5 at once
	if !rl.AllowN(5) {
		t.Error("AllowN(5) should return true")
	}

	// Should allow another 5
	if !rl.AllowN(5) {
		t.Error("AllowN(5) should return true")
	}

	// Should deny when not enough tokens
	if rl.AllowN(1) {
		t.Error("AllowN(1) should return false when bucket is empty")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := NewRateLimiter(100, 5) // 100 tokens/sec

	// Drain the bucket
	for i := 0; i < 5; i++ {
		rl.Allow()
	}

	// Wait for refill (50ms = 5 tokens at 100/sec)
	time.Sleep(60 * time.Millisecond)

	// Should have refilled
	if !rl.Allow() {
		t.Error("Allow() should return true after refill")
	}
}

func TestRateLimiter_BurstCap(t *testing.T) {
	rl := NewRateLimiter(1000, 5) // High rate, low burst

	// Wait to accumulate tokens
	time.Sleep(50 * time.Millisecond)

	// Should be capped at burst size
	tokens := rl.Tokens()
	if tokens > 5.1 {
		t.Errorf("Tokens() = %v, should be capped at burst size 5", tokens)
	}
}

func TestRateLimiter_SetRate(t *testing.T) {
	rl := NewRateLimiter(10, 5)

	rl.SetRate(100)
	if rl.Rate() != 100 {
		t.Errorf("Rate() = %v, want 100", rl.Rate())
	}
}

func TestRateLimiter_SetBurst(t *testing.T) {
	rl := NewRateLimiter(10, 10)

	// Tokens should be capped when burst is reduced
	rl.SetBurst(3)
	if rl.Burst() != 3 {
		t.Errorf("Burst() = %v, want 3", rl.Burst())
	}
	if rl.Tokens() > 3.1 {
		t.Errorf("Tokens() = %v, should be capped at new burst 3", rl.Tokens())
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(1000, 100)

	var wg sync.WaitGroup
	var allowed, denied int64
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				mu.Lock()
				if rl.Allow() {
					allowed++
				} else {
					denied++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// Should have allowed at least burst size
	if allowed < 100 {
		t.Errorf("allowed = %d, want at least 100", allowed)
	}
}

func TestRateLimiter_ZeroRate(t *testing.T) {
	rl := NewRateLimiter(0, 5)

	// Should allow burst
	for i := 0; i < 5; i++ {
		if !rl.Allow() {
			t.Errorf("Allow() should return true for request %d", i)
		}
	}

	// Should deny after burst (no refill with zero rate)
	if rl.Allow() {
		t.Error("Allow() should return false with zero rate after burst")
	}
}
