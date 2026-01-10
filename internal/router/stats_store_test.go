package router_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/router"
	"github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupRedisStoreIfAvailable attempts to start a Redis container for testing.
// Returns nil if Docker is not available or container fails to start.
// This allows tests to gracefully degrade to memory-only mode.
func setupRedisStoreIfAvailable(t *testing.T) router.StatsStore {
	t.Helper()

	// Recover from panics (e.g., "rootless Docker is not supported on Windows")
	defer func() {
		if r := recover(); r != nil {
			t.Logf("⚠️ Docker setup failed (panic recovered): %v", r)
		}
	}()

	ctx := context.Background()

	// Try to start Redis container
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(30 * time.Second),
	}

	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		// Docker not available or container failed to start
		t.Logf("⚠️ Failed to start Redis container: %v", err)
		return nil
	}

	// Register cleanup
	t.Cleanup(func() {
		if terminateErr := redisContainer.Terminate(ctx); terminateErr != nil {
			t.Logf("Failed to terminate Redis container: %v", terminateErr)
		}
	})

	// Get container host and port
	host, err := redisContainer.Host(ctx)
	if err != nil {
		t.Logf("Failed to get container host: %v", err)
		return nil
	}

	port, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		t.Logf("Failed to get container port: %v", err)
		return nil
	}

	// Create Redis client
	addr := fmt.Sprintf("%s:%s", host, port.Port())
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// Verify connection
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		t.Logf("Failed to ping Redis: %v", err)
		return nil
	}

	t.Logf("✅ Redis container started successfully at %s", addr)

	// Create and return RedisStatsStore
	return router.NewRedisStatsStore(client)
}

// TestStatsStore_Interface defines the contract that all StatsStore implementations must satisfy.
// This is a comprehensive test suite that should be run against both MemoryStatsStore and RedisStatsStore.
func TestStatsStore_Interface(t *testing.T) {
	stores := map[string]router.StatsStore{
		"Memory": router.NewMemoryStatsStore(),
	}

	// Try to add Redis store (requires Docker)
	if redisStore := setupRedisStoreIfAvailable(t); redisStore != nil {
		stores["Redis"] = redisStore
		t.Log("✅ Redis container started, testing distributed stats")
	} else {
		t.Log("⚠️ Docker not available, skipping Redis tests (Memory tests only)")
	}

	for storeName, store := range stores {
		t.Run(storeName, func(t *testing.T) {
			// Ensure cleanup after all subtests
			defer func() {
				if err := store.Close(); err != nil {
					t.Logf("Store cleanup error: %v", err)
				}
			}()

			// Run all contract tests
			t.Run("RecordSuccess", func(t *testing.T) {
				testRecordSuccess(t, store)
			})

			t.Run("RecordFailure", func(t *testing.T) {
				testRecordFailure(t, store)
			})

			t.Run("ActiveRequestsIncDec", func(t *testing.T) {
				testActiveRequestsIncDec(t, store)
			})

			t.Run("CooldownManagement", func(t *testing.T) {
				testCooldownManagement(t, store)
			})

			t.Run("LatencyHistoryRolling", func(t *testing.T) {
				testLatencyHistoryRolling(t, store)
			})

			t.Run("TPMRPMTracking", func(t *testing.T) {
				testTPMRPMTracking(t, store)
			})

			t.Run("ListAndDeleteDeployments", func(t *testing.T) {
				testListAndDeleteDeployments(t, store)
			})

			t.Run("ConcurrentAccess", func(t *testing.T) {
				testConcurrentAccess(t, store)
			})
		})
	}
}

// testRecordSuccess verifies basic success recording functionality.
func testRecordSuccess(t *testing.T, store router.StatsStore) {
	ctx := context.Background()
	deploymentID := "test-deployment-success"

	// Test 1: Record a successful request
	metrics := &router.ResponseMetrics{
		Latency:          100 * time.Millisecond,
		TimeToFirstToken: 50 * time.Millisecond,
		TotalTokens:      50,
		InputTokens:      10,
		OutputTokens:     40,
	}

	err := store.RecordSuccess(ctx, deploymentID, metrics)
	require.NoError(t, err)

	// Test 2: Verify stats were recorded
	stats, err := store.GetStats(ctx, deploymentID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.TotalRequests)
	assert.Equal(t, int64(1), stats.SuccessCount)
	assert.Equal(t, int64(0), stats.FailureCount)

	// Test 3: Verify latency was recorded
	require.Len(t, stats.LatencyHistory, 1)
	assert.Equal(t, 100.0, stats.LatencyHistory[0])

	// Test 4: Verify TTFT was recorded
	require.Len(t, stats.TTFTHistory, 1)
	assert.Equal(t, 50.0, stats.TTFTHistory[0])

	// Test 5: Verify average latency
	assert.Equal(t, 100.0, stats.AvgLatencyMs)
}

