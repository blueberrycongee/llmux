package providers

import (
	"testing"

	"github.com/blueberrycongee/llmux/internal/provider"
)

func TestProviderFactories(t *testing.T) {
	// Test that all factories are registered
	if len(ProviderFactories) < 40 {
		t.Errorf("Expected at least 40 provider factories, got %d", len(ProviderFactories))
	}

	// Test key providers exist
	requiredProviders := []string{
		"openai",
		"anthropic",
		"gemini",
		"azure",
		"bedrock",
		"groq",
		"deepseek",
		"together",
		"fireworks",
		"mistral",
		"cohere",
		"perplexity",
		"openrouter",
		"ollama",
		"qwen",
		"zhipu",
	}

	for _, name := range requiredProviders {
		if _, ok := ProviderFactories[name]; !ok {
			t.Errorf("Required provider %q not found in ProviderFactories", name)
		}
	}
}

func TestProviderCount(t *testing.T) {
	count := ProviderCount()
	if count < 40 {
		t.Errorf("ProviderCount() = %d, want >= 40", count)
	}
}

func TestRegisterAllProviders(t *testing.T) {
	registry := provider.NewRegistry()
	RegisterAllProviders(registry)

	// Test by creating a provider - this verifies factories are registered
	cfg := provider.ProviderConfig{
		Name:   "test-openai",
		Type:   "openai",
		APIKey: "test-key",
		Models: []string{"gpt-4"},
	}
	
	p, err := registry.CreateProvider(cfg)
	if err != nil {
		t.Fatalf("CreateProvider() error = %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("Provider.Name() = %v, want openai", p.Name())
	}
}

func TestGetProviderInfo(t *testing.T) {
	tests := []struct {
		name       string
		wantNil    bool
		displayName string
	}{
		{"openai", false, "OpenAI"},
		{"anthropic", false, "Anthropic"},
		{"groq", false, "Groq"},
		{"nonexistent", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := GetProviderInfo(tt.name)
			if (info == nil) != tt.wantNil {
				t.Errorf("GetProviderInfo(%q) = %v, wantNil %v", tt.name, info, tt.wantNil)
			}
			if !tt.wantNil && info.DisplayName != tt.displayName {
				t.Errorf("DisplayName = %v, want %v", info.DisplayName, tt.displayName)
			}
		})
	}
}

func TestGetProvidersByCategory(t *testing.T) {
	tests := []struct {
		category string
		minCount int
	}{
		{"commercial", 3},
		{"fast-inference", 3},
		{"chinese", 5},
		{"self-hosted", 2},
		{"aggregator", 2},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			providers := GetProvidersByCategory(tt.category)
			if len(providers) < tt.minCount {
				t.Errorf("GetProvidersByCategory(%q) returned %d providers, want >= %d",
					tt.category, len(providers), tt.minCount)
			}
		})
	}
}

func TestAllProvidersHaveRequiredFields(t *testing.T) {
	for _, info := range AllProviders {
		t.Run(info.Name, func(t *testing.T) {
			if info.Name == "" {
				t.Error("Provider name is empty")
			}
			if info.DisplayName == "" {
				t.Error("Provider display name is empty")
			}
			if info.Description == "" {
				t.Error("Provider description is empty")
			}
			if info.Website == "" {
				t.Error("Provider website is empty")
			}
			if len(info.Categories) == 0 {
				t.Error("Provider has no categories")
			}
		})
	}
}

func TestFactoryCreatesValidProvider(t *testing.T) {
	// Test a few representative providers
	testCases := []struct {
		providerType string
		apiKey       string
	}{
		{"openai", "test-key"},
		{"anthropic", "test-key"},
		{"groq", "test-key"},
		{"deepseek", "test-key"},
		{"together", "test-key"},
	}

	for _, tc := range testCases {
		t.Run(tc.providerType, func(t *testing.T) {
			factory, ok := ProviderFactories[tc.providerType]
			if !ok {
				t.Fatalf("Factory for %q not found", tc.providerType)
			}

			cfg := provider.ProviderConfig{
				APIKey: tc.apiKey,
				Models: []string{"test-model"},
			}

			p, err := factory(cfg)
			if err != nil {
				t.Fatalf("Factory() error = %v", err)
			}

			if p == nil {
				t.Fatal("Factory() returned nil provider")
			}

			// Provider should implement the interface
			if p.Name() == "" {
				t.Error("Provider.Name() returned empty string")
			}
		})
	}
}
