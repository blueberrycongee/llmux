package api //nolint:revive // package name is intentional

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/mcp"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
	llmtypes "github.com/blueberrycongee/llmux/pkg/types"
)

const (
	conversationLabScenarioGroundedRepoQA = "grounded-repo-qa"
	conversationLabScenarioSpeedChat      = "speed-chat"
	conversationLabScenarioMemoryFollowUp = "memory-follow-up"
)

type ConversationLabRequest struct {
	Scenario             string            `json:"scenario,omitempty"`
	Prompt               string            `json:"prompt"`
	TrafficType          string            `json:"traffic_type,omitempty"`
	SessionID            string            `json:"session_id,omitempty"`
	ComplexityHint       int               `json:"complexity_hint,omitempty"`
	RequireTeam          *bool             `json:"require_team,omitempty"`
	RequiredTools        []string          `json:"required_tools,omitempty"`
	RequiredCapabilities []string          `json:"required_capabilities,omitempty"`
	BaselineProvider     string            `json:"baseline_provider,omitempty"`
	BaselineModel        string            `json:"baseline_model,omitempty"`
	TokenOptimization    bool              `json:"token_optimization,omitempty"`
	CostOptimization     bool              `json:"cost_optimization,omitempty"`
	Experiment           ExperimentOptions `json:"experiment,omitempty"`
}

type ConversationLabQuality struct {
	Label             string   `json:"label"`
	ToolUsed          bool     `json:"tool_used"`
	ToolCalls         int      `json:"tool_calls"`
	ToolNames         []string `json:"tool_names,omitempty"`
	GroundednessScore float64  `json:"groundedness_score"`
	HallucinationRisk float64  `json:"hallucination_risk"`
	RouteDepth        int      `json:"route_depth"`
	MemoryHit         bool     `json:"memory_hit"`
	TeamRoute         bool     `json:"team_route"`
	QualityNotes      []string `json:"quality_notes,omitempty"`
}

type ConversationLabRun struct {
	Mode                     string                 `json:"mode"`
	Succeeded                bool                   `json:"succeeded"`
	ErrorMessage             string                 `json:"error_message,omitempty"`
	Provider                 string                 `json:"provider,omitempty"`
	Model                    string                 `json:"model,omitempty"`
	LatencyMs                int                    `json:"latency_ms"`
	PromptTokens             int                    `json:"prompt_tokens,omitempty"`
	CompletionTokens         int                    `json:"completion_tokens,omitempty"`
	TotalTokens              int                    `json:"total_tokens,omitempty"`
	EstimatedCost            float64                `json:"estimated_cost,omitempty"`
	ResponseChars            int                    `json:"response_chars,omitempty"`
	ResponseText             string                 `json:"response_text,omitempty"`
	FinishReason             string                 `json:"finish_reason,omitempty"`
	MemoryInfluence          string                 `json:"memory_influence,omitempty"`
	RouteSource              string                 `json:"route_source,omitempty"`
	SelectedTeamID           string                 `json:"selected_team_id,omitempty"`
	SelectedAgentID          string                 `json:"selected_agent_id,omitempty"`
	SelectedCandidate        string                 `json:"selected_candidate,omitempty"`
	UsedTeam                 bool                   `json:"used_team,omitempty"`
	UsedMemory               bool                   `json:"used_memory,omitempty"`
	RequiredTools            []string               `json:"required_tools,omitempty"`
	BoundTools               []string               `json:"bound_tools,omitempty"`
	RoutingReasoning         []string               `json:"routing_reasoning,omitempty"`
	CandidateFailovers       []CandidateFailover    `json:"candidate_failovers,omitempty"`
	Quality                  ConversationLabQuality `json:"quality"`
	TokenOptimizationEnabled bool                   `json:"token_optimization_enabled,omitempty"`
	OptimizationNotes        []string               `json:"optimization_notes,omitempty"`
}

type ConversationLabResponse struct {
	RequestID          string             `json:"request_id"`
	Scenario           string             `json:"scenario"`
	Prompt             string             `json:"prompt"`
	TrafficType        string             `json:"traffic_type"`
	Baseline           ConversationLabRun `json:"baseline"`
	Optimized          ConversationLabRun `json:"optimized"`
	LatencyDeltaMs     int                `json:"latency_delta_ms"`
	TotalTokenDelta    int                `json:"total_token_delta"`
	CostDelta          float64            `json:"cost_delta"`
	ResponseCharsDelta int                `json:"response_chars_delta"`
	WinningDimensions  []string           `json:"winning_dimensions"`
	RecommendedWinner  string             `json:"recommended_winner"`
	ValueSummary       []string           `json:"value_summary"`
	GeneratedAt        string             `json:"generated_at"`
}

