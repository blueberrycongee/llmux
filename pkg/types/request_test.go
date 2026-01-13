package types //nolint:revive // package name is intentional

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatRequestUnmarshal_ExtraFieldsCaptured(t *testing.T) {
	data := []byte(`{
		"model": "gpt-4",
		"messages": [{"role": "user", "content": "hi"}],
		"temperature": 0.5,
		"stream_options": {"include_usage": true},
		"foo": "bar",
		"nested": {"enabled": true}
	}`)

	var req ChatRequest
	err := json.Unmarshal(data, &req)
	require.NoError(t, err)

	require.NotNil(t, req.Extra)
	assert.JSONEq(t, `"bar"`, string(req.Extra["foo"]))
	assert.JSONEq(t, `{"enabled": true}`, string(req.Extra["nested"]))
	assert.NotContains(t, req.Extra, "model")
	assert.NotContains(t, req.Extra, "messages")
	assert.NotContains(t, req.Extra, "temperature")
	assert.NotContains(t, req.Extra, "stream_options")
}

func TestChatRequestUnmarshal_NoExtraFields(t *testing.T) {
	data := []byte(`{
		"model": "gpt-4",
		"messages": [{"role": "user", "content": "hi"}],
		"stream": true
	}`)

	var req ChatRequest
	err := json.Unmarshal(data, &req)
	require.NoError(t, err)

	assert.Nil(t, req.Extra)
}

func TestChatRequestUnmarshal_AliasFieldsMapped(t *testing.T) {
	data := []byte(`{
		"model": "gpt-4",
		"messages": [{"role": "user", "content": "hi"}],
		"max_output_tokens": 7,
		"end_user": "user-42",
		"tag": "alpha"
	}`)

	var req ChatRequest
	err := json.Unmarshal(data, &req)
	require.NoError(t, err)

	assert.Equal(t, 7, req.MaxTokens)
	assert.Equal(t, "user-42", req.User)
	assert.Equal(t, []string{"alpha"}, req.Tags)
	if req.Extra != nil {
		assert.NotContains(t, req.Extra, "max_output_tokens")
		assert.NotContains(t, req.Extra, "end_user")
		assert.NotContains(t, req.Extra, "tag")
	}
}

func TestChatRequestUnmarshal_DoesNotOverrideMaxTokens(t *testing.T) {
	data := []byte(`{
		"model": "gpt-4",
		"messages": [{"role": "user", "content": "hi"}],
		"max_tokens": 5,
		"max_output_tokens": 10
	}`)

	var req ChatRequest
	err := json.Unmarshal(data, &req)
	require.NoError(t, err)
	assert.Equal(t, 5, req.MaxTokens)
}

func TestChatRequestReset_ClearsStreamOptions(t *testing.T) {
	req := &ChatRequest{
		Model: "gpt-4",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
		},
		StreamOptions: &StreamOptions{IncludeUsage: true},
	}

	req.Reset()

	assert.Nil(t, req.StreamOptions)
}
