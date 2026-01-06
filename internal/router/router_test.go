package router

import (
	"context"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/provider"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
)

// Helper to create test deployments
func createTestDeployments(count int) []*provider.Deployment {
	deployments := make([]*provider.Deployment, count)
	for i := 0; i < count; i++ {
		deployments[i] = &provider.Deployment{
			ID:           string(rune('a' + i)),
			ProviderName: "test",
			ModelName:    "gpt-4",
		}
	}
	return deployments
}

func TestSimpleShuffleRouter_Pick(t *testing.T) {
	config := DefaultRouterConfig()
	config.Strategy = StrategySimpleShuffle
	router := NewSimpleShuffleRouter(config)

	deployments := createTestDeployments(3)
	for _, d := range deployments {
		router.AddDeployment(d)
	}

	ctx := context.Background()

	// Should pick a deployment
	picked, err := router.Pick(ctx, "gpt-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if picked == nil {
		t.Fatal("expected deployment, got nil")
	}

	// Should return error for unknown model
	_, err = router.Pick(ctx, "unknown-model")
	if err != ErrNoAvailableDeployment {
		t.Errorf("expected ErrNoAvailableDeployment, got %v", err)
	}
}

func TestSimpleShuffleRouter_WeightedPick(t *testing.T) {
	config := DefaultRouterConfig()
	router := NewSimpleShuffleRouter(config)

	// Add deployments with weights
	d1 := &provider.Deployment{ID: "a", ModelName: "gpt-4"}
	d2 := &provider.Deployment{ID: "b", ModelName: "gpt-4"}
	router.AddDeploymentWithConfig(d1, DeploymentConfig{Weight: 0.9})
	router.AddDeploymentWithConfig(d2, DeploymentConfig{Weight: 0.1})

	ctx := context.Background()
	counts := make(map[string]int)

	// Pick many times and count distribution
	for i := 0; i < 1000; i++ {
		picked, _ := router.Pick(ctx, "gpt-4")
		counts[picked.ID]++
	}

	// d1 should be picked significantly more often
	if counts["a"] < counts["b"]*2 {
		t.Errorf("weighted selection not working: a=%d, b=%d", counts["a"], counts["b"])
	}
}

func TestLowestLatencyRouter_Pick(t *testing.T) {
	config := DefaultRouterConfig()
	config.Strategy = StrategyLowestLatency
	config.LatencyBuffer = 0.1
	router := NewLowestLatencyRouter(config)

	deployments := createTestDeployments(3)
	for _, d := range deployments {
		router.AddDeployment(d)
	}

	ctx := context.Background()

	// Report different latencies
	router.ReportSuccess(deployments[0], &ResponseMetrics{Latency: 100 * time.Millisecond})
	router.ReportSuccess(deployments[1], &ResponseMetrics{Latency: 50 * time.Millisecond})
	router.ReportSuccess(deployments[2], &ResponseMetrics{Latency: 200 * time.Millisecond})

	// Should prefer deployment with lowest latency
	counts := make(map[string]int)
	for i := 0; i < 100; i++ {
		picked, _ := router.Pick(ctx, "gpt-4")
		counts[picked.ID]++
	}

	// Deployment "b" (50ms) should be picked most often
	if counts["b"] < counts["a"] || counts["b"] < counts["c"] {
		t.Errorf("lowest latency not preferred: a=%d, b=%d, c=%d", counts["a"], counts["b"], counts["c"])
	}
}

func TestLowestLatencyRouter_StreamingTTFT(t *testing.T) {
	config := DefaultRouterConfig()
	router := NewLowestLatencyRouter(config)

	deployments := createTestDeployments(2)
	for _, d := range deployments {
		router.AddDeployment(d)
	}

	// d1: high latency but low TTFT
	// d2: low latency but high TTFT
	router.ReportSuccess(deployments[0], &ResponseMetrics{
		Latency:          500 * time.Millisecond,
		TimeToFirstToken: 50 * time.Millisecond,
	})
	router.ReportSuccess(deployments[1], &ResponseMetrics{
		Latency:          100 * time.Millisecond,
		TimeToFirstToken: 200 * time.Millisecond,
	})

	ctx := context.Background()

	// For streaming requests, should prefer d1 (lower TTFT)
	counts := make(map[string]int)
	for i := 0; i < 100; i++ {
		picked, _ := router.PickWithContext(ctx, &RequestContext{
			Model:       "gpt-4",
			IsStreaming: true,
		})
		counts[picked.ID]++
	}

	if counts["a"] < counts["b"] {
		t.Errorf("TTFT not preferred for streaming: a=%d, b=%d", counts["a"], counts["b"])
	}
}

