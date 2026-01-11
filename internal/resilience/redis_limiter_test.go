package resilience

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisLimiter_CheckAllow(t *testing.T) {
	// Start miniredis
	s := miniredis.RunT(t)

	// Create redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	var client redis.UniversalClient = rdb

	// Initialize RedisLimiter
	limiter := NewRedisLimiter(client)

	ctx := context.Background()

	t.Run("Single Limit - Under Limit", func(t *testing.T) {
		desc := Descriptor{
			Key:    "user-1",
			Value:  "model-a",
			Limit:  10,
			Type:   LimitTypeRequests,
			Window: time.Minute,
		}

		results, err := limiter.CheckAllow(ctx, []Descriptor{desc})
		require.NoError(t, err)
		require.Len(t, results, 1)

		assert.True(t, results[0].Allowed)
		assert.Equal(t, int64(1), results[0].Current)
		assert.Equal(t, int64(9), results[0].Remaining)
	})

	t.Run("Single Limit - Exceed Limit", func(t *testing.T) {
		key := "user-2"
		desc := Descriptor{
			Key:    key,
			Value:  "model-a",
			Limit:  2,
			Type:   LimitTypeRequests,
			Window: time.Minute,
		}

		// 1st request
		results, err := limiter.CheckAllow(ctx, []Descriptor{desc})
		require.NoError(t, err)
		assert.True(t, results[0].Allowed)
		assert.Equal(t, int64(1), results[0].Current)

		// 2nd request
		results, err = limiter.CheckAllow(ctx, []Descriptor{desc})
		require.NoError(t, err)
		assert.True(t, results[0].Allowed)
		assert.Equal(t, int64(2), results[0].Current)

		// 3rd request (should fail)
		results, err = limiter.CheckAllow(ctx, []Descriptor{desc})
		require.NoError(t, err)
		assert.False(t, results[0].Allowed)
		assert.Equal(t, int64(3), results[0].Current)
		assert.Equal(t, int64(0), results[0].Remaining)
	})

	t.Run("Batch Limits - Mixed Results", func(t *testing.T) {
		// User 3: Limit 2
		// User 4: Limit 10
		desc1 := Descriptor{Key: "user-3", Value: "model-a", Limit: 2, Type: LimitTypeRequests, Window: time.Minute}
		desc2 := Descriptor{Key: "user-4", Value: "model-a", Limit: 10, Type: LimitTypeRequests, Window: time.Minute}

		// Exhaust User 3
		_, _ = limiter.CheckAllow(ctx, []Descriptor{desc1, desc1})

		// Check both
		results, err := limiter.CheckAllow(ctx, []Descriptor{desc1, desc2})
		require.NoError(t, err)
		require.Len(t, results, 2)

		// User 3: 3rd request -> Denied
		assert.False(t, results[0].Allowed)
		assert.Equal(t, int64(3), results[0].Current)

		// User 4: 1st request -> Allowed
		assert.True(t, results[1].Allowed)
		assert.Equal(t, int64(1), results[1].Current)
	})

	t.Run("Window Expiry", func(t *testing.T) {
		desc := Descriptor{
			Key:    "user-5",
			Value:  "model-a",
			Limit:  5,
			Type:   LimitTypeRequests,
			Window: time.Second, // Short window
		}

		// 1st request
		results, err := limiter.CheckAllow(ctx, []Descriptor{desc})
		require.NoError(t, err)
		assert.Equal(t, int64(1), results[0].Current)

		// Fast forward time
		s.FastForward(2 * time.Second)

		// 2nd request (should be 1 again, new window)
		results, err = limiter.CheckAllow(ctx, []Descriptor{desc})
		require.NoError(t, err)
		assert.Equal(t, int64(1), results[0].Current)
	})
}

func TestRedisLimiter_KeyHashTagAndScript(t *testing.T) {
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	var client redis.UniversalClient = rdb

	limiter := NewRedisLimiter(client)
	ctx := context.Background()

	desc := Descriptor{
		Key:    "user-1",
		Value:  "model-a",
		Limit:  5,
		Type:   LimitTypeRequests,
		Window: time.Minute,
	}

	_, err := limiter.CheckAllow(ctx, []Descriptor{desc})
	require.NoError(t, err)

	tag := "{user-1:model-a}"
	windowKey := tag + ":requests:window"
	counterKey := tag + ":requests:count"

	require.ElementsMatch(t, []string{counterKey, windowKey}, s.Keys())

	counterVal, err := client.Get(ctx, counterKey).Result()
	require.NoError(t, err)
	require.Equal(t, "1", counterVal)

	windowVal, err := client.Get(ctx, windowKey).Result()
	require.NoError(t, err)
	require.NotEmpty(t, windowVal)
}
