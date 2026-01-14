package routers

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

func TestMemoryRoundRobinStore_NextIndexCycles(t *testing.T) {
	store := NewMemoryRoundRobinStore()
	ctx := context.Background()

	idx, err := store.NextIndex(ctx, "model-a", 2)
	require.NoError(t, err)
	require.Equal(t, 0, idx)

	idx, err = store.NextIndex(ctx, "model-a", 2)
	require.NoError(t, err)
	require.Equal(t, 1, idx)

	idx, err = store.NextIndex(ctx, "model-a", 2)
	require.NoError(t, err)
	require.Equal(t, 0, idx)

	require.NoError(t, store.Reset(ctx, "model-a"))
	idx, err = store.NextIndex(ctx, "model-a", 2)
	require.NoError(t, err)
	require.Equal(t, 0, idx)
}

func TestRedisRoundRobinStore_NextIndexCycles(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	store := NewRedisRoundRobinStore(client)
	ctx := context.Background()

	idx, err := store.NextIndex(ctx, "model-b", 2)
	require.NoError(t, err)
	require.Equal(t, 0, idx)

	idx, err = store.NextIndex(ctx, "model-b", 2)
	require.NoError(t, err)
	require.Equal(t, 1, idx)

	idx, err = store.NextIndex(ctx, "model-b", 2)
	require.NoError(t, err)
	require.Equal(t, 0, idx)

	require.NoError(t, store.Reset(ctx, "model-b"))
	idx, err = store.NextIndex(ctx, "model-b", 2)
	require.NoError(t, err)
	require.Equal(t, 0, idx)
}

func TestRedisRoundRobinStore_NextIndexSetsTTL(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	store := NewRedisRoundRobinStore(client)
	ctx := context.Background()

	_, err := store.NextIndex(ctx, "model-ttl", 2)
	require.NoError(t, err)

	ttl := s.TTL(roundRobinKeyPrefix + "model-ttl")
	if ttl <= 0 {
		t.Fatalf("expected TTL to be set, got %v", ttl)
	}
	if ttl > 48*time.Hour {
		t.Fatalf("expected TTL to be bounded, got %v", ttl)
	}
}

func TestRoundRobinRouter_WithDistributedStore_UsesSharedCounter(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	rrStore := NewRedisRoundRobinStore(client)

	cfg := router.DefaultConfig()
	r1 := newRoundRobinRouterWithStores(cfg, nil, rrStore)
	r2 := newRoundRobinRouterWithStores(cfg, nil, rrStore)

	depA := &provider.Deployment{ID: "a", ProviderName: "p1", ModelName: "gpt-4"}
	depB := &provider.Deployment{ID: "b", ProviderName: "p2", ModelName: "gpt-4"}

	r1.AddDeployment(depA)
	r1.AddDeployment(depB)
	r2.AddDeployment(depA)
	r2.AddDeployment(depB)

	ctx := context.Background()

	pick1, err := r1.Pick(ctx, "gpt-4")
	require.NoError(t, err)
	require.Equal(t, "a", pick1.ID)

	pick2, err := r2.Pick(ctx, "gpt-4")
	require.NoError(t, err)
	require.Equal(t, "b", pick2.ID)

	pick3, err := r1.Pick(ctx, "gpt-4")
	require.NoError(t, err)
	require.Equal(t, "a", pick3.ID)

	pick4, err := r2.Pick(ctx, "gpt-4")
	require.NoError(t, err)
	require.Equal(t, "b", pick4.ID)
}
