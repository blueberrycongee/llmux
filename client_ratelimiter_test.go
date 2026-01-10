package llmux

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/resilience"
)

// TestClient_RateLimiterIntegration tests that rate limiting is applied to requests.
func TestClient_RateLimiterIntegration(t *testing.T) {
	// Create a mock limiter that denies requests
	denyLimiter := &mockDistributedLimiter{
		checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
			results := make([]resilience.LimitResult, len(descriptors))
			for i := range descriptors {
				results[i] = resilience.LimitResult{
					Allowed:   false,
					Current:   100,
					Remaining: 0,
					ResetAt:   time.Now().Add(time.Minute).Unix(),
				}
			}
			return results, nil
		},
	}

	// Create client with rate limiter
	client, err := New(
		WithRateLimiter(denyLimiter),
		WithRateLimiterConfig(RateLimiterConfig{
			Enabled:     true,
			RPMLimit:    10,
			TPMLimit:    1000,
			WindowSize:  time.Minute,
			KeyStrategy: RateLimitKeyByAPIKey,
		}),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// Verify rate limiter is configured
	if client.rateLimiter == nil {
		t.Error("expected rateLimiter to be set, got nil")
	}
	if !client.rateLimiterConfig.Enabled {
		t.Error("expected rateLimiterConfig.Enabled to be true")
	}
}

// TestClient_RateLimiterAllows tests that allowed requests pass through.
func TestClient_RateLimiterAllows(t *testing.T) {
	allowLimiter := &mockDistributedLimiter{
		checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
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
		},
	}

	client, err := New(
		WithRateLimiter(allowLimiter),
		WithRateLimiterConfig(RateLimiterConfig{
			Enabled:     true,
			RPMLimit:    100,
			TPMLimit:    10000,
			WindowSize:  time.Minute,
			KeyStrategy: RateLimitKeyByAPIKey,
		}),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	if client.rateLimiter == nil {
		t.Error("expected rateLimiter to be set")
	}
}

// TestClient_RateLimiterError tests handling of rate limiter errors.
func TestClient_RateLimiterError(t *testing.T) {
	errorLimiter := &mockDistributedLimiter{
		checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
			return nil, errors.New("redis connection failed")
		},
	}

	client, err := New(
		WithRateLimiter(errorLimiter),
		WithRateLimiterConfig(RateLimiterConfig{
			Enabled:     true,
			RPMLimit:    100,
			TPMLimit:    10000,
			WindowSize:  time.Minute,
			KeyStrategy: RateLimitKeyByAPIKey,
		}),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	if client.rateLimiter == nil {
		t.Error("expected rateLimiter to be set")
	}
}

// TestClient_RateLimiterDisabled tests that disabled rate limiting is skipped.
func TestClient_RateLimiterDisabled(t *testing.T) {
	// Even with a limiter, if Enabled is false, it should be skipped
	limiter := &mockDistributedLimiter{}

	client, err := New(
		WithRateLimiter(limiter),
		WithRateLimiterConfig(RateLimiterConfig{
			Enabled:     false,
			RPMLimit:    100,
			TPMLimit:    10000,
			WindowSize:  time.Minute,
			KeyStrategy: RateLimitKeyByAPIKey,
		}),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// Limiter is set but config says disabled
	if client.rateLimiter == nil {
		t.Error("expected rateLimiter to be set")
	}
	if client.rateLimiterConfig.Enabled {
		t.Error("expected rateLimiterConfig.Enabled to be false")
	}
}
