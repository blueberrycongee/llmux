package routers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const roundRobinKeyPrefix = "llmux:rr:"
const roundRobinKeyTTL = 24 * time.Hour

// MemoryRoundRobinStore keeps round-robin counters in memory.
type MemoryRoundRobinStore struct {
	mu       sync.Mutex
	counters map[string]uint64
}

// NewMemoryRoundRobinStore creates a new in-memory round-robin store.
func NewMemoryRoundRobinStore() *MemoryRoundRobinStore {
	return &MemoryRoundRobinStore{
		counters: make(map[string]uint64),
	}
}

// NextIndex returns the next round-robin index for the key.
func (m *MemoryRoundRobinStore) NextIndex(_ context.Context, key string, modulo int) (int, error) {
	if modulo <= 0 {
		return 0, fmt.Errorf("modulo must be positive")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	next := m.counters[key]
	m.counters[key] = next + 1
	// #nosec G115 -- modulo bounds the value; result fits in int.
	return int(next % uint64(modulo)), nil
}

// Reset clears the counter for the key.
func (m *MemoryRoundRobinStore) Reset(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.counters, key)
	return nil
}

// Close releases resources (no-op for memory store).
func (m *MemoryRoundRobinStore) Close() error {
	return nil
}

// RedisRoundRobinStore keeps round-robin counters in Redis.
type RedisRoundRobinStore struct {
	client redis.UniversalClient
}

// NewRedisRoundRobinStore creates a new Redis-backed round-robin store.
func NewRedisRoundRobinStore(client redis.UniversalClient) *RedisRoundRobinStore {
	return &RedisRoundRobinStore{client: client}
}

// NextIndex returns the next round-robin index for the key.
func (r *RedisRoundRobinStore) NextIndex(ctx context.Context, key string, modulo int) (int, error) {
	if modulo <= 0 {
		return 0, fmt.Errorf("modulo must be positive")
	}
	if r == nil || r.client == nil {
		return 0, fmt.Errorf("redis client is nil")
	}
	fullKey := roundRobinKeyPrefix + key
	pipe := r.client.Pipeline()
	incr := pipe.Incr(ctx, fullKey)
	pipe.Expire(ctx, fullKey, roundRobinKeyTTL)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	idx := (incr.Val() - 1) % int64(modulo)
	return int(idx), nil
}

// Reset clears the counter for the key.
func (r *RedisRoundRobinStore) Reset(ctx context.Context, key string) error {
	if r == nil || r.client == nil {
		return nil
	}
	fullKey := roundRobinKeyPrefix + key
	return r.client.Del(ctx, fullKey).Err()
}

// Close releases resources (no-op, client is managed externally).
func (r *RedisRoundRobinStore) Close() error {
	return nil
}
