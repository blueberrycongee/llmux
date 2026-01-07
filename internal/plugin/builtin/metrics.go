package builtin

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/blueberrycongee/llmux/internal/plugin"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// MetricsPlugin collects request metrics including latency, token usage,
// success/failure counts, and more.
type MetricsPlugin struct {
	priority int

	// Global metrics
	TotalRequests      atomic.Int64
	SuccessfulRequests atomic.Int64
	FailedRequests     atomic.Int64
	TotalTokens        atomic.Int64
	PromptTokens       atomic.Int64
	CompletionTokens   atomic.Int64
	CacheHits          atomic.Int64
	RateLimited        atomic.Int64

	// Latency tracking (in milliseconds)
	latencies     []int64
	latenciesLock sync.Mutex

	// Per-model metrics
	modelMetrics sync.Map // map[string]*ModelMetrics

	// Per-provider metrics
	providerMetrics sync.Map // map[string]*ProviderMetrics

	// Callback for external metric systems (Prometheus, StatsD, etc.)
	OnMetrics func(m *RequestMetrics)
}

// ModelMetrics tracks metrics for a specific model.
type ModelMetrics struct {
	Requests       atomic.Int64
	Successes      atomic.Int64
	Failures       atomic.Int64
	TotalTokens    atomic.Int64
	TotalLatencyMs atomic.Int64
}

// ProviderMetrics tracks metrics for a specific provider.
type ProviderMetrics struct {
	Requests       atomic.Int64
	Successes      atomic.Int64
	Failures       atomic.Int64
	TotalLatencyMs atomic.Int64
}

// RequestMetrics contains metrics for a single request.
type RequestMetrics struct {
	RequestID        string
	Model            string
	Provider         string
	LatencyMs        int64
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Success          bool
	CacheHit         bool
	Error            string
	Timestamp        time.Time
}

// MetricsOption configures the MetricsPlugin.
type MetricsOption func(*MetricsPlugin)

// WithMetricsPriority sets the plugin priority.
func WithMetricsPriority(priority int) MetricsOption {
	return func(p *MetricsPlugin) {
		p.priority = priority
	}
}

// WithMetricsCallback sets a callback function for each request's metrics.
func WithMetricsCallback(fn func(m *RequestMetrics)) MetricsOption {
	return func(p *MetricsPlugin) {
		p.OnMetrics = fn
	}
}

// NewMetricsPlugin creates a new metrics collection plugin.
// Default priority is 999 (very low, to capture final request state).
func NewMetricsPlugin(opts ...MetricsOption) *MetricsPlugin {
	p := &MetricsPlugin{
		priority:  999,
		latencies: make([]int64, 0, 1000),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *MetricsPlugin) Name() string  { return "metrics" }
func (p *MetricsPlugin) Priority() int { return p.priority }

func (p *MetricsPlugin) PreHook(ctx *plugin.Context, req *types.ChatRequest) (*types.ChatRequest, *plugin.ShortCircuit, error) {
	p.TotalRequests.Add(1)

	// Track per-model metrics
	p.getModelMetrics(req.Model).Requests.Add(1)

	// Store start time for latency calculation
	ctx.Set("metrics_start_time", time.Now())

	return req, nil, nil
}

func (p *MetricsPlugin) PostHook(ctx *plugin.Context, resp *types.ChatResponse, err error) (*types.ChatResponse, error, error) {
	// Calculate latency
	var latencyMs int64
	if startTime, ok := ctx.Get("metrics_start_time"); ok {
		if t, ok := startTime.(time.Time); ok {
			latencyMs = time.Since(t).Milliseconds()
		}
	}

	// Build request metrics
	rm := &RequestMetrics{
		RequestID: ctx.RequestID,
		Model:     ctx.Model,
		Provider:  ctx.Provider,
		LatencyMs: latencyMs,
		Timestamp: time.Now(),
	}

	// Track per-provider metrics
	if ctx.Provider != "" {
		pm := p.getProviderMetrics(ctx.Provider)
		pm.Requests.Add(1)
		pm.TotalLatencyMs.Add(latencyMs)
	}

	// Get per-model metrics
	mm := p.getModelMetrics(ctx.Model)
	mm.TotalLatencyMs.Add(latencyMs)

	if err != nil {
		p.FailedRequests.Add(1)
		mm.Failures.Add(1)
		if ctx.Provider != "" {
			p.getProviderMetrics(ctx.Provider).Failures.Add(1)
		}
		rm.Success = false
		rm.Error = err.Error()

		// Check for rate limit
		if _, ok := err.(*RateLimitError); ok {
			p.RateLimited.Add(1)
		}
	} else {
		p.SuccessfulRequests.Add(1)
		mm.Successes.Add(1)
		if ctx.Provider != "" {
			p.getProviderMetrics(ctx.Provider).Successes.Add(1)
		}
		rm.Success = true

		// Track token usage
		if resp != nil && resp.Usage != nil {
			p.PromptTokens.Add(int64(resp.Usage.PromptTokens))
			p.CompletionTokens.Add(int64(resp.Usage.CompletionTokens))
			p.TotalTokens.Add(int64(resp.Usage.TotalTokens))
			mm.TotalTokens.Add(int64(resp.Usage.TotalTokens))

			rm.PromptTokens = resp.Usage.PromptTokens
			rm.CompletionTokens = resp.Usage.CompletionTokens
			rm.TotalTokens = resp.Usage.TotalTokens
		}
	}

	// Check for cache hit
	if cacheHit, ok := ctx.Get("cache_hit"); ok && cacheHit.(bool) {
		p.CacheHits.Add(1)
		rm.CacheHit = true
	}

	// Track latency
	p.latenciesLock.Lock()
	p.latencies = append(p.latencies, latencyMs)
	p.latenciesLock.Unlock()

	// Call external callback if set
	if p.OnMetrics != nil {
		p.OnMetrics(rm)
	}

	return resp, err, nil
}

func (p *MetricsPlugin) Cleanup() error {
	return nil
}

func (p *MetricsPlugin) getModelMetrics(model string) *ModelMetrics {
	if mm, ok := p.modelMetrics.Load(model); ok {
		return mm.(*ModelMetrics)
	}
	newMM := &ModelMetrics{}
	actual, _ := p.modelMetrics.LoadOrStore(model, newMM)
	return actual.(*ModelMetrics)
}

func (p *MetricsPlugin) getProviderMetrics(provider string) *ProviderMetrics {
	if pm, ok := p.providerMetrics.Load(provider); ok {
		return pm.(*ProviderMetrics)
	}
	newPM := &ProviderMetrics{}
	actual, _ := p.providerMetrics.LoadOrStore(provider, newPM)
	return actual.(*ProviderMetrics)
}

// Snapshot returns a snapshot of current metrics.
type MetricsSnapshot struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TotalTokens        int64
	PromptTokens       int64
	CompletionTokens   int64
	CacheHits          int64
	RateLimited        int64
	AvgLatencyMs       float64
	P50LatencyMs       int64
	P95LatencyMs       int64
	P99LatencyMs       int64
	ModelsStats        map[string]ModelStats
	ProviderStats      map[string]ProviderStats
}

