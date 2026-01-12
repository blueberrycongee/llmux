package routers

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
	"github.com/stretchr/testify/require"
)

func TestLatencyRouter_Pick_WeightedBufferPrefersWeight(t *testing.T) {
	cfg := router.DefaultConfig()
	cfg.Strategy = router.StrategyLowestLatency
	cfg.LatencyBuffer = 1.0

	r := NewLatencyRouterWithConfig(cfg)
	r.rng = rand.New(rand.NewSource(1))

	depA := &provider.Deployment{ID: "dep-a", ProviderName: "p1", ModelName: "gpt-4"}
	depB := &provider.Deployment{ID: "dep-b", ProviderName: "p2", ModelName: "gpt-4"}
	r.AddDeploymentWithConfig(depA, router.DeploymentConfig{Weight: 1})
	r.AddDeploymentWithConfig(depB, router.DeploymentConfig{Weight: 0})

	r.ReportSuccess(depA, &router.ResponseMetrics{Latency: 100 * time.Millisecond})
	r.ReportSuccess(depB, &router.ResponseMetrics{Latency: 100 * time.Millisecond})

	for i := 0; i < 10; i++ {
		dep, err := r.Pick(context.Background(), "gpt-4")
		require.NoError(t, err)
		require.Equal(t, "dep-a", dep.ID)
	}
}
