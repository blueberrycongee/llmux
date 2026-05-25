package api

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/mcp"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
)

const (
	memoryConsultThreshold  = 0.45
	memoryReuseThreshold    = 0.80
	memoryDefaultTTL        = 30 * 24 * time.Hour
	memoryHighScoreTTL      = 90 * 24 * time.Hour
	candidateSafetyFraction = 0.90
)

type RoutingProfile struct {
	Query                    string
	Intent                   string
	QueryTokens              []string
	Complexity               int
	RequireTeam              bool
	RequiredTools            []string
	RequiredCapabilities     []string
	LatencyBudgetMS          int
	EstimatedTokens          int
	TenantID                 string
	OrganizationID           string
	TokenOptimizationEnabled bool
	MemoryReuseEnabled       bool
	TeamRoutingEnabled       bool
	CandidateStrategy        string
	ToolInjectionEnabled     bool
	ToolScopePruningEnabled  bool
	MaxToolIterations        int
	ExperimentID             string
	ExperimentGroup          string
	BaselineName             string
}

func buildRoutingProfile(req AgentChatRequest, query, intent string, authCtx *auth.AuthContext) RoutingProfile {
	tenantID, orgID := resolveRoutingTenantScope(req.TenantID, req.OrganizationID, authCtx)
	requiredTools := cleanStringList(req.RequiredTools)
	if !req.DisableToolInference {
		requiredTools = cleanStringList(append(requiredTools, inferRequiredTools(query)...))
	}
	requiredCapabilities := cleanStringList(append(req.RequiredCapabilities, inferRequiredCapabilities(query, intent)...))
	estimatedTokens := estimateConversationTokens(req.Messages)
	forceTeam := req.RequireTeam != nil && *req.RequireTeam
	complexity := inferTaskComplexity(query, intent, estimatedTokens, req.ComplexityHint, forceTeam, requiredTools)
	requireTeam := forceTeam || complexity >= 3
	latencyBudget := inferLatencyBudget(intent, complexity, estimatedTokens, req.LatencyBudgetMS)
	profile := RoutingProfile{
		Query:                    query,
		Intent:                   intent,
		QueryTokens:              tokenizeQuery(query),
		Complexity:               complexity,
		RequireTeam:              requireTeam,
		RequiredTools:            requiredTools,
		RequiredCapabilities:     requiredCapabilities,
		LatencyBudgetMS:          latencyBudget,
		EstimatedTokens:          estimatedTokens,
		TenantID:                 tenantID,
		OrganizationID:           orgID,
		TokenOptimizationEnabled: req.TokenOptimization || req.CostOptimization,
		MemoryReuseEnabled:       true,
		TeamRoutingEnabled:       true,
		CandidateStrategy:        candidateStrategyPredictive,
		ToolInjectionEnabled:     true,
		ToolScopePruningEnabled:  true,
		MaxToolIterations:        mcp.MaxToolIterations,
	}
	applyExperimentOptionsToProfile(&profile, req.Experiment)
	return profile
}

func resolveRoutingTenantScope(requestTenantID, requestOrgID string, authCtx *auth.AuthContext) (string, string) {
	tenantID := strings.TrimSpace(requestTenantID)
	orgID := strings.TrimSpace(requestOrgID)
	if authCtx == nil {
		return tenantID, orgID
	}
	if tenantID == "" {
		switch {
		case authCtx.Team != nil && authCtx.Team.ID != "":
			tenantID = authCtx.Team.ID
		case authCtx.APIKey != nil && authCtx.APIKey.TeamID != nil:
			tenantID = *authCtx.APIKey.TeamID
		case authCtx.User != nil && authCtx.User.TeamID != nil:
			tenantID = *authCtx.User.TeamID
		}
	}
	if orgID == "" {
		switch {
		case authCtx.Team != nil && authCtx.Team.OrganizationID != nil:
			orgID = *authCtx.Team.OrganizationID
		case authCtx.APIKey != nil && authCtx.APIKey.OrganizationID != nil:
			orgID = *authCtx.APIKey.OrganizationID
		case authCtx.User != nil && authCtx.User.OrganizationID != nil:
			orgID = *authCtx.User.OrganizationID
		}
	}
	return tenantID, orgID
}

