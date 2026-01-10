package routers

import (
	"fmt"

	internalRouter "github.com/blueberrycongee/llmux/internal/router"
	"github.com/blueberrycongee/llmux/pkg/router"
)

// New creates a new router based on the specified strategy.
// Returns an error if the strategy is not recognized.
func New(config router.Config) (router.Router, error) {
	return NewWithStore(config, nil)
}

// NewWithStore creates a new router with a distributed stats store.
// When store is nil, the router uses local in-memory stats (single-instance mode).
// When store is provided, stats are shared across multiple instances (distributed mode).
func NewWithStore(config router.Config, store internalRouter.StatsStore) (router.Router, error) {
	switch config.Strategy {
	case router.StrategySimpleShuffle, "":
		return newShuffleRouterWithStore(config, store), nil
	case router.StrategyLowestLatency:
		return newLatencyRouterWithStore(config, store), nil
	case router.StrategyLeastBusy:
		return newLeastBusyRouterWithStore(config, store), nil
	case router.StrategyLowestTPMRPM:
		return newTPMRPMRouterWithStore(config, store), nil
	case router.StrategyLowestCost:
		return newCostRouterWithStore(config, store), nil
	case router.StrategyTagBased:
		return newTagBasedRouterWithStore(config, store), nil
	default:
		return nil, fmt.Errorf("unknown routing strategy: %s", config.Strategy)
	}
}

// MustNew creates a new router and panics if the strategy is invalid.
func MustNew(config router.Config) router.Router {
	r, err := New(config)
	if err != nil {
		panic(err)
	}
	return r
}

// AvailableStrategies returns a list of all available routing strategies.
func AvailableStrategies() []router.Strategy {
	return []router.Strategy{
		router.StrategySimpleShuffle,
		router.StrategyLowestLatency,
		router.StrategyLeastBusy,
		router.StrategyLowestTPMRPM,
		router.StrategyLowestCost,
		router.StrategyTagBased,
	}
}

// IsValidStrategy checks if a strategy string is valid.
func IsValidStrategy(s string) bool {
	strategy := router.Strategy(s)
	for _, valid := range AvailableStrategies() {
		if strategy == valid {
			return true
		}
	}
	return false
}
