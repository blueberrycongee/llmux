// Package vertexai implements the Google Vertex AI provider adapter.
package vertexai

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/goccy/go-json"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/blueberrycongee/llmux/internal/httputil"

	"github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/types"
)

const (
	ProviderName = "vertexai"
)

type Provider struct {
	projectID string
	location  string
	tokenSrc  oauth2.TokenSource
}

func New(projectID, location string, tokenSrc oauth2.TokenSource) *Provider {
	return &Provider{
		projectID: projectID,
		location:  location,
		tokenSrc:  tokenSrc,
	}
}

func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	ctx := context.Background()
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("find default credentials: %w", err)
	}

	projectID := creds.ProjectID
	if cfg.Headers["project_id"] != "" {
		projectID = cfg.Headers["project_id"]
	}
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required (env var or config header)")
	}

	location := "us-central1"
	if cfg.Headers["location"] != "" {
		location = cfg.Headers["location"]
	}

	return New(projectID, location, creds.TokenSource), nil
}

func (p *Provider) Name() string {
	return ProviderName
}

func (p *Provider) SupportedModels() []string {
	return []string{"gemini-1.5-pro", "gemini-1.5-flash", "gemini-1.0-pro"}
}

func (p *Provider) SupportsModel(model string) bool {
	return strings.HasPrefix(model, "gemini-")
}

func (p *Provider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
	// 1. Get Token
	token, err := p.tokenSrc.Token()
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	// 2. Construct URL
	// https://{location}-aiplatform.googleapis.com/v1/projects/{project}/locations/{location}/publishers/google/models/{model}:streamGenerateContent
	method := "generateContent"
	if req.Stream {
		method = "streamGenerateContent"
	}

	// Handle model versions if needed, e.g. gemini-1.5-pro-001
	modelID := req.Model

	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:%s",
		p.location, p.projectID, p.location, modelID, method)

	// 3. Construct Payload (Gemini format)
	geminiReq, err := p.convertPayload(req)
	if err != nil {
		return nil, fmt.Errorf("convert payload: %w", err)
	}
	bodyBytes, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	// 4. Create Request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token.AccessToken)

	return httpReq, nil
}

type geminiRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig *genConfig      `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type genConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
}

func (p *Provider) convertPayload(req *types.ChatRequest) (*geminiRequest, error) {
	contents := make([]geminiContent, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}

		var text string
		// Try to unmarshal as string first
		if err := json.Unmarshal(m.Content, &text); err != nil {
			// If it fails, it might be a complex object (multimodal), which we don't fully support yet in this simple adapter.
			// Or it might be a string that wasn't properly quoted? No, RawMessage should be valid JSON.
			// Let's try to treat it as a string directly if unmarshal fails, or just error out.
			// For now, let's assume text content.
			return nil, fmt.Errorf("unmarshal content for message role %s: %w", m.Role, err)
		}

		contents = append(contents, geminiContent{
			Role: role,
			Parts: []geminiPart{{
				Text: text,
			}},
		})
	}

	return &geminiRequest{
		Contents: contents,
		GenerationConfig: &genConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
			TopP:            req.TopP,
		},
	}, nil
}

func (p *Provider) ParseResponse(resp *http.Response) (*types.ChatResponse, error) {
	body, err := httputil.ReadLimitedBody(resp.Body, httputil.DefaultMaxResponseBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	content := ""
	if len(geminiResp.Candidates[0].Content.Parts) > 0 {
		content = geminiResp.Candidates[0].Content.Parts[0].Text
	}

	contentBytes, _ := json.Marshal(content)

	return &types.ChatResponse{
		Object: "chat.completion",
		Choices: []types.Choice{{
			Index: 0,
			Message: types.ChatMessage{
				Role:    "assistant",
				Content: contentBytes,
			},
			FinishReason: strings.ToLower(geminiResp.Candidates[0].FinishReason),
		}},
	}, nil
}

func (p *Provider) ParseStreamChunk(data []byte) (*types.StreamChunk, error) {
	// Vertex AI returns JSON array stream.
	// The internal/streaming/parsers.go GeminiParser handles this.
	// We just need to implement this method to satisfy interface,
	// but actually the Parser in streaming package does the work?
	// Wait, internal/api/handler.go calls streaming.GetParser().
	// And streaming.GetParser("vertexai") should return GeminiParser.
	// But Provider interface requires ParseStreamChunk.
	// The Forwarder calls parser.ParseChunk.
	// If parser is GeminiParser, it parses.
	// Does it call Provider.ParseStreamChunk?
	// No, Forwarder uses the Parser interface.

	// However, stream.go (StreamReader) calls s.provider.ParseStreamChunk.
	// We are using Handler (internal/api/handler.go) which uses Forwarder.
	// BUT, if we use library mode (pkg/llm/client.go), it uses StreamReader.
	// So we MUST implement ParseStreamChunk here for library mode compatibility.

	// Reuse logic from GeminiParser?
	// We can't import internal/streaming.
	// So we duplicate logic.

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if bytes.HasPrefix(trimmed, []byte("[")) {
		trimmed = bytes.TrimPrefix(trimmed, []byte("["))
	}
	if bytes.HasSuffix(trimmed, []byte("]")) {
		trimmed = bytes.TrimSuffix(trimmed, []byte("]"))
	}
	if bytes.HasPrefix(trimmed, []byte(",")) {
		trimmed = bytes.TrimPrefix(trimmed, []byte(","))
	}
	trimmed = bytes.TrimSpace(trimmed)
	if len(trimmed) == 0 {
		return nil, nil
	}

	var resp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(trimmed, &resp); err != nil {
		return nil, nil
	}

	if len(resp.Candidates) == 0 {
		return nil, nil
	}

	content := ""
	for _, part := range resp.Candidates[0].Content.Parts {
		content += part.Text
	}

	return &types.StreamChunk{
		Object: "chat.completion.chunk",
		Choices: []types.StreamChoice{{
			Index: 0,
			Delta: types.StreamDelta{Content: content},
		}},
	}, nil
}

func (p *Provider) MapError(statusCode int, body []byte) error {
	return errors.NewInternalError(ProviderName, "", fmt.Sprintf("vertexai error %d: %s", statusCode, string(body)))
}
