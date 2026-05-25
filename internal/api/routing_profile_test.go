package api

import (
	"context"
	"testing"
	"time"

	"github.com/blueberrycongee/llmux/internal/auth"
)

func TestBuildRoutingProfileInfersComplexityAndTools(t *testing.T) {
	req := AgentChatRequest{
		Messages: []ConversationTurn{{
			Role:    "user",
			Content: "Design a multi-agent gateway architecture, inspect a github repo, and check the sqlite schema before implementation.",
		}},
	}

	profile := buildRoutingProfile(req, req.Messages[0].Content, "research", nil)

	if profile.Complexity < 3 {
		t.Fatalf("expected complexity >= 3, got %d", profile.Complexity)
	}
	if !profile.RequireTeam {
		t.Fatalf("expected team routing to be required")
	}
	if !containsString(profile.RequiredTools, "preset-github-repo") {
		t.Fatalf("expected github tool to be inferred, got %v", profile.RequiredTools)
	}
	if !containsString(profile.RequiredTools, "preset-sqlite-query") {
		t.Fatalf("expected sqlite tool to be inferred, got %v", profile.RequiredTools)
	}
}

func TestBuildRoutingProfileAppliesExperimentOverrides(t *testing.T) {
	req := AgentChatRequest{
		Messages: []ConversationTurn{{
			Role:    "user",
			Content: "Inspect the repository and explain the routing pipeline.",
		}},
		Experiment: ExperimentOptions{
			ExperimentID:            "exp-memory-off",
			ExperimentGroup:         "baseline",
			BaselineName:            "memoryless-routing",
			MemoryReuseEnabled:      boolPtr(false),
			TeamRoutingEnabled:      boolPtr(false),
			CandidateStrategy:       candidateStrategyGreedyCurrent,
			ToolInjectionEnabled:    boolPtr(false),
			ToolScopePruningEnabled: boolPtr(false),
			MaxToolIterations:       3,
		},
	}

	profile := buildRoutingProfile(req, req.Messages[0].Content, "research", nil)

	if profile.MemoryReuseEnabled {
		t.Fatalf("expected memory reuse to be disabled")
	}
	if profile.TeamRoutingEnabled {
		t.Fatalf("expected team routing to be disabled")
	}
	if profile.ToolInjectionEnabled {
		t.Fatalf("expected tool injection to be disabled")
	}
	if profile.ToolScopePruningEnabled {
		t.Fatalf("expected tool scope pruning to be disabled")
	}
	if profile.CandidateStrategy != candidateStrategyGreedyCurrent {
		t.Fatalf("expected greedy-current strategy, got %s", profile.CandidateStrategy)
	}
	if profile.MaxToolIterations != 3 {
		t.Fatalf("expected max tool iterations 3, got %d", profile.MaxToolIterations)
	}
	if profile.ExperimentID != "exp-memory-off" || profile.ExperimentGroup != "baseline" || profile.BaselineName != "memoryless-routing" {
		t.Fatalf("unexpected experiment metadata: %+v", profile)
	}
}

func TestSelectConversationAgentByScoreHonorsRequiredTools(t *testing.T) {
	profile := RoutingProfile{
		Query:                "Debug the SQL schema mismatch and inspect the sqlite database.",
		Intent:               "coding",
		QueryTokens:          tokenizeQuery("Debug the SQL schema mismatch and inspect the sqlite database."),
		RequiredTools:        []string{"preset-sqlite-query"},
		RequiredCapabilities: []string{"coding", "sql"},
		LatencyBudgetMS:      2400,
	}

	agent, _, _, _, ok := selectConversationAgentByScore(profile)
	if !ok {
		t.Fatalf("expected a candidate agent")
	}
	if agent.ID != "code-specialist" {
		t.Fatalf("expected code-specialist, got %s", agent.ID)
	}
}

