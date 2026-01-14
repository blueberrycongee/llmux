// Package metrics provides budget-related Prometheus metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// =============================================================================
// Budget Metrics - Team
// =============================================================================

var (
	// TeamRemainingBudget tracks remaining budget for teams.
	TeamRemainingBudget = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "team_remaining_budget",
			Help:      "Remaining budget for team in USD",
		},
		[]string{"team", "team_alias"},
	)

	// TeamMaxBudget tracks maximum budget for teams.
	TeamMaxBudget = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "team_max_budget",
			Help:      "Maximum budget for team in USD",
		},
		[]string{"team", "team_alias"},
	)

	// TeamBudgetRemainingHours tracks hours until budget reset for teams.
	TeamBudgetRemainingHours = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "team_budget_remaining_hours",
			Help:      "Hours until budget reset for team",
		},
		[]string{"team", "team_alias"},
	)
)

// =============================================================================
// Budget Metrics - Organization
// =============================================================================

var (
	// OrgRemainingBudget tracks remaining budget for organizations.
	OrgRemainingBudget = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "org_remaining_budget",
			Help:      "Remaining budget for organization in USD",
		},
		[]string{"organization_id", "organization_alias"},
	)

	// OrgMaxBudget tracks maximum budget for organizations.
	OrgMaxBudget = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "org_max_budget",
			Help:      "Maximum budget for organization in USD",
		},
		[]string{"organization_id", "organization_alias"},
	)
)

// =============================================================================
// Budget Metrics - Provider
// =============================================================================

var (
	// ProviderRemainingBudget tracks remaining budget for providers.
	ProviderRemainingBudget = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "provider_remaining_budget",
			Help:      "Remaining budget for provider in USD",
		},
		[]string{"api_provider"},
	)
)
