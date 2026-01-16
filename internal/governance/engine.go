// Package governance provides policy evaluation and accounting for gateway requests.
package governance

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/blueberrycongee/llmux/internal/auth"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
)

// Engine evaluates governance policy and records usage.
type Engine struct {
	store       auth.Store
	rateLimiter *auth.TenantRateLimiter
	auditLogger *auth.AuditLogger
	idempotency IdempotencyStore
	logger      *slog.Logger
	config      atomic.Value
	enforcer    *auth.CasbinEnforcer
}

// NewEngine creates a governance engine with the provided config.
func NewEngine(cfg Config, opts ...Option) *Engine {
	engine := &Engine{
		logger: slog.Default(),
	}
	engine.config.Store(cfg)
	for _, opt := range opts {
		opt(engine)
	}
	if engine.logger == nil {
		engine.logger = slog.Default()
	}
	return engine
}

// UpdateConfig updates governance configuration at runtime.
func (e *Engine) UpdateConfig(cfg Config) {
	if e == nil {
		return
	}
	e.config.Store(cfg)
}

// Evaluate enforces governance checks before request execution.
func (e *Engine) Evaluate(ctx context.Context, input RequestInput) error {
	if e == nil {
		return nil
	}
	cfg := e.loadConfig()
	if !cfg.Enabled {
		return nil
	}

	authCtx := auth.GetAuthContext(ctx)
	resolved, err := e.resolveEntities(ctx, authCtx, input.EndUserID)
	if err != nil {
		return llmerrors.NewInternalError("gateway", input.Model, "failed to resolve auth context")
	}

	if err := e.checkModelAccess(ctx, input.Model, authCtx); err != nil {
		return err
	}

	if err := e.checkBudgets(input.Model, authCtx, resolved); err != nil {
		return err
	}

	if err := e.checkRateLimit(ctx, input, authCtx, resolved); err != nil {
		return err
	}

	return nil
}

// Account records usage and spend updates after request completion.
func (e *Engine) Account(ctx context.Context, input AccountInput) {
	if e == nil {
		return
	}
	cfg := e.loadConfig()
	if !cfg.Enabled {
		return
	}

	if cfg.AsyncAccounting {
		go e.account(ctx, input)
		return
	}
	e.account(ctx, input)
}

