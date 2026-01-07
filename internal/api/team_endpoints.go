// Package api provides HTTP handlers for the LLM gateway API.
// Team management endpoints.
package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/internal/auth"
)

// ============================================================================
// Team Management Endpoints
// ============================================================================

// NewTeamRequest represents a request to create a new team.
type NewTeamRequest struct {
	TeamID          string             `json:"team_id,omitempty"`
	TeamAlias       *string            `json:"team_alias,omitempty"`
	OrganizationID  *string            `json:"organization_id,omitempty"`
	Members         []TeamMember       `json:"members_with_roles,omitempty"`
	Models          []string           `json:"models,omitempty"`
	MaxBudget       *float64           `json:"max_budget,omitempty"`
	BudgetDuration  string             `json:"budget_duration,omitempty"`
	TPMLimit        *int64             `json:"tpm_limit,omitempty"`
	RPMLimit        *int64             `json:"rpm_limit,omitempty"`
	MaxParallelReqs *int               `json:"max_parallel_requests,omitempty"`
	ModelMaxBudget  map[string]float64 `json:"model_max_budget,omitempty"`
	ModelTPMLimit   map[string]int64   `json:"model_tpm_limit,omitempty"`
	ModelRPMLimit   map[string]int64   `json:"model_rpm_limit,omitempty"`
	Metadata        auth.Metadata      `json:"metadata,omitempty"`
	Blocked         bool               `json:"blocked,omitempty"`
}

// TeamMember represents a team member with role.
type TeamMember struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"` // admin, user
}

// NewTeam handles POST /team/new
func (h *ManagementHandler) NewTeam(w http.ResponseWriter, r *http.Request) {
	var req NewTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	now := time.Now()
	teamID := req.TeamID
	if teamID == "" {
		teamID = auth.GenerateUUID()
	}

	team := &auth.Team{
		ID:                  teamID,
		Alias:               req.TeamAlias,
		OrganizationID:      req.OrganizationID,
		Models:              req.Models,
		TPMLimit:            req.TPMLimit,
		RPMLimit:            req.RPMLimit,
		MaxParallelRequests: req.MaxParallelReqs,
		ModelMaxBudget:      req.ModelMaxBudget,
		ModelTPMLimit:       req.ModelTPMLimit,
		ModelRPMLimit:       req.ModelRPMLimit,
		Metadata:            req.Metadata,
		IsActive:            true,
		Blocked:             req.Blocked,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if req.MaxBudget != nil {
		team.MaxBudget = *req.MaxBudget
	}

	if req.BudgetDuration != "" {
		team.BudgetDuration = auth.BudgetDuration(req.BudgetDuration)
		team.BudgetResetAt = team.BudgetDuration.NextResetTime()
	}

	// Extract member IDs
	members := make([]string, 0, len(req.Members))
	for _, m := range req.Members {
		members = append(members, m.UserID)
	}
	team.Members = members

	if err := h.store.CreateTeam(r.Context(), team); err != nil {
		h.logger.Error("failed to create team", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to create team")
		return
	}

	// Create team memberships
	for _, m := range req.Members {
		membership := &auth.TeamMembership{
			UserID: m.UserID,
			TeamID: teamID,
		}
		if err := h.store.CreateTeamMembership(r.Context(), membership); err != nil {
			h.logger.Warn("failed to create team membership", "user_id", m.UserID, "error", err)
		}
	}

	h.writeJSON(w, http.StatusOK, team)
}

// UpdateTeamRequest represents a request to update a team.
type UpdateTeamRequest struct {
	TeamID          string             `json:"team_id"`
	TeamAlias       *string            `json:"team_alias,omitempty"`
	Models          []string           `json:"models,omitempty"`
	MaxBudget       *float64           `json:"max_budget,omitempty"`
	BudgetDuration  *string            `json:"budget_duration,omitempty"`
	TPMLimit        *int64             `json:"tpm_limit,omitempty"`
	RPMLimit        *int64             `json:"rpm_limit,omitempty"`
	MaxParallelReqs *int               `json:"max_parallel_requests,omitempty"`
	ModelMaxBudget  map[string]float64 `json:"model_max_budget,omitempty"`
	Metadata        auth.Metadata      `json:"metadata,omitempty"`
	Blocked         *bool              `json:"blocked,omitempty"`
}

// UpdateTeam handles POST /team/update
func (h *ManagementHandler) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	var req UpdateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TeamID == "" {
		h.writeError(w, http.StatusBadRequest, "team_id is required")
		return
	}

	team, err := h.store.GetTeam(r.Context(), req.TeamID)
	if err != nil || team == nil {
		h.writeError(w, http.StatusNotFound, "team not found")
		return
	}

	// Update fields
	if req.TeamAlias != nil {
		team.Alias = req.TeamAlias
	}
	if req.Models != nil {
		team.Models = req.Models
	}
	if req.MaxBudget != nil {
		team.MaxBudget = *req.MaxBudget
	}
	if req.BudgetDuration != nil {
		team.BudgetDuration = auth.BudgetDuration(*req.BudgetDuration)
		team.BudgetResetAt = team.BudgetDuration.NextResetTime()
	}
	if req.TPMLimit != nil {
		team.TPMLimit = req.TPMLimit
	}
	if req.RPMLimit != nil {
		team.RPMLimit = req.RPMLimit
	}
	if req.MaxParallelReqs != nil {
		team.MaxParallelRequests = req.MaxParallelReqs
	}
	if req.ModelMaxBudget != nil {
		team.ModelMaxBudget = req.ModelMaxBudget
	}
	if req.Metadata != nil {
		team.Metadata = mergeMetadata(team.Metadata, req.Metadata)
	}
	if req.Blocked != nil {
		team.Blocked = *req.Blocked
	}

	team.UpdatedAt = time.Now()

	if err := h.store.UpdateTeam(r.Context(), team); err != nil {
		h.logger.Error("failed to update team", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to update team")
		return
	}

	h.writeJSON(w, http.StatusOK, team)
}

// DeleteTeamRequest represents a request to delete teams.
type DeleteTeamRequest struct {
	TeamIDs []string `json:"team_ids"`
}

// DeleteTeam handles POST /team/delete
func (h *ManagementHandler) DeleteTeam(w http.ResponseWriter, r *http.Request) {
	var req DeleteTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.TeamIDs) == 0 {
		h.writeError(w, http.StatusBadRequest, "team_ids is required")
		return
	}

	deleted := make([]string, 0, len(req.TeamIDs))
	for _, teamID := range req.TeamIDs {
		if err := h.store.DeleteTeam(r.Context(), teamID); err != nil {
			h.logger.Warn("failed to delete team", "team_id", teamID, "error", err)
			continue
		}
		deleted = append(deleted, teamID)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"deleted_teams": deleted,
	})
}

