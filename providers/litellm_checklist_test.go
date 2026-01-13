package providers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiteLLMProviderChecklist_AlignedProvidersRegistered(t *testing.T) {
	data, err := os.ReadFile(filepath.FromSlash("docs/LITELLM_PROVIDER_CHECKLIST.md"))
	if err != nil {
		data, err = os.ReadFile(filepath.FromSlash("../docs/LITELLM_PROVIDER_CHECKLIST.md"))
	}
	if err != nil {
		t.Fatalf("read checklist: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	inTable := false

	var seenRows int
	var alignedRows int
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "## Provider Modules") {
			inTable = false
			continue
		}
		if strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "## Provider Modules") {
			inTable = false
		}
		if strings.HasPrefix(line, "| Provider |") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if line == "" || strings.HasPrefix(line, "| ---") {
			continue
		}
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}

		cols := strings.Split(line, "|")
		if len(cols) < 4 {
			t.Fatalf("unexpected checklist table row: %q", line)
		}

		providerName := strings.TrimSpace(cols[1])
		if providerName == "" || providerName == "Provider" {
			continue
		}
		status := strings.TrimSpace(cols[2])
		seenRows++

		switch status {
		case "[x]":
			alignedRows++
			if _, ok := Get(providerName); !ok {
				t.Errorf("checklist marks %q aligned, but it is not registered", providerName)
			}
		case "[~]":
			// explicitly deferred / not applicable
		default:
			t.Errorf("provider %q has unexpected status %q (expected [x] or [~])", providerName, status)
		}
	}

	if seenRows == 0 {
		t.Fatalf("no provider rows parsed from checklist")
	}
	if alignedRows == 0 {
		t.Fatalf("no aligned providers found in checklist")
	}
}