func (e *Engine) account(ctx context.Context, input AccountInput) {
	if e.store == nil {
		return
	}
	cfg := e.loadConfig()
	logKey := e.idempotencyKey(input)
	if logKey != "" && e.idempotency != nil && cfg.IdempotencyWindow > 0 {
		idempotencyCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		ok, err := e.idempotency.PutIfAbsent(idempotencyCtx, logKey, cfg.IdempotencyWindow)
		cancel()
		if err != nil {
			e.logger.Warn("idempotency check failed", "error", err, "request_id", input.RequestID)
		} else if !ok {
			return
		}
	}

	authCtx := auth.GetAuthContext(ctx)
	latency := input.Latency
	if latency <= 0 && !input.Start.IsZero() {
		latency = time.Since(input.Start)
	}
	endTime := time.Now()
	if !input.Start.IsZero() && latency > 0 {
		endTime = input.Start.Add(latency)
	}

	log := &auth.UsageLog{
		RequestID:    input.RequestID,
		Model:        input.Model,
		Provider:     providerLabel(input.Usage.Provider),
		CallType:     input.CallType,
		InputTokens:  input.Usage.PromptTokens,
		OutputTokens: input.Usage.CompletionTokens,
		TotalTokens:  input.Usage.TotalTokens,
		Cost:         input.Usage.Cost,
		StartTime:    input.Start,
		EndTime:      endTime,
		LatencyMs:    int(latency.Milliseconds()),
		RequestTags:  append([]string(nil), input.RequestTags...),
	}
	if input.StatusCode != nil {
		log.StatusCode = input.StatusCode
	}
	if input.Status != nil {
		log.Status = input.Status
	}

	if authCtx != nil && authCtx.APIKey != nil {
		log.APIKeyID = authCtx.APIKey.ID
		log.TeamID = authCtx.APIKey.TeamID
		log.OrganizationID = authCtx.APIKey.OrganizationID
		log.UserID = authCtx.APIKey.UserID
	}
	if log.TeamID == nil {
		if teamID := teamIDFromAuth(authCtx); teamID != "" {
			log.TeamID = &teamID
		}
	}
	if log.UserID == nil {
		if userID := userIDFromAuth(authCtx); userID != "" {
			log.UserID = &userID
		}
	}
	if log.OrganizationID == nil {
		if orgID := orgIDFromAuth(authCtx); orgID != "" {
			log.OrganizationID = &orgID
		}
	}

	endUserID := input.EndUserID
	if endUserID == "" && authCtx != nil {
		endUserID = authCtx.EndUserID
	}
	if endUserID != "" {
		log.EndUserID = &endUserID
	}

	bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := e.store.LogUsage(bgCtx, log); err != nil {
		e.logger.Warn("failed to log usage", "error", err, "request_id", input.RequestID)
	}

	if input.Usage.Cost <= 0 {
		return
	}

	if authCtx != nil && authCtx.APIKey != nil {
		if err := e.store.UpdateAPIKeySpent(bgCtx, authCtx.APIKey.ID, input.Usage.Cost); err != nil {
			e.logger.Warn("failed to update api key spend", "error", err, "key_id", authCtx.APIKey.ID)
		}
		if input.Model != "" {
			if err := e.store.UpdateAPIKeyModelSpent(bgCtx, authCtx.APIKey.ID, input.Model, input.Usage.Cost); err != nil {
				e.logger.Warn("failed to update api key model spend", "error", err, "key_id", authCtx.APIKey.ID, "model", input.Model)
			}
		}
	}

	teamID := teamIDFromAuth(authCtx)
	if teamID != "" {
		if err := e.store.UpdateTeamSpent(bgCtx, teamID, input.Usage.Cost); err != nil {
			e.logger.Warn("failed to update team spend", "error", err, "team_id", teamID)
		}
		if input.Model != "" {
			if err := e.store.UpdateTeamModelSpent(bgCtx, teamID, input.Model, input.Usage.Cost); err != nil {
				e.logger.Warn("failed to update team model spend", "error", err, "team_id", teamID, "model", input.Model)
			}
		}
	}

	userID := userIDFromAuth(authCtx)
	if userID != "" {
		if err := e.store.UpdateUserSpent(bgCtx, userID, input.Usage.Cost); err != nil {
			e.logger.Warn("failed to update user spend", "error", err, "user_id", userID)
		}
		if teamID != "" {
			if err := e.store.UpdateTeamMembershipSpent(bgCtx, userID, teamID, input.Usage.Cost); err != nil {
				e.logger.Warn("failed to update team membership spend", "error", err, "user_id", userID, "team_id", teamID)
			}
		}
	}

	orgID := orgIDFromAuth(authCtx)
	if orgID != "" {
		if err := e.store.UpdateOrganizationSpent(bgCtx, orgID, input.Usage.Cost); err != nil {
			e.logger.Warn("failed to update org spend", "error", err, "org_id", orgID)
		}
		if userID != "" {
			if err := e.store.UpdateOrganizationMembershipSpent(bgCtx, userID, orgID, input.Usage.Cost); err != nil {
				e.logger.Warn("failed to update org membership spend", "error", err, "user_id", userID, "org_id", orgID)
			}
		}
	}

	if endUserID != "" {
		if err := e.store.UpdateEndUserSpent(bgCtx, endUserID, input.Usage.Cost); err != nil {
			e.logger.Warn("failed to update end user spend", "error", err, "end_user_id", endUserID)
		}
	}
}

type resolvedEntities struct {
	team    *auth.Team
	user    *auth.User
	org     *auth.Organization
	endUser *auth.EndUser
}

func (e *Engine) resolveEntities(ctx context.Context, authCtx *auth.AuthContext, endUserID string) (resolvedEntities, error) {
	var resolved resolvedEntities
	if authCtx != nil {
		resolved.team = authCtx.Team
		resolved.user = authCtx.User
	}

	if e.store == nil {
		return resolved, nil
	}

	if resolved.team == nil && authCtx != nil && authCtx.APIKey != nil && authCtx.APIKey.TeamID != nil {
		team, err := e.store.GetTeam(ctx, *authCtx.APIKey.TeamID)
		if err != nil {
			return resolved, err
		}
		resolved.team = team
	}

	if resolved.user == nil && authCtx != nil && authCtx.APIKey != nil && authCtx.APIKey.UserID != nil {
		user, err := e.store.GetUser(ctx, *authCtx.APIKey.UserID)
		if err != nil {
			return resolved, err
		}
		resolved.user = user
	}

	orgID := orgIDFromAuth(authCtx)
	if orgID != "" {
		org, err := e.store.GetOrganization(ctx, orgID)
		if err != nil && !errors.Is(err, context.Canceled) {
			return resolved, err
		}
		resolved.org = org
	}

	if endUserID != "" {
		endUser, err := e.store.GetEndUser(ctx, endUserID)
		if err != nil {
			return resolved, err
		}
		resolved.endUser = endUser
	}

	return resolved, nil
}

