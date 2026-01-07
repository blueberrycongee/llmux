package semantic

import (
	"strings"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// MessagesToPrompt converts a slice of ChatMessage to a single prompt string.
// This is used to generate embeddings for semantic similarity matching.
// The conversion follows LiteLLM's get_str_from_messages logic.
func MessagesToPrompt(messages []types.ChatMessage) string {
	var sb strings.Builder

	for i, msg := range messages {
		if i > 0 {
			sb.WriteString("\n")
		}

		// Add role prefix for context
		sb.WriteString(msg.Role)
		sb.WriteString(": ")

		// Extract content - handle both string and array formats
		content := extractContent(msg.Content)
		sb.WriteString(content)
	}

	return sb.String()
}

// extractContent extracts text content from ChatMessage.Content.
// Content can be either a string or an array of content parts.
func extractContent(content json.RawMessage) string {
	if len(content) == 0 {
		return ""
	}

	// Try to unmarshal as string first
	var strContent string
	if err := json.Unmarshal(content, &strContent); err == nil {
		return strContent
	}

	// Try to unmarshal as array of content parts
	var parts []contentPart
	if err := json.Unmarshal(content, &parts); err == nil {
		var sb strings.Builder
		for _, part := range parts {
			if part.Type == "text" && part.Text != "" {
				if sb.Len() > 0 {
					sb.WriteString(" ")
				}
				sb.WriteString(part.Text)
			}
		}
		return sb.String()
	}

	// Fallback: return raw content as string
	return string(content)
}

// contentPart represents a part of multimodal content.
type contentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// PromptHash generates a simple hash key for the prompt.
// This is used as a fallback identifier when vector search is not available.
func PromptHash(prompt string) string {
	// Use a simple approach - first 64 chars + length
	if len(prompt) <= 64 {
		return prompt
	}
	return prompt[:64]
}
