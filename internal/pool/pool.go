// Package pool provides object pooling for request and response types
// to reduce memory allocations and improve performance.
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
	v := requestPool.Get()
	if req, ok := v.(*types.ChatRequest); ok {
		return req
	}
	return new(types.ChatRequest)
}

// PutChatRequest returns a ChatRequest to the pool.
// It resets the request before returning it.
func PutChatRequest(req *types.ChatRequest) {
	req.Reset()
	requestPool.Put(req)
}

// GetChatResponse gets a ChatResponse from the pool.
func GetChatResponse() *types.ChatResponse {
	v := responsePool.Get()
	if resp, ok := v.(*types.ChatResponse); ok {
		return resp
	}
	return new(types.ChatResponse)
}

// PutChatResponse returns a ChatResponse to the pool.
// It resets the response before returning it.
func PutChatResponse(resp *types.ChatResponse) {
	resp.Reset()
	responsePool.Put(resp)
}
