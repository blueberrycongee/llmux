package api //nolint:revive // package name is intentional

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/goccy/go-json"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/mcp"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
	"github.com/blueberrycongee/llmux/pkg/types"
)

type mcpTestProvider struct {
	baseURL string
}

func (p *mcpTestProvider) Name() string {
	return "mock"
}

func (p *mcpTestProvider) SupportedModels() []string {
	return []string{"gpt-4o"}
}

func (p *mcpTestProvider) SupportsModel(model string) bool {
	return model == "gpt-4o"
}

func (p *mcpTestProvider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func (p *mcpTestProvider) ParseResponse(resp *http.Response) (*types.ChatResponse, error) {
	var chatResp types.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}
	return &chatResp, nil
}

func (p *mcpTestProvider) ParseStreamChunk(data []byte) (*types.StreamChunk, error) {
	return nil, nil
}

func (p *mcpTestProvider) MapError(statusCode int, body []byte) error {
	return llmerrors.NewServiceUnavailableError("mock", "", string(body))
}

func (p *mcpTestProvider) SupportEmbedding() bool {
	return false
}

func (p *mcpTestProvider) BuildEmbeddingRequest(ctx context.Context, req *types.EmbeddingRequest) (*http.Request, error) {
	return nil, errors.New("embeddings not supported in mock provider")
}

func (p *mcpTestProvider) ParseEmbeddingResponse(resp *http.Response) (*types.EmbeddingResponse, error) {
	return nil, errors.New("embeddings not supported in mock provider")
}

func TestClientHandlerChatCompletions_MCPToolExecution(t *testing.T) {
	var mu sync.Mutex
	var requests []types.ChatRequest
	var decodeErr error
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req types.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			mu.Lock()
			decodeErr = err
			mu.Unlock()
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		mu.Lock()
		requests = append(requests, req)
		callCount++
		call := callCount
		mu.Unlock()

		var resp types.ChatResponse
		if call == 1 {
			resp = types.ChatResponse{
				ID:     "resp-1",
				Object: "chat.completion",
				Model:  "gpt-4o",
				Choices: []types.Choice{
					{
						Index:        0,
						FinishReason: "tool_calls",
						Message: types.ChatMessage{
							Role:    "assistant",
							Content: json.RawMessage("null"),
							ToolCalls: []types.ToolCall{
								{
									ID:   "call_1",
									Type: "function",
									Function: types.ToolCallFunction{
										Name:      "tool_one",
										Arguments: `{"value":"hello"}`,
									},
								},
							},
						},
					},
				},
			}
		} else {
			content, _ := json.Marshal("done")
			resp = types.ChatResponse{
				ID:     "resp-2",
				Object: "chat.completion",
				Model:  "gpt-4o",
				Choices: []types.Choice{
					{
						Index:        0,
						FinishReason: "stop",
						Message: types.ChatMessage{
							Role:    "assistant",
							Content: content,
						},
					},
				},
			}
		}

		payload, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	prov := &mcpTestProvider{baseURL: server.URL}
	client, err := llmux.New(llmux.WithProviderInstance("mock", prov, []string{"gpt-4o"}))
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	defer func() { _ = client.Close() }()

	manager := mcp.NewMockManager()
	manager.AddMockClient("client-1", "Client 1", mcp.ConnectionTypeHTTP, []types.Tool{
		{
			Type: "function",
			Function: types.ToolFunction{
				Name: "tool_one",
			},
		},
	})

	var execMu sync.Mutex
	var executed []types.ToolCall
	manager.SetExecuteFunc(func(ctx context.Context, toolCall types.ToolCall) (*mcp.ToolExecutionResult, error) {
		execMu.Lock()
		executed = append(executed, toolCall)
		execMu.Unlock()
		return &mcp.ToolExecutionResult{
			ToolCallID: toolCall.ID,
			ToolName:   toolCall.Function.Name,
			Content:    "ok",
		}, nil
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewClientHandler(client, logger, &ClientHandlerConfig{
		MCPManager: manager,
	})

	reqBody, _ := json.Marshal(types.ChatRequest{
		Model: "gpt-4o",
		Messages: []types.ChatMessage{
			{
				Role:    "user",
				Content: json.RawMessage(`"hello"`),
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(reqBody))
	recorder := httptest.NewRecorder()

	handler.ChatCompletions(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	mu.Lock()
	currentDecodeErr := decodeErr
	requestsCopy := append([]types.ChatRequest(nil), requests...)
	mu.Unlock()

	if currentDecodeErr != nil {
		t.Fatalf("decode request: %v", currentDecodeErr)
	}

	execMu.Lock()
	executedCount := len(executed)
	var executedToolName string
	if executedCount > 0 {
		executedToolName = executed[0].Function.Name
	}
	execMu.Unlock()

	if executedCount != 1 {
		t.Fatalf("executed tools = %d, want %d", executedCount, 1)
	}

	if executedToolName != "tool_one" {
		t.Fatalf("tool name = %q, want %q", executedToolName, "tool_one")
	}

	if len(requestsCopy) != 2 {
		t.Fatalf("requests = %d, want %d", len(requestsCopy), 2)
	}

	firstReq := requestsCopy[0]
	if len(firstReq.Tools) != 1 || firstReq.Tools[0].Function.Name != "tool_one" {
		t.Fatalf("injected tools = %#v, want tool_one", firstReq.Tools)
	}

	secondReq := requestsCopy[1]
	foundToolMessage := false
	for _, msg := range secondReq.Messages {
		if msg.Role == "tool" && msg.ToolCallID == "call_1" {
			foundToolMessage = true
			break
		}
	}
	if !foundToolMessage {
		t.Fatalf("tool response message not found in second request")
	}

	var resp types.ChatResponse
	if err := json.NewDecoder(recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Choices) != 1 || string(resp.Choices[0].Message.Content) != `"done"` {
		t.Fatalf("response content = %s, want %q", resp.Choices[0].Message.Content, `"done"`)
	}
}