func inferRequiredTools(query string) []string {
	text := strings.ToLower(query)
	tokens := tokenizeQuery(query)
	tools := make([]string, 0, 4)
	if containsAny(text, "web", "search", "browser", "latest", "http", "fetch") {
		tools = append(tools, "preset-fetch-web")
	}
	if containsAny(text, "browser", "playwright", "navigate", "click", "page automation") {
		tools = append(tools, "preset-browser-open")
	}
	if containsAny(text, "file", "repo", "read", "source", "filesystem", "directory") {
		tools = append(tools, "preset-filesystem-read", "preset-filesystem-list")
	}
	if containsAny(text, "sql", "sqlite", "schema", "query", "database") {
		tools = append(tools, "preset-sqlite-query")
	}
	if containsAny(text, "github", "repo", "pull request") || containsAnyToken(tokens, "pr", "issue", "issues") {
		tools = append(tools, "preset-github-repo")
	}
	return cleanStringList(tools)
}

func inferRequiredCapabilities(query, intent string) []string {
	text := strings.ToLower(query)
	capabilities := make([]string, 0, 4)
	if intent != "" {
		capabilities = append(capabilities, intent)
	}
	if containsAny(text, "architecture", "system design", "design", "roadmap", "plan", "implementation", "refactor") {
		capabilities = append(capabilities, "architecture", "planning")
	}
	if containsAny(text, "research", "thesis", "experiment", "abstract", "question", "analysis") {
		capabilities = append(capabilities, "research", "analysis", "thesis")
	}
	if containsAny(text, "code", "debug", "bug", "go", "golang", "sql", "api") {
		capabilities = append(capabilities, "coding", "debugging", "go", "sql")
	}
	if containsAny(text, "translate", "rewrite", "writing", "editing", "english") {
		capabilities = append(capabilities, "writing", "translation", "editing")
	}
	return cleanStringList(capabilities)
}

func inferTaskComplexity(query, intent string, estimatedTokens, hint int, forceTeam bool, requiredTools []string) int {
	if hint > 0 {
		if forceTeam && hint < 3 {
			return 3
		}
		if hint > 4 {
			return 4
		}
		return hint
	}
	complexity := 1
	text := strings.ToLower(query)
	if intent == "coding" || intent == "research" {
		complexity++
	}
	if containsAny(text, "team", "collaboration", "multi-agent", "rehearsal", "workflow") {
		complexity += 2
	}
	if containsAny(text, "architecture", "system design", "refactor", "gateway", "thesis", "experiment", "roadmap", "plan", "algorithm") {
		complexity++
	}
	if estimatedTokens > 1000 || len([]rune(strings.TrimSpace(query))) > 220 {
		complexity++
	}
	if len(requiredTools) >= 2 {
		complexity++
	}
	if forceTeam && complexity < 3 {
		complexity = 3
	}
	if complexity < 1 {
		return 1
	}
	if complexity > 4 {
		return 4
	}
	return complexity
}

func inferLatencyBudget(intent string, complexity, estimatedTokens, explicitBudget int) int {
	if explicitBudget > 0 {
		return explicitBudget
	}
	budget := intentLatencyBudget(intent)
	if budget <= 0 {
		budget = 1800
	}
	if complexity >= 3 {
		budget += 500
	}
	if estimatedTokens > 1200 {
		budget += 400
	}
	return budget
}

func scopeMatches(scopes []string, tenantID, orgID string) bool {
	if len(scopes) == 0 {
		return true
	}
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	orgID = strings.ToLower(strings.TrimSpace(orgID))
	for _, scope := range scopes {
		normalized := strings.ToLower(strings.TrimSpace(scope))
		switch {
		case normalized == "", normalized == "*":
			return true
		case strings.HasPrefix(normalized, "team:") && tenantID != "" && strings.TrimPrefix(normalized, "team:") == tenantID:
			return true
		case strings.HasPrefix(normalized, "tenant:") && tenantID != "" && strings.TrimPrefix(normalized, "tenant:") == tenantID:
			return true
		case strings.HasPrefix(normalized, "org:") && orgID != "" && strings.TrimPrefix(normalized, "org:") == orgID:
			return true
		case !strings.Contains(normalized, ":") && ((tenantID != "" && normalized == tenantID) || (orgID != "" && normalized == orgID)):
			return true
		}
	}
	return false
}

