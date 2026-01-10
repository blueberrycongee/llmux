// Package tokenizer provides token counting helpers for LLM requests and responses.
package tokenizer

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/pkoukk/tiktoken-go"

	"github.com/blueberrycongee/llmux/pkg/types"
)

var (
	encodingCache sync.Map
	defaultOnce   sync.Once
	defaultEnc    *tiktoken.Tiktoken
)

// CountTextTokens returns the token count for the given text using tiktoken.
// If no encoding is available, it falls back to a conservative len/4 estimate.
func CountTextTokens(model, text string) int {
	if text == "" {
		return 0
	}
	enc := getEncoding(model)
	if enc == nil {
		return len(text) / 4
	}
	return len(enc.Encode(text, nil, nil))
}

// EstimatePromptTokens estimates prompt tokens for chat requests.
// This uses tiktoken on message content and adds a small overhead per message.
func EstimatePromptTokens(model string, req *types.ChatRequest) int {
	if req == nil {
		return 0
	}

	total := 0
	for _, msg := range req.Messages {
		total += estimateMessageTokens(model, msg)
	}

	if len(req.Tools) > 0 {
		if toolsJSON, err := json.Marshal(req.Tools); err == nil {
			total += CountTextTokens(model, string(toolsJSON))
		}
	}

	if len(req.ToolChoice) > 0 {
		total += CountTextTokens(model, string(req.ToolChoice))
	}

	// Small reply primer overhead used by common chat formats.
	total += 3
	return total
}

// EstimateCompletionTokens estimates output tokens from a response.
// If no response choices are available, it falls back to the provided text.
func EstimateCompletionTokens(model string, resp *types.ChatResponse, fallbackText string) int {
	if resp != nil && len(resp.Choices) > 0 {
		total := 0
		for i := range resp.Choices {
			total += estimateMessageContentTokens(model, resp.Choices[i].Message)
		}
		if total > 0 {
			return total
		}
	}

	return CountTextTokens(model, fallbackText)
}

func estimateMessageTokens(model string, msg types.ChatMessage) int {
	total := 0
	total += CountTextTokens(model, msg.Role)
	total += CountTextTokens(model, msg.Name)
	total += CountTextTokens(model, extractMessageText(msg.Content))
	total += toolCallsTokens(model, msg.ToolCalls)
	total += CountTextTokens(model, msg.ToolCallID)

	// Small overhead per message for role/formatting tokens.
	total += 2
	return total
}

func estimateMessageContentTokens(model string, msg types.ChatMessage) int {
	total := 0
	total += CountTextTokens(model, extractMessageText(msg.Content))
	total += toolCallsTokens(model, msg.ToolCalls)
	return total
}

func toolCallsTokens(model string, calls []types.ToolCall) int {
	if len(calls) == 0 {
		return 0
	}
	total := 0
	for _, call := range calls {
		total += CountTextTokens(model, call.ID)
		total += CountTextTokens(model, call.Function.Name)
		total += CountTextTokens(model, call.Function.Arguments)
	}
	return total
}

func extractMessageText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var content string
	if err := json.Unmarshal(raw, &content); err == nil {
		return content
	}

	var parts []struct {
		Type      string `json:"type"`
		Text      string `json:"text"`
		InputText string `json:"input_text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, part := range parts {
			switch part.Type {
			case "text":
				b.WriteString(part.Text)
			case "input_text":
				b.WriteString(part.InputText)
			}
		}
		return b.String()
	}

	return string(raw)
}

func getEncoding(model string) *tiktoken.Tiktoken {
	base := normalizeModelName(model)
	if cached, ok := encodingCache.Load(base); ok {
		if enc, ok := cached.(*tiktoken.Tiktoken); ok {
			return enc
		}
		return getDefaultEncoding()
	}

	enc, err := tiktoken.EncodingForModel(base)
	if err != nil {
		enc = getDefaultEncoding()
	}
	if enc != nil {
		encodingCache.Store(base, enc)
	}
	return enc
}

func getDefaultEncoding() *tiktoken.Tiktoken {
	defaultOnce.Do(func() {
		enc, err := tiktoken.GetEncoding("cl100k_base")
		if err == nil {
			defaultEnc = enc
		}
	})
	return defaultEnc
}

func normalizeModelName(model string) string {
	if model == "" {
		return model
	}
	if idx := strings.LastIndex(model, "/"); idx >= 0 && idx+1 < len(model) {
		return model[idx+1:]
	}
	return model
}
