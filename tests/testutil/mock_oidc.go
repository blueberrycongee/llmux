package testutil

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// MockOIDCProvider simulates an OIDC provider for testing.
type MockOIDCProvider struct {
	server              *httptest.Server
	privateKey          *rsa.PrivateKey
	signer              jose.Signer
	issuer              string
	tokenClaims         map[string]interface{}
	requireCodeVerifier bool
	mu                  sync.RWMutex
}

// NewMockOIDCProvider creates a new mock OIDC provider.
func NewMockOIDCProvider() (*MockOIDCProvider, error) {
	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create signer
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: privateKey}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}

	p := &MockOIDCProvider{
		privateKey: privateKey,
		signer:     signer,
	}

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", p.handleDiscovery)
	mux.HandleFunc("/keys", p.handleJWKS)
	mux.HandleFunc("/token", p.handleToken)

	p.server = httptest.NewServer(mux)
	p.issuer = p.server.URL

	return p, nil
}

// URL returns the provider's base URL.
func (p *MockOIDCProvider) URL() string {
	return p.server.URL
}

// Close shuts down the provider.
func (p *MockOIDCProvider) Close() {
	p.server.Close()
}

// SetTokenClaims configures the claims returned from the token endpoint.
func (p *MockOIDCProvider) SetTokenClaims(claims map[string]interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tokenClaims = claims
}

// RequireCodeVerifier enforces PKCE validation in the token endpoint.
func (p *MockOIDCProvider) RequireCodeVerifier() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requireCodeVerifier = true
}

// SignToken creates a signed JWT with the given claims.
func (p *MockOIDCProvider) SignToken(claims map[string]interface{}) (string, error) {
	// Add standard claims if not present
	if _, ok := claims["iss"]; !ok {
		claims["iss"] = p.issuer
	}
	if _, ok := claims["exp"]; !ok {
		claims["exp"] = time.Now().Add(time.Hour).Unix()
	}
	if _, ok := claims["iat"]; !ok {
		claims["iat"] = time.Now().Unix()
	}
	if _, ok := claims["aud"]; !ok {
		claims["aud"] = "llmux-client-id"
	}

	return jwt.Signed(p.signer).Claims(claims).Serialize()
}

func (p *MockOIDCProvider) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	config := map[string]interface{}{
		"issuer":                 p.issuer,
		"authorization_endpoint": p.issuer + "/auth",
		"token_endpoint":         p.issuer + "/token",
		"jwks_uri":               p.issuer + "/keys",
		"response_types_supported": []string{
			"code",
			"token",
			"id_token",
		},
		"subject_types_supported": []string{
			"public",
		},
		"id_token_signing_alg_values_supported": []string{
			"RS256",
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func (p *MockOIDCProvider) handleJWKS(w http.ResponseWriter, r *http.Request) {
	jwk := jose.JSONWebKey{
		Key:       &p.privateKey.PublicKey,
		KeyID:     "test-key",
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}

	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{jwk},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jwks)
}

func (p *MockOIDCProvider) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	p.mu.RLock()
	requireVerifier := p.requireCodeVerifier
	p.mu.RUnlock()
	if requireVerifier && r.FormValue("code_verifier") == "" {
		http.Error(w, "missing code_verifier", http.StatusBadRequest)
		return
	}

	claims := map[string]interface{}{
		"sub":   "user-1",
		"email": "user@example.com",
	}
	p.mu.RLock()
	for k, v := range p.tokenClaims {
		claims[k] = v
	}
	p.mu.RUnlock()

	idToken, err := p.SignToken(claims)
	if err != nil {
		http.Error(w, "failed to sign token", http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"access_token": "access-token",
		"id_token":     idToken,
		"token_type":   "Bearer",
		"expires_in":   3600,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