func toolCoverageScore(available, required []string) float64 {
	if len(required) == 0 {
		return 1
	}
	availableSet := map[string]struct{}{}
	for _, item := range available {
		availableSet[strings.ToLower(strings.TrimSpace(item))] = struct{}{}
	}
	matched := 0
	for _, item := range required {
		if _, ok := availableSet[strings.ToLower(strings.TrimSpace(item))]; ok {
			matched++
		}
	}
	return clamp01(float64(matched) / float64(len(required)))
}

func capabilityCoverageScore(capabilities []string, profile RoutingProfile, category string) float64 {
	capabilitySet := map[string]struct{}{}
	for _, capability := range capabilities {
		capabilitySet[strings.ToLower(strings.TrimSpace(capability))] = struct{}{}
	}
	if category != "" {
		capabilitySet[strings.ToLower(strings.TrimSpace(category))] = struct{}{}
	}
	if len(profile.RequiredCapabilities) == 0 {
		if profile.Intent == "" {
			return 0.5
		}
		if _, ok := capabilitySet[strings.ToLower(profile.Intent)]; ok {
			return 1
		}
	}
	matched := 0
	for _, capability := range profile.RequiredCapabilities {
		if _, ok := capabilitySet[strings.ToLower(strings.TrimSpace(capability))]; ok {
			matched++
		}
	}
	capabilityFit := 0.0
	if len(profile.RequiredCapabilities) > 0 {
		capabilityFit = float64(matched) / float64(len(profile.RequiredCapabilities))
	}
	queryFit := tokenOverlapScore(profile.QueryTokens, keysFromSet(capabilitySet))
	return clamp01(0.65*capabilityFit + 0.35*queryFit)
}

func containsAnyToken(tokens []string, candidates ...string) bool {
	if len(tokens) == 0 || len(candidates) == 0 {
		return false
	}
	set := map[string]struct{}{}
	for _, token := range tokens {
		set[strings.ToLower(strings.TrimSpace(token))] = struct{}{}
	}
	for _, candidate := range candidates {
		if _, ok := set[strings.ToLower(strings.TrimSpace(candidate))]; ok {
			return true
		}
	}
	return false
}

func keysFromSet(set map[string]struct{}) []string {
	items := make([]string, 0, len(set))
	for key := range set {
		items = append(items, key)
	}
	sort.Strings(items)
	return items
}

func scoreRouteMemorySimilarity(profile RoutingProfile, record RouteMemoryRecord) float64 {
	queryScore := tokenOverlapScore(profile.QueryTokens, tokenizeQuery(record.Query))
	summaryScore := tokenOverlapScore(profile.QueryTokens, tokenizeQuery(record.AnswerPreview+" "+record.CompressedSummary))
	hintScore := tokenOverlapScore(profile.QueryTokens, tokenizeQuery(strings.Join(record.RetrievalHints, " ")))
	representativeScore := tokenOverlapScore(profile.QueryTokens, tokenizeQuery(strings.Join(record.RepresentativeQueries, " ")))
	score := 0.40*queryScore + 0.25*summaryScore + 0.20*hintScore + 0.15*representativeScore
	if record.Intent == profile.Intent && profile.Intent != "" {
		score += 0.12
	}
	if record.SourceCount > 1 {
		score += math.Min(0.08, float64(record.SourceCount-1)*0.02)
	}
	return clamp01(score)
}

func scoreTeamMemorySimilarity(profile RoutingProfile, record TeamMemoryRecord) float64 {
	queryScore := tokenOverlapScore(profile.QueryTokens, tokenizeQuery(record.Query))
	summaryScore := tokenOverlapScore(profile.QueryTokens, tokenizeQuery(record.Summary+" "+record.CompressedSummary))
	hintScore := tokenOverlapScore(profile.QueryTokens, tokenizeQuery(strings.Join(record.RetrievalHints, " ")))
	representativeScore := tokenOverlapScore(profile.QueryTokens, tokenizeQuery(strings.Join(record.RepresentativeQueries, " ")))
	score := 0.40*queryScore + 0.25*summaryScore + 0.20*hintScore + 0.15*representativeScore
	if record.Intent == profile.Intent && profile.Intent != "" {
		score += 0.12
	}
	if record.SourceCount > 1 {
		score += math.Min(0.08, float64(record.SourceCount-1)*0.02)
	}
	return clamp01(score)
}