// testRecordFailure verifies failure recording functionality.
func testRecordFailure(t *testing.T, store router.StatsStore) {
	ctx := context.Background()
	deploymentID := "test-deployment-failure"

	// Test 1: Record a failed request
	err := store.RecordFailure(ctx, deploymentID, errors.NewServiceUnavailableError(
		"openai",
		"gpt-4",
		"Internal Server Error",
	))
	require.NoError(t, err)

	// Test 2: Verify stats were recorded
	stats, err := store.GetStats(ctx, deploymentID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.TotalRequests)
	assert.Equal(t, int64(0), stats.SuccessCount)
	assert.Equal(t, int64(1), stats.FailureCount)
}

// testActiveRequestsIncDec verifies active request counting.
func testActiveRequestsIncDec(t *testing.T, store router.StatsStore) {
	ctx := context.Background()
	deploymentID := "test-deployment-active"

	// Test 1: Increment active requests
	err := store.IncrementActiveRequests(ctx, deploymentID)
	require.NoError(t, err, "IncrementActiveRequests (1st call) should succeed")

	err = store.IncrementActiveRequests(ctx, deploymentID)
	require.NoError(t, err, "IncrementActiveRequests (2nd call) should succeed")

	stats, err := store.GetStats(ctx, deploymentID)
	if err != nil {
		t.Logf("GetStats returned error: %v (type: %T)", err, err)
	}
	require.NoError(t, err, "GetStats should succeed after IncrementActiveRequests")
	t.Logf("Stats: ActiveRequests=%d, TotalRequests=%d, SuccessCount=%d, FailureCount=%d",
		stats.ActiveRequests, stats.TotalRequests, stats.SuccessCount, stats.FailureCount)
	assert.Equal(t, int64(2), stats.ActiveRequests)

	// Test 2: Decrement active requests
	err = store.DecrementActiveRequests(ctx, deploymentID)
	require.NoError(t, err)

	stats, err = store.GetStats(ctx, deploymentID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.ActiveRequests)

	// Test 3: Decrement to zero
	err = store.DecrementActiveRequests(ctx, deploymentID)
	require.NoError(t, err)

	stats, err = store.GetStats(ctx, deploymentID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), stats.ActiveRequests)

	// Test 4: Decrementing below zero should not panic or error
	err = store.DecrementActiveRequests(ctx, deploymentID)
	require.NoError(t, err)

	stats, err = store.GetStats(ctx, deploymentID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), stats.ActiveRequests, "Active requests should not go below zero")
}

// testCooldownManagement verifies cooldown functionality.
func testCooldownManagement(t *testing.T, store router.StatsStore) {
	ctx := context.Background()
	deploymentID := "test-deployment-cooldown"

	// Test 1: Initially no cooldown
	cooldownUntil, err := store.GetCooldownUntil(ctx, deploymentID)
	require.NoError(t, err)
	assert.True(t, cooldownUntil.IsZero())

	// Test 2: Set cooldown
	future := time.Now().Add(5 * time.Minute)
	err = store.SetCooldown(ctx, deploymentID, future)
	require.NoError(t, err)

	// Test 3: Verify cooldown was set
	cooldownUntil, err = store.GetCooldownUntil(ctx, deploymentID)
	require.NoError(t, err)
	assert.True(t, cooldownUntil.After(time.Now()))
	assert.WithinDuration(t, future, cooldownUntil, time.Second)

	// Test 4: Stats should reflect cooldown
	stats, err := store.GetStats(ctx, deploymentID)
	require.NoError(t, err)
	assert.True(t, stats.CooldownUntil.After(time.Now()))
}

// testLatencyHistoryRolling verifies rolling window behavior.
func testLatencyHistoryRolling(t *testing.T, store router.StatsStore) {
	ctx := context.Background()
	deploymentID := "test-deployment-rolling"

	// Create store with small max size for testing
	_, ok := store.(*router.MemoryStatsStore)
	if !ok {
		t.Skip("Rolling window test only for MemoryStatsStore")
	}

	// Re-create with max size = 3
	memStore := router.NewMemoryStatsStoreWithConfig(3)
	defer memStore.Close()

	// Test 1: Add latencies up to max size
	for i := 0; i < 3; i++ {
		metrics := &router.ResponseMetrics{
			Latency: time.Duration(i*100) * time.Millisecond,
		}
		err := memStore.RecordSuccess(ctx, deploymentID, metrics)
		require.NoError(t, err)
	}

	stats, _ := memStore.GetStats(ctx, deploymentID)
	require.Len(t, stats.LatencyHistory, 3)
	assert.Equal(t, []float64{0.0, 100.0, 200.0}, stats.LatencyHistory)

	// Test 2: Add one more to trigger rolling
	metrics := &router.ResponseMetrics{
		Latency: 300 * time.Millisecond,
	}
	err := memStore.RecordSuccess(ctx, deploymentID, metrics)
	require.NoError(t, err)

	stats, _ = memStore.GetStats(ctx, deploymentID)
	require.Len(t, stats.LatencyHistory, 3, "History should maintain max size")
	assert.Equal(t, []float64{100.0, 200.0, 300.0}, stats.LatencyHistory, "Oldest value should be dropped")
}

