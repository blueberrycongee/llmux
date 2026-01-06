package pool

import (
	"sync"

	"github.com/blueberrycongee/llmux/pkg/types"
)

var (
	requestPool = sync.Pool{
		New: func() any {
			return new(types.ChatRequest)
		},
	}

	responsePool = sync.Pool{
		New: func() any {
			return new(types.ChatResponse)
		},
	}
)

// GetChatRequest gets a ChatRequest from the pool.
func GetChatRequest() *types.ChatRequest {
	return requestPool.Get().(*types.ChatRequest)
}

// PutChatRequest returns a ChatRequest to the pool.
// It resets the request before returning it.
func PutChatRequest(req *types.ChatRequest) {
	req.Reset()
	requestPool.Put(req)
}

// GetChatResponse gets a ChatResponse from the pool.
func GetChatResponse() *types.ChatResponse {
	return responsePool.Get().(*types.ChatResponse)
}

// PutChatResponse returns a ChatResponse to the pool.
// It resets the response before returning it.
func PutChatResponse(resp *types.ChatResponse) {
	resp.Reset()
	responsePool.Put(resp)
}
