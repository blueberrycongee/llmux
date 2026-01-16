package embedding

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/internal/httputil"
)

// AzureEmbedder implements Embedder using Azure OpenAI's embedding API.
type AzureEmbedder struct {
	client     *http.Client
	apiKey     string
	apiBase    string
	apiVersion string
	deployment string
	dimension  int
}

// AzureConfig holds configuration for Azure OpenAI embedder.
type AzureConfig struct {
	APIKey     string
	APIBase    string // e.g., "https://<resource>.openai.azure.com"
	APIVersion string // e.g., "2024-02-01"
	Deployment string // Deployment name for the embedding model
	Dimension  int
	Timeout    time.Duration
}

// DefaultAzureConfig returns sensible defaults for Azure embedder.
func DefaultAzureConfig() AzureConfig {
	return AzureConfig{
		APIVersion: "2024-02-01",
		Dimension:  1536,
		Timeout:    30 * time.Second,
	}
}

// NewAzureEmbedder creates a new Azure OpenAI embedder.
func NewAzureEmbedder(cfg AzureConfig) (*AzureEmbedder, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("azure api_key is required")
	}
	if cfg.APIBase == "" {
		return nil, fmt.Errorf("azure api_base is required")
	}
	if cfg.Deployment == "" {
		return nil, fmt.Errorf("azure deployment is required")
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = "2024-02-01"
	}
	if cfg.Dimension <= 0 {
		cfg.Dimension = 1536
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &AzureEmbedder{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		apiKey:     cfg.APIKey,
		apiBase:    cfg.APIBase,
		apiVersion: cfg.APIVersion,
		deployment: cfg.Deployment,
		dimension:  cfg.Dimension,
	}, nil
}

// Embed generates an embedding for a single text.
func (e *AzureEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	embeddings, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts.
func (e *AzureEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := azureEmbeddingRequest{
		Input: texts,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Azure OpenAI URL format:
	// https://<resource>.openai.azure.com/openai/deployments/<deployment>/embeddings?api-version=<version>
	url := fmt.Sprintf("%s/openai/deployments/%s/embeddings?api-version=%s",
		e.apiBase, e.deployment, e.apiVersion)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := httputil.ReadLimitedBody(resp.Body, httputil.DefaultMaxResponseBodyBytes)
		return nil, fmt.Errorf("embedding failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var embResp azureEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Sort by index to ensure correct order
	embeddings := make([][]float64, len(texts))
	for _, data := range embResp.Data {
		if data.Index < len(embeddings) {
			embeddings[data.Index] = data.Embedding
		}
	}

	return embeddings, nil
}

// Model returns the deployment name.
func (e *AzureEmbedder) Model() string {
	return e.deployment
}

// Dimension returns the embedding dimension.
func (e *AzureEmbedder) Dimension() int {
	return e.dimension
}

// Azure API types (same structure as OpenAI)

type azureEmbeddingRequest struct {
	Input []string `json:"input"`
}

type azureEmbeddingResponse struct {
	Object string               `json:"object"`
	Data   []azureEmbeddingData `json:"data"`
	Model  string               `json:"model"`
	Usage  azureEmbeddingUsage  `json:"usage"`
}

type azureEmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type azureEmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
