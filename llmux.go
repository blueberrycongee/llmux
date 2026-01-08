// Package llmux provides a high-performance LLM gateway as a Go library.
// It supports 44+ LLM providers with unified OpenAI-compatible API.
//
// LLMux can be used in two modes:
//   - Library Mode: Import and use directly in your Go application
//   - Gateway Mode: Run as a standalone HTTP proxy server
//
// Basic usage:
//
//	client, err := llmux.New(
//	    llmux.WithProvider(llmux.ProviderConfig{
//	        Name:   "openai",
//	        Type:   "openai",
//	        APIKey: os.Getenv("OPENAI_API_KEY"),
//	        Models: []string{"gpt-4o", "gpt-4o-mini"},
//	    }),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	resp, err := client.ChatCompletion(ctx, &llmux.ChatRequest{
//	    Model: "gpt-4o",
//	    Messages: []llmux.ChatMessage{
//	        {Role: "user", Content: json.RawMessage(`"Hello!"`)},
//	    },
//	})
package llmux

import (
	"github.com/blueberrycongee/llmux/internal/plugin"
	"github.com/blueberrycongee/llmux/pkg/cache"
	"github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
	"github.com/blueberrycongee/llmux/pkg/types"
)

// Version is the current version of LLMux.
const Version = "2.0.0"

// Re-export core request/response types for convenience.
// Users can use llmux.ChatRequest instead of types.ChatRequest.
type (
	// ChatRequest represents an OpenAI-compatible chat completion request.
	ChatRequest = types.ChatRequest

	// ChatResponse represents an OpenAI-compatible chat completion response.
	ChatResponse = types.ChatResponse

	// ChatMessage represents a single message in the conversation.
	ChatMessage = types.ChatMessage

	// StreamChunk represents a single chunk in a streaming response.
	StreamChunk = types.StreamChunk

	// Tool represents a function that the model can call.
	Tool = types.Tool

	// ToolCall represents a function call made by the model.
	ToolCall = types.ToolCall

	// ToolFunction describes a callable function.
	ToolFunction = types.ToolFunction

	// ToolCallFunction contains the function name and arguments.
	ToolCallFunction = types.ToolCallFunction

	// Usage contains token usage statistics for the request.
	Usage = types.Usage

	// Choice represents a single completion choice.
	Choice = types.Choice

	// StreamChoice represents a choice in a streaming response.
	StreamChoice = types.StreamChoice

	// StreamDelta contains the incremental content in a stream chunk.
	StreamDelta = types.StreamDelta

	// ResponseFormat specifies the output format for the model.
	ResponseFormat = types.ResponseFormat

	// StreamOptions specifies options for streaming responses.
	StreamOptions = types.StreamOptions
)

// Re-export provider types.
type (
	// Provider defines the interface that all LLM provider adapters must implement.
	Provider = provider.Provider

	// Deployment represents a specific model deployment configuration.
	Deployment = provider.Deployment

	// ProviderConfig contains provider-specific configuration.
	ProviderConfig = provider.Config

	// ProviderFactory creates provider instances from configuration.
	ProviderFactory = provider.Factory
)

// Re-export router types.
type (
	// Router selects the best deployment for a given request.
	Router = router.Router

	// Strategy defines the routing strategy type.
	Strategy = router.Strategy

	// RequestContext contains request-specific information for routing decisions.
	RequestContext = router.RequestContext

	// ResponseMetrics contains metrics from a completed request.
	ResponseMetrics = router.ResponseMetrics

	// DeploymentStats tracks performance metrics for a deployment.
	DeploymentStats = router.DeploymentStats

	// DeploymentConfig contains deployment-specific configuration for routing.
	DeploymentConfig = router.DeploymentConfig

	// RouterConfig contains router configuration options.
	RouterConfig = router.Config
)

// Re-export cache types.
type (
	// Cache defines the interface for all cache implementations.
	Cache = cache.Cache

	// CacheType represents the type of cache backend.
	CacheType = cache.Type

	// CacheEntry represents a single cache entry for pipeline operations.
	CacheEntry = cache.Entry

	// CacheStats holds cache statistics for monitoring.
	CacheStats = cache.Stats

	// CacheControl allows per-request cache behavior customization.
	CacheControl = cache.Control
)

