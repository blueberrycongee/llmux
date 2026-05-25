package api //nolint:revive // package name is intentional

import (
	"context"
	stderrors "errors"
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
)

type CandidateModel struct {
	Provider string  `json:"provider"`
	Model    string  `json:"model"`
	Weight   float64 `json:"weight,omitempty"`
	RPMLimit int     `json:"rpm_limit,omitempty"`
	TPMLimit int     `json:"tpm_limit,omitempty"`
}

type CandidateModelState struct {
	Provider        string  `json:"provider"`
	Model           string  `json:"model"`
	Weight          float64 `json:"weight,omitempty"`
	RPMLimit        int     `json:"rpm_limit,omitempty"`
	TPMLimit        int     `json:"tpm_limit,omitempty"`
	SafeRPMLimit    int     `json:"safe_rpm_limit,omitempty"`
	SafeTPMLimit    int     `json:"safe_tpm_limit,omitempty"`
	CurrentRPM      int     `json:"current_rpm"`
	CurrentTPM      int     `json:"current_tpm"`
	PredictedRPM    int     `json:"predicted_rpm"`
	PredictedTPM    int     `json:"predicted_tpm"`
	RPMCapacityLeft float64 `json:"rpm_capacity_left,omitempty"`
	TPMCapacityLeft float64 `json:"tpm_capacity_left,omitempty"`
	LatencyMS       int     `json:"latency_ms,omitempty"`
	LatencyFit      float64 `json:"latency_fit,omitempty"`
	SelectionScore  float64 `json:"selection_score,omitempty"`
	Authorized      bool    `json:"authorized"`
	DecisionReason  string  `json:"decision_reason,omitempty"`
	Status          string  `json:"status"`
	SelectedCount   int     `json:"selected_count"`
	FailoverCount   int     `json:"failover_count"`
	LastError       string  `json:"last_error,omitempty"`
	CooldownUntil   string  `json:"cooldown_until,omitempty"`
}

type CandidateFailover struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Outcome  string `json:"outcome"`
	Reason   string `json:"reason,omitempty"`
}

type candidateUsageEvent struct {
	At     time.Time
	Tokens int
}

type ConversationAgent struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Description     string                `json:"description"`
	Category        string                `json:"category"`
	Provider        string                `json:"provider"`
	Model           string                `json:"model"`
	CandidateModels []CandidateModel      `json:"candidate_models,omitempty"`
	CandidateStates []CandidateModelState `json:"candidate_states,omitempty"`
	Strategy        string                `json:"strategy"`
	Capabilities    []string              `json:"capabilities"`
	SystemPrompt    string                `json:"system_prompt"`
	Accent          string                `json:"accent"`
	Tools           []string              `json:"tools,omitempty"`
	TenantScopes    []string              `json:"tenant_scopes,omitempty"`
	Enabled         bool                  `json:"enabled"`
	CreatedAt       string                `json:"created_at"`
	UpdatedAt       string                `json:"updated_at"`
}

type ConversationTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RouteMemoryRecord struct {
	ID                    string    `json:"id"`
	SessionID             string    `json:"session_id"`
	Query                 string    `json:"query"`
	Intent                string    `json:"intent"`
	SelectedAgentID       string    `json:"selected_agent_id"`
	SelectedModel         string    `json:"selected_model"`
	SelectedPath          string    `json:"selected_path"`
	RouteSource           string    `json:"route_source"`
	AnswerPreview         string    `json:"answer_preview"`
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

type AgentChatRequest struct {
	SessionID            string             `json:"session_id"`
	Messages             []ConversationTurn `json:"messages"`
	ComplexityHint       int                `json:"complexity_hint,omitempty"`
	RequireTeam          *bool              `json:"require_team,omitempty"`
	RequiredTools        []string           `json:"required_tools,omitempty"`
	RequiredCapabilities []string           `json:"required_capabilities,omitempty"`
	LatencyBudgetMS      int                `json:"latency_budget_ms,omitempty"`
	TenantID             string             `json:"tenant_id,omitempty"`
	OrganizationID       string             `json:"organization_id,omitempty"`
	TokenOptimization    bool               `json:"token_optimization,omitempty"`
	CostOptimization     bool               `json:"cost_optimization,omitempty"`
	DisableToolInference bool               `json:"disable_tool_inference,omitempty"`
	Experiment           ExperimentOptions  `json:"experiment,omitempty"`
}

type AgentChatResponse struct {
	RequestID                string                              `json:"request_id"`
	SessionID                string                              `json:"session_id"`
	Intent                   string                              `json:"intent"`
	SelectedTeam             *AgentTeam                          `json:"selected_team,omitempty"`
	TeamParticipants         []ConversationAgent                 `json:"team_participants,omitempty"`
	SelectedAgent            ConversationAgent                   `json:"selected_agent"`
	SelectedCandidate        *CandidateModel                     `json:"selected_candidate,omitempty"`
	CandidateFailovers       []CandidateFailover                 `json:"candidate_failovers,omitempty"`
	RouteSource              string                              `json:"route_source"`
	RoutingReasoning         []string                            `json:"routing_reasoning"`
	MemoryInfluence          string                              `json:"memory_influence,omitempty"`
	ConsultedMemories        []RouteMemoryRecord                 `json:"consulted_memories,omitempty"`
	MemoryHit                *RouteMemoryRecord                  `json:"memory_hit,omitempty"`
	TeamConsultedMemories    []TeamMemoryRecord                  `json:"team_consulted_memories,omitempty"`
	TeamMemoryHit            *TeamMemoryRecord                   `json:"team_memory_hit,omitempty"`
	GatewayRequest           map[string]any                      `json:"gateway_request"`
	GatewayResponse          any                                 `json:"gateway_response,omitempty"`
	AssistantMessage         string                              `json:"assistant_message,omitempty"`
	RecordedMemory           RouteMemoryRecord                   `json:"recorded_memory"`
	ExecutionTrace           *ConversationExecutionTraceResponse `json:"execution_trace,omitempty"`
	Succeeded                bool                                `json:"succeeded"`
	ErrorMessage             string                              `json:"error_message,omitempty"`
	TokenOptimizationEnabled bool                                `json:"token_optimization_enabled,omitempty"`
	OptimizationNotes        []string                            `json:"optimization_notes,omitempty"`
	GeneratedAt              string                              `json:"generated_at"`
}

type ConversationExecutionTraceResponse struct {
	Iterations       int      `json:"iterations"`
	ToolCalls        int      `json:"tool_calls"`
	ToolNames        []string `json:"tool_names,omitempty"`
	MaxIterationsHit bool     `json:"max_iterations_hit,omitempty"`
}

type AgentToolOption struct {
	ID       string `json:"id"`
	ClientID string `json:"client_id"`
	ToolName string `json:"tool_name"`
	Label    string `json:"label"`
}

type ToolMarketplaceItem struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Category       string   `json:"category"`
	SourceClientID string   `json:"source_client_id"`
	SourceToolName string   `json:"source_tool_name"`
	Tags           []string `json:"tags"`
	Enabled        bool     `json:"enabled"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

type conversationRequestBuildOptions struct {
	CompactToolInventory bool
	MaxTokens            int
}

type UpsertToolMarketplaceRequest struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Category       string   `json:"category"`
	SourceClientID string   `json:"source_client_id"`
	SourceToolName string   `json:"source_tool_name"`
	Tags           []string `json:"tags"`
	Enabled        *bool    `json:"enabled"`
}

type DeleteToolMarketplaceRequest struct {
	ID string `json:"id"`
}

type UpsertConversationAgentRequest struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	Category        string           `json:"category"`
	Provider        string           `json:"provider"`
	Model           string           `json:"model"`
	CandidateModels []CandidateModel `json:"candidate_models"`
	Strategy        string           `json:"strategy"`
	Capabilities    []string         `json:"capabilities"`
	SystemPrompt    string           `json:"system_prompt"`
	Accent          string           `json:"accent"`
	Tools           []string         `json:"tools"`
	TenantScopes    []string         `json:"tenant_scopes"`
	Enabled         *bool            `json:"enabled"`
}

type DeleteConversationAgentRequest struct {
	ID string `json:"id"`
}

var conversationAgentStore = struct {
	sync.RWMutex
	agents []ConversationAgent
}{agents: defaultConversationAgents()}

func defaultConversationAgents() []ConversationAgent {
	now := time.Now().Format(time.RFC3339)
	return []ConversationAgent{
		{
			ID:              "general-orchestrator",
			Name:            "General Orchestrator",
			Description:     "负责通用问答、总结和多轮对话收束，优先给出稳定、清晰、低延迟的回答。",
			Category:        "general",
			Provider:        "deepseek-primary",
			Model:           "deepseek-chat",
			CandidateModels: []CandidateModel{{Provider: "deepseek-primary", Model: "deepseek-chat", Weight: 1, RPMLimit: 24, TPMLimit: 24000}, {Provider: "deepseek-primary", Model: "deepseek-reasoner", Weight: 0.7, RPMLimit: 12, TPMLimit: 12000}},
			Strategy:        "lowest-latency",
			Capabilities:    []string{"general-chat", "summarization", "qa", "light-routing"},
			SystemPrompt:    "你是一个总控对话 Agent。请给出清晰、结构化、直接可用的中文回答；优先稳定、简洁、易执行。",
			Accent:          "cyan",
			Enabled:         true,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "code-specialist",
			Name:            "Code Specialist",
			Description:     "处理代码阅读、Bug 分析、系统设计、并发与工程实现问题。",
			Category:        "coding",
			Provider:        "deepseek-primary",
			Model:           "deepseek-reasoner",
			CandidateModels: []CandidateModel{{Provider: "deepseek-primary", Model: "deepseek-reasoner", Weight: 1, RPMLimit: 12, TPMLimit: 12000}, {Provider: "deepseek-primary", Model: "deepseek-chat", Weight: 0.6, RPMLimit: 24, TPMLimit: 24000}},
			Strategy:        "least-busy",
			Capabilities:    []string{"coding", "debugging", "architecture", "go", "sql"},
			SystemPrompt:    "你是代码专家 Agent。请对代码、系统设计、调试问题进行严谨推理，回答要具体、可执行，并尽量解释原因。",
			Accent:          "emerald",
			Tools:           []string{"preset-filesystem-list", "preset-filesystem-read", "preset-github-repo", "preset-sqlite-query"},
			Enabled:         true,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "research-strategist",
			Name:            "Research Strategist",
			Description:     "处理论文结构、研究问题、方案比较、路线设计和深度分析。",
			Category:        "research",
			Provider:        "deepseek-primary",
			Model:           "deepseek-reasoner",
			CandidateModels: []CandidateModel{{Provider: "deepseek-primary", Model: "deepseek-reasoner", Weight: 1, RPMLimit: 12, TPMLimit: 12000}, {Provider: "deepseek-primary", Model: "deepseek-chat", Weight: 0.75, RPMLimit: 24, TPMLimit: 24000}},
			Strategy:        "lowest-latency",
			Capabilities:    []string{"research", "planning", "analysis", "thesis", "strategy"},
			SystemPrompt:    "你是研究与规划 Agent。请优先给出结构化分析、选项比较、方案拆解和明确下一步建议。",
			Accent:          "amber",
			Tools:           []string{"preset-fetch-web", "preset-browser-open", "preset-filesystem-read", "preset-filesystem-list"},
			Enabled:         true,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "writer-translator",
			Name:            "Writer / Translator",
			Description:     "处理改写、润色、翻译、文案和表达优化。",
			Category:        "writing",
			Provider:        "deepseek-primary",
			Model:           "deepseek-chat",
			CandidateModels: []CandidateModel{{Provider: "deepseek-primary", Model: "deepseek-chat", Weight: 1, RPMLimit: 24, TPMLimit: 24000}, {Provider: "deepseek-primary", Model: "deepseek-reasoner", Weight: 0.55, RPMLimit: 12, TPMLimit: 12000}},
			Strategy:        "lowest-latency",
			Capabilities:    []string{"writing", "translation", "editing", "tone-shift"},
			SystemPrompt:    "你是写作与翻译 Agent。请关注表达质量、语言流畅性、风格控制和可读性。",
			Accent:          "fuchsia",
			Tools:           []string{"preset-fetch-web", "preset-filesystem-read"},
			Enabled:         true,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}
}

var toolMarketplaceStore = struct {
	sync.RWMutex
	items []ToolMarketplaceItem
}{items: defaultMarketplaceItems()}

var conversationMemoryStore = struct {
	sync.RWMutex
	episodes []RouteMemoryRecord
	records  []RouteMemoryRecord
	stats    map[string]routeMemoryAccessStats
}{episodes: []RouteMemoryRecord{}, records: []RouteMemoryRecord{}, stats: map[string]routeMemoryAccessStats{}}

var agentModelCooldownStore = struct {
	sync.RWMutex
	until map[string]time.Time
}{until: map[string]time.Time{}}

var agentCandidateHealthStore = struct {
	sync.RWMutex
	stats map[string]*CandidateModelState
}{stats: map[string]*CandidateModelState{}}

var candidateUsageStore = struct {
	sync.RWMutex
	entries map[string][]candidateUsageEvent
}{entries: map[string][]candidateUsageEvent{}}

func (h *ManagementHandler) ListConversationAgents(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]any{"data": getConversationAgents()})
}

func (h *ManagementHandler) CreateConversationAgent(w http.ResponseWriter, r *http.Request) {
	h.upsertConversationAgent(w, r, false)
}

func (h *ManagementHandler) UpdateConversationAgent(w http.ResponseWriter, r *http.Request) {
	h.upsertConversationAgent(w, r, true)
}

func (h *ManagementHandler) upsertConversationAgent(w http.ResponseWriter, r *http.Request, mustExist bool) {
	var req UpsertConversationAgentRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	agent, err := upsertConversationAgent(req, mustExist)
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, agent)
}

func (h *ManagementHandler) DeleteConversationAgent(w http.ResponseWriter, r *http.Request) {
	var req DeleteConversationAgentRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	conversationAgentStore.Lock()
	defer conversationAgentStore.Unlock()
	filtered := conversationAgentStore.agents[:0]
	deleted := false
	for _, agent := range conversationAgentStore.agents {
		if agent.ID == req.ID {
			deleted = true
			continue
		}
		filtered = append(filtered, agent)
	}
	if !deleted {
		h.writeError(w, r, http.StatusNotFound, "agent not found")
		return
	}
	conversationAgentStore.agents = append([]ConversationAgent(nil), filtered...)
	h.writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *ManagementHandler) ListConversationTools(w http.ResponseWriter, r *http.Request) {
	manager := mcp.GetManager(r.Context())
	options := []AgentToolOption{}
	if manager != nil {
		for _, client := range manager.GetClients() {
			for _, tool := range client.Tools {
				options = append(options, AgentToolOption{ID: fmt.Sprintf("%s/%s", client.ID, tool), ClientID: client.ID, ToolName: tool, Label: fmt.Sprintf("%s / %s", client.Name, tool)})
			}
		}
	}
	sort.Slice(options, func(i, j int) bool { return options[i].ID < options[j].ID })
	h.writeJSON(w, http.StatusOK, map[string]any{"data": options})
}

func (h *ManagementHandler) ListToolMarketplace(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]any{"data": getMarketplaceItems()})
}

func (h *ManagementHandler) CreateToolMarketplaceItem(w http.ResponseWriter, r *http.Request) {
	h.upsertToolMarketplaceItem(w, r, false)
}
func (h *ManagementHandler) UpdateToolMarketplaceItem(w http.ResponseWriter, r *http.Request) {
	h.upsertToolMarketplaceItem(w, r, true)
}

func (h *ManagementHandler) upsertToolMarketplaceItem(w http.ResponseWriter, r *http.Request, mustExist bool) {
	var req UpsertToolMarketplaceRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := upsertMarketplaceItem(req, mustExist)
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, item)
}

func (h *ManagementHandler) DeleteToolMarketplaceItem(w http.ResponseWriter, r *http.Request) {
	var req DeleteToolMarketplaceRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	toolMarketplaceStore.Lock()
	defer toolMarketplaceStore.Unlock()
	filtered := toolMarketplaceStore.items[:0]
	deleted := false
	for _, item := range toolMarketplaceStore.items {
		if item.ID == req.ID {
			deleted = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !deleted {
		h.writeError(w, r, http.StatusNotFound, "tool marketplace item not found")
		return
	}
	toolMarketplaceStore.items = append([]ToolMarketplaceItem(nil), filtered...)
	h.writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *ManagementHandler) ImportConversationTool(w http.ResponseWriter, r *http.Request) {
	var req UpsertToolMarketplaceRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		req.ID = fmt.Sprintf("tool-%s", strings.ReplaceAll(strings.ToLower(req.SourceToolName), "_", "-"))
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = req.SourceToolName
	}
	if strings.TrimSpace(req.Category) == "" {
		req.Category = "general"
	}
	item, err := upsertMarketplaceItem(req, false)
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, item)
}

func (h *ManagementHandler) ListConversationMemory(w http.ResponseWriter, r *http.Request) {
	items := conversationMemorySnapshot()
	h.writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (h *ManagementHandler) ConversationChat(w http.ResponseWriter, r *http.Request) {
	var req AgentChatRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Messages) == 0 {
		h.writeError(w, r, http.StatusBadRequest, "messages are required")
		return
	}
	if req.SessionID == "" {
		req.SessionID = fmt.Sprintf("session-%d", time.Now().UnixNano())
	}

	lastUserMessage := lastUserTurn(req.Messages)
	if lastUserMessage == "" {
		h.writeError(w, r, http.StatusBadRequest, "at least one user message is required")
		return
	}

	intent := inferConversationIntent(lastUserMessage)
	profile := buildRoutingProfile(req, lastUserMessage, intent, auth.GetAuthContext(r.Context()))
	client, release := h.acquireClient()
	defer release()
	if client == nil {
		h.writeError(w, r, http.StatusServiceUnavailable, "client not available")
		return
	}
	selectedAgent, routeSource, reasons, hit, consulted := selectConversationAgentForProfile(r.Context(), client, profile)
	var selectedTeam *AgentTeam
	var teamParticipants []ConversationAgent
	var teamConsulted []TeamMemoryRecord
	var teamMemoryHit *TeamMemoryRecord
	if team, participants, lead, teamReasons, teamRouteSource, consultedTeamMemory, hitTeamMemory, ok := teamRouteConversationWithProfile(profile); ok {
		selectedTeam = team
		teamParticipants = participants
		teamConsulted = consultedTeamMemory
		teamMemoryHit = hitTeamMemory
		selectedAgent = lead
		routeSource = teamRouteSource
		reasons = append(teamReasons, reasons...)
	} else if profile.RequireTeam {
		reasons = append([]string{"team routing requested but no eligible team was found; falling back to single-agent execution"}, reasons...)
	}
	memoryInfluence := "none"
	if len(consulted) > 0 || len(teamConsulted) > 0 {
		memoryInfluence = "consulted"
	}
	if hit != nil || teamMemoryHit != nil {
		memoryInfluence = "reused"
	}
	noteRouteMemoryConsulted(consulted)
	if hit != nil {
		noteRouteMemoryReused(hit)
	}
	optimizationPlan := buildConversationOptimizationPlan(profile)
	optimizationNotes := []string{}
	if optimizationPlan.Enabled {
		var messageNotes []string
		req.Messages, messageNotes = optimizeConversationTurns(req.Messages, true)
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
	gatewayReq := buildConversationGatewayRequest(req.Messages, selectedAgent, toolSummaries, marketplaceTools)
	if optimizationPlan.Enabled {
		gatewayReq = buildConversationGatewayRequestOptimized(req.Messages, selectedAgent, toolSummaries, marketplaceTools, optimizationPlan)
	}
	ctx := r.Context()
	if profile.MaxToolIterations > 0 {
		ctx = mcp.WithMaxToolIterations(ctx, profile.MaxToolIterations)
	}
	if profile.ToolInjectionEnabled && len(marketplaceTools) > 0 {
		ctx = mcp.WithIncludeTools(ctx, marketplaceTools)
	}
	if manager := mcp.GetManager(ctx); manager != nil && profile.ToolInjectionEnabled && len(marketplaceTools) > 0 {
		gatewayReq.Tools = manager.GetAvailableTools(ctx)
	}

	var modelAccess *auth.ModelAccess
	if h.store != nil {
		access, err := auth.NewModelAccess(r.Context(), h.store, auth.GetAuthContext(r.Context()))
		if err != nil {
			h.writeError(w, r, http.StatusInternalServerError, "failed to evaluate model access")
			return
		}
		modelAccess = access
	}

	resp, selectedCandidate, failovers, trace, err := executeConversationWithProfileCandidates(ctx, client, gatewayReq, selectedAgent, profile, modelAccess)
	selectedAgent.CandidateStates = buildCandidateStatesForProfile(selectedAgent, profile, modelAccess)
	if selectedCandidate != nil {
		selectedAgent.Provider = selectedCandidate.Provider
		selectedAgent.Model = selectedCandidate.Model
	}
	reasons = append([]string{
		fmt.Sprintf("routing_profile complexity=%d latency_budget_ms=%d", profile.Complexity, profile.LatencyBudgetMS),
	}, reasons...)
	if len(profile.RequiredTools) > 0 {
		reasons = append(reasons, fmt.Sprintf("routing_profile required_tools=%s", strings.Join(profile.RequiredTools, ",")))
	}
	result := AgentChatResponse{
		RequestID:             fmt.Sprintf("conv-%d", time.Now().UnixNano()),
		SessionID:             req.SessionID,
		Intent:                intent,
		SelectedTeam:          selectedTeam,
		TeamParticipants:      teamParticipants,
		SelectedAgent:         selectedAgent,
		SelectedCandidate:     selectedCandidate,
		CandidateFailovers:    failovers,
		RouteSource:           routeSource,
		RoutingReasoning:      reasons,
		MemoryInfluence:       memoryInfluence,
		ConsultedMemories:     consulted,
		MemoryHit:             hit,
		TeamConsultedMemories: teamConsulted,
		TeamMemoryHit:         teamMemoryHit,
		GatewayRequest: map[string]any{
			"model":                gatewayReq.Model,
			"tags":                 gatewayReq.Tags,
			"tool_marketplace_ids": selectedAgent.Tools,
			"resolved_mcp_tools":   marketplaceTools,
			"routing_profile": map[string]any{
				"complexity":                 profile.Complexity,
				"require_team":               profile.RequireTeam,
				"required_tools":             profile.RequiredTools,
				"required_capabilities":      profile.RequiredCapabilities,
				"latency_budget_ms":          profile.LatencyBudgetMS,
				"tenant_id":                  profile.TenantID,
				"organization_id":            profile.OrganizationID,
				"memory_reuse_enabled":       profile.MemoryReuseEnabled,
				"team_routing_enabled":       profile.TeamRoutingEnabled,
				"candidate_strategy":         profile.CandidateStrategy,
				"tool_injection_enabled":     profile.ToolInjectionEnabled,
				"tool_scope_pruning_enabled": profile.ToolScopePruningEnabled,
				"max_tool_iterations":        profile.MaxToolIterations,
			},
			"experiment": map[string]any{
				"experiment_id":    profile.ExperimentID,
				"experiment_group": profile.ExperimentGroup,
				"baseline_name":    profile.BaselineName,
			},
			"user":     gatewayReq.User,
			"messages": req.Messages,
		},
		TokenOptimizationEnabled: profile.TokenOptimizationEnabled,
		OptimizationNotes:        cleanStringList(optimizationNotes),
		ExecutionTrace:           buildConversationExecutionTraceResponse(trace),
		GeneratedAt:              time.Now().Format(time.RFC3339),
	}

	recorded := RouteMemoryRecord{
		ID:              fmt.Sprintf("route-%d", time.Now().UnixNano()),
		SessionID:       req.SessionID,
		Query:           lastUserMessage,
		Intent:          intent,
		SelectedAgentID: selectedAgent.ID,
		SelectedModel:   selectedAgent.Model,
		SelectedPath:    fmt.Sprintf("%s/%s/%s", selectedAgent.ID, selectedAgent.Provider, selectedAgent.Model),
		RouteSource:     routeSource,
		CreatedAt:       time.Now(),
	}

	if err != nil {
		recorded.Succeeded = false
		recorded.OutcomeScore = 0.18
		recorded.AnswerPreview = err.Error()
		result.Succeeded = false
		result.ErrorMessage = err.Error()
		result.RecordedMemory = persistConversationMemory(recorded)
		h.writeJSON(w, http.StatusOK, result)
		return
	}

	assistantMessage := extractAssistantText(resp)
	recorded.Succeeded = true
	recorded.AnswerPreview = truncateForPreview(assistantMessage, 180)
	recorded.OutcomeScore = scoreConversationOutcome(intent, assistantMessage, hit != nil)
	result.Succeeded = true
	result.AssistantMessage = assistantMessage
	result.GatewayResponse = resp
	result.RecordedMemory = persistConversationMemory(recorded)
	if selectedTeam != nil {
		participantIDs := make([]string, 0, len(teamParticipants))
		for _, item := range teamParticipants {
			participantIDs = append(participantIDs, item.ID)
		}
		persistTeamMemory(TeamMemoryRecord{ID: fmt.Sprintf("team-route-%d", time.Now().UnixNano()), SessionID: req.SessionID, Query: lastUserMessage, Intent: intent, SelectedTeamID: selectedTeam.ID, SelectedAgentIDs: participantIDs, Summary: truncateForPreview(assistantMessage, 220), Succeeded: true, OutcomeScore: scoreConversationOutcome(intent, assistantMessage, teamMemoryHit != nil), CreatedAt: time.Now()})
	}
	h.writeJSON(w, http.StatusOK, result)
}

func prepareAgentExecution(agent ConversationAgent) (ConversationAgent, []string) {
	candidates := cleanCandidateModels(agent)
	if len(candidates) == 0 {
		return agent, nil
	}
	chosen := candidates[0]
	agent.Provider = chosen.Provider
	agent.Model = chosen.Model
	reasons := []string{fmt.Sprintf("该 expert agent 当前挂载 %d 个候选模型，默认优先尝试 %s/%s", len(candidates), chosen.Provider, chosen.Model)}
	if len(candidates) > 1 {
		reasons = append(reasons, "若首选候选命中 TPM/QPM/并发限制，将自动切换到同 agent 的下一候选模型")
	}
	return agent, reasons
}

func executeConversationWithCandidates(ctx context.Context, client *llmux.Client, gatewayReq *llmux.ChatRequest, agent ConversationAgent, estimatedTokens int) (*llmux.ChatResponse, *CandidateModel, []CandidateFailover, error) {
	candidates := rankCandidateModels(agent, estimatedTokens)
	if len(candidates) == 0 {
		resp, err := executeConversationChatCompletion(ctx, client, gatewayReq)
		return resp, nil, nil, err
	}
	trail := make([]CandidateFailover, 0, len(candidates))
	var lastErr error
	for _, candidate := range candidates {
		predictedState := describeCandidateState(candidate, estimatedTokens)
		if predictedState.Status == "predicted_saturated" {
			trail = append(trail, CandidateFailover{Provider: candidate.Provider, Model: candidate.Model, Outcome: "skipped", Reason: fmt.Sprintf("predicted saturation: rpm=%d/%d tpm=%d/%d", predictedState.CurrentRPM, candidate.RPMLimit, predictedState.CurrentTPM, candidate.TPMLimit)})
			updateCandidateHealth(candidate, predictedState, false, true, "predicted saturation")
			lastErr = llmerrors.NewRateLimitError(candidate.Provider, candidate.Model, "candidate model predicted to exceed rpm/tpm limit")
			continue
		}
		if candidateOnCooldown(candidate) {
			trail = append(trail, CandidateFailover{Provider: candidate.Provider, Model: candidate.Model, Outcome: "skipped", Reason: "cooldown active"})
			predictedState.Status = "cooling_down"
			predictedState.LastError = "cooldown active"
			updateCandidateHealth(candidate, predictedState, false, false, "cooldown active")
			lastErr = llmerrors.NewRateLimitError(candidate.Provider, candidate.Model, "candidate model is in cooldown")
			continue
		}
		candidateReq := *gatewayReq
		candidateReq.Model = candidate.Provider + "/" + candidate.Model
		resp, err := executeConversationChatCompletion(ctx, client, &candidateReq)
		if err == nil {
			recordCandidateUsage(candidate, estimatedTokens)
			trail = append(trail, CandidateFailover{Provider: candidate.Provider, Model: candidate.Model, Outcome: "selected"})
			selectedState := describeCandidateState(candidate, estimatedTokens)
			selectedState.Status = "ready"
			updateCandidateHealth(candidate, selectedState, true, false, "")
			return resp, &candidate, trail, nil
		}
		lastErr = err
		if isCandidateThrottled(err) {
			markCandidateCooldown(candidate, 75*time.Second)
			trail = append(trail, CandidateFailover{Provider: candidate.Provider, Model: candidate.Model, Outcome: "throttled", Reason: err.Error()})
			throttledState := describeCandidateState(candidate, estimatedTokens)
			throttledState.Status = "cooling_down"
			updateCandidateHealth(candidate, throttledState, false, true, err.Error())
			continue
		}
		trail = append(trail, CandidateFailover{Provider: candidate.Provider, Model: candidate.Model, Outcome: "failed", Reason: err.Error()})
		failedState := describeCandidateState(candidate, estimatedTokens)
		failedState.Status = "degraded"
		updateCandidateHealth(candidate, failedState, false, false, err.Error())
		return nil, nil, trail, err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no candidate models available for agent=%s", agent.ID)
	}
	return nil, nil, trail, lastErr
}

func cleanCandidateModels(agent ConversationAgent) []CandidateModel {
	items := append([]CandidateModel{}, agent.CandidateModels...)
	if len(items) == 0 && strings.TrimSpace(agent.Model) != "" {
		items = append(items, CandidateModel{Provider: agent.Provider, Model: agent.Model, Weight: 1})
	}
	filtered := make([]CandidateModel, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item.Provider = strings.TrimSpace(item.Provider)
		item.Model = strings.TrimSpace(item.Model)
		if item.Provider == "" || item.Model == "" {
			continue
		}
		key := item.Provider + "/" + item.Model
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if item.Weight <= 0 {
			item.Weight = 1
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Weight > filtered[j].Weight })
	return filtered
}

func estimateConversationTokens(messages []ConversationTurn) int {
	totalChars := 0
	for _, message := range messages {
		totalChars += len(strings.TrimSpace(message.Content))
	}
	if totalChars <= 0 {
		return 256
	}
	estimate := totalChars/3 + 128
	if estimate < 256 {
		estimate = 256
	}
	return estimate
}

func rankCandidateModels(agent ConversationAgent, estimatedTokens int) []CandidateModel {
	items := cleanCandidateModels(agent)
	sort.SliceStable(items, func(i, j int) bool {
		left := describeCandidateState(items[i], estimatedTokens)
		right := describeCandidateState(items[j], estimatedTokens)
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

func stateRank(status string) int {
	switch status {
	case "ready":
		return 0
	case "warm":
		return 1
	case "latency_exceeded":
		return 2
	case "cooling_down":
		return 3
	case "predicted_saturated":
		return 4
	case "unauthorized":
		return 5
	case "degraded":
		return 6
	default:
		return 7
	}
}

func describeCandidateState(candidate CandidateModel, estimatedTokens int) CandidateModelState {
	rpm, tpm := getCandidateUsage(candidate)
	predictedRPM, predictedTPM := predictCandidateUsage(candidate, estimatedTokens)
	latencyMS := estimateCandidateLatency(candidate)
	latencyFit := candidateLatencyFit(candidate, estimatedTokens)
	state := CandidateModelState{Provider: candidate.Provider, Model: candidate.Model, Weight: candidate.Weight, RPMLimit: candidate.RPMLimit, TPMLimit: candidate.TPMLimit, CurrentRPM: rpm, CurrentTPM: tpm, PredictedRPM: predictedRPM, PredictedTPM: predictedTPM, LatencyMS: latencyMS, LatencyFit: latencyFit, Status: "ready"}
	rpmCapacityLeft := 1.0
	if candidate.RPMLimit > 0 {
		rpmCapacityLeft = clamp01(1 - float64(predictedRPM)/float64(candidate.RPMLimit))
	}
	tpmCapacityLeft := 1.0
	if candidate.TPMLimit > 0 {
		tpmCapacityLeft = clamp01(1 - float64(predictedTPM)/float64(candidate.TPMLimit))
	}
	state.RPMCapacityLeft = rpmCapacityLeft
	state.TPMCapacityLeft = tpmCapacityLeft
	state.SelectionScore = 0.35*candidate.Weight + 0.25*rpmCapacityLeft + 0.25*tpmCapacityLeft + 0.15*latencyFit
	if until, ok := candidateCooldownUntil(candidate); ok {
		state.Status = "cooling_down"
		state.CooldownUntil = until.Format(time.RFC3339)
	}
	if state.Status != "cooling_down" {
		switch {
		case candidate.RPMLimit > 0 && predictedRPM > candidate.RPMLimit:
			state.Status = "predicted_saturated"
		case candidate.TPMLimit > 0 && predictedTPM > candidate.TPMLimit:
			state.Status = "predicted_saturated"
		case rpmCapacityLeft < 0.2 || tpmCapacityLeft < 0.2:
			state.Status = "warm"
		default:
			state.Status = "ready"
		}
	}
	if stored := getStoredCandidateHealth(candidate); stored != nil {
		state.SelectedCount = stored.SelectedCount
		state.FailoverCount = stored.FailoverCount
		state.LastError = stored.LastError
	}
	return state
}

func getCandidateUsage(candidate CandidateModel) (int, int) {
	key := candidate.Provider + "/" + candidate.Model
	now := time.Now()
	currentCutoff := now.Add(-1 * time.Minute)
	historyCutoff := now.Add(-5 * time.Minute)
	candidateUsageStore.Lock()
	defer candidateUsageStore.Unlock()
	entries := candidateUsageStore.entries[key]
	filtered := entries[:0]
	rpm := 0
	tpm := 0
	for _, entry := range entries {
		if entry.At.Before(historyCutoff) {
			continue
		}
		filtered = append(filtered, entry)
		if entry.At.Before(currentCutoff) {
			continue
		}
		rpm++
		tpm += entry.Tokens
	}
	candidateUsageStore.entries[key] = append([]candidateUsageEvent(nil), filtered...)
	return rpm, tpm
}

func predictCandidateUsage(candidate CandidateModel, estimatedTokens int) (int, int) {
	key := candidate.Provider + "/" + candidate.Model
	cutoff := time.Now().Add(-5 * time.Minute)
	candidateUsageStore.RLock()
	entries := append([]candidateUsageEvent(nil), candidateUsageStore.entries[key]...)
	candidateUsageStore.RUnlock()
	rpmBuckets := [5]int{}
	tpmBuckets := [5]int{}
	now := time.Now()
	for _, entry := range entries {
		if entry.At.Before(cutoff) {
			continue
		}
		ageMinutes := int(now.Sub(entry.At) / time.Minute)
		if ageMinutes < 0 || ageMinutes >= len(rpmBuckets) {
			continue
		}
		bucket := len(rpmBuckets) - 1 - ageMinutes
		rpmBuckets[bucket]++
		tpmBuckets[bucket] += entry.Tokens
	}
	predictedRPM := averageIntWindow(rpmBuckets[:]) + 1
	predictedTPM := averageIntWindow(tpmBuckets[:]) + estimatedTokens
	return predictedRPM, predictedTPM
}

func averageIntWindow(values []int) int {
	if len(values) == 0 {
		return 0
	}
	total := 0
	for _, value := range values {
		total += value
	}
	return total / len(values)
}

func estimateCandidateLatency(candidate CandidateModel) int {
	model := strings.ToLower(candidate.Model)
	switch {
	case strings.Contains(model, "reasoner"):
		return 1750
	case strings.Contains(model, "chat"):
		return 900
	default:
		return 1200
	}
}

func candidateLatencyFit(candidate CandidateModel, estimatedTokens int) float64 {
	budget := 1800
	if estimatedTokens > 1200 {
		budget = 2600
	}
	return candidateLatencyFitWithBudget(candidate, budget)
}

func candidateLatencyFitWithBudget(candidate CandidateModel, budget int) float64 {
	if budget <= 0 {
		return 0.5
	}
	fit := 1 - float64(estimateCandidateLatency(candidate))/float64(budget)
	if fit < 0.05 {
		return 0.05
	}
	return clamp01(fit)
}

func recordCandidateUsage(candidate CandidateModel, estimatedTokens int) {
	key := candidate.Provider + "/" + candidate.Model
	candidateUsageStore.Lock()
	candidateUsageStore.entries[key] = append(candidateUsageStore.entries[key], candidateUsageEvent{At: time.Now(), Tokens: estimatedTokens})
	candidateUsageStore.Unlock()
}

func getStoredCandidateHealth(candidate CandidateModel) *CandidateModelState {
	key := candidate.Provider + "/" + candidate.Model
	agentCandidateHealthStore.RLock()
	defer agentCandidateHealthStore.RUnlock()
	return agentCandidateHealthStore.stats[key]
}

func candidateOnCooldown(candidate CandidateModel) bool {
	key := candidate.Provider + "/" + candidate.Model
	agentModelCooldownStore.RLock()
	until, ok := agentModelCooldownStore.until[key]
	agentModelCooldownStore.RUnlock()
	return ok && time.Now().Before(until)
}

func markCandidateCooldown(candidate CandidateModel, duration time.Duration) {
	key := candidate.Provider + "/" + candidate.Model
	agentModelCooldownStore.Lock()
	agentModelCooldownStore.until[key] = time.Now().Add(duration)
	agentModelCooldownStore.Unlock()
}

func updateCandidateHealth(candidate CandidateModel, state CandidateModelState, selected bool, failover bool, lastError string) {
	key := candidate.Provider + "/" + candidate.Model
	agentCandidateHealthStore.Lock()
	defer agentCandidateHealthStore.Unlock()
	stored, ok := agentCandidateHealthStore.stats[key]
	if !ok {
		stored = &CandidateModelState{Provider: candidate.Provider, Model: candidate.Model, Weight: candidate.Weight, RPMLimit: candidate.RPMLimit, TPMLimit: candidate.TPMLimit}
		agentCandidateHealthStore.stats[key] = stored
	}
	stored.Provider = candidate.Provider
	stored.Model = candidate.Model
	stored.Weight = candidate.Weight
	stored.RPMLimit = candidate.RPMLimit
	stored.TPMLimit = candidate.TPMLimit
	stored.CurrentRPM = state.CurrentRPM
	stored.CurrentTPM = state.CurrentTPM
	stored.SelectionScore = state.SelectionScore
	stored.Status = state.Status
	stored.LastError = lastError
	if selected {
		stored.SelectedCount++
	}
	if failover {
		stored.FailoverCount++
	}
	if until, ok := candidateCooldownUntil(candidate); ok {
		stored.CooldownUntil = until.Format(time.RFC3339)
	} else {
		stored.CooldownUntil = ""
	}
}

func candidateCooldownUntil(candidate CandidateModel) (time.Time, bool) {
	key := candidate.Provider + "/" + candidate.Model
	agentModelCooldownStore.RLock()
	until, ok := agentModelCooldownStore.until[key]
	agentModelCooldownStore.RUnlock()
	if !ok || time.Now().After(until) {
		return time.Time{}, false
	}
	return until, true
}

func buildCandidateStates(agent ConversationAgent) []CandidateModelState {
	candidates := cleanCandidateModels(agent)
	states := make([]CandidateModelState, 0, len(candidates))
	for _, candidate := range candidates {
		states = append(states, describeCandidateState(candidate, 0))
	}
	return states
}

func isCandidateThrottled(err error) bool {
	if err == nil {
		return false
	}
	var llmErr *llmerrors.LLMError
	if stderrors.As(err, &llmErr) {
		return llmErr.Type == llmerrors.TypeRateLimit || llmErr.Type == llmerrors.TypeServiceUnavailable
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "rate limit") || strings.Contains(message, "concurrency limit") || strings.Contains(message, "too many requests")
}

func buildConversationGatewayRequest(messages []ConversationTurn, agent ConversationAgent, toolSummaries, resolvedTools []string) *llmux.ChatRequest {
	inventory := map[string]any{
		"agent_id":                   agent.ID,
		"agent_name":                 agent.Name,
		"tool_marketplace_ids":       agent.Tools,
		"tool_marketplace_summaries": toolSummaries,
		"resolved_mcp_tools":         resolvedTools,
	}
	inventoryJSON, _ := json.MarshalIndent(inventory, "", "  ")
	systemPrompt := agent.SystemPrompt + "\n\n[TOOL INVENTORY / CURRENT CAPABILITIES]\n" + string(inventoryJSON) + "\n\n你必须把上面的 TOOL INVENTORY 当作当前唯一有效的工具能力边界。\n1. 如果用户问你当前有哪些工具、能做什么工具调用，必须严格根据 TOOL INVENTORY 回答。\n2. 如果某个工具不在 TOOL INVENTORY 里，就明确说当前没有该工具。\n3. 不要虚构浏览器、数据库、代码执行器、搜索、文件系统等任何未列出的工具。\n4. 如果存在 tools schema，则只允许调用当前请求中提供的那些 tools。"
	gatewayMessages := []llmux.ChatMessage{{
		Role:    "system",
		Content: json.RawMessage(fmt.Sprintf("%q", systemPrompt)),
	}}
	for _, msg := range messages {
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		role := msg.Role
		if role == "" {
			role = "user"
		}
		gatewayMessages = append(gatewayMessages, llmux.ChatMessage{
			Role:    role,
			Content: json.RawMessage(fmt.Sprintf("%q", msg.Content)),
		})
	}
	temperature := 0.2
	return &llmux.ChatRequest{
		Model:       agent.Model,
		Messages:    gatewayMessages,
		Temperature: &temperature,
		MaxTokens:   512,
		Tags:        []string{"conversation-router", agent.ID, agent.Category, agent.Strategy},
		User:        "conversation-router",
	}
}

func buildConversationGatewayRequestOptimized(messages []ConversationTurn, agent ConversationAgent, toolSummaries, resolvedTools []string, plan conversationOptimizationPlan) *llmux.ChatRequest {
	inventory := map[string]any{
		"agent_id":             agent.ID,
		"agent_name":           agent.Name,
		"tool_marketplace_ids": agent.Tools,
		"resolved_mcp_tools":   resolvedTools,
	}
	if !plan.CompactToolInventory {
		inventory["tool_marketplace_summaries"] = toolSummaries
	}

	inventoryJSON, _ := json.MarshalIndent(inventory, "", "  ")
	systemPrompt := agent.SystemPrompt + "\n\n[TOOL INVENTORY / CURRENT CAPABILITIES]\n" + string(inventoryJSON) + "\n\n浣犲繀椤绘妸涓婇潰鐨?TOOL INVENTORY 褰撲綔褰撳墠鍞竴鏈夋晥鐨勫伐鍏疯兘鍔涜竟鐣屻€俓n1. 濡傛灉鐢ㄦ埛闂綘褰撳墠鏈夊摢浜涘伐鍏枫€佽兘鍋氫粈涔堝伐鍏疯皟鐢紝蹇呴』涓ユ牸鏍规嵁 TOOL INVENTORY 鍥炵瓟銆俓n2. 濡傛灉鏌愪釜宸ュ叿涓嶅湪 TOOL INVENTORY 閲岋紝灏辨槑纭褰撳墠娌℃湁璇ュ伐鍏枫€俓n3. 涓嶈铏氭瀯娴忚鍣ㄣ€佹暟鎹簱銆佷唬鐮佹墽琛屽櫒銆佹悳绱€佹枃浠剁郴缁熺瓑浠讳綍鏈垪鍑虹殑宸ュ叿銆俓n4. 濡傛灉瀛樺湪 tools schema锛屽垯鍙厑璁歌皟鐢ㄥ綋鍓嶈姹備腑鎻愪緵鐨勯偅浜?tools銆?\n\nToken optimization mode is enabled. Keep the answer concise, avoid redundant restatement, and prefer the minimum tool usage needed to ground the answer."

	gatewayMessages := []llmux.ChatMessage{{
		Role:    "system",
		Content: json.RawMessage(fmt.Sprintf("%q", systemPrompt)),
	}}
	for _, msg := range messages {
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		role := msg.Role
		if role == "" {
			role = "user"
		}
		gatewayMessages = append(gatewayMessages, llmux.ChatMessage{
			Role:    role,
			Content: json.RawMessage(fmt.Sprintf("%q", msg.Content)),
		})
	}
	temperature := 0.1
	maxTokens := plan.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 320
	}
	return &llmux.ChatRequest{
		Model:       agent.Model,
		Messages:    gatewayMessages,
		Temperature: &temperature,
		MaxTokens:   maxTokens,
		Tags:        []string{"conversation-router", agent.ID, agent.Category, agent.Strategy, "token-optimized"},
		User:        "conversation-router",
	}
}

func inferConversationIntent(query string) string {
	text := strings.ToLower(query)
	switch {
	case containsAny(text, "go", "golang", "code", "代码", "bug", "debug", "sql", "api", "并发", "报错"):
		return "coding"
	case containsAny(text, "论文", "research", "study", "方案", "架构", "设计", "路线", "thesis", "实验", "摘要", "贡献点", "研究问题"):
		return "research"
	case containsAny(text, "翻译", "润色", "改写", "英文", "文案", "写作", "表达"):
		return "writing"
	default:
		return "general"
	}
}

func selectConversationAgent(ctx context.Context, client *llmux.Client, query, intent string) (ConversationAgent, string, []string, *RouteMemoryRecord, []RouteMemoryRecord) {
	if agent, matchedBy, ok := resolveExplicitAgentTarget(query); ok {
		reasons := []string{
			fmt.Sprintf("用户消息中显式指定了 agent=%s", agent.ID),
			fmt.Sprintf("显式指定命中方式=%s", matchedBy),
			fmt.Sprintf("agent 背后执行 provider=%s model=%s", agent.Provider, agent.Model),
		}
		if len(agent.Tools) > 0 {
			reasons = append(reasons, fmt.Sprintf("该 agent 绑定 tools=%d 个，将按需注入到本轮请求", len(agent.Tools)))
		}
		return agent, "explicit-agent", reasons, nil, nil
	}
	if agent, reasoning, memoryHit, consulted, ok := selectConversationAgentByLLM(ctx, client, query, intent); ok {
		reasons := append([]string{"自主路由器根据 Agent Registry 与相似历史 memory 进行了语义选路"}, reasoning...)
		if memoryHit != nil {
			reasons = append(reasons, fmt.Sprintf("参考并复用了相似历史 memory，历史 agent=%s score=%.2f", memoryHit.SelectedAgentID, memoryHit.OutcomeScore))
		}
		if len(agent.Tools) > 0 {
			reasons = append(reasons, fmt.Sprintf("该 agent 绑定 tools=%d 个，将按需注入到本轮请求", len(agent.Tools)))
		}
		return agent, "autonomous-router", reasons, memoryHit, consulted
	}
	if hit := findBestMemoryHit(query, intent); hit != nil && hit.OutcomeScore >= 0.55 {
		if agent, ok := findConversationAgent(hit.SelectedAgentID); ok {
			return agent, "memory-hit", []string{
				"route memory 找到了相似 query 的高分历史路径",
				fmt.Sprintf("复用了历史最优 agent=%s model=%s", hit.SelectedAgentID, hit.SelectedModel),
			}, hit, []RouteMemoryRecord{*hit}
		}
	}

	agentID := mapIntentToAgent(intent)
	agent, _ := findConversationAgent(agentID)
	reasons := []string{
		fmt.Sprintf("根据 query 关键词推断意图=%s", intent),
		fmt.Sprintf("将本轮对话路由到 agent=%s", agent.ID),
		fmt.Sprintf("agent 背后执行 provider=%s model=%s", agent.Provider, agent.Model),
	}
	if len(agent.Tools) > 0 {
		reasons = append(reasons, fmt.Sprintf("该 agent 绑定 tools=%d 个，将按需注入到本轮请求", len(agent.Tools)))
	}
	return agent, "intent-router", reasons, nil, nil
}

func selectConversationAgentByLLM(ctx context.Context, client *llmux.Client, query, intent string) (ConversationAgent, []string, *RouteMemoryRecord, []RouteMemoryRecord, bool) {
	if client == nil {
		return ConversationAgent{}, nil, nil, nil, false
	}
	agents := getConversationAgents()
	registry := make([]map[string]any, 0, len(agents))
	for _, agent := range agents {
		if !agent.Enabled {
			continue
		}
		registry = append(registry, map[string]any{
			"id":           agent.ID,
			"name":         agent.Name,
			"description":  agent.Description,
			"category":     agent.Category,
			"capabilities": agent.Capabilities,
			"tools":        agent.Tools,
		})
	}
	memoryCandidates := findRelevantMemoryCandidates(query, intent, 3)
	memoryContext := make([]map[string]any, 0, len(memoryCandidates))
	for _, item := range memoryCandidates {
		memoryContext = append(memoryContext, map[string]any{
			"query":             item.Query,
			"selected_agent_id": item.SelectedAgentID,
			"selected_model":    item.SelectedModel,
			"route_source":      item.RouteSource,
			"outcome_score":     item.OutcomeScore,
			"answer_preview":    item.AnswerPreview,
		})
	}
	registryJSON, _ := json.MarshalIndent(registry, "", "  ")
	memoryJSON, _ := json.MarshalIndent(memoryContext, "", "  ")
	temperature := 0.0
	routerReq := &llmux.ChatRequest{
		Model: "deepseek-chat",
		Messages: []llmux.ChatMessage{
			{Role: "system", Content: json.RawMessage(fmt.Sprintf("%q", "你是一个对话路由器。你只能从给定的 Agent Registry 中选择一个最适合回答用户问题的 agent。你会同时看到相似历史 memory，它们只是先验，不是硬约束；你可以复用，也可以不复用。输出必须是 JSON，对象格式为 {\"agent_id\":\"...\",\"reason\":\"...\"}。agent_id 必须来自 registry。"))},
			{Role: "user", Content: json.RawMessage(fmt.Sprintf("%q", "intent="+intent+"\nquery="+query+"\nagent_registry=\n"+string(registryJSON)+"\n\nsimilar_route_memory=\n"+string(memoryJSON)))},
		},
		Temperature:    &temperature,
		MaxTokens:      180,
		ResponseFormat: &llmux.ResponseFormat{Type: "json_object"},
	}
	resp, err := client.ChatCompletion(ctx, routerReq)
	if err != nil {
		return ConversationAgent{}, nil, nil, memoryCandidates, false
	}
	var decision struct {
		AgentID string `json:"agent_id"`
		Reason  string `json:"reason"`
	}
	raw := extractAssistantText(resp)
	if err := unmarshalAssistantJSON(raw, &decision); err != nil {
		return ConversationAgent{}, nil, nil, memoryCandidates, false
	}
	agent, ok := findConversationAgent(strings.TrimSpace(decision.AgentID))
	if !ok || agent.ID == "" {
		return ConversationAgent{}, nil, nil, memoryCandidates, false
	}
	reason := strings.TrimSpace(decision.Reason)
	if reason == "" {
		reason = "LLM router 在 Agent Registry 中选择了最匹配的 agent"
	}
	var memoryHit *RouteMemoryRecord
	for i := range memoryCandidates {
		if memoryCandidates[i].SelectedAgentID == agent.ID {
			copied := memoryCandidates[i]
			memoryHit = &copied
			break
		}
	}
	return agent, []string{reason}, memoryHit, memoryCandidates, true
}

func resolveExplicitAgentTarget(query string) (ConversationAgent, string, bool) {
	text := strings.ToLower(strings.TrimSpace(query))
	for _, agent := range getConversationAgents() {
		if !agent.Enabled {
			continue
		}
		id := strings.ToLower(agent.ID)
		name := strings.ToLower(agent.Name)
		if strings.Contains(text, "@"+id) || strings.Contains(text, "agent="+id) || strings.Contains(text, "指定agent="+id) {
			return agent, "agent-id-token", true
		}
		if strings.Contains(text, id) {
			return agent, "agent-id-substring", true
		}
		if name != "" && strings.Contains(text, name) {
			return agent, "agent-name-substring", true
		}
	}
	return ConversationAgent{}, "", false
}

func mapIntentToAgent(intent string) string {
	agents := getConversationAgents()
	for _, agent := range agents {
		if agent.Enabled && agent.Category == intent {
			return agent.ID
		}
	}
	for _, agent := range agents {
		if agent.Enabled && agent.Category == "general" {
			return agent.ID
		}
	}
	if len(agents) > 0 {
		return agents[0].ID
	}
	return ""
}

func findBestMemoryHit(query, intent string) *RouteMemoryRecord {
	items := findRelevantMemoryCandidates(query, intent, 1)
	if len(items) == 0 {
		return nil
	}
	item := items[0]
	return &item
}

func findRelevantMemoryCandidates(query, intent string, limit int) []RouteMemoryRecord {
	profile := RoutingProfile{
		Query:              query,
		Intent:             intent,
		QueryTokens:        tokenizeQuery(query),
		MemoryReuseEnabled: true,
	}
	return findRelevantMemoryCandidatesForProfile(profile, limit)
}

func persistConversationMemory(record RouteMemoryRecord) RouteMemoryRecord {
	conversationMemoryStore.Lock()
	defer conversationMemoryStore.Unlock()
	now := time.Now()
	episode := normalizeRouteMemoryEpisode(record)
	conversationMemoryStore.episodes = append(conversationMemoryStore.episodes, episode)
	pruneRouteMemoryLocked(now)
	records := rebuildRouteMemoryRecordsLocked()
	profile := RoutingProfile{
		Query:              episode.Query,
		Intent:             episode.Intent,
		QueryTokens:        tokenizeQuery(episode.Query),
		MemoryReuseEnabled: true,
	}
	best := episode
	bestScore := 0.0
	for _, item := range records {
		if item.SelectedAgentID != episode.SelectedAgentID {
			continue
		}
		score := scoreRouteMemorySimilarity(profile, item)
		if score >= bestScore {
			best = item
			bestScore = score
		}
	}
	return best
}

func getConversationAgents() []ConversationAgent {
	conversationAgentStore.RLock()
	defer conversationAgentStore.RUnlock()
	items := make([]ConversationAgent, len(conversationAgentStore.agents))
	copy(items, conversationAgentStore.agents)
	for i := range items {
		items[i].CandidateStates = buildCandidateStates(items[i])
	}
	return items
}

func upsertConversationAgent(req UpsertConversationAgentRequest, mustExist bool) (ConversationAgent, error) {
	if strings.TrimSpace(req.ID) == "" {
		return ConversationAgent{}, fmt.Errorf("id is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return ConversationAgent{}, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(req.Model) == "" {
		return ConversationAgent{}, fmt.Errorf("model is required")
	}
	if strings.TrimSpace(req.Category) == "" {
		req.Category = "general"
	}
	if strings.TrimSpace(req.Provider) == "" {
		req.Provider = "deepseek-primary"
	}
	if strings.TrimSpace(req.Strategy) == "" {
		req.Strategy = "lowest-latency"
	}
	if strings.TrimSpace(req.Accent) == "" {
		req.Accent = "cyan"
	}
	conversationAgentStore.Lock()
	defer conversationAgentStore.Unlock()
	now := time.Now().Format(time.RFC3339)
	for i, agent := range conversationAgentStore.agents {
		if agent.ID != req.ID {
			continue
		}
		enabled := agent.Enabled
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		updated := ConversationAgent{ID: req.ID, Name: req.Name, Description: req.Description, Category: req.Category, Provider: req.Provider, Model: req.Model, CandidateModels: cleanCandidateModels(ConversationAgent{Provider: req.Provider, Model: req.Model, CandidateModels: req.CandidateModels}), Strategy: req.Strategy, Capabilities: cleanStringList(req.Capabilities), SystemPrompt: req.SystemPrompt, Accent: req.Accent, Tools: cleanStringList(req.Tools), TenantScopes: cleanStringList(req.TenantScopes), Enabled: enabled, CreatedAt: agent.CreatedAt, UpdatedAt: now}
		conversationAgentStore.agents[i] = updated
		return updated, nil
	}
	if mustExist {
		return ConversationAgent{}, fmt.Errorf("agent not found")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	created := ConversationAgent{ID: req.ID, Name: req.Name, Description: req.Description, Category: req.Category, Provider: req.Provider, Model: req.Model, CandidateModels: cleanCandidateModels(ConversationAgent{Provider: req.Provider, Model: req.Model, CandidateModels: req.CandidateModels}), Strategy: req.Strategy, Capabilities: cleanStringList(req.Capabilities), SystemPrompt: req.SystemPrompt, Accent: req.Accent, Tools: cleanStringList(req.Tools), TenantScopes: cleanStringList(req.TenantScopes), Enabled: enabled, CreatedAt: now, UpdatedAt: now}
	conversationAgentStore.agents = append([]ConversationAgent{created}, conversationAgentStore.agents...)
	return created, nil
}

func getMarketplaceItems() []ToolMarketplaceItem {
	toolMarketplaceStore.RLock()
	defer toolMarketplaceStore.RUnlock()
	items := make([]ToolMarketplaceItem, len(toolMarketplaceStore.items))
	copy(items, toolMarketplaceStore.items)
	return items
}

func upsertMarketplaceItem(req UpsertToolMarketplaceRequest, mustExist bool) (ToolMarketplaceItem, error) {
	if strings.TrimSpace(req.ID) == "" {
		return ToolMarketplaceItem{}, fmt.Errorf("id is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return ToolMarketplaceItem{}, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(req.SourceClientID) == "" {
		return ToolMarketplaceItem{}, fmt.Errorf("source_client_id is required")
	}
	if strings.TrimSpace(req.SourceToolName) == "" {
		return ToolMarketplaceItem{}, fmt.Errorf("source_tool_name is required")
	}
	if strings.TrimSpace(req.Category) == "" {
		req.Category = "general"
	}
	toolMarketplaceStore.Lock()
	defer toolMarketplaceStore.Unlock()
	now := time.Now().Format(time.RFC3339)
	for i, item := range toolMarketplaceStore.items {
		if item.ID != req.ID {
			continue
		}
		enabled := item.Enabled
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		updated := ToolMarketplaceItem{ID: req.ID, Name: req.Name, Description: req.Description, Category: req.Category, SourceClientID: req.SourceClientID, SourceToolName: req.SourceToolName, Tags: cleanStringList(req.Tags), Enabled: enabled, CreatedAt: item.CreatedAt, UpdatedAt: now}
		toolMarketplaceStore.items[i] = updated
		return updated, nil
	}
	if mustExist {
		return ToolMarketplaceItem{}, fmt.Errorf("tool marketplace item not found")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	created := ToolMarketplaceItem{ID: req.ID, Name: req.Name, Description: req.Description, Category: req.Category, SourceClientID: req.SourceClientID, SourceToolName: req.SourceToolName, Tags: cleanStringList(req.Tags), Enabled: enabled, CreatedAt: now, UpdatedAt: now}
	toolMarketplaceStore.items = append([]ToolMarketplaceItem{created}, toolMarketplaceStore.items...)
	return created, nil
}

func resolveMarketplaceTools(ids []string) []string {
	items := getMarketplaceItems()
	resolved := make([]string, 0, len(ids))
	for _, id := range ids {
		for _, item := range items {
			if item.ID == id && item.Enabled {
				resolved = append(resolved, fmt.Sprintf("%s/%s", item.SourceClientID, item.SourceToolName))
				break
			}
		}
	}
	return cleanStringList(resolved)
}

func resolveMarketplaceToolSummaries(ids []string) []string {
	items := getMarketplaceItems()
	summaries := make([]string, 0, len(ids))
	for _, id := range ids {
		for _, item := range items {
			if item.ID == id && item.Enabled {
				summaries = append(summaries, fmt.Sprintf("%s (%s -> %s/%s)", item.Name, item.ID, item.SourceClientID, item.SourceToolName))
				break
			}
		}
	}
	return cleanStringList(summaries)
}

func findConversationAgent(id string) (ConversationAgent, bool) {
	for _, agent := range getConversationAgents() {
		if agent.ID == id && agent.Enabled {
			return agent, true
		}
	}
	agents := getConversationAgents()
	if len(agents) > 0 {
		return agents[0], false
	}
	return ConversationAgent{}, false
}

func scoreConversationOutcome(intent, answer string, reused bool) float64 {
	score := 0.72
	if len(strings.TrimSpace(answer)) > 120 {
		score += 0.1
	}
	if reused {
		score += 0.06
	}
	if intent == "coding" || intent == "research" {
		score += 0.04
	}
	if score > 0.98 {
		return 0.98
	}
	return score
}

func extractAssistantText(resp *llmux.ChatResponse) string {
	if resp == nil || len(resp.Choices) == 0 {
		return ""
	}
	return decodeChatMessageContent(resp.Choices[0].Message.Content)
}

func unmarshalAssistantJSON(raw string, target any) error {
	if err := json.Unmarshal([]byte(raw), target); err == nil {
		return nil
	}
	var text string
	if err := json.Unmarshal([]byte(raw), &text); err != nil {
		return err
	}
	return json.Unmarshal([]byte(text), target)
}

func truncateForPreview(value string, max int) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= max {
		return trimmed
	}
	return trimmed[:max] + "..."
}

func lastUserTurn(messages []ConversationTurn) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && strings.TrimSpace(messages[i].Content) != "" {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}

func tokenizeQuery(query string) []string {
	query = strings.ToLower(query)
	replacer := strings.NewReplacer(",", " ", ".", " ", "?", " ", "!", " ", "，", " ", "。", " ", "？", " ", "！", " ", "：", " ", ":", " ", "；", " ", ";", " ", "（", " ", "）", " ", "(", " ", ")", " ", "\n", " ", "\t", " ")
	normalized := replacer.Replace(query)
	parts := strings.Fields(normalized)
	set := map[string]struct{}{}
	for _, part := range parts {
		if len([]rune(part)) <= 1 {
			continue
		}
		set[part] = struct{}{}
	}
	compact := strings.Join(parts, "")
	runes := []rune(compact)
	for i := 0; i+1 < len(runes); i++ {
		gram := string(runes[i : i+2])
		if strings.TrimSpace(gram) == "" {
			continue
		}
		set[gram] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for token := range set {
		result = append(result, token)
	}
	sort.Strings(result)
	return result
}

func tokenOverlapScore(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := map[string]struct{}{}
	for _, token := range b {
		set[token] = struct{}{}
	}
	matches := 0
	for _, token := range a {
		if _, ok := set[token]; ok {
			matches++
		}
	}
	denominator := len(a)
	if len(b) > denominator {
		denominator = len(b)
	}
	return float64(matches) / float64(denominator)
}

func containsAny(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func cleanStringList(items []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	sort.Strings(result)
	return result
}
