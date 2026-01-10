package types //nolint:revive // package name is intentional

// ChatResponse represents an OpenAI-compatible chat completion response.
// All provider responses are transformed into this unified format.
type ChatResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             *Usage   `json:"usage,omitempty"`
	SystemFingerprint string   `json:"system_fingerprint,omitempty"`
}

// Choice represents a single completion choice.
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
	Logprobs     *Logprobs   `json:"logprobs,omitempty"`
}

// Usage contains token usage statistics for the request.
type Usage struct {
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	Provider         string `json:"-"`
}

// Logprobs contains log probability information.
type Logprobs struct {
	Content []LogprobContent `json:"content,omitempty"`
}

// LogprobContent represents log probability for a single token.
type LogprobContent struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
	Bytes   []int   `json:"bytes,omitempty"`
}

// StreamChunk represents a single chunk in a streaming response.
type StreamChunk struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	Choices           []StreamChoice `json:"choices"`
	Usage             *Usage         `json:"usage,omitempty"`
	SystemFingerprint string         `json:"system_fingerprint,omitempty"`
}

// StreamChoice represents a choice in a streaming response.
type StreamChoice struct {
	Index        int         `json:"index"`
	Delta        StreamDelta `json:"delta"`
	FinishReason string      `json:"finish_reason,omitempty"`
}

// StreamDelta contains the incremental content in a stream chunk.
type StreamDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Reset clears the ChatResponse for reuse.
func (r *ChatResponse) Reset() {
	r.ID = ""
	r.Object = ""
	r.Created = 0
	r.Model = ""
	r.Choices = r.Choices[:0]
	r.Usage = nil
	r.SystemFingerprint = ""
}
