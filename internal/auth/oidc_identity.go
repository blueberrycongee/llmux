// Package auth provides API key authentication and multi-tenant support.
package auth

import "strings"

// OIDCIdentity represents the resolved identity from OIDC claims.
type OIDCIdentity struct {
	UserID    string
	Email     string
	Role      UserRole
	TeamIDs   []string
	OrgID     string
	EndUserID string
	SSOUserID string
	User      *User
}

// ExtractOIDCIdentity maps raw claims into a normalized identity.
func ExtractOIDCIdentity(cfg OIDCConfig, claims map[string]interface{}, subject string) OIDCIdentity {
	userInfo := extractUserInfo(cfg, claims, subject)
	role := extractRole(cfg, claims)
	teamIDs := extractTeamIDs(cfg, claims)
	orgID := extractOrgID(cfg, claims)
	endUserID := extractStringClaim(claims, cfg.EndUserIDJWTField)

	user := &User{
		ID:             userInfo.UserID,
		Email:          strPtr(userInfo.Email),
		Role:           role,
		Teams:          teamIDs,
		OrganizationID: strPtr(orgID),
	}
	if len(teamIDs) > 0 {
		user.TeamID = strPtr(teamIDs[0])
	} else if cfg.DefaultTeamID != "" {
		user.TeamID = strPtr(cfg.DefaultTeamID)
	}

	return OIDCIdentity{
		UserID:    userInfo.UserID,
		Email:     userInfo.Email,
		Role:      UserRole(role),
		TeamIDs:   teamIDs,
		OrgID:     orgID,
		EndUserID: endUserID,
		SSOUserID: subject,
		User:      user,
	}
}

// IsEmailDomainAllowed checks if the email matches the configured domain.
func IsEmailDomainAllowed(cfg OIDCConfig, email string) bool {
	if cfg.UserAllowedEmailDomain == "" || email == "" {
		return true
	}
	domain := strings.ToLower(strings.TrimSpace(cfg.UserAllowedEmailDomain))
	candidate := strings.ToLower(email)
	return strings.HasSuffix(candidate, "@"+domain)
}
