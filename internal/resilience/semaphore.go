package resilience

import (
	"context"
	"errors"
	"sync"
)

// ErrSemaphoreFull is returned when the semaphore is at capacity.
var ErrSemaphoreFull = errors.New("semaphore is full")

// Semaphore implements a counting semaphore for concurrency control.
// It limits the number of concurrent operations.
type Semaphore struct {
	mu       sync.Mutex
	capacity int
	current  int
	waiters  []chan struct{}
}

// NewSemaphore creates a new semaphore with the given capacity.
func NewSemaphore(capacity int) *Semaphore {
	if capacity <= 0 {
		capacity = 1
	}
	return &Semaphore{
		capacity: capacity,
		waiters:  make([]chan struct{}, 0),
	}
}

// TryAcquire attempts to acquire a permit without blocking.
// Returns true if acquired, false if semaphore is full.
func (s *Semaphore) TryAcquire() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current < s.capacity {
		s.current++
		return true
	}
	return false
}

// Acquire acquires a permit, blocking until one is available or context is cancelled.
func (s *Semaphore) Acquire(ctx context.Context) error {
	// Try non-blocking first
	if s.TryAcquire() {
		return nil
	}

	// Create a waiter channel
	s.mu.Lock()
	waiter := make(chan struct{})
	s.waiters = append(s.waiters, waiter)
	s.mu.Unlock()

	// Wait for permit or context cancellation
	select {
	case <-waiter:
		return nil
	case <-ctx.Done():
		// Remove ourselves from waiters
		s.mu.Lock()
		for i, w := range s.waiters {
			if w == waiter {
				s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
		return ctx.Err()
	}
}

// Release releases a permit, potentially waking a waiter.
func (s *Semaphore) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current <= 0 {
		return // Nothing to release
	}

	// If there are waiters, wake one up
	if len(s.waiters) > 0 {
		waiter := s.waiters[0]
		s.waiters = s.waiters[1:]
		close(waiter) // Signal the waiter
		// Don't decrement current since we're transferring the permit
		return
	}

	s.current--
}

// Current returns the current number of acquired permits.
func (s *Semaphore) Current() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

// Capacity returns the semaphore capacity.
func (s *Semaphore) Capacity() int {
	return s.capacity
}

// Available returns the number of available permits.
func (s *Semaphore) Available() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.capacity - s.current
}
