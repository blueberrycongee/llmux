//nolint:bodyclose // test code - response bodies are handled appropriately
package e2e

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/pkg/types"
	"github.com/blueberrycongee/llmux/tests/testutil"
)

type completionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		Text         string `json:"text"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *types.Usage `json:"usage,omitempty"`
}

func TestAPI_Completions_Basic(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	reqBody := map[string]any{
		"model":  "gpt-4o-mock",
		"prompt": "Hello, world!",
	}

	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, testClient.BaseURL()+"/v1/completions", bytes.NewReader(body))
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := testClient.HTTPClient().Do(httpReq)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	testutil.AssertJSONResponse(t, resp)

	var completionResp completionResponse
	err = json.NewDecoder(resp.Body).Decode(&completionResp)
	require.NoError(t, err)

	assert.NotEmpty(t, completionResp.ID)
	assert.Equal(t, "text_completion", completionResp.Object)
	assert.NotZero(t, completionResp.Created)
	assert.NotEmpty(t, completionResp.Model)
	require.NotEmpty(t, completionResp.Choices)
	assert.NotEmpty(t, completionResp.Choices[0].Text)
	assert.NotEmpty(t, completionResp.Choices[0].FinishReason)
	require.NotNil(t, completionResp.Usage)
	assert.Greater(t, completionResp.Usage.PromptTokens, 0)
	assert.Greater(t, completionResp.Usage.CompletionTokens, 0)

	testutil.AssertRequestCount(t, mockLLM, 1)
}
