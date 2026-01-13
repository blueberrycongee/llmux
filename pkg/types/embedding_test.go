package types_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/pkg/types"
)

// =============================================================================
// EmbeddingInput UnmarshalJSON Tests
// =============================================================================

func TestEmbeddingInput_UnmarshalJSON_String(t *testing.T) {
	jsonStr := `"Hello, world!"`
	var input types.EmbeddingInput
	err := json.Unmarshal([]byte(jsonStr), &input)

	require.NoError(t, err)
	assert.NotNil(t, input.Text)
	assert.Equal(t, "Hello, world!", *input.Text)
	assert.Nil(t, input.Texts)
	assert.Nil(t, input.Tokens)
	assert.Nil(t, input.TokensList)
}

func TestEmbeddingInput_UnmarshalJSON_StringArray(t *testing.T) {
	jsonStr := `["Hello", "World"]`
	var input types.EmbeddingInput
	err := json.Unmarshal([]byte(jsonStr), &input)

	require.NoError(t, err)
	assert.Nil(t, input.Text)
	assert.Equal(t, []string{"Hello", "World"}, input.Texts)
	assert.Nil(t, input.Tokens)
	assert.Nil(t, input.TokensList)
}

func TestEmbeddingInput_UnmarshalJSON_IntArray(t *testing.T) {
	jsonStr := `[1234, 5678, 9012]`
	var input types.EmbeddingInput
	err := json.Unmarshal([]byte(jsonStr), &input)

	require.NoError(t, err)
	assert.Nil(t, input.Text)
	assert.Nil(t, input.Texts)
	assert.Equal(t, []int{1234, 5678, 9012}, input.Tokens)
	assert.Nil(t, input.TokensList)
}

func TestEmbeddingInput_UnmarshalJSON_IntArrayList(t *testing.T) {
	jsonStr := `[[1, 2, 3], [4, 5, 6]]`
	var input types.EmbeddingInput
	err := json.Unmarshal([]byte(jsonStr), &input)

	require.NoError(t, err)
	assert.Nil(t, input.Text)
	assert.Nil(t, input.Texts)
	assert.Nil(t, input.Tokens)
	assert.Equal(t, [][]int{{1, 2, 3}, {4, 5, 6}}, input.TokensList)
}

func TestEmbeddingInput_UnmarshalJSON_Invalid(t *testing.T) {
	testCases := []struct {
		name    string
		jsonStr string
	}{
		{"object", `{"key": "value"}`},
		{"boolean", `true`},
		{"null", `null`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var input types.EmbeddingInput
			err := json.Unmarshal([]byte(tc.jsonStr), &input)
			assert.Error(t, err)
		})
	}
}

// =============================================================================
// EmbeddingInput MarshalJSON Tests
// =============================================================================

func TestEmbeddingInput_MarshalJSON_String(t *testing.T) {
	text := "Hello, world!"
	input := types.EmbeddingInput{Text: &text}

	data, err := json.Marshal(&input)
	require.NoError(t, err)
	assert.Equal(t, `"Hello, world!"`, string(data))
}

func TestEmbeddingInput_MarshalJSON_StringArray(t *testing.T) {
	input := types.EmbeddingInput{Texts: []string{"Hello", "World"}}

	data, err := json.Marshal(&input)
	require.NoError(t, err)
	assert.Equal(t, `["Hello","World"]`, string(data))
}

func TestEmbeddingInput_MarshalJSON_IntArray(t *testing.T) {
	input := types.EmbeddingInput{Tokens: []int{1, 2, 3}}

	data, err := json.Marshal(&input)
	require.NoError(t, err)
	assert.Equal(t, `[1,2,3]`, string(data))
}

func TestEmbeddingInput_MarshalJSON_IntArrayList(t *testing.T) {
	input := types.EmbeddingInput{TokensList: [][]int{{1, 2}, {3, 4}}}

	data, err := json.Marshal(&input)
	require.NoError(t, err)
	assert.Equal(t, `[[1,2],[3,4]]`, string(data))
}

