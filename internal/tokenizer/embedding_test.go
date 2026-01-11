package tokenizer

import (
	"testing"

	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestEstimateEmbeddingTokens_StringInput(t *testing.T) {
	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromString("hello world"),
	}

	got := EstimateEmbeddingTokens(req.Model, req)
	want := CountTextTokens(req.Model, "hello world")

	if diff := absDiff(got, want); diff > 1 {
		t.Fatalf("EstimateEmbeddingTokens() diff = %d, want <= 1 (got=%d want=%d)", diff, got, want)
	}
}

func TestEstimateEmbeddingTokens_StringArrayInput(t *testing.T) {
	texts := []string{"hello world", "embedding tokens"}
	req := &types.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: types.NewEmbeddingInputFromStrings(texts),
	}

	got := EstimateEmbeddingTokens(req.Model, req)
	want := CountTextTokens(req.Model, texts[0]) + CountTextTokens(req.Model, texts[1])

	if diff := absDiff(got, want); diff > 2 {
		t.Fatalf("EstimateEmbeddingTokens() diff = %d, want <= 2 (got=%d want=%d)", diff, got, want)
	}
}

func absDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}
