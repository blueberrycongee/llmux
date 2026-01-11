package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/blueberrycongee/llmux/internal/resilience"
)

type mockDistributedLimiter struct {
	checkAllowFunc func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error)
}

func (m *mockDistributedLimiter) CheckAllow(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
	if m.checkAllowFunc != nil {
		return m.checkAllowFunc(ctx, descriptors)
	}
	return nil, nil
}

func TestTenantRateLimiter_Distributed(t *testing.T) {
	called := false
	mockLimiter := &mockDistributedLimiter{
		checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
			called = true
			if len(descriptors) == 0 {
				t.Error("expected descriptors, got empty")
			}
			// Verify descriptor content
			if descriptors[0].Key != "tenant-dist" {
				t.Errorf("expected key tenant-dist, got %s", descriptors[0].Key)
			}

			results := make([]resilience.LimitResult, len(descriptors))
			for i := range descriptors {
				results[i] = resilience.LimitResult{
					Allowed:   true,
					Remaining: 10,
				}
			}
			return results, nil
		},
	}

	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:   60,
		DefaultBurst: 5,
	})

	// Inject the distributed limiter
	trl.SetDistributedLimiter(mockLimiter)

	tenantID := "tenant-dist"
	// Test Check
	allowed, err := trl.Check(context.Background(), tenantID, 60, 5)
	if err != nil {
		t.Errorf("Check returned error: %v", err)
	}
	if !allowed {
		t.Error("Check should succeed with distributed limiter")
	}

	if !called {
		t.Error("DistributedLimiter.CheckAllow was not called")
	}
}

func TestTenantRateLimiter_DistributedFailOpen(t *testing.T) {
	mockLimiter := &mockDistributedLimiter{
		checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
			return nil, errors.New("redis down")
		},
	}

	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:   60,
		DefaultBurst: 5,
		FailOpen:     true,
	})
	trl.SetDistributedLimiter(mockLimiter)

	allowed, err := trl.Check(context.Background(), "tenant-fail-open", 60, 5)
	if err == nil {
		t.Error("expected error on distributed limiter failure")
	}
	if !allowed {
		t.Error("expected allow on fail-open")
	}
}

func TestTenantRateLimiter_DistributedFailClosed(t *testing.T) {
	mockLimiter := &mockDistributedLimiter{
		checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
			return nil, errors.New("redis down")
		},
	}

	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:   60,
		DefaultBurst: 5,
		FailOpen:     false,
	})
	trl.SetDistributedLimiter(mockLimiter)

	allowed, err := trl.Check(context.Background(), "tenant-fail-closed", 60, 5)
	if err == nil {
		t.Error("expected error on distributed limiter failure")
	}
	if allowed {
		t.Error("expected deny on fail-closed")
	}
}

func TestTenantRateLimiter_DistributedFailOpen_EmptyResults(t *testing.T) {
	mockLimiter := &mockDistributedLimiter{
		checkAllowFunc: func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
			return []resilience.LimitResult{}, nil
		},
	}

	trl := NewTenantRateLimiter(&TenantRateLimiterConfig{
		DefaultRPM:   60,
		DefaultBurst: 5,
		FailOpen:     true,
	})
	trl.SetDistributedLimiter(mockLimiter)

	allowed, err := trl.Check(context.Background(), "tenant-empty-results", 60, 5)
	if err == nil {
		t.Error("expected error on empty distributed limiter results")
	}
	if !allowed {
		t.Error("expected allow on fail-open with empty results")
	}
}
