package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// This file contains extended PostgresStore methods (Part 2)
// Includes: Membership, EndUser, DailyUsage, and Budget Reset operations

// ========================================================================
// Team Membership Operations
// ========================================================================

// GetTeamMembership retrieves a team membership.
func (s *PostgresStore) GetTeamMembership(ctx context.Context, userID, teamID string) (*TeamMembership, error) {
	query := `
		SELECT user_id, team_id, user_role, spend, budget_id, created_at, updated_at
		FROM team_memberships
		WHERE user_id = $1 AND team_id = $2`

	var membership TeamMembership
	var role, budgetID sql.NullString
	var createdAt, updatedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, userID, teamID).Scan(
		&membership.UserID, &membership.TeamID, &role, &membership.Spend,
		&budgetID, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query team membership: %w", err)
	}

	if role.Valid {
		membership.Role = role.String
	}
	if budgetID.Valid {
		membership.BudgetID = &budgetID.String
	}
	if createdAt.Valid {
		membership.JoinedAt = &createdAt.Time
	}

	return &membership, nil
}

// CreateTeamMembership creates a new team membership.
func (s *PostgresStore) CreateTeamMembership(ctx context.Context, membership *TeamMembership) error {
	query := `
		INSERT INTO team_memberships (user_id, team_id, user_role, spend, budget_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	now := time.Now()
	_, err := s.db.ExecContext(ctx, query,
		membership.UserID, membership.TeamID, membership.Role, membership.Spend,
		membership.BudgetID, now, now,
	)
	return err
}

// UpdateTeamMembership updates a team membership.
func (s *PostgresStore) UpdateTeamMembership(ctx context.Context, membership *TeamMembership) error {
	query := `
		UPDATE team_memberships SET
			user_role = $1, spend = $2, budget_id = $3, updated_at = $4
		WHERE user_id = $5 AND team_id = $6`

	_, err := s.db.ExecContext(ctx, query,
		membership.Role, membership.Spend, membership.BudgetID, time.Now(),
		membership.UserID, membership.TeamID,
	)
	return err
}

// UpdateTeamMembershipSpent updates the spent amount for a team membership.
func (s *PostgresStore) UpdateTeamMembershipSpent(ctx context.Context, userID, teamID string, amount float64) error {
	query := `UPDATE team_memberships SET spend = spend + $1, updated_at = $2 WHERE user_id = $3 AND team_id = $4`
	_, err := s.db.ExecContext(ctx, query, amount, time.Now(), userID, teamID)
	return err
}

// DeleteTeamMembership deletes a team membership.
func (s *PostgresStore) DeleteTeamMembership(ctx context.Context, userID, teamID string) error {
	query := `DELETE FROM team_memberships WHERE user_id = $1 AND team_id = $2`
	_, err := s.db.ExecContext(ctx, query, userID, teamID)
	return err
}

// ListTeamMembers returns all members of a team.
func (s *PostgresStore) ListTeamMembers(ctx context.Context, teamID string) ([]*TeamMembership, error) {
	query := `
		SELECT user_id, team_id, user_role, spend, budget_id, created_at
		FROM team_memberships
		WHERE team_id = $1
		ORDER BY created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("query team members: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var members []*TeamMembership
	for rows.Next() {
		var m TeamMembership
		var role, budgetID sql.NullString
		var createdAt sql.NullTime

		if err := rows.Scan(&m.UserID, &m.TeamID, &role, &m.Spend, &budgetID, &createdAt); err != nil {
			return nil, fmt.Errorf("scan team member: %w", err)
		}

		if role.Valid {
			m.Role = role.String
		}
		if budgetID.Valid {
			m.BudgetID = &budgetID.String
		}
		if createdAt.Valid {
			m.JoinedAt = &createdAt.Time
		}
		members = append(members, &m)
	}

	return members, rows.Err()
}

// ListUserTeamMemberships returns all team memberships for a user.
func (s *PostgresStore) ListUserTeamMemberships(ctx context.Context, userID string) ([]*TeamMembership, error) {
	query := `
		SELECT user_id, team_id, user_role, spend, budget_id, created_at
		FROM team_memberships
		WHERE user_id = $1
		ORDER BY created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query user team memberships: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var memberships []*TeamMembership
	for rows.Next() {
		var m TeamMembership
		var role, budgetID sql.NullString
		var createdAt sql.NullTime

		if err := rows.Scan(&m.UserID, &m.TeamID, &role, &m.Spend, &budgetID, &createdAt); err != nil {
			return nil, fmt.Errorf("scan team membership: %w", err)
		}

		if role.Valid {
			m.Role = role.String
		}
		if budgetID.Valid {
			m.BudgetID = &budgetID.String
		}
		if createdAt.Valid {
			m.JoinedAt = &createdAt.Time
		}
		memberships = append(memberships, &m)
	}

	return memberships, rows.Err()
}

// ========================================================================
// Organization Membership Operations
// ========================================================================

// GetOrganizationMembership retrieves an organization membership.
func (s *PostgresStore) GetOrganizationMembership(ctx context.Context, userID, orgID string) (*OrganizationMembership, error) {
	query := `
		SELECT user_id, organization_id, user_role, spend, budget_id, created_at, updated_at
		FROM organization_memberships
		WHERE user_id = $1 AND organization_id = $2`

	var membership OrganizationMembership
	var role, budgetID sql.NullString
	var createdAt, updatedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, userID, orgID).Scan(
		&membership.UserID, &membership.OrganizationID, &role, &membership.Spend,
		&budgetID, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query organization membership: %w", err)
	}

	if role.Valid {
		membership.UserRole = role.String
	}
	if budgetID.Valid {
		membership.BudgetID = &budgetID.String
	}
	if createdAt.Valid {
		membership.JoinedAt = &createdAt.Time
	}

	return &membership, nil
}

// CreateOrganizationMembership creates a new organization membership.
func (s *PostgresStore) CreateOrganizationMembership(ctx context.Context, membership *OrganizationMembership) error {
	query := `
		INSERT INTO organization_memberships (user_id, organization_id, user_role, spend, budget_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	now := time.Now()
	_, err := s.db.ExecContext(ctx, query,
		membership.UserID, membership.OrganizationID, membership.UserRole, membership.Spend,
		membership.BudgetID, now, now,
	)
	return err
}

// UpdateOrganizationMembership updates an organization membership.
func (s *PostgresStore) UpdateOrganizationMembership(ctx context.Context, membership *OrganizationMembership) error {
	query := `
		UPDATE organization_memberships SET
			user_role = $1, spend = $2, budget_id = $3, updated_at = $4
		WHERE user_id = $5 AND organization_id = $6`

	_, err := s.db.ExecContext(ctx, query,
		membership.UserRole, membership.Spend, membership.BudgetID, time.Now(),
		membership.UserID, membership.OrganizationID,
	)
	return err
}

// UpdateOrganizationMembershipSpent updates the spent amount for an organization membership.
func (s *PostgresStore) UpdateOrganizationMembershipSpent(ctx context.Context, userID, orgID string, amount float64) error {
	query := `UPDATE organization_memberships SET spend = spend + $1, updated_at = $2 WHERE user_id = $3 AND organization_id = $4`
	_, err := s.db.ExecContext(ctx, query, amount, time.Now(), userID, orgID)
	return err
}

// DeleteOrganizationMembership deletes an organization membership.
func (s *PostgresStore) DeleteOrganizationMembership(ctx context.Context, userID, orgID string) error {
	query := `DELETE FROM organization_memberships WHERE user_id = $1 AND organization_id = $2`
	_, err := s.db.ExecContext(ctx, query, userID, orgID)
	return err
}

// ListOrganizationMembers returns all members of an organization.
func (s *PostgresStore) ListOrganizationMembers(ctx context.Context, orgID string) ([]*OrganizationMembership, error) {
	query := `
		SELECT user_id, organization_id, user_role, spend, budget_id, created_at
		FROM organization_memberships
		WHERE organization_id = $1
		ORDER BY created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("query organization members: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var members []*OrganizationMembership
	for rows.Next() {
		var m OrganizationMembership
		var role, budgetID sql.NullString
		var createdAt sql.NullTime

		if err := rows.Scan(&m.UserID, &m.OrganizationID, &role, &m.Spend, &budgetID, &createdAt); err != nil {
			return nil, fmt.Errorf("scan organization member: %w", err)
		}

		if role.Valid {
			m.UserRole = role.String
		}
		if budgetID.Valid {
			m.BudgetID = &budgetID.String
		}
		if createdAt.Valid {
			m.JoinedAt = &createdAt.Time
		}
		members = append(members, &m)
	}

	return members, rows.Err()
}

// ListUserOrganizationMemberships returns all organization memberships for a user.
func (s *PostgresStore) ListUserOrganizationMemberships(ctx context.Context, userID string) ([]*OrganizationMembership, error) {
	query := `
		SELECT user_id, organization_id, user_role, spend, budget_id, created_at
		FROM organization_memberships
		WHERE user_id = $1
		ORDER BY created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query user organization memberships: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var memberships []*OrganizationMembership
	for rows.Next() {
		var m OrganizationMembership
		var role, budgetID sql.NullString
		var createdAt sql.NullTime

		if err := rows.Scan(&m.UserID, &m.OrganizationID, &role, &m.Spend, &budgetID, &createdAt); err != nil {
			return nil, fmt.Errorf("scan organization membership: %w", err)
		}

		if role.Valid {
			m.UserRole = role.String
		}
		if budgetID.Valid {
			m.BudgetID = &budgetID.String
		}
		if createdAt.Valid {
			m.JoinedAt = &createdAt.Time
		}
		memberships = append(memberships, &m)
	}

	return memberships, rows.Err()
}

// ========================================================================
// End User Operations
// ========================================================================

// GetEndUser retrieves an end user by ID.
func (s *PostgresStore) GetEndUser(ctx context.Context, userID string) (*EndUser, error) {
	query := `
		SELECT user_id, alias, spend, budget_id, blocked, created_at, updated_at
		FROM end_users
		WHERE user_id = $1`

	var endUser EndUser
	var alias, budgetID sql.NullString
	var createdAt, updatedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&endUser.UserID, &alias, &endUser.Spend, &budgetID, &endUser.Blocked,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query end user: %w", err)
	}

	if alias.Valid {
		endUser.Alias = &alias.String
	}
	if budgetID.Valid {
		endUser.BudgetID = &budgetID.String
	}

	return &endUser, nil
}

// CreateEndUser creates a new end user.
func (s *PostgresStore) CreateEndUser(ctx context.Context, endUser *EndUser) error {
	query := `
		INSERT INTO end_users (user_id, alias, spend, budget_id, blocked, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	now := time.Now()
	metadata := []byte("{}")
	_, err := s.db.ExecContext(ctx, query,
		endUser.UserID, endUser.Alias, endUser.Spend, endUser.BudgetID, endUser.Blocked,
		string(metadata), now, now,
	)
	return err
}

// UpdateEndUserSpent updates the spent amount for an end user.
func (s *PostgresStore) UpdateEndUserSpent(ctx context.Context, userID string, amount float64) error {
	query := `UPDATE end_users SET spend = spend + $1, updated_at = $2 WHERE user_id = $3`
	_, err := s.db.ExecContext(ctx, query, amount, time.Now(), userID)
	return err
}

// BlockEndUser blocks or unblocks an end user.
func (s *PostgresStore) BlockEndUser(ctx context.Context, userID string, blocked bool) error {
	query := `UPDATE end_users SET blocked = $1, updated_at = $2 WHERE user_id = $3`
	_, err := s.db.ExecContext(ctx, query, blocked, time.Now(), userID)
	return err
}

// DeleteEndUser deletes an end user.
func (s *PostgresStore) DeleteEndUser(ctx context.Context, userID string) error {
	query := `DELETE FROM end_users WHERE user_id = $1`
	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}

// ========================================================================
// Daily Usage Operations
// ========================================================================

// GetDailyUsage retrieves daily usage statistics.
func (s *PostgresStore) GetDailyUsage(ctx context.Context, filter DailyUsageFilter) ([]*DailyUsage, error) {
	query := `
		SELECT id, date, api_key_id, team_id, model, provider, prompt_tokens, completion_tokens, spend, api_requests
		FROM daily_usage
		WHERE 1=1`

	args := []interface{}{}
	argIdx := 1

	if filter.StartDate != "" {
		query += fmt.Sprintf(" AND date >= $%d", argIdx)
		args = append(args, filter.StartDate)
		argIdx++
	}
	if filter.EndDate != "" {
		query += fmt.Sprintf(" AND date <= $%d", argIdx)
		args = append(args, filter.EndDate)
		argIdx++
	}
	if filter.APIKeyID != nil {
		query += fmt.Sprintf(" AND api_key_id = $%d", argIdx)
		args = append(args, *filter.APIKeyID)
		argIdx++
	}
	if filter.TeamID != nil {
		query += fmt.Sprintf(" AND team_id = $%d", argIdx)
		args = append(args, *filter.TeamID)
		argIdx++
	}
	if filter.Model != nil {
		query += fmt.Sprintf(" AND model = $%d", argIdx)
		args = append(args, *filter.Model)
		argIdx++
	}
	if filter.Provider != nil {
		query += fmt.Sprintf(" AND provider = $%d", argIdx)
		args = append(args, *filter.Provider)
		argIdx++
	}

	query += " ORDER BY date DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query daily usage: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var usages []*DailyUsage
	for rows.Next() {
		var usage DailyUsage
		var teamID, model, provider sql.NullString

		if err := rows.Scan(
			&usage.ID, &usage.Date, &usage.APIKeyID, &teamID, &model, &provider,
			&usage.InputTokens, &usage.OutputTokens, &usage.Spend, &usage.APIRequests,
		); err != nil {
			return nil, fmt.Errorf("scan daily usage: %w", err)
		}

		if teamID.Valid {
			usage.TeamID = &teamID.String
		}
		if model.Valid {
			usage.Model = &model.String
		}
		if provider.Valid {
			usage.Provider = &provider.String
		}
		usages = append(usages, &usage)
	}

	return usages, rows.Err()
}

// ========================================================================
// Budget Reset Operations
// ========================================================================

// GetKeysNeedingBudgetReset retrieves API keys that need budget reset.
func (s *PostgresStore) GetKeysNeedingBudgetReset(ctx context.Context) ([]*APIKey, error) {
	query := `
		SELECT id, key_prefix, name, team_id, max_budget, spent_budget, budget_duration, budget_reset_at
		FROM api_keys
		WHERE is_active = true 
		  AND budget_reset_at IS NOT NULL 
		  AND budget_reset_at <= NOW()
		ORDER BY budget_reset_at ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query keys needing budget reset: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var keys []*APIKey
	for rows.Next() {
		var key APIKey
		var teamID sql.NullString
		var budgetDuration sql.NullString
		var budgetResetAt sql.NullTime

		if err := rows.Scan(
			&key.ID, &key.KeyPrefix, &key.Name, &teamID, &key.MaxBudget, &key.SpentBudget,
			&budgetDuration, &budgetResetAt,
		); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}

		if teamID.Valid {
			key.TeamID = &teamID.String
		}
		if budgetDuration.Valid {
			key.BudgetDuration = BudgetDuration(budgetDuration.String)
		}
		if budgetResetAt.Valid {
			key.BudgetResetAt = &budgetResetAt.Time
		}
		keys = append(keys, &key)
	}

	return keys, rows.Err()
}

