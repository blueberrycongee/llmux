// Package auth provides API key authentication and multi-tenant support.
package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// OIDCConfig contains configuration for OIDC authentication.
// This structure aligns with LiteLLM's advanced SSO features.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string

	// Role mapping
	RoleClaim        string            // JWT claim for role extraction (e.g. "groups", "roles")
	RolesMap         map[string]string // Claim value to role mapping
	UseRoleHierarchy bool              // Enable hierarchical role priority

	// Team mapping
	TeamIDJWTField  string            // Single team ID field
	TeamIDsJWTField string            // Multiple team IDs field
	TeamAliasMap    map[string]string // JWT team alias to internal team ID

	// Organization mapping
	OrgIDJWTField string            // Organization ID field
	OrgAliasMap   map[string]string // JWT org alias to internal org ID

	// User ID mapping
	UserIDJWTField    string // Custom user ID field (default: "sub")
	UserEmailJWTField string // Email field (default: "email")

	// User provisioning
	UserIDUpsert           bool   // Auto-create users on SSO login
	TeamIDUpsert           bool   // Auto-create teams on SSO login
	UserAllowedEmailDomain string // Restrict to specific email domain

	// End user tracking
	EndUserIDJWTField string // End user ID field for tracking

	// Default values
	DefaultRole   string // Default role if no mapping matches
	DefaultTeamID string // Default team if no team claim found

	// UserInfo endpoint configuration (LiteLLM compatibility)
	UserInfoEnabled  bool   // Enable fetching additional info from UserInfo endpoint
	UserInfoCacheTTL int    // Cache TTL in seconds (default: 300)
	UserInfoURL      string // Override UserInfo URL (auto-discovered from issuer if empty)
}

// roleHierarchy defines the priority order of roles (highest to lowest).
// When a user matches multiple roles, the highest priority role is assigned.
var roleHierarchy = []UserRole{
	UserRoleProxyAdmin,
	UserRoleProxyAdminViewer, // Added for LiteLLM compatibility
	UserRoleOrgAdmin,
	UserRoleInternalUser,
	UserRoleInternalUserViewer, // Added for LiteLLM compatibility
	UserRoleTeam,
}

// roleHierarchyIndex returns the priority index of a role (lower = higher priority).
func roleHierarchyIndex(role string) int {
	for i, r := range roleHierarchy {
		if string(r) == role {
			return i
		}
	}
	return len(roleHierarchy) // Unknown roles have lowest priority
}

