package types

import (
	"strings"
	"testing"
)

func TestValidateModelName(t *testing.T) {
	valid := strings.Repeat("a", MaxModelNameLength)
	if err := ValidateModelName(valid); err != nil {
		t.Fatalf("expected valid model name, got error: %v", err)
	}

	invalid := strings.Repeat("a", MaxModelNameLength+1)
	if err := ValidateModelName(invalid); err == nil {
		t.Fatal("expected error for too-long model name")
	}
}
