package plugin

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// =============================================================================
// Mock Plugin Implementations
// =============================================================================

// mockPlugin is a basic mock plugin for testing.
type mockPlugin struct {
	name     string
	priority int

	preHookCalled  atomic.Bool
	postHookCalled atomic.Bool
	cleanupCalled  atomic.Bool

	preHookReq    *types.ChatRequest
	preHookErr    error
	preHookSC     *ShortCircuit
	postHookResp  *types.ChatResponse
	postHookErr   error
	postHookPErr  error
	cleanupErr    error

	preHookDelay  time.Duration
	postHookDelay time.Duration
}

func newMockPlugin(name string, priority int) *mockPlugin {
	return &mockPlugin{
		name:     name,
		priority: priority,
	}
}

func (m *mockPlugin) Name() string  { return m.name }
func (m *mockPlugin) Priority() int { return m.priority }

func (m *mockPlugin) PreHook(ctx *Context, req *types.ChatRequest) (*types.ChatRequest, *ShortCircuit, error) {
	m.preHookCalled.Store(true)
	if m.preHookDelay > 0 {
		time.Sleep(m.preHookDelay)
	}
	if m.preHookReq != nil {
		return m.preHookReq, m.preHookSC, m.preHookErr
	}
	return req, m.preHookSC, m.preHookErr
}

func (m *mockPlugin) PostHook(ctx *Context, resp *types.ChatResponse, err error) (*types.ChatResponse, error, error) {
	m.postHookCalled.Store(true)
	if m.postHookDelay > 0 {
		time.Sleep(m.postHookDelay)
	}
	if m.postHookResp != nil {
		return m.postHookResp, m.postHookErr, m.postHookPErr
	}
	return resp, err, m.postHookPErr
}

func (m *mockPlugin) Cleanup() error {
	m.cleanupCalled.Store(true)
	return m.cleanupErr
}

// trackingPlugin wraps mockPlugin with callbacks for tracking execution order
type trackingPlugin struct {
	*mockPlugin
	onPreHook  func()
	onPostHook func()
}

func newTrackingPlugin(name string, priority int, onPreHook, onPostHook func()) *trackingPlugin {
	return &trackingPlugin{
		mockPlugin: newMockPlugin(name, priority),
		onPreHook:  onPreHook,
		onPostHook: onPostHook,
	}
}

func (t *trackingPlugin) PreHook(ctx *Context, req *types.ChatRequest) (*types.ChatRequest, *ShortCircuit, error) {
	if t.onPreHook != nil {
		t.onPreHook()
	}
	return t.mockPlugin.PreHook(ctx, req)
}

func (t *trackingPlugin) PostHook(ctx *Context, resp *types.ChatResponse, err error) (*types.ChatResponse, error, error) {
	if t.onPostHook != nil {
		t.onPostHook()
	}
	return t.mockPlugin.PostHook(ctx, resp, err)
}

// mockStreamPlugin implements StreamPlugin for testing.
type mockStreamPlugin struct {
	mockPlugin
	preStreamHookCalled  atomic.Bool
	onStreamChunkCalled  atomic.Int32
	postStreamHookCalled atomic.Bool

	streamShortCircuit   *StreamShortCircuit
	onChunkFilter        bool // If true, return nil chunk to filter
}

func newMockStreamPlugin(name string, priority int) *mockStreamPlugin {
	return &mockStreamPlugin{
		mockPlugin: mockPlugin{
			name:     name,
			priority: priority,
		},
	}
}

func (m *mockStreamPlugin) PreStreamHook(ctx *Context, req *types.ChatRequest) (*types.ChatRequest, *StreamShortCircuit, error) {
	m.preStreamHookCalled.Store(true)
	return req, m.streamShortCircuit, nil
}

func (m *mockStreamPlugin) OnStreamChunk(ctx *Context, chunk *types.StreamChunk) (*types.StreamChunk, error) {
	m.onStreamChunkCalled.Add(1)
	if m.onChunkFilter {
		return nil, nil
	}
	return chunk, nil
}

func (m *mockStreamPlugin) PostStreamHook(ctx *Context, err error) error {
	m.postStreamHookCalled.Store(true)
	return nil
}

