package types //nolint:revive // package name is intentional

// StreamOptions specifies options for streaming responses.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}
