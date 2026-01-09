package llmux_test

import (
	"context"
	"testing"

	"github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/resilience"
	"github.com/stretchr/testify/assert"
)

// MockDistributedLimiter is a mock implementation of resilience.DistributedLimiter
type MockDistributedLimiter struct{}

func (m *MockDistributedLimiter) CheckAllow(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
	return nil, nil
}

func TestWithRateLimiter_Applied(t *testing.T) {
	limiter := &MockDistributedLimiter{}

	// We need to construct a config.
	// Since `defaultConfig` is unexported, we can just use a zero value struct since we are testing the Option application.
	cfg := &llmux.ClientConfig{}

	opt := llmux.WithRateLimiter(limiter)
	opt(cfg)

	assert.Equal(t, limiter, cfg.RateLimiter)
}
