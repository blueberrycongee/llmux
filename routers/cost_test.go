package routers

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

func TestCostRouter_Pick_WithRegistry(t *testing.T) {
	// This test verifies that the CostRouter uses the PriceRegistry to fetch prices
	// when they are not explicitly configured.

	r := NewCostRouter()

	// Deployment A: OpenAI gpt-4o. No explicit cost.
	// Should fetch from registry: Input 0.000005, Output 0.000015 -> Total 0.000020
	depA := &provider.Deployment{
		ID:           "dep-a",
		ModelName:    "gpt-4o",
		ProviderName: "openai",
	}
	r.AddDeployment(depA)

	// Deployment B: Custom provider. Explicit cost set to 1.0.
	// 1.0 is much higher than 0.000020, but lower than the default 5.0.
	depB := &provider.Deployment{
		ID:           "dep-b",
		ModelName:    "gpt-4o",
		ProviderName: "custom",
	}
	configB := router.DeploymentConfig{
		InputCostPerToken:  0.5,
		OutputCostPerToken: 0.5,
	}
	r.AddDeploymentWithConfig(depB, configB)

	// Current behavior (without registry):
	// depA: defaults to 5.0 + 5.0 = 10.0
	// depB: 0.5 + 0.5 = 1.0
	// Router picks depB.

	// Desired behavior (with registry):
	// depA: 0.000005 + 0.000015 = 0.000020
	// depB: 1.0
	// Router picks depA.

	ctx := context.Background()
	picked, err := r.Pick(ctx, "gpt-4o")
	assert.NoError(t, err)

	// We expect depA to be picked because it's cheaper in reality.
	// If the registry is not working, depB will be picked (1.0 < 10.0).
	assert.Equal(t, depA.ID, picked.ID, "Should pick depA (real cost ~0.00002) over depB (manual cost 1.0)")
}

func TestCostRouter_WithPricingFile(t *testing.T) {
	// Create a temporary pricing file
	pricingContent := `
    {
        "custom-model": {
            "provider": "custom",
            "input_cost_per_token": 0.000001,
            "output_cost_per_token": 0.000001,
            "mode": "chat"
        }
    }`
	tmpfile, err := os.CreateTemp("", "pricing_*.json")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())
	_, err = tmpfile.Write([]byte(pricingContent))
	assert.NoError(t, err)
	tmpfile.Close()

	// Create router with config pointing to this file
	config := router.Config{
		Strategy:    router.StrategyLowestCost,
		PricingFile: tmpfile.Name(),
	}
	r := NewCostRouterWithConfig(config)

	// Add deployment for custom-model
	dep := &provider.Deployment{
		ID:           "dep-custom",
		ModelName:    "custom-model",
		ProviderName: "custom",
	}
	r.AddDeployment(dep)

	// Add another deployment with higher cost
	depExpensive := &provider.Deployment{
		ID:           "dep-expensive",
		ModelName:    "custom-model",
		ProviderName: "expensive",
	}
	// Set explicit cost for expensive one
	r.AddDeploymentWithConfig(depExpensive, router.DeploymentConfig{
		InputCostPerToken:  1.0,
		OutputCostPerToken: 1.0,
	})

	ctx := context.Background()
	picked, err := r.Pick(ctx, "custom-model")
	assert.NoError(t, err)
	assert.Equal(t, dep.ID, picked.ID)
}