// OIDCMiddleware creates a new OIDC authentication middleware.
// It supports LiteLLM-compatible features including:
// - Hierarchical role priority
// - Team and organization extraction from JWT claims
// - Email domain restriction
// - Custom claim mapping
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
			idToken, err := verifier.Verify(r.Context(), rawToken)
			if err != nil {
				// Not a valid OIDC token, pass to next handler (might be API Key)
				next.ServeHTTP(w, r)
				return
			}

			// Extract raw claims for flexible field access
			var rawClaims map[string]interface{}
			if err := idToken.Claims(&rawClaims); err != nil {
				http.Error(w, "failed to parse claims", http.StatusInternalServerError)
				return
			}

			// Extract user info
			userInfo := extractUserInfo(cfg, rawClaims, idToken.Subject)

			// Validate email domain if restriction is configured
			if cfg.UserAllowedEmailDomain != "" && userInfo.Email != "" {
				if !strings.HasSuffix(userInfo.Email, "@"+cfg.UserAllowedEmailDomain) {
					http.Error(w, fmt.Sprintf("email domain not allowed: expected @%s", cfg.UserAllowedEmailDomain), http.StatusForbidden)
					return
				}
			}

			// Extract role using hierarchical priority if enabled
			role := extractRole(cfg, rawClaims)

			// Extract team IDs
			teamIDs := extractTeamIDs(cfg, rawClaims)

			// Extract organization ID
			orgID := extractOrgID(cfg, rawClaims)

			// Extract end user ID (for downstream tracking)
			endUserID := extractStringClaim(rawClaims, cfg.EndUserIDJWTField)

			// Build User object
			user := &User{
				ID:             userInfo.UserID,
				Email:          strPtr(userInfo.Email),
				Role:           role,
				Teams:          teamIDs,
				OrganizationID: strPtr(orgID),
			}

			// Set team ID if available (first team for single-team scenarios)
			if len(teamIDs) > 0 {
				user.TeamID = strPtr(teamIDs[0])
			} else if cfg.DefaultTeamID != "" {
				user.TeamID = strPtr(cfg.DefaultTeamID)
			}

			// Create AuthContext
			authCtx := &AuthContext{
				User:      user,
				UserRole:  UserRole(role),
				EndUserID: endUserID,
			}

			ctx := context.WithValue(r.Context(), AuthContextKey, authCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

// userInfo holds extracted user information from JWT claims.
type userInfo struct {
	UserID string
	Email  string
}

// extractUserInfo extracts user ID and email from JWT claims.
func extractUserInfo(cfg OIDCConfig, claims map[string]interface{}, subject string) userInfo {
	info := userInfo{
		UserID: subject, // Default to JWT subject
	}

	// Custom user ID field
	if cfg.UserIDJWTField != "" {
		if uid := extractStringClaim(claims, cfg.UserIDJWTField); uid != "" {
			info.UserID = uid
		}
	}

	// Email field
	emailField := cfg.UserEmailJWTField
	if emailField == "" {
		emailField = "email"
	}
	info.Email = extractStringClaim(claims, emailField)

	// Fallback: use email as user ID if ID is empty
	if info.UserID == "" && info.Email != "" {
		info.UserID = info.Email
	}

	return info
}

// extractRole determines the user's role from JWT claims.
// If UseRoleHierarchy is enabled, returns the highest priority matching role.
func extractRole(cfg OIDCConfig, claims map[string]interface{}) string {
	// Default role
	defaultRole := cfg.DefaultRole
	if defaultRole == "" {
		defaultRole = string(UserRoleInternalUser)
	}

	// Determine which claim to check
	roleClaim := cfg.RoleClaim
	if roleClaim == "" {
		roleClaim = "groups"
	}

	val, ok := claims[roleClaim]
	if !ok {
		return defaultRole
	}

	// Collect all matching roles
	var matchedRoles []string

	switch v := val.(type) {
	case string:
		if role, found := cfg.RolesMap[v]; found {
			matchedRoles = append(matchedRoles, role)
		}
	case []interface{}:
		for _, g := range v {
			if groupStr, ok := g.(string); ok {
				if role, found := cfg.RolesMap[groupStr]; found {
					matchedRoles = append(matchedRoles, role)
				}
			}
		}
	}

	if len(matchedRoles) == 0 {
		return defaultRole
	}

	// If hierarchy is enabled, return highest priority role
	if cfg.UseRoleHierarchy && len(matchedRoles) > 1 {
		return getHighestPriorityRole(matchedRoles)
	}

	// Otherwise, first match wins
	return matchedRoles[0]
}

// getHighestPriorityRole returns the role with the highest priority.
func getHighestPriorityRole(roles []string) string {
	if len(roles) == 0 {
		return string(UserRoleInternalUser)
	}

	highest := roles[0]
	highestIndex := roleHierarchyIndex(highest)

	for _, role := range roles[1:] {
		idx := roleHierarchyIndex(role)
		if idx < highestIndex {
			highest = role
			highestIndex = idx
		}
	}

	return highest
}

// extractTeamIDs extracts team IDs from JWT claims.
// Supports both single team ID and multiple team IDs fields.
// Also supports alias mapping.
func extractTeamIDs(cfg OIDCConfig, claims map[string]interface{}) []string {
	var teamIDs []string

	// Extract from single team field
	if cfg.TeamIDJWTField != "" {
		if teamID := extractStringClaim(claims, cfg.TeamIDJWTField); teamID != "" {
			// Check if alias mapping exists
			if mappedID, ok := cfg.TeamAliasMap[teamID]; ok {
				teamIDs = append(teamIDs, mappedID)
			} else {
				teamIDs = append(teamIDs, teamID)
			}
		}
	}

	// Extract from multiple teams field
	if cfg.TeamIDsJWTField != "" {
		if val, ok := claims[cfg.TeamIDsJWTField]; ok {
			switch v := val.(type) {
			case []interface{}:
				for _, t := range v {
					if tStr, ok := t.(string); ok {
						// Check alias mapping
						if mappedID, found := cfg.TeamAliasMap[tStr]; found {
							teamIDs = append(teamIDs, mappedID)
						} else {
							teamIDs = append(teamIDs, tStr)
						}
					}
				}
			case string:
				// Single value in array field
				if mappedID, found := cfg.TeamAliasMap[v]; found {
					teamIDs = append(teamIDs, mappedID)
				} else {
					teamIDs = append(teamIDs, v)
				}
			}
		}
	}

	return teamIDs
}

// extractOrgID extracts organization ID from JWT claims.
// Supports alias mapping.
func extractOrgID(cfg OIDCConfig, claims map[string]interface{}) string {
	if cfg.OrgIDJWTField == "" {
		return ""
	}

	orgID := extractStringClaim(claims, cfg.OrgIDJWTField)
	if orgID == "" {
		return ""
	}

	// Check alias mapping
	if mappedID, ok := cfg.OrgAliasMap[orgID]; ok {
		return mappedID
	}

	return orgID
}

// extractStringClaim safely extracts a string value from claims.
func extractStringClaim(claims map[string]interface{}, field string) string {
	if field == "" {
		return ""
	}
	if val, ok := claims[field]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// strPtr returns a pointer to the string, or nil if empty.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// OIDCMiddlewareWithSync creates a new OIDC authentication middleware with user-team synchronization.
// This middleware extends OIDCMiddleware by invoking UserTeamSyncer.SyncUserTeams after successful
// JWT verification. If syncer is nil, synchronization is skipped (equivalent to OIDCMiddleware).
//
// This function addresses the critical integration gap where SSO sync logic existed but was never invoked.
func OIDCMiddlewareWithSync(cfg OIDCConfig, syncer *UserTeamSyncer) (func(http.Handler) http.Handler, error) {
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
			idToken, err := verifier.Verify(r.Context(), rawToken)
			if err != nil {
				// Not a valid OIDC token, pass to next handler (might be API Key)
				next.ServeHTTP(w, r)
				return
			}

			// Extract raw claims for flexible field access
			var rawClaims map[string]interface{}
			if err := idToken.Claims(&rawClaims); err != nil {
				http.Error(w, "failed to parse claims", http.StatusInternalServerError)
				return
			}

			// Extract user info
			userInfo := extractUserInfo(cfg, rawClaims, idToken.Subject)

			// Validate email domain if restriction is configured
			if cfg.UserAllowedEmailDomain != "" && userInfo.Email != "" {
				if !strings.HasSuffix(userInfo.Email, "@"+cfg.UserAllowedEmailDomain) {
					http.Error(w, fmt.Sprintf("email domain not allowed: expected @%s", cfg.UserAllowedEmailDomain), http.StatusForbidden)
					return
				}
			}

			// Extract role using hierarchical priority if enabled
			role := extractRole(cfg, rawClaims)

			// Extract team IDs
			teamIDs := extractTeamIDs(cfg, rawClaims)

			// Extract organization ID
			orgID := extractOrgID(cfg, rawClaims)

			// Extract end user ID (for downstream tracking)
			endUserID := extractStringClaim(rawClaims, cfg.EndUserIDJWTField)

			// Build User object
			user := &User{
				ID:             userInfo.UserID,
				Email:          strPtr(userInfo.Email),
				Role:           role,
				Teams:          teamIDs,
				OrganizationID: strPtr(orgID),
			}

			// Set team ID if available (first team for single-team scenarios)
			if len(teamIDs) > 0 {
				user.TeamID = strPtr(teamIDs[0])
			} else if cfg.DefaultTeamID != "" {
				user.TeamID = strPtr(cfg.DefaultTeamID)
			}

			// === SSO SYNC INTEGRATION (P0 Fix) ===
			// Invoke UserTeamSyncer to synchronize user roles and team memberships
			if syncer != nil {
				syncReq := &SyncRequest{
					UserID:         userInfo.UserID,
					Email:          strPtr(userInfo.Email),
					SSOUserID:      idToken.Subject,
					Role:           role,
					TeamIDs:        teamIDs,
					OrganizationID: strPtr(orgID),
				}

				// Run sync in the request context (non-blocking for the main flow)
				// Note: We ignore sync errors to avoid blocking authentication for sync issues
				// Sync warnings are logged internally by the syncer
				_, _ = syncer.SyncUserTeams(r.Context(), syncReq)
			}

			// Create AuthContext
			authCtx := &AuthContext{
				User:      user,
				UserRole:  UserRole(role),
				EndUserID: endUserID,
			}

			ctx := context.WithValue(r.Context(), AuthContextKey, authCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}
