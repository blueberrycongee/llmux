package azure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestBuildRequest_EscapesDeploymentNameAndUsesQueryParams(t *testing.T) {
	pAny, err := NewFromConfig(provider.Config{
		APIKey:  "k",
		BaseURL: "https://example.com/prefix",
		Headers: map[string]string{
			"api-version": "2024-02-15-preview",
			"X-Foo":       "bar",
		},
	})
	require.NoError(t, err)
	p := pAny.(*Provider)

	req, err := p.BuildRequest(context.Background(), &types.ChatRequest{
		Model: "dep/with/slash?x=y",
	})
	require.NoError(t, err)

	require.Equal(t, "/prefix/openai/deployments/dep%2Fwith%2Fslash%3Fx=y/chat/completions", req.URL.Path)
	require.Equal(t, "2024-02-15-preview", req.URL.Query().Get("api-version"))
	require.Len(t, req.URL.Query(), 1)
	require.Equal(t, "bar", req.Header.Get("X-Foo"))
}
