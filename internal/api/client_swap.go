package api

import (
	"sync/atomic"

	llmux "github.com/blueberrycongee/llmux"
)

type clientSwap[T interface{ Close() error }] struct {
	current atomic.Pointer[clientRef[T]]
}

type clientRef[T interface{ Close() error }] struct {
	client  T
	refs    atomic.Int64
	closing atomic.Bool
	closed  atomic.Bool
}

func newClientSwap[T interface{ Close() error }](client T) *clientSwap[T] {
	swap := &clientSwap[T]{}
	swap.current.Store(&clientRef[T]{client: client})
	return swap
}

func (s *clientSwap[T]) acquire() (T, func()) {
	ref := s.current.Load()
	if ref == nil {
		var zero T
		return zero, func() {}
	}

	ref.refs.Add(1)

	release := func() {
		if ref.refs.Add(-1) == 0 && ref.closing.Load() {
			ref.closeOnce()
		}
	}

	return ref.client, release
}

func (s *clientSwap[T]) swap(next T) {
	nextRef := &clientRef[T]{client: next}
	prev := s.current.Swap(nextRef)
	if prev == nil {
		return
	}

	prev.closing.Store(true)
	if prev.refs.Load() == 0 {
		prev.closeOnce()
	}
}

func (s *clientSwap[T]) closeCurrent() {
	ref := s.current.Load()
	if ref == nil {
		return
	}

	ref.closing.Store(true)
	if ref.refs.Load() == 0 {
		ref.closeOnce()
	}
}

func (s *clientSwap[T]) currentClient() T {
	ref := s.current.Load()
	if ref == nil {
		var zero T
		return zero
	}
	return ref.client
}

func (r *clientRef[T]) closeOnce() {
	if r.closed.CompareAndSwap(false, true) {
		_ = r.client.Close()
	}
}

// ClientSwapper manages a hot-swappable llmux.Client for gateway handlers.
type ClientSwapper struct {
	swapper *clientSwap[*llmux.Client]
}

// NewClientSwapper creates a new swapper seeded with the initial client.
func NewClientSwapper(client *llmux.Client) *ClientSwapper {
	return &ClientSwapper{swapper: newClientSwap[*llmux.Client](client)}
}

// Acquire returns the current client and a release function.
// Call release when the request is done to allow safe client shutdown.
func (s *ClientSwapper) Acquire() (*llmux.Client, func()) {
	return s.swapper.acquire()
}

// Swap atomically replaces the current client with the next one.
// The old client is closed once in-flight users release it.
func (s *ClientSwapper) Swap(next *llmux.Client) {
	s.swapper.swap(next)
}

// Close marks the current client as closing and closes it when idle.
func (s *ClientSwapper) Close() {
	s.swapper.closeCurrent()
}

// Current returns the current client without affecting its lifetime.
func (s *ClientSwapper) Current() *llmux.Client {
	return s.swapper.currentClient()
}
