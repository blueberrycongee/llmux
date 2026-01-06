// Package api provides HTTP handlers for the LLM gateway API.
// User management endpoints.
package api

import (
	"github.com/goccy/go-json"
	"net/http"
	"strconv"
	"time"

	"github.com/blueberrycongee/llmux/internal/auth"
)

// ============================================================================
// User Management Endpoints
// ============================================================================

// NewUserRequest represents a request to create a new user.
type NewUserRequest struct {
	UserID          string             `json:"user_id,omitempty"`
	UserAlias       *string            `json:"user_alias,omitempty"`
	UserEmail       *string            `json:"user_email,omitempty"`
	TeamID          *string            `json:"team_id,omitempty"`
	OrganizationID  *string            `json:"organization_id,omitempty"`
	UserRole        string             `json:"user_role,omitempty"` // proxy_admin, org_admin, internal_user
	Models          []string           `json:"models,omitempty"`
	MaxBudget       *float64           `json:"max_budget,omitempty"`
	BudgetDuration  string             `json:"budget_duration,omitempty"`
	TPMLimit        *int64             `json:"tpm_limit,omitempty"`
	RPMLimit        *int64             `json:"rpm_limit,omitempty"`
	MaxParallelReqs *int               `json:"max_parallel_requests,omitempty"`
	ModelMaxBudget  map[string]float64 `json:"model_max_budget,omitempty"`
	Metadata        auth.Metadata      `json:"metadata,omitempty"`
}

// NewUser handles POST /user/new
func (h *ManagementHandler) NewUser(w http.ResponseWriter, r *http.Request) {
	var req NewUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	now := time.Now()
	userID := req.UserID
	if userID == "" {
		userID = auth.GenerateUUID()
	}

	role := req.UserRole
	if role == "" {
		role = string(auth.UserRoleInternalUser)
	}

	user := &auth.User{
		ID:                  userID,
		Alias:               req.UserAlias,
		Email:               req.UserEmail,
		TeamID:              req.TeamID,
		OrganizationID:      req.OrganizationID,
		Role:                role,
		Models:              req.Models,
		TPMLimit:            req.TPMLimit,
		RPMLimit:            req.RPMLimit,
		MaxParallelRequests: req.MaxParallelReqs,
		ModelMaxBudget:      req.ModelMaxBudget,
		Metadata:            req.Metadata,
		IsActive:            true,
		CreatedAt:           &now,
		UpdatedAt:           &now,
	}

	if req.MaxBudget != nil {
		user.MaxBudget = *req.MaxBudget
	}

	if req.BudgetDuration != "" {
		user.BudgetDuration = auth.BudgetDuration(req.BudgetDuration)
		user.BudgetResetAt = user.BudgetDuration.NextResetTime()
	}

	if err := h.store.CreateUser(r.Context(), user); err != nil {
		h.logger.Error("failed to create user", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	h.writeJSON(w, http.StatusOK, user)
}

// UpdateUserRequest represents a request to update a user.
type UpdateUserRequest struct {
	UserID          string             `json:"user_id"`
	UserAlias       *string            `json:"user_alias,omitempty"`
	UserEmail       *string            `json:"user_email,omitempty"`
	UserRole        *string            `json:"user_role,omitempty"`
	Models          []string           `json:"models,omitempty"`
	MaxBudget       *float64           `json:"max_budget,omitempty"`
	BudgetDuration  *string            `json:"budget_duration,omitempty"`
	TPMLimit        *int64             `json:"tpm_limit,omitempty"`
	RPMLimit        *int64             `json:"rpm_limit,omitempty"`
	MaxParallelReqs *int               `json:"max_parallel_requests,omitempty"`
	ModelMaxBudget  map[string]float64 `json:"model_max_budget,omitempty"`
	Metadata        auth.Metadata      `json:"metadata,omitempty"`
}

// UpdateUser handles POST /user/update
func (h *ManagementHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.UserID == "" {
		h.writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	user, err := h.store.GetUser(r.Context(), req.UserID)
	if err != nil || user == nil {
		h.writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// Update fields
	if req.UserAlias != nil {
		user.Alias = req.UserAlias
	}
	if req.UserEmail != nil {
		user.Email = req.UserEmail
	}
	if req.UserRole != nil {
		user.Role = *req.UserRole
	}
	if req.Models != nil {
		user.Models = req.Models
	}
	if req.MaxBudget != nil {
		user.MaxBudget = *req.MaxBudget
	}
	if req.BudgetDuration != nil {
		user.BudgetDuration = auth.BudgetDuration(*req.BudgetDuration)
		user.BudgetResetAt = user.BudgetDuration.NextResetTime()
	}
	if req.TPMLimit != nil {
		user.TPMLimit = req.TPMLimit
	}
	if req.RPMLimit != nil {
		user.RPMLimit = req.RPMLimit
	}
	if req.MaxParallelReqs != nil {
		user.MaxParallelRequests = req.MaxParallelReqs
	}
	if req.ModelMaxBudget != nil {
		user.ModelMaxBudget = req.ModelMaxBudget
	}
	if req.Metadata != nil {
		user.Metadata = mergeMetadata(user.Metadata, req.Metadata)
	}

	now := time.Now()
	user.UpdatedAt = &now

	if err := h.store.UpdateUser(r.Context(), user); err != nil {
		h.logger.Error("failed to update user", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	h.writeJSON(w, http.StatusOK, user)
}

// DeleteUserRequest represents a request to delete users.
type DeleteUserRequest struct {
	UserIDs []string `json:"user_ids"`
}

// DeleteUser handles POST /user/delete
func (h *ManagementHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	var req DeleteUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.UserIDs) == 0 {
		h.writeError(w, http.StatusBadRequest, "user_ids is required")
		return
	}

	deleted := make([]string, 0, len(req.UserIDs))
	for _, userID := range req.UserIDs {
		if err := h.store.DeleteUser(r.Context(), userID); err != nil {
			h.logger.Warn("failed to delete user", "user_id", userID, "error", err)
			continue
		}
		deleted = append(deleted, userID)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"deleted_users": deleted,
	})
}

// GetUserInfo handles GET /user/info
func (h *ManagementHandler) GetUserInfo(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		h.writeError(w, http.StatusBadRequest, "user_id parameter is required")
		return
	}

	user, err := h.store.GetUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get user info", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to get user info")
		return
	}
	if user == nil {
		h.writeError(w, http.StatusNotFound, "user not found")
		return
	}

	h.writeJSON(w, http.StatusOK, user)
}

// ListUsers handles GET /user/list
func (h *ManagementHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	teamID := r.URL.Query().Get("team_id")
	orgID := r.URL.Query().Get("organization_id")
	role := r.URL.Query().Get("role")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		limit = 50
	}
	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil || offset < 0 {
		offset = 0
	}

	filter := auth.UserFilter{
		Limit:  limit,
		Offset: offset,
	}
	if teamID != "" {
		filter.TeamID = &teamID
	}
	if orgID != "" {
		filter.OrganizationID = &orgID
	}
	if role != "" {
		r := auth.UserRole(role)
		filter.Role = &r
	}

	users, total, err := h.store.ListUsers(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list users", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":  users,
		"total": total,
	})
}
