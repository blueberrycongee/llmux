package api //nolint:revive // package name is intentional

// ErrorResponse is the OpenAI-compatible error envelope.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail describes the error payload.
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}