func (h *ManagementHandler) CompareConversationLab(w http.ResponseWriter, r *http.Request) {
	var req ConversationLabRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	req = normalizeConversationLabRequest(req)
	client, release := h.acquireClient()
	defer release()
	if client == nil {
		h.writeError(w, r, http.StatusServiceUnavailable, "client not available")
		return
	}
	if req.Scenario == conversationLabScenarioMemoryFollowUp {
		seedConversationLabMemoryScenario(r.Context(), client, req)
	}

	ctx := r.Context()
	var baseline ConversationLabRun
	var optimized ConversationLabRun
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		baseline = h.runConversationLabBaseline(ctx, client, req)
	}()

	go func() {
		defer wg.Done()
		optimized = h.runConversationLabOptimized(ctx, client, req)
	}()

	wg.Wait()

	response := ConversationLabResponse{
		RequestID:          fmt.Sprintf("lab-%d", time.Now().UnixNano()),
		Scenario:           req.Scenario,
		Prompt:             req.Prompt,
		TrafficType:        req.TrafficType,
		Baseline:           baseline,
		Optimized:          optimized,
		LatencyDeltaMs:     optimized.LatencyMs - baseline.LatencyMs,
		TotalTokenDelta:    optimized.TotalTokens - baseline.TotalTokens,
		CostDelta:          optimized.EstimatedCost - baseline.EstimatedCost,
		ResponseCharsDelta: optimized.ResponseChars - baseline.ResponseChars,
		WinningDimensions:  compareWinningDimensions(req.Scenario, baseline, optimized),
		RecommendedWinner:  recommendConversationLabWinner(req.Scenario, baseline, optimized),
		ValueSummary:       buildConversationLabValueSummary(req.Scenario, baseline, optimized),
		GeneratedAt:        time.Now().Format(time.RFC3339),
	}

	h.writeJSON(w, http.StatusOK, response)
}

func normalizeConversationLabRequest(req ConversationLabRequest) ConversationLabRequest {
	req.Scenario = normalizeConversationLabScenario(req.Scenario)
	req.Prompt = strings.TrimSpace(req.Prompt)
	if strings.TrimSpace(req.TrafficType) == "" {
		req.TrafficType = "value-lab"
	}
	if req.SessionID == "" {
		req.SessionID = fmt.Sprintf("lab-session-%d", time.Now().UnixNano())
	}

	switch req.Scenario {
	case conversationLabScenarioSpeedChat:
		if req.Prompt == "" {
			req.Prompt = "Summarize what this gateway project does in five concise bullets for an engineering manager."
		}
		if req.ComplexityHint <= 0 {
			req.ComplexityHint = 1
		}
		if req.RequireTeam == nil {
			req.RequireTeam = boolPtr(false)
		}
		req.RequiredTools = nil
		if len(req.RequiredCapabilities) == 0 {
			req.RequiredCapabilities = []string{"general-chat", "summarization"}
		}
	case conversationLabScenarioMemoryFollowUp:
		if req.Prompt == "" {
			req.Prompt = "Continue the earlier architecture discussion and tell me which subsystem should change first if we want to improve memory-aware routing quality."
		}
		if req.ComplexityHint <= 0 {
			req.ComplexityHint = 2
		}
	default:
		if req.Prompt == "" {
			req.Prompt = "Analyze this gateway project as a memory/team/agent/model routing system. Use filesystem tools to inspect the top-level project structure, then explain how the optimized chain creates value over a plain chat response."
		}
		if req.ComplexityHint <= 0 {
			req.ComplexityHint = 2
		}
		if req.RequireTeam == nil {
			req.RequireTeam = boolPtr(false)
		}
		if len(req.RequiredTools) == 0 {
			req.RequiredTools = []string{"preset-filesystem-list", "preset-filesystem-read"}
		}
		if len(req.RequiredCapabilities) == 0 {
			req.RequiredCapabilities = []string{"architecture", "coding", "analysis"}
		}
	}

	req.RequiredTools = cleanStringList(req.RequiredTools)
	req.RequiredCapabilities = cleanStringList(req.RequiredCapabilities)
	return req
}

func normalizeConversationLabScenario(scenario string) string {
	switch strings.TrimSpace(strings.ToLower(scenario)) {
	case conversationLabScenarioSpeedChat:
		return conversationLabScenarioSpeedChat
	case conversationLabScenarioMemoryFollowUp:
		return conversationLabScenarioMemoryFollowUp
	default:
		return conversationLabScenarioGroundedRepoQA
	}
}

func buildBaselinePromptForScenario(req ConversationLabRequest) string {
	switch req.Scenario {
	case conversationLabScenarioSpeedChat:
		return "Answer directly as a plain chat completion with no tools, no routing explanation, and no extra setup.\n\n" + req.Prompt
	case conversationLabScenarioMemoryFollowUp:
		return "Treat this as a standalone follow-up with no prior memory or hidden context. Answer directly.\n\n" + req.Prompt
	default:
		return "Answer directly as a plain chat completion. Do not claim to execute tools or inspect files you cannot actually access.\n\n" + req.Prompt
	}
}

func buildOptimizedPromptForScenario(req ConversationLabRequest) string {
	switch req.Scenario {
	case conversationLabScenarioSpeedChat:
		return "Lab constraint: keep the optimized path lightweight. Avoid unnecessary tool calls and answer directly.\n\n" + req.Prompt
	case conversationLabScenarioMemoryFollowUp:
		return "Lab constraint: treat this as a follow-up. Reuse route memory if available, use the minimum necessary tool calls, and answer directly.\n\n" + req.Prompt
	default:
		return "Lab constraint: if tools are available, use the minimum number of tool calls needed to ground the answer, ideally one filesystem pass, and then answer directly.\n\n" + req.Prompt
	}
}