func TestPersistTeamMemoryReplacesHighlySimilarRecord(t *testing.T) {
	restore := withIsolatedTeamMemoryStore()
	defer restore()

	first := TeamMemoryRecord{
		ID:             "old-record",
		Query:          "design multi agent thesis gateway",
		Intent:         "research",
		SelectedTeamID: "thesis-lab-team",
		OutcomeScore:   0.82,
		Succeeded:      true,
		CreatedAt:      time.Now().Add(-time.Hour),
	}
	second := TeamMemoryRecord{
		ID:             "new-record",
		Query:          "design multi-agent thesis gateway",
		Intent:         "research",
		SelectedTeamID: "thesis-lab-team",
		OutcomeScore:   0.93,
		Succeeded:      true,
		CreatedAt:      time.Now(),
	}

	persistTeamMemory(first)
	persistTeamMemory(second)

	teamMemoryStore.RLock()
	defer teamMemoryStore.RUnlock()
	if len(teamMemoryStore.records) != 1 {
		t.Fatalf("expected exactly one deduplicated team memory record, got %d", len(teamMemoryStore.records))
	}
	if teamMemoryStore.records[0].SelectedTeamID != second.SelectedTeamID {
		t.Fatalf("expected aggregated record to keep team %s, got %s", second.SelectedTeamID, teamMemoryStore.records[0].SelectedTeamID)
	}
	if teamMemoryStore.records[0].SourceCount != 2 {
		t.Fatalf("expected aggregated record to contain 2 source episodes, got %d", teamMemoryStore.records[0].SourceCount)
	}
	if teamMemoryStore.records[0].MemoryStage != "summary" {
		t.Fatalf("expected aggregated memory stage summary, got %s", teamMemoryStore.records[0].MemoryStage)
	}
}

func TestPersistTeamMemoryBuildsCompressedInsight(t *testing.T) {
	restore := withIsolatedTeamMemoryStore()
	defer restore()

	baseTime := time.Now()
	for idx, query := range []string{
		"design a gateway rollout plan for a multi agent research team",
		"create a rollout plan for the same multi agent research gateway",
		"review the team workflow for multi agent gateway research rollout",
		"improve the gateway research team workflow and rollout plan",
	} {
		persistTeamMemory(TeamMemoryRecord{
			ID:               "episode-" + string(rune('a'+idx)),
			Query:            query,
			Intent:           "research",
			SelectedTeamID:   "thesis-lab-team",
			SelectedAgentIDs: []string{"research-strategist", "code-specialist", "writer-translator"},
			Summary:          "research strategist drafts the plan, code specialist checks implementation, writer translator polishes delivery",
			OutcomeScore:     0.9,
			Succeeded:        true,
			CreatedAt:        baseTime.Add(time.Duration(idx) * time.Minute),
		})
	}

	items := teamMemorySnapshot()
	if len(items) != 1 {
		t.Fatalf("expected one compressed team memory record, got %d", len(items))
	}
	record := items[0]
	if record.MemoryStage != "insight" {
		t.Fatalf("expected insight memory stage, got %s", record.MemoryStage)
	}
	if record.SourceCount != 4 {
		t.Fatalf("expected 4 source episodes, got %d", record.SourceCount)
	}
	if len(record.DistilledLearnings) == 0 {
		t.Fatalf("expected distilled learnings to be generated")
	}
	if len(record.RepresentativeQueries) == 0 {
		t.Fatalf("expected representative queries to be retained")
	}
	if record.CompressedSummary == "" {
		t.Fatalf("expected compressed summary to be generated")
	}
	if record.CompressionRatio <= 0 || record.CompressionRatio > 1 {
		t.Fatalf("expected compression ratio to be in (0, 1], got %.2f", record.CompressionRatio)
	}
}

