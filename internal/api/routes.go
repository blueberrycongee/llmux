// Package api provides HTTP handlers for the LLM gateway API.
// Route registration for all API endpoints.
package api //nolint:revive // package name is intentional

import (
	"net/http"
)

// RegisterRoutes registers all API routes on the given mux.
func (h *ManagementHandler) RegisterRoutes(mux *http.ServeMux) {
	// ========================================================================
	// Key Management Routes
	// ========================================================================
	mux.HandleFunc("POST /key/generate", h.GenerateKey)
	mux.HandleFunc("POST /key/update", h.UpdateKey)
	mux.HandleFunc("POST /key/delete", h.DeleteKey)
	mux.HandleFunc("GET /key/info", h.GetKeyInfo)
	mux.HandleFunc("GET /key/list", h.ListKeys)
	mux.HandleFunc("POST /key/block", h.BlockKey)
	mux.HandleFunc("POST /key/unblock", h.UnblockKey)
	mux.HandleFunc("POST /key/regenerate", h.RegenerateKey)

	// ========================================================================
	// Team Management Routes
	// ========================================================================
	mux.HandleFunc("POST /team/new", h.NewTeam)
	mux.HandleFunc("POST /team/update", h.UpdateTeam)
	mux.HandleFunc("POST /team/delete", h.DeleteTeam)
	mux.HandleFunc("GET /team/info", h.GetTeamInfo)
	mux.HandleFunc("GET /team/list", h.ListTeams)
	mux.HandleFunc("POST /team/block", h.BlockTeam)
	mux.HandleFunc("POST /team/unblock", h.UnblockTeam)
	mux.HandleFunc("POST /team/member_add", h.AddTeamMember)
	mux.HandleFunc("POST /team/member_delete", h.DeleteTeamMember)

	// ========================================================================
	// User Management Routes
	// ========================================================================
	mux.HandleFunc("POST /user/new", h.NewUser)
	mux.HandleFunc("POST /user/update", h.UpdateUser)
	mux.HandleFunc("POST /user/delete", h.DeleteUser)
	mux.HandleFunc("GET /user/info", h.GetUserInfo)
	mux.HandleFunc("GET /user/list", h.ListUsers)

	// ========================================================================
	// Organization Management Routes
	// ========================================================================
	mux.HandleFunc("POST /organization/new", h.NewOrganization)
	mux.HandleFunc("PATCH /organization/update", h.UpdateOrganization)
	mux.HandleFunc("DELETE /organization/delete", h.DeleteOrganization)
	mux.HandleFunc("GET /organization/info", h.GetOrganizationInfo)
	mux.HandleFunc("GET /organization/list", h.ListOrganizations)
	mux.HandleFunc("POST /organization/member_add", h.AddOrganizationMember)
	mux.HandleFunc("POST /organization/member_update", h.UpdateOrganizationMember)
	mux.HandleFunc("POST /organization/member_delete", h.DeleteOrganizationMember)
	mux.HandleFunc("GET /organization/members", h.ListOrganizationMembers)

	// ========================================================================
	// Spend Tracking Routes
	// ========================================================================
	mux.HandleFunc("GET /spend/logs", h.GetSpendLogs)
	mux.HandleFunc("GET /spend/keys", h.GetSpendByKeys)
	mux.HandleFunc("GET /spend/teams", h.GetSpendByTeams)
	mux.HandleFunc("GET /spend/users", h.GetSpendByUsers)

	// ========================================================================
	// Global Analytics Routes
	// ========================================================================
	mux.HandleFunc("GET /global/activity", h.GetGlobalActivity)
	mux.HandleFunc("GET /global/spend/models", h.GetGlobalSpendByModel)
	mux.HandleFunc("GET /global/spend/provider", h.GetGlobalSpendByProvider)
}