func labMaxTokens(scenario, mode string) int {
	switch scenario {
	case conversationLabScenarioSpeedChat:
		if mode == "baseline" {
			return 220
		}
		return 260
	case conversationLabScenarioMemoryFollowUp:
		if mode == "baseline" {
			return 320
		}
		return 360
	default:
		if mode == "baseline" {
			return 420
		}
		return 460
	}
}

func seedConversationLabMemoryScenario(ctx context.Context, client *llmux.Client, req ConversationLabRequest) {
	if client == nil {
		return
	}
	now := time.Now()
	query := strings.TrimSpace(req.Prompt)
	if query == "" {
		query = "Continue the earlier architecture discussion and tell me which subsystem should change first if we want to improve memory-aware routing quality."
	}

	seedReq := AgentChatRequest{
		SessionID:            req.SessionID,
		Messages:             []ConversationTurn{{Role: "user", Content: query}},
		ComplexityHint:       req.ComplexityHint,
		RequireTeam:          req.RequireTeam,
		RequiredTools:        req.RequiredTools,
		RequiredCapabilities: req.RequiredCapabilities,
		TokenOptimization:    req.TokenOptimization,
		CostOptimization:     req.CostOptimization,
		DisableToolInference: len(req.RequiredTools) == 0,
	}
	intent := inferConversationIntent(query)
	profile := buildRoutingProfile(seedReq, query, intent, nil)
	profile.MemoryReuseEnabled = false
	applyExperimentOptionsToProfile(&profile, req.Experiment)
	profile.MemoryReuseEnabled = false

	selectedAgent, routeSource, _, _, _ := selectConversationAgentForProfile(ctx, client, profile)
	var selectedTeam *AgentTeam
	var teamParticipants []ConversationAgent
	if team, participants, lead, _, teamRouteSource, _, _, ok := teamRouteConversationWithProfile(profile); ok {
		selectedTeam = team
		teamParticipants = participants
		selectedAgent = lead
		routeSource = teamRouteSource
	}

	record := RouteMemoryRecord{
		ID:              fmt.Sprintf("lab-seed-route-%d", now.UnixNano()),
		SessionID:       req.SessionID,
		Query:           query,
		Intent:          intent,
		SelectedAgentID: selectedAgent.ID,
		SelectedModel:   selectedAgent.Model,
		SelectedPath:    fmt.Sprintf("%s/%s/%s", selectedAgent.ID, selectedAgent.Provider, selectedAgent.Model),
		RouteSource:     "value-lab-seed:" + routeSource,
		AnswerPreview:   fmt.Sprintf("Seeded prior run for scenario=%s selected agent=%s with route source=%s.", req.Scenario, selectedAgent.ID, routeSource),
		Succeeded:       true,
		OutcomeScore:    0.96,
		CreatedAt:       now.Add(-2 * time.Minute),
	}
	persistConversationMemory(record)

	if selectedTeam != nil {
		participantIDs := make([]string, 0, len(teamParticipants))
		for _, item := range teamParticipants {
			participantIDs = append(participantIDs, item.ID)
		}
		teamRecord := TeamMemoryRecord{
			ID:               fmt.Sprintf("lab-seed-team-%d", now.UnixNano()),
			SessionID:        req.SessionID,
			Query:            query,
			Intent:           intent,
			SelectedTeamID:   selectedTeam.ID,
			SelectedAgentIDs: participantIDs,
			Summary:          fmt.Sprintf("Seeded prior team run for scenario=%s selected team=%s.", req.Scenario, selectedTeam.ID),
			Succeeded:        true,
			OutcomeScore:     0.94,
			CreatedAt:        now.Add(-2 * time.Minute),
		}
		persistTeamMemory(teamRecord)
	}
}

func (h *ManagementHandler) runConversationLabBaseline(ctx context.Context, client *llmux.Client, req ConversationLabRequest) ConversationLabRun {
	if req.Scenario == conversationLabScenarioMemoryFollowUp {
		return h.runConversationLabRouted(ctx, client, req, "baseline", false)
	}

	start := time.Now()
	providerName, modelName := selectConversationLabBaselineTarget(client, req.BaselineProvider, req.BaselineModel)
	run := ConversationLabRun{
		Mode:     "baseline",
		Provider: providerName,
		Model:    modelName,
	}
	prompt := buildBaselinePromptForScenario(req)

	chatReq := &llmux.ChatRequest{
		Model: modelName,
		Messages: []llmux.ChatMessage{{
			Role:    "user",
			Content: llmtypesMustQuote(prompt),
		}},
		Temperature: float64Ptr(0.2),
		MaxTokens:   labMaxTokens(req.Scenario, "baseline"),
		Tags:        []string{"value-lab", req.Scenario, "baseline", req.TrafficType},
		User:        "value-lab-baseline",
	}

	resp, trace, err := executeConversationChatCompletionWithTrace(ctx, client, chatReq)
	run.LatencyMs = int(time.Since(start).Milliseconds())
	if err != nil {
		run.ErrorMessage = err.Error()
		run.Quality = evaluateConversationLabQuality(run, req.Scenario, trace)
		return run
	}

	run.Succeeded = true
	applyLabChatMetrics(&run, client, modelName, resp)
	run.Quality = evaluateConversationLabQuality(run, req.Scenario, trace)
	return run
}

