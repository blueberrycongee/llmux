package types //nolint:revive // package name is intentional

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
)

func TestResponseRequestToChatRequest_TextInput(t *testing.T) {
	data := []byte(`{
		"model": "gpt-4o",
		"input": "hello",
		"max_output_tokens": 12,
		"temperature": 0.2,
		"top_p": 0.9,
		"user": "user-1"
	}`)

	var req ResponseRequest
	require.NoError(t, json.Unmarshal(data, &req))

	chatReq, err := req.ToChatRequest()
	require.NoError(t, err)
	require.Equal(t, "gpt-4o", chatReq.Model)
	require.Equal(t, 12, chatReq.MaxTokens)
	require.NotNil(t, chatReq.Temperature)
	require.Equal(t, 0.2, *chatReq.Temperature)
	require.NotNil(t, chatReq.TopP)
	require.Equal(t, 0.9, *chatReq.TopP)
	require.Equal(t, "user-1", chatReq.User)
	require.Len(t, chatReq.Messages, 1)
	require.Equal(t, "user", chatReq.Messages[0].Role)

	var content string
	require.NoError(t, json.Unmarshal(chatReq.Messages[0].Content, &content))
	require.Equal(t, "hello", content)
}

func TestResponseRequestToChatRequest_MessageInput(t *testing.T) {
	data := []byte(`{
		"model": "gpt-4o",
		"input": [{"role": "user", "content": "ping"}]
	}`)

	var req ResponseRequest
	require.NoError(t, json.Unmarshal(data, &req))

	chatReq, err := req.ToChatRequest()
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 1)
	require.Equal(t, "user", chatReq.Messages[0].Role)

	var content string
	require.NoError(t, json.Unmarshal(chatReq.Messages[0].Content, &content))
	require.Equal(t, "ping", content)
}

func TestResponseRequestToChatRequest_TextListInput(t *testing.T) {
	data := []byte(`{
		"model": "gpt-4o",
		"input": ["one", "two"]
	}`)

	var req ResponseRequest
	require.NoError(t, json.Unmarshal(data, &req))

	chatReq, err := req.ToChatRequest()
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 2)

	var content0 string
	require.NoError(t, json.Unmarshal(chatReq.Messages[0].Content, &content0))
	require.Equal(t, "one", content0)
	var content1 string
	require.NoError(t, json.Unmarshal(chatReq.Messages[1].Content, &content1))
	require.Equal(t, "two", content1)
}

func TestResponseRequestUnmarshalRejectsNullInput(t *testing.T) {
	data := []byte(`{
		"model": "gpt-4o",
		"input": null
	}`)

	var req ResponseRequest
	require.Error(t, json.Unmarshal(data, &req))
}
