package metrics

import (
	"strings"
	"testing"
)

func TestSanitizeModelLabel_StripsProviderPrefix(t *testing.T) {
	if got := sanitizeModelLabel("openai/gpt-4o-mini"); got != "gpt-4o-mini" {
		t.Fatalf("sanitizeModelLabel = %q, want %q", got, "gpt-4o-mini")
	}
}

func TestSanitizeModelLabel_ReplacesInvalidChars(t *testing.T) {
	got := sanitizeModelLabel("gpt-4o-mini\n\t🚨")
	if strings.ContainsAny(got, "\n\t") {
		t.Fatalf("sanitizeModelLabel contains whitespace: %q", got)
	}
	if got == "unknown" {
		t.Fatalf("sanitizeModelLabel unexpectedly returned %q", got)
	}
}

func TestSanitizeModelLabel_CapsLength(t *testing.T) {
	long := strings.Repeat("a", maxModelLabelLen+50)
	got := sanitizeModelLabel(long)
	if len(got) != maxModelLabelLen {
		t.Fatalf("sanitizeModelLabel len=%d, want %d", len(got), maxModelLabelLen)
	}
}

func TestSanitizeModelLabel_EmptyFallback(t *testing.T) {
	if got := sanitizeModelLabel("   "); got != "unknown" {
		t.Fatalf("sanitizeModelLabel = %q, want %q", got, "unknown")
	}
}