func (h *ManagementHandler) runConversationLabOptimized(ctx context.Context, client *llmux.Client, req ConversationLabRequest) ConversationLabRun {
	return h.runConversationLabRouted(ctx, client, req, "optimized", true)
}

func (h *ManagementHandler) runConversationLabRouted(ctx context.Context, client *llmux.Client, req ConversationLabRequest, mode string, memoryReuseEnabled bool) ConversationLabRun {
	start := time.Now()
	run := ConversationLabRun{Mode: mode}
	labPrompt := buildOptimizedPromptForScenario(req)
	if mode == "baseline" && req.Scenario == conversationLabScenarioMemoryFollowUp {
		labPrompt = "Lab constraint: compare against the same routed chain, but do not reuse prior route or team memory.\n\n" + req.Prompt
	}

	chatReq := AgentChatRequest{
		SessionID:            req.SessionID,
		Messages:             []ConversationTurn{{Role: "user", Content: labPrompt}},
		ComplexityHint:       req.ComplexityHint,
		RequireTeam:          req.RequireTeam,
		RequiredTools:        req.RequiredTools,
		RequiredCapabilities: req.RequiredCapabilities,
		TokenOptimization:    req.TokenOptimization,
		CostOptimization:     req.CostOptimization,
		DisableToolInference: req.Scenario == conversationLabScenarioSpeedChat && len(req.RequiredTools) == 0,
	}

	lastUserMessage := labPrompt
	intent := inferConversationIntent(lastUserMessage)
	profile := buildRoutingProfile(chatReq, lastUserMessage, intent, auth.GetAuthContext(ctx))
	applyExperimentOptionsToProfile(&profile, req.Experiment)
	if len(req.RequiredTools) > 0 {
		profile.RequiredTools = cleanStringList(req.RequiredTools)
	}
	if len(req.RequiredCapabilities) > 0 {
		profile.RequiredCapabilities = cleanStringList(req.RequiredCapabilities)
	}
	if req.RequireTeam != nil {
		profile.RequireTeam = *req.RequireTeam
	}
	profile.MemoryReuseEnabled = memoryReuseEnabled

	selectedAgent, routeSource, reasons, hit, consulted := selectConversationAgentForProfile(ctx, client, profile)
	var selectedTeam *AgentTeam
	var teamConsulted []TeamMemoryRecord
	var teamMemoryHit *TeamMemoryRecord
	if team, _, lead, teamReasons, teamRouteSource, consultedTeamMemory, hitTeamMemory, ok := teamRouteConversationWithProfile(profile); ok {
		selectedTeam = team
		teamConsulted = consultedTeamMemory
		teamMemoryHit = hitTeamMemory
		selectedAgent = lead
		routeSource = teamRouteSource
		reasons = append(teamReasons, reasons...)
	}

	memoryInfluence := "none"
	if len(consulted) > 0 || len(teamConsulted) > 0 {
		memoryInfluence = "consulted"
	}
	if hit != nil || teamMemoryHit != nil {
		memoryInfluence = "reused"
	}

	optimizationPlan := buildConversationOptimizationPlan(profile)
	optimizationNotes := []string{}
	if optimizationPlan.Enabled {
		var messageNotes []string
		chatReq.Messages, messageNotes = optimizeConversationTurns(chatReq.Messages, true)
		optimizationNotes = append(optimizationNotes, messageNotes...)
		var costNotes []string
		selectedAgent, costNotes = optimizeAgentForCost(selectedAgent, true)
		optimizationNotes = append(optimizationNotes, costNotes...)
		optimizationNotes = append(optimizationNotes, optimizationPlan.Notes...)
	}

	selectedAgent, executionTrail := prepareAgentExecution(selectedAgent)
	reasons = append(reasons, executionTrail...)
	marketplaceTools := resolveMarketplaceTools(selectedAgent.Tools)
	toolSummaries := resolveMarketplaceToolSummaries(selectedAgent.Tools)
	if !profile.ToolInjectionEnabled {
		selectedAgent.Tools = nil
		marketplaceTools = nil
		toolSummaries = nil
	}
	if optimizationPlan.Enabled && profile.ToolScopePruningEnabled {
		var toolOptimizationNotes []string
		toolSummaries, marketplaceTools, toolOptimizationNotes = filterToolSummariesForRequiredTools(toolSummaries, marketplaceTools, profile.RequiredTools)
		optimizationNotes = append(optimizationNotes, toolOptimizationNotes...)
	}
	gatewayReq := buildConversationGatewayRequest(chatReq.Messages, selectedAgent, toolSummaries, marketplaceTools)
	if optimizationPlan.Enabled {
		gatewayReq = buildConversationGatewayRequestOptimized(chatReq.Messages, selectedAgent, toolSummaries, marketplaceTools, optimizationPlan)
	}
	gatewayReq.MaxTokens = labMaxTokens(req.Scenario, "optimized")

	execCtx := ctx
	if profile.MaxToolIterations > 0 {
		execCtx = mcp.WithMaxToolIterations(execCtx, profile.MaxToolIterations)
	}
	if profile.ToolInjectionEnabled && len(marketplaceTools) > 0 {
		execCtx = mcp.WithIncludeTools(execCtx, marketplaceTools)
	}
	if manager := mcp.GetManager(execCtx); manager != nil && profile.ToolInjectionEnabled && len(marketplaceTools) > 0 {
		gatewayReq.Tools = manager.GetAvailableTools(execCtx)
	}

	var modelAccess *auth.ModelAccess
	if h.store != nil {
		access, err := auth.NewModelAccess(ctx, h.store, auth.GetAuthContext(ctx))
		if err != nil {
			run.ErrorMessage = "failed to evaluate model access"
			run.LatencyMs = int(time.Since(start).Milliseconds())
			return run
		}
		modelAccess = access
	}

	resp, selectedCandidate, failovers, trace, err := executeConversationWithProfileCandidatesForLab(execCtx, client, gatewayReq, selectedAgent, profile, modelAccess)
	run.LatencyMs = int(time.Since(start).Milliseconds())
	run.MemoryInfluence = memoryInfluence
	run.RouteSource = routeSource
	run.SelectedAgentID = selectedAgent.ID
	run.RequiredTools = profile.RequiredTools
	run.BoundTools = append([]string(nil), selectedAgent.Tools...)
	run.RoutingReasoning = append([]string(nil), reasons...)
	run.CandidateFailovers = append([]CandidateFailover(nil), failovers...)
	run.UsedMemory = memoryInfluence != "none"
	run.UsedTeam = selectedTeam != nil
	run.TokenOptimizationEnabled = profile.TokenOptimizationEnabled

	if selectedTeam != nil {
		run.SelectedTeamID = selectedTeam.ID
	}
	if selectedCandidate != nil {
		run.SelectedCandidate = selectedCandidate.Provider + "/" + selectedCandidate.Model
		run.Provider = selectedCandidate.Provider
		run.Model = selectedCandidate.Model
	} else {
		run.Provider = selectedAgent.Provider
		run.Model = selectedAgent.Model
		if run.Provider != "" && run.Model != "" {
			run.SelectedCandidate = run.Provider + "/" + run.Model
		}
	}

	if err != nil {
		run.ErrorMessage = err.Error()
		run.OptimizationNotes = cleanStringList(optimizationNotes)
		run.Quality = evaluateConversationLabQuality(run, req.Scenario, trace)
		return run
	}

	run.Succeeded = true
	applyLabChatMetrics(&run, client, run.Model, resp)
	run.OptimizationNotes = cleanStringList(optimizationNotes)
	run.Quality = evaluateConversationLabQuality(run, req.Scenario, trace)
	return run
}

