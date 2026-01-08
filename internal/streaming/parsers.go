package streaming

import (
	"bytes"
	"fmt"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// OpenAIParser parses OpenAI SSE stream chunks.
// Format: data: {"id":"...","object":"chat.completion.chunk",...}\n\n
type OpenAIParser struct{}

// ParseChunk implements ChunkParser for OpenAI format.
func (p *OpenAIParser) ParseChunk(data []byte) (*types.StreamChunk, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	// Remove "data: " prefix
	if bytes.HasPrefix(trimmed, []byte(SSEDataPrefix)) {
		trimmed = bytes.TrimPrefix(trimmed, []byte(SSEDataPrefix))
	}

	// Skip [DONE] marker
	if bytes.Equal(trimmed, []byte(SSEDone)) {
		return nil, nil
	}

	var chunk types.StreamChunk
	if err := json.Unmarshal(trimmed, &chunk); err != nil {
		return nil, fmt.Errorf("unmarshal openai chunk: %w", err)
	}

	return &chunk, nil
}

// AnthropicParser parses Anthropic SSE stream chunks.
// Format: event: content_block_delta\ndata: {"type":"content_block_delta",...}\n\n
type AnthropicParser struct {
	currentID    string
	currentModel string
}

// ParseChunk implements ChunkParser for Anthropic format.
func (p *AnthropicParser) ParseChunk(data []byte) (*types.StreamChunk, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	// Skip event lines
	if bytes.HasPrefix(trimmed, []byte("event:")) {
		return nil, nil
	}

	// Remove "data: " prefix
	if bytes.HasPrefix(trimmed, []byte(SSEDataPrefix)) {
		trimmed = bytes.TrimPrefix(trimmed, []byte(SSEDataPrefix))
	}

	// Skip [DONE] marker
	if bytes.Equal(trimmed, []byte(SSEDone)) {
		return nil, nil
	}

	var event map[string]any
	if err := json.Unmarshal(trimmed, &event); err != nil {
		return nil, nil // Skip unparseable events
	}

	eventType, ok := event["type"].(string)
	if !ok {
		return nil, nil
	}
	return p.handleEvent(eventType, event)
}

func (p *AnthropicParser) handleEvent(eventType string, event map[string]any) (*types.StreamChunk, error) {
	switch eventType {
	case "message_start":
		return p.handleMessageStart(event)
	case "content_block_delta":
		return p.handleContentDelta(event)
	case "message_delta":
		return p.handleMessageDelta(event)
	case "message_stop", "content_block_start", "content_block_stop", "ping":
		return nil, nil
	default:
		return nil, nil
	}
}

func (p *AnthropicParser) handleMessageStart(event map[string]any) (*types.StreamChunk, error) {
	msg, ok := event["message"].(map[string]any)
	if !ok {
		return nil, nil
	}

	if id, ok := msg["id"].(string); ok {
		p.currentID = id
	}
	if model, ok := msg["model"].(string); ok {
		p.currentModel = model
	}

	return &types.StreamChunk{
		ID:     p.currentID,
		Object: "chat.completion.chunk",
		Model:  p.currentModel,
		Choices: []types.StreamChoice{{
			Index: 0,
			Delta: types.StreamDelta{Role: "assistant"},
		}},
	}, nil
}

func (p *AnthropicParser) handleContentDelta(event map[string]any) (*types.StreamChunk, error) {
	delta, ok := event["delta"].(map[string]any)
	if !ok {
		return nil, nil
	}

	if delta["type"] != "text_delta" {
		return nil, nil
	}

	text, ok := delta["text"].(string)
	if !ok {
		return nil, nil
	}
	return &types.StreamChunk{
		ID:     p.currentID,
		Object: "chat.completion.chunk",
		Model:  p.currentModel,
		Choices: []types.StreamChoice{{
			Index: 0,
			Delta: types.StreamDelta{Content: text},
		}},
	}, nil
}

func (p *AnthropicParser) handleMessageDelta(event map[string]any) (*types.StreamChunk, error) {
	delta, ok := event["delta"].(map[string]any)
	if !ok {
		return nil, nil
	}

	stopReason, ok := delta["stop_reason"].(string)
	if !ok || stopReason == "" {
		return nil, nil
	}

	return &types.StreamChunk{
		ID:     p.currentID,
		Object: "chat.completion.chunk",
		Model:  p.currentModel,
		Choices: []types.StreamChoice{{
			Index:        0,
			FinishReason: mapAnthropicStopReason(stopReason),
		}},
	}, nil
}

func mapAnthropicStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "stop_sequence":
		return "stop"
	case "tool_use":
		return "tool_calls"
	default:
		return reason
	}
}

// GeminiParser parses Gemini streaming responses.
// Gemini uses JSON array streaming, not standard SSE format.
type GeminiParser struct{}

// ParseChunk implements ChunkParser for Gemini format.
func (p *GeminiParser) ParseChunk(data []byte) (*types.StreamChunk, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	// Gemini returns JSON objects directly (may be wrapped in array)
	// Strip array brackets if present
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

	var resp geminiStreamResponse
	if err := json.Unmarshal(trimmed, &resp); err != nil {
		return nil, nil // Skip unparseable chunks
	}

	if len(resp.Candidates) == 0 {
		return nil, nil
	}

	candidate := resp.Candidates[0]
	var textContent string
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			textContent += part.Text
		}
	}

	chunk := &types.StreamChunk{
		Object: "chat.completion.chunk",
		Choices: []types.StreamChoice{{
			Index: 0,
			Delta: types.StreamDelta{Content: textContent},
		}},
	}

	if candidate.FinishReason != "" {
		chunk.Choices[0].FinishReason = mapGeminiFinishReason(candidate.FinishReason)
	}

	return chunk, nil
}

// geminiStreamResponse represents a Gemini streaming response chunk.
type geminiStreamResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

func mapGeminiFinishReason(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION":
		return "content_filter"
	default:
		return reason
	}
}

// AzureParser is an alias for OpenAIParser since Azure uses the same format.
type AzureParser = OpenAIParser

// GetParser returns the appropriate parser for a provider.
func GetParser(providerName string) ChunkParser {
	switch providerName {
	case "openai", "azure":
		return &OpenAIParser{}
	case "anthropic":
		return &AnthropicParser{}
	case "gemini":
		return &GeminiParser{}
	default:
		return &OpenAIParser{} // Default to OpenAI format
	}
}
