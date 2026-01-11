// Package routers provides public router implementations for LLMux library mode.
// All routers implement the router.Router interface from pkg/router.
package routers

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

// Re-export types from pkg/router for convenience
type (
	Config           = router.Config
	DeploymentConfig = router.DeploymentConfig
	DeploymentStats  = router.DeploymentStats
	RequestContext   = router.RequestContext
	ResponseMetrics  = router.ResponseMetrics
	Strategy         = router.Strategy
	StatsStore       = router.StatsStore
)

// Re-export error variables
var (
	ErrStatsNotFound     = router.ErrStatsNotFound
	ErrStoreNotAvailable = router.ErrStoreNotAvailable
)

// Re-export constants
const (
	StrategyRoundRobin    = router.StrategyRoundRobin
	StrategySimpleShuffle = router.StrategySimpleShuffle
	StrategyLowestLatency = router.StrategyLowestLatency
	StrategyLeastBusy     = router.StrategyLeastBusy
	StrategyLowestTPMRPM  = router.StrategyLowestTPMRPM
	StrategyLowestCost    = router.StrategyLowestCost
	StrategyTagBased      = router.StrategyTagBased
)

// Re-export functions
var DefaultConfig = router.DefaultConfig

// ExtendedDeployment wraps a deployment with routing-specific configuration.
type ExtendedDeployment struct {
	*provider.Deployment
	Config router.DeploymentConfig
}
