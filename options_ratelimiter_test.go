package llmux

import (
	"context"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/resilience"
)

// TestWithRateLimiter tests that the WithRateLimiter option correctly sets the rate limiter.
func TestWithRateLimiter(t *testing.T) {
	// Create a mock rate limiter
	mockLimiter := &mockDistributedLimiter{}

	// Apply option
	cfg := defaultConfig()
	opt := WithRateLimiter(mockLimiter)
	opt(cfg)

	// Verify
	if cfg.RateLimiter == nil {
		t.Error("expected RateLimiter to be set, got nil")
	}
}

// TestWithRateLimiterConfig tests the WithRateLimiterConfig option.
func TestWithRateLimiterConfig(t *testing.T) {
	cfg := defaultConfig()

	rateLimiterConfig := RateLimiterConfig{
		Enabled:     true,
		RPMLimit:    100,
		TPMLimit:    10000,
		WindowSize:  time.Minute,
		KeyStrategy: RateLimitKeyByAPIKey,
	}

	opt := WithRateLimiterConfig(rateLimiterConfig)
	opt(cfg)

	if !cfg.RateLimiterConfig.Enabled {
		t.Error("expected RateLimiterConfig.Enabled to be true")
	}
	if cfg.RateLimiterConfig.RPMLimit != 100 {
		t.Errorf("expected RPMLimit 100, got %d", cfg.RateLimiterConfig.RPMLimit)
	}
	if cfg.RateLimiterConfig.TPMLimit != 10000 {
		t.Errorf("expected TPMLimit 10000, got %d", cfg.RateLimiterConfig.TPMLimit)
	}
	if cfg.RateLimiterConfig.KeyStrategy != RateLimitKeyByAPIKey {
		t.Errorf("expected KeyStrategy RateLimitKeyByAPIKey, got %v", cfg.RateLimiterConfig.KeyStrategy)
	}
}

// mockDistributedLimiter is a mock implementation of DistributedLimiter for testing.
type mockDistributedLimiter struct {
	checkAllowFunc func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error)
}

func (m *mockDistributedLimiter) CheckAllow(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
	if m.checkAllowFunc != nil {
		return m.checkAllowFunc(ctx, descriptors)
	}
	// Default: allow all
	results := make([]resilience.LimitResult, len(descriptors))
	for i, desc := range descriptors {
		results[i] = resilience.LimitResult{
			Allowed:   true,
			Current:   1,
			Remaining: desc.Limit - 1,
			ResetAt:   time.Now().Add(time.Minute).Unix(),
		}
	}
	return results, nil
}
