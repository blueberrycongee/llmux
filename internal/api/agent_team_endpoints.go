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
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Mode        string   `json:"mode"`
	AgentIDs    []string `json:"agent_ids"`
	Enabled     bool     `json:"enabled"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type TeamMemoryRecord struct {
	ID               string    `json:"id"`
	SessionID        string    `json:"session_id"`
	Query            string    `json:"query"`
	Intent           string    `json:"intent"`
	SelectedTeamID   string    `json:"selected_team_id"`
	SelectedAgentIDs []string  `json:"selected_agent_ids"`
	Summary          string    `json:"summary"`
	Succeeded        bool      `json:"succeeded"`
	OutcomeScore     float64   `json:"outcome_score"`
	CreatedAt        time.Time `json:"created_at"`
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
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Mode        string   `json:"mode"`
	AgentIDs    []string `json:"agent_ids"`
	Enabled     *bool    `json:"enabled"`
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
	records []TeamMemoryRecord
}{records: []TeamMemoryRecord{}}

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
	teamMemoryStore.RLock()
	defer teamMemoryStore.RUnlock()
	items := make([]TeamMemoryRecord, len(teamMemoryStore.records))
	copy(items, teamMemoryStore.records)
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
		updated := AgentTeam{ID: req.ID, Name: req.Name, Description: req.Description, Mode: req.Mode, AgentIDs: req.AgentIDs, Enabled: enabled, CreatedAt: team.CreatedAt, UpdatedAt: now}
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
	created := AgentTeam{ID: req.ID, Name: req.Name, Description: req.Description, Mode: req.Mode, AgentIDs: req.AgentIDs, Enabled: enabled, CreatedAt: now, UpdatedAt: now}
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

func selectAgentTeam(query, intent string) (*AgentTeam, []TeamMemoryRecord, *TeamMemoryRecord) {
	consulted := findRelevantTeamMemory(query, intent, 3)
	if len(consulted) > 0 {
		if team, ok := findAgentTeam(consulted[0].SelectedTeamID); ok {
			return &team, consulted, &consulted[0]
		}
	}
	for _, team := range getAgentTeams() {
		if !team.Enabled {
			continue
		}
		if team.ID == "thesis-lab-team" && intent == "research" {
			copied := team
			return &copied, consulted, nil
		}
		if team.ID == "system-design-team" && containsAny(strings.ToLower(query), "系统", "架构", "gateway", "网关", "设计", "代码") {
			copied := team
			return &copied, consulted, nil
		}
	}
	return nil, consulted, nil
}

func chooseLeadAgentFromTeam(team AgentTeam, _ string, intent string) (ConversationAgent, []ConversationAgent) {
	participants := resolveTeamAgents(team)
	for _, agent := range participants {
		if agent.Category == intent {
			return agent, participants
		}
	}
	if len(participants) > 0 {
		return participants[0], participants
	}
	return ConversationAgent{}, nil
}

func findRelevantTeamMemory(query, intent string, limit int) []TeamMemoryRecord {
	queryTokens := tokenizeQuery(query)
	if len(queryTokens) == 0 || limit <= 0 {
		return nil
	}
	type candidate struct {
		record TeamMemoryRecord
		score  float64
	}
	teamMemoryStore.RLock()
	defer teamMemoryStore.RUnlock()
	matches := make([]candidate, 0, limit)
	for _, item := range teamMemoryStore.records {
		if !item.Succeeded {
			continue
		}
		score := tokenOverlapScore(queryTokens, tokenizeQuery(item.Query))
		if score < 0.34 {
			continue
		}
		if item.Intent == intent {
			score += 0.08
		}
		matches = append(matches, candidate{record: item, score: score})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].record.OutcomeScore > matches[j].record.OutcomeScore
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

func persistTeamMemory(record TeamMemoryRecord) TeamMemoryRecord {
	teamMemoryStore.Lock()
	defer teamMemoryStore.Unlock()
	teamMemoryStore.records = append([]TeamMemoryRecord{record}, teamMemoryStore.records...)
	if len(teamMemoryStore.records) > 60 {
		teamMemoryStore.records = teamMemoryStore.records[:60]
	}
	return record
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
	if score > 0.98 {
		return 0.98
	}
	return score
}

func teamRouteConversation(query, intent string) (*AgentTeam, []ConversationAgent, ConversationAgent, []string, string, []TeamMemoryRecord, *TeamMemoryRecord, bool) {
	if !shouldRouteToTeam(query, intent) {
		return nil, nil, ConversationAgent{}, nil, "", nil, nil, false
	}
	team, consulted, hit := selectAgentTeam(query, intent)
	if team == nil {
		return nil, nil, ConversationAgent{}, nil, "", consulted, hit, false
	}
	lead, participants := chooseLeadAgentFromTeam(*team, query, intent)
	if lead.ID == "" {
		return nil, nil, ConversationAgent{}, nil, "", consulted, hit, false
	}
	reasons := []string{fmt.Sprintf("检测到复杂任务，先路由到专家团队=%s", team.ID), fmt.Sprintf("团队模式=%s，参与 agent 数量=%d", team.Mode, len(participants)), fmt.Sprintf("根据 intent=%s 选择 lead agent=%s", intent, lead.ID)}
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
