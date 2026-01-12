package governance

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// IdempotencyStore prevents duplicate accounting writes.
type IdempotencyStore interface {
	PutIfAbsent(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// MemoryIdempotencyStore keeps idempotency keys in memory.
type MemoryIdempotencyStore struct {
	mu      sync.Mutex
	entries map[string]time.Time
}

// NewMemoryIdempotencyStore creates an in-memory idempotency store.
func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{
		entries: make(map[string]time.Time),
	}
}

// PutIfAbsent records the key if missing or expired.
func (s *MemoryIdempotencyStore) PutIfAbsent(_ context.Context, key string, ttl time.Duration) (bool, error) {
	if key == "" {
		return true, nil
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	if expiresAt, ok := s.entries[key]; ok && expiresAt.After(now) {
		return false, nil
	}
	if ttl <= 0 {
		delete(s.entries, key)
		return true, nil
	}
	s.entries[key] = now.Add(ttl)
	return true, nil
}

// RedisIdempotencyStore stores idempotency keys in Redis.
type RedisIdempotencyStore struct {
	client redis.UniversalClient
	prefix string
}

// NewRedisIdempotencyStore creates a Redis-backed idempotency store.
func NewRedisIdempotencyStore(client redis.UniversalClient, prefix string) *RedisIdempotencyStore {
	return &RedisIdempotencyStore{
		client: client,
		prefix: prefix,
	}
}

// PutIfAbsent records the key in Redis if missing.
func (s *RedisIdempotencyStore) PutIfAbsent(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if key == "" {
		return true, nil
	}
	if ttl <= 0 {
		return true, nil
	}
	ok, err := s.client.SetNX(ctx, s.prefix+key, "1", ttl).Result()
	return ok, err
}
