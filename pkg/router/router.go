// Package router provides public request routing and load balancing interfaces.
// It supports multiple strategies including simple shuffle, lowest latency, least busy,
// lowest TPM/RPM, lowest cost, and tag-based routing.
package router

import (
	"context"
	"time"

	"github.com/blueberrycongee/llmux/pkg/provider"
)

// Strategy defines the routing strategy type.
type Strategy string

const (
	// StrategySimpleShuffle randomly selects from available deployments.
	// Supports weighted selection based on weight/rpm/tpm parameters.
	StrategySimpleShuffle Strategy = "simple-shuffle"

	// StrategyLowestLatency selects the deployment with lowest average latency.
	// For streaming requests, uses Time To First Token (TTFT) instead.
	StrategyLowestLatency Strategy = "lowest-latency"

	// StrategyLeastBusy selects the deployment with fewest active requests.
	StrategyLeastBusy Strategy = "least-busy"

	// StrategyLowestTPMRPM selects the deployment with lowest TPM/RPM usage.
	// Useful for staying within rate limits across multiple deployments.
	StrategyLowestTPMRPM Strategy = "lowest-tpm-rpm"

	// StrategyLowestCost selects the deployment with lowest cost per token.
	// Considers both input and output token costs.
	StrategyLowestCost Strategy = "lowest-cost"

	// StrategyTagBased filters deployments based on request tags.
	// Only deployments with matching tags are considered.
	StrategyTagBased Strategy = "tag-based"
)

// Router selects the best deployment for a given request.
// It tracks deployment health and performance metrics for intelligent routing.
type Router interface {
	// Pick selects the best available deployment for the given model.
	// Returns ErrNoAvailableDeployment if all deployments are unavailable.
	Pick(ctx context.Context, model string) (*provider.Deployment, error)

	// PickWithContext selects the best deployment using request context.
	// This enables advanced routing strategies like tag-based and streaming-aware routing.
	PickWithContext(ctx context.Context, reqCtx *RequestContext) (*provider.Deployment, error)

	// ReportSuccess records a successful request to update routing metrics.
	ReportSuccess(deployment *provider.Deployment, metrics *ResponseMetrics)

	// ReportFailure records a failed request and potentially triggers cooldown.
	ReportFailure(deployment *provider.Deployment, err error)

	// ReportRequestStart records when a request starts (for least-busy tracking).
	ReportRequestStart(deployment *provider.Deployment)

	// ReportRequestEnd records when a request ends (for least-busy tracking).
	ReportRequestEnd(deployment *provider.Deployment)

	// IsCircuitOpen checks if the circuit breaker is open for a deployment.
	IsCircuitOpen(deployment *provider.Deployment) bool

	// AddDeployment registers a new deployment with the router.
	AddDeployment(deployment *provider.Deployment)

	// AddDeploymentWithConfig registers a deployment with routing configuration.
	AddDeploymentWithConfig(deployment *provider.Deployment, config DeploymentConfig)

	// RemoveDeployment removes a deployment from the router.
	RemoveDeployment(deploymentID string)

	// GetDeployments returns all deployments for a model.
	GetDeployments(model string) []*provider.Deployment

	// GetStats returns the current stats for a deployment.
	GetStats(deploymentID string) *DeploymentStats

	// GetStrategy returns the current routing strategy.
	GetStrategy() Strategy
}

// RequestContext contains request-specific information for routing decisions.
type RequestContext struct {
	// Model is the requested model name
	Model string

	// IsStreaming indicates if this is a streaming request
	IsStreaming bool

	// Tags are request-level tags for tag-based routing
	Tags []string

	// EstimatedInputTokens for TPM/RPM calculations
	EstimatedInputTokens int

	// Metadata contains additional request metadata
	Metadata map[string]string
}

