package router

import "fmt"

// New creates a new router based on the specified strategy.
// Returns an error if the strategy is not recognized.
func New(config RouterConfig) (Router, error) {
	switch config.Strategy {
	case StrategySimpleShuffle, "":
		return NewSimpleShuffleRouter(config), nil
	case StrategyLowestLatency:
		return NewLowestLatencyRouter(config), nil
	case StrategyLeastBusy:
		return NewLeastBusyRouter(config), nil
	case StrategyLowestTPMRPM:
		return NewLowestTPMRPMRouter(config), nil
	case StrategyLowestCost:
		return NewLowestCostRouter(config), nil
	case StrategyTagBased:
		return NewTagBasedRouter(config), nil
	default:
		return nil, fmt.Errorf("unknown routing strategy: %s", config.Strategy)
	}
}

// MustNew creates a new router and panics if the strategy is invalid.
// Use this only when you're certain the strategy is valid.
func MustNew(config RouterConfig) Router {
	r, err := New(config)
	if err != nil {
		panic(err)
	}
	return r
}

// AvailableStrategies returns a list of all available routing strategies.
func AvailableStrategies() []Strategy {
	return []Strategy{
		StrategySimpleShuffle,
		StrategyLowestLatency,
		StrategyLeastBusy,
		StrategyLowestTPMRPM,
		StrategyLowestCost,
		StrategyTagBased,
	}
}

// IsValidStrategy checks if a strategy string is valid.
func IsValidStrategy(s string) bool {
	strategy := Strategy(s)
	for _, valid := range AvailableStrategies() {
		if strategy == valid {
			return true
		}
	}
	return false
}
