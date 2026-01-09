// Package providers provides a centralized registry for all LLM providers.
// It enables easy registration and discovery of available providers.
package providers

import (
	"github.com/blueberrycongee/llmux/internal/provider"
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

// ProviderFactories maps provider type names to their factory functions.
// This allows dynamic provider creation based on configuration.
var ProviderFactories = map[string]provider.ProviderFactory{
	// === Tier 1: Major Commercial Providers ===
	"openai":    provider.AdaptFactory(openai.NewFromConfig),
	"anthropic": provider.AdaptFactory(anthropic.NewFromConfig),
	"gemini":    provider.AdaptFactory(gemini.NewFromConfig),
	"azure":     provider.AdaptFactory(azure.NewFromConfig),
	"bedrock":   provider.AdaptFactory(bedrock.NewFromConfig),
	"cohere":    provider.AdaptFactory(cohere.NewFromConfig),
	"mistral":   provider.AdaptFactory(mistral.NewFromConfig),

	// === Tier 2: Fast Inference Providers ===
	"groq":      provider.AdaptFactory(groq.NewFromConfig),
	"cerebras":  provider.AdaptFactory(cerebras.NewFromConfig),
	"sambanova": provider.AdaptFactory(sambanova.NewFromConfig),
	"fireworks": provider.AdaptFactory(fireworks.NewFromConfig),
	"together":  provider.AdaptFactory(together.NewFromConfig),

	// === Tier 3: Model Aggregators ===
	"openrouter":  provider.AdaptFactory(openrouter.NewFromConfig),
	"deepinfra":   provider.AdaptFactory(deepinfra.NewFromConfig),
	"huggingface": provider.AdaptFactory(huggingface.NewFromConfig),
	"anyscale":    provider.AdaptFactory(anyscale.NewFromConfig),
	"replicate":   provider.AdaptFactory(replicate.NewFromConfig),

	// === Tier 4: Specialized Providers ===
	"deepseek":   provider.AdaptFactory(deepseek.NewFromConfig),
	"perplexity": provider.AdaptFactory(perplexity.NewFromConfig),
	"xai":        provider.AdaptFactory(xai.NewFromConfig),
	"ai21":       provider.AdaptFactory(ai21.NewFromConfig),
	"nvidia":     provider.AdaptFactory(nvidia.NewFromConfig),

	// === Tier 5: Chinese Providers ===
	"qwen":        provider.AdaptFactory(qwen.NewFromConfig),
	"zhipu":       provider.AdaptFactory(zhipu.NewFromConfig),
	"moonshot":    provider.AdaptFactory(moonshot.NewFromConfig),
	"baichuan":    provider.AdaptFactory(baichuan.NewFromConfig),
	"minimax":     provider.AdaptFactory(minimax.NewFromConfig),
	"yi":          provider.AdaptFactory(yi.NewFromConfig),
	"volcengine":  provider.AdaptFactory(volcengine.NewFromConfig),
	"hunyuan":     provider.AdaptFactory(hunyuan.NewFromConfig),
	"stepfun":     provider.AdaptFactory(stepfun.NewFromConfig),
	"siliconflow": provider.AdaptFactory(siliconflow.NewFromConfig),

	// === Tier 6: GPU Cloud Providers ===
	"lambda":     provider.AdaptFactory(lambda.NewFromConfig),
	"hyperbolic": provider.AdaptFactory(hyperbolic.NewFromConfig),
	"novita":     provider.AdaptFactory(novita.NewFromConfig),

	// === Tier 7: Local/Self-hosted Providers ===
	"ollama":   provider.AdaptFactory(ollama.NewFromConfig),
	"lmstudio": provider.AdaptFactory(lmstudio.NewFromConfig),
	"vllm":     provider.AdaptFactory(vllm.NewFromConfig),

	// === Tier 8: Cloud/Enterprise Providers ===
	"vertexai":   provider.AdaptFactory(vertexai.NewFromConfig),
	"github":     provider.AdaptFactory(github.NewFromConfig),
	"cloudflare": provider.AdaptFactory(cloudflare.NewFromConfig),
	"databricks": provider.AdaptFactory(databricks.NewFromConfig),
	"snowflake":  provider.AdaptFactory(snowflake.NewFromConfig),
	"watsonx":    provider.AdaptFactory(watsonx.NewFromConfig),
}

// ProviderInfo describes a provider's capabilities and configuration.
type ProviderInfo struct {
	Name          string   // Provider identifier
	DisplayName   string   // Human-readable name
	Description   string   // Brief description
	Website       string   // Provider website
	DefaultModels []string // Default model list
	Categories    []string // Provider categories
}

// AllProviders returns information about all supported providers.
var AllProviders = []ProviderInfo{
	// Tier 1: Major Commercial Providers
	{
		Name:          "openai",
		DisplayName:   "OpenAI",
		Description:   "GPT-4, GPT-4o, GPT-3.5 Turbo and other models from OpenAI",
		Website:       "https://openai.com",
		DefaultModels: []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"},
		Categories:    []string{"commercial", "general-purpose"},
	},
	{
		Name:          "anthropic",
		DisplayName:   "Anthropic",
		Description:   "Claude 3.5, Claude 3 Opus, Sonnet, and Haiku models",
		Website:       "https://anthropic.com",
		DefaultModels: []string{"claude-3-5-sonnet-20241022", "claude-3-opus-20240229"},
		Categories:    []string{"commercial", "reasoning"},
	},
	{
		Name:          "gemini",
		DisplayName:   "Google Gemini",
		Description:   "Gemini Pro, Gemini Ultra, and Gemini Flash models",
		Website:       "https://ai.google.dev",
		DefaultModels: []string{"gemini-1.5-pro", "gemini-1.5-flash", "gemini-2.0-flash-exp"},
		Categories:    []string{"commercial", "multimodal"},
	},
	{
		Name:          "azure",
		DisplayName:   "Azure OpenAI",
		Description:   "OpenAI models deployed on Microsoft Azure",
		Website:       "https://azure.microsoft.com/en-us/products/ai-services/openai-service",
		DefaultModels: []string{"gpt-4o", "gpt-4"},
		Categories:    []string{"enterprise", "cloud"},
	},
	{
		Name:          "bedrock",
		DisplayName:   "AWS Bedrock",
		Description:   "Claude, Llama, Titan, and other models on AWS",
		Website:       "https://aws.amazon.com/bedrock",
		DefaultModels: []string{"anthropic.claude-3-5-sonnet-20241022-v2:0"},
		Categories:    []string{"enterprise", "cloud"},
	},
	{
		Name:          "cohere",
		DisplayName:   "Cohere",
		Description:   "Command R+ and Command R models for enterprise RAG",
		Website:       "https://cohere.com",
		DefaultModels: []string{"command-r-plus", "command-r"},
		Categories:    []string{"commercial", "enterprise"},
	},
	{
		Name:          "mistral",
		DisplayName:   "Mistral AI",
		Description:   "Mistral Large, Medium, and open-weight models",
		Website:       "https://mistral.ai",
		DefaultModels: []string{"mistral-large-latest", "mistral-medium-latest"},
		Categories:    []string{"commercial", "open-weights"},
	},

	// Tier 2: Fast Inference Providers
	{
		Name:          "groq",
		DisplayName:   "Groq",
		Description:   "Ultra-fast inference for Llama, Mixtral, and Gemma",
		Website:       "https://groq.com",
		DefaultModels: groq.DefaultModels,
		Categories:    []string{"fast-inference", "open-source"},
	},
	{
		Name:          "cerebras",
		DisplayName:   "Cerebras",
		Description:   "Fastest inference on Wafer Scale Engine",
		Website:       "https://cerebras.ai",
		DefaultModels: cerebras.DefaultModels,
		Categories:    []string{"fast-inference"},
	},
	{
		Name:          "sambanova",
		DisplayName:   "SambaNova",
		Description:   "Ultra-fast inference on custom RDU hardware",
		Website:       "https://sambanova.ai",
		DefaultModels: sambanova.DefaultModels,
		Categories:    []string{"fast-inference"},
	},
	{
		Name:          "fireworks",
		DisplayName:   "Fireworks AI",
		Description:   "Fast and affordable inference for open-source models",
		Website:       "https://fireworks.ai",
		DefaultModels: fireworks.DefaultModels,
		Categories:    []string{"fast-inference", "open-source"},
	},
	{
		Name:          "together",
		DisplayName:   "Together AI",
		Description:   "Access to 100+ open-source models",
		Website:       "https://together.ai",
		DefaultModels: together.DefaultModels,
		Categories:    []string{"fast-inference", "open-source"},
	},

	// Tier 3: Model Aggregators
	{
		Name:          "openrouter",
		DisplayName:   "OpenRouter",
		Description:   "Unified API for 100+ models from multiple providers",
		Website:       "https://openrouter.ai",
		DefaultModels: openrouter.DefaultModels,
		Categories:    []string{"aggregator"},
	},
	{
		Name:          "deepinfra",
		DisplayName:   "DeepInfra",
		Description:   "Scalable inference for open-source models",
		Website:       "https://deepinfra.com",
		DefaultModels: deepinfra.DefaultModels,
		Categories:    []string{"aggregator", "open-source"},
	},
	{
		Name:          "huggingface",
		DisplayName:   "Hugging Face",
		Description:   "Inference API for hosted models",
		Website:       "https://huggingface.co",
		DefaultModels: huggingface.DefaultModels,
		Categories:    []string{"aggregator", "open-source"},
	},

	// Tier 4: Specialized Providers
	{
		Name:          "deepseek",
		DisplayName:   "DeepSeek",
		Description:   "DeepSeek-Coder, DeepSeek-Chat, and DeepSeek-Reasoner",
		Website:       "https://deepseek.com",
		DefaultModels: deepseek.DefaultModels,
		Categories:    []string{"coding", "reasoning"},
	},
	{
		Name:          "perplexity",
		DisplayName:   "Perplexity AI",
		Description:   "AI-powered search with Sonar models",
		Website:       "https://perplexity.ai",
		DefaultModels: perplexity.DefaultModels,
		Categories:    []string{"search"},
	},
	{
		Name:          "xai",
		DisplayName:   "xAI (Grok)",
		Description:   "Grok models from xAI",
		Website:       "https://x.ai",
		DefaultModels: xai.DefaultModels,
		Categories:    []string{"commercial"},
	},

	// Tier 5: Chinese Providers
	{
		Name:          "qwen",
		DisplayName:   "Alibaba Qwen",
		Description:   "Qwen series from Alibaba Cloud",
		Website:       "https://qwenlm.github.io",
		DefaultModels: qwen.DefaultModels,
		Categories:    []string{"chinese", "open-weights"},
	},
	{
		Name:          "zhipu",
		DisplayName:   "Zhipu AI (ChatGLM)",
		Description:   "GLM series with strong Chinese language support",
		Website:       "https://www.zhipuai.cn",
		DefaultModels: zhipu.DefaultModels,
		Categories:    []string{"chinese"},
	},
	{
		Name:          "moonshot",
		DisplayName:   "Moonshot AI (Kimi)",
		Description:   "Long-context models supporting 128K+ tokens",
		Website:       "https://www.moonshot.cn",
		DefaultModels: moonshot.DefaultModels,
		Categories:    []string{"chinese", "long-context"},
	},
	{
		Name:          "volcengine",
		DisplayName:   "Volcengine (DouBao)",
		Description:   "ByteDance's Doubao models",
		Website:       "https://www.volcengine.com",
		DefaultModels: volcengine.DefaultModels,
		Categories:    []string{"chinese"},
	},
	{
		Name:          "siliconflow",
		DisplayName:   "SiliconFlow",
		Description:   "Affordable access to Qwen, DeepSeek, and more",
		Website:       "https://siliconflow.cn",
		DefaultModels: siliconflow.DefaultModels,
		Categories:    []string{"chinese", "aggregator"},
	},

	// Tier 6: Local/Self-hosted
	{
		Name:          "ollama",
		DisplayName:   "Ollama",
		Description:   "Local LLM inference with OpenAI-compatible API",
		Website:       "https://ollama.ai",
		DefaultModels: ollama.DefaultModels,
		Categories:    []string{"local", "self-hosted"},
	},
	{
		Name:          "lmstudio",
		DisplayName:   "LM Studio",
		Description:   "Desktop app for running local LLMs",
		Website:       "https://lmstudio.ai",
		DefaultModels: []string{},
		Categories:    []string{"local", "self-hosted"},
	},
	{
		Name:          "vllm",
		DisplayName:   "vLLM",
		Description:   "High-throughput LLM serving engine",
		Website:       "https://docs.vllm.ai",
		DefaultModels: []string{},
		Categories:    []string{"self-hosted"},
	},
}

// RegisterAllProviders registers all provider factories with the given registry.
func RegisterAllProviders(registry *provider.Registry) {
	for name, factory := range ProviderFactories {
		registry.RegisterFactory(name, factory)
	}
}

// GetProviderInfo returns information about a specific provider.
func GetProviderInfo(name string) *ProviderInfo {
	for _, info := range AllProviders {
		if info.Name == name {
			return &info
		}
	}
	return nil
}

// GetProvidersByCategory returns all providers in a category.
func GetProvidersByCategory(category string) []ProviderInfo {
	var result []ProviderInfo
	for _, info := range AllProviders {
		for _, cat := range info.Categories {
			if cat == category {
				result = append(result, info)
				break
			}
		}
	}
	return result
}

// ProviderCount returns the total number of supported providers.
func ProviderCount() int {
	return len(ProviderFactories)
}
