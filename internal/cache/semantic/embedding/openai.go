package embedding

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/goccy/go-json"
)

// OpenAIEmbedder implements Embedder using OpenAI's embedding API.
// It directly calls the OpenAI API, bypassing LLMux's request chain
// to avoid circular caching issues.
type OpenAIEmbedder struct {
	client    *http.Client
	apiKey    string
	apiBase   string
	model     string
	dimension int
}

// OpenAIConfig holds configuration for OpenAI embedder.
type OpenAIConfig struct {
	APIKey    string
	APIBase   string
	Model     string
	Dimension int
	Timeout   time.Duration
}

// DefaultOpenAIConfig returns sensible defaults for OpenAI embedder.
func DefaultOpenAIConfig() OpenAIConfig {
	return OpenAIConfig{
		APIBase:   "https://api.openai.com/v1",
		Model:     "text-embedding-ada-002",
		Dimension: 1536,
		Timeout:   30 * time.Second,
	}
}

// NewOpenAIEmbedder creates a new OpenAI embedder.
func NewOpenAIEmbedder(cfg OpenAIConfig) (*OpenAIEmbedder, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("openai api_key is required")
	}
	if cfg.APIBase == "" {
		cfg.APIBase = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "text-embedding-ada-002"
	}
	if cfg.Dimension <= 0 {
		cfg.Dimension = 1536
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &OpenAIEmbedder{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		apiKey:    cfg.APIKey,
		apiBase:   cfg.APIBase,
		model:     cfg.Model,
		dimension: cfg.Dimension,
	}, nil
}

// Embed generates an embedding for a single text.
func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
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
func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := openAIEmbeddingRequest{
		Model: e.model,
		Input: texts,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/embeddings", e.apiBase)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.apiKey))

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var embResp openAIEmbeddingResponse
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

// Model returns the embedding model name.
func (e *OpenAIEmbedder) Model() string {
	return e.model
}

// Dimension returns the embedding dimension.
func (e *OpenAIEmbedder) Dimension() int {
	return e.dimension
}

// OpenAI API types

type openAIEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openAIEmbeddingResponse struct {
	Object string                `json:"object"`
	Data   []openAIEmbeddingData `json:"data"`
	Model  string                `json:"model"`
	Usage  openAIEmbeddingUsage  `json:"usage"`
}

type openAIEmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type openAIEmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
