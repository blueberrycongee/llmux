package llmux

import (
	"reflect"
	"testing"

	"github.com/blueberrycongee/llmux/pkg/types"
)

func TestBuildRouterRequestContext_NilRequest(t *testing.T) {
	reqCtx := buildRouterRequestContext(nil, 0, false)
	if reqCtx == nil {
		t.Fatalf("expected non-nil request context")
	}
	if reqCtx.Model != "" || reqCtx.IsStreaming || len(reqCtx.Tags) != 0 || reqCtx.EstimatedInputTokens != 0 {
		t.Fatalf("expected zero-value fields for nil request, got %+v", reqCtx)
	}
}

func TestSanitizeChatRequestForProvider_RemovesTags(t *testing.T) {
	req := &types.ChatRequest{
		Model: "test-model",
		Tags:  []string{"a", "b"},
	}

	sanitized := sanitizeChatRequestForProvider(req)
	if sanitized == req {
		t.Fatalf("expected sanitized request to be a copy when tags are present")
	}
	if len(sanitized.Tags) != 0 {
		t.Fatalf("expected tags to be removed, got %v", sanitized.Tags)
	}
	if !reflect.DeepEqual(req.Tags, []string{"a", "b"}) {
		t.Fatalf("expected original tags to remain unchanged")
	}
}

func TestSanitizeChatRequestForProvider_NoTags(t *testing.T) {
	req := &types.ChatRequest{Model: "test-model"}
	sanitized := sanitizeChatRequestForProvider(req)
	if sanitized != req {
		t.Fatalf("expected same request when no tags are present")
	}
}

func TestSanitizeChatRequestForProvider_StripsProviderPrefixInModel(t *testing.T) {
	req := &types.ChatRequest{
		Model: "openai/test-model",
	}
	sanitized := sanitizeChatRequestForProvider(req)
	if sanitized == req {
		t.Fatalf("expected sanitized request to be a copy when model has provider prefix")
	}
	if sanitized.Model != "test-model" {
		t.Fatalf("expected provider prefix stripped, got %q", sanitized.Model)
	}
	if req.Model != "openai/test-model" {
		t.Fatalf("expected original model to remain unchanged")
	}
}