func TestPersistConversationMemoryBuildsCompressedInsightAndStats(t *testing.T) {
	restore := withIsolatedConversationMemorySnapshot()
	defer restore()

	baseTime := time.Now()
	for idx, query := range []string{
		"analyze how the gateway should route a coding follow-up about latency and retries",
		"review the routing strategy for coding follow-up latency and retry handling",
		"explain which agent should handle gateway coding follow-up about retries and latency",
		"summarize the best route for gateway coding follow-up on retry and latency issues",
	} {
		persistConversationMemory(RouteMemoryRecord{
			ID:              "route-episode-" + string(rune('a'+idx)),
			Query:           query,
			Intent:          "coding",
			SelectedAgentID: "code-specialist",
			SelectedModel:   "deepseek-reasoner",
			SelectedPath:    "code-specialist/deepseek-primary/deepseek-reasoner",
			RouteSource:     "score-router",
			AnswerPreview:   "route to code specialist for deeper gateway retry and latency analysis",
			OutcomeScore:    0.91,
			Succeeded:       true,
			CreatedAt:       baseTime.Add(time.Duration(idx) * time.Minute),
		})
	}

	items := conversationMemorySnapshot()
	if len(items) != 1 {
		t.Fatalf("expected one compressed route memory record, got %d", len(items))
	}
	record := items[0]
	if record.MemoryStage != "insight" {
		t.Fatalf("expected insight memory stage, got %s", record.MemoryStage)
	}
	if record.SourceCount != 4 {
		t.Fatalf("expected 4 source episodes, got %d", record.SourceCount)
	}
	if len(record.DistilledLearnings) == 0 {
		t.Fatalf("expected distilled learnings to be generated")
	}
	if record.CompressedSummary == "" {
		t.Fatalf("expected compressed summary to be generated")
	}

	noteRouteMemoryConsulted(items)
	noteRouteMemoryReused(&record)

	refreshed := conversationMemorySnapshot()
	if len(refreshed) != 1 {
		t.Fatalf("expected one route memory record after stats update, got %d", len(refreshed))
	}
	if refreshed[0].ConsultCount != 1 {
		t.Fatalf("expected consult count 1, got %d", refreshed[0].ConsultCount)
	}
	if refreshed[0].ReuseCount != 1 {
		t.Fatalf("expected reuse count 1, got %d", refreshed[0].ReuseCount)
	}
}

func TestDescribeCandidateStateForProfileUsesSafeThresholdAndAuthorization(t *testing.T) {
	restore := withIsolatedCandidateStores()
	defer restore()

	candidate := CandidateModel{
		Provider: "restricted-provider",
		Model:    "restricted-model",
		Weight:   1,
		RPMLimit: 10,
		TPMLimit: 1000,
	}

	setCandidateUsageHistory(candidate, []int{9, 9, 9, 9, 9}, 80)
	state := describeCandidateStateForProfile(candidate, RoutingProfile{
		EstimatedTokens: 100,
		LatencyBudgetMS: 2000,
	}, nil)

	if state.SafeRPMLimit != 9 {
		t.Fatalf("expected safe rpm limit 9, got %d", state.SafeRPMLimit)
	}
	if state.Status != "predicted_saturated" {
		t.Fatalf("expected predicted_saturated, got %s", state.Status)
	}

	access, err := auth.NewModelAccess(context.Background(), auth.NewMemoryStore(), &auth.AuthContext{
		APIKey: &auth.APIKey{AllowedModels: []string{"allowed-model"}},
	})
	if err != nil {
		t.Fatalf("unexpected model access error: %v", err)
	}

	unauthorized := describeCandidateStateForProfile(candidate, RoutingProfile{
		EstimatedTokens: 100,
		LatencyBudgetMS: 2000,
	}, access)
	if unauthorized.Authorized {
		t.Fatalf("expected candidate to be unauthorized")
	}
	if unauthorized.Status != "unauthorized" {
		t.Fatalf("expected unauthorized status, got %s", unauthorized.Status)
	}
}

func TestRankCandidateModelsForProfileSupportsPassiveAndPredictiveStrategies(t *testing.T) {
	restore := withIsolatedCandidateStores()
	defer restore()

	agent := ConversationAgent{
		ID: "test-agent",
		CandidateModels: []CandidateModel{
			{Provider: "p1", Model: "model-hot", Weight: 1.0, RPMLimit: 10, TPMLimit: 1000},
			{Provider: "p1", Model: "model-cool", Weight: 0.5, RPMLimit: 10, TPMLimit: 1000},
		},
	}

	setCandidateUsageHistory(agent.CandidateModels[0], []int{9, 9, 9, 9, 9}, 50)
	setCandidateUsageHistory(agent.CandidateModels[1], []int{1, 1, 1, 1, 1}, 50)

	predictive := rankCandidateModelsForProfile(agent, RoutingProfile{
		EstimatedTokens:   100,
		LatencyBudgetMS:   2000,
		CandidateStrategy: candidateStrategyPredictive,
	}, nil)
	if len(predictive) != 2 {
		t.Fatalf("expected 2 predictive candidates, got %d", len(predictive))
	}
	if predictive[0].Model != "model-cool" {
		t.Fatalf("expected predictive strategy to prioritize cool candidate, got %s", predictive[0].Model)
	}

	passive := rankCandidateModelsForProfile(agent, RoutingProfile{
		EstimatedTokens:   100,
		LatencyBudgetMS:   2000,
		CandidateStrategy: candidateStrategyPassive,
	}, nil)
	if len(passive) != 2 {
		t.Fatalf("expected 2 passive candidates, got %d", len(passive))
	}
	if passive[0].Model != "model-hot" {
		t.Fatalf("expected passive strategy to preserve declared/weight order, got %s", passive[0].Model)
	}
}

