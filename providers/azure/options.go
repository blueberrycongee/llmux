package azure

type Option func(*Provider)

func WithAPIKey(key string) Option {
	return func(p *Provider) { p.apiKey = key }
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
