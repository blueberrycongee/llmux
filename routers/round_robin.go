package routers

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

// RoundRobinRouter implements strict round-robin selection.
type RoundRobinRouter struct {
	*BaseRouter
	counters sync.Map // map[string]*atomic.Uint64
	rrStore  router.RoundRobinStore
}

// NewRoundRobinRouter creates a new round-robin router with default config.
func NewRoundRobinRouter(cooldownPeriod ...interface{}) *RoundRobinRouter {
	config := router.DefaultConfig()
	config.Strategy = router.StrategyRoundRobin
	return &RoundRobinRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// NewRoundRobinRouterWithConfig creates a new round-robin router with custom config.
func NewRoundRobinRouterWithConfig(config router.Config) *RoundRobinRouter {
	config.Strategy = router.StrategyRoundRobin
	return &RoundRobinRouter{
		BaseRouter: NewBaseRouter(config),
	}
}

// newRoundRobinRouterWithStores creates a round-robin router with optional stats and RR stores.
func newRoundRobinRouterWithStores(config router.Config, store router.StatsStore, rrStore router.RoundRobinStore) *RoundRobinRouter {
	config.Strategy = router.StrategyRoundRobin
	var base *BaseRouter
	if store != nil {
		base = NewBaseRouterWithStore(config, store)
	} else {
		base = NewBaseRouter(config)
	}
	return &RoundRobinRouter{
		BaseRouter: base,
		rrStore:    rrStore,
	}
}

// Pick selects the next deployment in round-robin order.
func (r *RoundRobinRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	return r.PickWithContext(ctx, &router.RequestContext{Model: model})
}

// PickWithContext selects the next deployment in round-robin order, honoring request context filters.
func (r *RoundRobinRouter) PickWithContext(ctx context.Context, reqCtx *router.RequestContext) (*provider.Deployment, error) {
	if reqCtx == nil {
		reqCtx = &router.RequestContext{}
	}

	deployments := r.snapshotDeployments(reqCtx.Model)
	if len(deployments) == 0 {
		return nil, ErrNoAvailableDeployment
	}
	statsByID := r.statsSnapshot(ctx, deployments)
	healthy := r.getHealthyDeployments(deployments, statsByID)
	if len(healthy) == 0 {
		return nil, ErrNoAvailableDeployment
	}

	if r.config.EnableTagFiltering && len(reqCtx.Tags) > 0 {
		healthy = r.filterByTags(healthy, reqCtx.Tags)
		if len(healthy) == 0 {
			return nil, ErrNoDeploymentsWithTag
		}
	}

	if reqCtx.EstimatedInputTokens > 0 {
		healthy = r.filterByTPMRPM(healthy, statsByID, reqCtx.EstimatedInputTokens)
		if len(healthy) == 0 {
			return nil, ErrNoAvailableDeployment
		}
	}

	index := r.nextIndex(reqCtx.Model, len(healthy))
	return healthy[index].Deployment, nil
}

func (r *RoundRobinRouter) nextIndex(model string, count int) int {
	if count <= 0 {
		return 0
	}
	if r.rrStore != nil {
		idx, err := r.rrStore.NextIndex(context.Background(), model, count)
		if err == nil && idx >= 0 && idx < count {
			return idx
		}
	}
	counter := r.counterForModel(model)
	next := counter.Add(1) - 1
	return int(next % uint64(count))
}

func (r *RoundRobinRouter) counterForModel(model string) *atomic.Uint64 {
	if existing, ok := r.counters.Load(model); ok {
		if counter, ok := existing.(*atomic.Uint64); ok {
			return counter
		}
	}
	counter := &atomic.Uint64{}
	actual, _ := r.counters.LoadOrStore(model, counter)
	if stored, ok := actual.(*atomic.Uint64); ok {
		return stored
	}
	return counter
}
