// Package router provides public request routing and load balancing interfaces.
package router

import (
	"context"
)

// RoundRobinStore defines the interface for shared round-robin counters.
// Implementations can be in-memory (single-instance) or distributed (Redis).
type RoundRobinStore interface {
	// NextIndex returns the next round-robin index for a key, modulo the given size.
	NextIndex(ctx context.Context, key string, modulo int) (int, error)

	// Reset clears the counter for the key.
	Reset(ctx context.Context, key string) error

	// Close releases any resources held by the store.
	Close() error
}
