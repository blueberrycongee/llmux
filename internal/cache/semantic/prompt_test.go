package semantic

import (
	"testing"

	"github.com/blueberrycongee/llmux/pkg/types"
	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
)

func TestMessagesToPrompt(t *testing.T) {
	tests := []struct {
		name     string
		messages []types.ChatMessage
		expected string
	}{
		{
			name:     "should handle empty messages",
			messages: []types.ChatMessage{},
			expected: "",
		},
		{
			name: "should convert single message",
			messages: []types.ChatMessage{
				{Role: "user", Content: json.RawMessage(`"Hello, world!"`)},
			},
			expected: "user: Hello, world!",
		},
		{
			name: "should convert multiple messages",
			messages: []types.ChatMessage{
				{Role: "system", Content: json.RawMessage(`"You are a helpful assistant."`)},
				{Role: "user", Content: json.RawMessage(`"What is the capital of France?"`)},
			},
			expected: "system: You are a helpful assistant.\nuser: What is the capital of France?",
		},
		{
			name: "should handle conversation with assistant",
			messages: []types.ChatMessage{
				{Role: "user", Content: json.RawMessage(`"Hi"`)},
				{Role: "assistant", Content: json.RawMessage(`"Hello! How can I help you?"`)},
				{Role: "user", Content: json.RawMessage(`"Tell me a joke"`)},
			},
			expected: "user: Hi\nassistant: Hello! How can I help you?\nuser: Tell me a joke",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MessagesToPrompt(tt.messages)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractContent(t *testing.T) {
	tests := []struct {
		name     string
		content  json.RawMessage
		expected string
	}{
		{
			name:     "should handle empty content",
			content:  json.RawMessage{},
			expected: "",
		},
		{
			name:     "should handle string content",
			content:  json.RawMessage(`"Hello, world!"`),
			expected: "Hello, world!",
		},
		{
			name:     "should handle array content with text",
			content:  json.RawMessage(`[{"type": "text", "text": "Hello"}, {"type": "text", "text": "World"}]`),
			expected: "Hello World",
		},
		{
			name:     "should handle array content with mixed types",
			content:  json.RawMessage(`[{"type": "text", "text": "Describe this image"}, {"type": "image_url", "image_url": {"url": "http://example.com/image.png"}}]`),
			expected: "Describe this image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractContent(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPromptHash(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		expected string
	}{
		{
			name:     "should handle short prompt",
			prompt:   "Hello",
			expected: "Hello",
		},
		{
			name:     "should truncate long prompt",
			prompt:   "This is a very long prompt that exceeds sixty-four characters and should be truncated",
			expected: "This is a very long prompt that exceeds sixty-four characters an",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PromptHash(tt.prompt)
			assert.Equal(t, tt.expected, result)
		})
	}
}
