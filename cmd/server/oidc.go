package main

import (
	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
)

func mapOIDCConfig(cfg config.OIDCConfig) auth.OIDCConfig {
	return auth.OIDCConfig{
		IssuerURL:              cfg.IssuerURL,
		ClientID:               cfg.ClientID,
		ClientSecret:           cfg.ClientSecret,
		RedirectURL:            cfg.RedirectURL,
		RoleClaim:              cfg.ClaimMapping.RoleClaim,
		RolesMap:               cfg.ClaimMapping.Roles,
		UseRoleHierarchy:       cfg.ClaimMapping.UseRoleHierarchy,
		TeamIDJWTField:         cfg.ClaimMapping.TeamIDJWTField,
		TeamIDsJWTField:        cfg.ClaimMapping.TeamIDsJWTField,
		TeamAliasMap:           cfg.ClaimMapping.TeamAliasMap,
		OrgIDJWTField:          cfg.ClaimMapping.OrgIDJWTField,
		OrgAliasMap:            cfg.ClaimMapping.OrgAliasMap,
		UserIDJWTField:         cfg.ClaimMapping.UserIDJWTField,
		UserEmailJWTField:      cfg.ClaimMapping.UserEmailJWTField,
		EndUserIDJWTField:      cfg.ClaimMapping.EndUserIDJWTField,
		DefaultRole:            cfg.ClaimMapping.DefaultRole,
		DefaultTeamID:          cfg.ClaimMapping.DefaultTeamID,
		UserIDUpsert:           cfg.UserIDUpsert,
		TeamIDUpsert:           cfg.TeamIDUpsert,
		UserAllowedEmailDomain: cfg.UserAllowedEmailDomain,
		UserInfoEnabled:        cfg.OIDCUserInfoEnabled,
		UserInfoCacheTTL:       int(cfg.OIDCUserInfoCacheTTL),
	}
}
