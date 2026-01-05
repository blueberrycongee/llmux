package streaming

import (
	"testing"
)

func TestOpenAIParser_ParseChunk(t *testing.T) {
	parser := &OpenAIParser{}

	tests := []struct {
		name        string
		input       []byte
		wantContent string
		wantNil     bool
	}{
		{
			name:    "should return nil for empty input",
			input:   []byte(""),
			wantNil: true,
		},
		{
			name:    "should return nil for DONE marker",
			input:   []byte("data: [DONE]"),
			wantNil: true,
		},
		{
			name:        "should parse valid chunk",
			input:       []byte(`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"}}]}`),
			wantContent: "Hello",
			wantNil:     false,
		},
		{
			name:        "should handle chunk without data prefix",
			input:       []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"World"}}]}`),
			wantContent: "World",
			wantNil:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, err := parser.ParseChunk(tt.input)
			if err != nil {
				t.Errorf("ParseChunk() error = %v", err)
				return
			}
			if tt.wantNil {
				if chunk != nil {
					t.Errorf("ParseChunk() = %v, want nil", chunk)
				}
				return
			}
			if chunk == nil {
				t.Error("ParseChunk() returned nil, want chunk")
				return
			}
			if len(chunk.Choices) == 0 {
				t.Error("ParseChunk() returned chunk with no choices")
				return
			}
			if chunk.Choices[0].Delta.Content != tt.wantContent {
				t.Errorf("content = %v, want %v", chunk.Choices[0].Delta.Content, tt.wantContent)
			}
		})
	}
}

func TestAnthropicParser_ParseChunk(t *testing.T) {
	parser := &AnthropicParser{}

	tests := []struct {
		name        string
		input       []byte
		wantContent string
		wantRole    string
		wantNil     bool
	}{
		{
			name:    "should return nil for empty input",
			input:   []byte(""),
			wantNil: true,
		},
		{
			name:    "should skip event lines",
			input:   []byte("event: content_block_delta"),
			wantNil: true,
		},
		{
			name:     "should parse message_start",
			input:    []byte(`data: {"type":"message_start","message":{"id":"msg_123","model":"claude-3"}}`),
			wantRole: "assistant",
			wantNil:  false,
		},
		{
			name:        "should parse content_block_delta",
			input:       []byte(`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}`),
			wantContent: "Hello",
			wantNil:     false,
		},
		{
			name:    "should skip ping events",
			input:   []byte(`data: {"type":"ping"}`),
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, err := parser.ParseChunk(tt.input)
			if err != nil {
				t.Errorf("ParseChunk() error = %v", err)
				return
			}
			if tt.wantNil {
				if chunk != nil {
					t.Errorf("ParseChunk() = %v, want nil", chunk)
				}
				return
			}
			if chunk == nil {
				t.Error("ParseChunk() returned nil, want chunk")
				return
			}
			if len(chunk.Choices) == 0 {
				t.Error("ParseChunk() returned chunk with no choices")
				return
			}
			if tt.wantContent != "" && chunk.Choices[0].Delta.Content != tt.wantContent {
				t.Errorf("content = %v, want %v", chunk.Choices[0].Delta.Content, tt.wantContent)
			}
			if tt.wantRole != "" && chunk.Choices[0].Delta.Role != tt.wantRole {
				t.Errorf("role = %v, want %v", chunk.Choices[0].Delta.Role, tt.wantRole)
			}
		})
	}
}

func TestAnthropicParser_MessageDelta(t *testing.T) {
	parser := &AnthropicParser{}

	input := []byte(`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`)
	chunk, err := parser.ParseChunk(input)
	if err != nil {
		t.Fatalf("ParseChunk() error = %v", err)
	}
	if chunk == nil {
		t.Fatal("ParseChunk() returned nil")
	}
	if chunk.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason = %v, want stop", chunk.Choices[0].FinishReason)
	}
}

func TestGeminiParser_ParseChunk(t *testing.T) {
	parser := &GeminiParser{}

	tests := []struct {
		name        string
		input       []byte
		wantContent string
		wantNil     bool
	}{
		{
			name:    "should return nil for empty input",
			input:   []byte(""),
			wantNil: true,
		},
		{
			name:        "should parse JSON object",
			input:       []byte(`{"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]}`),
			wantContent: "Hello",
			wantNil:     false,
		},
		{
			name:        "should handle array wrapper",
			input:       []byte(`[{"candidates":[{"content":{"parts":[{"text":"World"}]}}]}]`),
			wantContent: "World",
			wantNil:     false,
		},
		{
			name:    "should return nil for empty candidates",
			input:   []byte(`{"candidates":[]}`),
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk, err := parser.ParseChunk(tt.input)
			if err != nil {
				t.Errorf("ParseChunk() error = %v", err)
				return
			}
			if tt.wantNil {
				if chunk != nil {
					t.Errorf("ParseChunk() = %v, want nil", chunk)
				}
				return
			}
			if chunk == nil {
				t.Error("ParseChunk() returned nil, want chunk")
				return
			}
			if len(chunk.Choices) == 0 {
				t.Error("ParseChunk() returned chunk with no choices")
				return
			}
			if chunk.Choices[0].Delta.Content != tt.wantContent {
				t.Errorf("content = %v, want %v", chunk.Choices[0].Delta.Content, tt.wantContent)
			}
		})
	}
}

func TestGeminiParser_FinishReason(t *testing.T) {
	parser := &GeminiParser{}

	input := []byte(`{"candidates":[{"content":{"parts":[{"text":"Done"}]},"finishReason":"STOP"}]}`)
	chunk, err := parser.ParseChunk(input)
	if err != nil {
		t.Fatalf("ParseChunk() error = %v", err)
	}
	if chunk == nil {
		t.Fatal("ParseChunk() returned nil")
	}
	if chunk.Choices[0].FinishReason != "stop" {
		t.Errorf("finish_reason = %v, want stop", chunk.Choices[0].FinishReason)
	}
}

func TestGetParser(t *testing.T) {
	tests := []struct {
		provider string
		wantType string
	}{
		{"openai", "*streaming.OpenAIParser"},
		{"azure", "*streaming.OpenAIParser"},
		{"anthropic", "*streaming.AnthropicParser"},
		{"gemini", "*streaming.GeminiParser"},
		{"unknown", "*streaming.OpenAIParser"}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			parser := GetParser(tt.provider)
			if parser == nil {
				t.Error("GetParser() returned nil")
			}
		})
	}
}

func TestMapAnthropicStopReason(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"end_turn", "stop"},
		{"max_tokens", "length"},
		{"stop_sequence", "stop"},
		{"tool_use", "tool_calls"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapAnthropicStopReason(tt.input)
			if got != tt.want {
				t.Errorf("mapAnthropicStopReason(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMapGeminiFinishReason(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"STOP", "stop"},
		{"MAX_TOKENS", "length"},
		{"SAFETY", "content_filter"},
		{"RECITATION", "content_filter"},
		{"OTHER", "OTHER"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapGeminiFinishReason(tt.input)
			if got != tt.want {
				t.Errorf("mapGeminiFinishReason(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
