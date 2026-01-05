package resilience

import (
	"sync"
	"testing"
	"time"
)

func TestCircuitBreakerState_String(t *testing.T) {
	tests := []struct {
		state CircuitState
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("CircuitState.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewCircuitBreaker(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", cfg)

	if cb.Name() != "test" {
		t.Errorf("Name() = %v, want test", cb.Name())
	}
	if cb.State() != StateClosed {
		t.Errorf("State() = %v, want StateClosed", cb.State())
	}
}

func TestCircuitBreaker_ClosedState(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             100 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker("test", cfg)

	// Should allow requests in closed state
	for i := 0; i < 10; i++ {
		if !cb.Allow() {
			t.Error("should allow requests in closed state")
		}
		cb.RecordSuccess()
	}

	if cb.State() != StateClosed {
		t.Errorf("State() = %v, want StateClosed", cb.State())
	}
}

func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold:    3,
		SuccessThreshold:    2,
		Timeout:             100 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker("test", cfg)

	// Record failures up to threshold
	for i := 0; i < 3; i++ {
		cb.Allow()
		cb.RecordFailure()
	}

	if cb.State() != StateOpen {
		t.Errorf("State() = %v, want StateOpen", cb.State())
	}

	// Should block requests when open
	if cb.Allow() {
		t.Error("should block requests when circuit is open")
	}
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    2,
		Timeout:             50 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker("test", cfg)

	// Open the circuit
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Fatalf("State() = %v, want StateOpen", cb.State())
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Should transition to half-open on next Allow
	if !cb.Allow() {
		t.Error("should allow request after timeout (half-open)")
	}

	if cb.State() != StateHalfOpen {
		t.Errorf("State() = %v, want StateHalfOpen", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenToClose(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    2,
		Timeout:             50 * time.Millisecond,
		HalfOpenMaxRequests: 3,
	}
	cb := NewCircuitBreaker("test", cfg)

	// Open the circuit
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	// Wait for timeout and transition to half-open
	time.Sleep(60 * time.Millisecond)
	cb.Allow()

	// Record successes to close
	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.State() != StateClosed {
		t.Errorf("State() = %v, want StateClosed", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    2,
		Timeout:             50 * time.Millisecond,
		HalfOpenMaxRequests: 3,
	}
	cb := NewCircuitBreaker("test", cfg)

	// Open the circuit
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	// Wait for timeout and transition to half-open
	time.Sleep(60 * time.Millisecond)
	cb.Allow()

	if cb.State() != StateHalfOpen {
		t.Fatalf("State() = %v, want StateHalfOpen", cb.State())
	}

	// Any failure in half-open reopens
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Errorf("State() = %v, want StateOpen", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenLimitsRequests(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    2,
		Timeout:             50 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker("test", cfg)

	// Open the circuit
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// First request transitions to half-open
	if !cb.Allow() {
		t.Error("should allow first request in half-open")
	}
	// Second request allowed
	if !cb.Allow() {
		t.Error("should allow second request in half-open")
	}
	// Third request blocked (max is 2)
	if cb.Allow() {
		t.Error("should block requests beyond HalfOpenMaxRequests")
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    2,
		Timeout:             1 * time.Hour,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker("test", cfg)

	// Open the circuit
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Fatalf("State() = %v, want StateOpen", cb.State())
	}

	// Reset
	cb.Reset()

	if cb.State() != StateClosed {
		t.Errorf("State() = %v, want StateClosed after reset", cb.State())
	}

	// Should allow requests again
	if !cb.Allow() {
		t.Error("should allow requests after reset")
	}
}

func TestCircuitBreaker_OnStateChange(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold:    2,
		SuccessThreshold:    1,
		Timeout:             50 * time.Millisecond,
		HalfOpenMaxRequests: 2,
	}
	cb := NewCircuitBreaker("test", cfg)

	var mu sync.Mutex
	var transitions []struct{ from, to CircuitState }

	cb.OnStateChange(func(name string, from, to CircuitState) {
		mu.Lock()
		transitions = append(transitions, struct{ from, to CircuitState }{from, to})
		mu.Unlock()
	})

	// Trigger closed -> open
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	// Wait for callback
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if len(transitions) != 1 {
		t.Errorf("expected 1 transition, got %d", len(transitions))
	}
	if len(transitions) > 0 && (transitions[0].from != StateClosed || transitions[0].to != StateOpen) {
		t.Errorf("expected closed->open, got %v->%v", transitions[0].from, transitions[0].to)
	}
	mu.Unlock()
}

func TestCircuitBreaker_Concurrent(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold:    100,
		SuccessThreshold:    10,
		Timeout:             1 * time.Second,
		HalfOpenMaxRequests: 10,
	}
	cb := NewCircuitBreaker("test", cfg)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if cb.Allow() {
					if j%2 == 0 {
						cb.RecordSuccess()
					} else {
						cb.RecordFailure()
					}
				}
			}
		}()
	}
	wg.Wait()

	// Just verify no panics occurred
	_ = cb.State()
}
