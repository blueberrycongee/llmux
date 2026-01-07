package anthropic

// Option configures the Anthropic provider.
type Option func(*Provider)

// WithAPIKey sets the API key.
func WithAPIKey(key string) Option {
	return func(p *Provider) {
		p.apiKey = key
	}
}

// WithBaseURL sets the base URL.
func WithBaseURL(url string) Option {
	return func(p *Provider) {
		if url != "" {
			p.baseURL = url
		}
	}
}

// WithModels sets the supported models.
func WithModels(models ...string) Option {
	return func(p *Provider) {
		p.models = models
	}
}

// WithAPIVersion sets the Anthropic API version.
func WithAPIVersion(version string) Option {
	return func(p *Provider) {
		if version != "" {
			p.apiVersion = version
		}
	}
}

// WithHeader adds a custom header.
func WithHeader(key, value string) Option {
	return func(p *Provider) {
		p.headers[key] = value
	}
}
