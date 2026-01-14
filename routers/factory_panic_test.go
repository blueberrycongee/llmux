package routers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/blueberrycongee/llmux/pkg/router"
)

func TestNewWithStores_LowestCost_InvalidPricingFile_ReturnsError(t *testing.T) {
	cfg := router.Config{
		Strategy:    router.StrategyLowestCost,
		PricingFile: "this-file-should-not-exist.json",
	}

	assert.NotPanics(t, func() {
		r, err := NewWithStores(cfg, nil, nil)
		assert.Error(t, err)
		assert.Nil(t, r)
	})
}
