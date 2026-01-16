// Package httputil provides helpers for working with HTTP payloads safely.
package httputil

import (
	"errors"
	"io"
)

const (
	// DefaultMaxResponseBodyBytes caps upstream response bodies to 10MB.
	DefaultMaxResponseBodyBytes int64 = 10 * 1024 * 1024
)

var ErrResponseBodyTooLarge = errors.New("response body too large")

// ReadLimitedBody reads up to maxBytes from reader and returns ErrResponseBodyTooLarge when exceeded.
func ReadLimitedBody(reader io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return io.ReadAll(reader)
	}

	limited := io.LimitReader(reader, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return body, err
	}
	if int64(len(body)) > maxBytes {
		body = body[:int(maxBytes)]
		return body, ErrResponseBodyTooLarge
	}
	return body, nil
}