// =============================================================================
// Pipeline Creation Tests
// =============================================================================

func TestNewPipeline(t *testing.T) {
	logger := slog.Default()
	config := DefaultPipelineConfig()

	p := NewPipeline(logger, config)

	if p == nil {
		t.Fatal("NewPipeline returned nil")
	}
	if p.PluginCount() != 0 {
		t.Errorf("PluginCount = %d, want 0", p.PluginCount())
	}
	if p.IsClosed() {
		t.Error("Pipeline should not be closed")
	}
}

func TestNewPipeline_DefaultConfig(t *testing.T) {
	p := NewPipeline(nil, PipelineConfig{})

	if p.config.PreHookTimeout != 10*time.Second {
		t.Errorf("PreHookTimeout = %v, want 10s", p.config.PreHookTimeout)
	}
	if p.config.PostHookTimeout != 10*time.Second {
		t.Errorf("PostHookTimeout = %v, want 10s", p.config.PostHookTimeout)
	}
	if p.config.MaxPlugins != 100 {
		t.Errorf("MaxPlugins = %d, want 100", p.config.MaxPlugins)
	}
}

// =============================================================================
// Plugin Registration Tests
// =============================================================================

func TestPipeline_Register(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	plugin := newMockPlugin("test-plugin", 10)
	err := p.Register(plugin)

	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if p.PluginCount() != 1 {
		t.Errorf("PluginCount = %d, want 1", p.PluginCount())
	}
}

func TestPipeline_Register_PriorityOrder(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	// Register in random priority order
	_ = p.Register(newMockPlugin("mid", 50))
	_ = p.Register(newMockPlugin("first", 10))
	_ = p.Register(newMockPlugin("last", 100))

	plugins := p.Plugins()

	if len(plugins) != 3 {
		t.Fatalf("Plugins count = %d, want 3", len(plugins))
	}
	if plugins[0].Name() != "first" {
		t.Errorf("First plugin = %s, want 'first'", plugins[0].Name())
	}
	if plugins[1].Name() != "mid" {
		t.Errorf("Second plugin = %s, want 'mid'", plugins[1].Name())
	}
	if plugins[2].Name() != "last" {
		t.Errorf("Third plugin = %s, want 'last'", plugins[2].Name())
	}
}

func TestPipeline_Register_Duplicate(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	_ = p.Register(newMockPlugin("duplicate", 10))
	err := p.Register(newMockPlugin("duplicate", 20))

	if !errors.Is(err, ErrDuplicatePlugin) {
		t.Errorf("err = %v, want ErrDuplicatePlugin", err)
	}
}

func TestPipeline_Register_NilPlugin(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	err := p.Register(nil)

	if !errors.Is(err, ErrNilPlugin) {
		t.Errorf("err = %v, want ErrNilPlugin", err)
	}
}

func TestPipeline_Register_TooMany(t *testing.T) {
	config := DefaultPipelineConfig()
	config.MaxPlugins = 2
	p := NewPipeline(nil, config)

	_ = p.Register(newMockPlugin("p1", 1))
	_ = p.Register(newMockPlugin("p2", 2))
	err := p.Register(newMockPlugin("p3", 3))

	if !errors.Is(err, ErrTooManyPlugins) {
		t.Errorf("err = %v, want ErrTooManyPlugins", err)
	}
}

func TestPipeline_Register_AfterClose(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())
	_ = p.Shutdown()

	err := p.Register(newMockPlugin("test", 10))

	if !errors.Is(err, ErrPipelineClosed) {
		t.Errorf("err = %v, want ErrPipelineClosed", err)
	}
}

func TestPipeline_Unregister(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())
	_ = p.Register(newMockPlugin("test", 10))

	err := p.Unregister("test")

	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}
	if p.PluginCount() != 0 {
		t.Errorf("PluginCount = %d, want 0", p.PluginCount())
	}
}

func TestPipeline_Unregister_NotFound(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	err := p.Unregister("nonexistent")

	if !errors.Is(err, ErrPluginNotFound) {
		t.Errorf("err = %v, want ErrPluginNotFound", err)
	}
}

// =============================================================================
// PreHook Execution Tests
// =============================================================================

