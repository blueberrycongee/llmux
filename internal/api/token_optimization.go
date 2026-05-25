package api

import (
	"fmt"
	"sort"
	"strings"
)

type conversationOptimizationPlan struct {
	Enabled              bool
	CompactToolInventory bool
	MaxTokens            int
	Notes                []string
}

func shouldUseFastPath(profile RoutingProfile) bool {
	if !profile.TokenOptimizationEnabled {
		return false
	}
	if profile.RequireTeam {
		return false
	}
	if len(profile.RequiredTools) > 0 {
		return false
	}
	if profile.Complexity > 1 {
		return false
	}
	switch profile.Intent {
	case "general", "writing":
		return true
	default:
		return false
	}
}

func buildConversationOptimizationPlan(profile RoutingProfile) conversationOptimizationPlan {
	if !profile.TokenOptimizationEnabled {
		return conversationOptimizationPlan{
			Enabled:              false,
			CompactToolInventory: false,
			MaxTokens:            512,
		}
	}

	maxTokens := 320
	if profile.Intent == "general" || profile.Intent == "writing" {
		maxTokens = 220
	} else if profile.Intent == "coding" || profile.Intent == "research" {
		maxTokens = 320
	}
	if profile.RequireTeam {
		maxTokens += 40
	}
	if profile.Complexity >= 4 {
		maxTokens += 40
	}

	return conversationOptimizationPlan{
		Enabled:              true,
		CompactToolInventory: true,
		MaxTokens:            maxTokens,
		Notes: []string{
			fmt.Sprintf("token optimization enabled: capped response budget at %d tokens", maxTokens),
			"token optimization enabled: compact tool inventory enabled",
		},
	}
}

func optimizeConversationTurns(messages []ConversationTurn, enabled bool) ([]ConversationTurn, []string) {
	if !enabled {
		return messages, nil
	}

	if len(messages) == 0 {
		return messages, nil
	}

	notes := []string{}
	optimized := make([]ConversationTurn, 0, len(messages))
	start := 0
	if len(messages) > 4 {
		start = len(messages) - 4
		notes = append(notes, fmt.Sprintf("token optimization enabled: preserved the latest %d turns verbatim and compressed older context", len(messages)-start))
	}

	for i, msg := range messages {
		content := normalizeConversationWhitespace(msg.Content)
		if i < start {
			limit := 180
			if msg.Role == "assistant" {
				limit = 220
			}
			content = summarizeForTokenBudget(content, limit)
			if msg.Role == "assistant" {
				content = "[compressed assistant context] " + content
			} else {
				content = "[compressed prior context] " + content
			}
		}
		optimized = append(optimized, ConversationTurn{
			Role:    msg.Role,
			Content: content,
		})
	}

	return optimized, notes
}

func normalizeConversationWhitespace(input string) string {
	fields := strings.Fields(strings.TrimSpace(input))
	return strings.Join(fields, " ")
}

func summarizeForTokenBudget(input string, limit int) string {
	if limit <= 0 {
		limit = 160
	}
	runes := []rune(strings.TrimSpace(input))
	if len(runes) <= limit {
		return string(runes)
	}
	if limit < 6 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}

func optimizeAgentForCost(agent ConversationAgent, enabled bool) (ConversationAgent, []string) {
	if !enabled {
		return agent, nil
	}

	candidates := cleanCandidateModels(agent)
	if len(candidates) == 0 {
		return agent, nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidateCostRank(candidates[i])
		right := candidateCostRank(candidates[j])
		if left == right {
			return candidates[i].Weight > candidates[j].Weight
		}
		return left < right
	})

	agent.CandidateModels = candidates
	agent.Provider = candidates[0].Provider
	agent.Model = candidates[0].Model

	notes := []string{
		fmt.Sprintf("token optimization enabled: reordered candidate models to prefer lower-cost paths, starting with %s/%s", agent.Provider, agent.Model),
	}
	return agent, notes
}

func candidateCostRank(candidate CandidateModel) int {
	model := strings.ToLower(strings.TrimSpace(candidate.Model))
	rank := 100
	switch {
	case strings.Contains(model, "chat"), strings.Contains(model, "mini"), strings.Contains(model, "flash"):
		rank -= 35
	case strings.Contains(model, "reasoner"), strings.Contains(model, "thinking"), strings.Contains(model, "pro"):
		rank += 25
	}
	if candidate.Weight > 0 {
		rank -= int(candidate.Weight * 10)
	}
	return rank
}

func filterToolSummariesForRequiredTools(summaries, resolvedTools, required []string) ([]string, []string, []string) {
	if len(required) == 0 {
		return summaries, resolvedTools, nil
	}

	requiredSet := map[string]struct{}{}
	for _, item := range cleanStringList(required) {
		requiredSet[strings.ToLower(item)] = struct{}{}
	}

	filteredResolved := make([]string, 0, len(resolvedTools))
	filteredSummaries := make([]string, 0, len(summaries))
	for i, tool := range resolvedTools {
		if matchesRequiredTool(tool, requiredSet) {
			filteredResolved = append(filteredResolved, tool)
			if i < len(summaries) {
				filteredSummaries = append(filteredSummaries, summaries[i])
			}
		}
	}

	if len(filteredResolved) == 0 {
		return summaries, resolvedTools, nil
	}

	return filteredSummaries, filteredResolved, []string{
		fmt.Sprintf("token optimization enabled: pruned tool scope to required tools only (%d active)", len(filteredResolved)),
	}
}

func matchesRequiredTool(resolvedTool string, requiredSet map[string]struct{}) bool {
	resolvedTool = strings.ToLower(strings.TrimSpace(resolvedTool))
	if resolvedTool == "" {
		return false
	}
	for required := range requiredSet {
		switch required {
		case "preset-filesystem-list":
			if strings.Contains(resolvedTool, "list_directory") {
				return true
			}
		case "preset-filesystem-read":
			if strings.Contains(resolvedTool, "read_file") {
				return true
			}
		case "preset-fetch-web":
			if strings.Contains(resolvedTool, "/fetch") {
				return true
			}
		case "preset-browser-open":
			if strings.Contains(resolvedTool, "browser_") {
				return true
			}
		case "preset-github-repo":
			if strings.Contains(resolvedTool, "get_file_contents") {
				return true
			}
		case "preset-sqlite-query":
			if strings.HasSuffix(resolvedTool, "/query") {
				return true
			}
		}
	}
	return false
}
