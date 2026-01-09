package resilience_test

import (
	"context"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/resilience"
	"github.com/stretchr/testify/assert"
)

// MockDistributedLimiter is a mock implementation of DistributedLimiter for testing purposes.
type MockDistributedLimiter struct {
	CheckAllowFunc func(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error)
}

func (m *MockDistributedLimiter) CheckAllow(ctx context.Context, descriptors []resilience.Descriptor) ([]resilience.LimitResult, error) {
	if m.CheckAllowFunc != nil {
		return m.CheckAllowFunc(ctx, descriptors)
	}
	return nil, nil
}

func TestDistributedLimiterTypes(t *testing.T) {
	// Test LimitType constants
	assert.Equal(t, resilience.LimitType("requests"), resilience.LimitTypeRequests)
	assert.Equal(t, resilience.LimitType("tokens"), resilience.LimitTypeTokens)

	// Test Descriptor struct
	desc := resilience.Descriptor{
		Key:    "test-key",
		Value:  "test-value",
		Limit:  100,
		Type:   resilience.LimitTypeRequests,
		Window: time.Minute,
	}
	assert.Equal(t, "test-key", desc.Key)
	assert.Equal(t, "test-value", desc.Value)
	assert.Equal(t, int64(100), desc.Limit)
	assert.Equal(t, resilience.LimitTypeRequests, desc.Type)
	assert.Equal(t, time.Minute, desc.Window)

	// Test LimitResult struct
	now := time.Now().Unix()
	res := resilience.LimitResult{
		Allowed:   true,
		Current:   1,
		Remaining: 99,
		ResetAt:   now,
		Error:     nil,
	}
	assert.True(t, res.Allowed)
	assert.Equal(t, int64(1), res.Current)
	assert.Equal(t, int64(99), res.Remaining)
	assert.Equal(t, now, res.ResetAt)
	assert.Nil(t, res.Error)
}

func TestDistributedLimiterInterface(t *testing.T) {
	// Verify that MockDistributedLimiter implements DistributedLimiter
	var _ resilience.DistributedLimiter = &MockDistributedLimiter{}
}
