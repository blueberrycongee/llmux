package types //nolint:revive // package name is intentional

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/goccy/go-json"
)

// CompletionPrompt represents the prompt for a completion request.
// It supports either a single string or an array of strings.
type CompletionPrompt struct {
	Text  *string
	Texts []string
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (p *CompletionPrompt) UnmarshalJSON(data []byte) error {
	p.Text = nil
	p.Texts = nil

	if bytes.Equal(data, []byte("null")) {
		return fmt.Errorf("prompt cannot be null")
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		p.Text = &s
		return nil
	}

	var ss []string
	if err := json.Unmarshal(data, &ss); err == nil {
		p.Texts = ss
		return nil
	}

	return fmt.Errorf("prompt must be string or []string")
}

// MarshalJSON implements custom JSON marshaling.
func (p *CompletionPrompt) MarshalJSON() ([]byte, error) {
	set := 0
	if p.Text != nil {
		set++
	}
	if p.Texts != nil {
		set++
	}
	if set == 0 {
		return nil, fmt.Errorf("prompt is empty")
	}
	if set > 1 {
		return nil, fmt.Errorf("prompt must set exactly one field")
	}
	if p.Text != nil {
		return json.Marshal(*p.Text)
	}
	return json.Marshal(p.Texts)
}

// Validate checks whether the prompt is non-empty.
func (p *CompletionPrompt) Validate() error {
	if p.Text != nil {
		if *p.Text == "" {
			return fmt.Errorf("prompt cannot be empty")
		}
		return nil
	}
	if p.Texts != nil {
		if len(p.Texts) == 0 {
			return fmt.Errorf("prompt list cannot be empty")
		}
		for i, s := range p.Texts {
			if s == "" {
				return fmt.Errorf("prompt list contains empty string at index %d", i)
			}
		}
		return nil
	}
	return fmt.Errorf("prompt is required")
}

// AsText returns a single prompt string for chat conversion.
func (p *CompletionPrompt) AsText() (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	if p.Text != nil {
		return *p.Text, nil
	}
	return strings.Join(p.Texts, "\n"), nil
}

// CompletionStop supports "stop" as a string or array of strings.
type CompletionStop struct {
	Values []string
}

// UnmarshalJSON implements custom JSON unmarshaling.
func (s *CompletionStop) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		return fmt.Errorf("stop cannot be null")
	}

	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		s.Values = []string{single}
		return nil
	}

	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		s.Values = list
		return nil
	}

	return fmt.Errorf("stop must be string or []string")
}

