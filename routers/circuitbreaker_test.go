package routers_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
	"github.com/blueberrycongee/llmux/routers"
)

// =============================================================================
// Circuit Breaker / Cooldown Tests (LiteLLM-style failure rate logic)
// =============================================================================

func TestBaseRouter_CooldownOn429(t *testing.T) {
	// 429 Rate Limit errors should trigger immediate cooldown
	config := router.DefaultConfig()
	config.CooldownPeriod = 5 * time.Minute
	config.ImmediateCooldownOn429 = true
	r := routers.NewBaseRouter(config)

	deployment := &provider.Deployment{ID: "test-deployment", ModelName: "gpt-4"}
	secondary := &provider.Deployment{ID: "test-deployment-2", ModelName: "gpt-4"}
	r.AddDeployment(deployment)
	r.AddDeployment(secondary)

	// Report 429 error
	err := llmerrors.NewRateLimitError("openai", "gpt-4", "rate limited")
	r.ReportFailure(context.Background(), deployment, err)

	// Should be in cooldown
	assert.True(t, r.IsCircuitOpen(deployment), "Circuit should be open after 429 error")
}

func TestBaseRouter_CooldownOn429_Disabled(t *testing.T) {
	// When ImmediateCooldownOn429 is false, 429 should not trigger immediate cooldown
	config := router.DefaultConfig()
	config.CooldownPeriod = 5 * time.Minute
	config.ImmediateCooldownOn429 = false
	config.MinRequestsForThreshold = 10 // High threshold to prevent rate-based cooldown
	r := routers.NewBaseRouter(config)

	deployment := &provider.Deployment{ID: "test-deployment", ModelName: "gpt-4"}
	secondary := &provider.Deployment{ID: "test-deployment-2", ModelName: "gpt-4"}
	r.AddDeployment(deployment)
	r.AddDeployment(secondary)

	// Report 429 error
	err := llmerrors.NewRateLimitError("openai", "gpt-4", "rate limited")
	r.ReportFailure(context.Background(), deployment, err)

	// Should NOT be in cooldown (not enough requests for rate-based cooldown)
	assert.False(t, r.IsCircuitOpen(deployment), "Circuit should not be open when ImmediateCooldownOn429 is disabled")
}

func TestBaseRouter_CooldownOn401(t *testing.T) {
	// 401 Auth errors should trigger immediate cooldown
	config := router.DefaultConfig()
	config.CooldownPeriod = 5 * time.Minute
	r := routers.NewBaseRouter(config)

	deployment := &provider.Deployment{ID: "test-deployment", ModelName: "gpt-4"}
	secondary := &provider.Deployment{ID: "test-deployment-2", ModelName: "gpt-4"}
	r.AddDeployment(deployment)
	r.AddDeployment(secondary)

	// Report 401 error
	err := llmerrors.NewAuthenticationError("openai", "gpt-4", "unauthorized")
	r.ReportFailure(context.Background(), deployment, err)

	// Should be in cooldown
	assert.True(t, r.IsCircuitOpen(deployment), "Circuit should be open after 401 error")
}

func TestBaseRouter_CooldownOn404(t *testing.T) {
	// 404 Not Found errors should trigger immediate cooldown
	config := router.DefaultConfig()
	config.CooldownPeriod = 5 * time.Minute
	r := routers.NewBaseRouter(config)

	deployment := &provider.Deployment{ID: "test-deployment", ModelName: "gpt-4"}
	secondary := &provider.Deployment{ID: "test-deployment-2", ModelName: "gpt-4"}
	r.AddDeployment(deployment)
	r.AddDeployment(secondary)

	// Report 404 error
	err := llmerrors.NewNotFoundError("openai", "gpt-4", "not found")
	r.ReportFailure(context.Background(), deployment, err)

	// Should be in cooldown
	assert.True(t, r.IsCircuitOpen(deployment), "Circuit should be open after 404 error")
}