// GetTeamInfo handles GET /team/info
func (h *ManagementHandler) GetTeamInfo(w http.ResponseWriter, r *http.Request) {
	teamID := r.URL.Query().Get("team_id")
	if teamID == "" {
		h.writeError(w, http.StatusBadRequest, "team_id parameter is required")
		return
	}

	team, err := h.store.GetTeam(r.Context(), teamID)
	if err != nil {
		h.logger.Error("failed to get team info", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to get team info")
		return
	}
	if team == nil {
		h.writeError(w, http.StatusNotFound, "team not found")
		return
	}

	// Get team members
	members, err := h.store.ListTeamMembers(r.Context(), teamID)
	if err != nil {
		h.logger.Warn("failed to get team members", "error", err)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"team":    team,
		"members": members,
	})
}

// ListTeams handles GET /team/list
func (h *ManagementHandler) ListTeams(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("organization_id")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		limit = 50
	}
	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil || offset < 0 {
		offset = 0
	}

	filter := auth.TeamFilter{
		Limit:  limit,
		Offset: offset,
	}
	if orgID != "" {
		filter.OrganizationID = &orgID
	}

	teams, total, err := h.store.ListTeams(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list teams", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to list teams")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":  teams,
		"total": total,
	})
}

// BlockTeam handles POST /team/block
func (h *ManagementHandler) BlockTeam(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TeamID string `json:"team_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.store.BlockTeam(r.Context(), req.TeamID, true); err != nil {
		h.logger.Error("failed to block team", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to block team")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "blocked"})
}

// UnblockTeam handles POST /team/unblock
func (h *ManagementHandler) UnblockTeam(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TeamID string `json:"team_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.store.BlockTeam(r.Context(), req.TeamID, false); err != nil {
		h.logger.Error("failed to unblock team", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to unblock team")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "unblocked"})
}

// TeamMemberAddRequest represents a request to add members to a team.
type TeamMemberAddRequest struct {
	TeamID  string       `json:"team_id"`
	Members []TeamMember `json:"members"`
}

// AddTeamMember handles POST /team/member_add
func (h *ManagementHandler) AddTeamMember(w http.ResponseWriter, r *http.Request) {
	var req TeamMemberAddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TeamID == "" {
		h.writeError(w, http.StatusBadRequest, "team_id is required")
		return
	}

	added := make([]string, 0, len(req.Members))
	for _, m := range req.Members {
		membership := &auth.TeamMembership{
			UserID: m.UserID,
			TeamID: req.TeamID,
		}
		if err := h.store.CreateTeamMembership(r.Context(), membership); err != nil {
			h.logger.Warn("failed to add team member", "user_id", m.UserID, "error", err)
			continue
		}
		added = append(added, m.UserID)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"team_id":       req.TeamID,
		"added_members": added,
	})
}

// TeamMemberDeleteRequest represents a request to remove members from a team.
type TeamMemberDeleteRequest struct {
	TeamID  string   `json:"team_id"`
	UserIDs []string `json:"user_ids"`
}

// DeleteTeamMember handles POST /team/member_delete
func (h *ManagementHandler) DeleteTeamMember(w http.ResponseWriter, r *http.Request) {
	var req TeamMemberDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TeamID == "" {
		h.writeError(w, http.StatusBadRequest, "team_id is required")
		return
	}

	removed := make([]string, 0, len(req.UserIDs))
	for _, userID := range req.UserIDs {
		if err := h.store.DeleteTeamMembership(r.Context(), userID, req.TeamID); err != nil {
			h.logger.Warn("failed to remove team member", "user_id", userID, "error", err)
			continue
		}
		removed = append(removed, userID)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"team_id":         req.TeamID,
		"removed_members": removed,
	})
}