func applyLabChatMetrics(run *ConversationLabRun, client *llmux.Client, fallbackModel string, resp *llmux.ChatResponse) {
	if run == nil || resp == nil {
		return
	}

	if run.Model == "" {
		run.Model = resp.Model
	}
	if resp.Usage != nil && resp.Usage.Provider != "" {
		run.Provider = resp.Usage.Provider
	}
	run.ResponseText = extractAssistantText(resp)
	run.ResponseChars = len([]rune(run.ResponseText))
	run.FinishReason = extractFinishReason(resp)

	if resp.Usage == nil {
		return
	}

	run.PromptTokens = resp.Usage.PromptTokens
	run.CompletionTokens = resp.Usage.CompletionTokens
	run.TotalTokens = resp.Usage.TotalTokens
	run.EstimatedCost = client.CalculateCost(canonicalChatModel(fallbackModel), resp.Usage)
}

func executeConversationWithProfileCandidatesForLab(ctx context.Context, client *llmux.Client, gatewayReq *llmux.ChatRequest, agent ConversationAgent, profile RoutingProfile, access *auth.ModelAccess) (*llmux.ChatResponse, *CandidateModel, []CandidateFailover, conversationExecutionTrace, error) {
	candidates := rankCandidateModelsForProfile(agent, profile, access)
	if len(candidates) == 0 {
		resp, trace, err := executeConversationChatCompletionWithTrace(ctx, client, gatewayReq)
		return resp, nil, nil, trace, err
	}

	trail := make([]CandidateFailover, 0, len(candidates))
	var lastErr error
	var lastTrace conversationExecutionTrace

	for _, candidate := range candidates {
		predictedState := describeCandidateStateForProfile(candidate, profile, access)
		if shouldPreSkipCandidate(profile.CandidateStrategy, predictedState.Status) {
			switch predictedState.Status {
			case "unauthorized":
				trail = append(trail, CandidateFailover{Provider: candidate.Provider, Model: candidate.Model, Outcome: "skipped", Reason: predictedState.DecisionReason})
				updateCandidateHealth(candidate, predictedState, false, false, predictedState.DecisionReason)
				lastErr = llmerrors.NewPermissionError(candidate.Provider, candidate.Model, predictedState.DecisionReason)
				continue
			case "predicted_saturated":
				trail = append(trail, CandidateFailover{Provider: candidate.Provider, Model: candidate.Model, Outcome: "skipped", Reason: predictedState.DecisionReason})
				updateCandidateHealth(candidate, predictedState, false, true, predictedState.DecisionReason)
				lastErr = llmerrors.NewRateLimitError(candidate.Provider, candidate.Model, predictedState.DecisionReason)
				continue
			case "cooling_down":
				trail = append(trail, CandidateFailover{Provider: candidate.Provider, Model: candidate.Model, Outcome: "skipped", Reason: predictedState.DecisionReason})
				updateCandidateHealth(candidate, predictedState, false, false, predictedState.DecisionReason)
				lastErr = llmerrors.NewRateLimitError(candidate.Provider, candidate.Model, predictedState.DecisionReason)
				continue
			}
		}

		candidateReq := *gatewayReq
		candidateReq.Model = candidate.Provider + "/" + candidate.Model
		resp, trace, err := executeConversationChatCompletionWithTrace(ctx, client, &candidateReq)
		lastTrace = trace
		if err == nil {
			recordCandidateUsage(candidate, profile.EstimatedTokens)
			trail = append(trail, CandidateFailover{Provider: candidate.Provider, Model: candidate.Model, Outcome: "selected"})
			selectedState := describeCandidateStateForProfile(candidate, profile, access)
			selectedState.Status = "ready"
			updateCandidateHealth(candidate, selectedState, true, false, "")
			return resp, &candidate, trail, trace, nil
		}

		lastErr = err
		if isCandidateThrottled(err) {
			markCandidateCooldown(candidate, 75*time.Second)
			trail = append(trail, CandidateFailover{Provider: candidate.Provider, Model: candidate.Model, Outcome: "throttled", Reason: err.Error()})
			throttledState := describeCandidateStateForProfile(candidate, profile, access)
			throttledState.Status = "cooling_down"
			throttledState.DecisionReason = err.Error()
			updateCandidateHealth(candidate, throttledState, false, true, err.Error())
			continue
		}

		trail = append(trail, CandidateFailover{Provider: candidate.Provider, Model: candidate.Model, Outcome: "failed", Reason: err.Error()})
		failedState := describeCandidateStateForProfile(candidate, profile, access)
		failedState.Status = "degraded"
		failedState.DecisionReason = err.Error()
		updateCandidateHealth(candidate, failedState, false, false, err.Error())
		return nil, nil, trail, trace, err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no candidate models available for agent=%s", agent.ID)
	}
	return nil, nil, trail, lastTrace, lastErr
}

