package azure

import "github.com/blueberrycongee/llmux/pkg/provider"

type Option func(*Provider)

func WithAPIKey(key string) Option {
	return func(p *Provider) { p.apiKey = key }
}

func WithTokenSource(ts provider.TokenSource) Option {
	return func(p *Provider) { p.tokenSource = ts }
}

func WithBaseURL(url string) Option {
	return func(p *Provider) {
		if url != "" {
			p.baseURL = url
		}
	}
}

func WithModels(models ...string) Option {
	return func(p *Provider) { p.models = models }
}

func WithAPIVersion(version string) Option {
	return func(p *Provider) {
		if version != "" {
			p.apiVersion = version
		}
	}
}

func WithHeader(key, value string) Option {
	return func(p *Provider) { p.headers[key] = value }
}
