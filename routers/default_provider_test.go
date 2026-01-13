package routers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

func TestLeastBusyRouter_DefaultProviderPreferred(t *testing.T) {
	config := router.DefaultConfig()
	config.Strategy = router.StrategyLeastBusy
	config.DefaultProvider = "primary"

	r := NewLeastBusyRouterWithConfig(config)

	primary := &provider.Deployment{
		ID:           "primary-gpt-4",
		ModelName:    "gpt-4",
		ProviderName: "primary",
	}
	secondary := &provider.Deployment{
		ID:           "secondary-gpt-4",
		ModelName:    "gpt-4",
		ProviderName: "secondary",
	}
	r.AddDeployment(primary)
	r.AddDeployment(secondary)

	// Make the primary deployment appear busier to prove preference wins.
	r.ReportRequestStart(primary)

	picked, err := r.Pick(context.Background(), "gpt-4")
	require.NoError(t, err)
	assert.Equal(t, primary.ID, picked.ID)
}

func TestShuffleRouter_DefaultProviderFallsBackWhenThrottled(t *testing.T) {
	config := router.DefaultConfig()
	config.Strategy = router.StrategySimpleShuffle
	config.DefaultProvider = "primary"

	r := NewShuffleRouterWithConfig(config)

	primary := &provider.Deployment{
		ID:           "primary-gpt-4",
		ModelName:    "gpt-4",
		ProviderName: "primary",
	}
	secondary := &provider.Deployment{
		ID:           "secondary-gpt-4",
		ModelName:    "gpt-4",
		ProviderName: "secondary",
	}
	r.AddDeploymentWithConfig(primary, router.DeploymentConfig{TPMLimit: 1})
	r.AddDeployment(secondary)

	r.ReportSuccess(primary, &router.ResponseMetrics{
		Latency:     time.Millisecond,
		TotalTokens: 1,
	})

	picked, err := r.PickWithContext(context.Background(), &router.RequestContext{
		Model:                "gpt-4",
		EstimatedInputTokens: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, secondary.ID, picked.ID)
}