func memoryTTL(outcomeScore float64) time.Duration {
	if outcomeScore >= 0.90 {
		return memoryHighScoreTTL
	}
	return memoryDefaultTTL
}

func conversationMemorySnapshot() []RouteMemoryRecord {
	conversationMemoryStore.Lock()
	defer conversationMemoryStore.Unlock()
	pruneRouteMemoryLocked(time.Now())
	items := rebuildRouteMemoryRecordsLocked()
	snapshot := make([]RouteMemoryRecord, len(items))
	copy(snapshot, items)
	return snapshot
}

func teamMemorySnapshot() []TeamMemoryRecord {
	teamMemoryStore.Lock()
	defer teamMemoryStore.Unlock()
	pruneTeamMemoryLocked(time.Now())
	items := rebuildTeamMemoryRecordsLocked()
	snapshot := make([]TeamMemoryRecord, len(items))
	copy(snapshot, items)
	return snapshot
}

func findRelevantMemoryCandidatesForProfile(profile RoutingProfile, limit int) []RouteMemoryRecord {
	if !profile.MemoryReuseEnabled {
		return nil
	}
	if len(profile.QueryTokens) == 0 || limit <= 0 {
		return nil
	}
	type candidate struct {
		record RouteMemoryRecord
		score  float64
	}
	items := conversationMemorySnapshot()
	matches := make([]candidate, 0, limit)
	for _, item := range items {
		if !item.Succeeded {
			continue
		}
		score := scoreRouteMemorySimilarity(profile, item)
		if score < memoryConsultThreshold {
			continue
		}
		matches = append(matches, candidate{record: item, score: score})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			if matches[i].record.SourceCount == matches[j].record.SourceCount {
				return matches[i].record.OutcomeScore > matches[j].record.OutcomeScore
			}
			return matches[i].record.SourceCount > matches[j].record.SourceCount
		}
		return matches[i].score > matches[j].score
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	result := make([]RouteMemoryRecord, 0, len(matches))
	for _, item := range matches {
		result = append(result, item.record)
	}
	return result
}

func findRelevantTeamMemoryForProfile(profile RoutingProfile, limit int) []TeamMemoryRecord {
	if !profile.MemoryReuseEnabled {
		return nil
	}
	if len(profile.QueryTokens) == 0 || limit <= 0 {
		return nil
	}
	type candidate struct {
		record TeamMemoryRecord
		score  float64
	}
	items := teamMemorySnapshot()
	matches := make([]candidate, 0, limit)
	for _, item := range items {
		if !item.Succeeded {
			continue
		}
		score := scoreTeamMemorySimilarity(profile, item)
		if score < memoryConsultThreshold {
			continue
		}
		matches = append(matches, candidate{record: item, score: score})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			if matches[i].record.SourceCount == matches[j].record.SourceCount {
				return matches[i].record.OutcomeScore > matches[j].record.OutcomeScore
			}
			return matches[i].record.SourceCount > matches[j].record.SourceCount
		}
		return matches[i].score > matches[j].score
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	result := make([]TeamMemoryRecord, 0, len(matches))
	for _, item := range matches {
		result = append(result, item.record)
	}
	return result
}

func bestConversationMemoryForAgent(profile RoutingProfile, agentID string, consulted []RouteMemoryRecord) (*RouteMemoryRecord, float64) {
	bestWeight := 0.0
	var best *RouteMemoryRecord
	for i := range consulted {
		if consulted[i].SelectedAgentID != agentID {
			continue
		}
		similarity := scoreRouteMemorySimilarity(profile, consulted[i])
		if similarity < memoryConsultThreshold {
			continue
		}
		maturity := 0.0
		if consulted[i].SourceCount > 1 {
			maturity = math.Min(1, float64(consulted[i].SourceCount-1)/4)
		}
		reuseEvidence := math.Min(1, float64(consulted[i].ReuseCount)/4)
		weight := clamp01(0.45*consulted[i].OutcomeScore + 0.30*similarity + 0.15*maturity + 0.10*reuseEvidence)
		if weight > bestWeight {
			bestWeight = weight
			copied := consulted[i]
			best = &copied
		}
	}
	return best, bestWeight
}

