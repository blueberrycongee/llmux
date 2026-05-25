package api

import (
	"strings"

	"github.com/goccy/go-json"
)

func decodeChatMessageContent(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}

	var parts []map[string]any
	if err := json.Unmarshal(raw, &parts); err == nil {
		var builder strings.Builder
		for _, part := range parts {
			partType, _ := part["type"].(string)
			if partType != "text" {
				continue
			}
			if value, ok := part["text"].(string); ok {
				if builder.Len() > 0 {
					builder.WriteString("\n")
				}
				builder.WriteString(value)
			}
		}
		if builder.Len() > 0 {
			return strings.TrimSpace(builder.String())
		}
	}

	return trimmed
}