func (e *Engine) checkModelAccess(ctx context.Context, model string, authCtx *auth.AuthContext) error {
	if model == "" || authCtx == nil {
		return nil
	}

	if e.enforcer != nil && authCtx.APIKey != nil {
		sub := auth.KeySub(authCtx.APIKey.ID)
		obj := auth.ModelObj(model)
		act := auth.ActionUse()

		// Sync roles for the key
		_, _ = e.enforcer.AddRoleForUser(sub, auth.RoleSub(string(authCtx.APIKey.KeyType)))
		if authCtx.Team != nil {
			_, _ = e.enforcer.AddGroupingPolicy(sub, auth.TeamSub(authCtx.Team.ID))
		}
		if authCtx.User != nil {
			_, _ = e.enforcer.AddGroupingPolicy(sub, auth.UserSub(authCtx.User.ID))
		}

		// Legacy support for allowed_models
		if len(authCtx.APIKey.AllowedModels) > 0 {
			for _, am := range authCtx.APIKey.AllowedModels {
				_, _ = e.enforcer.AddPolicy(sub, auth.ModelObj(am), act)
			}
		} else {
			_, _ = e.enforcer.AddPolicy(sub, auth.ModelObj("*"), act)
		}

		allowed, err := e.enforcer.Enforce(sub, obj, act)
		if err != nil {
			e.logger.Error("failed to evaluate model access via casbin", "error", err)
			return llmerrors.NewInternalError("gateway", model, "failed to evaluate model access")
		}
		if allowed {
			return nil
		}
		return llmerrors.NewPermissionError("gateway", model, "model access denied")
	}

	access, err := auth.NewModelAccess(ctx, e.store, authCtx)
	if err != nil {
		e.logger.Error("failed to evaluate model access", "error", err)
		return llmerrors.NewInternalError("gateway", model, "failed to evaluate model access")
	}
	if access == nil || access.Allows(model) {
		return nil
	}
	return llmerrors.NewPermissionError("gateway", model, "model access denied")
}

func (e *Engine) checkBudgets(model string, authCtx *auth.AuthContext, resolved resolvedEntities) error {
	if authCtx == nil {
		return nil
	}

	if authCtx.APIKey != nil {
		if authCtx.APIKey.IsOverBudget() || isModelOverBudget(model, authCtx.APIKey.ModelMaxBudget, authCtx.APIKey.ModelSpend) {
			e.auditBudgetExceeded(authCtx, auth.AuditObjectAPIKey, authCtx.APIKey.ID, model)
			return llmerrors.NewInsufficientQuotaError("gateway", model, "api key budget exceeded")
		}
	}

	if resolved.team != nil {
		if resolved.team.IsOverBudget() || isModelOverBudget(model, resolved.team.ModelMaxBudget, resolved.team.ModelSpend) {
			e.auditBudgetExceeded(authCtx, auth.AuditObjectTeam, resolved.team.ID, model)
			return llmerrors.NewInsufficientQuotaError("gateway", model, "team budget exceeded")
		}
	}

	if resolved.user != nil {
		if resolved.user.IsOverBudget() || isModelOverBudget(model, resolved.user.ModelMaxBudget, resolved.user.ModelSpend) {
			e.auditBudgetExceeded(authCtx, auth.AuditObjectUser, resolved.user.ID, model)
			return llmerrors.NewInsufficientQuotaError("gateway", model, "user budget exceeded")
		}
	}

	if resolved.org != nil {
		if resolved.org.IsOverBudget() {
			e.auditBudgetExceeded(authCtx, auth.AuditObjectOrganization, resolved.org.ID, model)
			return llmerrors.NewInsufficientQuotaError("gateway", model, "organization budget exceeded")
		}
	}

	if resolved.endUser != nil {
		if resolved.endUser.IsOverBudget() {
			e.auditBudgetExceeded(authCtx, auth.AuditObjectEndUser, resolved.endUser.UserID, model)
			return llmerrors.NewInsufficientQuotaError("gateway", model, "end user budget exceeded")
		}
		if resolved.endUser.IsBlocked() {
			return llmerrors.NewPermissionError("gateway", model, "end user blocked")
		}
	}

	return nil
}

