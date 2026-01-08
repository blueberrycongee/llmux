// Package api provides HTTP handlers for the LLM gateway API.
// Invitation link management endpoints.
package api //nolint:revive // package name is intentional

import (
	"net/http"
	"strconv"
	"time"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/internal/auth"
)

// InvitationHandler handles invitation link API endpoints.
type InvitationHandler struct {
	service *auth.InvitationService
	store   auth.InvitationLinkStore
	logger  Logger
}

// Logger interface for logging.
type Logger interface {
	Error(msg string, args ...any)
	Warn(msg string, args ...any)
	Info(msg string, args ...any)
}

// NewInvitationHandler creates a new invitation handler.
func NewInvitationHandler(service *auth.InvitationService, store auth.InvitationLinkStore, logger Logger) *InvitationHandler {
	return &InvitationHandler{
		service: service,
		store:   store,
		logger:  logger,
	}
}

// RegisterInvitationRoutes registers invitation routes on the given mux.
func (h *InvitationHandler) RegisterInvitationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /invitation/new", h.CreateInvitation)
	mux.HandleFunc("POST /invitation/accept", h.AcceptInvitation)
	mux.HandleFunc("GET /invitation/info", h.GetInvitationInfo)
	mux.HandleFunc("GET /invitation/list", h.ListInvitations)
	mux.HandleFunc("POST /invitation/deactivate", h.DeactivateInvitation)
	mux.HandleFunc("POST /invitation/delete", h.DeleteInvitation)
}

// CreateInvitationRequest represents a request to create an invitation link.
type CreateInvitationRequest struct {
	TeamID         *string  `json:"team_id,omitempty"`
	OrganizationID *string  `json:"organization_id,omitempty"`
	Role           string   `json:"role,omitempty"`
	MaxUses        int      `json:"max_uses,omitempty"`
	MaxBudget      *float64 `json:"max_budget,omitempty"`
	ExpiresIn      int      `json:"expires_in,omitempty"` // Hours until expiration
	Description    string   `json:"description,omitempty"`
}

