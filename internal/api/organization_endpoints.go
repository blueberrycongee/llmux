// Package api provides HTTP handlers for the LLM gateway API.
// Organization management endpoints.
package api //nolint:revive // package name is intentional

import (
	"net/http"
	"strconv"
	"time"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/internal/auth"
)

// ============================================================================
// Organization Management Endpoints
// ============================================================================

// NewOrganizationRequest represents a request to create a new organization.
type NewOrganizationRequest struct {
	OrganizationID    string             `json:"organization_id,omitempty"`
	OrganizationAlias string             `json:"organization_alias"`
	Models            []string           `json:"models,omitempty"`
	MaxBudget         *float64           `json:"max_budget,omitempty"`
	BudgetDuration    string             `json:"budget_duration,omitempty"`
	TPMLimit          *int64             `json:"tpm_limit,omitempty"`
	RPMLimit          *int64             `json:"rpm_limit,omitempty"`
	ModelMaxBudget    map[string]float64 `json:"model_max_budget,omitempty"`
	Metadata          auth.Metadata      `json:"metadata,omitempty"`
}

// NewOrganization handles POST /organization/new
func (h *ManagementHandler) NewOrganization(w http.ResponseWriter, r *http.Request) {
	var req NewOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OrganizationAlias == "" {
		h.writeError(w, http.StatusBadRequest, "organization_alias is required")
		return
	}

	now := time.Now()
	orgID := req.OrganizationID
	if orgID == "" {
		orgID = auth.GenerateUUID()
	}

	org := &auth.Organization{
		ID:        orgID,
		Alias:     req.OrganizationAlias,
		Models:    req.Models,
		Metadata:  req.Metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if req.MaxBudget != nil {
		org.MaxBudget = *req.MaxBudget
	}

	// Create a budget if budget params are provided
	if req.MaxBudget != nil || req.TPMLimit != nil || req.RPMLimit != nil {
		budget := &auth.Budget{
			ID:        auth.GenerateUUID(),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if req.MaxBudget != nil {
			budget.MaxBudget = req.MaxBudget
		}
		if req.TPMLimit != nil {
			budget.TPMLimit = req.TPMLimit
		}
		if req.RPMLimit != nil {
			budget.RPMLimit = req.RPMLimit
		}
		if req.ModelMaxBudget != nil {
			budget.ModelMaxBudget = req.ModelMaxBudget
		}
		if req.BudgetDuration != "" {
			budget.BudgetDuration = auth.BudgetDuration(req.BudgetDuration)
			budget.BudgetResetAt = budget.BudgetDuration.NextResetTime()
		}

		if err := h.store.CreateBudget(r.Context(), budget); err != nil {
			h.logger.Error("failed to create budget", "error", err)
			h.writeError(w, http.StatusInternalServerError, "failed to create organization budget")
			return
		}
		org.BudgetID = &budget.ID
	}

	if err := h.store.CreateOrganization(r.Context(), org); err != nil {
		h.logger.Error("failed to create organization", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to create organization")
		return
	}

	h.writeJSON(w, http.StatusOK, org)
}

// UpdateOrganizationRequest represents a request to update an organization.
type UpdateOrganizationRequest struct {
	OrganizationID    string             `json:"organization_id"`
	OrganizationAlias *string            `json:"organization_alias,omitempty"`
	Models            []string           `json:"models,omitempty"`
	MaxBudget         *float64           `json:"max_budget,omitempty"`
	BudgetDuration    *string            `json:"budget_duration,omitempty"`
	TPMLimit          *int64             `json:"tpm_limit,omitempty"`
	RPMLimit          *int64             `json:"rpm_limit,omitempty"`
	ModelMaxBudget    map[string]float64 `json:"model_max_budget,omitempty"`
	Metadata          auth.Metadata      `json:"metadata,omitempty"`
}

// UpdateOrganization handles PATCH /organization/update
func (h *ManagementHandler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	var req UpdateOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OrganizationID == "" {
		h.writeError(w, http.StatusBadRequest, "organization_id is required")
		return
	}

	org, err := h.store.GetOrganization(r.Context(), req.OrganizationID)
	if err != nil || org == nil {
		h.writeError(w, http.StatusNotFound, "organization not found")
		return
	}

	// Update fields
	if req.OrganizationAlias != nil {
		org.Alias = *req.OrganizationAlias
	}
	if req.Models != nil {
		org.Models = req.Models
	}
	if req.MaxBudget != nil {
		org.MaxBudget = *req.MaxBudget
	}
	if req.Metadata != nil {
		org.Metadata = mergeMetadata(org.Metadata, req.Metadata)
	}

	org.UpdatedAt = time.Now()

	// Update budget if exists
	if org.BudgetID != nil {
		budget, err := h.store.GetBudget(r.Context(), *org.BudgetID)
		if err == nil && budget != nil {
			if req.MaxBudget != nil {
				budget.MaxBudget = req.MaxBudget
			}
			if req.TPMLimit != nil {
				budget.TPMLimit = req.TPMLimit
			}
			if req.RPMLimit != nil {
				budget.RPMLimit = req.RPMLimit
			}
			if req.ModelMaxBudget != nil {
				budget.ModelMaxBudget = req.ModelMaxBudget
			}
			if req.BudgetDuration != nil {
				budget.BudgetDuration = auth.BudgetDuration(*req.BudgetDuration)
				budget.BudgetResetAt = budget.BudgetDuration.NextResetTime()
			}
			budget.UpdatedAt = time.Now()
			if err := h.store.UpdateBudget(r.Context(), budget); err != nil {
				h.logger.Error("failed to update budget", "error", err)
			}
		}
	}

	if err := h.store.UpdateOrganization(r.Context(), org); err != nil {
		h.logger.Error("failed to update organization", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to update organization")
		return
	}

	h.writeJSON(w, http.StatusOK, org)
}

// DeleteOrganizationRequest represents a request to delete organizations.
type DeleteOrganizationRequest struct {
	OrganizationIDs []string `json:"organization_ids"`
}

// DeleteOrganization handles DELETE /organization/delete
func (h *ManagementHandler) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	var req DeleteOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.OrganizationIDs) == 0 {
		h.writeError(w, http.StatusBadRequest, "organization_ids is required")
		return
	}

	deleted := make([]string, 0, len(req.OrganizationIDs))
	for _, orgID := range req.OrganizationIDs {
		if err := h.store.DeleteOrganization(r.Context(), orgID); err != nil {
			h.logger.Warn("failed to delete organization", "org_id", orgID, "error", err)
			continue
		}
		deleted = append(deleted, orgID)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"deleted_organizations": deleted,
	})
}

// GetOrganizationInfo handles GET /organization/info
func (h *ManagementHandler) GetOrganizationInfo(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("organization_id")
	if orgID == "" {
		h.writeError(w, http.StatusBadRequest, "organization_id parameter is required")
		return
	}

	org, err := h.store.GetOrganization(r.Context(), orgID)
	if err != nil {
		h.logger.Error("failed to get organization info", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to get organization info")
		return
	}
	if org == nil {
		h.writeError(w, http.StatusNotFound, "organization not found")
		return
	}

	// Get budget info if exists
	var budget *auth.Budget
	if org.BudgetID != nil {
		budget, err = h.store.GetBudget(r.Context(), *org.BudgetID)
		if err != nil {
			h.logger.Error("failed to get budget", "error", err)
		}
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"organization": org,
		"budget":       budget,
	})
}

// ListOrganizations handles GET /organization/list
func (h *ManagementHandler) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit <= 0 {
		limit = 50
	}
	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil || offset < 0 {
		offset = 0
	}

	orgs, total, err := h.store.ListOrganizations(r.Context(), limit, offset)
	if err != nil {
		h.logger.Error("failed to list organizations", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to list organizations")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":  orgs,
		"total": total,
	})
}

