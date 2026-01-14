package gemini

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestBuildRequest_EncodesAPIKeyAndEscapesModel(t *testing.T) {
	pAny, err := NewFromConfig(provider.Config{
		APIKey:  "abc&evil=1",
		BaseURL: "https://example.com",
		Headers: map[string]string{
			"X-Foo": "bar",
		},
	})
	require.NoError(t, err)
	p := pAny.(*Provider)

	req, err := p.BuildRequest(context.Background(), &types.ChatRequest{
		Model: "gemini-1.5/flash",
	})
	require.NoError(t, err)

	require.Equal(t, "/v1beta/models/gemini-1.5%2Fflash:generateContent", req.URL.Path)
	require.Equal(t, "abc&evil=1", req.URL.Query().Get("key"))
	require.Len(t, req.URL.Query(), 1)
	require.Equal(t, "bar", req.Header.Get("X-Foo"))
}
