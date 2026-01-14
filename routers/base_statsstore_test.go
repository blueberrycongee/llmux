package routers_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
	"github.com/blueberrycongee/llmux/routers"
)

// =============================================================================
// TDD Tests for Distributed Router (StatsStore Integration)
// =============================================================================
// These tests define the EXPECTED BEHAVIOR of BaseRouter when using a StatsStore.
// They were written BEFORE the implementation to ensure true TDD.
//
// Requirements:
// 1. BaseRouter should support an optional StatsStore for distributed stats
// 2. When StatsStore is nil, existing local behavior should be preserved
// 3. When StatsStore is set, stats operations should delegate to it
// 4. Delegation should be fail-safe (errors logged, not propagated)
// =============================================================================

// TestBaseRouter_WithStatsStore_DelegatesReportRequestStart verifies that
// when a StatsStore is configured, ReportRequestStart delegates to it.
func TestBaseRouter_WithStatsStore_DelegatesReportRequestStart(t *testing.T) {
	// Arrange: Create a mock stats store that tracks calls
	mockStore := &mockStatsStore{}
	config := router.DefaultConfig()

	// Act: Create router with stats store and report request start
	r := routers.NewBaseRouterWithStore(config, mockStore)

	deployment := &provider.Deployment{ID: "test-deployment-1", ModelName: "gpt-4"}
	r.AddDeployment(deployment)
	r.ReportRequestStart(context.Background(), deployment)

	// Assert: StatsStore.IncrementActiveRequests was called
	assert.Equal(t, 1, mockStore.incrementCalls, "IncrementActiveRequests should be called once")
	assert.Equal(t, "test-deployment-1", mockStore.lastDeploymentID, "Should pass correct deployment ID")
}

// TestBaseRouter_WithStatsStore_DelegatesReportRequestEnd verifies that
// when a StatsStore is configured, ReportRequestEnd delegates to it.
func TestBaseRouter_WithStatsStore_DelegatesReportRequestEnd(t *testing.T) {
	// Arrange
	mockStore := &mockStatsStore{}
	config := router.DefaultConfig()
	r := routers.NewBaseRouterWithStore(config, mockStore)

	deployment := &provider.Deployment{ID: "test-deployment-2", ModelName: "gpt-4"}
	r.AddDeployment(deployment)

	// Act
	r.ReportRequestEnd(context.Background(), deployment)

	// Assert
	assert.Equal(t, 1, mockStore.decrementCalls, "DecrementActiveRequests should be called once")
	assert.Equal(t, "test-deployment-2", mockStore.lastDeploymentID)
}

// TestBaseRouter_WithStatsStore_DelegatesReportSuccess verifies that
// when a StatsStore is configured, ReportSuccess delegates to it.
func TestBaseRouter_WithStatsStore_DelegatesReportSuccess(t *testing.T) {
	// Arrange
	mockStore := &mockStatsStore{}
	config := router.DefaultConfig()
	r := routers.NewBaseRouterWithStore(config, mockStore)

	deployment := &provider.Deployment{ID: "test-deployment-3", ModelName: "gpt-4"}
	r.AddDeployment(deployment)

	metrics := &router.ResponseMetrics{
		Latency:          100 * time.Millisecond,
		TimeToFirstToken: 50 * time.Millisecond,
		TotalTokens:      150,
	}

	// Act
	r.ReportSuccess(context.Background(), deployment, metrics)

	// Assert
	assert.Equal(t, 1, mockStore.successCalls, "RecordSuccess should be called once")
	assert.Equal(t, "test-deployment-3", mockStore.lastDeploymentID)
	require.NotNil(t, mockStore.lastMetrics)
	assert.Equal(t, 100*time.Millisecond, mockStore.lastMetrics.Latency)
	assert.Equal(t, 150, mockStore.lastMetrics.TotalTokens)
}

// TestBaseRouter_WithStatsStore_DelegatesReportFailure verifies that
// when a StatsStore is configured, ReportFailure delegates to it.
func TestBaseRouter_WithStatsStore_DelegatesReportFailure(t *testing.T) {
	// Arrange
	mockStore := &mockStatsStore{}
	config := router.DefaultConfig()
	r := routers.NewBaseRouterWithStore(config, mockStore)

	deployment := &provider.Deployment{ID: "test-deployment-4", ModelName: "gpt-4"}
	r.AddDeployment(deployment)

	testErr := assert.AnError

	// Act
	r.ReportFailure(context.Background(), deployment, testErr)

	// Assert
	assert.Equal(t, 1, mockStore.failureCalls, "RecordFailure should be called once")
	assert.Equal(t, "test-deployment-4", mockStore.lastDeploymentID)
	assert.Equal(t, testErr, mockStore.lastError)
}

