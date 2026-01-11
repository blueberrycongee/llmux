package openai

import (
	"context"
	"io"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestBuildRequest_MergesExtraWithoutOverwriting(t *testing.T) {
	temp := 0.2
	req := &types.ChatRequest{
		Model:       "gpt-4",
		Messages:    []types.ChatMessage{{Role: "user", Content: json.RawMessage(`"hi"`)}},
		Temperature: &temp,
		Extra: map[string]json.RawMessage{
			"foo":         json.RawMessage(`"bar"`),
			"model":       json.RawMessage(`"override"`),
			"temperature": json.RawMessage(`0.9`),
		},
	}

	provider := New(
		WithAPIKey("test-key"),
		WithBaseURL("https://api.test.com"),
	)

	httpReq, err := provider.BuildRequest(context.Background(), req)
	require.NoError(t, err)

	body, err := io.ReadAll(httpReq.Body)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(body, &payload)
	require.NoError(t, err)

	assert.Equal(t, "gpt-4", payload["model"])
	assert.InDelta(t, 0.2, payload["temperature"].(float64), 0.0001)
	assert.Equal(t, "bar", payload["foo"])
}
