package types //nolint:revive // package name is intentional

import (
	"bytes"
	"fmt"

	"github.com/goccy/go-json"
)

// ResponseInput represents input for the OpenAI responses API.
// Supports string, []string, or []ChatMessage.
type ResponseInput struct {
	Text     *string
	Texts    []string
	Messages []ChatMessage
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (r *ResponseInput) UnmarshalJSON(data []byte) error {
	r.Text = nil
	r.Texts = nil
	r.Messages = nil

	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fmt.Errorf("input cannot be null")
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		r.Text = &s
		return nil
	}

	var msgs []ChatMessage
	if err := json.Unmarshal(data, &msgs); err == nil && len(msgs) > 0 {
		r.Messages = msgs
		return nil
	}

	var list []string
	if err := json.Unmarshal(data, &list); err == nil && len(list) > 0 {
		r.Texts = list
		return nil
	}

	return fmt.Errorf("input must be string, []string, or []message")
}

// ResponseRequest represents an OpenAI responses API request.
type ResponseRequest struct {
	Model           string                     `json:"model"`
	Input           ResponseInput              `json:"input"`
	Stream          bool                       `json:"stream,omitempty"`
	MaxOutputTokens int                        `json:"max_output_tokens,omitempty"`
	MaxTokens       int                        `json:"max_tokens,omitempty"`
	Temperature     *float64                   `json:"temperature,omitempty"`
	TopP            *float64                   `json:"top_p,omitempty"`
	User            string                     `json:"user,omitempty"`
	Tools           []Tool                     `json:"tools,omitempty"`
	ToolChoice      json.RawMessage            `json:"tool_choice,omitempty"`
	ResponseFormat  *ResponseFormat            `json:"response_format,omitempty"`
	StreamOptions   *StreamOptions             `json:"stream_options,omitempty"`
	Tags            []string                   `json:"tags,omitempty"`
	Extra           map[string]json.RawMessage `json:"-"`
}

var responseRequestKnownFields = map[string]struct{}{
	"model":             {},
	"input":             {},
	"stream":            {},
	"max_output_tokens": {},
	"max_tokens":        {},
	"temperature":       {},
	"top_p":             {},
	"user":              {},
	"tools":             {},
	"tool_choice":       {},
	"response_format":   {},
	"stream_options":    {},
	"tags":              {},
}

// UnmarshalJSON captures unknown fields into Extra for passthrough.
func (r *ResponseRequest) UnmarshalJSON(data []byte) error {
	type Alias ResponseRequest

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	var parsed Alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	*r = ResponseRequest(parsed)
	for key := range responseRequestKnownFields {
		delete(payload, key)
	}

	if len(payload) == 0 {
		r.Extra = nil
	} else {
		r.Extra = payload
	}

	return nil
}

// ToChatRequest converts a responses request into a chat request.
func (r *ResponseRequest) ToChatRequest() (*ChatRequest, error) {
	if r.Model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if err := ValidateModelName(r.Model); err != nil {
		return nil, err
	}

	messages, err := responseInputToMessages(r.Input)
	if err != nil {
		return nil, err
	}

	maxTokens := r.MaxTokens
	if r.MaxOutputTokens > 0 {
		maxTokens = r.MaxOutputTokens
	}

	return &ChatRequest{
		Model:          r.Model,
		Messages:       messages,
		Stream:         r.Stream,
		MaxTokens:      maxTokens,
		Temperature:    r.Temperature,
		TopP:           r.TopP,
		User:           r.User,
		Tools:          r.Tools,
		ToolChoice:     r.ToolChoice,
		ResponseFormat: r.ResponseFormat,
		StreamOptions:  r.StreamOptions,
		Tags:           r.Tags,
		Extra:          r.Extra,
	}, nil
}

// ResponseContent represents response output content.
type ResponseContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ResponseOutput represents a response output item.
type ResponseOutput struct {
	Type    string            `json:"type"`
	Role    string            `json:"role,omitempty"`
	Content []ResponseContent `json:"content,omitempty"`
}

// ResponseResponse represents a responses API response.
type ResponseResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Output  []ResponseOutput `json:"output"`
	Usage   *Usage           `json:"usage,omitempty"`
}

// ResponseStreamChunk represents a streaming responses API event.
type ResponseStreamChunk struct {
	Type     string            `json:"type"`
	Delta    string            `json:"delta,omitempty"`
	Response *ResponseResponse `json:"response,omitempty"`
}

// ResponseResponseFromChat converts a chat completion response to responses format.
func ResponseResponseFromChat(resp *ChatResponse) *ResponseResponse {
	if resp == nil {
		return nil
	}

	output := make([]ResponseOutput, 0, len(resp.Choices))
	for i := range resp.Choices {
		choice := &resp.Choices[i]
		text := extractMessageText(choice.Message)
		output = append(output, ResponseOutput{
			Type: "message",
			Role: choice.Message.Role,
			Content: []ResponseContent{
				{Type: "output_text", Text: text},
			},
		})
	}

	return &ResponseResponse{
		ID:      resp.ID,
		Object:  "response",
		Created: resp.Created,
		Model:   resp.Model,
		Output:  output,
		Usage:   resp.Usage,
	}
}

func responseInputToMessages(input ResponseInput) ([]ChatMessage, error) {
	if input.Text != nil {
		content, err := json.Marshal(*input.Text)
		if err != nil {
			return nil, fmt.Errorf("marshal input text: %w", err)
		}
		return []ChatMessage{{Role: "user", Content: content}}, nil
	}

	if len(input.Texts) > 0 {
		messages := make([]ChatMessage, 0, len(input.Texts))
		for _, text := range input.Texts {
			content, err := json.Marshal(text)
			if err != nil {
				return nil, fmt.Errorf("marshal input text: %w", err)
			}
			messages = append(messages, ChatMessage{Role: "user", Content: content})
		}
		return messages, nil
	}

	if len(input.Messages) > 0 {
		return input.Messages, nil
	}

	return nil, fmt.Errorf("input is required")
}