func bestTeamMemoryForTeam(profile RoutingProfile, teamID string, consulted []TeamMemoryRecord) (*TeamMemoryRecord, float64) {
	bestWeight := 0.0
	var best *TeamMemoryRecord
	for i := range consulted {
		if consulted[i].SelectedTeamID != teamID {
			continue
		}
		similarity := scoreTeamMemorySimilarity(profile, consulted[i])
		if similarity < memoryConsultThreshold {
			continue
		}
		maturity := 0.0
		if consulted[i].SourceCount > 1 {
			maturity = math.Min(1, float64(consulted[i].SourceCount-1)/4)
		}
		reuseEvidence := math.Min(1, float64(consulted[i].ReuseCount)/4)
		weight := clamp01(0.45*consulted[i].OutcomeScore + 0.30*similarity + 0.15*maturity + 0.10*reuseEvidence)
		if weight > bestWeight {
			bestWeight = weight
			copied := consulted[i]
			best = &copied
		}
	}
	return best, bestWeight
}

func selectConversationAgentByScore(profile RoutingProfile) (ConversationAgent, []string, *RouteMemoryRecord, []RouteMemoryRecord, bool) {
	consulted := findRelevantMemoryCandidatesForProfile(profile, 3)
	type candidate struct {
		agent        ConversationAgent
		score        float64
		match        float64
		toolCoverage float64
		latencyFit   float64
		memoryWeight float64
		hit          *RouteMemoryRecord
	}
	scoreCandidates := func(requireAllTools bool) []candidate {
		items := make([]candidate, 0, len(getConversationAgents()))
		for _, agent := range getConversationAgents() {
			if !agent.Enabled || !scopeMatches(agent.TenantScopes, profile.TenantID, profile.OrganizationID) {
				continue
			}
			availableTools := effectiveAgentTools(agent, profile)
			match := capabilityCoverageScore(agent.Capabilities, profile, agent.Category)
			toolCoverage := toolCoverageScore(availableTools, profile.RequiredTools)
			if requireAllTools && len(profile.RequiredTools) > 0 && toolCoverage < 1 {
				continue
			}
			latencyFit := agentLatencyFitWithBudget(agent, profile.LatencyBudgetMS)
			hit, memoryWeight := bestConversationMemoryForAgent(profile, agent.ID, consulted)
			score := 0.25*agentBaseScore(agent, availableTools) + 0.35*match + 0.20*memoryWeight + 0.20*latencyFit
			if !requireAllTools && len(profile.RequiredTools) > 0 {
				score = 0.20*agentBaseScore(agent, availableTools) + 0.30*match + 0.20*memoryWeight + 0.15*latencyFit + 0.15*toolCoverage
			}
			items = append(items, candidate{
				agent:        agent,
				score:        score,
				match:        match,
				toolCoverage: toolCoverage,
				latencyFit:   latencyFit,
				memoryWeight: memoryWeight,
				hit:          hit,
			})
		}
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].score == items[j].score {
				if items[i].match == items[j].match {
					return items[i].agent.ID < items[j].agent.ID
				}
				return items[i].match > items[j].match
			}
			return items[i].score > items[j].score
		})
		return items
	}

	candidates := scoreCandidates(true)
	degraded := false
	if len(candidates) == 0 {
		candidates = scoreCandidates(false)
		degraded = len(candidates) > 0 && len(profile.RequiredTools) > 0
	}
	if len(candidates) == 0 {
		return ConversationAgent{}, nil, nil, consulted, false
	}
	best := candidates[0]
	reasons := []string{
		fmt.Sprintf("routing_profile complexity=%d latency_budget_ms=%d", profile.Complexity, profile.LatencyBudgetMS),
		fmt.Sprintf("agent score-router selected %s with capability_match=%.2f latency_fit=%.2f", best.agent.ID, best.match, best.latencyFit),
	}
	if len(profile.RequiredTools) > 0 {
		reasons = append(reasons, fmt.Sprintf("required_tools=%s tool_coverage=%.2f", strings.Join(profile.RequiredTools, ","), best.toolCoverage))
	}
	if best.hit != nil && best.memoryWeight >= memoryReuseThreshold {
		reasons = append(reasons, fmt.Sprintf("reused route memory agent=%s outcome_score=%.2f", best.hit.SelectedAgentID, best.hit.OutcomeScore))
	} else if len(consulted) > 0 {
		reasons = append(reasons, fmt.Sprintf("consulted_route_memory=%d", len(consulted)))
	}
	if degraded {
		reasons = append(reasons, "no agent satisfied all required tools; used best partial-coverage fallback")
	}
	return best.agent, reasons, best.hit, consulted, true
}