// ResponseMetrics contains metrics from a completed request.
type ResponseMetrics struct {
	// Latency is the total request duration
	Latency time.Duration

	// TimeToFirstToken is the time until first streaming chunk (for streaming requests)
	TimeToFirstToken time.Duration

	// TotalTokens is the total tokens used (input + output)
	TotalTokens int

	// InputTokens is the number of input tokens
	InputTokens int

	// OutputTokens is the number of output tokens
	OutputTokens int

	// Cost is the calculated cost of the request
	Cost float64
}

// DeploymentStats tracks performance metrics for a deployment.
type DeploymentStats struct {
	// Request counts
	TotalRequests  int64
	SuccessCount   int64
	FailureCount   int64
	ActiveRequests int64

	// Latency tracking (rolling window)
	LatencyHistory     []float64 // in milliseconds
	TTFTHistory        []float64 // Time To First Token for streaming
	AvgLatencyMs       float64
	AvgTTFTMs          float64
	MaxLatencyListSize int

	// Usage tracking (per minute)
	CurrentMinuteTPM int64  // Tokens Per Minute
	CurrentMinuteRPM int64  // Requests Per Minute
	CurrentMinuteKey string // Format: "YYYY-MM-DD-HH-MM"

	// Timing
	LastRequestTime time.Time
	CooldownUntil   time.Time
}

// DeploymentConfig contains deployment-specific configuration for routing.
type DeploymentConfig struct {
	// Weight for weighted random selection (simple-shuffle)
	Weight float64

	// TPMLimit for this deployment (0 = unlimited)
	TPMLimit int64

	// RPMLimit for this deployment (0 = unlimited)
	RPMLimit int64

	// InputCostPerToken for cost-based routing
	InputCostPerToken float64

	// OutputCostPerToken for cost-based routing
	OutputCostPerToken float64

	// Tags for tag-based routing
	Tags []string
}

// Config contains router configuration options.
type Config struct {
	// Strategy determines how deployments are selected
	Strategy Strategy

	// CooldownPeriod is how long to wait before retrying a failed deployment
	CooldownPeriod time.Duration

	// LatencyBuffer for lowest-latency: select randomly within this % of lowest
	// e.g., 0.1 means select from deployments within 10% of the lowest latency
	LatencyBuffer float64

	// MaxLatencyListSize is the maximum number of latency samples to keep
	MaxLatencyListSize int

	// MetricsTTL for cached metrics (default: 1 hour)
	MetricsTTL time.Duration

	// EnableTagFiltering enables tag-based deployment filtering
	EnableTagFiltering bool

	// PricingFile is the path to the custom pricing JSON file
	PricingFile string

	// --- Circuit Breaker / Cooldown Configuration (LiteLLM-style) ---

	// FailureThresholdPercent is the failure rate threshold for triggering cooldown.
	// When failure_count / total_requests exceeds this value, the deployment enters cooldown.
	// Default: 0.5 (50%). Set to 1.0 to disable failure-rate based cooldown.
	FailureThresholdPercent float64

	// MinRequestsForThreshold is the minimum number of requests required before
	// applying failure rate based cooldown. This prevents premature cooldown
	// when sample size is too small.
	// Default: 5 requests.
	MinRequestsForThreshold int

	// ImmediateCooldownOn429 triggers immediate cooldown on 429 (Rate Limit) errors.
	// This is typically desired for LLM APIs where 429 indicates the provider
	// needs time to recover from rate limiting.
	// Default: true.
	ImmediateCooldownOn429 bool
}

// DefaultConfig returns sensible default router configuration.
func DefaultConfig() Config {
	return Config{
		Strategy:                StrategySimpleShuffle,
		CooldownPeriod:          60 * time.Second,
		LatencyBuffer:           0.1, // 10% buffer
		MaxLatencyListSize:      10,
		MetricsTTL:              1 * time.Hour,
		EnableTagFiltering:      false,
		FailureThresholdPercent: 0.5,  // 50% failure rate
		MinRequestsForThreshold: 5,    // Minimum 5 requests before checking rate
		ImmediateCooldownOn429:  true, // Immediate cooldown on rate limit
	}
}