// testTPMRPMTracking verifies per-minute token and request tracking.
func testTPMRPMTracking(t *testing.T, store router.StatsStore) {
	ctx := context.Background()
	deploymentID := "test-deployment-tpm-rpm"

	// Test 1: Record multiple requests
	for i := 0; i < 5; i++ {
		metrics := &router.ResponseMetrics{
			Latency:     100 * time.Millisecond,
			TotalTokens: 100,
		}
		err := store.RecordSuccess(ctx, deploymentID, metrics)
		require.NoError(t, err)
	}

	// Test 2: Verify TPM/RPM
	stats, err := store.GetStats(ctx, deploymentID)
	require.NoError(t, err)
	assert.Equal(t, int64(500), stats.CurrentMinuteTPM, "TPM = 5 requests * 100 tokens")
	assert.Equal(t, int64(5), stats.CurrentMinuteRPM, "RPM = 5 requests")

	// Test 3: Verify minute key format
	expectedMinute := time.Now().Format("2006-01-02-15-04")
	assert.Equal(t, expectedMinute, stats.CurrentMinuteKey)
}

// testListAndDeleteDeployments verifies deployment listing and deletion.
func testListAndDeleteDeployments(t *testing.T, store router.StatsStore) {
	ctx := context.Background()

	// Test 1: Initially empty
	deployments, err := store.ListDeployments(ctx)
	require.NoError(t, err)
	initialCount := len(deployments)

	// Test 2: Add some deployments
	for i := 0; i < 3; i++ {
		deploymentID := "test-deployment-" + string(rune('A'+i))
		metrics := &router.ResponseMetrics{Latency: 100 * time.Millisecond}
		recordErr := store.RecordSuccess(ctx, deploymentID, metrics)
		require.NoError(t, recordErr)
	}

	deployments, err = store.ListDeployments(ctx)
	require.NoError(t, err)
	assert.Len(t, deployments, initialCount+3)

	// Test 3: Delete one deployment
	err = store.DeleteStats(ctx, "test-deployment-A")
	require.NoError(t, err)

	deployments, err = store.ListDeployments(ctx)
	require.NoError(t, err)
	assert.Len(t, deployments, initialCount+2)

	// Test 4: Deleted deployment should return error
	_, err = store.GetStats(ctx, "test-deployment-A")
	assert.ErrorIs(t, err, router.ErrStatsNotFound)
}

// testConcurrentAccess verifies thread-safety.
func testConcurrentAccess(t *testing.T, store router.StatsStore) {
	ctx := context.Background()
	deploymentID := "test-deployment-concurrent"

	const (
		goroutines = 100
		operations = 10
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Test: Concurrent increments
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < operations; j++ {
				// Mix of operations
				_ = store.IncrementActiveRequests(ctx, deploymentID)

				metrics := &router.ResponseMetrics{
					Latency:     100 * time.Millisecond,
					TotalTokens: 50,
				}
				_ = store.RecordSuccess(ctx, deploymentID, metrics)

				_ = store.DecrementActiveRequests(ctx, deploymentID)
			}
		}()
	}

	wg.Wait()

	// Verify: Stats should be consistent
	stats, err := store.GetStats(ctx, deploymentID)
	require.NoError(t, err)

	// Total requests should equal goroutines * operations
	assert.Equal(t, int64(goroutines*operations), stats.TotalRequests)
	assert.Equal(t, int64(goroutines*operations), stats.SuccessCount)

	// Active requests should be zero (all increments matched with decrements)
	assert.Equal(t, int64(0), stats.ActiveRequests)
}

// TestMemoryStatsStore_DeepCopy verifies that GetStats returns a copy.
func TestMemoryStatsStore_DeepCopy(t *testing.T) {
	store := router.NewMemoryStatsStore()
	defer store.Close()

	ctx := context.Background()
	deploymentID := "test-deployment-copy"

	// Record some stats
	metrics := &router.ResponseMetrics{
		Latency:     100 * time.Millisecond,
		TotalTokens: 50,
	}
	err := store.RecordSuccess(ctx, deploymentID, metrics)
	require.NoError(t, err)

	// Get stats twice
	stats1, err := store.GetStats(ctx, deploymentID)
	require.NoError(t, err)

	stats2, err := store.GetStats(ctx, deploymentID)
	require.NoError(t, err)

	// Modify stats1.LatencyHistory
	stats1.LatencyHistory[0] = 999.0

	// Verify stats2 was not affected (deep copy)
	assert.NotEqual(t, stats1.LatencyHistory[0], stats2.LatencyHistory[0])
	assert.Equal(t, 100.0, stats2.LatencyHistory[0], "Stats should be independent copies")
}