// RouteInfo describes an API route.
type RouteInfo struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// GetRoutes returns information about all registered routes.
func GetRoutes() []RouteInfo {
	return []RouteInfo{
		// Key Management
		{Method: "POST", Path: "/key/generate", Description: "Generate a new API key", Category: "key"},
		{Method: "POST", Path: "/key/update", Description: "Update an existing API key", Category: "key"},
		{Method: "POST", Path: "/key/delete", Description: "Delete API keys", Category: "key"},
		{Method: "GET", Path: "/key/info", Description: "Get API key information", Category: "key"},
		{Method: "GET", Path: "/key/list", Description: "List API keys", Category: "key"},
		{Method: "POST", Path: "/key/block", Description: "Block an API key", Category: "key"},
		{Method: "POST", Path: "/key/unblock", Description: "Unblock an API key", Category: "key"},
		{Method: "POST", Path: "/key/regenerate", Description: "Regenerate an API key", Category: "key"},

		// Team Management
		{Method: "POST", Path: "/team/new", Description: "Create a new team", Category: "team"},
		{Method: "POST", Path: "/team/update", Description: "Update a team", Category: "team"},
		{Method: "POST", Path: "/team/delete", Description: "Delete teams", Category: "team"},
		{Method: "GET", Path: "/team/info", Description: "Get team information", Category: "team"},
		{Method: "GET", Path: "/team/list", Description: "List teams", Category: "team"},
		{Method: "POST", Path: "/team/block", Description: "Block a team", Category: "team"},
		{Method: "POST", Path: "/team/unblock", Description: "Unblock a team", Category: "team"},
		{Method: "POST", Path: "/team/member_add", Description: "Add members to a team", Category: "team"},
		{Method: "POST", Path: "/team/member_delete", Description: "Remove members from a team", Category: "team"},

		// User Management
		{Method: "POST", Path: "/user/new", Description: "Create a new user", Category: "user"},
		{Method: "POST", Path: "/user/update", Description: "Update a user", Category: "user"},
		{Method: "POST", Path: "/user/delete", Description: "Delete users", Category: "user"},
		{Method: "GET", Path: "/user/info", Description: "Get user information", Category: "user"},
		{Method: "GET", Path: "/user/list", Description: "List users", Category: "user"},

		// Organization Management
		{Method: "POST", Path: "/organization/new", Description: "Create a new organization", Category: "organization"},
		{Method: "PATCH", Path: "/organization/update", Description: "Update an organization", Category: "organization"},
		{Method: "DELETE", Path: "/organization/delete", Description: "Delete organizations", Category: "organization"},
		{Method: "GET", Path: "/organization/info", Description: "Get organization information", Category: "organization"},
		{Method: "GET", Path: "/organization/list", Description: "List organizations", Category: "organization"},
		{Method: "POST", Path: "/organization/member_add", Description: "Add members to an organization", Category: "organization"},
		{Method: "POST", Path: "/organization/member_update", Description: "Update an organization member", Category: "organization"},
		{Method: "POST", Path: "/organization/member_delete", Description: "Remove members from an organization", Category: "organization"},
		{Method: "GET", Path: "/organization/members", Description: "List organization members", Category: "organization"},

		// Spend Tracking
		{Method: "GET", Path: "/spend/logs", Description: "Get spend logs", Category: "spend"},
		{Method: "GET", Path: "/spend/keys", Description: "Get spend by API keys", Category: "spend"},
		{Method: "GET", Path: "/spend/teams", Description: "Get spend by teams", Category: "spend"},
		{Method: "GET", Path: "/spend/users", Description: "Get spend by users", Category: "spend"},

		// Global Analytics
		{Method: "GET", Path: "/global/activity", Description: "Get global activity metrics", Category: "analytics"},
		{Method: "GET", Path: "/global/spend/models", Description: "Get spend by model", Category: "analytics"},
		{Method: "GET", Path: "/global/spend/provider", Description: "Get spend by provider", Category: "analytics"},

		// Audit Logs
		{Method: "GET", Path: "/audit/logs", Description: "List audit logs", Category: "audit"},
		{Method: "GET", Path: "/audit/log", Description: "Get audit log by ID", Category: "audit"},
		{Method: "GET", Path: "/audit/stats", Description: "Get audit log statistics", Category: "audit"},
		{Method: "POST", Path: "/audit/delete", Description: "Delete old audit logs", Category: "audit"},
	}
}
