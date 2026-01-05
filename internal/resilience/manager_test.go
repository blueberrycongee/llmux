package resilience

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	cfg := DefaultManagerConfig()
	m := NewManager(cfg)

	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
}

func TestManager_GetCircuitBreaker(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	cb1 := m.GetCircuitBreaker("provider-a")
	cb2 := m.GetCircuitBreaker("provider-a")
	cb3 := m.GetCircuitBreaker("provider-b")

	// Same key should return same instance
	if cb1 != cb2 {
		t.Error("GetCircuitBreaker should return same instance for same key")
	}

	// Different key should return different instance
	if cb1 == cb3 {
		t.Error("GetCircuitBreaker should return different instance for different key")
	}
}

func TestManager_GetRateLimiter(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	rl1 := m.GetRateLimiter("provider-a")
	rl2 := m.GetRateLimiter("provider-a")
	rl3 := m.GetRateLimiter("provider-b")

	if rl1 != rl2 {
		t.Error("GetRateLimiter should return same instance for same key")
	}

	if rl1 == rl3 {
		t.Error("GetRateLimiter should return different instance for different key")
	}
}

func TestManager_GetSemaphore(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	s1 := m.GetSemaphore("provider-a", 10)
	s2 := m.GetSemaphore("provider-a", 10)
	s3 := m.GetSemaphore("provider-b", 5)

	if s1 != s2 {
		t.Error("GetSemaphore should return same instance for same key")
	}

	if s1 == s3 {
		t.Error("GetSemaphore should return different instance for different key")
	}

	if s1.Capacity() != 10 {
		t.Errorf("Capacity() = %d, want 10", s1.Capacity())
	}
}

func TestManager_SetRateLimiter(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	m.SetRateLimiter("custom", 500, 100)
	rl := m.GetRateLimiter("custom")

	if rl.Rate() != 500 {
		t.Errorf("Rate() = %v, want 500", rl.Rate())
	}
	if rl.Burst() != 100 {
		t.Errorf("Burst() = %v, want 100", rl.Burst())
	}
}

func TestManager_CheckAndAcquire_Success(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	ctx := context.Background()

	err := m.CheckAndAcquire(ctx, "test", 10)
	if err != nil {
		t.Errorf("CheckAndAcquire() error = %v", err)
	}

	// Release
	m.Release("test", 10)
}

func TestManager_CheckAndAcquire_CircuitOpen(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.CircuitBreaker.FailureThreshold = 2
	m := NewManager(cfg)
	ctx := context.Background()

	// Open the circuit
	cb := m.GetCircuitBreaker("test")
	cb.RecordFailure()
	cb.RecordFailure()

	err := m.CheckAndAcquire(ctx, "test", 10)
	if err != ErrCircuitOpen {
		t.Errorf("CheckAndAcquire() error = %v, want ErrCircuitOpen", err)
	}
}

func TestManager_CheckAndAcquire_RateLimited(t *testing.T) {
	cfg := DefaultManagerConfig()
	m := NewManager(cfg)
	ctx := context.Background()

	// Set very low rate limit
	m.SetRateLimiter("test", 0, 1)

	// First request should succeed
	err := m.CheckAndAcquire(ctx, "test", 0)
	if err != nil {
		t.Errorf("First CheckAndAcquire() error = %v", err)
	}

	// Second request should be rate limited
	err = m.CheckAndAcquire(ctx, "test", 0)
	if err != ErrRateLimited {
		t.Errorf("Second CheckAndAcquire() error = %v, want ErrRateLimited", err)
	}
}

func TestManager_CheckAndAcquire_SemaphoreFull(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	// Set capacity to 1
	m.SetSemaphore("test", 1)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// First acquire
	err := m.CheckAndAcquire(ctx, "test", 1)
	if err != nil {
		t.Errorf("First CheckAndAcquire() error = %v", err)
	}

	// Second acquire should timeout
	err = m.CheckAndAcquire(ctx, "test", 1)
	if err != context.DeadlineExceeded {
		t.Errorf("Second CheckAndAcquire() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestManager_RecordSuccessFailure(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.CircuitBreaker.FailureThreshold = 3
	m := NewManager(cfg)

	// Record successes
	m.RecordSuccess("test")
	m.RecordSuccess("test")

	cb := m.GetCircuitBreaker("test")
	if cb.State() != StateClosed {
		t.Errorf("State() = %v, want StateClosed", cb.State())
	}

	// Record failures to open circuit
	m.RecordFailure("test")
	m.RecordFailure("test")
	m.RecordFailure("test")

	if cb.State() != StateOpen {
		t.Errorf("State() = %v, want StateOpen", cb.State())
	}
}

func TestManager_Stats(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	// Initialize components
	m.GetCircuitBreaker("test")
	m.GetRateLimiter("test")
	m.GetSemaphore("test", 10)

	stats := m.Stats("test")

	if stats.Key != "test" {
		t.Errorf("Key = %v, want test", stats.Key)
	}
	if stats.CircuitState != "closed" {
		t.Errorf("CircuitState = %v, want closed", stats.CircuitState)
	}
	if stats.ConcurrentCapacity != 10 {
		t.Errorf("ConcurrentCapacity = %v, want 10", stats.ConcurrentCapacity)
	}
}

func TestRateLimitError(t *testing.T) {
	err := &RateLimitError{}

	if err.Error() != "rate limit exceeded" {
		t.Errorf("Error() = %v, want 'rate limit exceeded'", err.Error())
	}

	if err.RetryAfter() != time.Second {
		t.Errorf("RetryAfter() = %v, want 1s", err.RetryAfter())
	}
}
