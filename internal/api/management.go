// Package api provides HTTP handlers for the LLM gateway API.
// Management endpoints for API keys, teams, users, and organizations.
package api //nolint:revive // package name is intentional

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/goccy/go-json"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
)

// ManagementHandler handles management API endpoints.
type ManagementHandler struct {
	store         auth.Store
	auditStore    auth.AuditLogStore
	auditLogger   *auth.AuditLogger
	clientSwapper *ClientSwapper
	configManager *config.Manager
	logger        *slog.Logger
}

// NewManagementHandler creates a new management handler.
func NewManagementHandler(store auth.Store, auditStore auth.AuditLogStore, logger *slog.Logger, swapper *ClientSwapper, cfgManager *config.Manager, auditLogger *auth.AuditLogger) *ManagementHandler {
	return &ManagementHandler{
		store:         store,
		auditStore:    auditStore,
		auditLogger:   auditLogger,
		clientSwapper: swapper,
		configManager: cfgManager,
		logger:        logger,
	}
}

// ============================================================================
// API Key Management Endpoints
// ============================================================================

// GenerateKeyRequest represents a request to generate a new API key.
type GenerateKeyRequest struct {
	Name             string             `json:"key_name,omitempty"`
	KeyAlias         *string            `json:"key_alias,omitempty"`
	TeamID           *string            `json:"team_id,omitempty"`
	UserID           *string            `json:"user_id,omitempty"`
	OrganizationID   *string            `json:"organization_id,omitempty"`
	Models           []string           `json:"models,omitempty"`
	MaxBudget        *float64           `json:"max_budget,omitempty"`
	SoftBudget       *float64           `json:"soft_budget,omitempty"`
	BudgetDuration   string             `json:"budget_duration,omitempty"`
	TPMLimit         *int64             `json:"tpm_limit,omitempty"`
	RPMLimit         *int64             `json:"rpm_limit,omitempty"`
	MaxParallelReqs  *int               `json:"max_parallel_requests,omitempty"`
	ModelMaxBudget   map[string]float64 `json:"model_max_budget,omitempty"`
	ModelTPMLimit    map[string]int64   `json:"model_tpm_limit,omitempty"`
	ModelRPMLimit    map[string]int64   `json:"model_rpm_limit,omitempty"`
	Duration         string             `json:"duration,omitempty"` // Key expiry duration
	Metadata         auth.Metadata      `json:"metadata,omitempty"`
	KeyType          string             `json:"key_type,omitempty"` // llm_api, management, read_only
	AutoRotate       bool               `json:"auto_rotate,omitempty"`
	RotationInterval string             `json:"rotation_interval,omitempty"` // e.g., "30d", "90d"
}

