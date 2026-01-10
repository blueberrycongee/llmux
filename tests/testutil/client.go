package testutil

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// TestClient provides helper methods for making API requests in tests.
type TestClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

// NewTestClient creates a new test client.
func NewTestClient(baseURL string) *TestClient {
	return &TestClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// WithAPIKey sets the API key for requests.
func (c *TestClient) WithAPIKey(apiKey string) *TestClient {
	c.apiKey = apiKey
	return c
}

// BaseURL returns the client's base URL.
func (c *TestClient) BaseURL() string {
	return c.baseURL
}

// HTTPClient returns the underlying http.Client.
func (c *TestClient) HTTPClient() *http.Client {
	return c.httpClient
}

// APIKey returns the configured API key.
func (c *TestClient) APIKey() string {
	return c.apiKey
}

// ChatCompletionRequest represents a chat completion request.
type ChatCompletionRequest struct {
	Model          string            `json:"model"`
	Messages       []ChatMessage     `json:"messages"`
	Stream         bool              `json:"stream,omitempty"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
	Temperature    *float64          `json:"temperature,omitempty"`
	Tools          []json.RawMessage `json:"tools,omitempty"`
	ToolChoice     any               `json:"tool_choice,omitempty"`
	ResponseFormat *ResponseFormat   `json:"response_format,omitempty"`
	Stop           []string          `json:"stop,omitempty"`
	TopP           *float64          `json:"top_p,omitempty"`
	N              int               `json:"n,omitempty"`
	Seed           *int              `json:"seed,omitempty"`
}

// ResponseFormat specifies the output format.
type ResponseFormat struct {
	Type       string      `json:"type"` // "text", "json_object", "json_schema"
	JSONSchema *JSONSchema `json:"json_schema,omitempty"`
}

// JSONSchema for structured outputs.
type JSONSchema struct {
	Name   string `json:"name"`
	Schema any    `json:"schema"`
	Strict bool   `json:"strict,omitempty"`
}

// ChatMessage represents a message in the conversation.
type ChatMessage struct {
	Role       string            `json:"role"`
	Content    string            `json:"content,omitempty"`
	ToolCalls  []ToolCallMessage `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
}

// ToolCallMessage represents a tool call in a message.
type ToolCallMessage struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Function FunctionCallMessage `json:"function"`
}

// FunctionCallMessage represents a function call.
type FunctionCallMessage struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatCompletionResponse represents a chat completion response.
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role      string         `json:"role"`
			Content   string         `json:"content"`
			ToolCalls []ToolCallResp `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *types.Usage `json:"usage,omitempty"`
}

// ToolCallResp represents a tool call in the response.
type ToolCallResp struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ErrorResponse represents an API error response.
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

// StreamChunk represents a streaming response chunk.
type StreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
}

// ChatCompletion sends a chat completion request.
func (c *TestClient) ChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, *http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, resp, nil
	}

	var chatResp ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		resp.Body.Close()
		return nil, resp, fmt.Errorf("decode response: %w", err)
	}
	resp.Body.Close()

	return &chatResp, resp, nil
}

// ChatCompletionWithTools sends a chat completion request with tools/function calling.
func (c *TestClient) ChatCompletionWithTools(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, *http.Response, error) {
	return c.ChatCompletion(ctx, req)
}

// ChatCompletionWithFormat sends a chat completion request with response_format.
func (c *TestClient) ChatCompletionWithFormat(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, *http.Response, error) {
	return c.ChatCompletion(ctx, req)
}

// ChatCompletionWithToolResult sends a chat completion with tool results in messages.
func (c *TestClient) ChatCompletionWithToolResult(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, *http.Response, error) {
	return c.ChatCompletion(ctx, req)
}

// ChatCompletionStream sends a streaming chat completion request.
func (c *TestClient) ChatCompletionStream(ctx context.Context, req *ChatCompletionRequest) (*StreamReader, *http.Response, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, resp, nil
	}

	return &StreamReader{
		reader: bufio.NewReader(resp.Body),
		body:   resp.Body,
	}, resp, nil
}

// HealthCheck checks the health endpoint.
func (c *TestClient) HealthCheck(ctx context.Context, path string) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	return c.httpClient.Do(httpReq)
}

// ListModels lists available models.
func (c *TestClient) ListModels(ctx context.Context) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/v1/models", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	return c.httpClient.Do(httpReq)
}

// GetMetrics fetches the metrics endpoint.
func (c *TestClient) GetMetrics(ctx context.Context) (string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/metrics", http.NoBody)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	return string(body), nil
}

// PostJSON sends a POST request with JSON body.
func (c *TestClient) PostJSON(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	return c.httpClient.Do(req)
}

// GetJSON sends a GET request.
func (c *TestClient) GetJSON(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	return c.httpClient.Do(req)
}

// StreamReader reads SSE events from a streaming response.
type StreamReader struct {
	reader *bufio.Reader
	body   io.ReadCloser
}

// Next reads the next chunk from the stream.
// Returns nil, io.EOF when the stream is complete.
func (r *StreamReader) Next() (*StreamChunk, error) {
	for {
		line, err := r.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return nil, io.EOF
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return nil, fmt.Errorf("unmarshal chunk: %w", err)
		}

		return &chunk, nil
	}
}

// Close closes the stream.
func (r *StreamReader) Close() error {
	return r.body.Close()
}

// CollectContent reads all chunks and returns the accumulated content.
func (r *StreamReader) CollectContent() (string, error) {
	var content strings.Builder
	for {
		chunk, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if len(chunk.Choices) > 0 {
			content.WriteString(chunk.Choices[0].Delta.Content)
		}
	}
	return content.String(), nil
}

// EmbeddingRequest represents an embedding request for testing.
type EmbeddingRequest struct {
	Model          string `json:"model"`
	Input          any    `json:"input"` // string, []string, []int, or [][]int
	EncodingFormat string `json:"encoding_format,omitempty"`
	User           string `json:"user,omitempty"`
	Dimensions     int    `json:"dimensions,omitempty"`
}

// EmbeddingResponse represents an embedding response for testing.
type EmbeddingResponse struct {
	Object string            `json:"object"`
	Data   []EmbeddingObject `json:"data"`
	Model  string            `json:"model"`
	Usage  types.Usage       `json:"usage"`
}

// EmbeddingObject represents a single embedding object.
type EmbeddingObject struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// Embedding sends an embedding request.
func (c *TestClient) Embedding(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, *http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, resp, nil
	}

	var embResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		resp.Body.Close()
		return nil, resp, fmt.Errorf("decode response: %w", err)
	}
	resp.Body.Close()

	return &embResp, resp, nil
}
