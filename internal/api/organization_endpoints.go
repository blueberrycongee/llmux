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
