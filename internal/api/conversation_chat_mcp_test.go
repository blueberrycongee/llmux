package api

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/goccy/go-json"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/mcp"
	"github.com/blueberrycongee/llmux/pkg/types"
)

type conversationToolLoopTestProvider struct {
	baseURL string
}

func (p *conversationToolLoopTestProvider) Name() string {
	return "mock-conversation"
}

func (p *conversationToolLoopTestProvider) SupportedModels() []string {
	return []string{"gpt-4o"}
}

func (p *conversationToolLoopTestProvider) SupportsModel(model string) bool {
	return model == "gpt-4o"
}

func (p *conversationToolLoopTestProvider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
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

func (p *conversationToolLoopTestProvider) ParseResponse(resp *http.Response) (*types.ChatResponse, error) {
	var chatResp types.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}
	return &chatResp, nil
}

func (p *conversationToolLoopTestProvider) ParseStreamChunk(data []byte) (*types.StreamChunk, error) {
	return nil, nil
}

func (p *conversationToolLoopTestProvider) MapError(statusCode int, body []byte) error {
	return llmux.NewServiceUnavailableError("mock-conversation", "", string(body))
}

func (p *conversationToolLoopTestProvider) SupportEmbedding() bool {
	return false
}

func (p *conversationToolLoopTestProvider) BuildEmbeddingRequest(ctx context.Context, req *types.EmbeddingRequest) (*http.Request, error) {
	return nil, io.EOF
}

func (p *conversationToolLoopTestProvider) ParseEmbeddingResponse(resp *http.Response) (*types.EmbeddingResponse, error) {
	return nil, io.EOF
}

func TestManagementHandlerConversationChat_ExecutesMCPToolCalls(t *testing.T) {
	restoreCandidates := withIsolatedCandidateStores()
	defer restoreCandidates()
	restoreMemory := withIsolatedConversationMemoryStore()
	defer restoreMemory()
	restoreAgents := withIsolatedConversationAgentStore([]ConversationAgent{
		{
			ID:              "tool-agent",
			Name:            "Tool Agent",
			Description:     "Test agent with filesystem tool access.",
			Category:        "coding",
			Provider:        "openai",
			Model:           "gpt-4o",
			CandidateModels: []CandidateModel{{Provider: "openai", Model: "gpt-4o", Weight: 1}},
			Strategy:        "least-busy",
			Capabilities:    []string{"coding", "filesystem"},
			SystemPrompt:    "Use tools when needed.",
			Tools:           []string{"preset-filesystem-list"},
			Enabled:         true,
			CreatedAt:       "2026-04-16T00:00:00Z",
			UpdatedAt:       "2026-04-16T00:00:00Z",
		},
	})
	defer restoreAgents()

	var mu sync.Mutex
	var requests []types.ChatRequest
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req types.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
				ID:     "conv-resp-1",
				Object: "chat.completion",
				Model:  "openai/gpt-4o",
				Choices: []types.Choice{
					{
						Index:        0,
						FinishReason: "tool_calls",
						Message: types.ChatMessage{
							Role:    "assistant",
							Content: json.RawMessage(`""`),
							ToolCalls: []types.ToolCall{
								{
									ID:   "call_1",
									Type: "function",
									Function: types.ToolCallFunction{
										Name:      "list_directory",
										Arguments: `{"path":"."}`,
									},
								},
							},
						},
					},
				},
			}
		} else {
			resp = types.ChatResponse{
				ID:     "conv-resp-2",
				Object: "chat.completion",
				Model:  "openai/gpt-4o",
				Choices: []types.Choice{
					{
						Index:        0,
						FinishReason: "stop",
						Message: types.ChatMessage{
							Role:    "assistant",
							Content: json.RawMessage(`"directory contents: README.md"`),
						},
					},
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := &conversationToolLoopTestProvider{baseURL: server.URL}
	client, err := llmux.New(llmux.WithProviderInstance("openai", provider, []string{"gpt-4o"}))
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	defer func() { _ = client.Close() }()

	manager := mcp.NewMockManager()
	manager.AddMockClient("filesystem", "Filesystem MCP", mcp.ConnectionTypeHTTP, []types.Tool{
		{
			Type: "function",
			Function: types.ToolFunction{
				Name: "list_directory",
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
			Content:    "README.md",
		}, nil
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewManagementHandler(nil, nil, logger, NewClientSwapper(client), nil, nil)

	body, _ := json.Marshal(AgentChatRequest{
		SessionID: "session-conv-mcp",
		Messages: []ConversationTurn{
			{
				Role:    "user",
				Content: "@tool-agent list the current directory",
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/control/conversation/chat", bytes.NewReader(body))
	req = req.WithContext(mcp.WithManager(req.Context(), manager))
	recorder := httptest.NewRecorder()

	handler.ConversationChat(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	execMu.Lock()
	executedCount := len(executed)
	executedToolName := ""
	if executedCount > 0 {
		executedToolName = executed[0].Function.Name
	}
	execMu.Unlock()

	if executedCount != 1 {
		t.Fatalf("executed tools = %d, want %d", executedCount, 1)
	}
	if executedToolName != "list_directory" {
		t.Fatalf("tool name = %q, want %q", executedToolName, "list_directory")
	}

	mu.Lock()
	requestsCopy := append([]types.ChatRequest(nil), requests...)
	mu.Unlock()

	if len(requestsCopy) != 2 {
		t.Fatalf("requests = %d, want %d", len(requestsCopy), 2)
	}

	firstReq := requestsCopy[0]
	if len(firstReq.Tools) != 1 || firstReq.Tools[0].Function.Name != "list_directory" {
		t.Fatalf("first request tools = %#v, want list_directory", firstReq.Tools)
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

	var result AgentChatResponse
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !result.Succeeded {
		t.Fatalf("expected succeeded response, got error %q", result.ErrorMessage)
	}
	if !strings.Contains(result.AssistantMessage, "README.md") {
		t.Fatalf("assistant_message = %q, want final tool-resolved answer", result.AssistantMessage)
	}
}

func withIsolatedConversationMemoryStore() func() {
	conversationMemoryStore.Lock()
	oldRecords := append([]RouteMemoryRecord(nil), conversationMemoryStore.records...)
	conversationMemoryStore.records = nil
	conversationMemoryStore.Unlock()

	return func() {
		conversationMemoryStore.Lock()
		conversationMemoryStore.records = oldRecords
		conversationMemoryStore.Unlock()
	}
}

func withIsolatedConversationAgentStore(agents []ConversationAgent) func() {
	conversationAgentStore.Lock()
	oldAgents := append([]ConversationAgent(nil), conversationAgentStore.agents...)
	conversationAgentStore.agents = append([]ConversationAgent(nil), agents...)
	conversationAgentStore.Unlock()

	return func() {
		conversationAgentStore.Lock()
		conversationAgentStore.agents = oldAgents
		conversationAgentStore.Unlock()
	}
}
