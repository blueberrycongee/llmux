// Package providers provides a unified registry for all LLMux provider implementations.
package providers

import (
	"fmt"
	"sync"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/ai21"
	"github.com/blueberrycongee/llmux/providers/anthropic"
	"github.com/blueberrycongee/llmux/providers/anyscale"
	"github.com/blueberrycongee/llmux/providers/azure"
	"github.com/blueberrycongee/llmux/providers/baichuan"
	"github.com/blueberrycongee/llmux/providers/bedrock"
	"github.com/blueberrycongee/llmux/providers/cerebras"
	"github.com/blueberrycongee/llmux/providers/cloudflare"
	"github.com/blueberrycongee/llmux/providers/cohere"
	"github.com/blueberrycongee/llmux/providers/databricks"
	"github.com/blueberrycongee/llmux/providers/deepinfra"
	"github.com/blueberrycongee/llmux/providers/deepseek"
	"github.com/blueberrycongee/llmux/providers/fireworks"
	"github.com/blueberrycongee/llmux/providers/gemini"
	"github.com/blueberrycongee/llmux/providers/github"
	"github.com/blueberrycongee/llmux/providers/groq"
	"github.com/blueberrycongee/llmux/providers/huggingface"
	"github.com/blueberrycongee/llmux/providers/hunyuan"
	"github.com/blueberrycongee/llmux/providers/hyperbolic"
	"github.com/blueberrycongee/llmux/providers/lambda"
	"github.com/blueberrycongee/llmux/providers/lmstudio"
	"github.com/blueberrycongee/llmux/providers/minimax"
	"github.com/blueberrycongee/llmux/providers/mistral"
	"github.com/blueberrycongee/llmux/providers/moonshot"
	"github.com/blueberrycongee/llmux/providers/novita"
	"github.com/blueberrycongee/llmux/providers/nvidia"
	"github.com/blueberrycongee/llmux/providers/ollama"
	"github.com/blueberrycongee/llmux/providers/openai"
	"github.com/blueberrycongee/llmux/providers/openrouter"
	"github.com/blueberrycongee/llmux/providers/perplexity"
	"github.com/blueberrycongee/llmux/providers/qwen"
	"github.com/blueberrycongee/llmux/providers/replicate"
	"github.com/blueberrycongee/llmux/providers/sambanova"
	"github.com/blueberrycongee/llmux/providers/siliconflow"
	"github.com/blueberrycongee/llmux/providers/snowflake"
	"github.com/blueberrycongee/llmux/providers/stepfun"
	"github.com/blueberrycongee/llmux/providers/together"
	"github.com/blueberrycongee/llmux/providers/vertexai"
	"github.com/blueberrycongee/llmux/providers/vllm"
	"github.com/blueberrycongee/llmux/providers/volcengine"
	"github.com/blueberrycongee/llmux/providers/watsonx"
	"github.com/blueberrycongee/llmux/providers/xai"
	"github.com/blueberrycongee/llmux/providers/yi"
	"github.com/blueberrycongee/llmux/providers/zhipu"
)

var (
	registry     = make(map[string]provider.Factory)
	registryOnce sync.Once
	registryMu   sync.RWMutex
)

// Register registers a provider factory with the given type name.
func Register(providerType string, factory provider.Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[providerType] = factory
}

// Get returns the factory for the given provider type.
func Get(providerType string) (provider.Factory, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	f, ok := registry[providerType]
	return f, ok
}

// Create creates a provider instance from configuration.
func Create(cfg provider.Config) (provider.Provider, error) {
	registryMu.RLock()
	factory, ok := registry[cfg.Type]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown provider type: %s (available: %v)", cfg.Type, List())
	}
	return factory(cfg)
}

// List returns all registered provider type names.
func List() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// RegisterBuiltins registers all built-in provider factories.
func RegisterBuiltins() {
	registryOnce.Do(func() {
		// Commercial providers
		Register("openai", openai.NewFromConfig)
		Register("anthropic", anthropic.NewFromConfig)
		Register("azure", azure.NewFromConfig)
		Register("gemini", gemini.NewFromConfig)
		Register("bedrock", bedrock.NewFromConfig)
		Register("vertexai", vertexai.NewFromConfig)
		Register("cohere", cohere.NewFromConfig)
		Register("mistral", mistral.NewFromConfig)

		// Fast inference providers
		Register("groq", groq.NewFromConfig)
		Register("cerebras", cerebras.NewFromConfig)
		Register("sambanova", sambanova.NewFromConfig)
		Register("fireworks", fireworks.NewFromConfig)
		Register("together", together.NewFromConfig)

		// Aggregators
		Register("openrouter", openrouter.NewFromConfig)
		Register("deepinfra", deepinfra.NewFromConfig)
		Register("huggingface", huggingface.NewFromConfig)
		Register("replicate", replicate.NewFromConfig)
		Register("anyscale", anyscale.NewFromConfig)

		// Specialized providers
		Register("deepseek", deepseek.NewFromConfig)
		Register("perplexity", perplexity.NewFromConfig)
		Register("xai", xai.NewFromConfig)
		Register("ai21", ai21.NewFromConfig)
		Register("hyperbolic", hyperbolic.NewFromConfig)
		Register("lambda", lambda.NewFromConfig)
		Register("novita", novita.NewFromConfig)
		Register("nvidia", nvidia.NewFromConfig)

		// Chinese providers
		Register("qwen", qwen.NewFromConfig)
		Register("zhipu", zhipu.NewFromConfig)
		Register("moonshot", moonshot.NewFromConfig)
		Register("baichuan", baichuan.NewFromConfig)
		Register("hunyuan", hunyuan.NewFromConfig)
		Register("volcengine", volcengine.NewFromConfig)
		Register("siliconflow", siliconflow.NewFromConfig)
		Register("yi", yi.NewFromConfig)
		Register("stepfun", stepfun.NewFromConfig)
		Register("minimax", minimax.NewFromConfig)

		// Self-hosted
		Register("ollama", ollama.NewFromConfig)
		Register("lmstudio", lmstudio.NewFromConfig)
		Register("vllm", vllm.NewFromConfig)

		// Enterprise/Cloud
		Register("databricks", databricks.NewFromConfig)
		Register("snowflake", snowflake.NewFromConfig)
		Register("watsonx", watsonx.NewFromConfig)
		Register("cloudflare", cloudflare.NewFromConfig)
		Register("github", github.NewFromConfig)
	})
}

func init() {
	RegisterBuiltins()
}
