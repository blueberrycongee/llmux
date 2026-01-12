package governance

import (
	"net/http"
	"time"
)

// Call type labels for usage logging.
const (
	CallTypeChatCompletion = "chat_completion"
	CallTypeCompletion     = "completion"
	CallTypeEmbedding      = "embedding"
)

// Config controls governance behavior.
type Config struct {
	Enabled           bool
	AsyncAccounting   bool
	IdempotencyWindow time.Duration
	AuditEnabled      bool
}

// RequestInput captures request context for governance evaluation.
type RequestInput struct {
	Request   *http.Request
	Model     string
	CallType  string
	EndUserID string
	Tags      []string
}

// Usage captures token and cost information for accounting.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Cost             float64
	Provider         string
}

// AccountInput captures the details needed for accounting.
type AccountInput struct {
	RequestID   string
	Model       string
	CallType    string
	EndUserID   string
	RequestTags []string
	Usage       Usage
	Start       time.Time
	Latency     time.Duration
	StatusCode  *int
	Status      *string
}
