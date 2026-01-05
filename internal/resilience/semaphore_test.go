package resilience

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewSemaphore(t *testing.T) {
	s := NewSemaphore(5)

	if s.Capacity() != 5 {
		t.Errorf("Capacity() = %v, want 5", s.Capacity())
	}
	if s.Current() != 0 {
		t.Errorf("Current() = %v, want 0", s.Current())
	}
	if s.Available() != 5 {
		t.Errorf("Available() = %v, want 5", s.Available())
	}
}

func TestNewSemaphore_InvalidCapacity(t *testing.T) {
	s := NewSemaphore(0)
	if s.Capacity() != 1 {
		t.Errorf("Capacity() = %v, want 1 for invalid input", s.Capacity())
	}

	s = NewSemaphore(-5)
	if s.Capacity() != 1 {
		t.Errorf("Capacity() = %v, want 1 for negative input", s.Capacity())
	}
}

func TestSemaphore_TryAcquire(t *testing.T) {
	s := NewSemaphore(2)

	// Should acquire up to capacity
	if !s.TryAcquire() {
		t.Error("TryAcquire() should return true")
	}
	if !s.TryAcquire() {
		t.Error("TryAcquire() should return true")
	}

	// Should fail when full
	if s.TryAcquire() {
		t.Error("TryAcquire() should return false when full")
	}

	if s.Current() != 2 {
		t.Errorf("Current() = %v, want 2", s.Current())
	}
	if s.Available() != 0 {
		t.Errorf("Available() = %v, want 0", s.Available())
	}
}

func TestSemaphore_Release(t *testing.T) {
	s := NewSemaphore(2)

	s.TryAcquire()
	s.TryAcquire()

	if s.Available() != 0 {
		t.Errorf("Available() = %v, want 0", s.Available())
	}

	s.Release()
	if s.Available() != 1 {
		t.Errorf("Available() = %v, want 1", s.Available())
	}

	s.Release()
	if s.Available() != 2 {
		t.Errorf("Available() = %v, want 2", s.Available())
	}

	// Extra release should be safe
	s.Release()
	if s.Available() != 2 {
		t.Errorf("Available() = %v, want 2 (no change)", s.Available())
	}
}

func TestSemaphore_Acquire(t *testing.T) {
	s := NewSemaphore(1)
	ctx := context.Background()

	// Should acquire immediately
	if err := s.Acquire(ctx); err != nil {
		t.Errorf("Acquire() error = %v", err)
	}

	// Start a goroutine that will release after delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.Release()
	}()

	// Should block and then succeed
	start := time.Now()
	if err := s.Acquire(ctx); err != nil {
		t.Errorf("Acquire() error = %v", err)
	}
	elapsed := time.Since(start)

	if elapsed < 40*time.Millisecond {
		t.Errorf("Acquire() should have blocked, elapsed = %v", elapsed)
	}
}

func TestSemaphore_AcquireContextCancel(t *testing.T) {
	s := NewSemaphore(1)
	s.TryAcquire() // Fill the semaphore

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := s.Acquire(ctx)
	elapsed := time.Since(start)

	if err != context.DeadlineExceeded {
		t.Errorf("Acquire() error = %v, want context.DeadlineExceeded", err)
	}

	if elapsed < 40*time.Millisecond {
		t.Errorf("Acquire() should have waited for timeout, elapsed = %v", elapsed)
	}
}

func TestSemaphore_AcquireContextCancelCleanup(t *testing.T) {
	s := NewSemaphore(1)
	s.TryAcquire() // Fill the semaphore

	ctx, cancel := context.WithCancel(context.Background())

	// Start acquire in goroutine
	done := make(chan error)
	go func() {
		done <- s.Acquire(ctx)
	}()

	// Cancel immediately
	time.Sleep(10 * time.Millisecond)
	cancel()

	err := <-done
	if err != context.Canceled {
		t.Errorf("Acquire() error = %v, want context.Canceled", err)
	}

	// Release and verify semaphore is still usable
	s.Release()
	if !s.TryAcquire() {
		t.Error("Semaphore should be usable after canceled acquire")
	}
}

func TestSemaphore_Concurrent(t *testing.T) {
	s := NewSemaphore(5)
	var wg sync.WaitGroup
	var maxConcurrent int
	var mu sync.Mutex

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			if err := s.Acquire(ctx); err != nil {
				return
			}
			defer s.Release()

			mu.Lock()
			current := s.Current()
			if current > maxConcurrent {
				maxConcurrent = current
			}
			mu.Unlock()

			time.Sleep(10 * time.Millisecond)
		}()
	}
	wg.Wait()

	if maxConcurrent > 5 {
		t.Errorf("maxConcurrent = %d, should not exceed capacity 5", maxConcurrent)
	}
}

func TestSemaphore_WaiterWakeup(t *testing.T) {
	s := NewSemaphore(1)
	s.TryAcquire()

	// Start multiple waiters
	var wg sync.WaitGroup
	results := make(chan int, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctx := context.Background()
			if err := s.Acquire(ctx); err != nil {
				return
			}
			results <- id
			time.Sleep(10 * time.Millisecond)
			s.Release()
		}(i)
	}

	// Release to wake up waiters one by one
	time.Sleep(20 * time.Millisecond)
	s.Release()

	wg.Wait()
	close(results)

	count := 0
	for range results {
		count++
	}

	if count != 3 {
		t.Errorf("expected 3 waiters to complete, got %d", count)
	}
}
