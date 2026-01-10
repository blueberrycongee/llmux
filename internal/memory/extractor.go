package memory

import (
	"context"
	"fmt"

	"github.com/blueberrycongee/llmux/pkg/types"
	"github.com/goccy/go-json"
)

// LLMClient defines the interface for interacting with an LLM provider.
// This decouples the memory module from the core provider implementation.
type LLMClient interface {
	ChatCompletion(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error)
}

// Extractor uses an LLM to extract structured facts from raw text.
type Extractor struct {
	client LLMClient
	model  string
}

// NewExtractor creates a new Extractor instance.
func NewExtractor(client LLMClient, model string) *Extractor {
	return &Extractor{
		client: client,
		model:  model,
	}
}

// Extract analyzes the input text and returns a list of structured facts.
func (e *Extractor) Extract(ctx context.Context, text string) ([]Fact, error) {
	prompt := fmt.Sprintf(`You are a Memory Extraction AI. Your goal is to extract key facts, preferences, and events from the user's input to be stored in long-term memory.

Rules:
1. Extract independent, standalone facts.
2. Ignore casual conversation (e.g., "Hello", "How are you").
3. Categorize each fact as "preference", "fact", or "event".
4. Determine the action type: "ADD" (new info), "UPDATE" (change existing info), or "DELETE" (remove info).
5. Output JSON only.

User Input: "%s"

Output Format:
{
  "facts": [
    { "content": "User prefers Python", "category": "preference", "type": "ADD" },
    { "content": "User moved to Berlin", "category": "fact", "type": "UPDATE" },
    { "content": "User hates Java", "category": "preference", "type": "DELETE" }
  ]
}
`, text)

	req := &types.ChatRequest{
		Model: e.model,
		Messages: []types.ChatMessage{
			{
				Role:    "system",
				Content: json.RawMessage(`"You are a helpful assistant that outputs JSON."`),
			},
			{
				Role:    "user",
				Content: json.RawMessage(fmt.Sprintf("%q", prompt)), // Use %q to safely quote the prompt string
			},
		},
		ResponseFormat: &types.ResponseFormat{Type: "json_object"},
		Temperature:    float64Ptr(0.0), // Deterministic output
	}

	resp, err := e.client.ChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm extraction failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from llm")
	}

	content := resp.Choices[0].Message.Content
	// Unmarshal content
	var result ExtractionResult

	// ChatMessage.Content is json.RawMessage ([]byte).
	// Depending on how it was unmarshaled, it might be a JSON string or raw bytes.
	// If it's a raw JSON object string like "{\"facts\": ...}", we need to unquote it first if it's double encoded,
	// or just unmarshal it if it's the raw bytes of the object.

	// Typically, ChatMessage.Content comes from the provider as a string.
	// pkg/types/request.go: Content is json.RawMessage.

	// Let's assume the LLM provider puts the raw JSON string into Content.
	// If Content is `{"facts": [...]}` (bytes), we can unmarshal directly.
	// If Content is `"{\"facts\": [...]}"` (quoted string bytes), we need to unquote.

	// Safe approach: Try to unmarshal into string first, then into ExtractionResult.
	// Or try direct unmarshal.

	var jsonStr string
	if err := json.Unmarshal(content, &jsonStr); err == nil {
		// It was a quoted string, now we have the inner JSON
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			return nil, fmt.Errorf("failed to parse inner extraction result: %w", err)
		}
	} else {
		// It wasn't a quoted string, maybe it's the raw object directly
		if err := json.Unmarshal(content, &result); err != nil {
			return nil, fmt.Errorf("failed to parse extraction result: %w", err)
		}
	}

	return result.Facts, nil
}

func float64Ptr(v float64) *float64 {
	return &v
}
