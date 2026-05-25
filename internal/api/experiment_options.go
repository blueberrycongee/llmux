package api

import "strings"

const (
	candidateStrategyPredictive    = "predictive"
	candidateStrategyGreedyCurrent = "greedy-current"
	candidateStrategyPassive       = "passive"
)

type ExperimentOptions struct {
	ExperimentID            string `json:"experiment_id,omitempty"`
	ExperimentGroup         string `json:"experiment_group,omitempty"`
	BaselineName            string `json:"baseline_name,omitempty"`
	MemoryReuseEnabled      *bool  `json:"memory_reuse_enabled,omitempty"`
	TeamRoutingEnabled      *bool  `json:"team_routing_enabled,omitempty"`
	CandidateStrategy       string `json:"candidate_strategy,omitempty"`
	ToolInjectionEnabled    *bool  `json:"tool_injection_enabled,omitempty"`
	ToolScopePruningEnabled *bool  `json:"tool_scope_pruning_enabled,omitempty"`
	MaxToolIterations       int    `json:"max_tool_iterations,omitempty"`
}

func normalizeCandidateStrategy(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "", candidateStrategyPredictive:
		return candidateStrategyPredictive
	case candidateStrategyGreedyCurrent:
		return candidateStrategyGreedyCurrent
	case candidateStrategyPassive:
		return candidateStrategyPassive
	default:
		return candidateStrategyPredictive
	}
}

func normalizeExperimentOptions(opts ExperimentOptions) ExperimentOptions {
	opts.ExperimentID = strings.TrimSpace(opts.ExperimentID)
	opts.ExperimentGroup = strings.TrimSpace(opts.ExperimentGroup)
	opts.BaselineName = strings.TrimSpace(opts.BaselineName)
	opts.CandidateStrategy = normalizeCandidateStrategy(opts.CandidateStrategy)
	if opts.MaxToolIterations < 0 {
		opts.MaxToolIterations = 0
	}
	return opts
}

func applyExperimentOptionsToProfile(profile *RoutingProfile, opts ExperimentOptions) {
	if profile == nil {
		return
	}
	opts = normalizeExperimentOptions(opts)
	profile.ExperimentID = opts.ExperimentID
	profile.ExperimentGroup = opts.ExperimentGroup
	profile.BaselineName = opts.BaselineName
	if opts.MemoryReuseEnabled != nil {
		profile.MemoryReuseEnabled = *opts.MemoryReuseEnabled
	}
	if opts.TeamRoutingEnabled != nil {
		profile.TeamRoutingEnabled = *opts.TeamRoutingEnabled
	}
	if opts.ToolInjectionEnabled != nil {
		profile.ToolInjectionEnabled = *opts.ToolInjectionEnabled
	}
	if opts.ToolScopePruningEnabled != nil {
		profile.ToolScopePruningEnabled = *opts.ToolScopePruningEnabled
	}
	if opts.CandidateStrategy != "" {
		profile.CandidateStrategy = opts.CandidateStrategy
	}
	if opts.MaxToolIterations > 0 {
		profile.MaxToolIterations = opts.MaxToolIterations
	}
}

func effectiveAgentTools(agent ConversationAgent, profile RoutingProfile) []string {
	if !profile.ToolInjectionEnabled {
		return nil
	}
	return cleanStringList(agent.Tools)
}

func experimentAwareAgent(agent ConversationAgent, profile RoutingProfile) ConversationAgent {
	if profile.ToolInjectionEnabled {
		return agent
	}
	agent.Tools = nil
	return agent
}
