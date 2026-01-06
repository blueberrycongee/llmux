// Package api provides HTTP handlers for the LLM gateway API.
// Spend tracking and analytics endpoints.
package api

import (
	"net/http"
	"time"

	"github.com/blueberrycongee/llmux/internal/auth"
)

// ============================================================================
// Spend Tracking Endpoints
// ============================================================================

// GetSpendLogs handles GET /spend/logs
func (h *ManagementHandler) GetSpendLogs(w http.ResponseWriter, r *http.Request) {
	apiKeyID := r.URL.Query().Get("api_key")
	teamID := r.URL.Query().Get("team_id")
	userID := r.URL.Query().Get("user_id")
	model := r.URL.Query().Get("model")
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	// Parse dates
	var startDate, endDate time.Time
	var err error
	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid start_date format, use YYYY-MM-DD")
			return
		}
	} else {
		startDate = time.Now().AddDate(0, 0, -30) // Default: last 30 days
	}

	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid end_date format, use YYYY-MM-DD")
			return
		}
	} else {
		endDate = time.Now()
	}

	filter := auth.UsageFilter{
		StartTime: startDate,
		EndTime:   endDate,
	}
	if apiKeyID != "" {
		filter.APIKeyID = &apiKeyID
	}
	if teamID != "" {
		filter.TeamID = &teamID
	}
	if model != "" {
		filter.Model = &model
	}

	stats, err := h.store.GetUsageStats(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to get spend logs", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to get spend logs")
		return
	}

	// Get daily breakdown
	dailyFilter := auth.DailyUsageFilter{
		StartDate: startDate.Format("2006-01-02"),
		EndDate:   endDate.Format("2006-01-02"),
		GroupBy:   []string{"date"},
	}
	if apiKeyID != "" {
		dailyFilter.APIKeyID = &apiKeyID
	}
	if teamID != "" {
		dailyFilter.TeamID = &teamID
	}
	if model != "" {
		dailyFilter.Model = &model
	}

	dailyUsage, err := h.store.GetDailyUsage(r.Context(), dailyFilter)
	if err != nil {
		h.logger.Warn("failed to get daily usage", "error", err)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"summary":     stats,
		"daily_usage": dailyUsage,
		"filters": map[string]any{
			"api_key_id": apiKeyID,
			"team_id":    teamID,
			"user_id":    userID,
			"model":      model,
			"start_date": startDate.Format("2006-01-02"),
			"end_date":   endDate.Format("2006-01-02"),
		},
	})
}

// GetSpendByKeys handles GET /spend/keys
func (h *ManagementHandler) GetSpendByKeys(w http.ResponseWriter, r *http.Request) {
	keys, _, err := h.store.ListAPIKeys(r.Context(), auth.APIKeyFilter{
		Limit: 100,
	})
	if err != nil {
		h.logger.Error("failed to get keys", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to get spend by keys")
		return
	}

	// Build response with spend info
	type keySpend struct {
		KeyID      string  `json:"key_id"`
		KeyPrefix  string  `json:"key_prefix"`
		KeyName    string  `json:"key_name"`
		TeamID     *string `json:"team_id,omitempty"`
		Spend      float64 `json:"spend"`
		MaxBudget  float64 `json:"max_budget"`
	}

	result := make([]keySpend, 0, len(keys))
	for _, k := range keys {
		result = append(result, keySpend{
			KeyID:     k.ID,
			KeyPrefix: k.KeyPrefix,
			KeyName:   k.Name,
			TeamID:    k.TeamID,
			Spend:     k.SpentBudget,
			MaxBudget: k.MaxBudget,
		})
	}

	h.writeJSON(w, http.StatusOK, result)
}

// GetSpendByTeams handles GET /spend/teams
func (h *ManagementHandler) GetSpendByTeams(w http.ResponseWriter, r *http.Request) {
	teams, _, err := h.store.ListTeams(r.Context(), auth.TeamFilter{
		Limit: 100,
	})
	if err != nil {
		h.logger.Error("failed to get teams", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to get spend by teams")
		return
	}

	type teamSpend struct {
		TeamID    string  `json:"team_id"`
		TeamAlias *string `json:"team_alias,omitempty"`
		Spend     float64 `json:"spend"`
		MaxBudget float64 `json:"max_budget"`
	}

	result := make([]teamSpend, 0, len(teams))
	for _, t := range teams {
		result = append(result, teamSpend{
			TeamID:    t.ID,
			TeamAlias: t.Alias,
			Spend:     t.SpentBudget,
			MaxBudget: t.MaxBudget,
		})
	}

	h.writeJSON(w, http.StatusOK, result)
}

// GetSpendByUsers handles GET /spend/users
func (h *ManagementHandler) GetSpendByUsers(w http.ResponseWriter, r *http.Request) {
	users, _, err := h.store.ListUsers(r.Context(), auth.UserFilter{
		Limit: 100,
	})
	if err != nil {
		h.logger.Error("failed to get users", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to get spend by users")
		return
	}

	type userSpend struct {
		UserID    string  `json:"user_id"`
		UserAlias *string `json:"user_alias,omitempty"`
		Email     *string `json:"email,omitempty"`
		Spend     float64 `json:"spend"`
		MaxBudget float64 `json:"max_budget"`
	}

	result := make([]userSpend, 0, len(users))
	for _, u := range users {
		result = append(result, userSpend{
			UserID:    u.ID,
			UserAlias: u.Alias,
			Email:     u.Email,
			Spend:     u.Spend,
			MaxBudget: u.MaxBudget,
		})
	}

	h.writeJSON(w, http.StatusOK, result)
}

// GetGlobalActivity handles GET /global/activity
func (h *ManagementHandler) GetGlobalActivity(w http.ResponseWriter, r *http.Request) {
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	var startDate, endDate time.Time
	var err error
	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid start_date format")
			return
		}
	} else {
		startDate = time.Now().AddDate(0, 0, -30)
	}

	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid end_date format")
			return
		}
	} else {
		endDate = time.Now()
	}

	filter := auth.UsageFilter{
		StartTime: startDate,
		EndTime:   endDate,
	}

	stats, err := h.store.GetUsageStats(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to get global activity", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to get global activity")
		return
	}

	// Get daily breakdown
	dailyFilter := auth.DailyUsageFilter{
		StartDate: startDate.Format("2006-01-02"),
		EndDate:   endDate.Format("2006-01-02"),
		GroupBy:   []string{"date"},
	}

	dailyUsage, err := h.store.GetDailyUsage(r.Context(), dailyFilter)
	if err != nil {
		h.logger.Warn("failed to get daily usage", "error", err)
	}

	// Format daily data for charts
	dailyData := make([]map[string]any, 0)
	var sumRequests int64
	var sumTokens int64

	for _, d := range dailyUsage {
		dailyData = append(dailyData, map[string]any{
			"date":         d.Date,
			"api_requests": d.APIRequests,
			"total_tokens": d.InputTokens + d.OutputTokens,
			"spend":        d.Spend,
		})
		sumRequests += d.APIRequests
		sumTokens += d.InputTokens + d.OutputTokens
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"daily_data":        dailyData,
		"sum_api_requests":  sumRequests,
		"sum_total_tokens":  sumTokens,
		"total_cost":        stats.TotalCost,
		"avg_latency_ms":    stats.AvgLatencyMs,
		"success_rate":      stats.SuccessRate,
		"unique_models":     stats.UniqueModels,
		"unique_providers":  stats.UniqueProviders,
	})
}