// Re-export plugin types.
// For full plugin functionality, import github.com/blueberrycongee/llmux/internal/plugin or pkg/plugin.
type (
	// PluginInterface defines the interface for LLMux plugins.
	// Plugins can intercept and modify requests/responses at various lifecycle points.
	PluginInterface = plugin.Plugin

	// PluginContext provides execution context for plugins.
	PluginContext = plugin.Context

	// PluginShortCircuit represents a plugin's decision to short-circuit the request.
	PluginShortCircuit = plugin.ShortCircuit

	// PluginPipelineConfig holds configuration for the plugin pipeline.
	PluginPipelineConfig = plugin.PipelineConfig
)

// Re-export error types.
type (
	// LLMError represents a standardized error from an LLM provider.
	LLMError = errors.LLMError
)

// Re-export routing strategy constants.
const (
	// StrategySimpleShuffle randomly selects from available deployments.
	StrategySimpleShuffle = router.StrategySimpleShuffle

	// StrategyShuffle is an alias for StrategySimpleShuffle.
	StrategyShuffle = router.StrategySimpleShuffle

	// StrategyRoundRobin is an alias for StrategySimpleShuffle (Go doesn't have true round-robin).
	StrategyRoundRobin = router.StrategySimpleShuffle

	// StrategyLowestLatency selects the deployment with lowest average latency.
	StrategyLowestLatency = router.StrategyLowestLatency

	// StrategyLeastBusy selects the deployment with fewest active requests.
	StrategyLeastBusy = router.StrategyLeastBusy

	// StrategyLowestTPMRPM selects the deployment with lowest TPM/RPM usage.
	StrategyLowestTPMRPM = router.StrategyLowestTPMRPM

	// StrategyLowestCost selects the deployment with lowest cost per token.
	StrategyLowestCost = router.StrategyLowestCost

	// StrategyTagBased filters deployments based on request tags.
	StrategyTagBased = router.StrategyTagBased
)

// Re-export cache type constants.
const (
	// CacheTypeLocal is an in-memory cache.
	CacheTypeLocal = cache.TypeLocal

	// CacheTypeRedis is a Redis cache.
	CacheTypeRedis = cache.TypeRedis

	// CacheTypeDual is an in-memory + Redis dual cache.
	CacheTypeDual = cache.TypeDual

	// CacheTypeSemantic is a semantic cache with vector similarity.
	CacheTypeSemantic = cache.TypeSemantic
)

// Re-export error type constants.
const (
	TypeAuthentication     = errors.TypeAuthentication
	TypeRateLimit          = errors.TypeRateLimit
	TypeInvalidRequest     = errors.TypeInvalidRequest
	TypeNotFound           = errors.TypeNotFound
	TypeTimeout            = errors.TypeTimeout
	TypeServiceUnavailable = errors.TypeServiceUnavailable
	TypeInternalError      = errors.TypeInternalError
	TypeContextLength      = errors.TypeContextLength
	TypeContentPolicy      = errors.TypeContentPolicy
)

// Re-export error factory functions.
var (
	NewAuthenticationError     = errors.NewAuthenticationError
	NewRateLimitError          = errors.NewRateLimitError
	NewInvalidRequestError     = errors.NewInvalidRequestError
	NewNotFoundError           = errors.NewNotFoundError
	NewTimeoutError            = errors.NewTimeoutError
	NewServiceUnavailableError = errors.NewServiceUnavailableError
	NewInternalError           = errors.NewInternalError
)

// Model represents an available model from a provider.
type Model struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Object   string `json:"object,omitempty"`
}

// EmbeddingRequest represents an embedding request (future support).
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbeddingResponse represents an embedding response (future support).
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  *EmbeddingUsage `json:"usage,omitempty"`
}

// EmbeddingData represents a single embedding vector.
type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbeddingUsage contains token usage for embedding requests.
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