func TestLeastBusyRouter_Pick(t *testing.T) {
	config := DefaultRouterConfig()
	router := NewLeastBusyRouter(config)

	deployments := createTestDeployments(3)
	for _, d := range deployments {
		router.AddDeployment(d)
	}

	// Simulate active requests
	router.ReportRequestStart(deployments[0])
	router.ReportRequestStart(deployments[0])
	router.ReportRequestStart(deployments[1])
	// deployments[2] has 0 active requests

	ctx := context.Background()

	// Should always pick deployment with fewest active requests
	for i := 0; i < 10; i++ {
		picked, _ := router.Pick(ctx, "gpt-4")
		if picked.ID != "c" {
			t.Errorf("expected deployment c (0 active), got %s", picked.ID)
		}
	}
}

func TestLowestTPMRPMRouter_Pick(t *testing.T) {
	config := DefaultRouterConfig()
	router := NewLowestTPMRPMRouter(config)

	d1 := &provider.Deployment{ID: "a", ModelName: "gpt-4"}
	d2 := &provider.Deployment{ID: "b", ModelName: "gpt-4"}
	router.AddDeploymentWithConfig(d1, DeploymentConfig{TPMLimit: 1000, RPMLimit: 10})
	router.AddDeploymentWithConfig(d2, DeploymentConfig{TPMLimit: 1000, RPMLimit: 10})

	// Simulate usage on d1
	router.ReportSuccess(d1, &ResponseMetrics{TotalTokens: 500})
	router.ReportSuccess(d1, &ResponseMetrics{TotalTokens: 300})

	ctx := context.Background()

	// Should prefer d2 (lower usage)
	picked, _ := router.Pick(ctx, "gpt-4")
	if picked.ID != "b" {
		t.Errorf("expected deployment b (lower usage), got %s", picked.ID)
	}
}

func TestLowestTPMRPMRouter_RespectLimits(t *testing.T) {
	config := DefaultRouterConfig()
	router := NewLowestTPMRPMRouter(config)

	d1 := &provider.Deployment{ID: "a", ModelName: "gpt-4"}
	router.AddDeploymentWithConfig(d1, DeploymentConfig{TPMLimit: 100, RPMLimit: 5})

	// Exhaust the TPM limit
	router.ReportSuccess(d1, &ResponseMetrics{TotalTokens: 100})

	ctx := context.Background()
	reqCtx := &RequestContext{
		Model:                "gpt-4",
		EstimatedInputTokens: 50,
	}

	// Should return error when limit exceeded
	_, err := router.PickWithContext(ctx, reqCtx)
	if err != ErrNoAvailableDeployment {
		t.Errorf("expected ErrNoAvailableDeployment, got %v", err)
	}
}

func TestLowestCostRouter_Pick(t *testing.T) {
	config := DefaultRouterConfig()
	router := NewLowestCostRouter(config)

	d1 := &provider.Deployment{ID: "expensive", ModelName: "gpt-4"}
	d2 := &provider.Deployment{ID: "cheap", ModelName: "gpt-4"}
	router.AddDeploymentWithConfig(d1, DeploymentConfig{
		InputCostPerToken:  0.01,
		OutputCostPerToken: 0.03,
	})
	router.AddDeploymentWithConfig(d2, DeploymentConfig{
		InputCostPerToken:  0.001,
		OutputCostPerToken: 0.002,
	})

	ctx := context.Background()

	// Should always pick cheaper deployment
	for i := 0; i < 10; i++ {
		picked, _ := router.Pick(ctx, "gpt-4")
		if picked.ID != "cheap" {
			t.Errorf("expected cheap deployment, got %s", picked.ID)
		}
	}
}

