package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCache_BasicOperations(t *testing.T) {
	cfg := MemoryCacheConfig{
		MaxSize:         100,
		DefaultTTL:      time.Minute,
		MaxItemSize:     1024 * 1024,
		CleanupInterval: time.Hour, // Disable auto cleanup for tests
	}
	cache := NewMemoryCache(cfg)
	defer cache.Close()

	ctx := context.Background()

	t.Run("set and get", func(t *testing.T) {
		err := cache.Set(ctx, "key1", []byte("value1"), 0)
		require.NoError(t, err)

		val, err := cache.Get(ctx, "key1")
		require.NoError(t, err)
		assert.Equal(t, []byte("value1"), val)
	})

	t.Run("get non-existent key", func(t *testing.T) {
		val, err := cache.Get(ctx, "non-existent")
		require.NoError(t, err)
		assert.Nil(t, val)
	})

	t.Run("delete", func(t *testing.T) {
		err := cache.Set(ctx, "key2", []byte("value2"), 0)
		require.NoError(t, err)

		err = cache.Delete(ctx, "key2")
		require.NoError(t, err)

		val, err := cache.Get(ctx, "key2")
		require.NoError(t, err)
		assert.Nil(t, val)
	})

	t.Run("overwrite", func(t *testing.T) {
		err := cache.Set(ctx, "key3", []byte("value3"), 0)
		require.NoError(t, err)

		err = cache.Set(ctx, "key3", []byte("value3-updated"), 0)
		require.NoError(t, err)

		val, err := cache.Get(ctx, "key3")
		require.NoError(t, err)
		assert.Equal(t, []byte("value3-updated"), val)
	})
}

func TestMemoryCache_TTL(t *testing.T) {
	cfg := MemoryCacheConfig{
		MaxSize:         100,
		DefaultTTL:      100 * time.Millisecond,
		MaxItemSize:     1024 * 1024,
		CleanupInterval: time.Hour,
	}
	cache := NewMemoryCache(cfg)
	defer cache.Close()

	ctx := context.Background()

	t.Run("default TTL expiration", func(t *testing.T) {
		err := cache.Set(ctx, "ttl-key", []byte("value"), 0)
		require.NoError(t, err)

		// Should exist immediately
		val, err := cache.Get(ctx, "ttl-key")
		require.NoError(t, err)
		assert.NotNil(t, val)

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)

		// Should be expired
		val, err = cache.Get(ctx, "ttl-key")
		require.NoError(t, err)
		assert.Nil(t, val)
	})

	t.Run("custom TTL", func(t *testing.T) {
		err := cache.Set(ctx, "custom-ttl", []byte("value"), 50*time.Millisecond)
		require.NoError(t, err)

		// Should exist immediately
		val, err := cache.Get(ctx, "custom-ttl")
		require.NoError(t, err)
		assert.NotNil(t, val)

		// Wait for custom TTL
		time.Sleep(60 * time.Millisecond)

		// Should be expired
		val, err = cache.Get(ctx, "custom-ttl")
		require.NoError(t, err)
		assert.Nil(t, val)
	})
}

func TestMemoryCache_Eviction(t *testing.T) {
	cfg := MemoryCacheConfig{
		MaxSize:         5,
		DefaultTTL:      time.Hour,
		MaxItemSize:     1024 * 1024,
		CleanupInterval: time.Hour,
	}
	cache := NewMemoryCache(cfg)
	defer cache.Close()

	ctx := context.Background()

	t.Run("evict when full", func(t *testing.T) {
		// Fill the cache
		for i := 0; i < 10; i++ {
			key := string(rune('a' + i))
			err := cache.Set(ctx, key, []byte("value"), 0)
			require.NoError(t, err)
		}

		// Cache should not exceed max size
		assert.LessOrEqual(t, cache.Len(), cfg.MaxSize)
	})
}

func TestMemoryCache_MaxItemSize(t *testing.T) {
	cfg := MemoryCacheConfig{
		MaxSize:         100,
		DefaultTTL:      time.Hour,
		MaxItemSize:     100, // 100 bytes max
		CleanupInterval: time.Hour,
	}
	cache := NewMemoryCache(cfg)
	defer cache.Close()

	ctx := context.Background()

	t.Run("reject oversized item", func(t *testing.T) {
		largeValue := make([]byte, 200)
		err := cache.Set(ctx, "large", largeValue, 0)
		require.NoError(t, err) // Should not error, just skip

		// Should not be cached
		val, err := cache.Get(ctx, "large")
		require.NoError(t, err)
		assert.Nil(t, val)
	})

	t.Run("accept small item", func(t *testing.T) {
		smallValue := make([]byte, 50)
		err := cache.Set(ctx, "small", smallValue, 0)
		require.NoError(t, err)

		val, err := cache.Get(ctx, "small")
		require.NoError(t, err)
		assert.NotNil(t, val)
	})
}

