package types

import (
	"encoding/json"
	"fmt"
)

// EmbeddingInput represents the input for an embedding request.
// It supports multiple input formats as per OpenAI's API specification:
// - A single string
// - An array of strings
// - An array of token IDs (integers)
// - An array of token ID arrays (for batch processing)
//
// This design is aligned with Bifrost's type-safe approach, using custom
// JSON marshaling/unmarshaling to automatically infer the input type.
type EmbeddingInput struct {
	// Text is a single string input.
	Text *string `json:"-"`
	// Texts is an array of string inputs.
	Texts []string `json:"-"`
	// Tokens is an array of token IDs.
	Tokens []int `json:"-"`
	// TokensList is an array of token ID arrays (batch).
	TokensList [][]int `json:"-"`
}

// UnmarshalJSON implements custom JSON unmarshaling with automatic type inference.
// It tries to parse the input in order: string -> []string -> []int -> [][]int.
func (e *EmbeddingInput) UnmarshalJSON(data []byte) error {
	// Reset all fields
	e.Text = nil
	e.Texts = nil
	e.Tokens = nil
	e.TokensList = nil

	// Reject null
	if string(data) == "null" {
		return fmt.Errorf("input cannot be null")
	}

	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		e.Text = &s
		return nil
	}

	// Try []string
	var ss []string
	if err := json.Unmarshal(data, &ss); err == nil {
		// Verify it's actually strings, not integers
		e.Texts = ss
		return nil
	}

	// Try []int (token IDs)
	var tokens []int
	if err := json.Unmarshal(data, &tokens); err == nil {
		e.Tokens = tokens
		return nil
	}

	// Try [][]int (batch token IDs)
	var tokensList [][]int
	if err := json.Unmarshal(data, &tokensList); err == nil {
		e.TokensList = tokensList
		return nil
	}

	return fmt.Errorf("input must be string, []string, []int, or [][]int")
}

// MarshalJSON implements custom JSON marshaling.
// It enforces that exactly one field is set.
func (e *EmbeddingInput) MarshalJSON() ([]byte, error) {
	set := 0
	if e.Text != nil {
		set++
	}
	if e.Texts != nil {
		set++
	}
	if e.Tokens != nil {
		set++
	}
	if e.TokensList != nil {
		set++
	}

	if set == 0 {
		return nil, fmt.Errorf("embedding input is empty")
	}
	if set > 1 {
		return nil, fmt.Errorf("embedding input must set exactly one field")
	}

	if e.Text != nil {
		return json.Marshal(*e.Text)
	}
	if e.Texts != nil {
		return json.Marshal(e.Texts)
	}
	if e.Tokens != nil {
		return json.Marshal(e.Tokens)
	}
	return json.Marshal(e.TokensList)
}

// Validate checks if the embedding input is valid (non-empty).
func (e *EmbeddingInput) Validate() error {
	if e.Text != nil {
		if *e.Text == "" {
			return fmt.Errorf("input string cannot be empty")
		}
		return nil
	}
	if e.Texts != nil {
		if len(e.Texts) == 0 {
			return fmt.Errorf("input array cannot be empty")
		}
		for i, s := range e.Texts {
			if s == "" {
				return fmt.Errorf("input array contains empty string at index %d", i)
			}
		}
		return nil
	}
	if e.Tokens != nil {
		if len(e.Tokens) == 0 {
			return fmt.Errorf("token array cannot be empty")
		}
		return nil
	}
	if e.TokensList != nil {
		if len(e.TokensList) == 0 {
			return fmt.Errorf("token list cannot be empty")
		}
		for i, tokens := range e.TokensList {
			if len(tokens) == 0 {
				return fmt.Errorf("token list contains empty array at index %d", i)
			}
		}
		return nil
	}
	return fmt.Errorf("input cannot be nil")
}

// IsEmpty returns true if no input is set.
func (e *EmbeddingInput) IsEmpty() bool {
	return e.Text == nil && e.Texts == nil && e.Tokens == nil && e.TokensList == nil
}

// NewEmbeddingInputFromString creates an EmbeddingInput from a single string.
func NewEmbeddingInputFromString(s string) *EmbeddingInput {
	return &EmbeddingInput{Text: &s}
}

// NewEmbeddingInputFromStrings creates an EmbeddingInput from a string slice.
func NewEmbeddingInputFromStrings(ss []string) *EmbeddingInput {
	return &EmbeddingInput{Texts: ss}
}

// NewEmbeddingInputFromTokens creates an EmbeddingInput from token IDs.
func NewEmbeddingInputFromTokens(tokens []int) *EmbeddingInput {
	return &EmbeddingInput{Tokens: tokens}
}

// EmbeddingRequest represents an OpenAI-compatible embedding request.
type EmbeddingRequest struct {
	// Model is the ID of the model to use.
	Model string `json:"model"`

	// Input is the text to embed. Uses a type-safe structure that supports
	// string, []string, []int, or [][]int formats.
	Input *EmbeddingInput `json:"input"`

	// EncodingFormat is the format to return the embeddings in.
	// Can be "float" or "base64". Defaults to "float".
	EncodingFormat string `json:"encoding_format,omitempty"`

	// User is a unique identifier representing your end-user.
	User string `json:"user,omitempty"`

	// Dimensions is the number of dimensions the resulting output embeddings should have.
	// Only supported in text-embedding-3 and later models.
	Dimensions int `json:"dimensions,omitempty"`
}

// Validate checks if the embedding request is valid.
func (r *EmbeddingRequest) Validate() error {
	if r.Input == nil {
		return fmt.Errorf("input cannot be nil")
	}
	return r.Input.Validate()
}

// EmbeddingResponse represents an OpenAI-compatible embedding response.
type EmbeddingResponse struct {
	Object string            `json:"object"`
	Data   []EmbeddingObject `json:"data"`
	Model  string            `json:"model"`
	Usage  Usage             `json:"usage"`
}

// EmbeddingObject represents a single embedding object.
type EmbeddingObject struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}