// CreateInvitationResponse represents the response after creating an invitation.
type CreateInvitationResponse struct {
	ID             string     `json:"id"`
	Token          string     `json:"token"` // Raw token for sharing
	TeamID         *string    `json:"team_id,omitempty"`
	OrganizationID *string    `json:"organization_id,omitempty"`
	Role           string     `json:"role,omitempty"`
	MaxUses        int        `json:"max_uses,omitempty"`
	MaxBudget      *float64   `json:"max_budget,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	Description    string     `json:"description,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// CreateInvitation handles POST /invitation/new
func (h *InvitationHandler) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	var req CreateInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate: must have either team_id or organization_id
	if req.TeamID == nil && req.OrganizationID == nil {
		h.writeError(w, http.StatusBadRequest, "team_id or organization_id is required")
		return
	}

	// Get creator from context (assuming auth middleware sets this)
	createdBy := r.Header.Get("X-User-ID")
	if createdBy == "" {
		createdBy = "system"
	}

	createReq := &auth.CreateInvitationRequest{
		TeamID:         req.TeamID,
		OrganizationID: req.OrganizationID,
		Role:           req.Role,
		MaxUses:        req.MaxUses,
		MaxBudget:      req.MaxBudget,
		ExpiresIn:      req.ExpiresIn,
		Description:    req.Description,
		CreatedBy:      createdBy,
	}

	link, rawToken, err := h.service.CreateInvitationLink(r.Context(), createReq)
	if err != nil {
		h.logger.Error("failed to create invitation link", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to create invitation link")
		return
	}

	resp := CreateInvitationResponse{
		ID:             link.ID,
		Token:          rawToken,
		TeamID:         link.TeamID,
		OrganizationID: link.OrganizationID,
		Role:           link.Role,
		MaxUses:        link.MaxUses,
		MaxBudget:      link.MaxBudget,
		ExpiresAt:      link.ExpiresAt,
		Description:    link.Description,
		CreatedAt:      link.CreatedAt,
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// AcceptInvitationRequest represents a request to accept an invitation.
type AcceptInvitationRequest struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
	Email  string `json:"email,omitempty"`
}

// AcceptInvitation handles POST /invitation/accept
func (h *InvitationHandler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	var req AcceptInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Token == "" {
		h.writeError(w, http.StatusBadRequest, "token is required")
		return
	}
	if req.UserID == "" {
		h.writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	acceptReq := &auth.AcceptInvitationRequest{
		Token:  req.Token,
		UserID: req.UserID,
		Email:  req.Email,
	}

	result, err := h.service.AcceptInvitation(r.Context(), acceptReq)
	if err != nil {
		h.logger.Error("failed to accept invitation", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to accept invitation")
		return
	}

	if !result.Success {
		h.writeError(w, http.StatusBadRequest, result.Message)
		return
	}

	h.writeJSON(w, http.StatusOK, result)
}

// GetInvitationInfo handles GET /invitation/info
func (h *InvitationHandler) GetInvitationInfo(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "id parameter is required")
		return
	}

	link, err := h.store.GetInvitationLink(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get invitation link", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to get invitation link")
		return
	}
	if link == nil {
		h.writeError(w, http.StatusNotFound, "invitation not found")
		return
	}

	// Don't expose the token hash
	link.Token = ""
	link.TokenHash = ""

	h.writeJSON(w, http.StatusOK, link)
}

// ListInvitations handles GET /invitation/list
func (h *InvitationHandler) ListInvitations(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filter := auth.InvitationLinkFilter{}

	if teamID := query.Get("team_id"); teamID != "" {
		filter.TeamID = &teamID
	}
	if orgID := query.Get("organization_id"); orgID != "" {
		filter.OrganizationID = &orgID
	}
	if createdBy := query.Get("created_by"); createdBy != "" {
		filter.CreatedBy = &createdBy
	}
	if isActiveStr := query.Get("is_active"); isActiveStr != "" {
		isActive := isActiveStr == "true"
		filter.IsActive = &isActive
	}
	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	// Default limit
	if filter.Limit <= 0 {
		filter.Limit = 50
	}

	links, err := h.store.ListInvitationLinks(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list invitation links", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to list invitation links")
		return
	}

	// Don't expose token hashes
	for _, link := range links {
		link.Token = ""
		link.TokenHash = ""
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":  links,
		"total": len(links),
	})
}

// DeactivateInvitationRequest represents a request to deactivate an invitation.
type DeactivateInvitationRequest struct {
	ID string `json:"id"`
}

// DeactivateInvitation handles POST /invitation/deactivate
func (h *InvitationHandler) DeactivateInvitation(w http.ResponseWriter, r *http.Request) {
	var req DeactivateInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		h.writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	if err := h.service.DeactivateInvitation(r.Context(), req.ID); err != nil {
		h.logger.Error("failed to deactivate invitation", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to deactivate invitation")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{
		"status":  "deactivated",
		"message": "invitation link has been deactivated",
	})
}

// DeleteInvitationRequest represents a request to delete invitations.
type DeleteInvitationRequest struct {
	IDs []string `json:"ids"`
}

// DeleteInvitation handles POST /invitation/delete
func (h *InvitationHandler) DeleteInvitation(w http.ResponseWriter, r *http.Request) {
	var req DeleteInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.IDs) == 0 {
		h.writeError(w, http.StatusBadRequest, "ids is required")
		return
	}

	deleted := make([]string, 0, len(req.IDs))
	for _, id := range req.IDs {
		if err := h.store.DeleteInvitationLink(r.Context(), id); err != nil {
			h.logger.Warn("failed to delete invitation", "id", id, "error", err)
			continue
		}
		deleted = append(deleted, id)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"deleted_ids": deleted,
	})
}

// Helper functions
//
//nolint:unparam // status parameter kept for future flexibility
func (h *InvitationHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode JSON response", "error", err)
	}
}

func (h *InvitationHandler) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"message": message,
			"type":    "api_error",
		},
	})
}