func findBestMemoryHitForProfile(profile RoutingProfile) *RouteMemoryRecord {
	if !profile.MemoryReuseEnabled {
		return nil
	}
	items := findRelevantMemoryCandidatesForProfile(profile, 1)
	if len(items) == 0 {
		return nil
	}
	item := items[0]
	if scoreRouteMemorySimilarity(profile, item) < memoryReuseThreshold {
		return nil
	}
	return &item
}

func selectConversationAgentForProfile(ctx context.Context, client *llmux.Client, profile RoutingProfile) (ConversationAgent, string, []string, *RouteMemoryRecord, []RouteMemoryRecord) {
	if agent, matchedBy, ok := resolveExplicitAgentTarget(profile.Query); ok && scopeMatches(agent.TenantScopes, profile.TenantID, profile.OrganizationID) {
		availableTools := effectiveAgentTools(agent, profile)
		reasons := []string{
			fmt.Sprintf("explicit agent target matched=%s", matchedBy),
			fmt.Sprintf("selected agent=%s provider=%s model=%s", agent.ID, agent.Provider, agent.Model),
		}
		if len(profile.RequiredTools) > 0 {
			reasons = append(reasons, fmt.Sprintf("required_tools=%s coverage=%.2f", strings.Join(profile.RequiredTools, ","), toolCoverageScore(availableTools, profile.RequiredTools)))
		}
		return experimentAwareAgent(agent, profile), "explicit-agent", reasons, nil, nil
	}
	if shouldUseFastPath(profile) {
		if agent, ok := findConversationAgent("general-orchestrator"); ok && scopeMatches(agent.TenantScopes, profile.TenantID, profile.OrganizationID) {
			return experimentAwareAgent(agent, profile), "token-fast-path", []string{
				"token optimization enabled and request complexity is low, so the router selected the lightweight fast path",
				fmt.Sprintf("selected agent=%s provider=%s model=%s", agent.ID, agent.Provider, agent.Model),
			}, nil, nil
		}
	}
	if agent, reasoning, memoryHit, consulted, ok := selectConversationAgentByScore(profile); ok {
		reasons := append([]string{"constraint-aware score router selected the agent"}, reasoning...)
		return experimentAwareAgent(agent, profile), "score-router", reasons, memoryHit, consulted
	}
	if agent, reasoning, memoryHit, consulted, ok := selectConversationAgentByLLM(ctx, client, profile.Query, profile.Intent); ok && scopeMatches(agent.TenantScopes, profile.TenantID, profile.OrganizationID) {
		reasons := append([]string{"llm router fallback selected the agent"}, reasoning...)
		return experimentAwareAgent(agent, profile), "autonomous-router", reasons, memoryHit, consulted
	}
	if hit := findBestMemoryHitForProfile(profile); hit != nil && hit.OutcomeScore >= 0.55 {
		if agent, ok := findConversationAgent(hit.SelectedAgentID); ok && scopeMatches(agent.TenantScopes, profile.TenantID, profile.OrganizationID) {
			return experimentAwareAgent(agent, profile), "memory-hit", []string{
				"reused a high-score route memory record",
				fmt.Sprintf("agent=%s model=%s", hit.SelectedAgentID, hit.SelectedModel),
			}, hit, []RouteMemoryRecord{*hit}
		}
	}

	agentID := mapIntentToAgent(profile.Intent)
	agent, _ := findConversationAgent(agentID)
	reasons := []string{
		fmt.Sprintf("intent fallback selected intent=%s", profile.Intent),
		fmt.Sprintf("selected agent=%s provider=%s model=%s", agent.ID, agent.Provider, agent.Model),
	}
	return experimentAwareAgent(agent, profile), "intent-router", reasons, nil, nil
}

