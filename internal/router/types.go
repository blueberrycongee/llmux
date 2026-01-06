// Package router provides request routing and load balancing for LLM deployments.
package router

import (
	"time"

	"github.com/blueberrycongee/llmux/internal/provider"
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

// RouterConfig contains router configuration options.
type RouterConfig struct {
	// Strategy determines how deployments are selected
	Strategy Strategy

	// CooldownPeriod is how long to wait before retrying a failed deployment
	CooldownPeriod time.Duration

	// LatencyBuffer for lowest-latency: select randomly within this % of lowest
	// e.g., 0.1 means select from deployments within 10% of the lowest latency
	LatencyBuffer float64

	// MaxLatencyListSize is the maximum number of latency samples to keep
	MaxLatencyListSize int

	// TTL for cached metrics (default: 1 hour)
	MetricsTTL time.Duration

	// EnableTagFiltering enables tag-based deployment filtering
	EnableTagFiltering bool
}

// DefaultRouterConfig returns sensible default router configuration.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		Strategy:           StrategySimpleShuffle,
		CooldownPeriod:     60 * time.Second,
		LatencyBuffer:      0.1, // 10% buffer
		MaxLatencyListSize: 10,
		MetricsTTL:         1 * time.Hour,
		EnableTagFiltering: false,
	}
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

// DeploymentConfig contains deployment-specific configuration for routing.
type DeploymentConfig struct {
	// Weight for weighted random selection (simple-shuffle)
	Weight float64

	// TPM limit for this deployment (0 = unlimited)
	TPMLimit int64

	// RPM limit for this deployment (0 = unlimited)
	RPMLimit int64

	// InputCostPerToken for cost-based routing
	InputCostPerToken float64

	// OutputCostPerToken for cost-based routing
	OutputCostPerToken float64

	// Tags for tag-based routing
	Tags []string
}

// ExtendedDeployment wraps a deployment with routing-specific configuration.
type ExtendedDeployment struct {
	*provider.Deployment
	Config DeploymentConfig
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
