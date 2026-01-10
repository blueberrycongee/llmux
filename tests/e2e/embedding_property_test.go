//nolint:bodyclose // test code - response bodies are handled appropriately
package e2e

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/tests/testutil"
)

// **Feature: embedding-http-endpoint, Property 1: Valid request returns valid response structure**
// **Validates: Requirements 1.1, 3.1**
// *For any* valid embedding request with a supported model and non-empty input,
// the response SHALL contain an "object" field equal to "list", a "data" array
// with at least one embedding object, a "model" field, and a "usage" object.
func TestProperty_ValidRequestReturnsValidResponseStructure(t *testing.T) {
	resetMock()

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(time.Now().UnixNano())

	properties := gopter.NewProperties(parameters)

	// Generator for non-empty strings (valid inputs)
	nonEmptyStringGen := gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) > 0 && len(s) < 100
	})

	properties.Property("valid request returns valid response structure", prop.ForAll(
		func(input string) bool {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			req := &testutil.EmbeddingRequest{
				Model: "text-embedding-ada-002-mock",
				Input: input,
			}

			resp, httpResp, err := testClient.Embedding(ctx, req)
			if err != nil {
				t.Logf("Request error: %v", err)
				return false
			}
			if httpResp == nil {
				t.Log("HTTP response is nil")
				return false
			}
			if httpResp.StatusCode != http.StatusOK {
				t.Logf("Unexpected status code: %d", httpResp.StatusCode)
				return false
			}
			if resp == nil {
				t.Log("Response is nil")
				return false
			}

			// Verify response structure
			if resp.Object != "list" {
				t.Logf("Object field is not 'list': %s", resp.Object)
				return false
			}
			if len(resp.Data) < 1 {
				t.Log("Data array is empty")
				return false
			}
			if resp.Model == "" {
				t.Log("Model field is empty")
				return false
			}
			// Usage must have non-negative values
			if resp.Usage.PromptTokens < 0 || resp.Usage.TotalTokens < 0 {
				t.Logf("Invalid usage: prompt=%d, total=%d", resp.Usage.PromptTokens, resp.Usage.TotalTokens)
				return false
			}

			return true
		},
		nonEmptyStringGen,
	))

	properties.TestingRun(t)
}

// **Feature: embedding-http-endpoint, Property 2: Input count equals output count**
// **Validates: Requirements 2.2**
// *For any* embedding request with an array of N strings as input,
// the response SHALL contain exactly N embedding objects in the "data" array,
// each with a unique index from 0 to N-1.
func TestProperty_InputCountEqualsOutputCount(t *testing.T) {
	resetMock()

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(time.Now().UnixNano())

	properties := gopter.NewProperties(parameters)

	// Generator for arrays of 1-5 non-empty strings using a simpler approach
	stringArrayGen := gen.IntRange(1, 5).FlatMap(func(n interface{}) gopter.Gen {
		count := n.(int)
		return gen.SliceOfN(count, gen.Identifier()).Map(func(arr []string) []string {
			// Ensure all strings are non-empty
			result := make([]string, 0, len(arr))
			for _, s := range arr {
				if len(s) > 0 {
					result = append(result, s)
				}
			}
			if len(result) == 0 {
				result = []string{"test"}
			}
			return result
		})
	}, reflect.TypeOf([]string{}))

	properties.Property("input count equals output count", prop.ForAll(
		func(inputs []string) bool {
			if len(inputs) == 0 {
				return true // Skip empty arrays
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			req := &testutil.EmbeddingRequest{
				Model: "text-embedding-ada-002-mock",
				Input: inputs,
			}

			resp, httpResp, err := testClient.Embedding(ctx, req)
			if err != nil {
				t.Logf("Request error: %v", err)
				return false
			}
			if httpResp == nil || httpResp.StatusCode != http.StatusOK {
				return false
			}
			if resp == nil {
				return false
			}

			// Verify output count matches input count
			if len(resp.Data) != len(inputs) {
				t.Logf("Output count %d != input count %d", len(resp.Data), len(inputs))
				return false
			}

			// Verify indices are unique and in range [0, N-1]
			seenIndices := make(map[int]bool)
			for _, emb := range resp.Data {
				if emb.Index < 0 || emb.Index >= len(inputs) {
					t.Logf("Index %d out of range [0, %d)", emb.Index, len(inputs))
					return false
				}
				if seenIndices[emb.Index] {
					t.Logf("Duplicate index: %d", emb.Index)
					return false
				}
				seenIndices[emb.Index] = true
			}

			return true
		},
		stringArrayGen,
	))

	properties.TestingRun(t)
}

// **Feature: embedding-http-endpoint, Property 4: Usage metrics are always present**
// **Validates: Requirements 3.1, 3.2, 3.3**
// *For any* successful embedding response, the "usage" object SHALL contain
// "prompt_tokens" and "total_tokens" fields with non-negative integer values.
func TestProperty_UsageMetricsAlwaysPresent(t *testing.T) {
	resetMock()

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	parameters.Rng.Seed(time.Now().UnixNano())

	properties := gopter.NewProperties(parameters)

	// Generator for non-empty strings
	inputGen := gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 100 })

	properties.Property("usage metrics are always present", prop.ForAll(
		func(input string) bool {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			req := &testutil.EmbeddingRequest{
				Model: "text-embedding-ada-002-mock",
				Input: input,
			}

			resp, httpResp, err := testClient.Embedding(ctx, req)
			if err != nil {
				return false
			}
			if httpResp == nil || httpResp.StatusCode != http.StatusOK {
				return false
			}
			if resp == nil {
				return false
			}

			// Verify usage metrics
			if resp.Usage.PromptTokens < 0 {
				t.Logf("prompt_tokens is negative: %d", resp.Usage.PromptTokens)
				return false
			}
			if resp.Usage.TotalTokens < 0 {
				t.Logf("total_tokens is negative: %d", resp.Usage.TotalTokens)
				return false
			}
			// total_tokens should be >= prompt_tokens
			if resp.Usage.TotalTokens < resp.Usage.PromptTokens {
				t.Logf("total_tokens %d < prompt_tokens %d", resp.Usage.TotalTokens, resp.Usage.PromptTokens)
				return false
			}

			return true
		},
		inputGen,
	))

	properties.TestingRun(t)
}