func aggregateTeamTools(team AgentTeam, profile RoutingProfile) []string {
	tools := make([]string, 0, len(team.AgentIDs)*2)
	for _, agent := range resolveTeamAgents(team) {
		tools = append(tools, effectiveAgentTools(agent, profile)...)
	}
	return cleanStringList(tools)
}

func safeCandidateLimit(limit int) int {
	if limit <= 0 {
		return 0
	}
	safe := int(math.Floor(float64(limit) * candidateSafetyFraction))
	if safe < 1 {
		return limit
	}
	return safe
}

func candidateAuthorized(access *auth.ModelAccess, candidate CandidateModel) bool {
	if access == nil {
		return true
	}
	fullModel := strings.TrimSpace(candidate.Provider + "/" + candidate.Model)
	if access.Allows(fullModel) {
		return true
	}
	return access.Allows(candidate.Model)
}

func strategyEffectiveUsage(candidate CandidateModel, profile RoutingProfile) (int, int, string) {
	rpm, tpm := getCandidateUsage(candidate)
	switch normalizeCandidateStrategy(profile.CandidateStrategy) {
	case candidateStrategyGreedyCurrent:
		return rpm + 1, tpm + profile.EstimatedTokens, "current-snapshot"
	case candidateStrategyPassive:
		return rpm, tpm, "passive-fallback"
	default:
		predictedRPM, predictedTPM := predictCandidateUsage(candidate, profile.EstimatedTokens)
		return predictedRPM, predictedTPM, "predictive-window"
	}
}

func shouldPreSkipCandidate(strategy, status string) bool {
	switch status {
	case "unauthorized", "cooling_down":
		return true
	}
	if normalizeCandidateStrategy(strategy) == candidateStrategyPassive {
		return false
	}
	return status == "predicted_saturated"
}

