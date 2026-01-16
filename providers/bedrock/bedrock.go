// Package bedrock implements the AWS Bedrock provider adapter.
package bedrock

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/internal/httputil"

	"github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/types"
)

const (
	ProviderName = "bedrock"
)

// Provider implements the AWS Bedrock provider.
type Provider struct {
	cfg    aws.Config
	region string
}

// New creates a new Bedrock provider.
func New(cfg aws.Config) *Provider {
	return &Provider{
		cfg:    cfg,
		region: cfg.Region,
	}
}

// NewFromConfig creates a provider from the global configuration.
func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	// Load AWS config from environment or default profile
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Override region if specified in config metadata or base URL (if applicable)
	// For now, we rely on standard AWS env vars or config file.
	// If BaseURL is set, we might extract region from it, but standard AWS SDK usage prefers env vars.

	return New(awsCfg), nil
}

func (p *Provider) Name() string {
	return ProviderName
}

func (p *Provider) SupportedModels() []string {
	// This list is illustrative; Bedrock supports many models.
	return []string{
		"anthropic.claude-3-sonnet-20240229-v1:0",
		"anthropic.claude-3-opus-20240229-v1:0",
		"anthropic.claude-3-haiku-20240307-v1:0",
		"meta.llama3-8b-instruct-v1:0",
		"meta.llama3-70b-instruct-v1:0",
	}
}

func (p *Provider) SupportsModel(model string) bool {
	return true // Bedrock supports many models, assume true and let API fail if invalid
}

func (p *Provider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
	// 1. Construct Payload
	payload, err := p.constructPayload(req)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	// 2. Determine Endpoint
	// Format: https://bedrock-runtime.{region}.amazonaws.com/model/{modelId}/invoke
	// Or: .../invoke-with-response-stream
	method := "invoke"
	if req.Stream {
		method = "invoke-with-response-stream"
	}

	url := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/%s", p.region, req.Model, method)

	// 3. Create Request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// 4. Sign Request (SigV4)
	signer := v4.NewSigner()
	creds, err := p.cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("retrieve credentials: %w", err)
	}

	payloadHash := sha256.Sum256(bodyBytes)
	hexHash := hex.EncodeToString(payloadHash[:])

	if err := signer.SignHTTP(ctx, creds, httpReq, hexHash, "bedrock", p.region, time.Now()); err != nil {
		return nil, fmt.Errorf("sign request: %w", err)
	}

	// 5. Attach Response Transformer for Streaming
	if req.Stream {
		ctx = context.WithValue(ctx, provider.ResponseTransformerKey, provider.ResponseTransformer(p.transformStream))
		httpReq = httpReq.WithContext(ctx)
	}

	return httpReq, nil
}

func (p *Provider) constructPayload(req *types.ChatRequest) (any, error) {
	if strings.HasPrefix(req.Model, "anthropic.claude-3") {
		return p.constructClaude3Payload(req)
	}
	if strings.HasPrefix(req.Model, "meta.llama3") {
		return p.constructLlama3Payload(req), nil
	}
	// Fallback or error
	return nil, fmt.Errorf("unsupported model family for %s", req.Model)
}

// Claude 3 Payload
type claude3Payload struct {
	AnthropicVersion string    `json:"anthropic_version"`
	MaxTokens        int       `json:"max_tokens"`
	Messages         []message `json:"messages"`
	System           string    `json:"system,omitempty"`
	Temperature      *float64  `json:"temperature,omitempty"`
	TopP             *float64  `json:"top_p,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (p *Provider) constructClaude3Payload(req *types.ChatRequest) (*claude3Payload, error) {
	payload := &claude3Payload{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        req.MaxTokens,
		Temperature:      req.Temperature,
		TopP:             req.TopP,
	}
	if payload.MaxTokens == 0 {
		payload.MaxTokens = 2048 // Default
	}

	for _, m := range req.Messages {
		var text string
		if err := json.Unmarshal(m.Content, &text); err != nil {
			return nil, fmt.Errorf("unmarshal content: %w", err)
		}

		if m.Role == "system" {
			payload.System = text
			continue
		}
		payload.Messages = append(payload.Messages, message{
			Role:    m.Role,
			Content: text,
		})
	}
	return payload, nil
}

// Llama 3 Payload
type llama3Payload struct {
	Prompt      string   `json:"prompt"`
	MaxGenLen   int      `json:"max_gen_len,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
}