func extractFinishReason(resp *llmux.ChatResponse) string {
	if resp == nil || len(resp.Choices) == 0 {
		return ""
	}
	return strings.TrimSpace(resp.Choices[0].FinishReason)
}

func canonicalChatModel(model string) string {
	_, canonical := llmtypes.SplitProviderModel(strings.TrimSpace(model))
	if canonical != "" {
		return canonical
	}
	return strings.TrimSpace(model)
}

func selectConversationLabBaselineTarget(client *llmux.Client, preferredProvider, preferredModel string) (string, string) {
	if client == nil {
		return "", preferredModel
	}

	preferredProvider = strings.TrimSpace(preferredProvider)
	preferredModel = strings.TrimSpace(preferredModel)
	if providerName, modelName := splitPreferredTarget(preferredProvider, preferredModel); providerName != "" && modelName != "" {
		if prov, ok := client.GetProvider(providerName); ok && prov != nil && prov.SupportsModel(modelName) {
			return providerName, modelName
		}
	}

	if prov, ok := client.GetProvider("deepseek-primary"); ok && prov != nil {
		models := prov.SupportedModels()
		if sliceContainsString(models, "deepseek-chat") {
			return "deepseek-primary", "deepseek-chat"
		}
		if len(models) > 0 {
			return "deepseek-primary", models[0]
		}
	}

	providers := client.GetProviders()
	sort.Strings(providers)
	for _, providerName := range providers {
		prov, ok := client.GetProvider(providerName)
		if !ok || prov == nil {
			continue
		}
		models := prov.SupportedModels()
		if len(models) == 0 {
			continue
		}
		return providerName, models[0]
	}

	return preferredProvider, preferredModel
}

func splitPreferredTarget(preferredProvider, preferredModel string) (string, string) {
	if preferredProvider != "" && preferredModel != "" {
		return preferredProvider, canonicalChatModel(preferredModel)
	}
	if preferredModel == "" {
		return "", ""
	}
	providerName, modelName := llmtypes.SplitProviderModel(preferredModel)
	if providerName == "" {
		return preferredProvider, canonicalChatModel(preferredModel)
	}
	return providerName, modelName
}

