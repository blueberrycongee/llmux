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
	if req == nil {
		return nil
	}

	_, modelName := types.SplitProviderModel(req.Model)
	needsClone := len(req.Tags) > 0 || (modelName != "" && modelName != req.Model)
	if !needsClone {
		return req
	}

	cloned := *req
	cloned.Tags = nil
	if modelName != "" && modelName != cloned.Model {
		cloned.Model = modelName
	}
	return &cloned
}
