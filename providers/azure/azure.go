// Package azure provides the Azure OpenAI provider for LLMux library mode.
// Azure OpenAI uses the same API format as OpenAI but with different authentication and endpoints.
package azure

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/types"
)

const (
	ProviderName      = "azure"
	DefaultAPIVersion = "2024-02-15-preview"
)

type Provider struct {
	apiKey      string
	tokenSource provider.TokenSource
	baseURL     string
	apiVersion  string
	models      []string
	headers     map[string]string
}

func New(opts ...Option) *Provider {
	p := &Provider{
		apiVersion: DefaultAPIVersion,
		headers:    make(map[string]string),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("azure provider requires base_url")
	}
	opts := []Option{
		WithAPIKey(cfg.APIKey),
		WithBaseURL(cfg.BaseURL),
		WithModels(cfg.Models...),
	}
	if cfg.TokenSource != nil {
		opts = append(opts, WithTokenSource(cfg.TokenSource))
	}
	p := New(opts...)
	if v, ok := cfg.Headers["api-version"]; ok {
		p.apiVersion = v
	}
	for k, v := range cfg.Headers {
		if k == "api-version" {
			continue
		}
		p.headers[k] = v
	}
	return p, nil
}

func (p *Provider) Name() string              { return ProviderName }
func (p *Provider) SupportedModels() []string { return p.models }
func (p *Provider) SupportsModel(model string) bool {
	for _, m := range p.models {
		if m == model {
			return true
		}
	}
	return false
}

func (p *Provider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
	deploymentName := req.Model
	base, err := url.Parse(strings.TrimSuffix(p.baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse base_url: %w", err)
	}
	base.Path = base.Path + "/openai/deployments/" + url.PathEscape(deploymentName) + "/chat/completions"
	q := base.Query()
	q.Set("api-version", p.apiVersion)
	base.RawQuery = q.Encode()

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Get token from TokenSource or fallback to apiKey
	token, err := provider.GetToken(p.tokenSource, p.apiKey)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", token)
	for k, v := range p.headers {
		httpReq.Header.Set(k, v)
	}
	return httpReq, nil
}

func (p *Provider) ParseResponse(resp *http.Response) (*types.ChatResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	var chatResp types.ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &chatResp, nil
}

func (p *Provider) ParseStreamChunk(data []byte) (*types.StreamChunk, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("[DONE]")) {
		return nil, nil
	}
	if bytes.HasPrefix(trimmed, []byte("data: ")) {
		trimmed = bytes.TrimPrefix(trimmed, []byte("data: "))
	}
	if bytes.Equal(trimmed, []byte("[DONE]")) {
		return nil, nil
	}
	var chunk types.StreamChunk
	if err := json.Unmarshal(trimmed, &chunk); err != nil {
		return nil, fmt.Errorf("unmarshal chunk: %w", err)
	}
	return &chunk, nil
}

func (p *Provider) MapError(statusCode int, body []byte) error {
	var errResp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	message := "unknown error"
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		message = errResp.Error.Message
	}
	switch statusCode {
	case http.StatusUnauthorized:
		return errors.NewAuthenticationError(ProviderName, "", message)
	case http.StatusTooManyRequests:
		return errors.NewRateLimitError(ProviderName, "", message)
	case http.StatusBadRequest:
		return errors.NewInvalidRequestError(ProviderName, "", message)
	case http.StatusNotFound:
		return errors.NewNotFoundError(ProviderName, "", message)
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return errors.NewTimeoutError(ProviderName, "", message)
	case http.StatusServiceUnavailable, http.StatusBadGateway:
		return errors.NewServiceUnavailableError(ProviderName, "", message)
	default:
		return errors.NewInternalError(ProviderName, "", message)
	}
}