func TestTagBasedRouter_Pick(t *testing.T) {
	config := DefaultRouterConfig()
	router := NewTagBasedRouter(config)

	d1 := &provider.Deployment{ID: "premium", ModelName: "gpt-4"}
	d2 := &provider.Deployment{ID: "standard", ModelName: "gpt-4"}
	d3 := &provider.Deployment{ID: "default", ModelName: "gpt-4"}
	router.AddDeploymentWithConfig(d1, DeploymentConfig{Tags: []string{"premium", "fast"}})
	router.AddDeploymentWithConfig(d2, DeploymentConfig{Tags: []string{"standard"}})
	router.AddDeploymentWithConfig(d3, DeploymentConfig{Tags: []string{"default"}})

	ctx := context.Background()

	// Request with "premium" tag should only get premium deployment
	picked, _ := router.PickWithContext(ctx, &RequestContext{
		Model: "gpt-4",
		Tags:  []string{"premium"},
	})
	if picked.ID != "premium" {
		t.Errorf("expected premium deployment, got %s", picked.ID)
	}

	// Request with unknown tag should fall back to default
	picked, _ = router.PickWithContext(ctx, &RequestContext{
		Model: "gpt-4",
		Tags:  []string{"unknown"},
	})
	if picked.ID != "default" {
		t.Errorf("expected default deployment, got %s", picked.ID)
	}
}

func TestTagBasedRouter_NoMatchingTags(t *testing.T) {
	config := DefaultRouterConfig()
	router := NewTagBasedRouter(config)

	d1 := &provider.Deployment{ID: "premium", ModelName: "gpt-4"}
	router.AddDeploymentWithConfig(d1, DeploymentConfig{Tags: []string{"premium"}})

	ctx := context.Background()

	// Request with non-matching tag and no default should fail
	_, err := router.PickWithContext(ctx, &RequestContext{
		Model: "gpt-4",
		Tags:  []string{"standard"},
	})
	if err != ErrNoDeploymentsWithTag {
		t.Errorf("expected ErrNoDeploymentsWithTag, got %v", err)
	}
}

func TestRouter_Cooldown(t *testing.T) {
	config := DefaultRouterConfig()
	config.CooldownPeriod = 100 * time.Millisecond
	router := NewSimpleShuffleRouter(config)

	d := &provider.Deployment{ID: "a", ModelName: "gpt-4"}
	router.AddDeployment(d)

	// Trigger cooldown with a rate limit error
	rateLimitErr := llmerrors.NewRateLimitError("test", "gpt-4", "rate limited")
	router.ReportFailure(d, rateLimitErr)

	// Should be in cooldown
	if !router.IsCircuitOpen(d) {
		t.Error("expected circuit to be open")
	}

	ctx := context.Background()
	_, err := router.Pick(ctx, "gpt-4")
	if err != ErrNoAvailableDeployment {
		t.Errorf("expected ErrNoAvailableDeployment during cooldown, got %v", err)
	}

	// Wait for cooldown to expire
	time.Sleep(150 * time.Millisecond)

	// Should be available again
	if router.IsCircuitOpen(d) {
		t.Error("expected circuit to be closed after cooldown")
	}

	picked, err := router.Pick(ctx, "gpt-4")
	if err != nil {
		t.Errorf("unexpected error after cooldown: %v", err)
	}
	if picked.ID != "a" {
		t.Errorf("expected deployment a, got %s", picked.ID)
	}
}

func TestFactory_New(t *testing.T) {
	strategies := AvailableStrategies()

	for _, strategy := range strategies {
		config := RouterConfig{Strategy: strategy}
		router, err := New(config)
		if err != nil {
			t.Errorf("failed to create router for strategy %s: %v", strategy, err)
		}
		if router.GetStrategy() != strategy {
			t.Errorf("expected strategy %s, got %s", strategy, router.GetStrategy())
		}
	}
}

func TestFactory_InvalidStrategy(t *testing.T) {
	config := RouterConfig{Strategy: "invalid-strategy"}
	_, err := New(config)
	if err == nil {
		t.Error("expected error for invalid strategy")
	}
}

func TestIsValidStrategy(t *testing.T) {
	if !IsValidStrategy("simple-shuffle") {
		t.Error("simple-shuffle should be valid")
	}
	if !IsValidStrategy("lowest-latency") {
		t.Error("lowest-latency should be valid")
	}
	if IsValidStrategy("invalid") {
		t.Error("invalid should not be valid")
	}
}
