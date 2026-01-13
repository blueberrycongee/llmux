package api //nolint:revive // package name is intentional

import (
	"io"
	"net/http"
	"time"

	"github.com/goccy/go-json"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/observability"
	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/router"
)

type deploymentControlStatus struct {
	Deployment     *provider.Deployment    `json:"deployment"`
	Stats          *router.DeploymentStats `json:"stats,omitempty"`
	CooldownActive bool                    `json:"cooldown_active"`
}

type providerControlStatus struct {
	Provider   string                `json:"provider"`
	Models     []string              `json:"models,omitempty"`
	Resilience llmux.ResilienceStats `json:"resilience"`
}

type cooldownRequest struct {
	DeploymentID    string `json:"deployment_id"`
	CooldownSeconds int    `json:"cooldown_seconds"`
}

type configReloadRequest struct {
	ExpectedChecksum string `json:"expected_checksum,omitempty"`
}

func (h *ManagementHandler) ListDeployments(w http.ResponseWriter, r *http.Request) {
	client, release := h.acquireClient()
	defer release()
	if client == nil {
		h.writeError(w, http.StatusServiceUnavailable, "client not available")
		return
	}

	deployments := client.ListDeployments()
	statuses := make([]deploymentControlStatus, 0, len(deployments))
	now := time.Now()
	for _, deployment := range deployments {
		if deployment == nil {
			continue
		}
		stats := client.GetStats(deployment.ID)
		cooldownActive := false
		if stats != nil && !stats.CooldownUntil.IsZero() {
			cooldownActive = now.Before(stats.CooldownUntil)
		}
		statuses = append(statuses, deploymentControlStatus{
			Deployment:     deployment,
			Stats:          stats,
			CooldownActive: cooldownActive,
		})
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data": statuses,
	})
}

func (h *ManagementHandler) UpdateDeploymentCooldown(w http.ResponseWriter, r *http.Request) {
	client, release := h.acquireClient()
	defer release()
	if client == nil {
		h.writeError(w, http.StatusServiceUnavailable, "client not available")
		return
	}

	var req cooldownRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DeploymentID == "" {
		h.writeError(w, http.StatusBadRequest, "deployment_id is required")
		return
	}
	if req.CooldownSeconds < 0 {
		req.CooldownSeconds = 0
	}

	var exists bool
	for _, deployment := range client.ListDeployments() {
		if deployment != nil && deployment.ID == req.DeploymentID {
			exists = true
			break
		}
	}
	if !exists {
		h.writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	before := client.GetStats(req.DeploymentID)
	var beforeCooldown time.Time
	if before != nil {
		beforeCooldown = before.CooldownUntil
	}

	var until time.Time
	action := "clear"
	if req.CooldownSeconds > 0 {
		until = time.Now().Add(time.Duration(req.CooldownSeconds) * time.Second)
		action = "set"
	}

	if err := client.SetCooldown(req.DeploymentID, until); err != nil {
		h.auditControlAction(r, auth.AuditActionUpdate, auth.AuditObjectModel, req.DeploymentID, false, nil, nil, map[string]any{
			"cooldown_seconds": req.CooldownSeconds,
			"action":           action,
		}, err.Error())
		h.writeError(w, http.StatusInternalServerError, "failed to update cooldown")
		return
	}

	h.auditControlAction(r, auth.AuditActionUpdate, auth.AuditObjectModel, req.DeploymentID, true, map[string]any{
		"cooldown_until": beforeCooldown,
	}, map[string]any{
		"cooldown_until": until,
	}, map[string]any{
		"cooldown_seconds": req.CooldownSeconds,
		"action":           action,
	}, "")

	stats := client.GetStats(req.DeploymentID)
	cooldownActive := false
	cooldownUntil := until
	if stats != nil && !stats.CooldownUntil.IsZero() {
		cooldownUntil = stats.CooldownUntil
		cooldownActive = time.Now().Before(stats.CooldownUntil)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"deployment_id":   req.DeploymentID,
		"cooldown_until":  cooldownUntil,
		"cooldown_active": cooldownActive,
	})
}

func (h *ManagementHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	client, release := h.acquireClient()
	defer release()
	if client == nil {
		h.writeError(w, http.StatusServiceUnavailable, "client not available")
		return
	}

	names := client.GetProviders()
	statuses := make([]providerControlStatus, 0, len(names))
	for _, name := range names {
		prov, _ := client.GetProvider(name)
		models := []string{}
		if prov != nil {
			models = append(models, prov.SupportedModels()...)
		}
		statuses = append(statuses, providerControlStatus{
			Provider:   name,
			Models:     models,
			Resilience: client.ResilienceStats(name),
		})
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data": statuses,
	})
}