// GetGlobalSpendByModel handles GET /global/spend/models
func (h *ManagementHandler) GetGlobalSpendByModel(w http.ResponseWriter, r *http.Request) {
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	var startDate, endDate time.Time
	var err error
	if startDateStr != "" {
		startDate, _ = time.Parse("2006-01-02", startDateStr)
	} else {
		startDate = time.Now().AddDate(0, 0, -30)
	}
	if endDateStr != "" {
		endDate, _ = time.Parse("2006-01-02", endDateStr)
	} else {
		endDate = time.Now()
	}

	dailyFilter := auth.DailyUsageFilter{
		StartDate: startDate.Format("2006-01-02"),
		EndDate:   endDate.Format("2006-01-02"),
		GroupBy:   []string{"model"},
	}

	dailyUsage, err := h.store.GetDailyUsage(r.Context(), dailyFilter)
	if err != nil {
		h.logger.Error("failed to get spend by model", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to get spend by model")
		return
	}

	// Aggregate by model
	modelSpend := make(map[string]float64)
	modelRequests := make(map[string]int64)
	modelTokens := make(map[string]int64)

	for _, d := range dailyUsage {
		if d.Model != nil {
			modelSpend[*d.Model] += d.Spend
			modelRequests[*d.Model] += d.APIRequests
			modelTokens[*d.Model] += d.InputTokens + d.OutputTokens
		}
	}

	result := make([]map[string]any, 0)
	for model, spend := range modelSpend {
		result = append(result, map[string]any{
			"model":        model,
			"spend":        spend,
			"api_requests": modelRequests[model],
			"total_tokens": modelTokens[model],
		})
	}

	h.writeJSON(w, http.StatusOK, result)
}

// GetGlobalSpendByProvider handles GET /global/spend/provider
func (h *ManagementHandler) GetGlobalSpendByProvider(w http.ResponseWriter, r *http.Request) {
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	var startDate, endDate time.Time
	if startDateStr != "" {
		startDate, _ = time.Parse("2006-01-02", startDateStr)
	} else {
		startDate = time.Now().AddDate(0, 0, -30)
	}
	if endDateStr != "" {
		endDate, _ = time.Parse("2006-01-02", endDateStr)
	} else {
		endDate = time.Now()
	}

	dailyFilter := auth.DailyUsageFilter{
		StartDate: startDate.Format("2006-01-02"),
		EndDate:   endDate.Format("2006-01-02"),
		GroupBy:   []string{"provider"},
	}

	dailyUsage, err := h.store.GetDailyUsage(r.Context(), dailyFilter)
	if err != nil {
		h.logger.Error("failed to get spend by provider", "error", err)
		h.writeError(w, http.StatusInternalServerError, "failed to get spend by provider")
		return
	}

	// Aggregate by provider
	providerSpend := make(map[string]float64)
	providerRequests := make(map[string]int64)

	for _, d := range dailyUsage {
		if d.Provider != nil {
			providerSpend[*d.Provider] += d.Spend
			providerRequests[*d.Provider] += d.APIRequests
		}
	}

	result := make([]map[string]any, 0)
	for provider, spend := range providerSpend {
		result = append(result, map[string]any{
			"provider":     provider,
			"spend":        spend,
			"api_requests": providerRequests[provider],
		})
	}

	h.writeJSON(w, http.StatusOK, result)
}
