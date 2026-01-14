package routers

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/router"
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

func TestRedisStatsStore_TenantScope_IsolatesCooldown(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	store := NewRedisStatsStore(client)

	deploymentID := "deployment-scope"
	ctxA := router.WithTenantScope(context.Background(), "tenant-a")
	ctxB := router.WithTenantScope(context.Background(), "tenant-b")

	require.NoError(t, store.RecordFailure(ctxA, deploymentID, llmerrors.NewRateLimitError("openai", "gpt-4", "rate limited")))

	cooldownUntilA, err := store.GetCooldownUntil(ctxA, deploymentID)
	require.NoError(t, err)
	require.True(t, cooldownUntilA.After(time.Now()))

	cooldownUntilB, err := store.GetCooldownUntil(ctxB, deploymentID)
	require.NoError(t, err)
	require.True(t, cooldownUntilB.IsZero())
}

func TestRedisStatsStore_KeyHashTag(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	store := NewRedisStatsStore(client)

	ctx := router.WithTenantScope(context.Background(), "tenant-a")
	deploymentID := "deployment-1"
	minute := "12345"
	keys := []string{
		store.latencyKey(ctx, deploymentID),
		store.ttftKey(ctx, deploymentID),
		store.countersKey(ctx, deploymentID),
		store.cooldownKey(ctx, deploymentID),
		store.usageKey(ctx, deploymentID, minute),
		store.successKey(ctx, deploymentID, minute),
		store.failureKey(ctx, deploymentID, minute),
	}

	for _, key := range keys {
		require.Contains(t, key, "{tenant-a:"+deploymentID+"}")
	}

	require.Equal(t, deploymentID, store.extractDeploymentID(store.countersKey(ctx, deploymentID)))
}

type denyTimeHook struct{}

func (denyTimeHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (denyTimeHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if strings.EqualFold(cmd.Name(), "time") {
			return errors.New("TIME command disabled")
		}
		return next(ctx, cmd)
	}
}

func (denyTimeHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		for _, cmd := range cmds {
			if strings.EqualFold(cmd.Name(), "time") {
				return errors.New("TIME command disabled")
			}
		}
		return next(ctx, cmds)
	}
}

func TestRedisStatsStore_UsesRedisTimeInScripts(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	client.AddHook(denyTimeHook{})

	store := NewRedisStatsStore(client)
	ctx := context.Background()
	deploymentID := "deployment-time"
	fixed := time.Date(2026, 1, 10, 12, 34, 56, 0, time.UTC)
	s.SetTime(fixed)

	require.NoError(t, store.RecordSuccess(ctx, deploymentID, &router.ResponseMetrics{
		Latency:     10 * time.Millisecond,
		TotalTokens: 7,
	}))

	stats, err := store.GetStats(ctx, deploymentID)
	require.NoError(t, err)
	require.Equal(t, minuteKey(fixed), stats.CurrentMinuteKey)
}

func TestRedisStatsStore_UsesRedisTimeForBucketKeys(t *testing.T) {
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	client.AddHook(denyTimeHook{})

	store := NewRedisStatsStore(client, WithFailureBucketSeconds(10))
	ctx := context.Background()
	deploymentID := "deployment-bucket"
	fixed := time.Date(2026, 1, 10, 0, 0, 25, 0, time.UTC)
	s.SetTime(fixed)

	require.NoError(t, store.RecordSuccess(ctx, deploymentID, &router.ResponseMetrics{
		Latency:     5 * time.Millisecond,
		TotalTokens: 3,
	}))

	bucket := fixed.Unix() / 10
	expectedKey := store.successKey(ctx, deploymentID, strconv.FormatInt(bucket, 10))
	require.True(t, s.Exists(expectedKey))
}
