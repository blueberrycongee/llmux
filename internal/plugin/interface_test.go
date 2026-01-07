package plugin

import (
	"context"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Context Tests
// =============================================================================

func TestNewContext(t *testing.T) {
	ctx := context.Background()
	requestID := "test-request-123"

	pluginCtx := NewContext(ctx, requestID)

	if pluginCtx == nil {
		t.Fatal("NewContext returned nil")
	}
	if pluginCtx.RequestID != requestID {
		t.Errorf("RequestID = %q, want %q", pluginCtx.RequestID, requestID)
	}
	if pluginCtx.Context != ctx {
		t.Error("Context not set correctly")
	}
	if pluginCtx.StartTime.IsZero() {
		t.Error("StartTime should be set")
	}
	if pluginCtx.values == nil {
		t.Error("values map should be initialized")
	}
}

func TestContext_SetGet(t *testing.T) {
	ctx := NewContext(context.Background(), "test")

	// Test basic set/get
	ctx.Set("key1", "value1")
	val, ok := ctx.Get("key1")
	if !ok {
		t.Error("Get should return true for existing key")
	}
	if val != "value1" {
		t.Errorf("Get = %v, want %v", val, "value1")
	}

	// Test non-existent key
	_, ok = ctx.Get("nonexistent")
	if ok {
		t.Error("Get should return false for non-existent key")
	}
}

func TestContext_SetGet_NilValues(t *testing.T) {
	ctx := &Context{
		Context:   context.Background(),
		RequestID: "test",
		values:    nil, // Explicitly nil
	}

	// Get on nil values should not panic
	_, ok := ctx.Get("key")
	if ok {
		t.Error("Get on nil values should return false")
	}

	// Set on nil values should initialize the map
	ctx.Set("key", "value")
	val, ok := ctx.Get("key")
	if !ok || val != "value" {
		t.Error("Set should initialize values map and store value")
	}
}

func TestContext_GetString(t *testing.T) {
	ctx := NewContext(context.Background(), "test")

	// Test with string value
	ctx.Set("str", "hello")
	if got := ctx.GetString("str"); got != "hello" {
		t.Errorf("GetString = %q, want %q", got, "hello")
	}

	// Test with non-string value
	ctx.Set("int", 42)
	if got := ctx.GetString("int"); got != "" {
		t.Errorf("GetString for int = %q, want empty string", got)
	}

	// Test with non-existent key
	if got := ctx.GetString("nonexistent"); got != "" {
		t.Errorf("GetString for nonexistent = %q, want empty string", got)
	}
}

func TestContext_GetInt(t *testing.T) {
	ctx := NewContext(context.Background(), "test")

	// Test with int value
	ctx.Set("num", 42)
	if got := ctx.GetInt("num"); got != 42 {
		t.Errorf("GetInt = %d, want %d", got, 42)
	}

	// Test with non-int value
	ctx.Set("str", "hello")
	if got := ctx.GetInt("str"); got != 0 {
		t.Errorf("GetInt for string = %d, want 0", got)
	}

	// Test with non-existent key
	if got := ctx.GetInt("nonexistent"); got != 0 {
		t.Errorf("GetInt for nonexistent = %d, want 0", got)
	}
}

func TestContext_GetBool(t *testing.T) {
	ctx := NewContext(context.Background(), "test")

	// Test with bool value
	ctx.Set("flag", true)
	if got := ctx.GetBool("flag"); !got {
		t.Error("GetBool = false, want true")
	}

	ctx.Set("flag2", false)
	if got := ctx.GetBool("flag2"); got {
		t.Error("GetBool = true, want false")
	}

	// Test with non-bool value
	ctx.Set("str", "true")
	if got := ctx.GetBool("str"); got {
		t.Error("GetBool for string = true, want false")
	}

	// Test with non-existent key
	if got := ctx.GetBool("nonexistent"); got {
		t.Error("GetBool for nonexistent = true, want false")
	}
}

func TestContext_Reset(t *testing.T) {
	ctx := NewContext(context.Background(), "test-123")
	ctx.Model = "gpt-4"
	ctx.Provider = "openai"
	ctx.IsStreaming = true
	ctx.Set("key", "value")

	ctx.Reset()

	if ctx.RequestID != "" {
		t.Errorf("RequestID = %q, want empty", ctx.RequestID)
	}
	if ctx.Model != "" {
		t.Errorf("Model = %q, want empty", ctx.Model)
	}
	if ctx.Provider != "" {
		t.Errorf("Provider = %q, want empty", ctx.Provider)
	}
	if ctx.IsStreaming {
		t.Error("IsStreaming = true, want false")
	}
	if !ctx.StartTime.IsZero() {
		t.Error("StartTime should be zero")
	}
	if ctx.Context != nil {
		t.Error("Context should be nil")
	}

	// Values map should be cleared but not nil
	if _, ok := ctx.Get("key"); ok {
		t.Error("values should be cleared")
	}
}

func TestContext_ConcurrentAccess(t *testing.T) {
	ctx := NewContext(context.Background(), "test")
	var wg sync.WaitGroup
	iterations := 1000

	// Concurrent writes
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx.Set("key", i)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx.Get("key")
			ctx.GetString("key")
			ctx.GetInt("key")
			ctx.GetBool("key")
		}()
	}

	wg.Wait()
	// Test passes if no race condition detected
}

func TestContext_ContextCancellation(t *testing.T) {
	parentCtx, cancel := context.WithCancel(context.Background())
	ctx := NewContext(parentCtx, "test")

	// Cancel parent context
	cancel()

	// Plugin context should reflect cancellation
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Context should be canceled")
	}

	if ctx.Err() != context.Canceled {
		t.Errorf("Err = %v, want %v", ctx.Err(), context.Canceled)
	}
}

func TestContext_ContextTimeout(t *testing.T) {
	parentCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	ctx := NewContext(parentCtx, "test")

	// Wait for timeout
	time.Sleep(20 * time.Millisecond)

	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Context should be timed out")
	}

	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("Err = %v, want %v", ctx.Err(), context.DeadlineExceeded)
	}
}

// =============================================================================
// ShortCircuit Tests
// =============================================================================

func TestShortCircuit_Defaults(t *testing.T) {
	sc := &ShortCircuit{}

	if sc.Response != nil {
		t.Error("Response should be nil by default")
	}
	if sc.Error != nil {
		t.Error("Error should be nil by default")
	}
	if sc.AllowFallback {
		t.Error("AllowFallback should be false by default")
	}
	if sc.Metadata != nil {
		t.Error("Metadata should be nil by default")
	}
}

func TestShortCircuit_WithResponse(t *testing.T) {
	// This test verifies ShortCircuit can carry a response
	// Actual response type testing will be in pipeline tests
	sc := &ShortCircuit{
		AllowFallback: true,
		Metadata: map[string]any{
			"cache_hit": true,
		},
	}

	if !sc.AllowFallback {
		t.Error("AllowFallback should be true")
	}
	if sc.Metadata["cache_hit"] != true {
		t.Error("Metadata not set correctly")
	}
}

// =============================================================================
// StreamShortCircuit Tests
// =============================================================================

func TestStreamShortCircuit_Defaults(t *testing.T) {
	ssc := &StreamShortCircuit{}

	if ssc.Stream != nil {
		t.Error("Stream should be nil by default")
	}
	if ssc.Error != nil {
		t.Error("Error should be nil by default")
	}
	if ssc.AllowFallback {
		t.Error("AllowFallback should be false by default")
	}
}
