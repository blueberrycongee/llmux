// Package api provides HTTP handlers for the LLM gateway API.
// Audit log management endpoints.
package api //nolint:revive // package name is intentional

import (
	"net/http"
	"strconv"
	"time"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/internal/auth"
)

// ============================================================================
// Audit Log Endpoints
// ============================================================================

// AuditLogHandler handles audit log API endpoints.
type AuditLogHandler struct {
	auditStore auth.AuditLogStore
	writeJSON  func(w http.ResponseWriter, status int, data any)
	writeError func(w http.ResponseWriter, r *http.Request, status int, message string)
}

// NewAuditLogHandler creates a new audit log handler.
func NewAuditLogHandler(
	auditStore auth.AuditLogStore,
	writeJSON func(w http.ResponseWriter, status int, data any),
	writeError func(w http.ResponseWriter, r *http.Request, status int, message string),
) *AuditLogHandler {
	return &AuditLogHandler{
		auditStore: auditStore,
		writeJSON:  writeJSON,
		writeError: writeError,
	}
}

// ListAuditLogs handles GET /audit/logs
func (h *AuditLogHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Parse filter parameters
	filter := auth.AuditLogFilter{}

	if actorID := query.Get("actor_id"); actorID != "" {
		filter.ActorID = &actorID
	}
	if actorType := query.Get("actor_type"); actorType != "" {
		filter.ActorType = &actorType
	}
	if actionStr := query.Get("action"); actionStr != "" {
		action := auth.AuditAction(actionStr)
		filter.Action = &action
	}
	if objectTypeStr := query.Get("object_type"); objectTypeStr != "" {
		objectType := auth.AuditObjectType(objectTypeStr)
		filter.ObjectType = &objectType
	}
	if objectID := query.Get("object_id"); objectID != "" {
		filter.ObjectID = &objectID
	}
	if teamID := query.Get("team_id"); teamID != "" {
		filter.TeamID = &teamID
	}
	if orgID := query.Get("organization_id"); orgID != "" {
		filter.OrganizationID = &orgID
	}
	if successStr := query.Get("success"); successStr != "" {
		success := successStr == "true"
		filter.Success = &success
	}

	// Parse time range
	if startTimeStr := query.Get("start_time"); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			filter.StartTime = t
		}
	}
	if endTimeStr := query.Get("end_time"); endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			filter.EndTime = t
		}
	}

	// Parse pagination
	if limit, err := strconv.Atoi(query.Get("limit")); err == nil && limit > 0 {
		filter.Limit = limit
	} else {
		filter.Limit = 50
	}
	if offset, err := strconv.Atoi(query.Get("offset")); err == nil && offset >= 0 {
		filter.Offset = offset
	}

	logs, total, err := h.auditStore.ListAuditLogs(filter)
	if err != nil {
		h.writeError(w, r, http.StatusInternalServerError, "failed to list audit logs")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"logs":   logs,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// GetAuditLog handles GET /audit/log
func (h *AuditLogHandler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	logID := r.URL.Query().Get("id")
	if logID == "" {
		h.writeError(w, r, http.StatusBadRequest, "id parameter is required")
		return
	}

	log, err := h.auditStore.GetAuditLog(logID)
	if err != nil {
		h.writeError(w, r, http.StatusInternalServerError, "failed to get audit log")
		return
	}
	if log == nil {
		h.writeError(w, r, http.StatusNotFound, "audit log not found")
		return
	}

	h.writeJSON(w, http.StatusOK, log)
}

// GetAuditLogStats handles GET /audit/stats
func (h *AuditLogHandler) GetAuditLogStats(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filter := auth.AuditLogFilter{}

	// Parse time range (required for stats)
	if startTimeStr := query.Get("start_time"); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			filter.StartTime = t
		}
	} else {
		// Default to last 24 hours
		filter.StartTime = time.Now().Add(-24 * time.Hour)
	}

	if endTimeStr := query.Get("end_time"); endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			filter.EndTime = t
		}
	} else {
		filter.EndTime = time.Now()
	}

	// Optional filters
	if actorID := query.Get("actor_id"); actorID != "" {
		filter.ActorID = &actorID
	}
	if actionStr := query.Get("action"); actionStr != "" {
		action := auth.AuditAction(actionStr)
		filter.Action = &action
	}
	if objectTypeStr := query.Get("object_type"); objectTypeStr != "" {
		objectType := auth.AuditObjectType(objectTypeStr)
		filter.ObjectType = &objectType
	}

	stats, err := h.auditStore.GetAuditLogStats(filter)
	if err != nil {
		h.writeError(w, r, http.StatusInternalServerError, "failed to get audit stats")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"stats":      stats,
		"start_time": filter.StartTime.Format(time.RFC3339),
		"end_time":   filter.EndTime.Format(time.RFC3339),
	})
}

// DeleteAuditLogsRequest represents a request to delete old audit logs.
type DeleteAuditLogsRequest struct {
	OlderThanDays int `json:"older_than_days"`
}

// DeleteAuditLogs handles POST /audit/delete
func (h *AuditLogHandler) DeleteAuditLogs(w http.ResponseWriter, r *http.Request) {
	var req DeleteAuditLogsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OlderThanDays <= 0 {
		h.writeError(w, r, http.StatusBadRequest, "older_than_days must be positive")
		return
	}

	cutoff := time.Now().Add(-time.Duration(req.OlderThanDays) * 24 * time.Hour)
	deleted, err := h.auditStore.DeleteAuditLogs(cutoff)
	if err != nil {
		h.writeError(w, r, http.StatusInternalServerError, "failed to delete audit logs")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"deleted_count": deleted,
		"cutoff_date":   cutoff.Format(time.RFC3339),
	})
}

// RegisterAuditRoutes registers audit log routes on the given mux.
func (h *AuditLogHandler) RegisterAuditRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /audit/logs", h.ListAuditLogs)
	mux.HandleFunc("GET /audit/log", h.GetAuditLog)
	mux.HandleFunc("GET /audit/stats", h.GetAuditLogStats)
	mux.HandleFunc("POST /audit/delete", h.DeleteAuditLogs)
}