func TestBaseRouter_FailureRateThreshold(t *testing.T) {
	// Failure rate > 50% should trigger cooldown (after minimum requests)
	config := router.DefaultConfig()
	config.CooldownPeriod = 5 * time.Minute
	config.FailureThresholdPercent = 0.5 // 50%
	config.MinRequestsForThreshold = 5
	r := routers.NewBaseRouter(config)

	deployment := &provider.Deployment{ID: "test-deployment", ModelName: "gpt-4"}
	secondary := &provider.Deployment{ID: "test-deployment-2", ModelName: "gpt-4"}
	r.AddDeployment(deployment)
	r.AddDeployment(secondary)

	// Report 1 success and 4 failures (5 total, 80% failure rate)
	r.ReportSuccess(context.Background(), deployment, &router.ResponseMetrics{Latency: 100 * time.Millisecond})
	for i := 0; i < 4; i++ {
		err := llmerrors.NewInternalError("openai", "gpt-4", "server error")
		r.ReportFailure(context.Background(), deployment, err)
	}

	// Should be in cooldown (failure rate 80% > 50%)
	assert.True(t, r.IsCircuitOpen(deployment), "Circuit should be open when failure rate exceeds threshold")
}

func TestBaseRouter_FailureRateMinRequests(t *testing.T) {
	// Should NOT cooldown if request count < MinRequestsForThreshold
	config := router.DefaultConfig()
	config.CooldownPeriod = 5 * time.Minute
	config.FailureThresholdPercent = 0.5 // 50%
	config.MinRequestsForThreshold = 10  // Require 10 requests
	r := routers.NewBaseRouter(config)

	deployment := &provider.Deployment{ID: "test-deployment", ModelName: "gpt-4"}
	secondary := &provider.Deployment{ID: "test-deployment-2", ModelName: "gpt-4"}
	r.AddDeployment(deployment)
	r.AddDeployment(secondary)

	// Report 5 failures (100% failure rate, but only 5 requests)
	for i := 0; i < 5; i++ {
		err := llmerrors.NewInternalError("openai", "gpt-4", "server error")
		r.ReportFailure(context.Background(), deployment, err)
	}

	// Should NOT be in cooldown (not enough requests)
	assert.False(t, r.IsCircuitOpen(deployment), "Circuit should not be open when request count < MinRequestsForThreshold")
}

func TestBaseRouter_FailureRateBelowThreshold(t *testing.T) {
	// Failure rate <= 50% should NOT trigger cooldown
	config := router.DefaultConfig()
	config.CooldownPeriod = 5 * time.Minute
	config.FailureThresholdPercent = 0.5 // 50%
	config.MinRequestsForThreshold = 5
	r := routers.NewBaseRouter(config)

	deployment := &provider.Deployment{ID: "test-deployment", ModelName: "gpt-4"}
	r.AddDeployment(deployment)

	// Report 3 successes and 2 failures (5 total, 40% failure rate)
	for i := 0; i < 3; i++ {
		r.ReportSuccess(context.Background(), deployment, &router.ResponseMetrics{Latency: 100 * time.Millisecond})
	}
	for i := 0; i < 2; i++ {
		err := llmerrors.NewInternalError("openai", "gpt-4", "server error")
		r.ReportFailure(context.Background(), deployment, err)
	}

	// Should NOT be in cooldown (failure rate 40% < 50%)
	assert.False(t, r.IsCircuitOpen(deployment), "Circuit should not be open when failure rate is below threshold")
}

func TestBaseRouter_CooldownRecovery(t *testing.T) {
	// Deployment should recover after cooldown period expires
	config := router.DefaultConfig()
	config.CooldownPeriod = 10 * time.Millisecond // Very short for testing
	r := routers.NewBaseRouter(config)

	deployment := &provider.Deployment{ID: "test-deployment", ModelName: "gpt-4"}
	secondary := &provider.Deployment{ID: "test-deployment-2", ModelName: "gpt-4"}
	r.AddDeployment(deployment)
	r.AddDeployment(secondary)

	// Trigger cooldown
	err := llmerrors.NewRateLimitError("openai", "gpt-4", "rate limited")
	r.ReportFailure(context.Background(), deployment, err)

	// Should be in cooldown
	require.True(t, r.IsCircuitOpen(deployment), "Circuit should be open immediately after error")

	// Wait for cooldown to expire
	time.Sleep(20 * time.Millisecond)

	// Should have recovered
	assert.False(t, r.IsCircuitOpen(deployment), "Circuit should be closed after cooldown expires")
}