// TestBaseRouter_WithoutStatsStore_UsesLocalStats verifies backward compatibility:
// when no StatsStore is configured, local stats map is used.
func TestBaseRouter_WithoutStatsStore_UsesLocalStats(t *testing.T) {
	// Arrange: Create router WITHOUT stats store
	config := router.DefaultConfig()
	r := routers.NewBaseRouter(config)

	deployment := &provider.Deployment{ID: "local-deployment", ModelName: "gpt-4"}
	r.AddDeployment(deployment)

	// Act: Record some activity
	r.ReportRequestStart(context.Background(), deployment)
	r.ReportSuccess(context.Background(), deployment, &router.ResponseMetrics{
		Latency:     100 * time.Millisecond,
		TotalTokens: 50,
	})
	r.ReportRequestEnd(context.Background(), deployment)

	// Assert: Local stats should reflect the activity
	stats := r.GetStats("local-deployment")
	require.NotNil(t, stats, "Local stats should exist")
	assert.Equal(t, int64(1), stats.TotalRequests, "Should have 1 request")
	assert.Equal(t, int64(1), stats.SuccessCount, "Should have 1 success")
	assert.Equal(t, int64(0), stats.ActiveRequests, "Active should be 0 after end")
}

// TestBaseRouter_WithStatsStore_FailSafe verifies that StatsStore errors
// don't crash the router - they should be logged but not propagated.
func TestBaseRouter_WithStatsStore_FailSafe(t *testing.T) {
	// Arrange: Create a store that always returns errors
	failingStore := &mockStatsStore{shouldFail: true}
	config := router.DefaultConfig()
	r := routers.NewBaseRouterWithStore(config, failingStore)

	deployment := &provider.Deployment{ID: "fail-safe-deployment", ModelName: "gpt-4"}
	r.AddDeployment(deployment)

	// Act & Assert: None of these should panic or return error
	assert.NotPanics(t, func() {
		r.ReportRequestStart(context.Background(), deployment)
	}, "ReportRequestStart should not panic on store error")

	assert.NotPanics(t, func() {
		r.ReportRequestEnd(context.Background(), deployment)
	}, "ReportRequestEnd should not panic on store error")

	assert.NotPanics(t, func() {
		r.ReportSuccess(context.Background(), deployment, &router.ResponseMetrics{Latency: 100 * time.Millisecond})
	}, "ReportSuccess should not panic on store error")

	assert.NotPanics(t, func() {
		r.ReportFailure(context.Background(), deployment, assert.AnError)
	}, "ReportFailure should not panic on store error")
}

// TestBaseRouter_WithStatsStore_IsCircuitOpen verifies that cooldown checks
// delegate to the StatsStore when configured.
func TestBaseRouter_WithStatsStore_IsCircuitOpen(t *testing.T) {
	// Arrange
	mockStore := &mockStatsStore{
		cooldownUntil: time.Now().Add(5 * time.Minute), // Set future cooldown
	}
	config := router.DefaultConfig()
	r := routers.NewBaseRouterWithStore(config, mockStore)

	deployment := &provider.Deployment{ID: "cooldown-deployment", ModelName: "gpt-4"}
	r.AddDeployment(deployment)

	// Act
	isOpen := r.IsCircuitOpen(deployment)

	// Assert
	assert.True(t, isOpen, "Circuit should be open when cooldown is in future")
	assert.Equal(t, 1, mockStore.cooldownCheckCalls, "Should check cooldown via store")
}

// =============================================================================
// Mock StatsStore for Testing
// =============================================================================

type mockStatsStore struct {
	// Call tracking
	incrementCalls     int
	decrementCalls     int
	successCalls       int
	failureCalls       int
	cooldownCheckCalls int

	// Last call arguments
	lastDeploymentID string
	lastMetrics      *router.ResponseMetrics
	lastError        error

	// Behavior control
	shouldFail    bool
	cooldownUntil time.Time
}

func (m *mockStatsStore) GetStats(ctx context.Context, deploymentID string) (*router.DeploymentStats, error) {
	return &router.DeploymentStats{}, nil
}

func (m *mockStatsStore) IncrementActiveRequests(ctx context.Context, deploymentID string) error {
	m.incrementCalls++
	m.lastDeploymentID = deploymentID
	if m.shouldFail {
		return assert.AnError
	}
	return nil
}

func (m *mockStatsStore) DecrementActiveRequests(ctx context.Context, deploymentID string) error {
	m.decrementCalls++
	m.lastDeploymentID = deploymentID
	if m.shouldFail {
		return assert.AnError
	}
	return nil
}

func (m *mockStatsStore) RecordSuccess(ctx context.Context, deploymentID string, metrics *router.ResponseMetrics) error {
	m.successCalls++
	m.lastDeploymentID = deploymentID
	m.lastMetrics = metrics
	if m.shouldFail {
		return assert.AnError
	}
	return nil
}

func (m *mockStatsStore) RecordFailure(ctx context.Context, deploymentID string, err error) error {
	m.failureCalls++
	m.lastDeploymentID = deploymentID
	m.lastError = err
	if m.shouldFail {
		return assert.AnError
	}
	return nil
}

func (m *mockStatsStore) SetCooldown(ctx context.Context, deploymentID string, until time.Time) error {
	return nil
}

func (m *mockStatsStore) GetCooldownUntil(ctx context.Context, deploymentID string) (time.Time, error) {
	m.cooldownCheckCalls++
	m.lastDeploymentID = deploymentID
	return m.cooldownUntil, nil
}

func (m *mockStatsStore) ListDeployments(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockStatsStore) DeleteStats(ctx context.Context, deploymentID string) error {
	return nil
}

func (m *mockStatsStore) Close() error {
	return nil
}
