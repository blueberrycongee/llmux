package routers

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCostRouter_WithStatsStore_RespectsCooldown(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStatsStore()
	config := router.DefaultConfig()
	r := newCostRouterWithStore(config, store)

	depA := &provider.Deployment{ID: "dep-a", ModelName: "gpt-4", ProviderName: "a"}
	depB := &provider.Deployment{ID: "dep-b", ModelName: "gpt-4", ProviderName: "b"}
	r.AddDeploymentWithConfig(depA, router.DeploymentConfig{
		InputCostPerToken:  0.1,
		OutputCostPerToken: 0.1,
	})
	r.AddDeploymentWithConfig(depB, router.DeploymentConfig{
		InputCostPerToken:  1.0,
		OutputCostPerToken: 1.0,
	})

	require.NoError(t, store.SetCooldown(ctx, depA.ID, time.Now().Add(2*time.Minute)))

	picked, err := r.Pick(ctx, "gpt-4")
	require.NoError(t, err)
	assert.Equal(t, depB.ID, picked.ID)
}

func TestCostRouter_WithStatsStore_RespectsTPMRPM(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStatsStore()
	config := router.DefaultConfig()
	r := newCostRouterWithStore(config, store)

	depA := &provider.Deployment{ID: "dep-a", ModelName: "gpt-4", ProviderName: "a"}
	depB := &provider.Deployment{ID: "dep-b", ModelName: "gpt-4", ProviderName: "b"}
	r.AddDeploymentWithConfig(depA, router.DeploymentConfig{
		InputCostPerToken:  0.1,
		OutputCostPerToken: 0.1,
		TPMLimit:           100,
	})
	r.AddDeploymentWithConfig(depB, router.DeploymentConfig{
		InputCostPerToken:  1.0,
		OutputCostPerToken: 1.0,
		TPMLimit:           100,
	})

	require.NoError(t, store.RecordSuccess(ctx, depA.ID, &router.ResponseMetrics{
		Latency:     10 * time.Millisecond,
		TotalTokens: 95,
	}))

	picked, err := r.PickWithContext(ctx, &router.RequestContext{
		Model:                "gpt-4",
		EstimatedInputTokens: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, depB.ID, picked.ID)
}

func TestLeastBusyRouter_WithStatsStore_UsesActiveRequests(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStatsStore()
	config := router.DefaultConfig()
	r := newLeastBusyRouterWithStore(config, store)
	r.rng = rand.New(&fixedSource{value: 1})

	depA := &provider.Deployment{ID: "dep-a", ModelName: "gpt-4", ProviderName: "a"}
	depB := &provider.Deployment{ID: "dep-b", ModelName: "gpt-4", ProviderName: "b"}
	r.AddDeployment(depA)
	r.AddDeployment(depB)

	require.NoError(t, store.IncrementActiveRequests(ctx, depA.ID))
	require.NoError(t, store.IncrementActiveRequests(ctx, depA.ID))

	picked, err := r.Pick(ctx, "gpt-4")
	require.NoError(t, err)
	assert.Equal(t, depB.ID, picked.ID)
}

func TestLatencyRouter_StatsSnapshotCachedPerPick(t *testing.T) {
	ctx := context.Background()
	store := newCountingStatsStore(map[string]*router.DeploymentStats{
		"dep-a": {
			LatencyHistory: []float64{10},
		},
		"dep-b": {
			LatencyHistory: []float64{20},
		},
	})
	config := router.DefaultConfig()
	r := newLatencyRouterWithStore(config, store)

	depA := &provider.Deployment{ID: "dep-a", ModelName: "gpt-4"}
	depB := &provider.Deployment{ID: "dep-b", ModelName: "gpt-4"}
	r.AddDeployment(depA)
	r.AddDeployment(depB)

	picked, err := r.PickWithContext(ctx, &router.RequestContext{
		Model:                "gpt-4",
		EstimatedInputTokens: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, depA.ID, picked.ID)

	assert.Equal(t, 1, store.getCalls["dep-a"])
	assert.Equal(t, 1, store.getCalls["dep-b"])
}

func TestCostRouter_WithStatsStore_FailOpenOnStatsError(t *testing.T) {
	ctx := context.Background()
	store := &failingStatsStore{}
	config := router.DefaultConfig()
	r := newCostRouterWithStore(config, store)

	depA := &provider.Deployment{ID: "dep-a", ModelName: "gpt-4", ProviderName: "a"}
	depB := &provider.Deployment{ID: "dep-b", ModelName: "gpt-4", ProviderName: "b"}
	r.AddDeploymentWithConfig(depA, router.DeploymentConfig{
		InputCostPerToken:  0.1,
		OutputCostPerToken: 0.1,
	})
	r.AddDeploymentWithConfig(depB, router.DeploymentConfig{
		InputCostPerToken:  1.0,
		OutputCostPerToken: 1.0,
	})

	picked, err := r.Pick(ctx, "gpt-4")
	require.NoError(t, err)
	assert.Equal(t, depA.ID, picked.ID)
}

type fixedSource struct {
	value int64
}

func (s *fixedSource) Int63() int64 {
	return s.value
}

func (s *fixedSource) Seed(seed int64) {}

type countingStatsStore struct {
	mu       sync.Mutex
	stats    map[string]*router.DeploymentStats
	getCalls map[string]int
}

func newCountingStatsStore(stats map[string]*router.DeploymentStats) *countingStatsStore {
	return &countingStatsStore{
		stats:    stats,
		getCalls: make(map[string]int),
	}
}

func (c *countingStatsStore) GetStats(ctx context.Context, deploymentID string) (*router.DeploymentStats, error) {
	c.mu.Lock()
	c.getCalls[deploymentID]++
	stats, ok := c.stats[deploymentID]
	c.mu.Unlock()
	if !ok {
		return nil, router.ErrStatsNotFound
	}
	return stats, nil
}

func (c *countingStatsStore) IncrementActiveRequests(ctx context.Context, deploymentID string) error {
	return nil
}

func (c *countingStatsStore) DecrementActiveRequests(ctx context.Context, deploymentID string) error {
	return nil
}

func (c *countingStatsStore) RecordSuccess(ctx context.Context, deploymentID string, metrics *router.ResponseMetrics) error {
	return nil
}

func (c *countingStatsStore) RecordFailure(ctx context.Context, deploymentID string, err error) error {
	return nil
}

func (c *countingStatsStore) SetCooldown(ctx context.Context, deploymentID string, until time.Time) error {
	return nil
}

func (c *countingStatsStore) GetCooldownUntil(ctx context.Context, deploymentID string) (time.Time, error) {
	return time.Time{}, nil
}

func (c *countingStatsStore) ListDeployments(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (c *countingStatsStore) DeleteStats(ctx context.Context, deploymentID string) error {
	return nil
}

func (c *countingStatsStore) Close() error {
	return nil
}

type failingStatsStore struct{}

func (f *failingStatsStore) GetStats(ctx context.Context, deploymentID string) (*router.DeploymentStats, error) {
	return nil, router.ErrStoreNotAvailable
}

func (f *failingStatsStore) IncrementActiveRequests(ctx context.Context, deploymentID string) error {
	return router.ErrStoreNotAvailable
}

func (f *failingStatsStore) DecrementActiveRequests(ctx context.Context, deploymentID string) error {
	return router.ErrStoreNotAvailable
}

func (f *failingStatsStore) RecordSuccess(ctx context.Context, deploymentID string, metrics *router.ResponseMetrics) error {
	return router.ErrStoreNotAvailable
}

func (f *failingStatsStore) RecordFailure(ctx context.Context, deploymentID string, err error) error {
	return router.ErrStoreNotAvailable
}

func (f *failingStatsStore) SetCooldown(ctx context.Context, deploymentID string, until time.Time) error {
	return router.ErrStoreNotAvailable
}

func (f *failingStatsStore) GetCooldownUntil(ctx context.Context, deploymentID string) (time.Time, error) {
	return time.Time{}, router.ErrStoreNotAvailable
}

func (f *failingStatsStore) ListDeployments(ctx context.Context) ([]string, error) {
	return nil, router.ErrStoreNotAvailable
}

func (f *failingStatsStore) DeleteStats(ctx context.Context, deploymentID string) error {
	return router.ErrStoreNotAvailable
}

func (f *failingStatsStore) Close() error {
	return nil
}
