package anthropic

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestTransformRequest_ThinkingPassthrough(t *testing.T) {
	p := New()
	req := &types.ChatRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hello"`)},
		},
		Extra: map[string]json.RawMessage{
			"thinking": json.RawMessage(`{"type":"enabled","budget_tokens":1024}`),
		},
	}

	anthropicReq, err := p.transformRequest(req)
	require.NoError(t, err)
	require.NotNil(t, anthropicReq.Thinking)
	require.Equal(t, "enabled", anthropicReq.Thinking.Type)
	require.Equal(t, 1024, anthropicReq.Thinking.BudgetTokens)
}

func TestTransformRequest_ThinkingInvalidIgnored(t *testing.T) {
	p := New()
	req := &types.ChatRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hello"`)},
		},
		Extra: map[string]json.RawMessage{
			"thinking": json.RawMessage(`"not-an-object"`),
		},
	}

	anthropicReq, err := p.transformRequest(req)
	require.NoError(t, err)
	require.Nil(t, anthropicReq.Thinking)
}

func TestTransformRequest_ThinkingMissingTypeIgnored(t *testing.T) {
	p := New()
	req := &types.ChatRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []types.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hello"`)},
		},
		Extra: map[string]json.RawMessage{
			"thinking": json.RawMessage(`{"budget_tokens":512}`),
		},
	}

	anthropicReq, err := p.transformRequest(req)
	require.NoError(t, err)
	require.Nil(t, anthropicReq.Thinking)
}
