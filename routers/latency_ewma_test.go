package routers

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

func TestLatencyRouter_EWMADynamicWeighting(t *testing.T) {
	cfg := router.DefaultConfig()
	cfg.Strategy = router.StrategyLowestLatency
	cfg.LatencyBuffer = 10.0           // Very large buffer to ensure both are candidates
	cfg.EWMAAlpha = 0.5                // Fast smoothing factor for test
	cfg.FailureThresholdPercent = 1.0  // Disable circuit breaker for this test
	cfg.MinRequestsForThreshold = 1000 // Disable circuit breaker for this test

	r := NewLatencyRouterWithConfig(cfg)

	depA := &provider.Deployment{ID: "dep-a", ProviderName: "p1", ModelName: "gpt-4"}
	depB := &provider.Deployment{ID: "dep-b", ProviderName: "p2", ModelName: "gpt-4"}

	r.AddDeployment(depA)
	r.AddDeployment(depB)

	ctx := context.Background()

	// 1. Test Latency-based weighting
	// dep-a: 100ms, dep-b: 400ms. Both 100% success rate.
	for i := 0; i < 20; i++ {
		r.ReportSuccess(ctx, depA, &router.ResponseMetrics{Latency: 100 * time.Millisecond})
		r.ReportSuccess(ctx, depB, &router.ResponseMetrics{Latency: 400 * time.Millisecond})
	}

	counts := make(map[string]int)
	for i := 0; i < 1000; i++ {
		dep, err := r.Pick(ctx, "gpt-4")
		require.NoError(t, err)
		counts[dep.ID]++
	}
	// dep-a should have roughly 4x weight of dep-b (1/100 vs 1/400)
	t.Logf("Counts after latency test: A=%d, B=%d", counts["dep-a"], counts["dep-b"])
	require.Greater(t, counts["dep-a"], counts["dep-b"])
	require.Greater(t, counts["dep-a"], 600) // dep-a should get majority

	// 2. Test Success Rate-based weighting
	// Now make dep-a start failing, while dep-b remains stable.
	for i := 0; i < 20; i++ {
		r.ReportFailure(ctx, depA, nil)
		r.ReportSuccess(ctx, depB, &router.ResponseMetrics{Latency: 400 * time.Millisecond})
	}

	counts = make(map[string]int)
	for i := 0; i < 1000; i++ {
		dep, err := r.Pick(ctx, "gpt-4")
		require.NoError(t, err)
		counts[dep.ID]++
	}
	t.Logf("Counts after failure test: A=%d, B=%d", counts["dep-a"], counts["dep-b"])
	// Even though dep-a has lower latency, its success rate drop should shift weight to dep-b
	require.Greater(t, counts["dep-b"], counts["dep-a"])
}

func TestLatencyRouter_HighConcurrencyEWMA(t *testing.T) {
	cfg := router.DefaultConfig()
	cfg.Strategy = router.StrategyLowestLatency
	cfg.LatencyBuffer = 1.0
	cfg.EWMAAlpha = 0.1
	cfg.FailureThresholdPercent = 1.0
	cfg.MinRequestsForThreshold = 10000

	r := NewLatencyRouterWithConfig(cfg)

	depA := &provider.Deployment{ID: "dep-a", ProviderName: "p1", ModelName: "gpt-4"}
	depB := &provider.Deployment{ID: "dep-b", ProviderName: "p2", ModelName: "gpt-4"}

	r.AddDeployment(depA)
	r.AddDeployment(depB)

	ctx := context.Background()
	var wg sync.WaitGroup

	// Simulate concurrent requests and metrics reporting
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = r.Pick(ctx, "gpt-4")
				r.ReportSuccess(ctx, depA, &router.ResponseMetrics{Latency: 200 * time.Millisecond})
				r.ReportSuccess(ctx, depB, &router.ResponseMetrics{Latency: 200 * time.Millisecond})
			}
		}()
	}
	wg.Wait()

	// Verify stats are updated correctly
	statsA := r.GetStats("dep-a")
	statsB := r.GetStats("dep-b")
	require.NotNil(t, statsA)
	require.NotNil(t, statsB)
	require.InDelta(t, 200.0, statsA.EWMALatencyMs, 1.0)
	require.InDelta(t, 200.0, statsB.EWMALatencyMs, 1.0)
	require.InDelta(t, 1.0, statsA.EWMASuccessRate, 0.01)
	require.InDelta(t, 1.0, statsB.EWMASuccessRate, 0.01)
}