// GenerateKeyResponse represents the response after generating a key.
type GenerateKeyResponse struct {
	Key            string     `json:"key"`
	KeyID          string     `json:"token_id"`
	KeyPrefix      string     `json:"key_prefix"`
	Name           string     `json:"key_name,omitempty"`
	KeyAlias       *string    `json:"key_alias,omitempty"`
	TeamID         *string    `json:"team_id,omitempty"`
	UserID         *string    `json:"user_id,omitempty"`
	OrganizationID *string    `json:"organization_id,omitempty"`
	Models         []string   `json:"models,omitempty"`
	MaxBudget      float64    `json:"max_budget,omitempty"`
	SoftBudget     *float64   `json:"soft_budget,omitempty"`
	TPMLimit       *int64     `json:"tpm_limit,omitempty"`
	RPMLimit       *int64     `json:"rpm_limit,omitempty"`
	ExpiresAt      *time.Time `json:"expires,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// GenerateKey handles POST /key/generate
func (h *ManagementHandler) GenerateKey(w http.ResponseWriter, r *http.Request) {
	var req GenerateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Generate a new API key
	rawKey, keyHash, err := auth.GenerateAPIKey()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to generate api key")
		return
	}
	keyPrefix := auth.ExtractKeyPrefix(rawKey)

	now := time.Now()
	key := &auth.APIKey{
		ID:                  auth.GenerateUUID(),
		KeyHash:             keyHash,
		KeyPrefix:           keyPrefix,
		Name:                req.Name,
		KeyAlias:            req.KeyAlias,
		TeamID:              req.TeamID,
		UserID:              req.UserID,
		OrganizationID:      req.OrganizationID,
		AllowedModels:       req.Models,
		TPMLimit:            req.TPMLimit,
		RPMLimit:            req.RPMLimit,
		MaxParallelRequests: req.MaxParallelReqs,
		ModelMaxBudget:      req.ModelMaxBudget,
		ModelTPMLimit:       req.ModelTPMLimit,
		ModelRPMLimit:       req.ModelRPMLimit,
		Metadata:            req.Metadata,
		IsActive:            true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	// Set budget
	if req.MaxBudget != nil {
		key.MaxBudget = *req.MaxBudget
	}
	if req.SoftBudget != nil {
		key.SoftBudget = req.SoftBudget
	}

	// Set budget duration
	if req.BudgetDuration != "" {
		key.BudgetDuration = auth.BudgetDuration(req.BudgetDuration)
		key.BudgetResetAt = key.BudgetDuration.NextResetTime()
	}

	// Set key expiry
	if req.Duration != "" {
		expiry := auth.ParseDuration(req.Duration)
		if expiry != nil {
			key.ExpiresAt = expiry
		}
	}

	// Set key type
	if req.KeyType != "" {
		key.KeyType = auth.KeyType(req.KeyType)
	}

	// Set auto rotation
	if req.AutoRotate && req.RotationInterval != "" {
		key.Metadata = ensureMetadata(key.Metadata)
		key.Metadata["auto_rotate"] = true
		key.Metadata["rotation_interval"] = req.RotationInterval
		rotationAt := auth.CalculateRotationTime(req.RotationInterval)
		if rotationAt != nil {
			key.Metadata["key_rotation_at"] = rotationAt.Format(time.RFC3339)
		}
	}

	// Save to store
	if err := h.store.CreateAPIKey(r.Context(), key); err != nil {
		h.logger.Error("failed to create api key", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to create api key")
		return
	}

	resp := GenerateKeyResponse{
		Key:            rawKey,
		KeyID:          key.ID,
		KeyPrefix:      key.KeyPrefix,
		Name:           key.Name,
		KeyAlias:       key.KeyAlias,
		TeamID:         key.TeamID,
		UserID:         key.UserID,
		OrganizationID: key.OrganizationID,
		Models:         key.AllowedModels,
		MaxBudget:      key.MaxBudget,
		SoftBudget:     key.SoftBudget,
		TPMLimit:       key.TPMLimit,
		RPMLimit:       key.RPMLimit,
		ExpiresAt:      key.ExpiresAt,
		CreatedAt:      key.CreatedAt,
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// UpdateKeyRequest represents a request to update an API key.
type UpdateKeyRequest struct {
	Key              string             `json:"key"` // Key ID or hash
	Name             *string            `json:"key_name,omitempty"`
	KeyAlias         *string            `json:"key_alias,omitempty"`
	Models           []string           `json:"models,omitempty"`
	MaxBudget        *float64           `json:"max_budget,omitempty"`
	SoftBudget       *float64           `json:"soft_budget,omitempty"`
	BudgetDuration   *string            `json:"budget_duration,omitempty"`
	TPMLimit         *int64             `json:"tpm_limit,omitempty"`
	RPMLimit         *int64             `json:"rpm_limit,omitempty"`
	MaxParallelReqs  *int               `json:"max_parallel_requests,omitempty"`
	ModelMaxBudget   map[string]float64 `json:"model_max_budget,omitempty"`
	Metadata         auth.Metadata      `json:"metadata,omitempty"`
	Duration         *string            `json:"duration,omitempty"`
	AutoRotate       *bool              `json:"auto_rotate,omitempty"`
	RotationInterval *string            `json:"rotation_interval,omitempty"`
}

// UpdateKey handles POST /key/update
func (h *ManagementHandler) UpdateKey(w http.ResponseWriter, r *http.Request) {
	var req UpdateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Key == "" {
		h.writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	// Get existing key
	key, err := h.store.GetAPIKeyByID(r.Context(), req.Key)
	if err != nil || key == nil {
		h.writeError(w, http.StatusNotFound, "key not found")
		return
	}

	// Update fields
	if req.Name != nil {
		key.Name = *req.Name
	}
	if req.KeyAlias != nil {
		key.KeyAlias = req.KeyAlias
	}
	if req.Models != nil {
		key.AllowedModels = req.Models
	}
	if req.MaxBudget != nil {
		key.MaxBudget = *req.MaxBudget
	}
	if req.SoftBudget != nil {
		key.SoftBudget = req.SoftBudget
	}
	if req.BudgetDuration != nil {
		key.BudgetDuration = auth.BudgetDuration(*req.BudgetDuration)
		key.BudgetResetAt = key.BudgetDuration.NextResetTime()
	}
	if req.TPMLimit != nil {
		key.TPMLimit = req.TPMLimit
	}
	if req.RPMLimit != nil {
		key.RPMLimit = req.RPMLimit
	}
	if req.MaxParallelReqs != nil {
		key.MaxParallelRequests = req.MaxParallelReqs
	}
	if req.ModelMaxBudget != nil {
		key.ModelMaxBudget = req.ModelMaxBudget
	}
	if req.Metadata != nil {
		key.Metadata = mergeMetadata(key.Metadata, req.Metadata)
	}
	if req.Duration != nil {
		key.ExpiresAt = auth.ParseDuration(*req.Duration)
	}

	// Handle auto rotation update
	if req.AutoRotate != nil {
		key.Metadata = ensureMetadata(key.Metadata)
		key.Metadata["auto_rotate"] = *req.AutoRotate
		if *req.AutoRotate && req.RotationInterval != nil {
			key.Metadata["rotation_interval"] = *req.RotationInterval
			rotationAt := auth.CalculateRotationTime(*req.RotationInterval)
			if rotationAt != nil {
				key.Metadata["key_rotation_at"] = rotationAt.Format(time.RFC3339)
			}
		}
	}

	key.UpdatedAt = time.Now()

	if err := h.store.UpdateAPIKey(r.Context(), key); err != nil {
		h.logger.Error("failed to update api key", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to update api key")
		return
	}

	h.writeJSON(w, http.StatusOK, key)
}

// DeleteKeyRequest represents a request to delete API keys.
type DeleteKeyRequest struct {
	Keys []string `json:"keys"`
}

// DeleteKey handles POST /key/delete
func (h *ManagementHandler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	var req DeleteKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Keys) == 0 {
		h.writeError(w, http.StatusBadRequest, "keys is required")
		return
	}

	deleted := make([]string, 0, len(req.Keys))
	for _, keyID := range req.Keys {
		if err := h.store.DeleteAPIKey(r.Context(), keyID); err != nil {
			h.logger.Warn("failed to delete key", "key_id", keyID, "error", err)
			continue
		}
		deleted = append(deleted, keyID)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"deleted_keys": deleted,
	})
}

// GetKeyInfo handles GET /key/info
func (h *ManagementHandler) GetKeyInfo(w http.ResponseWriter, r *http.Request) {
	keyID := r.URL.Query().Get("key")
	if keyID == "" {
		h.writeError(w, http.StatusBadRequest, "key parameter is required")
		return
	}

	key, err := h.store.GetAPIKeyByID(r.Context(), keyID)
	if err != nil {
		h.logger.Error("failed to get key info", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to get key info")
		return
	}
	if key == nil {
		h.writeError(w, http.StatusNotFound, "key not found")
		return
	}

	h.writeJSON(w, http.StatusOK, key)
}

// ListKeys handles GET /key/list
func (h *ManagementHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	teamID := r.URL.Query().Get("team_id")
	userID := r.URL.Query().Get("user_id")
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		limit = 0
	}
	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil {
		offset = 0
	}

	if limit <= 0 {
		limit = 50
	}

	filter := auth.APIKeyFilter{
		Limit:  limit,
		Offset: offset,
	}
	if teamID != "" {
		filter.TeamID = &teamID
	}
	if userID != "" {
		filter.UserID = &userID
	}

	keys, total, err := h.store.ListAPIKeys(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list keys", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to list keys")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":  keys,
		"total": total,
	})
}

// BlockKey handles POST /key/block
func (h *ManagementHandler) BlockKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.store.BlockAPIKey(r.Context(), req.Key, true); err != nil {
		h.logger.Error("failed to block key", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to block key")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "blocked"})
}

// UnblockKey handles POST /key/unblock
func (h *ManagementHandler) UnblockKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.store.BlockAPIKey(r.Context(), req.Key, false); err != nil {
		h.logger.Error("failed to unblock key", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to unblock key")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "unblocked"})
}

// RegenerateKey handles POST /key/regenerate
func (h *ManagementHandler) RegenerateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"` // existing key ID
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Get existing key
	oldKey, err := h.store.GetAPIKeyByID(r.Context(), req.Key)
	if err != nil || oldKey == nil {
		h.writeError(w, http.StatusNotFound, "key not found")
		return
	}

	// Generate new key credentials
	rawKey, keyHash, err := auth.GenerateAPIKey()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to generate api key")
		return
	}
	keyPrefix := auth.ExtractKeyPrefix(rawKey)

	now := time.Now()
	oldKey.KeyHash = keyHash
	oldKey.KeyPrefix = keyPrefix
	oldKey.UpdatedAt = now

	// Update rotation count in metadata
	oldKey.Metadata = ensureMetadata(oldKey.Metadata)
	rotationCount := 0
	switch v := oldKey.Metadata["rotation_count"].(type) {
	case int:
		rotationCount = v
	case int64:
		rotationCount = int(v)
	case float64:
		rotationCount = int(v)
	}
	oldKey.Metadata["rotation_count"] = rotationCount + 1
	oldKey.Metadata["last_rotation_at"] = now.Format(time.RFC3339)

	// Recalculate next rotation time if auto_rotate is enabled
	if autoRotate, ok := oldKey.Metadata["auto_rotate"].(bool); ok && autoRotate {
		if interval, ok := oldKey.Metadata["rotation_interval"].(string); ok {
			rotationAt := auth.CalculateRotationTime(interval)
			if rotationAt != nil {
				oldKey.Metadata["key_rotation_at"] = rotationAt.Format(time.RFC3339)
			}
		}
	}

	if err := h.store.UpdateAPIKey(r.Context(), oldKey); err != nil {
		h.logger.Error("failed to update key during regenerate", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to regenerate key")
		return
	}

	h.writeJSON(w, http.StatusOK, GenerateKeyResponse{
		Key:            rawKey,
		KeyID:          oldKey.ID,
		KeyPrefix:      oldKey.KeyPrefix,
		Name:           oldKey.Name,
		KeyAlias:       oldKey.KeyAlias,
		TeamID:         oldKey.TeamID,
		UserID:         oldKey.UserID,
		OrganizationID: oldKey.OrganizationID,
		Models:         oldKey.AllowedModels,
		MaxBudget:      oldKey.MaxBudget,
		SoftBudget:     oldKey.SoftBudget,
		TPMLimit:       oldKey.TPMLimit,
		RPMLimit:       oldKey.RPMLimit,
		ExpiresAt:      oldKey.ExpiresAt,
		CreatedAt:      oldKey.CreatedAt,
	})
}

// Helper functions
//
//nolint:unparam // status parameter kept for future flexibility
func (h *ManagementHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode JSON response", "error", err)
	}
}

func (h *ManagementHandler) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"message": message,
			"type":    "api_error",
		},
	}); err != nil {
		h.logger.Error("failed to encode error response", "error", err)
	}
}

func ensureMetadata(m auth.Metadata) auth.Metadata {
	if m == nil {
		return make(auth.Metadata)
	}
	return m
}

func mergeMetadata(existing, updated auth.Metadata) auth.Metadata {
	if existing == nil {
		return updated
	}
	for k, v := range updated {
		existing[k] = v
	}
	return existing
}