// ============================================================================
// Organization Member Management Endpoints
// ============================================================================

// OrgMemberAddRequest represents a request to add members to an organization.
type OrgMemberAddRequest struct {
	OrganizationID string      `json:"organization_id"`
	Members        []OrgMember `json:"members"`
}

// OrgMember represents an organization member with role.
type OrgMember struct {
	UserID    string   `json:"user_id"`
	UserRole  string   `json:"user_role,omitempty"` // org_admin, member
	BudgetID  *string  `json:"budget_id,omitempty"`
	MaxBudget *float64 `json:"max_budget,omitempty"`
}

// AddOrganizationMember handles POST /organization/member_add
func (h *ManagementHandler) AddOrganizationMember(w http.ResponseWriter, r *http.Request) {
	var req OrgMemberAddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OrganizationID == "" {
		h.writeError(w, http.StatusBadRequest, "organization_id is required")
		return
	}

	if len(req.Members) == 0 {
		h.writeError(w, http.StatusBadRequest, "members is required")
		return
	}

	// Verify organization exists
	org, err := h.store.GetOrganization(r.Context(), req.OrganizationID)
	if err != nil || org == nil {
		h.writeError(w, http.StatusNotFound, "organization not found")
		return
	}

	added := make([]*auth.OrganizationMembership, 0, len(req.Members))
	now := time.Now()

	for _, member := range req.Members {
		if member.UserID == "" {
			continue
		}

		// Check if membership already exists
		existing, _ := h.store.GetOrganizationMembership(r.Context(), member.UserID, req.OrganizationID)
		if existing != nil {
			h.logger.Warn("member already exists in organization", "user_id", member.UserID, "org_id", req.OrganizationID)
			continue
		}

		// Set default role
		role := member.UserRole
		if role == "" {
			role = "member"
		}

		membership := &auth.OrganizationMembership{
			UserID:         member.UserID,
			OrganizationID: req.OrganizationID,
			UserRole:       role,
			BudgetID:       member.BudgetID,
			JoinedAt:       &now,
		}

		// Create member-specific budget if max_budget provided
		if member.MaxBudget != nil {
			budget := &auth.Budget{
				ID:        auth.GenerateUUID(),
				MaxBudget: member.MaxBudget,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := h.store.CreateBudget(r.Context(), budget); err != nil {
				h.logger.Error("failed to create member budget", "error", err)
			} else {
				membership.BudgetID = &budget.ID
			}
		}

		if err := h.store.CreateOrganizationMembership(r.Context(), membership); err != nil {
			h.logger.Error("failed to add organization member", "user_id", member.UserID, "error", err)
			continue
		}

		added = append(added, membership)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"added_members": added,
	})
}