// CompletionRequest represents an OpenAI-compatible completion request.
type CompletionRequest struct {
	Model            string           `json:"model"`
	Prompt           CompletionPrompt `json:"prompt"`
	Stream           bool             `json:"stream,omitempty"`
	MaxTokens        int              `json:"max_tokens,omitempty"`
	Temperature      *float64         `json:"temperature,omitempty"`
	TopP             *float64         `json:"top_p,omitempty"`
	N                int              `json:"n,omitempty"`
	Stop             *CompletionStop  `json:"stop,omitempty"`
	PresencePenalty  *float64         `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64         `json:"frequency_penalty,omitempty"`
	User             string           `json:"user,omitempty"`

	// Optional fields accepted for compatibility (currently ignored).
	BestOf   int    `json:"best_of,omitempty"`
	Echo     bool   `json:"echo,omitempty"`
	Logprobs int    `json:"logprobs,omitempty"`
	Suffix   string `json:"suffix,omitempty"`
}

// Validate checks the completion request.
func (r *CompletionRequest) Validate() error {
	if r.Model == "" {
		return fmt.Errorf("model is required")
	}
	return r.Prompt.Validate()
}

// ToChatRequest converts a completion request into a chat request.
func (r *CompletionRequest) ToChatRequest() (*ChatRequest, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}

	text, err := r.Prompt.AsText()
	if err != nil {
		return nil, err
	}

	content, err := json.Marshal(text)
	if err != nil {
		return nil, fmt.Errorf("marshal prompt: %w", err)
	}

	chatReq := &ChatRequest{
		Model:            r.Model,
		Messages:         []ChatMessage{{Role: "user", Content: content}},
		Stream:           r.Stream,
		MaxTokens:        r.MaxTokens,
		Temperature:      r.Temperature,
		TopP:             r.TopP,
		N:                r.N,
		PresencePenalty:  r.PresencePenalty,
		FrequencyPenalty: r.FrequencyPenalty,
		User:             r.User,
	}
	if r.Stop != nil {
		chatReq.Stop = r.Stop.Values
	}

	return chatReq, nil
}

// CompletionResponse represents an OpenAI-compatible completion response.
type CompletionResponse struct {
	ID                string             `json:"id"`
	Object            string             `json:"object"`
	Created           int64              `json:"created"`
	Model             string             `json:"model"`
	Choices           []CompletionChoice `json:"choices"`
	Usage             *Usage             `json:"usage,omitempty"`
	SystemFingerprint string             `json:"system_fingerprint,omitempty"`
}

// CompletionChoice represents a completion choice.
type CompletionChoice struct {
	Index        int                 `json:"index"`
	Text         string              `json:"text"`
	Logprobs     *CompletionLogprobs `json:"logprobs,omitempty"`
	FinishReason string              `json:"finish_reason,omitempty"`
}

// CompletionLogprobs represents log probability info for completions.
type CompletionLogprobs struct {
	Tokens        []string             `json:"tokens,omitempty"`
	TokenLogprobs []float64            `json:"token_logprobs,omitempty"`
	TopLogprobs   []map[string]float64 `json:"top_logprobs,omitempty"`
	TextOffset    []int                `json:"text_offset,omitempty"`
}

// CompletionStreamChunk represents a streaming completion chunk.
type CompletionStreamChunk struct {
	ID                string                   `json:"id"`
	Object            string                   `json:"object"`
	Created           int64                    `json:"created"`
	Model             string                   `json:"model"`
	Choices           []CompletionStreamChoice `json:"choices"`
	Usage             *Usage                   `json:"usage,omitempty"`
	SystemFingerprint string                   `json:"system_fingerprint,omitempty"`
}

// CompletionStreamChoice represents a streaming completion choice.
type CompletionStreamChoice struct {
	Index        int                 `json:"index"`
	Text         string              `json:"text"`
	Logprobs     *CompletionLogprobs `json:"logprobs,omitempty"`
	FinishReason string              `json:"finish_reason,omitempty"`
}

// CompletionResponseFromChat converts a chat completion response to a completion response.
func CompletionResponseFromChat(resp *ChatResponse) *CompletionResponse {
	if resp == nil {
		return nil
	}

	choices := make([]CompletionChoice, 0, len(resp.Choices))
	for i := range resp.Choices {
		choice := resp.Choices[i]
		choices = append(choices, CompletionChoice{
			Index:        choice.Index,
			Text:         extractMessageText(choice.Message),
			FinishReason: choice.FinishReason,
		})
	}

	return &CompletionResponse{
		ID:                resp.ID,
		Object:            "text_completion",
		Created:           resp.Created,
		Model:             resp.Model,
		Choices:           choices,
		Usage:             resp.Usage,
		SystemFingerprint: resp.SystemFingerprint,
	}
}

// CompletionStreamChunkFromChat converts a chat stream chunk to a completion stream chunk.
func CompletionStreamChunkFromChat(chunk *StreamChunk) *CompletionStreamChunk {
	if chunk == nil {
		return nil
	}

	choices := make([]CompletionStreamChoice, 0, len(chunk.Choices))
	for i := range chunk.Choices {
		choice := chunk.Choices[i]
		choices = append(choices, CompletionStreamChoice{
			Index:        choice.Index,
			Text:         choice.Delta.Content,
			FinishReason: choice.FinishReason,
		})
	}

	return &CompletionStreamChunk{
		ID:                chunk.ID,
		Object:            "text_completion",
		Created:           chunk.Created,
		Model:             chunk.Model,
		Choices:           choices,
		Usage:             chunk.Usage,
		SystemFingerprint: chunk.SystemFingerprint,
	}
}

type contentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func extractMessageText(msg ChatMessage) string {
	if len(msg.Content) == 0 {
		return ""
	}

	if bytes.Equal(msg.Content, []byte("null")) {
		return ""
	}

	var text string
	if err := json.Unmarshal(msg.Content, &text); err == nil {
		return text
	}

	var parts []contentPart
	if err := json.Unmarshal(msg.Content, &parts); err == nil {
		var b strings.Builder
		for _, part := range parts {
			if part.Type == "" || part.Type == "text" {
				b.WriteString(part.Text)
			}
		}
		return b.String()
	}

	return string(msg.Content)
}
