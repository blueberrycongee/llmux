// Package provider defines the interface for LLM provider adapters.
// Each provider (OpenAI, Anthropic, etc.) implements this interface
// to handle request/response transformation and API communication.
package provider

import (
	"context"
	"net/http"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// Provider defines the interface that all LLM provider adapters must implement.
// It handles the complete lifecycle of an LLM request: building, sending, and parsing.
type Provider interface {
	// Name returns the provider identifier (e.g., "openai", "anthropic").
	Name() string

	// SupportedModels returns the list of models this provider can handle.
	SupportedModels() []string

	// SupportsModel checks if the provider supports the given model.
	SupportsModel(model string) bool

	// BuildRequest transforms a unified ChatRequest into a provider-specific HTTP request.
	// It handles parameter mapping, header setup, and body serialization.
	BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error)

	// ParseResponse transforms a provider-specific response into a unified ChatResponse.
	// It handles response deserialization and format normalization.
	ParseResponse(resp *http.Response) (*types.ChatResponse, error)

	// ParseStreamChunk parses a single SSE chunk from a streaming response.
	// Returns nil, nil for keep-alive or empty chunks.
	ParseStreamChunk(data []byte) (*types.StreamChunk, error)

	// MapError converts a provider-specific error response into a standardized LLMError.
	MapError(statusCode int, body []byte) error

	// SupportEmbedding checks if the provider supports embedding requests.
	SupportEmbedding() bool

	// BuildEmbeddingRequest transforms a unified EmbeddingRequest into a provider-specific HTTP request.
	BuildEmbeddingRequest(ctx context.Context, req *types.EmbeddingRequest) (*http.Request, error)

	// ParseEmbeddingResponse transforms a provider-specific response into a unified EmbeddingResponse.
	ParseEmbeddingResponse(resp *http.Response) (*types.EmbeddingResponse, error)
}

// StreamHandler handles streaming responses from LLM providers.
// It provides an iterator-like interface for processing SSE events.
type StreamHandler interface {
	// Next returns the next chunk from the stream.
	// Returns io.EOF when the stream is complete.
	Next() (*types.StreamChunk, error)

	// Close releases resources associated with the stream.
	Close() error
}

// Deployment represents a specific model deployment configuration.
// It contains all information needed to route requests to a provider.
type Deployment struct {
	ID            string            `json:"id"`
	ProviderName  string            `json:"provider_name"`
	ModelName     string            `json:"model_name"`
	ModelAlias    string            `json:"model_alias,omitempty"`
	BaseURL       string            `json:"base_url"`
	APIKey        string            `json:"-"` // Never serialize
	MaxConcurrent int               `json:"max_concurrent"`
	Timeout       int               `json:"timeout_seconds"`
	Priority      int               `json:"priority"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// ProviderFactory creates provider instances from configuration.
type ProviderFactory func(cfg ProviderConfig) (Provider, error)

// ProviderConfig contains provider-specific configuration.
type ProviderConfig struct {
	Name                string
	Type                string
	APIKey              string
	TokenSource         TokenSource
	BaseURL             string
	AllowPrivateBaseURL bool
	Models              []string
	MaxConcurrent       int
	TimeoutSec          int
	Headers             map[string]string
}