// OrgMemberUpdateRequest represents a request to update an organization member.
type OrgMemberUpdateRequest struct {
	OrganizationID string   `json:"organization_id"`
	UserID         string   `json:"user_id"`
	UserRole       *string  `json:"user_role,omitempty"`
	MaxBudget      *float64 `json:"max_budget,omitempty"`
}

// UpdateOrganizationMember handles POST /organization/member_update
func (h *ManagementHandler) UpdateOrganizationMember(w http.ResponseWriter, r *http.Request) {
	var req OrgMemberUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OrganizationID == "" || req.UserID == "" {
		h.writeError(w, http.StatusBadRequest, "organization_id and user_id are required")
		return
	}

	membership, err := h.store.GetOrganizationMembership(r.Context(), req.UserID, req.OrganizationID)
	if err != nil || membership == nil {
		h.writeError(w, http.StatusNotFound, "membership not found")
		return
	}

	// Update role
	if req.UserRole != nil {
		membership.UserRole = *req.UserRole
	}

	// Update budget if exists
	if membership.BudgetID != nil && req.MaxBudget != nil {
		budget, err := h.store.GetBudget(r.Context(), *membership.BudgetID)
		if err == nil && budget != nil {
			budget.MaxBudget = req.MaxBudget
			budget.UpdatedAt = time.Now()
			if err := h.store.UpdateBudget(r.Context(), budget); err != nil {
				h.logger.Error("failed to update member budget", "error", err)
			}
		}
	}

	if err := h.store.UpdateOrganizationMembership(r.Context(), membership); err != nil {
		h.logger.Error("failed to update organization member", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to update member")
		return
	}

	h.writeJSON(w, http.StatusOK, membership)
}

// OrgMemberDeleteRequest represents a request to remove members from an organization.
type OrgMemberDeleteRequest struct {
	OrganizationID string   `json:"organization_id"`
	UserIDs        []string `json:"user_ids"`
}

// DeleteOrganizationMember handles POST /organization/member_delete
func (h *ManagementHandler) DeleteOrganizationMember(w http.ResponseWriter, r *http.Request) {
	var req OrgMemberDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OrganizationID == "" || len(req.UserIDs) == 0 {
		h.writeError(w, http.StatusBadRequest, "organization_id and user_ids are required")
		return
	}

	deleted := make([]string, 0, len(req.UserIDs))
	for _, userID := range req.UserIDs {
		if err := h.store.DeleteOrganizationMembership(r.Context(), userID, req.OrganizationID); err != nil {
			h.logger.Warn("failed to delete organization member", "user_id", userID, "error", err)
			continue
		}
		deleted = append(deleted, userID)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"deleted_members": deleted,
	})
}

// ListOrganizationMembers handles GET /organization/members
func (h *ManagementHandler) ListOrganizationMembers(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("organization_id")
	if orgID == "" {
		h.writeError(w, http.StatusBadRequest, "organization_id parameter is required")
		return
	}

	members, err := h.store.ListOrganizationMembers(r.Context(), orgID)
	if err != nil {
		h.logger.Error("failed to list organization members", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to list members")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"organization_id": orgID,
		"members":         members,
		"total":           len(members),
	})
}
