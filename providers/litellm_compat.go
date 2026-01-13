package providers

import (
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

type openAICompatDefaults struct {
	DefaultBaseURL    string
	SupportsStreaming bool
	SupportsEmbedding bool
	APIKeyHeader      string
	APIKeyPrefix      string
	ChatEndpoint      string
	EmbeddingEndpoint string
	ExtraHeaders      map[string]string
	ModelPrefixes     []string
}

func openaiLikeFactory(cfg provider.Config) (provider.Provider, error) {
	info := openailike.Info{
		Name:              "openai_like",
		DefaultBaseURL:    "",
		SupportsStreaming: true,
		SupportsEmbedding: true,
	}
	return openailike.NewFromConfig(info, cfg)
}

func registerLiteLLMOpenAICompat() {
	for providerName, defaults := range litellmOpenAICompatDefaults {
		if _, ok := Get(providerName); ok {
			continue
		}

		name := providerName
		defaults := defaults
		if !defaults.SupportsStreaming {
			defaults.SupportsStreaming = true
		}
		if !defaults.SupportsEmbedding {
			defaults.SupportsEmbedding = true
		}

		Register(name, func(cfg provider.Config) (provider.Provider, error) {
			info := openailike.Info{
				Name:              name,
				DefaultBaseURL:    defaults.DefaultBaseURL,
				APIKeyHeader:      defaults.APIKeyHeader,
				APIKeyPrefix:      defaults.APIKeyPrefix,
				ChatEndpoint:      defaults.ChatEndpoint,
				EmbeddingEndpoint: defaults.EmbeddingEndpoint,
				SupportsStreaming: defaults.SupportsStreaming,
				SupportsEmbedding: defaults.SupportsEmbedding,
				ExtraHeaders:      defaults.ExtraHeaders,
				ModelPrefixes:     defaults.ModelPrefixes,
			}
			return openailike.NewFromConfig(info, cfg)
		})
	}
}

var litellmOpenAICompatDefaults = map[string]openAICompatDefaults{
	// OpenAI-compatible providers that can be handled by the shared openailike adapter.
	// DefaultBaseURL values are best-effort and are always overrideable via config.
	"aiml":           {DefaultBaseURL: "https://api.aimlapi.com/v1"},
	"aiohttp_openai": {DefaultBaseURL: "https://api.openai.com/v1"},
	"amazon_nova":    {DefaultBaseURL: "https://api.nova.amazon.com/v1"},
	"azure_ai":       {DefaultBaseURL: ""},
	"baseten":        {DefaultBaseURL: ""},
	"bytez":          {DefaultBaseURL: "https://api.bytez.com/models/v2"},
	"clarifai":       {DefaultBaseURL: "https://api.clarifai.com/v2/ext/openai/v1"},
	"codestral":      {DefaultBaseURL: "https://codestral.mistral.ai/v1"},
	"cometapi":       {DefaultBaseURL: "https://api.cometapi.com/v1"},
	"compactifai":    {DefaultBaseURL: "https://api.compactif.ai/v1"},
	"dashscope":      {DefaultBaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1"},
	"datarobot":      {DefaultBaseURL: ""},
	"empower":        {DefaultBaseURL: ""},
	"friendliai":     {DefaultBaseURL: ""},
	"galadriel":      {DefaultBaseURL: ""},
	"gigachat":       {DefaultBaseURL: ""},
	"github_copilot": {DefaultBaseURL: "https://api.githubcopilot.com"},
	"gradient_ai":    {DefaultBaseURL: "https://inference.do-ai.run"},
	"infinity":       {DefaultBaseURL: ""},
	"jina_ai":        {DefaultBaseURL: "https://api.jina.ai/v1"},
	"lemonade":       {DefaultBaseURL: "http://localhost:8000"},
	"llamafile":      {DefaultBaseURL: "http://127.0.0.1:8080/v1"},
	"meta_llama":     {DefaultBaseURL: "https://api.llama.com/v1"},
	"morph":          {DefaultBaseURL: ""},
	"nebius":         {DefaultBaseURL: ""},
	"nlp_cloud":      {DefaultBaseURL: ""},
	"nscale":         {DefaultBaseURL: "https://inference.api.nscale.com/v1"},
	"oobabooga":      {DefaultBaseURL: ""},
	"ovhcloud":       {DefaultBaseURL: "https://oai.endpoints.kepler.ai.cloud.ovh.net/v1"},
	"parallel_ai":    {DefaultBaseURL: "https://api.parallel.ai"},
	"predibase":      {DefaultBaseURL: ""},
	"ragflow":        {DefaultBaseURL: "http://localhost:9380"},
	"sagemaker":      {DefaultBaseURL: ""},
	"sap":            {DefaultBaseURL: ""},
	"triton":         {DefaultBaseURL: ""},
	"voyage":         {DefaultBaseURL: "https://api.voyageai.com/v1"},
	"xinference":     {DefaultBaseURL: ""},
	"zai":            {DefaultBaseURL: "https://api.z.ai"},
}
