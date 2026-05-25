package api

import (
	"context"
	"fmt"
	"strings"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/mcp"
)

type conversationExecutionTrace struct {
	Iterations       int
	ToolCalls        int
	ToolNames        []string
	MaxIterationsHit bool
}

func buildConversationExecutionTraceResponse(trace conversationExecutionTrace) *ConversationExecutionTraceResponse {
	if trace.Iterations == 0 && trace.ToolCalls == 0 && !trace.MaxIterationsHit && len(trace.ToolNames) == 0 {
		return nil
	}
	return &ConversationExecutionTraceResponse{
		Iterations:       trace.Iterations,
		ToolCalls:        trace.ToolCalls,
		ToolNames:        append([]string(nil), trace.ToolNames...),
		MaxIterationsHit: trace.MaxIterationsHit,
	}
}

// executeConversationChatCompletion mirrors the non-streaming MCP tool loop used by
// the main chat-completions handler, but it does not auto-inject every available tool.
// The conversation router already curates req.Tools via its tool marketplace state.
func executeConversationChatCompletion(ctx context.Context, client *llmux.Client, req *llmux.ChatRequest) (*llmux.ChatResponse, error) {
	resp, _, err := executeConversationChatCompletionWithTrace(ctx, client, req)
	return resp, err
}

func executeConversationChatCompletionWithTrace(ctx context.Context, client *llmux.Client, req *llmux.ChatRequest) (*llmux.ChatResponse, conversationExecutionTrace, error) {
	manager := mcp.GetManager(ctx)
	if manager == nil {
		resp, err := client.ChatCompletion(ctx, req)
		return resp, conversationExecutionTrace{Iterations: 1}, err
	}

	trace := conversationExecutionTrace{}
	maxIterations := mcp.MaxToolIterationsFromContext(ctx)
	for iteration := 0; iteration < maxIterations; iteration++ {
		trace.Iterations = iteration + 1
		resp, err := client.ChatCompletion(ctx, req)
		if err != nil {
			return nil, trace, fmt.Errorf("LLM request failed: %w", err)
		}

		if !mcp.HasToolCalls(resp) {
			return resp, trace, nil
		}

		toolCalls := mcp.GetToolCalls(resp)
		trace.ToolCalls += len(toolCalls)
		for _, toolCall := range toolCalls {
			name := strings.TrimSpace(toolCall.Function.Name)
			if name == "" {
				continue
			}
			trace.ToolNames = append(trace.ToolNames, name)
		}

		results := manager.ExecuteToolCalls(ctx, toolCalls)
		mcp.AppendToolResults(req, resp.Choices[0].Message, results)
	}

	trace.MaxIterationsHit = true
	return nil, trace, fmt.Errorf("exceeded maximum tool iterations (%d)", maxIterations)
}
