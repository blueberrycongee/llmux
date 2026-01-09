package openai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/blueberrycongee/llmux/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider_SupportEmbedding(t *testing.T) {
	p := New()
	assert.True(t, p.SupportEmbedding())
}

func TestProvider_BuildEmbeddingRequest(t *testing.T) {
	p := New(WithAPIKey("test-key"))
	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromString("hello world"),
		User:  "user-123",
	}

	httpReq, err := p.BuildEmbeddingRequest(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, "POST", httpReq.Method)
	assert.Equal(t, "https://api.openai.com/v1/embeddings", httpReq.URL.String())
	assert.Equal(t, "Bearer test-key", httpReq.Header.Get("Authorization"))
	assert.Equal(t, "application/json", httpReq.Header.Get("Content-Type"))

	body, err := io.ReadAll(httpReq.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"model":"text-embedding-3-small"`)
	assert.Contains(t, string(body), `"input":"hello world"`)
	assert.Contains(t, string(body), `"user":"user-123"`)
}

func TestProvider_ParseEmbeddingResponse(t *testing.T) {
	p := New()
	respBody := `{
	  "object": "list",
	  "data": [
	    {
	      "object": "embedding",
	      "index": 0,
	      "embedding": [0.1, 0.2, 0.3]
	    }
	  ],
	  "model": "text-embedding-3-small",
	  "usage": {
	    "prompt_tokens": 5,
	    "total_tokens": 5
	  }
	}`

	httpResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(respBody)),
	}

	embResp, err := p.ParseEmbeddingResponse(httpResp)
	require.NoError(t, err)

	assert.Equal(t, "text-embedding-3-small", embResp.Model)
	assert.Len(t, embResp.Data, 1)
	assert.Equal(t, 0.1, embResp.Data[0].Embedding[0])
}
