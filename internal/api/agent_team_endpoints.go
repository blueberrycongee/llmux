package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type AgentTeam struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Mode         string   `json:"mode"`
	AgentIDs     []string `json:"agent_ids"`
	TenantScopes []string `json:"tenant_scopes,omitempty"`
	Enabled      bool     `json:"enabled"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type TeamMemoryRecord struct {
	ID                    string    `json:"id"`
	SessionID             string    `json:"session_id"`
	Query                 string    `json:"query"`
	Intent                string    `json:"intent"`
	SelectedTeamID        string    `json:"selected_team_id"`
	SelectedAgentIDs      []string  `json:"selected_agent_ids"`
	Summary               string    `json:"summary"`
	CompressedSummary     string    `json:"compressed_summary,omitempty"`
	DistilledLearnings    []string  `json:"distilled_learnings,omitempty"`
	RepresentativeQueries []string  `json:"representative_queries,omitempty"`
	RetrievalHints        []string  `json:"retrieval_hints,omitempty"`
	MemoryStage           string    `json:"memory_stage,omitempty"`
	SourceCount           int       `json:"source_count,omitempty"`
	SuccessCount          int       `json:"success_count,omitempty"`
	ConsultCount          int       `json:"consult_count,omitempty"`
	ReuseCount            int       `json:"reuse_count,omitempty"`
	CompressionRatio      float64   `json:"compression_ratio,omitempty"`
	FirstRecordedAt       time.Time `json:"first_recorded_at,omitempty"`
	LastReinforcedAt      time.Time `json:"last_reinforced_at,omitempty"`
	Succeeded             bool      `json:"succeeded"`
	OutcomeScore          float64   `json:"outcome_score"`
	CreatedAt             time.Time `json:"created_at"`
}

type teamSelectionMetrics struct {
	CapabilityMatch float64
	MemoryWeight    float64
}

type TeamRehearsalStep struct {
	AgentID           string          `json:"agent_id"`
	AgentName         string          `json:"agent_name"`
	SelectedCandidate *CandidateModel `json:"selected_candidate,omitempty"`
	OutputPreview     string          `json:"output_preview"`
	Succeeded         bool            `json:"succeeded"`
	ErrorMessage      string          `json:"error_message,omitempty"`
}

type UpsertAgentTeamRequest struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Mode         string   `json:"mode"`
	AgentIDs     []string `json:"agent_ids"`
	TenantScopes []string `json:"tenant_scopes"`
	Enabled      *bool    `json:"enabled"`
}

type DeleteAgentTeamRequest struct {
	ID string `json:"id"`
}

type AgentTeamRehearsalRequest struct {
	SessionID string `json:"session_id"`
	TeamID    string `json:"team_id"`
	Prompt    string `json:"prompt"`
}

type AgentTeamRehearsalResponse struct {
	RequestID           string              `json:"request_id"`
	SessionID           string              `json:"session_id"`
	SelectedTeam        AgentTeam           `json:"selected_team"`
	ParticipatingAgents []ConversationAgent `json:"participating_agents"`
	Steps               []TeamRehearsalStep `json:"steps"`
	RecordedMemory      TeamMemoryRecord    `json:"recorded_memory"`
	Succeeded           bool                `json:"succeeded"`
	GeneratedAt         string              `json:"generated_at"`
}

var agentTeamStore = struct {
	sync.RWMutex
	teams []AgentTeam
}{teams: defaultAgentTeams()}
var teamMemoryStore = struct {
	sync.RWMutex
	episodes []TeamMemoryRecord
	records  []TeamMemoryRecord
	stats    map[string]teamMemoryAccessStats
}{episodes: []TeamMemoryRecord{}, records: []TeamMemoryRecord{}, stats: map[string]teamMemoryAccessStats{}}

func defaultAgentTeams() []AgentTeam {
	now := time.Now().Format(time.RFC3339)
	return []AgentTeam{
		{ID: "thesis-lab-team", Name: "Thesis Lab Team", Description: "面向论文设计、研究问题提炼、实现路线与表达优化的专家团队。", Mode: "planner-reviewer", AgentIDs: []string{"research-strategist", "writer-translator", "code-specialist"}, Enabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "system-design-team", Name: "System Design Team", Description: "面向网关设计、架构拆解、代码实现与风险复盘的专家团队。", Mode: "planner-executor-reviewer", AgentIDs: []string{"code-specialist", "research-strategist", "general-orchestrator"}, Enabled: true, CreatedAt: now, UpdatedAt: now},
	}
}

func (h *ManagementHandler) ListAgentTeams(w http.ResponseWriter, _ *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]any{"data": getAgentTeams()})
}
func (h *ManagementHandler) CreateAgentTeam(w http.ResponseWriter, r *http.Request) {
	h.upsertAgentTeam(w, r, false)
}
func (h *ManagementHandler) UpdateAgentTeam(w http.ResponseWriter, r *http.Request) {
	h.upsertAgentTeam(w, r, true)
}

func (h *ManagementHandler) upsertAgentTeam(w http.ResponseWriter, r *http.Request, mustExist bool) {
	var req UpsertAgentTeamRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	team, err := upsertAgentTeam(req, mustExist)
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, team)
}

func (h *ManagementHandler) DeleteAgentTeam(w http.ResponseWriter, r *http.Request) {
	var req DeleteAgentTeamRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	agentTeamStore.Lock()
	defer agentTeamStore.Unlock()
	filtered := agentTeamStore.teams[:0]
	deleted := false
	for _, team := range agentTeamStore.teams {
		if team.ID == req.ID {
			deleted = true
			continue
		}
		filtered = append(filtered, team)
	}
	if !deleted {
		h.writeError(w, r, http.StatusNotFound, "agent team not found")
		return
	}
	agentTeamStore.teams = append([]AgentTeam(nil), filtered...)
	h.writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *ManagementHandler) ListTeamMemory(w http.ResponseWriter, _ *http.Request) {
	items := teamMemorySnapshot()
	h.writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func getAgentTeams() []AgentTeam {
	agentTeamStore.RLock()
	defer agentTeamStore.RUnlock()
	items := make([]AgentTeam, len(agentTeamStore.teams))
	copy(items, agentTeamStore.teams)
	return items
}

func findAgentTeam(id string) (AgentTeam, bool) {
	for _, team := range getAgentTeams() {
		if team.ID == id && team.Enabled {
			return team, true
		}
	}
	return AgentTeam{}, false
}

func upsertAgentTeam(req UpsertAgentTeamRequest, mustExist bool) (AgentTeam, error) {
	if strings.TrimSpace(req.ID) == "" || strings.TrimSpace(req.Name) == "" {
		return AgentTeam{}, fmt.Errorf("id and name are required")
	}
	if strings.TrimSpace(req.Mode) == "" {
		req.Mode = "collaborative"
	}
	req.AgentIDs = cleanStringList(req.AgentIDs)
	if len(req.AgentIDs) == 0 {
		return AgentTeam{}, fmt.Errorf("at least one agent_id is required")
	}
	now := time.Now().Format(time.RFC3339)
	agentTeamStore.Lock()
	defer agentTeamStore.Unlock()
	for i, team := range agentTeamStore.teams {
		if team.ID != req.ID {
			continue
		}
		enabled := team.Enabled
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		updated := AgentTeam{ID: req.ID, Name: req.Name, Description: req.Description, Mode: req.Mode, AgentIDs: req.AgentIDs, TenantScopes: cleanStringList(req.TenantScopes), Enabled: enabled, CreatedAt: team.CreatedAt, UpdatedAt: now}
		agentTeamStore.teams[i] = updated
		return updated, nil
	}
	if mustExist {
		return AgentTeam{}, fmt.Errorf("agent team not found")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	created := AgentTeam{ID: req.ID, Name: req.Name, Description: req.Description, Mode: req.Mode, AgentIDs: req.AgentIDs, TenantScopes: cleanStringList(req.TenantScopes), Enabled: enabled, CreatedAt: now, UpdatedAt: now}
	agentTeamStore.teams = append([]AgentTeam{created}, agentTeamStore.teams...)
	return created, nil
}

func resolveTeamAgents(team AgentTeam) []ConversationAgent {
	agents := make([]ConversationAgent, 0, len(team.AgentIDs))
	for _, id := range team.AgentIDs {
		if agent, ok := findConversationAgent(id); ok {
			agents = append(agents, agent)
		}
	}
	return agents
}

func shouldRouteToTeam(query, intent string) bool {
	text := strings.ToLower(query)
	if containsAny(text, "团队", "team", "协作", "多 agent", "multi-agent", "演练", "rehearsal", "一起", "联合") {
		return true
	}
	if containsAny(text, "system-design-team", "thesis-lab-team", "指定team", "team=") {
		return true
	}
	return (intent == "research" && containsAny(text, "方案", "论文", "架构", "路线", "实现", "对比", "设计", "研究问题", "贡献点", "实验")) ||
		(intent == "coding" && containsAny(text, "系统设计", "架构设计", "重构", "演进", "网关", "模块拆解", "技术方案"))
}

func selectAgentTeam(profile RoutingProfile) (*AgentTeam, []TeamMemoryRecord, *TeamMemoryRecord) {
	consulted := findRelevantTeamMemoryForProfile(profile, 3)
	type teamCandidate struct {
		team    AgentTeam
		score   float64
		metrics teamSelectionMetrics
		hit     *TeamMemoryRecord
	}
	candidates := make([]teamCandidate, 0, len(getAgentTeams()))
	for _, team := range getAgentTeams() {
		if !team.Enabled || !scopeMatches(team.TenantScopes, profile.TenantID, profile.OrganizationID) {
			continue
		}
		match := computeTeamCapabilityMatch(team, profile)
		if match <= 0.15 {
			continue
		}
		toolCoverage := toolCoverageScore(aggregateTeamTools(team, profile), profile.RequiredTools)
		if len(profile.RequiredTools) > 0 && toolCoverage < 1 {
			continue
		}
		hit, memoryWeight := bestTeamMemoryForTeam(profile, team.ID, consulted)
		score := 0.30*teamBaseScore(team) + 0.35*match + 0.20*memoryWeight + 0.15*toolCoverage
		candidates = append(candidates, teamCandidate{team: team, score: score, metrics: teamSelectionMetrics{CapabilityMatch: match, MemoryWeight: memoryWeight}, hit: hit})
	}
	if len(candidates) == 0 {
		return nil, consulted, nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].metrics.CapabilityMatch > candidates[j].metrics.CapabilityMatch
		}
		return candidates[i].score > candidates[j].score
	})
	best := candidates[0]
	return &best.team, consulted, best.hit
}

func chooseLeadAgentFromTeam(team AgentTeam, profile RoutingProfile) (ConversationAgent, []ConversationAgent) {
	participants := resolveTeamAgents(team)
	if len(participants) == 0 {
		return ConversationAgent{}, nil
	}
	consulted := findRelevantMemoryCandidatesForProfile(profile, 3)
	type leadCandidate struct {
		agent ConversationAgent
		score float64
	}
	candidates := make([]leadCandidate, 0, len(participants))
	for _, agent := range participants {
		match := capabilityCoverageScore(agent.Capabilities, profile, agent.Category)
		latencyFit := agentLatencyFitWithBudget(agent, profile.LatencyBudgetMS)
		_, memoryWeight := bestConversationMemoryForAgent(profile, agent.ID, consulted)
		score := 0.25*agentBaseScore(agent, effectiveAgentTools(agent, profile)) + 0.35*match + 0.20*memoryWeight + 0.20*latencyFit
		candidates = append(candidates, leadCandidate{agent: agent, score: score})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].agent.ID < candidates[j].agent.ID
		}
		return candidates[i].score > candidates[j].score
	})
	return candidates[0].agent, participants
}

func findRelevantTeamMemory(query, intent string, limit int) []TeamMemoryRecord {
	profile := RoutingProfile{
		Query:              query,
		Intent:             intent,
		QueryTokens:        tokenizeQuery(query),
		MemoryReuseEnabled: true,
	}
	return findRelevantTeamMemoryForProfile(profile, limit)
}

func persistTeamMemory(record TeamMemoryRecord) TeamMemoryRecord {
	teamMemoryStore.Lock()
	defer teamMemoryStore.Unlock()
	now := time.Now()
	episode := normalizeTeamMemoryEpisode(record)
	teamMemoryStore.episodes = append(teamMemoryStore.episodes, episode)
	pruneTeamMemoryLocked(now)
	records := rebuildTeamMemoryRecordsLocked()
	profile := RoutingProfile{
		Query:              episode.Query,
		Intent:             episode.Intent,
		QueryTokens:        tokenizeQuery(episode.Query),
		MemoryReuseEnabled: true,
	}
	best := episode
	bestScore := 0.0
	for _, item := range records {
		if item.SelectedTeamID != episode.SelectedTeamID {
			continue
		}
		score := scoreTeamMemorySimilarity(profile, item)
		if score >= bestScore {
			best = item
			bestScore = score
		}
	}
	return best
}

func scoreTeamOutcome(steps []TeamRehearsalStep) float64 {
	if len(steps) == 0 {
		return 0.4
	}
	success := 0
	for _, step := range steps {
		if step.Succeeded {
			success++
		}
	}
	score := 0.45 + float64(success)/float64(len(steps))*0.45
	if len(steps) >= 3 && success == len(steps) {
		score += 0.04
	}
	if score > 0.98 {
		return 0.98
	}
	return score
}

func teamBaseScore(team AgentTeam) float64 {
	switch team.Mode {
	case "planner-executor-reviewer":
		return 0.92
	case "planner-reviewer":
		return 0.88
	default:
		return 0.8
	}
}

func computeTeamCapabilityMatch(team AgentTeam, profile RoutingProfile) float64 {
	participants := resolveTeamAgents(team)
	if len(participants) == 0 {
		return 0
	}
	capabilities := make([]string, 0, len(participants)*4)
	for _, agent := range participants {
		capabilities = append(capabilities, agent.Capabilities...)
		if agent.Category != "" {
			capabilities = append(capabilities, agent.Category)
		}
	}
	return capabilityCoverageScore(capabilities, profile, "")
}

func agentBaseScore(agent ConversationAgent, availableTools []string) float64 {
	score := 0.76
	if len(availableTools) > 0 {
		score += 0.08
	}
	if len(agent.CandidateModels) > 1 {
		score += 0.06
	}
	if agent.Category == "research" || agent.Category == "coding" {
		score += 0.05
	}
	return clamp01(score)
}

func computeAgentCapabilityMatch(agent ConversationAgent, queryTokens []string, intent string) float64 {
	capabilitySet := map[string]struct{}{}
	for _, capability := range agent.Capabilities {
		capabilitySet[strings.ToLower(strings.TrimSpace(capability))] = struct{}{}
	}
	capabilitySet[strings.ToLower(agent.Category)] = struct{}{}
	if intent != "" {
		if _, ok := capabilitySet[strings.ToLower(intent)]; ok {
			return 1
		}
	}
	if len(queryTokens) == 0 {
		return 0.5
	}
	matches := 0
	for _, token := range queryTokens {
		if _, ok := capabilitySet[token]; ok {
			matches++
		}
	}
	return clamp01(float64(matches) / float64(len(queryTokens)))
}

func agentLatencyFit(agent ConversationAgent, intent string) float64 {
	budget := intentLatencyBudget(intent)
	return agentLatencyFitWithBudget(agent, budget)
}

func agentLatencyFitWithBudget(agent ConversationAgent, budget int) float64 {
	latency := estimateAgentLatency(agent)
	if budget <= 0 {
		return 0.5
	}
	fit := 1 - float64(latency)/float64(budget)
	if fit < 0.05 {
		return 0.05
	}
	return clamp01(fit)
}

func estimateAgentLatency(agent ConversationAgent) int {
	latency := 850
	for _, candidate := range cleanCandidateModels(agent) {
		current := estimateCandidateLatency(candidate)
		if current < latency {
			latency = current
		}
	}
	return latency
}

func intentLatencyBudget(intent string) int {
	switch intent {
	case "general", "writing":
		return 1400
	case "coding", "research":
		return 2200
	default:
		return 1800
	}
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func teamRouteConversation(query, intent string) (*AgentTeam, []ConversationAgent, ConversationAgent, []string, string, []TeamMemoryRecord, *TeamMemoryRecord, bool) {
	profile := buildRoutingProfile(AgentChatRequest{
		Messages: []ConversationTurn{{Role: "user", Content: query}},
	}, query, intent, nil)
	return teamRouteConversationWithProfile(profile)
}

func teamRouteConversationWithProfile(profile RoutingProfile) (*AgentTeam, []ConversationAgent, ConversationAgent, []string, string, []TeamMemoryRecord, *TeamMemoryRecord, bool) {
	if !profile.RequireTeam || !profile.TeamRoutingEnabled {
		return nil, nil, ConversationAgent{}, nil, "", nil, nil, false
	}
	team, consulted, hit := selectAgentTeam(profile)
	if team == nil {
		return nil, nil, ConversationAgent{}, nil, "", consulted, hit, false
	}
	lead, participants := chooseLeadAgentFromTeam(*team, profile)
	if lead.ID == "" {
		return nil, nil, ConversationAgent{}, nil, "", consulted, hit, false
	}
	noteTeamMemoryConsulted(consulted)
	if hit != nil {
		noteTeamMemoryReused(hit)
	}
	reasons := []string{fmt.Sprintf("检测到复杂任务，先路由到专家团队=%s", team.ID), fmt.Sprintf("团队模式=%s，参与 agent 数量=%d", team.Mode, len(participants)), fmt.Sprintf("根据能力匹配与延迟适配选择 lead agent=%s", lead.ID)}
	if hit != nil {
		reasons = append(reasons, fmt.Sprintf("复用了 team memory，历史 team=%s score=%.2f", hit.SelectedTeamID, hit.OutcomeScore))
	} else if len(consulted) > 0 {
		reasons = append(reasons, fmt.Sprintf("参考了 %d 条 team memory", len(consulted)))
	}
	return team, participants, lead, reasons, "team-router", consulted, hit, true
}

func (h *ManagementHandler) RehearseAgentTeam(w http.ResponseWriter, r *http.Request) {
	var req AgentTeamRehearsalRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.TeamID) == "" || strings.TrimSpace(req.Prompt) == "" {
		h.writeError(w, r, http.StatusBadRequest, "team_id and prompt are required")
		return
	}
	if req.SessionID == "" {
		req.SessionID = fmt.Sprintf("team-session-%d", time.Now().UnixNano())
	}
	team, ok := findAgentTeam(req.TeamID)
	if !ok {
		h.writeError(w, r, http.StatusNotFound, "agent team not found")
		return
	}
	participants := resolveTeamAgents(team)
	client, release := h.acquireClient()
	defer release()
	if client == nil {
		h.writeError(w, r, http.StatusServiceUnavailable, "client not available")
		return
	}
	steps := make([]TeamRehearsalStep, 0, len(participants))
	estimatedTokens := estimateConversationTokens([]ConversationTurn{{Role: "user", Content: req.Prompt}})
	succeeded := false
	for _, agent := range participants {
		gatewayReq := buildConversationGatewayRequest([]ConversationTurn{{Role: "user", Content: fmt.Sprintf("[TEAM REHEARSAL:%s]\n%s", team.Name, req.Prompt)}}, agent, resolveMarketplaceToolSummaries(agent.Tools), resolveMarketplaceTools(agent.Tools))
		resp, candidate, _, err := executeConversationWithCandidates(r.Context(), client, gatewayReq, agent, estimatedTokens)
		step := TeamRehearsalStep{AgentID: agent.ID, AgentName: agent.Name, SelectedCandidate: candidate}
		if err != nil {
			step.ErrorMessage = err.Error()
		} else {
			step.Succeeded = true
			step.OutputPreview = truncateForPreview(extractAssistantText(resp), 220)
			succeeded = true
		}
		steps = append(steps, step)
	}
	summaryParts := make([]string, 0, len(steps))
	agentIDs := make([]string, 0, len(steps))
	for _, step := range steps {
		agentIDs = append(agentIDs, step.AgentID)
		if step.Succeeded {
			summaryParts = append(summaryParts, fmt.Sprintf("%s: %s", step.AgentName, step.OutputPreview))
		}
	}
	record := persistTeamMemory(TeamMemoryRecord{ID: fmt.Sprintf("team-memory-%d", time.Now().UnixNano()), SessionID: req.SessionID, Query: req.Prompt, Intent: inferConversationIntent(req.Prompt), SelectedTeamID: team.ID, SelectedAgentIDs: agentIDs, Summary: truncateForPreview(strings.Join(summaryParts, " | "), 280), Succeeded: succeeded, OutcomeScore: scoreTeamOutcome(steps), CreatedAt: time.Now()})
	h.writeJSON(w, http.StatusOK, AgentTeamRehearsalResponse{RequestID: fmt.Sprintf("team-rehearsal-%d", time.Now().UnixNano()), SessionID: req.SessionID, SelectedTeam: team, ParticipatingAgents: participants, Steps: steps, RecordedMemory: record, Succeeded: succeeded, GeneratedAt: time.Now().Format(time.RFC3339)})
}

// APPEND_MARKER