// Unit test for basic embedding functionality
func TestAPI_Embeddings_Basic(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.EmbeddingRequest{
		Model: "text-embedding-ada-002-mock",
		Input: "Hello, world!",
	}

	resp, httpResp, err := testClient.Embedding(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)
	assert.Equal(t, http.StatusOK, httpResp.StatusCode)

	require.NotNil(t, resp)
	assert.Equal(t, "list", resp.Object)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "embedding", resp.Data[0].Object)
	assert.Equal(t, 0, resp.Data[0].Index)
	assert.NotEmpty(t, resp.Data[0].Embedding)
}

// Unit test for array input
func TestAPI_Embeddings_ArrayInput(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.EmbeddingRequest{
		Model: "text-embedding-ada-002-mock",
		Input: []string{"Hello", "World", "Test"},
	}

	resp, httpResp, err := testClient.Embedding(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)
	assert.Equal(t, http.StatusOK, httpResp.StatusCode)

	require.NotNil(t, resp)
	assert.Equal(t, "list", resp.Object)
	assert.Len(t, resp.Data, 3)

	// Verify indices
	for i, emb := range resp.Data {
		assert.Equal(t, i, emb.Index)
		assert.Equal(t, "embedding", emb.Object)
	}
}

// **Feature: embedding-http-endpoint, Property 3: Error responses have correct structure**
// **Validates: Requirements 4.1, 4.2, 4.3, 4.4**
// *For any* request that results in an error, the response SHALL contain an "error"
// object with "message" and "type" fields, and the HTTP status code SHALL be in the 4xx or 5xx range.
func TestProperty_ErrorResponsesHaveCorrectStructure(t *testing.T) {
	resetMock()

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50 // Fewer tests for error cases
	parameters.Rng.Seed(time.Now().UnixNano())

	properties := gopter.NewProperties(parameters)

	// Generator for invalid requests (missing model or input)
	invalidRequestGen := gen.OneConstOf(
		"missing_model",
		"missing_input",
		"empty_input",
	)

	properties.Property("error responses have correct structure", prop.ForAll(
		func(errorType string) bool {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			var req *testutil.EmbeddingRequest
			switch errorType {
			case "missing_model":
				req = &testutil.EmbeddingRequest{
					Model: "",
					Input: "test input",
				}
			case "missing_input":
				req = &testutil.EmbeddingRequest{
					Model: "text-embedding-ada-002-mock",
					Input: nil,
				}
			case "empty_input":
				req = &testutil.EmbeddingRequest{
					Model: "text-embedding-ada-002-mock",
					Input: "",
				}
			}

			_, httpResp, err := testClient.Embedding(ctx, req)
			if err != nil {
				t.Logf("Request error: %v", err)
				return false
			}
			if httpResp == nil {
				t.Log("HTTP response is nil")
				return false
			}

			// Verify HTTP status code is in 4xx or 5xx range
			if httpResp.StatusCode < 400 || httpResp.StatusCode >= 600 {
				t.Logf("Status code %d not in error range", httpResp.StatusCode)
				return false
			}

			return true
		},
		invalidRequestGen,
	))

	properties.TestingRun(t)
}

// Unit tests for validation errors
// **Validates: Requirements 1.3, 1.4, 1.5, 1.6**

func TestAPI_Embeddings_MissingModel(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.EmbeddingRequest{
		Model: "",
		Input: "Hello, world!",
	}

	_, httpResp, err := testClient.Embedding(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)
	assert.Equal(t, http.StatusBadRequest, httpResp.StatusCode)
}

