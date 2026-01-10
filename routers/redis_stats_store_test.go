package routers

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/router"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisStatsStore_FailureRateCooldown(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	store := NewRedisStatsStore(
		client,
		WithFailureThresholdPercent(0.5),
		WithMinRequestsForThreshold(5),
		WithCooldownPeriod(2*time.Minute),
	)

	ctx := context.Background()
	deploymentID := "deployment-1"

	require.NoError(t, store.RecordSuccess(ctx, deploymentID, &router.ResponseMetrics{
		Latency: 50 * time.Millisecond,
	}))
	for i := 0; i < 4; i++ {
		require.NoError(t, store.RecordFailure(ctx, deploymentID, llmerrors.NewInternalError("openai", "gpt-4", "boom")))
	}

	cooldownUntil, err := store.GetCooldownUntil(ctx, deploymentID)
	require.NoError(t, err)
	require.True(t, cooldownUntil.After(time.Now()))
}

func TestRedisStatsStore_Immediate429Cooldown(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	store := NewRedisStatsStore(client)

	ctx := context.Background()
	deploymentID := "deployment-2"

	require.NoError(t, store.RecordFailure(ctx, deploymentID, llmerrors.NewRateLimitError("openai", "gpt-4", "rate limited")))

	cooldownUntil, err := store.GetCooldownUntil(ctx, deploymentID)
	require.NoError(t, err)
	require.True(t, cooldownUntil.After(time.Now()))
}

func TestRedisStatsStore_Immediate429SingleDeploymentSkipsCooldown(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	store := NewRedisStatsStore(client)

	ctx := context.Background()
	deploymentID := "deployment-3"

	require.NoError(t, store.RecordFailureWithOptions(
		ctx,
		deploymentID,
		llmerrors.NewRateLimitError("openai", "gpt-4", "rate limited"),
		failureRecordOptions{isSingleDeployment: true},
	))

	cooldownUntil, err := store.GetCooldownUntil(ctx, deploymentID)
	require.NoError(t, err)
	require.True(t, cooldownUntil.IsZero())
}
