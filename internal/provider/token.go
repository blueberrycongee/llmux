package provider

// TokenSource defines the interface for retrieving access tokens.
// It allows for dynamic token retrieval (e.g. OIDC, IAM) vs static API keys.
type TokenSource interface {
	// Token returns a valid access token or error.
	Token() (string, error)
}

// StaticTokenSource implements TokenSource with a static key.
type StaticTokenSource struct {
	v string
}

// NewStaticTokenSource creates a new static token source.
func NewStaticTokenSource(v string) *StaticTokenSource {
	return &StaticTokenSource{v: v}
}

// Token returns the static token.
func (s *StaticTokenSource) Token() (string, error) {
	return s.v, nil
}
