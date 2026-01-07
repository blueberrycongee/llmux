package cache

import (
	"context"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestHandler_GetAndSetCachedResponse(t *testing.T) {
	cache := NewMemoryCache(DefaultMemoryCacheConfig())
	defer cache.Close()

	handler := NewHandler(cache, nil, DefaultHandlerConfig())
	ctx := context.Background()

	req := &types.ChatRequest{
		Model: "gpt-4",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	}

	t.Run("cache miss then hit", func(t *testing.T) {
		// First request - cache miss
		cached, err := handler.GetCachedResponse(ctx, req, nil)
		require.NoError(t, err)
		assert.Nil(t, cached)

		// Store response
		response := []byte(`{"id":"123","choices":[{"message":{"content":"Hi!"}}]}`)
		err = handler.SetCachedResponse(ctx, req, response, nil)
		require.NoError(t, err)

		// Second request - cache hit
		cached, err = handler.GetCachedResponse(ctx, req, nil)
		require.NoError(t, err)
		require.NotNil(t, cached)
		assert.Equal(t, response, cached.Response)
		assert.Equal(t, "gpt-4", cached.Model)
	})
}

func TestHandler_CacheControl(t *testing.T) {
	cache := NewMemoryCache(DefaultMemoryCacheConfig())
	defer cache.Close()

	handler := NewHandler(cache, nil, DefaultHandlerConfig())
	ctx := context.Background()

	req := &types.ChatRequest{
		Model: "gpt-4",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Test"`)},
		},
	}

	t.Run("no-cache skips read", func(t *testing.T) {
		// Store a response
		response := []byte(`{"cached": true}`)
		err := handler.SetCachedResponse(ctx, req, response, nil)
		require.NoError(t, err)

		// With no-cache, should skip reading
		ctrl := &CacheControl{NoCache: true}
		cached, err := handler.GetCachedResponse(ctx, req, ctrl)
		require.NoError(t, err)
		assert.Nil(t, cached)
	})

	t.Run("no-store skips write", func(t *testing.T) {
		req2 := &types.ChatRequest{
			Model: "gpt-4",
			Messages: []types.ChatMessage{
				{Role: "user", Content: json.RawMessage(`"NoStore"`)},
			},
		}

		ctrl := &CacheControl{NoStore: true}
		response := []byte(`{"should_not_cache": true}`)
		err := handler.SetCachedResponse(ctx, req2, response, ctrl)
		require.NoError(t, err)

		// Should not be cached
		cached, err := handler.GetCachedResponse(ctx, req2, nil)
		require.NoError(t, err)
		assert.Nil(t, cached)
	})

	t.Run("custom TTL", func(t *testing.T) {
		req3 := &types.ChatRequest{
			Model: "gpt-4",
			Messages: []types.ChatMessage{
				{Role: "user", Content: json.RawMessage(`"CustomTTL"`)},
			},
		}

		ctrl := &CacheControl{TTL: 50 * time.Millisecond}
		response := []byte(`{"ttl_test": true}`)
		err := handler.SetCachedResponse(ctx, req3, response, ctrl)
		require.NoError(t, err)

		// Should exist immediately
		cached, err := handler.GetCachedResponse(ctx, req3, nil)
		require.NoError(t, err)
		assert.NotNil(t, cached)

		// Wait for TTL
		time.Sleep(60 * time.Millisecond)

		// Should be expired
		cached, err = handler.GetCachedResponse(ctx, req3, nil)
		require.NoError(t, err)
		assert.Nil(t, cached)
	})

	t.Run("namespace isolation", func(t *testing.T) {
		req4 := &types.ChatRequest{
			Model: "gpt-4",
			Messages: []types.ChatMessage{
				{Role: "user", Content: json.RawMessage(`"Namespace"`)},
			},
		}

		// Store with namespace A
		ctrlA := &CacheControl{Namespace: "tenant-a"}
		responseA := []byte(`{"tenant": "a"}`)
		err := handler.SetCachedResponse(ctx, req4, responseA, ctrlA)
		require.NoError(t, err)

		// Store with namespace B
		ctrlB := &CacheControl{Namespace: "tenant-b"}
		responseB := []byte(`{"tenant": "b"}`)
		err = handler.SetCachedResponse(ctx, req4, responseB, ctrlB)
		require.NoError(t, err)

		// Get with namespace A
		cached, err := handler.GetCachedResponse(ctx, req4, ctrlA)
		require.NoError(t, err)
		require.NotNil(t, cached)
		assert.Equal(t, responseA, cached.Response)

		// Get with namespace B
		cached, err = handler.GetCachedResponse(ctx, req4, ctrlB)
		require.NoError(t, err)
		require.NotNil(t, cached)
		assert.Equal(t, responseB, cached.Response)
	})

	t.Run("max-age check", func(t *testing.T) {
		req5 := &types.ChatRequest{
			Model: "gpt-4",
			Messages: []types.ChatMessage{
				{Role: "user", Content: json.RawMessage(`"MaxAge"`)},
			},
		}

		// Store response
		response := []byte(`{"max_age_test": true}`)
		err := handler.SetCachedResponse(ctx, req5, response, nil)
		require.NoError(t, err)

		// Wait a bit
		time.Sleep(50 * time.Millisecond)

		// With short max-age, should be considered stale
		ctrl := &CacheControl{MaxAge: 10 * time.Millisecond}
		cached, err := handler.GetCachedResponse(ctx, req5, ctrl)
		require.NoError(t, err)
		assert.Nil(t, cached) // Too old

		// With longer max-age, should still be valid
		ctrl = &CacheControl{MaxAge: time.Hour}
		cached, err = handler.GetCachedResponse(ctx, req5, ctrl)
		require.NoError(t, err)
		assert.NotNil(t, cached)
	})
}

