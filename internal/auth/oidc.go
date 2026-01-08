package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// OIDCConfig contains configuration for OIDC authentication.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RoleClaim    string
	RolesMap     map[string]string
}

// OIDCMiddleware creates a new OIDC authentication middleware.
func OIDCMiddleware(cfg OIDCConfig) (func(http.Handler) http.Handler, error) {
	provider, err := oidc.NewProvider(context.Background(), cfg.IssuerURL)
	if err != nil {
		return nil, err
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if already authenticated
			if GetAuthContext(r.Context()) != nil {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				next.ServeHTTP(w, r)
				return
			}

			rawToken := strings.TrimPrefix(authHeader, "Bearer ")

			// Verify Token
			// Note: This is a synchronous verification.
			idToken, err := verifier.Verify(r.Context(), rawToken)
			if err != nil {
				// Not a valid OIDC token, pass to next handler (might be API Key)
				next.ServeHTTP(w, r)
				return
			}

			// Extract Claims
			var claims struct {
				Email  string   `json:"email"`
				Groups []string `json:"groups"`
			}
			if err := idToken.Claims(&claims); err != nil {
				http.Error(w, "failed to parse claims", http.StatusInternalServerError)
				return
			}

			// Map to User
			user := &User{
				ID:    claims.Email, // Use email as ID for now
				Email: &claims.Email,
				Role:  string(UserRoleInternalUser),
			}

			// Dynamic Role Mapping
			user.Role = string(UserRoleInternalUser) // Default role

			// Re-decode claims into map to support arbitrary fields if needed,
			// or just use the struct if we know the claim is "groups".
			// For robustness, let's decode into map as well.
			var rawClaims map[string]interface{}
			if err := idToken.Claims(&rawClaims); err == nil {
				// Determine which claim to check
				targetClaim := cfg.RoleClaim
				if targetClaim == "" {
					targetClaim = "groups"
				}

				if val, ok := rawClaims[targetClaim]; ok {
					switch v := val.(type) {
					case string:
						if role, found := cfg.RolesMap[v]; found {
							user.Role = role
						}
					case []interface{}:
						for _, g := range v {
							if groupStr, ok := g.(string); ok {
								if role, found := cfg.RolesMap[groupStr]; found {
									user.Role = role
									break // First match wins
								}
							}
						}
					}
				}
			}

			// Create AuthContext
			authCtx := &AuthContext{
				User:     user,
				UserRole: UserRole(user.Role),
			}

			ctx := context.WithValue(r.Context(), AuthContextKey, authCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}