func (h *ManagementHandler) GetConfigStatus(w http.ResponseWriter, r *http.Request) {
	if h.configManager == nil {
		h.writeError(w, http.StatusServiceUnavailable, "config manager not available")
		return
	}

	h.writeJSON(w, http.StatusOK, h.configManager.Status())
}

func (h *ManagementHandler) ReloadConfig(w http.ResponseWriter, r *http.Request) {
	if h.configManager == nil {
		h.writeError(w, http.StatusServiceUnavailable, "config manager not available")
		return
	}

	var req configReloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	before := h.configManager.Status()
	if req.ExpectedChecksum != "" && req.ExpectedChecksum != before.Checksum {
		h.writeError(w, http.StatusConflict, "config checksum mismatch")
		return
	}

	if err := h.configManager.Reload(); err != nil {
		h.auditControlAction(r, auth.AuditActionConfigUpdate, auth.AuditObjectConfig, "gateway", false, nil, nil, map[string]any{
			"previous_checksum": before.Checksum,
		}, err.Error())
		h.writeError(w, http.StatusInternalServerError, "failed to reload config")
		return
	}

	after := h.configManager.Status()
	h.auditControlAction(r, auth.AuditActionConfigUpdate, auth.AuditObjectConfig, "gateway", true, map[string]any{
		"checksum": before.Checksum,
	}, map[string]any{
		"checksum": after.Checksum,
	}, map[string]any{
		"previous_checksum": before.Checksum,
		"new_checksum":      after.Checksum,
	}, "")

	h.writeJSON(w, http.StatusOK, after)
}

func (h *ManagementHandler) acquireClient() (*llmux.Client, func()) {
	if h == nil || h.clientSwapper == nil {
		return nil, func() {}
	}
	return h.clientSwapper.Acquire()
}

type auditActor struct {
	id     string
	kind   string
	email  string
	teamID *string
	orgID  *string
}

func (h *ManagementHandler) auditControlAction(
	r *http.Request,
	action auth.AuditAction,
	objectType auth.AuditObjectType,
	objectID string,
	success bool,
	beforeValue, afterValue map[string]any,
	metadata map[string]any,
	errMsg string,
) {
	if h.auditLogger == nil || r == nil {
		return
	}

	actor := auditActorFromContext(auth.GetAuthContext(r.Context()))
	log := &auth.AuditLog{
		ActorID:        actor.id,
		ActorType:      actor.kind,
		ActorEmail:     actor.email,
		ActorIP:        requesterIP(r.RemoteAddr),
		Action:         action,
		ObjectType:     objectType,
		ObjectID:       objectID,
		TeamID:         actor.teamID,
		OrganizationID: actor.orgID,
		BeforeValue:    beforeValue,
		AfterValue:     afterValue,
		Metadata:       metadata,
		RequestID:      observability.RequestIDFromContext(r.Context()),
		UserAgent:      r.UserAgent(),
		RequestURI:     r.RequestURI,
		Success:        success,
		Error:          errMsg,
	}
	_ = h.auditLogger.Log(log)
}

func auditActorFromContext(authCtx *auth.AuthContext) auditActor {
	actor := auditActor{
		id:   "system",
		kind: "system",
	}

	if authCtx == nil {
		return actor
	}

	if authCtx.User != nil && authCtx.User.ID != "" {
		actor.id = authCtx.User.ID
		actor.kind = "user"
		if authCtx.User.Email != nil {
			actor.email = *authCtx.User.Email
		}
		if authCtx.User.TeamID != nil {
			actor.teamID = authCtx.User.TeamID
		}
		if authCtx.User.OrganizationID != nil {
			actor.orgID = authCtx.User.OrganizationID
		}
	}

	if actor.kind == "system" && authCtx.APIKey != nil && authCtx.APIKey.ID != "" {
		actor.id = authCtx.APIKey.ID
		actor.kind = "api_key"
		actor.teamID = authCtx.APIKey.TeamID
		actor.orgID = authCtx.APIKey.OrganizationID
	}

	if actor.teamID == nil && authCtx.Team != nil {
		actor.teamID = stringPtr(authCtx.Team.ID)
	}
	if actor.orgID == nil && authCtx.Team != nil {
		actor.orgID = authCtx.Team.OrganizationID
	}

	return actor
}