func TestHandler_InvalidateCache(t *testing.T) {
	cache := NewMemoryCache(DefaultMemoryCacheConfig())
	defer cache.Close()

	handler := NewHandler(cache, nil, DefaultHandlerConfig())
	ctx := context.Background()

	req := &types.ChatRequest{
		Model: "gpt-4",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Invalidate"`)},
		},
	}

	// Store response
	response := []byte(`{"to_invalidate": true}`)
	err := handler.SetCachedResponse(ctx, req, response, nil)
	require.NoError(t, err)

	// Verify it exists
	cached, err := handler.GetCachedResponse(ctx, req, nil)
	require.NoError(t, err)
	assert.NotNil(t, cached)

	// Invalidate
	err = handler.InvalidateCache(ctx, req, nil)
	require.NoError(t, err)

	// Should be gone
	cached, err = handler.GetCachedResponse(ctx, req, nil)
	require.NoError(t, err)
	assert.Nil(t, cached)
}

func TestHandler_Disabled(t *testing.T) {
	cache := NewMemoryCache(DefaultMemoryCacheConfig())
	defer cache.Close()

	cfg := DefaultHandlerConfig()
	cfg.Enabled = false
	handler := NewHandler(cache, nil, cfg)
	ctx := context.Background()

	req := &types.ChatRequest{
		Model: "gpt-4",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Disabled"`)},
		},
	}

	// Set should be no-op
	response := []byte(`{"disabled": true}`)
	err := handler.SetCachedResponse(ctx, req, response, nil)
	require.NoError(t, err)

	// Get should return nil
	cached, err := handler.GetCachedResponse(ctx, req, nil)
	require.NoError(t, err)
	assert.Nil(t, cached)

	// Enable at runtime
	handler.SetEnabled(true)

	// Now it should work
	err = handler.SetCachedResponse(ctx, req, response, nil)
	require.NoError(t, err)

	cached, err = handler.GetCachedResponse(ctx, req, nil)
	require.NoError(t, err)
	assert.NotNil(t, cached)
}

func TestHandler_MaxCacheableSize(t *testing.T) {
	cache := NewMemoryCache(DefaultMemoryCacheConfig())
	defer cache.Close()

	cfg := DefaultHandlerConfig()
	cfg.MaxCacheableSize = 100 // 100 bytes max
	handler := NewHandler(cache, nil, cfg)
	ctx := context.Background()

	req := &types.ChatRequest{
		Model: "gpt-4",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Size"`)},
		},
	}

	// Large response should not be cached
	largeResponse := make([]byte, 200)
	err := handler.SetCachedResponse(ctx, req, largeResponse, nil)
	require.NoError(t, err)

	cached, err := handler.GetCachedResponse(ctx, req, nil)
	require.NoError(t, err)
	assert.Nil(t, cached)

	// Small response should be cached
	smallResponse := []byte(`{"small": true}`)
	err = handler.SetCachedResponse(ctx, req, smallResponse, nil)
	require.NoError(t, err)

	cached, err = handler.GetCachedResponse(ctx, req, nil)
	require.NoError(t, err)
	assert.NotNil(t, cached)
}