func evaluateConversationLabQuality(run ConversationLabRun, scenario string, trace conversationExecutionTrace) ConversationLabQuality {
	quality := ConversationLabQuality{
		Label:      "Optimized routed answer",
		RouteDepth: 1,
		MemoryHit:  run.UsedMemory,
		TeamRoute:  run.UsedTeam,
		ToolCalls:  trace.ToolCalls,
		ToolNames:  cleanStringList(trace.ToolNames),
	}
	if run.Mode == "baseline" && run.RouteSource == "" {
		quality.Label = "Ungrounded direct answer"
	} else if run.Mode == "baseline" {
		quality.Label = "Routed baseline without memory reuse"
	}
	quality.ToolUsed = quality.ToolCalls > 0

	if run.RouteSource != "" {
		quality.RouteDepth++
	}
	if run.SelectedAgentID != "" {
		quality.RouteDepth++
	}
	if run.SelectedCandidate != "" {
		quality.RouteDepth++
	}
	if quality.TeamRoute {
		quality.RouteDepth++
	}
	if quality.MemoryHit {
		quality.RouteDepth++
	}
	if quality.ToolUsed {
		quality.RouteDepth++
	}

	if !run.Succeeded {
		quality.GroundednessScore = 0
		quality.HallucinationRisk = 1
		quality.QualityNotes = append(quality.QualityNotes, "The run did not complete successfully, so answer quality cannot be trusted.")
		if trace.MaxIterationsHit {
			quality.QualityNotes = append(quality.QualityNotes, "The optimized chain exceeded the maximum tool-iteration budget.")
		}
		if run.ErrorMessage != "" {
			quality.QualityNotes = append(quality.QualityNotes, run.ErrorMessage)
		}
		return quality
	}

	groundedness := 0.34
	hallucinationRisk := 0.62
	if run.Mode != "baseline" {
		groundedness += 0.10
		hallucinationRisk -= 0.08
	}

	lowerResponse := strings.ToLower(run.ResponseText)
	if quality.MemoryHit {
		groundedness += 0.06
		hallucinationRisk -= 0.04
		quality.QualityNotes = append(quality.QualityNotes, "The optimized path reused prior route memory when selecting the execution path.")
	}
	if quality.TeamRoute {
		groundedness += 0.04
		hallucinationRisk -= 0.02
		quality.QualityNotes = append(quality.QualityNotes, "The request escalated into a team route instead of a single direct answer.")
	}
	if quality.ToolUsed {
		groundedness += 0.30
		hallucinationRisk -= 0.24
		quality.QualityNotes = append(quality.QualityNotes, fmt.Sprintf("Observed %d tool call(s): %s.", quality.ToolCalls, strings.Join(quality.ToolNames, ", ")))
	} else if len(run.RequiredTools) > 0 {
		hallucinationRisk += 0.08
		quality.QualityNotes = append(quality.QualityNotes, "The scenario declared required tools, but no tool calls were observed.")
	}

	if hasRepoEvidence(lowerResponse) {
		groundedness += 0.14
		hallucinationRisk -= 0.08
		quality.QualityNotes = append(quality.QualityNotes, "The answer references concrete repository artifacts instead of speaking in generic terms.")
	}

	if strings.Contains(lowerResponse, "```bash") || strings.Contains(lowerResponse, "ls -la") || strings.Contains(lowerResponse, "find .") {
		if !quality.ToolUsed {
			groundedness -= 0.20
			hallucinationRisk += 0.24
			quality.QualityNotes = append(quality.QualityNotes, "The answer simulates shell inspection without verified tool execution.")
		}
	}

	if run.FinishReason == "length" {
		hallucinationRisk += 0.05
		quality.QualityNotes = append(quality.QualityNotes, "The answer hit the max-token cap, so the output may be truncated.")
	}

	if scenario == conversationLabScenarioSpeedChat && run.Mode == "baseline" {
		groundedness += 0.04
		hallucinationRisk -= 0.04
	}

	quality.GroundednessScore = clamp01(groundedness)
	quality.HallucinationRisk = clamp01(hallucinationRisk)
	return quality
}

func compareWinningDimensions(scenario string, baseline, optimized ConversationLabRun) []string {
	dimensions := make([]string, 0, 8)
	if optimized.UsedMemory {
		dimensions = append(dimensions, "memory-aware")
	}
	if optimized.UsedTeam {
		dimensions = append(dimensions, "team-route")
	}
	if optimized.SelectedAgentID != "" {
		dimensions = append(dimensions, "agent-selection")
	}
	if optimized.SelectedCandidate != "" && optimized.SelectedCandidate != baseline.Provider+"/"+baseline.Model {
		dimensions = append(dimensions, "model-routing")
	}
	if optimized.Quality.ToolUsed && !baseline.Quality.ToolUsed {
		dimensions = append(dimensions, "tool-aware")
	}
	if optimized.Quality.GroundednessScore > baseline.Quality.GroundednessScore+0.12 {
		dimensions = append(dimensions, "groundedness")
	}
	if optimized.Quality.HallucinationRisk+0.10 < baseline.Quality.HallucinationRisk {
		dimensions = append(dimensions, "hallucination-control")
	}
	if scenario == conversationLabScenarioSpeedChat && baseline.LatencyMs < optimized.LatencyMs {
		dimensions = append(dimensions, "baseline-speed")
	}
	if optimized.Succeeded && !baseline.Succeeded {
		dimensions = append(dimensions, "reliability")
	}
	return cleanStringList(dimensions)
}