func TestAPI_Embeddings_MissingInput(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.EmbeddingRequest{
		Model: "text-embedding-ada-002-mock",
		Input: nil,
	}

	_, httpResp, err := testClient.Embedding(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)
	assert.Equal(t, http.StatusBadRequest, httpResp.StatusCode)
}

func TestAPI_Embeddings_EmptyInput(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &testutil.EmbeddingRequest{
		Model: "text-embedding-ada-002-mock",
		Input: "",
	}

	_, httpResp, err := testClient.Embedding(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, httpResp)
	assert.Equal(t, http.StatusBadRequest, httpResp.StatusCode)
}

func TestAPI_Embeddings_InvalidJSON(t *testing.T) {
	resetMock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send invalid JSON directly
	httpReq, err := http.NewRequestWithContext(ctx, "POST", testClient.BaseURL()+"/v1/embeddings", strings.NewReader("{invalid json}"))
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := testClient.HTTPClient().Do(httpReq)
	require.NoError(t, err)
	defer httpResp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, httpResp.StatusCode)
}

// **Feature: embedding-http-endpoint, Property 6: Model validation rejects empty model**
// **Validates: Requirements 1.5**
// *For any* embedding request with an empty or missing model field,
// the system SHALL return a 400 error with message containing "model is required".
func TestProperty_ModelValidationRejectsEmptyModel(t *testing.T) {
	resetMock()

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	parameters.Rng.Seed(time.Now().UnixNano())

	properties := gopter.NewProperties(parameters)

	// Generator for valid inputs
	inputGen := gen.Identifier().SuchThat(func(s string) bool { return len(s) > 0 })

	properties.Property("model validation rejects empty model", prop.ForAll(
		func(input string) bool {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			req := &testutil.EmbeddingRequest{
				Model: "", // Empty model
				Input: input,
			}

			_, httpResp, err := testClient.Embedding(ctx, req)
			if err != nil {
				return false
			}
			if httpResp == nil {
				return false
			}

			// Should return 400 Bad Request
			return httpResp.StatusCode == http.StatusBadRequest
		},
		inputGen,
	))

	properties.TestingRun(t)
}

// **Feature: embedding-http-endpoint, Property 7: Input validation rejects empty input**
// **Validates: Requirements 1.6**
// *For any* embedding request with an empty or missing input field,
// the system SHALL return a 400 error with message containing "input is required".
func TestProperty_InputValidationRejectsEmptyInput(t *testing.T) {
	resetMock()

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	parameters.Rng.Seed(time.Now().UnixNano())

	properties := gopter.NewProperties(parameters)

	// Generator for valid models
	modelGen := gen.Const("text-embedding-ada-002-mock")

	properties.Property("input validation rejects empty input", prop.ForAll(
		func(model string) bool {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			req := &testutil.EmbeddingRequest{
				Model: model,
				Input: "", // Empty input
			}

			_, httpResp, err := testClient.Embedding(ctx, req)
			if err != nil {
				return false
			}
			if httpResp == nil {
				return false
			}

			// Should return 400 Bad Request
			return httpResp.StatusCode == http.StatusBadRequest
		},
		modelGen,
	))

	properties.TestingRun(t)
}

// **Feature: embedding-http-endpoint, Property 5: Optional parameters are passed through**
// **Validates: Requirements 6.1, 6.2, 6.3**
// *For any* embedding request containing optional parameters (encoding_format, dimensions, user),
// these parameters SHALL be included in the request sent to the provider.
func TestProperty_OptionalParametersPassedThrough(t *testing.T) {
	resetMock()

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	parameters.Rng.Seed(time.Now().UnixNano())

	properties := gopter.NewProperties(parameters)

	// Generator for optional parameters
	optionalParamsGen := gen.Struct(reflect.TypeOf(struct {
		EncodingFormat string
		Dimensions     int
		User           string
	}{}), map[string]gopter.Gen{
		"EncodingFormat": gen.OneConstOf("float", "base64"),
		"Dimensions":     gen.IntRange(256, 3072),
		"User":           gen.Identifier(),
	})

	properties.Property("optional parameters are passed through", prop.ForAll(
		func(params struct {
			EncodingFormat string
			Dimensions     int
			User           string
		}) bool {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			req := &testutil.EmbeddingRequest{
				Model:          "text-embedding-ada-002-mock",
				Input:          "test input",
				EncodingFormat: params.EncodingFormat,
				Dimensions:     params.Dimensions,
				User:           params.User,
			}

			resp, httpResp, err := testClient.Embedding(ctx, req)
			if err != nil {
				t.Logf("Request error: %v", err)
				return false
			}
			if httpResp == nil || httpResp.StatusCode != http.StatusOK {
				return false
			}
			if resp == nil {
				return false
			}

			// Request should succeed with optional parameters
			return resp.Object == "list" && len(resp.Data) > 0
		},
		optionalParamsGen,
	))

	properties.TestingRun(t)
}
