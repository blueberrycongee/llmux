package types

import "testing"

func TestSplitProviderModel(t *testing.T) {
	tests := []struct {
		in    string
		prov  string
		model string
	}{
		{in: "", prov: "", model: ""},
		{in: "gpt-4o", prov: "", model: "gpt-4o"},
		{in: "openai/gpt-4o", prov: "openai", model: "gpt-4o"},
		{in: " openai/gpt-4o ", prov: "openai", model: "gpt-4o"},
		{in: "/gpt-4o", prov: "", model: "/gpt-4o"},
		{in: "openai/", prov: "", model: "openai/"},
		{in: "a/b/c", prov: "a", model: "b/c"},
	}

	for _, tc := range tests {
		prov, model := SplitProviderModel(tc.in)
		if prov != tc.prov || model != tc.model {
			t.Fatalf("SplitProviderModel(%q) = (%q,%q), want (%q,%q)", tc.in, prov, model, tc.prov, tc.model)
		}
	}
}