type ModelStats struct {
	Requests     int64
	Successes    int64
	Failures     int64
	TotalTokens  int64
	AvgLatencyMs float64
}

type ProviderStats struct {
	Requests     int64
	Successes    int64
	Failures     int64
	AvgLatencyMs float64
}

// GetSnapshot returns a snapshot of current metrics.
func (p *MetricsPlugin) GetSnapshot() MetricsSnapshot {
	snapshot := MetricsSnapshot{
		TotalRequests:      p.TotalRequests.Load(),
		SuccessfulRequests: p.SuccessfulRequests.Load(),
		FailedRequests:     p.FailedRequests.Load(),
		TotalTokens:        p.TotalTokens.Load(),
		PromptTokens:       p.PromptTokens.Load(),
		CompletionTokens:   p.CompletionTokens.Load(),
		CacheHits:          p.CacheHits.Load(),
		RateLimited:        p.RateLimited.Load(),
		ModelsStats:        make(map[string]ModelStats),
		ProviderStats:      make(map[string]ProviderStats),
	}

	// Calculate latency percentiles
	p.latenciesLock.Lock()
	if len(p.latencies) > 0 {
		sorted := make([]int64, len(p.latencies))
		copy(sorted, p.latencies)
		sortInt64s(sorted)

		var sum int64
		for _, l := range sorted {
			sum += l
		}
		snapshot.AvgLatencyMs = float64(sum) / float64(len(sorted))
		snapshot.P50LatencyMs = percentile(sorted, 50)
		snapshot.P95LatencyMs = percentile(sorted, 95)
		snapshot.P99LatencyMs = percentile(sorted, 99)
	}
	p.latenciesLock.Unlock()

	// Collect model stats
	p.modelMetrics.Range(func(key, value any) bool {
		model := key.(string)
		mm := value.(*ModelMetrics)
		reqs := mm.Requests.Load()
		stats := ModelStats{
			Requests:    reqs,
			Successes:   mm.Successes.Load(),
			Failures:    mm.Failures.Load(),
			TotalTokens: mm.TotalTokens.Load(),
		}
		if reqs > 0 {
			stats.AvgLatencyMs = float64(mm.TotalLatencyMs.Load()) / float64(reqs)
		}
		snapshot.ModelsStats[model] = stats
		return true
	})

	// Collect provider stats
	p.providerMetrics.Range(func(key, value any) bool {
		provider := key.(string)
		pm := value.(*ProviderMetrics)
		reqs := pm.Requests.Load()
		stats := ProviderStats{
			Requests:  reqs,
			Successes: pm.Successes.Load(),
			Failures:  pm.Failures.Load(),
		}
		if reqs > 0 {
			stats.AvgLatencyMs = float64(pm.TotalLatencyMs.Load()) / float64(reqs)
		}
		snapshot.ProviderStats[provider] = stats
		return true
	})

	return snapshot
}

// Reset clears all collected metrics.
func (p *MetricsPlugin) Reset() {
	p.TotalRequests.Store(0)
	p.SuccessfulRequests.Store(0)
	p.FailedRequests.Store(0)
	p.TotalTokens.Store(0)
	p.PromptTokens.Store(0)
	p.CompletionTokens.Store(0)
	p.CacheHits.Store(0)
	p.RateLimited.Store(0)

	p.latenciesLock.Lock()
	p.latencies = p.latencies[:0]
	p.latenciesLock.Unlock()

	p.modelMetrics.Range(func(key, _ any) bool {
		p.modelMetrics.Delete(key)
		return true
	})

	p.providerMetrics.Range(func(key, _ any) bool {
		p.providerMetrics.Delete(key)
		return true
	})
}

// Helper functions

func sortInt64s(a []int64) {
	// Simple insertion sort for latency data
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func percentile(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (len(sorted) - 1) * p / 100
	return sorted[idx]
}

// Ensure MetricsPlugin implements Plugin interface
var _ plugin.Plugin = (*MetricsPlugin)(nil)