func withIsolatedTeamMemoryStore() func() {
	teamMemoryStore.Lock()
	oldEpisodes := append([]TeamMemoryRecord(nil), teamMemoryStore.episodes...)
	oldRecords := append([]TeamMemoryRecord(nil), teamMemoryStore.records...)
	oldStats := map[string]teamMemoryAccessStats{}
	for key, value := range teamMemoryStore.stats {
		oldStats[key] = value
	}
	teamMemoryStore.episodes = nil
	teamMemoryStore.records = nil
	teamMemoryStore.stats = map[string]teamMemoryAccessStats{}
	teamMemoryStore.Unlock()

	return func() {
		teamMemoryStore.Lock()
		teamMemoryStore.episodes = oldEpisodes
		teamMemoryStore.records = oldRecords
		teamMemoryStore.stats = oldStats
		teamMemoryStore.Unlock()
	}
}

func withIsolatedConversationMemorySnapshot() func() {
	conversationMemoryStore.Lock()
	oldEpisodes := append([]RouteMemoryRecord(nil), conversationMemoryStore.episodes...)
	oldRecords := append([]RouteMemoryRecord(nil), conversationMemoryStore.records...)
	oldStats := map[string]routeMemoryAccessStats{}
	for key, value := range conversationMemoryStore.stats {
		oldStats[key] = value
	}
	conversationMemoryStore.episodes = nil
	conversationMemoryStore.records = nil
	conversationMemoryStore.stats = map[string]routeMemoryAccessStats{}
	conversationMemoryStore.Unlock()

	return func() {
		conversationMemoryStore.Lock()
		conversationMemoryStore.episodes = oldEpisodes
		conversationMemoryStore.records = oldRecords
		conversationMemoryStore.stats = oldStats
		conversationMemoryStore.Unlock()
	}
}

func withIsolatedCandidateStores() func() {
	candidateUsageStore.Lock()
	oldUsage := map[string][]candidateUsageEvent{}
	for key, entries := range candidateUsageStore.entries {
		oldUsage[key] = append([]candidateUsageEvent(nil), entries...)
	}
	candidateUsageStore.entries = map[string][]candidateUsageEvent{}
	candidateUsageStore.Unlock()

	agentCandidateHealthStore.Lock()
	oldHealth := map[string]*CandidateModelState{}
	for key, state := range agentCandidateHealthStore.stats {
		copyState := *state
		oldHealth[key] = &copyState
	}
	agentCandidateHealthStore.stats = map[string]*CandidateModelState{}
	agentCandidateHealthStore.Unlock()

	agentModelCooldownStore.Lock()
	oldCooldowns := map[string]time.Time{}
	for key, until := range agentModelCooldownStore.until {
		oldCooldowns[key] = until
	}
	agentModelCooldownStore.until = map[string]time.Time{}
	agentModelCooldownStore.Unlock()

	return func() {
		candidateUsageStore.Lock()
		candidateUsageStore.entries = oldUsage
		candidateUsageStore.Unlock()

		agentCandidateHealthStore.Lock()
		agentCandidateHealthStore.stats = oldHealth
		agentCandidateHealthStore.Unlock()

		agentModelCooldownStore.Lock()
		agentModelCooldownStore.until = oldCooldowns
		agentModelCooldownStore.Unlock()
	}
}

func setCandidateUsageHistory(candidate CandidateModel, perMinute []int, tokensPerRequest int) {
	key := candidate.Provider + "/" + candidate.Model
	now := time.Now()
	entries := make([]candidateUsageEvent, 0)
	for minuteAgo, count := range perMinute {
		for i := 0; i < count; i++ {
			entries = append(entries, candidateUsageEvent{
				At:     now.Add(-(time.Duration(minuteAgo) * time.Minute) - 10*time.Second),
				Tokens: tokensPerRequest,
			})
		}
	}
	candidateUsageStore.Lock()
	candidateUsageStore.entries[key] = entries
	candidateUsageStore.Unlock()
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
