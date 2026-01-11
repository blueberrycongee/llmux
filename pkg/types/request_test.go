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
