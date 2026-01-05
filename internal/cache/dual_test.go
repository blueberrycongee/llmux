package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDualCache_LocalHit(t *testing.T) {
	local := NewMemoryCache(DefaultMemoryCacheConfig())

	dual := NewDualCache(local, nil, DefaultDualCacheConfig())
	defer func() { _ = dual.Close() }()

	ctx := context.Background()

	// Set in dual cache (goes to local only since no Redis)
	err := dual.Set(ctx, "key1", []byte("value1"), 0)
	require.NoError(t, err)

	// Get should hit local cache
	val, err := dual.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), val)

	stats := dual.DetailedStats()
	assert.Equal(t, int64(1), stats.LocalHits)
	assert.Equal(t, int64(0), stats.RedisHits)
}

func TestDualCache_Delete(t *testing.T) {
	local := NewMemoryCache(DefaultMemoryCacheConfig())

	dual := NewDualCache(local, nil, DefaultDualCacheConfig())
	defer func() { _ = dual.Close() }()

	ctx := context.Background()

	// Set and delete
	_ = dual.Set(ctx, "del-key", []byte("value"), 0)
	err := dual.Delete(ctx, "del-key")
	require.NoError(t, err)

	// Should be gone
	val, err := dual.Get(ctx, "del-key")
	require.NoError(t, err)
	assert.Nil(t, val)
}

func TestDualCache_SetLocalOnly(t *testing.T) {
	local := NewMemoryCache(DefaultMemoryCacheConfig())

	dual := NewDualCache(local, nil, DefaultDualCacheConfig())
	defer func() { _ = dual.Close() }()

	ctx := context.Background()

	err := dual.SetLocalOnly(ctx, "local-only", []byte("value"), 0)
	require.NoError(t, err)

	val, err := dual.Get(ctx, "local-only")
	require.NoError(t, err)
	assert.Equal(t, []byte("value"), val)
}

func TestDualCache_SetPipeline(t *testing.T) {
	local := NewMemoryCache(DefaultMemoryCacheConfig())

	dual := NewDualCache(local, nil, DefaultDualCacheConfig())
	defer func() { _ = dual.Close() }()

	ctx := context.Background()

	entries := []CacheEntry{
		{Key: "p1", Value: []byte("v1"), TTL: time.Minute},
		{Key: "p2", Value: []byte("v2"), TTL: time.Minute},
	}

	err := dual.SetPipeline(ctx, entries)
	require.NoError(t, err)

	// Verify
	val, _ := dual.Get(ctx, "p1")
	assert.Equal(t, []byte("v1"), val)
	val, _ = dual.Get(ctx, "p2")
	assert.Equal(t, []byte("v2"), val)
}

func TestDualCache_GetMulti(t *testing.T) {
	local := NewMemoryCache(DefaultMemoryCacheConfig())

	dual := NewDualCache(local, nil, DefaultDualCacheConfig())
	defer func() { _ = dual.Close() }()

	ctx := context.Background()

	// Set some values
	_ = dual.Set(ctx, "m1", []byte("v1"), 0)
	_ = dual.Set(ctx, "m2", []byte("v2"), 0)

	result, err := dual.GetMulti(ctx, []string{"m1", "m2", "m3"})
	require.NoError(t, err)

	assert.Equal(t, []byte("v1"), result["m1"])
	assert.Equal(t, []byte("v2"), result["m2"])
	_, exists := result["m3"]
	assert.False(t, exists)
}

func TestDualCache_Stats(t *testing.T) {
	local := NewMemoryCache(DefaultMemoryCacheConfig())

	dual := NewDualCache(local, nil, DefaultDualCacheConfig())
	defer func() { _ = dual.Close() }()

	ctx := context.Background()

	// Generate activity
	_ = dual.Set(ctx, "s1", []byte("v1"), 0)
	_, _ = dual.Get(ctx, "s1")      // Local hit
	_, _ = dual.Get(ctx, "missing") // Miss

	stats := dual.Stats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
}

func TestDualCache_DetailedStats(t *testing.T) {
	local := NewMemoryCache(DefaultMemoryCacheConfig())

	dual := NewDualCache(local, nil, DefaultDualCacheConfig())
	defer func() { _ = dual.Close() }()

	ctx := context.Background()

	_ = dual.Set(ctx, "d1", []byte("v1"), 0)
	_, _ = dual.Get(ctx, "d1")

	stats := dual.DetailedStats()
	assert.Equal(t, int64(1), stats.LocalHits)
	assert.Equal(t, int64(0), stats.RedisHits)
	assert.Equal(t, int64(0), stats.Misses)
}

func TestDualCache_Flush(t *testing.T) {
	local := NewMemoryCache(DefaultMemoryCacheConfig())

	dual := NewDualCache(local, nil, DefaultDualCacheConfig())
	defer func() { _ = dual.Close() }()

	ctx := context.Background()

	_ = dual.Set(ctx, "f1", []byte("v1"), 0)
	dual.Flush()

	val, _ := dual.Get(ctx, "f1")
	assert.Nil(t, val)
}

func TestDualCache_Ping(t *testing.T) {
	local := NewMemoryCache(DefaultMemoryCacheConfig())

	dual := NewDualCache(local, nil, DefaultDualCacheConfig())
	defer func() { _ = dual.Close() }()

	ctx := context.Background()
	err := dual.Ping(ctx)
	assert.NoError(t, err)
}

func TestDualCache_ThrottleMap(t *testing.T) {
	cfg := DualCacheConfig{
		LocalTTL:           time.Minute,
		RedisTTL:           time.Hour,
		BatchThrottleTime:  50 * time.Millisecond,
		MaxThrottleEntries: 5,
	}

	local := NewMemoryCache(DefaultMemoryCacheConfig())

	dual := NewDualCache(local, nil, cfg)
	defer func() { _ = dual.Close() }()

	// Simulate throttle map entries
	now := time.Now()
	dual.mu.Lock()
	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		dual.lastRedisAccessTime[key] = now.Add(-time.Minute) // Old entries
	}
	dual.mu.Unlock()

	// Cleanup should remove old entries
	dual.mu.Lock()
	dual.cleanupThrottleMap(now)
	dual.mu.Unlock()

	dual.mu.RLock()
	assert.Less(t, len(dual.lastRedisAccessTime), 10)
	dual.mu.RUnlock()
}

func BenchmarkDualCache_Get(b *testing.B) {
	local := NewMemoryCache(DefaultMemoryCacheConfig())

	dual := NewDualCache(local, nil, DefaultDualCacheConfig())
	defer func() { _ = dual.Close() }()

	ctx := context.Background()
	_ = dual.Set(ctx, "bench-key", []byte("benchmark value"), 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dual.Get(ctx, "bench-key")
	}
}

func BenchmarkDualCache_Set(b *testing.B) {
	local := NewMemoryCache(DefaultMemoryCacheConfig())

	dual := NewDualCache(local, nil, DefaultDualCacheConfig())
	defer func() { _ = dual.Close() }()

	ctx := context.Background()
	value := []byte("benchmark value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dual.Set(ctx, "bench-key", value, 0)
	}
}
