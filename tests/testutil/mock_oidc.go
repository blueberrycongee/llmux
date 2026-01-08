package testutil

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// MockOIDCProvider simulates an OIDC provider for testing.
type MockOIDCProvider struct {
	server     *httptest.Server
	privateKey *rsa.PrivateKey
	signer     jose.Signer
	issuer     string
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