func (e *Engine) checkRateLimit(ctx context.Context, input RequestInput, authCtx *auth.AuthContext, resolved resolvedEntities) error {
	if e.rateLimiter == nil {
		return nil
	}

	if authCtx == nil || authCtx.APIKey == nil {
		tenantID := ""
		if authCtx != nil && authCtx.User != nil {
			tenantID = authCtx.User.ID
		} else if input.Request != nil {
			tenantID = e.rateLimiter.AnonymousKey(input.Request)
		}
		if tenantID == "" {
			return nil
		}

		if resolved.team != nil && resolved.team.RPMLimit != nil && *resolved.team.RPMLimit > 0 {
			teamID := "team:" + resolved.team.ID
			teamRPM := int(*resolved.team.RPMLimit)
			allowed, _ := e.rateLimiter.Check(ctx, teamID, teamRPM, e.rateLimiter.BurstForRate(teamRPM, 0))
			if !allowed {
				return llmerrors.NewRateLimitError("gateway", input.Model, "team rate limit exceeded")
			}
		}

		rpm := 0
		if authCtx != nil && authCtx.User != nil && authCtx.User.RPMLimit != nil {
			rpm = int(*authCtx.User.RPMLimit)
		}
		burst := e.rateLimiter.BurstForRate(rpm, 1)
		allowed, _ := e.rateLimiter.Check(ctx, tenantID, rpm, burst)
		if !allowed {
			return llmerrors.NewRateLimitError("gateway", input.Model, "rate limit exceeded")
		}
		return nil
	}

	tenantID := authCtx.APIKey.ID
	rpm := 0
	if authCtx.APIKey.RPMLimit != nil {
		rpm = int(*authCtx.APIKey.RPMLimit)
	}
	burst := e.rateLimiter.BurstForRate(rpm, 1)

	if resolved.team != nil && resolved.team.RPMLimit != nil && *resolved.team.RPMLimit > 0 {
		teamID := "team:" + resolved.team.ID
		teamRPM := int(*resolved.team.RPMLimit)
		allowed, _ := e.rateLimiter.Check(ctx, teamID, teamRPM, e.rateLimiter.BurstForRate(teamRPM, 0))
		if !allowed {
			return llmerrors.NewRateLimitError("gateway", input.Model, "team rate limit exceeded")
		}
	}

	allowed, _ := e.rateLimiter.Check(ctx, tenantID, rpm, burst)
	if !allowed {
		return llmerrors.NewRateLimitError("gateway", input.Model, "rate limit exceeded")
	}
	return nil
}

func (e *Engine) auditBudgetExceeded(authCtx *auth.AuthContext, objectType auth.AuditObjectType, objectID, model string) {
	cfg := e.loadConfig()
	if !cfg.AuditEnabled || e.auditLogger == nil {
		return
	}

	actorID, actorType := auditActor(authCtx)
	before := map[string]any{
		"model": model,
	}
	if err := e.auditLogger.LogAction(actorID, actorType, auth.AuditActionBudgetExceeded, objectType, objectID, false, before, nil); err != nil {
		e.logger.Warn("failed to log budget audit event", "error", err)
	}
}

func (e *Engine) idempotencyKey(input AccountInput) string {
	if input.RequestID == "" {
		return ""
	}
	if input.CallType == "" {
		return "usage:" + input.RequestID
	}
	return fmt.Sprintf("usage:%s:%s", input.CallType, input.RequestID)
}

func (e *Engine) loadConfig() Config {
	if e == nil {
		return Config{}
	}
	if cfg, ok := e.config.Load().(Config); ok {
		return cfg
	}
	return Config{}
}

func isModelOverBudget(model string, maxBudget map[string]float64, spend map[string]float64) bool {
	if model == "" || len(maxBudget) == 0 {
		return false
	}
	limit, ok := maxBudget[model]
	if !ok || limit <= 0 {
		return false
	}
	if spend == nil {
		return false
	}
	return spend[model] >= limit
}

func providerLabel(provider string) string {
	if provider == "" {
		return "llmux"
	}
	return provider
}

func auditActor(authCtx *auth.AuthContext) (string, string) {
	if authCtx == nil {
		return "", ""
	}
	if authCtx.APIKey != nil {
		return authCtx.APIKey.ID, "api_key"
	}
	if authCtx.User != nil {
		return authCtx.User.ID, "user"
	}
	return "", ""
}

func teamIDFromAuth(authCtx *auth.AuthContext) string {
	if authCtx == nil {
		return ""
	}
	if authCtx.Team != nil {
		return authCtx.Team.ID
	}
	if authCtx.APIKey != nil && authCtx.APIKey.TeamID != nil {
		return *authCtx.APIKey.TeamID
	}
	if authCtx.User != nil && authCtx.User.TeamID != nil {
		return *authCtx.User.TeamID
	}
	return ""
}

func userIDFromAuth(authCtx *auth.AuthContext) string {
	if authCtx == nil {
		return ""
	}
	if authCtx.User != nil {
		return authCtx.User.ID
	}
	if authCtx.APIKey != nil && authCtx.APIKey.UserID != nil {
		return *authCtx.APIKey.UserID
	}
	return ""
}

func orgIDFromAuth(authCtx *auth.AuthContext) string {
	if authCtx == nil {
		return ""
	}
	if authCtx.APIKey != nil && authCtx.APIKey.OrganizationID != nil {
		return *authCtx.APIKey.OrganizationID
	}
	if authCtx.User != nil && authCtx.User.OrganizationID != nil {
		return *authCtx.User.OrganizationID
	}
	if authCtx.Team != nil && authCtx.Team.OrganizationID != nil {
		return *authCtx.Team.OrganizationID
	}
	return ""
}