func recommendConversationLabWinner(scenario string, baseline, optimized ConversationLabRun) string {
	switch scenario {
	case conversationLabScenarioSpeedChat:
		if baseline.Succeeded && baseline.LatencyMs <= optimized.LatencyMs {
			return "baseline"
		}
		if optimized.Succeeded {
			return "optimized"
		}
	default:
		if optimized.Succeeded && (optimized.Quality.ToolUsed || optimized.Quality.GroundednessScore >= baseline.Quality.GroundednessScore+0.12 || optimized.UsedMemory || optimized.UsedTeam) {
			return "optimized"
		}
		if baseline.Succeeded {
			return "baseline"
		}
	}
	return "needs-investigation"
}

func buildConversationLabValueSummary(scenario string, baseline, optimized ConversationLabRun) []string {
	summary := make([]string, 0, 10)

	switch {
	case optimized.Succeeded && !baseline.Succeeded:
		summary = append(summary, "The optimized chain completed while the plain baseline path failed.")
	case optimized.Succeeded && baseline.Succeeded:
		summary = append(summary, "Both paths completed, so the lab can compare speed, grounding, routing depth, and answer quality instead of just success/failure.")
	default:
		summary = append(summary, "This run still needs inspection before it can be used as a clean value demo.")
	}
	if scenario == conversationLabScenarioMemoryFollowUp {
		summary = append(summary, "In this scenario, the baseline path uses the same routed execution style but disables route-memory reuse, so the delta isolates memory value more clearly.")
	}

	if optimized.Quality.ToolUsed && !baseline.Quality.ToolUsed {
		summary = append(summary, fmt.Sprintf("The optimized path executed %d real tool call(s), while the baseline answer remained an ungrounded direct response.", optimized.Quality.ToolCalls))
	}
	if optimized.Quality.GroundednessScore > baseline.Quality.GroundednessScore+0.12 {
		summary = append(summary, fmt.Sprintf("Groundedness improved from %.2f to %.2f.", baseline.Quality.GroundednessScore, optimized.Quality.GroundednessScore))
	}
	if optimized.Quality.HallucinationRisk+0.10 < baseline.Quality.HallucinationRisk {
		summary = append(summary, fmt.Sprintf("Hallucination risk dropped from %.2f to %.2f.", baseline.Quality.HallucinationRisk, optimized.Quality.HallucinationRisk))
	}
	if optimized.UsedMemory {
		summary = append(summary, fmt.Sprintf("The optimized chain reused route memory (memory influence=%s) instead of treating the prompt as stateless.", optimized.MemoryInfluence))
	}
	if optimized.UsedTeam {
		summary = append(summary, fmt.Sprintf("The optimized chain escalated to team=%s, which the baseline path cannot do.", optimized.SelectedTeamID))
	}
	if optimized.SelectedAgentID != "" {
		summary = append(summary, fmt.Sprintf("The optimized path explicitly selected agent=%s and candidate=%s.", optimized.SelectedAgentID, optimized.SelectedCandidate))
	}
	if scenario == conversationLabScenarioSpeedChat && baseline.LatencyMs < optimized.LatencyMs {
		summary = append(summary, fmt.Sprintf("For simple prompts, the baseline path won on raw speed by %d ms.", optimized.LatencyMs-baseline.LatencyMs))
	} else if optimized.LatencyMs < baseline.LatencyMs {
		summary = append(summary, fmt.Sprintf("Optimized routing reduced end-to-end latency by %d ms.", baseline.LatencyMs-optimized.LatencyMs))
	} else if optimized.LatencyMs > baseline.LatencyMs {
		summary = append(summary, fmt.Sprintf("Optimized routing spent %d extra ms to buy better routing depth, tools, or memory behavior.", optimized.LatencyMs-baseline.LatencyMs))
	}
	if optimized.ResponseChars > baseline.ResponseChars {
		summary = append(summary, fmt.Sprintf("Optimized routing produced a richer answer with %d additional response characters.", optimized.ResponseChars-baseline.ResponseChars))
	}
	if optimized.TotalTokens != baseline.TotalTokens {
		summary = append(summary, fmt.Sprintf("Token consumption delta: %+d total tokens.", optimized.TotalTokens-baseline.TotalTokens))
	}
	if optimized.EstimatedCost != baseline.EstimatedCost {
		summary = append(summary, fmt.Sprintf("Estimated cost delta: %+0.6f.", optimized.EstimatedCost-baseline.EstimatedCost))
	}

	return summary
}

func hasRepoEvidence(response string) bool {
	keywords := []string{
		"go.mod",
		"readme.md",
		"dockerfile",
		"llmux.go",
		"client.go",
		"internal/",
		"providers/",
		"routers/",
		"ui/",
		"config/",
	}
	matches := 0
	for _, keyword := range keywords {
		if strings.Contains(response, keyword) {
			matches++
		}
	}
	return matches >= 2
}

func llmtypesMustQuote(value string) []byte {
	quoted, _ := json.Marshal(value)
	return quoted
}

func boolPtr(v bool) *bool { return &v }

func sliceContainsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