func (p *Provider) constructLlama3Payload(req *types.ChatRequest) *llama3Payload {
	// Simple prompt construction for Llama 3 Instruct
	var prompt strings.Builder
	prompt.WriteString("<|begin_of_text|>")
	for _, m := range req.Messages {
		prompt.WriteString(fmt.Sprintf("<|start_header_id|>%s<|end_header_id|>\n\n%s<|eot_id|>", m.Role, m.Content))
	}
	prompt.WriteString("<|start_header_id|>assistant<|end_header_id|>\n\n")

	payload := &llama3Payload{
		Prompt:      prompt.String(),
		MaxGenLen:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}
	if payload.MaxGenLen == 0 {
		payload.MaxGenLen = 512
	}
	return payload
}

func (p *Provider) ParseResponse(resp *http.Response) (*types.ChatResponse, error) {
	body, err := httputil.ReadLimitedBody(resp.Body, httputil.DefaultMaxResponseBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Determine model from request? We don't have request here.
	// But we can guess from response structure or just try parsing.
	// Ideally, we should know the model. But ParseResponse interface doesn't pass it.
	// We can try to detect based on JSON fields.

	// Try Claude 3
	var claudeResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &claudeResp); err == nil && len(claudeResp.Content) > 0 {
		contentBytes, _ := json.Marshal(claudeResp.Content[0].Text)
		return &types.ChatResponse{
			Object: "chat.completion",
			Choices: []types.Choice{
				{
					Index: 0,
					Message: types.ChatMessage{
						Role:    "assistant",
						Content: contentBytes,
					},
					FinishReason: "stop", // Simplified
				},
			},
			Usage: &types.Usage{
				PromptTokens:     claudeResp.Usage.InputTokens,
				CompletionTokens: claudeResp.Usage.OutputTokens,
				TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
			},
		}, nil
	}

	// Try Llama 3
	var llamaResp struct {
		Generation string `json:"generation"`
	}
	if err := json.Unmarshal(body, &llamaResp); err == nil && llamaResp.Generation != "" {
		contentBytes, _ := json.Marshal(llamaResp.Generation)
		return &types.ChatResponse{
			Object: "chat.completion",
			Choices: []types.Choice{
				{
					Index: 0,
					Message: types.ChatMessage{
						Role:    "assistant",
						Content: contentBytes,
					},
					FinishReason: "stop",
				},
			},
		}, nil
	}

	return nil, fmt.Errorf("unknown response format")
}

func (p *Provider) ParseStreamChunk(data []byte) (*types.StreamChunk, error) {
	// This is called by stream.go with the data from our transformed stream.
	// Our transformed stream sends standard SSE with JSON data.
	// The data here is the JSON payload of the chunk.

	// Try Claude 3 Stream Event
	// {"type":"content_block_delta", "delta":{"type":"text_delta", "text":"..."}}
	var event map[string]any
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}

	eventType, _ := event["type"].(string)

	// Claude 3
	if eventType == "content_block_delta" {
		delta, _ := event["delta"].(map[string]any)
		text, _ := delta["text"].(string)
		return &types.StreamChunk{
			Object: "chat.completion.chunk",
			Choices: []types.StreamChoice{{
				Index: 0,
				Delta: types.StreamDelta{Content: text},
			}},
		}, nil
	}
	if eventType == "message_stop" {
		return &types.StreamChunk{
			Object: "chat.completion.chunk",
			Choices: []types.StreamChoice{{
				Index:        0,
				FinishReason: "stop",
			}},
		}, nil
	}

	// Llama 3 Stream Event (assumed format, might need adjustment)
	// {"generation": "...", "stop_reason": ...}
	if gen, ok := event["generation"].(string); ok {
		return &types.StreamChunk{
			Object: "chat.completion.chunk",
			Choices: []types.StreamChoice{{
				Index: 0,
				Delta: types.StreamDelta{Content: gen},
			}},
		}, nil
	}

	return nil, nil
}

func (p *Provider) MapError(statusCode int, body []byte) error {
	return errors.NewInternalError(ProviderName, "", fmt.Sprintf("bedrock error %d: %s", statusCode, string(body)))
}

// transformStream decodes AWS EventStream and re-encodes as SSE.
func (p *Provider) transformStream(body io.ReadCloser) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		defer body.Close()
		defer pw.Close() //nolint:errcheck // pipe writer close error is ignored

		decoder := eventstream.NewDecoder()
		buf := make([]byte, 1024*64) // 64KB buffer
		for {
			msg, err := decoder.Decode(body, buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				// Log error?
				break
			}

			// Check event type
			// Bedrock sends ":event-type" header.
			// Value should be "chunk".
			// Payload is the JSON bytes.

			// We just assume payload is the JSON we want.
			// Format as SSE
			if _, err := fmt.Fprintf(pw, "data: %s\n\n", msg.Payload); err != nil {
				return // Pipe closed by reader
			}
		}
		_, _ = fmt.Fprintf(pw, "data: [DONE]\n\n")
	}()
	return pr
}
