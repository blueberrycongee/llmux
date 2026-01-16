package routers

import (
	"sync"
)

// EWMA represents an Exponentially Weighted Moving Average.
// It is used to track metrics that change over time, giving more weight to recent values.
type EWMA struct {
	alpha float64
	value float64
	initialized bool
	mu sync.RWMutex
}

// NewEWMA creates a new EWMA with the given alpha (smoothing factor).
// alpha should be between 0 and 1. A higher alpha discounts older observations faster.
func NewEWMA(alpha float64) *EWMA {
	return &EWMA{
		alpha: alpha,
	}
}

// Add updates the moving average with a new value.
func (e *EWMA) Add(newValue float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.initialized {
		e.value = newValue
		e.initialized = true
	} else {
		e.value = (e.alpha * newValue) + (1.0 - e.alpha) * e.value
	}
}

// Value returns the current moving average value.
func (e *EWMA) Value() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.value
}

// SetAlpha updates the smoothing factor.
func (e *EWMA) SetAlpha(alpha float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.alpha = alpha
}