func TestMemoryCache_GetMulti(t *testing.T) {
	cfg := DefaultMemoryCacheConfig()
	cache := NewMemoryCache(cfg)
	defer cache.Close()

	ctx := context.Background()

	// Set some values
	_ = cache.Set(ctx, "k1", []byte("v1"), 0)
	_ = cache.Set(ctx, "k2", []byte("v2"), 0)
	_ = cache.Set(ctx, "k3", []byte("v3"), 0)

	t.Run("get multiple keys", func(t *testing.T) {
		result, err := cache.GetMulti(ctx, []string{"k1", "k2", "k4"})
		require.NoError(t, err)

		assert.Equal(t, []byte("v1"), result["k1"])
		assert.Equal(t, []byte("v2"), result["k2"])
		_, exists := result["k4"]
		assert.False(t, exists)
	})

	t.Run("empty keys", func(t *testing.T) {
		result, err := cache.GetMulti(ctx, []string{})
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestMemoryCache_SetPipeline(t *testing.T) {
	cfg := DefaultMemoryCacheConfig()
	cache := NewMemoryCache(cfg)
	defer cache.Close()

	ctx := context.Background()

	entries := []CacheEntry{
		{Key: "p1", Value: []byte("v1"), TTL: time.Minute},
		{Key: "p2", Value: []byte("v2"), TTL: time.Minute},
		{Key: "p3", Value: []byte("v3"), TTL: time.Minute},
	}

	err := cache.SetPipeline(ctx, entries)
	require.NoError(t, err)

	// Verify all entries
	for _, e := range entries {
		val, err := cache.Get(ctx, e.Key)
		require.NoError(t, err)
		assert.Equal(t, e.Value, val)
	}
}

func TestMemoryCache_Stats(t *testing.T) {
	cfg := DefaultMemoryCacheConfig()
	cache := NewMemoryCache(cfg)
	defer cache.Close()

	ctx := context.Background()

	// Generate some hits and misses
	_ = cache.Set(ctx, "stats-key", []byte("value"), 0)
	_, _ = cache.Get(ctx, "stats-key") // Hit
	_, _ = cache.Get(ctx, "stats-key") // Hit
	_, _ = cache.Get(ctx, "missing")   // Miss

	stats := cache.Stats()
	assert.Equal(t, int64(2), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, int64(1), stats.Sets)
	assert.InDelta(t, 0.666, stats.HitRate, 0.01)
}

func TestMemoryCache_Concurrent(t *testing.T) {
	cfg := MemoryCacheConfig{
		MaxSize:         1000,
		DefaultTTL:      time.Minute,
		MaxItemSize:     1024 * 1024,
		CleanupInterval: time.Hour,
	}
	cache := NewMemoryCache(cfg)
	defer cache.Close()

	ctx := context.Background()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + (i % 26)))
			_ = cache.Set(ctx, key, []byte("value"), 0)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + (i % 26)))
			_, _ = cache.Get(ctx, key)
		}(i)
	}

	wg.Wait()
	// No race conditions or panics
}

func TestMemoryCache_Flush(t *testing.T) {
	cfg := DefaultMemoryCacheConfig()
	cache := NewMemoryCache(cfg)
	defer cache.Close()

	ctx := context.Background()

	// Add some entries
	_ = cache.Set(ctx, "f1", []byte("v1"), 0)
	_ = cache.Set(ctx, "f2", []byte("v2"), 0)

	assert.Equal(t, 2, cache.Len())

	// Flush
	cache.Flush()

	assert.Equal(t, 0, cache.Len())

	// Verify entries are gone
	val, _ := cache.Get(ctx, "f1")
	assert.Nil(t, val)
}

func BenchmarkMemoryCache_Set(b *testing.B) {
	cfg := DefaultMemoryCacheConfig()
	cache := NewMemoryCache(cfg)
	defer cache.Close()

	ctx := context.Background()
	value := []byte("benchmark value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Set(ctx, "bench-key", value, 0)
	}
}

func BenchmarkMemoryCache_Get(b *testing.B) {
	cfg := DefaultMemoryCacheConfig()
	cache := NewMemoryCache(cfg)
	defer cache.Close()

	ctx := context.Background()
	_ = cache.Set(ctx, "bench-key", []byte("benchmark value"), 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(ctx, "bench-key")
	}
}

func BenchmarkMemoryCache_Concurrent(b *testing.B) {
	cfg := MemoryCacheConfig{
		MaxSize:         10000,
		DefaultTTL:      time.Minute,
		MaxItemSize:     1024 * 1024,
		CleanupInterval: time.Hour,
	}
	cache := NewMemoryCache(cfg)
	defer cache.Close()

	ctx := context.Background()
	value := []byte("benchmark value")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := string(rune('a' + (i % 26)))
			if i%2 == 0 {
				_ = cache.Set(ctx, key, value, 0)
			} else {
				_, _ = cache.Get(ctx, key)
			}
			i++
		}
	})
}