func TestEmbeddingInput_MarshalJSON_Empty(t *testing.T) {
	input := types.EmbeddingInput{}

	_, err := json.Marshal(&input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestEmbeddingInput_MarshalJSON_MultipleFields(t *testing.T) {
	text := "hello"
	input := types.EmbeddingInput{
		Text:  &text,
		Texts: []string{"world"},
	}

	_, err := json.Marshal(&input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one")
}

// =============================================================================
// EmbeddingInput Validate Tests
// =============================================================================

func TestEmbeddingInput_Validate_ValidString(t *testing.T) {
	text := "Hello"
	input := types.EmbeddingInput{Text: &text}

	err := input.Validate()
	assert.NoError(t, err)
}

func TestEmbeddingInput_Validate_EmptyString(t *testing.T) {
	text := ""
	input := types.EmbeddingInput{Text: &text}

	err := input.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestEmbeddingInput_Validate_ValidStringArray(t *testing.T) {
	input := types.EmbeddingInput{Texts: []string{"Hello", "World"}}

	err := input.Validate()
	assert.NoError(t, err)
}

func TestEmbeddingInput_Validate_EmptyStringArray(t *testing.T) {
	input := types.EmbeddingInput{Texts: []string{}}

	err := input.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestEmbeddingInput_Validate_StringArrayWithEmpty(t *testing.T) {
	input := types.EmbeddingInput{Texts: []string{"Hello", "", "World"}}

	err := input.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty string at index 1")
}

func TestEmbeddingInput_Validate_ValidTokens(t *testing.T) {
	input := types.EmbeddingInput{Tokens: []int{1, 2, 3}}

	err := input.Validate()
	assert.NoError(t, err)
}

func TestEmbeddingInput_Validate_EmptyTokens(t *testing.T) {
	input := types.EmbeddingInput{Tokens: []int{}}

	err := input.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestEmbeddingInput_Validate_ValidTokensList(t *testing.T) {
	input := types.EmbeddingInput{TokensList: [][]int{{1, 2}, {3, 4}}}

	err := input.Validate()
	assert.NoError(t, err)
}

func TestEmbeddingInput_Validate_EmptyTokensList(t *testing.T) {
	input := types.EmbeddingInput{TokensList: [][]int{}}

	err := input.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestEmbeddingInput_Validate_TokensListWithEmpty(t *testing.T) {
	input := types.EmbeddingInput{TokensList: [][]int{{1, 2}, {}}}

	err := input.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty array at index 1")
}

func TestEmbeddingInput_Validate_Nil(t *testing.T) {
	input := types.EmbeddingInput{}

	err := input.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

// =============================================================================
// EmbeddingRequest Tests
// =============================================================================

func TestEmbeddingRequest_Marshal(t *testing.T) {
	req := types.EmbeddingRequest{
		Model:          "text-embedding-3-small",
		Input:          types.NewEmbeddingInputFromStrings([]string{"The food was delicious and the waiter..."}),
		EncodingFormat: "float",
		User:           "test-user",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var m map[string]interface{}
	err = json.Unmarshal(data, &m)
	require.NoError(t, err)

	assert.Equal(t, "text-embedding-3-small", m["model"])
	assert.Equal(t, "float", m["encoding_format"])
	assert.Equal(t, "test-user", m["user"])

	// Input should be a JSON array
	input := m["input"].([]interface{})
	assert.Equal(t, "The food was delicious and the waiter...", input[0])
}

func TestEmbeddingRequest_Unmarshal_StringInput(t *testing.T) {
	jsonStr := `{
		"model": "text-embedding-ada-002",
		"input": "Hello, world!"
	}`

	var req types.EmbeddingRequest
	err := json.Unmarshal([]byte(jsonStr), &req)

	require.NoError(t, err)
	assert.Equal(t, "text-embedding-ada-002", req.Model)
	require.NotNil(t, req.Input)
	require.NotNil(t, req.Input.Text)
	assert.Equal(t, "Hello, world!", *req.Input.Text)
}

func TestEmbeddingRequest_Unmarshal_StringArrayInput(t *testing.T) {
	// This is the critical test case that previously failed with []interface{}
	jsonStr := `{
		"model": "text-embedding-ada-002",
		"input": ["hello", "world"]
	}`

	var req types.EmbeddingRequest
	err := json.Unmarshal([]byte(jsonStr), &req)

	require.NoError(t, err)
	assert.Equal(t, "text-embedding-ada-002", req.Model)
	require.NotNil(t, req.Input)
	assert.Equal(t, []string{"hello", "world"}, req.Input.Texts)
}

func TestEmbeddingRequest_Unmarshal_TokenInput(t *testing.T) {
	jsonStr := `{
		"model": "text-embedding-ada-002",
		"input": [1234, 5678]
	}`

	var req types.EmbeddingRequest
	err := json.Unmarshal([]byte(jsonStr), &req)

	require.NoError(t, err)
	require.NotNil(t, req.Input)
	assert.Equal(t, []int{1234, 5678}, req.Input.Tokens)
}

func TestEmbeddingRequest_Validate_ValidString(t *testing.T) {
	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromString("Hello, world!"),
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestEmbeddingRequest_Validate_ValidStringArray(t *testing.T) {
	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromStrings([]string{"Hello", "World"}),
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestEmbeddingRequest_Validate_ValidIntArray(t *testing.T) {
	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromTokens([]int{1234, 5678, 9012}),
	}

	err := req.Validate()
	assert.NoError(t, err)
}

func TestEmbeddingRequest_Validate_NilInput(t *testing.T) {
	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: nil,
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestEmbeddingRequest_Validate_EmptyString(t *testing.T) {
	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromString(""),
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestEmbeddingRequest_Validate_EmptyArray(t *testing.T) {
	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromStrings([]string{}),
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestEmbeddingRequest_Validate_ArrayWithEmptyStrings(t *testing.T) {
	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromStrings([]string{"Hello", "", "World"}),
	}

	err := req.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty string")
}

// =============================================================================
// EmbeddingResponse Tests
// =============================================================================

func TestEmbeddingResponse_Unmarshal(t *testing.T) {
	jsonStr := `{
	  "object": "list",
	  "data": [
	    {
	      "object": "embedding",
	      "index": 0,
	      "embedding": [
	        -0.006929283495992422,
	        -0.005336422007530928
	      ]
	    }
	  ],
	  "model": "text-embedding-3-small",
	  "usage": {
	    "prompt_tokens": 5,
	    "total_tokens": 5
	  }
	}`

	var resp types.EmbeddingResponse
	err := json.Unmarshal([]byte(jsonStr), &resp)
	require.NoError(t, err)

	assert.Equal(t, "list", resp.Object)
	assert.Equal(t, "text-embedding-3-small", resp.Model)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "embedding", resp.Data[0].Object)
	assert.Equal(t, 0, resp.Data[0].Index)
	assert.Equal(t, 2, len(resp.Data[0].Embedding))
	assert.Equal(t, -0.006929283495992422, resp.Data[0].Embedding[0])
	assert.Equal(t, 5, resp.Usage.PromptTokens)
	assert.Equal(t, 5, resp.Usage.TotalTokens)
}

// =============================================================================
// Helper Functions Tests
// =============================================================================

func TestNewEmbeddingInputFromString(t *testing.T) {
	input := types.NewEmbeddingInputFromString("test")
	require.NotNil(t, input.Text)
	assert.Equal(t, "test", *input.Text)
}

func TestNewEmbeddingInputFromStrings(t *testing.T) {
	input := types.NewEmbeddingInputFromStrings([]string{"a", "b"})
	assert.Equal(t, []string{"a", "b"}, input.Texts)
}

func TestNewEmbeddingInputFromTokens(t *testing.T) {
	input := types.NewEmbeddingInputFromTokens([]int{1, 2, 3})
	assert.Equal(t, []int{1, 2, 3}, input.Tokens)
}

func TestEmbeddingInput_IsEmpty(t *testing.T) {
	assert.True(t, (&types.EmbeddingInput{}).IsEmpty())

	text := "hello"
	assert.False(t, (&types.EmbeddingInput{Text: &text}).IsEmpty())
	assert.False(t, (&types.EmbeddingInput{Texts: []string{"a"}}).IsEmpty())
	assert.False(t, (&types.EmbeddingInput{Tokens: []int{1}}).IsEmpty())
	assert.False(t, (&types.EmbeddingInput{TokensList: [][]int{{1}}}).IsEmpty())
}

// =============================================================================
// Round-trip Tests
// =============================================================================

func TestEmbeddingRequest_RoundTrip_String(t *testing.T) {
	original := types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromString("Hello, world!"),
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var parsed types.EmbeddingRequest
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, original.Model, parsed.Model)
	require.NotNil(t, parsed.Input.Text)
	assert.Equal(t, *original.Input.Text, *parsed.Input.Text)
}

func TestEmbeddingRequest_RoundTrip_StringArray(t *testing.T) {
	original := types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromStrings([]string{"Hello", "World"}),
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var parsed types.EmbeddingRequest
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, original.Model, parsed.Model)
	assert.Equal(t, original.Input.Texts, parsed.Input.Texts)
}
