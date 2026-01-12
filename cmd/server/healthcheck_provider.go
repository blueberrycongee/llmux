package main

import (
	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/api"
)

type swapperClientProvider struct {
	swapper *api.ClientSwapper
}

func (p swapperClientProvider) Acquire() (*llmux.Client, func()) {
	if p.swapper == nil {
		return nil, func() {}
	}
	return p.swapper.Acquire()
}
