package llmux

import (
	"github.com/blueberrycongee/llmux/pkg/router"
	"github.com/blueberrycongee/llmux/pkg/types"
)

func buildRouterRequestContext(req *types.ChatRequest, promptTokens int, isStreaming bool) *router.RequestContext {
	if req == nil {
		return &router.RequestContext{}
	}

	tags := make([]string, len(req.Tags))
	copy(tags, req.Tags)

	return &router.RequestContext{
		Model:                req.Model,
		IsStreaming:          isStreaming,
		Tags:                 tags,
		EstimatedInputTokens: promptTokens,
	}
}

func sanitizeChatRequestForProvider(req *types.ChatRequest) *types.ChatRequest {
	if req == nil || len(req.Tags) == 0 {
		return req
	}

	cloned := *req
	cloned.Tags = nil
	return &cloned
}