func TestPipeline_RunPreHooks_Empty(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())
	ctx := p.GetContext(context.Background(), "test-123")
	defer p.PutContext(ctx)

	req := &types.ChatRequest{Model: "gpt-4"}
	outReq, sc, count := p.RunPreHooks(ctx, req)

	if outReq != req {
		t.Error("Request should be unchanged")
	}
	if sc != nil {
		t.Error("ShortCircuit should be nil")
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestPipeline_RunPreHooks_SinglePlugin(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())
	plugin := newMockPlugin("test", 10)
	_ = p.Register(plugin)

	ctx := p.GetContext(context.Background(), "test-123")
	defer p.PutContext(ctx)

	req := &types.ChatRequest{Model: "gpt-4"}
	_, _, count := p.RunPreHooks(ctx, req)

	if !plugin.preHookCalled.Load() {
		t.Error("PreHook was not called")
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestPipeline_RunPreHooks_ExecutionOrder(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	var order []string
	var mu sync.Mutex

	// Create plugins that record their execution order
	for _, pdata := range []struct {
		name     string
		priority int
	}{
		{"third", 30},
		{"first", 10},
		{"second", 20},
	} {
		name := pdata.name
		plugin := newTrackingPlugin(pdata.name, pdata.priority, func() {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
		}, nil)
		_ = p.Register(plugin)
	}

	ctx := p.GetContext(context.Background(), "test")
	req := &types.ChatRequest{Model: "gpt-4"}
	p.RunPreHooks(ctx, req)

	mu.Lock()
	defer mu.Unlock()

	if len(order) != 3 {
		t.Fatalf("order len = %d, want 3", len(order))
	}
	if order[0] != "first" || order[1] != "second" || order[2] != "third" {
		t.Errorf("order = %v, want [first, second, third]", order)
	}
}

func TestPipeline_RunPreHooks_ShortCircuit(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	scPlugin := newMockPlugin("short-circuit", 10)
	scPlugin.preHookSC = &ShortCircuit{
		Response: &types.ChatResponse{ID: "cached"},
	}
	_ = p.Register(scPlugin)

	neverCalled := newMockPlugin("never-called", 20)
	_ = p.Register(neverCalled)

	ctx := p.GetContext(context.Background(), "test")
	req := &types.ChatRequest{Model: "gpt-4"}
	_, sc, count := p.RunPreHooks(ctx, req)

	if sc == nil {
		t.Fatal("ShortCircuit should not be nil")
	}
	if sc.Response.ID != "cached" {
		t.Errorf("Response.ID = %q, want 'cached'", sc.Response.ID)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1 (short-circuited)", count)
	}
	if neverCalled.preHookCalled.Load() {
		t.Error("Second plugin should not be called after short-circuit")
	}
}

func TestPipeline_RunPreHooks_ModifyRequest(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	modPlugin := newMockPlugin("modifier", 10)
	modPlugin.preHookReq = &types.ChatRequest{Model: "gpt-4-modified"}
	_ = p.Register(modPlugin)

	ctx := p.GetContext(context.Background(), "test")
	req := &types.ChatRequest{Model: "gpt-4"}
	outReq, _, _ := p.RunPreHooks(ctx, req)

	if outReq.Model != "gpt-4-modified" {
		t.Errorf("Model = %q, want 'gpt-4-modified'", outReq.Model)
	}
}

func TestPipeline_RunPreHooks_ErrorContinues(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	errorPlugin := newMockPlugin("error-plugin", 10)
	errorPlugin.preHookErr = errors.New("plugin error")
	_ = p.Register(errorPlugin)

	nextPlugin := newMockPlugin("next-plugin", 20)
	_ = p.Register(nextPlugin)

	ctx := p.GetContext(context.Background(), "test")
	req := &types.ChatRequest{Model: "gpt-4"}
	p.RunPreHooks(ctx, req)

	// Both should be called even if first returns error
	if !errorPlugin.preHookCalled.Load() {
		t.Error("errorPlugin PreHook should be called")
	}
	if !nextPlugin.preHookCalled.Load() {
		t.Error("nextPlugin PreHook should be called (error doesn't stop execution)")
	}
}

// =============================================================================
// PostHook Execution Tests
// =============================================================================

func TestPipeline_RunPostHooks_Empty(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())
	ctx := p.GetContext(context.Background(), "test")
	defer p.PutContext(ctx)

	resp := &types.ChatResponse{ID: "test-resp"}
	outResp, outErr := p.RunPostHooks(ctx, resp, nil, 0)

	if outResp != resp {
		t.Error("Response should be unchanged")
	}
	if outErr != nil {
		t.Error("Error should be nil")
	}
}

func TestPipeline_RunPostHooks_ReverseOrder(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	var order []string
	var mu sync.Mutex

	for _, pdata := range []struct {
		name     string
		priority int
	}{
		{"third", 30},
		{"first", 10},
		{"second", 20},
	} {
		name := pdata.name
		plugin := newTrackingPlugin(pdata.name, pdata.priority, nil, func() {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
		})
		_ = p.Register(plugin)
	}

	ctx := p.GetContext(context.Background(), "test")
	resp := &types.ChatResponse{ID: "test"}
	// runFrom = 3 means all 3 plugins' PreHooks were run
	p.RunPostHooks(ctx, resp, nil, 3)

	mu.Lock()
	defer mu.Unlock()

	if len(order) != 3 {
		t.Fatalf("order len = %d, want 3", len(order))
	}
	// PostHooks should run in reverse: third (30) -> second (20) -> first (10)
	if order[0] != "third" || order[1] != "second" || order[2] != "first" {
		t.Errorf("order = %v, want [third, second, first]", order)
	}
}

func TestPipeline_RunPostHooks_PartialExecution(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	p1 := newMockPlugin("p1", 10)
	p2 := newMockPlugin("p2", 20)
	p3 := newMockPlugin("p3", 30)
	_ = p.Register(p1)
	_ = p.Register(p2)
	_ = p.Register(p3)

	ctx := p.GetContext(context.Background(), "test")
	resp := &types.ChatResponse{}
	// runFrom = 2 means only first 2 plugins' PreHooks were run
	p.RunPostHooks(ctx, resp, nil, 2)

	// Only p1 and p2 PostHooks should be called
	if !p1.postHookCalled.Load() {
		t.Error("p1 PostHook should be called")
	}
	if !p2.postHookCalled.Load() {
		t.Error("p2 PostHook should be called")
	}
	if p3.postHookCalled.Load() {
		t.Error("p3 PostHook should NOT be called (runFrom=2)")
	}
}

func TestPipeline_RunPostHooks_ErrorRecovery(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	recoveryPlugin := newMockPlugin("recovery", 10)
	recoveryPlugin.postHookResp = &types.ChatResponse{ID: "recovered"}
	recoveryPlugin.postHookErr = nil // Clear the error
	_ = p.Register(recoveryPlugin)

	ctx := p.GetContext(context.Background(), "test")
	originalErr := errors.New("original error")
	outResp, outErr := p.RunPostHooks(ctx, nil, originalErr, 1)

	if outResp == nil {
		t.Error("Response should be recovered")
	}
	if outErr != nil {
		t.Error("Error should be recovered (nil)")
	}
}

// =============================================================================
// Stream Plugin Tests
// =============================================================================

func TestPipeline_RunStreamPreHooks(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	streamPlugin := newMockStreamPlugin("stream", 10)
	_ = p.Register(streamPlugin)

	ctx := p.GetContext(context.Background(), "test")
	req := &types.ChatRequest{Model: "gpt-4", Stream: true}
	_, _, count := p.RunStreamPreHooks(ctx, req)

	if !streamPlugin.preStreamHookCalled.Load() {
		t.Error("PreStreamHook should be called")
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestPipeline_RunStreamPreHooks_MixedPlugins(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	regularPlugin := newMockPlugin("regular", 10)
	streamPlugin := newMockStreamPlugin("stream", 20)
	_ = p.Register(regularPlugin)
	_ = p.Register(streamPlugin)

	ctx := p.GetContext(context.Background(), "test")
	req := &types.ChatRequest{Model: "gpt-4", Stream: true}
	_, _, count := p.RunStreamPreHooks(ctx, req)

	// Regular plugin is skipped but counted
	if !streamPlugin.preStreamHookCalled.Load() {
		t.Error("StreamPlugin's PreStreamHook should be called")
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestPipeline_RunOnStreamChunk(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	streamPlugin := newMockStreamPlugin("stream", 10)
	_ = p.Register(streamPlugin)

	ctx := p.GetContext(context.Background(), "test")
	chunk := &types.StreamChunk{ID: "chunk-1"}
	outChunk, err := p.RunOnStreamChunk(ctx, chunk)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if outChunk != chunk {
		t.Error("chunk should be unchanged")
	}
	if streamPlugin.onStreamChunkCalled.Load() != 1 {
		t.Errorf("OnStreamChunk called %d times, want 1", streamPlugin.onStreamChunkCalled.Load())
	}
}

func TestPipeline_RunOnStreamChunk_Filter(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	filterPlugin := newMockStreamPlugin("filter", 10)
	filterPlugin.onChunkFilter = true
	_ = p.Register(filterPlugin)

	ctx := p.GetContext(context.Background(), "test")
	chunk := &types.StreamChunk{ID: "chunk-1"}
	outChunk, _ := p.RunOnStreamChunk(ctx, chunk)

	if outChunk != nil {
		t.Error("chunk should be filtered (nil)")
	}
}

// =============================================================================
// Context Pool Tests
// =============================================================================

func TestPipeline_ContextPool(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	ctx1 := p.GetContext(context.Background(), "req-1")
	ctx1.Model = "gpt-4"
	ctx1.Set("key", "value")

	// Return to pool
	p.PutContext(ctx1)

	// Get from pool (might be the same one)
	ctx2 := p.GetContext(context.Background(), "req-2")

	if ctx2.RequestID != "req-2" {
		t.Errorf("RequestID = %q, want 'req-2'", ctx2.RequestID)
	}
	if ctx2.Model != "" {
		t.Errorf("Model should be reset, got %q", ctx2.Model)
	}
	if _, ok := ctx2.Get("key"); ok {
		t.Error("values should be cleared")
	}

	p.PutContext(ctx2)
}

// =============================================================================
// Shutdown Tests
// =============================================================================

func TestPipeline_Shutdown(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	p1 := newMockPlugin("p1", 10)
	p2 := newMockPlugin("p2", 20)
	_ = p.Register(p1)
	_ = p.Register(p2)

	err := p.Shutdown()

	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
	if !p.IsClosed() {
		t.Error("Pipeline should be closed")
	}
	if !p1.cleanupCalled.Load() {
		t.Error("p1 Cleanup should be called")
	}
	if !p2.cleanupCalled.Load() {
		t.Error("p2 Cleanup should be called")
	}
}

func TestPipeline_Shutdown_WithErrors(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())

	p1 := newMockPlugin("p1", 10)
	p1.cleanupErr = errors.New("cleanup failed")
	_ = p.Register(p1)

	err := p.Shutdown()

	if err == nil {
		t.Error("Shutdown should return error when cleanup fails")
	}
}

func TestPipeline_Shutdown_DoubleCall(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())
	_ = p.Shutdown()

	err := p.Shutdown()

	if !errors.Is(err, ErrPipelineClosed) {
		t.Errorf("err = %v, want ErrPipelineClosed", err)
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestPipeline_ConcurrentPreHooks(t *testing.T) {
	p := NewPipeline(nil, DefaultPipelineConfig())
	plugin := newMockPlugin("concurrent", 10)
	_ = p.Register(plugin)

	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := p.GetContext(context.Background(), "test")
			req := &types.ChatRequest{Model: "gpt-4"}
			p.RunPreHooks(ctx, req)
			p.PutContext(ctx)
		}()
	}

	wg.Wait()
}

func TestPipeline_ConcurrentRegisterUnregister(t *testing.T) {
	p := NewPipeline(nil, PipelineConfig{MaxPlugins: 1000})

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent registrations
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			plugin := newMockPlugin("plugin-"+string(rune('A'+i%26)), i)
			_ = p.Register(plugin)
		}(i)
	}

	wg.Wait()
}
