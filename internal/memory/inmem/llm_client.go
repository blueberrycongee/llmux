package inmem

import (
	"context"
	"strings"

	"github.com/blueberrycongee/llmux/pkg/types"
	"github.com/goccy/go-json"
)

// RealLLMClientSimulator simulates a real LLM for testing purposes.
// Instead of mocking the interface call, it implements a deterministic logic engine
// that behaves like a very simple LLM (Extracts facts based on rules).
// This avoids network calls but executes real logic flow.
type RealLLMClientSimulator struct{}

func NewRealLLMClientSimulator() *RealLLMClientSimulator {
	return &RealLLMClientSimulator{}
}

func (c *RealLLMClientSimulator) ChatCompletion(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error) {
	// Parse the user prompt from req.Messages
	var userPrompt string
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			// Extract the raw prompt content
			// Assuming it's a quoted string in JSON raw message
			var content string
			if err := json.Unmarshal(msg.Content, &content); err == nil {
				userPrompt = content
			} else {
				userPrompt = string(msg.Content)
			}
		}
	}

	// Simple Rule-Based Logic Engine (Simulating LLM Intelligence)
	// This makes the test "Real" in the sense that data flows through components,
	// but deterministic without external API dependency.

	// Check prompt content to decide output
	// Note: The prompt contains the system instructions and the User Input: "%s"

	var jsonResp string

	// Case 1: Smart Ingestion
	if contains(userPrompt, "I moved to Berlin and I like currywurst") {
		jsonResp = `{"facts": [
			{"content": "User lives in Berlin", "category": "fact", "type": "ADD"},
			{"content": "User likes currywurst", "category": "preference", "type": "ADD"}
		]}`
	} else if contains(userPrompt, "Forget I love Java") { // Case 2: Resolution (Delete)
		jsonResp = `{"facts": [
			{"content": "User loves Java", "category": "preference", "type": "DELETE"}
		]}`
	} else if contains(userPrompt, "I hate Java now") { // Case 3: Resolution (Update)
		jsonResp = `{"facts": [
			{"content": "User hates Java", "category": "preference", "type": "UPDATE"}
		]}`
	} else {
		// Default empty
		jsonResp = `{"facts": []}`
	}

	resp := &types.ChatResponse{
		Choices: []types.Choice{
			{
				Message: types.ChatMessage{
					Role:    "assistant",
					Content: json.RawMessage(jsonResp), // Return raw JSON bytes directly
				},
			},
		},
	}
	return resp, nil
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