func TestHandler_Stats(t *testing.T) {
	cache := NewMemoryCache(DefaultMemoryCacheConfig())
	defer cache.Close()

	handler := NewHandler(cache, nil, DefaultHandlerConfig())
	ctx := context.Background()

	req := &types.ChatRequest{
		Model: "gpt-4",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Stats"`)},
		},
	}

	// Generate activity
	_, _ = handler.GetCachedResponse(ctx, req, nil) // Miss
	_ = handler.SetCachedResponse(ctx, req, []byte(`{}`), nil)
	_, _ = handler.GetCachedResponse(ctx, req, nil) // Hit

	stats := handler.Stats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
}

func TestHandler_NilCache(t *testing.T) {
	handler := NewHandler(nil, nil, DefaultHandlerConfig())
	ctx := context.Background()

	req := &types.ChatRequest{
		Model: "gpt-4",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Nil"`)},
		},
	}

	// Should not panic with nil cache
	cached, err := handler.GetCachedResponse(ctx, req, nil)
	require.NoError(t, err)
	assert.Nil(t, cached)

	err = handler.SetCachedResponse(ctx, req, []byte(`{}`), nil)
	require.NoError(t, err)

	err = handler.Ping(ctx)
	require.NoError(t, err)

	err = handler.Close()
	require.NoError(t, err)
}

func TestParseCacheControl(t *testing.T) {
	t.Run("valid cache control", func(t *testing.T) {
		raw := json.RawMessage(`{"ttl": 3600000000000, "namespace": "test", "no-cache": true}`)
		ctrl := ParseCacheControl(raw)
		require.NotNil(t, ctrl)
		assert.Equal(t, time.Hour, ctrl.TTL)
		assert.Equal(t, "test", ctrl.Namespace)
		assert.True(t, ctrl.NoCache)
	})

	t.Run("empty input", func(t *testing.T) {
		ctrl := ParseCacheControl(nil)
		assert.Nil(t, ctrl)

		ctrl = ParseCacheControl(json.RawMessage{})
		assert.Nil(t, ctrl)
	})

	t.Run("invalid json", func(t *testing.T) {
		ctrl := ParseCacheControl(json.RawMessage(`invalid`))
		assert.Nil(t, ctrl)
	})
}

func TestHandler_DifferentRequestsProduceDifferentKeys(t *testing.T) {
	cache := NewMemoryCache(DefaultMemoryCacheConfig())
	defer cache.Close()

	handler := NewHandler(cache, nil, DefaultHandlerConfig())
	ctx := context.Background()

	req1 := &types.ChatRequest{
		Model: "gpt-4",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Hello"`)},
		},
	}

	req2 := &types.ChatRequest{
		Model: "gpt-4",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"World"`)},
		},
	}

	// Store different responses
	_ = handler.SetCachedResponse(ctx, req1, []byte(`{"response": "hello"}`), nil)
	_ = handler.SetCachedResponse(ctx, req2, []byte(`{"response": "world"}`), nil)

	// Verify they are stored separately
	cached1, _ := handler.GetCachedResponse(ctx, req1, nil)
	cached2, _ := handler.GetCachedResponse(ctx, req2, nil)

	require.NotNil(t, cached1)
	require.NotNil(t, cached2)
	assert.NotEqual(t, cached1.Response, cached2.Response)
}

func BenchmarkHandler_GetCachedResponse(b *testing.B) {
	cache := NewMemoryCache(DefaultMemoryCacheConfig())
	defer cache.Close()

	handler := NewHandler(cache, nil, DefaultHandlerConfig())
	ctx := context.Background()

	req := &types.ChatRequest{
		Model: "gpt-4",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Benchmark"`)},
		},
	}

	_ = handler.SetCachedResponse(ctx, req, []byte(`{"benchmark": true}`), nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = handler.GetCachedResponse(ctx, req, nil)
	}
}

func BenchmarkHandler_SetCachedResponse(b *testing.B) {
	cache := NewMemoryCache(DefaultMemoryCacheConfig())
	defer cache.Close()

	handler := NewHandler(cache, nil, DefaultHandlerConfig())
	ctx := context.Background()

	req := &types.ChatRequest{
		Model: "gpt-4",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"Benchmark"`)},
		},
	}
	response := []byte(`{"benchmark": true}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handler.SetCachedResponse(ctx, req, response, nil)
	}
}
