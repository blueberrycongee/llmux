package httputil

import (
	"errors"
	"strings"
	"testing"
)

func TestReadLimitedBody_AllowsWithinLimit(t *testing.T) {
	body, err := ReadLimitedBody(strings.NewReader("hello"), 10)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if string(body) != "hello" {
		t.Fatalf("unexpected body: %s", string(body))
	}
}

func TestReadLimitedBody_RejectsOversize(t *testing.T) {
	body, err := ReadLimitedBody(strings.NewReader("helloworld"), 5)
	if !errors.Is(err, ErrResponseBodyTooLarge) {
		t.Fatalf("expected ErrResponseBodyTooLarge, got %v", err)
	}
	if string(body) != "hello" {
		t.Fatalf("unexpected body: %s", string(body))
	}
}
