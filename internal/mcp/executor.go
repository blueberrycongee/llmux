package mcp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// SendFunc is a function type for sending chat requests.
type SendFunc func(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error)

// AgentExecutor handles the agentic loop for tool execution.
type AgentExecutor struct {
	manager       Manager
	maxIterations int
	logger        *slog.Logger
}

// NewAgentExecutor creates a new agent executor.
func NewAgentExecutor(manager Manager, maxIterations int, logger *slog.Logger) *AgentExecutor {
	if maxIterations <= 0 {
		maxIterations = MaxToolIterations
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &AgentExecutor{
		manager:       manager,
		maxIterations: maxIterations,
		logger:        logger,
	}
}

// Execute runs the agentic loop, executing tools until completion or max iterations.
func (e *AgentExecutor) Execute(ctx context.Context, req *types.ChatRequest, sendFn SendFunc) (*types.ChatResponse, error) {
	// Inject MCP tools into the request
	if injector, ok := e.manager.(ToolInjector); ok {
		injector.InjectTools(ctx, req)
	}

	for iteration := 0; iteration < e.maxIterations; iteration++ {
		e.logger.Debug(MCPLogPrefix+" agentic loop iteration",
			"iteration", iteration+1,
			"max", e.maxIterations,
		)

		// Send request to LLM
		resp, err := sendFn(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("LLM request failed: %w", err)
		}

		// Check if we need to execute tools
		if !HasToolCalls(resp) {
			e.logger.Debug(MCPLogPrefix+" agentic loop completed",
				"iterations", iteration+1,
				"finish_reason", resp.Choices[0].FinishReason,
			)
			return resp, nil
		}

		// Execute tool calls
		toolCalls := GetToolCalls(resp)
		e.logger.Debug(MCPLogPrefix+" executing tools",
			"count", len(toolCalls),
		)

		results := e.manager.ExecuteToolCalls(ctx, toolCalls)

		// Append results to conversation
		AppendToolResults(req, resp.Choices[0].Message, results)
	}

	return nil, fmt.Errorf("exceeded maximum tool iterations (%d)", e.maxIterations)
}

// ExecuteOnce executes a single round of tool calls without looping.
// Returns the tool execution results and whether there were any tool calls.
func (e *AgentExecutor) ExecuteOnce(ctx context.Context, resp *types.ChatResponse) ([]ToolExecutionResult, bool) {
	if !HasToolCalls(resp) {
		return nil, false
	}

	toolCalls := GetToolCalls(resp)
	results := e.manager.ExecuteToolCalls(ctx, toolCalls)

	return results, true
}