func describeCandidateStateForProfile(candidate CandidateModel, profile RoutingProfile, access *auth.ModelAccess) CandidateModelState {
	rpm, tpm := getCandidateUsage(candidate)
	effectiveRPM, effectiveTPM, usageMode := strategyEffectiveUsage(candidate, profile)
	latencyMS := estimateCandidateLatency(candidate)
	latencyFit := candidateLatencyFitWithBudget(candidate, profile.LatencyBudgetMS)
	safeRPM := safeCandidateLimit(candidate.RPMLimit)
	safeTPM := safeCandidateLimit(candidate.TPMLimit)
	state := CandidateModelState{
		Provider:       candidate.Provider,
		Model:          candidate.Model,
		Weight:         candidate.Weight,
		RPMLimit:       candidate.RPMLimit,
		TPMLimit:       candidate.TPMLimit,
		SafeRPMLimit:   safeRPM,
		SafeTPMLimit:   safeTPM,
		CurrentRPM:     rpm,
		CurrentTPM:     tpm,
		PredictedRPM:   effectiveRPM,
		PredictedTPM:   effectiveTPM,
		LatencyMS:      latencyMS,
		LatencyFit:     latencyFit,
		Authorized:     candidateAuthorized(access, candidate),
		Status:         "ready",
		SelectionScore: 0,
	}

	rpmWindowLimit := safeRPM
	if rpmWindowLimit <= 0 {
		rpmWindowLimit = candidate.RPMLimit
	}
	tpmWindowLimit := safeTPM
	if tpmWindowLimit <= 0 {
		tpmWindowLimit = candidate.TPMLimit
	}
	rpmCapacityLeft := 1.0
	if rpmWindowLimit > 0 {
		rpmCapacityLeft = clamp01(1 - float64(effectiveRPM)/float64(rpmWindowLimit))
	}
	tpmCapacityLeft := 1.0
	if tpmWindowLimit > 0 {
		tpmCapacityLeft = clamp01(1 - float64(effectiveTPM)/float64(tpmWindowLimit))
	}
	state.RPMCapacityLeft = rpmCapacityLeft
	state.TPMCapacityLeft = tpmCapacityLeft
	weightScore := clamp01(candidate.Weight / 2)
	switch normalizeCandidateStrategy(profile.CandidateStrategy) {
	case candidateStrategyGreedyCurrent:
		state.SelectionScore = 0.20*weightScore + 0.40*rpmCapacityLeft + 0.25*tpmCapacityLeft + 0.15*latencyFit
	case candidateStrategyPassive:
		state.SelectionScore = candidate.Weight
	default:
		state.SelectionScore = 0.25*weightScore + 0.30*rpmCapacityLeft + 0.25*tpmCapacityLeft + 0.20*latencyFit
	}

	switch {
	case !state.Authorized:
		state.Status = "unauthorized"
		state.DecisionReason = "model access denied by tenant policy"
	case candidateOnCooldown(candidate):
		state.Status = "cooling_down"
		state.DecisionReason = "candidate is currently cooling down"
		if until, ok := candidateCooldownUntil(candidate); ok {
			state.CooldownUntil = until.Format(time.RFC3339)
		}
	case normalizeCandidateStrategy(profile.CandidateStrategy) != candidateStrategyPassive && safeRPM > 0 && effectiveRPM > safeRPM:
		state.Status = "predicted_saturated"
		state.DecisionReason = fmt.Sprintf("%s rpm=%d exceeds safe rpm=%d", usageMode, effectiveRPM, safeRPM)
	case normalizeCandidateStrategy(profile.CandidateStrategy) != candidateStrategyPassive && safeTPM > 0 && effectiveTPM > safeTPM:
		state.Status = "predicted_saturated"
		state.DecisionReason = fmt.Sprintf("%s tpm=%d exceeds safe tpm=%d", usageMode, effectiveTPM, safeTPM)
	case profile.LatencyBudgetMS > 0 && latencyMS > profile.LatencyBudgetMS:
		state.Status = "latency_exceeded"
		state.DecisionReason = fmt.Sprintf("estimated latency=%d exceeds budget=%d", latencyMS, profile.LatencyBudgetMS)
	case normalizeCandidateStrategy(profile.CandidateStrategy) != candidateStrategyPassive && (rpmCapacityLeft < 0.20 || tpmCapacityLeft < 0.20):
		state.Status = "warm"
		state.DecisionReason = "candidate is approaching the safe utilization boundary"
	default:
		state.DecisionReason = "candidate is ready within current strategy constraints"
	}

	if stored := getStoredCandidateHealth(candidate); stored != nil {
		state.SelectedCount = stored.SelectedCount
		state.FailoverCount = stored.FailoverCount
		state.LastError = stored.LastError
		if state.CooldownUntil == "" {
			state.CooldownUntil = stored.CooldownUntil
		}
	}
	return state
}

func rankCandidateModelsForProfile(agent ConversationAgent, profile RoutingProfile, access *auth.ModelAccess) []CandidateModel {
	items := cleanCandidateModels(agent)
	if normalizeCandidateStrategy(profile.CandidateStrategy) == candidateStrategyPassive {
		return items
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := describeCandidateStateForProfile(items[i], profile, access)
		right := describeCandidateStateForProfile(items[j], profile, access)
		if left.Status != right.Status {
			return stateRank(left.Status) < stateRank(right.Status)
		}
		if left.SelectionScore == right.SelectionScore {
			return items[i].Weight > items[j].Weight
		}
		return left.SelectionScore > right.SelectionScore
	})
	return items
}

func buildCandidateStatesForProfile(agent ConversationAgent, profile RoutingProfile, access *auth.ModelAccess) []CandidateModelState {
	candidates := cleanCandidateModels(agent)
	states := make([]CandidateModelState, 0, len(candidates))
	for _, candidate := range candidates {
		states = append(states, describeCandidateStateForProfile(candidate, profile, access))
	}
	return states
}

func executeConversationWithProfileCandidates(ctx context.Context, client *llmux.Client, gatewayReq *llmux.ChatRequest, agent ConversationAgent, profile RoutingProfile, access *auth.ModelAccess) (*llmux.ChatResponse, *CandidateModel, []CandidateFailover, conversationExecutionTrace, error) {
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