// GetTeamsNeedingBudgetReset retrieves teams that need budget reset.
func (s *PostgresStore) GetTeamsNeedingBudgetReset(ctx context.Context) ([]*Team, error) {
	query := `
		SELECT id, team_alias, organization_id, max_budget, spend, budget_duration, budget_reset_at
		FROM teams
		WHERE is_active = true 
		  AND budget_reset_at IS NOT NULL 
		  AND budget_reset_at <= NOW()
		ORDER BY budget_reset_at ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query teams needing budget reset: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var teams []*Team
	for rows.Next() {
		var team Team
		var alias, orgID sql.NullString
		var budgetDuration sql.NullString
		var budgetResetAt sql.NullTime

		if err := rows.Scan(
			&team.ID, &alias, &orgID, &team.MaxBudget, &team.SpentBudget,
			&budgetDuration, &budgetResetAt,
		); err != nil {
			return nil, fmt.Errorf("scan team: %w", err)
		}

		if alias.Valid {
			team.Alias = &alias.String
		}
		if orgID.Valid {
			team.OrganizationID = &orgID.String
		}
		if budgetDuration.Valid {
			team.BudgetDuration = BudgetDuration(budgetDuration.String)
		}
		if budgetResetAt.Valid {
			team.BudgetResetAt = &budgetResetAt.Time
		}
		teams = append(teams, &team)
	}

	return teams, rows.Err()
}

// GetUsersNeedingBudgetReset retrieves users that need budget reset.
func (s *PostgresStore) GetUsersNeedingBudgetReset(ctx context.Context) ([]*User, error) {
	query := `
		SELECT id, user_alias, user_email, max_budget, spend, budget_duration, budget_reset_at
		FROM users
		WHERE is_active = true 
		  AND budget_reset_at IS NOT NULL 
		  AND budget_reset_at <= NOW()
		ORDER BY budget_reset_at ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query users needing budget reset: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*User
	for rows.Next() {
		var user User
		var alias, email sql.NullString
		var budgetDuration sql.NullString
		var budgetResetAt sql.NullTime

		if err := rows.Scan(
			&user.ID, &alias, &email, &user.MaxBudget, &user.Spend,
			&budgetDuration, &budgetResetAt,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}

		if alias.Valid {
			user.Alias = &alias.String
		}
		if email.Valid {
			user.Email = &email.String
		}
		if budgetDuration.Valid {
			user.BudgetDuration = BudgetDuration(budgetDuration.String)
		}
		if budgetResetAt.Valid {
			user.BudgetResetAt = &budgetResetAt.Time
		}
		users = append(users, &user)
	}

	return users, rows.Err()
}
