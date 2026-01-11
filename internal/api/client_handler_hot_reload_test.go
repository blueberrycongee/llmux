package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/pkg/types"
	"github.com/stretchr/testify/require"
)

type fakeProvider struct {
	name   string
	models []string
}

func (f *fakeProvider) Name() string {
	return f.name
}

func (f *fakeProvider) SupportedModels() []string {
	return append([]string{}, f.models...)
}

func (f *fakeProvider) SupportsModel(model string) bool {
	for _, m := range f.models {
		if m == model {
			return true
		}
	}
	return false
}

func (f *fakeProvider) BuildRequest(context.Context, *types.ChatRequest) (*http.Request, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeProvider) ParseResponse(*http.Response) (*types.ChatResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeProvider) ParseStreamChunk([]byte) (*types.StreamChunk, error) {
	return nil, nil
}

func (f *fakeProvider) MapError(int, []byte) error {
	return fmt.Errorf("not implemented")
}

func (f *fakeProvider) SupportEmbedding() bool {
	return false
}

func (f *fakeProvider) BuildEmbeddingRequest(context.Context, *types.EmbeddingRequest) (*http.Request, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeProvider) ParseEmbeddingResponse(*http.Response) (*types.EmbeddingResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestClientHandlerUsesLatestClient(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{}))

	clientA, err := llmux.New(llmux.WithProviderInstance(
		"prov-a",
		&fakeProvider{name: "prov-a", models: []string{"model-a"}},
		[]string{"model-a"},
	))
	require.NoError(t, err)

	clientB, err := llmux.New(llmux.WithProviderInstance(
		"prov-b",
		&fakeProvider{name: "prov-b", models: []string{"model-b"}},
		[]string{"model-b"},
	))
	require.NoError(t, err)

	swapper := NewClientSwapper(clientA)
	t.Cleanup(swapper.Close)
	handler := NewClientHandlerWithSwapper(swapper, logger, nil)

	assertModels := func(expected string) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		rec := httptest.NewRecorder()

		handler.ListModels(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var payload struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
		require.Len(t, payload.Data, 1)
		require.Equal(t, expected, payload.Data[0].ID)
	}

	assertModels("model-a")

	swapper.Swap(clientB)

	assertModels("model-b")
}
