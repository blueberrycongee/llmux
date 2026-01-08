package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/goccy/go-json"
)

// This file contains extended PostgresStore methods to complete the Store interface
// It's split from postgres.go to keep files manageable

// ========================================================================
// Budget Operations
// ========================================================================

// GetBudget retrieves a budget by ID.
func (s *PostgresStore) GetBudget(ctx context.Context, budgetID string) (*Budget, error) {
	query := `
		SELECT id, max_budget, soft_budget, max_parallel_requests, tpm_limit, rpm_limit,
		       model_max_budget, budget_duration, budget_reset_at, created_at, updated_at,
		       created_by, updated_by
		FROM budgets
		WHERE id = $1`

	var budget Budget
	var maxBudget, softBudget sql.NullFloat64
	var maxParallelRequests sql.NullInt32
	var tpmLimit, rpmLimit sql.NullInt64
	var modelMaxBudgetJSON sql.NullString
	var budgetDuration sql.NullString
	var budgetResetAt sql.NullTime
	var createdBy, updatedBy sql.NullString

	err := s.db.QueryRowContext(ctx, query, budgetID).Scan(
		&budget.ID, &maxBudget, &softBudget, &maxParallelRequests, &tpmLimit, &rpmLimit,
		&modelMaxBudgetJSON, &budgetDuration, &budgetResetAt, &budget.CreatedAt, &budget.UpdatedAt,
		&createdBy, &updatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query budget: %w", err)
	}

	if maxBudget.Valid {
		budget.MaxBudget = &maxBudget.Float64
	}
	if softBudget.Valid {
		budget.SoftBudget = &softBudget.Float64
	}
	if maxParallelRequests.Valid {
		val := int(maxParallelRequests.Int32)
		budget.MaxParallelRequests = &val
	}
	if tpmLimit.Valid {
		budget.TPMLimit = &tpmLimit.Int64
	}
	if rpmLimit.Valid {
		budget.RPMLimit = &rpmLimit.Int64
	}
	if budgetDuration.Valid {
		budget.BudgetDuration = BudgetDuration(budgetDuration.String)
	}
	if budgetResetAt.Valid {
		budget.BudgetResetAt = &budgetResetAt.Time
	}
	if createdBy.Valid {
		budget.CreatedBy = createdBy.String
	}
	if updatedBy.Valid {
		budget.UpdatedBy = updatedBy.String
	}
	if modelMaxBudgetJSON.Valid && modelMaxBudgetJSON.String != "" {
		_ = json.Unmarshal([]byte(modelMaxBudgetJSON.String), &budget.ModelMaxBudget)
	}

	return &budget, nil
}

// CreateBudget creates a new budget.
func (s *PostgresStore) CreateBudget(ctx context.Context, budget *Budget) error {
	modelMaxBudgetJSON, _ := json.Marshal(budget.ModelMaxBudget)

	query := `
		INSERT INTO budgets (id, max_budget, soft_budget, max_parallel_requests, tpm_limit, rpm_limit,
		                     model_max_budget, budget_duration, budget_reset_at, created_at, updated_at,
		                     created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err := s.db.ExecContext(ctx, query,
		budget.ID, budget.MaxBudget, budget.SoftBudget, budget.MaxParallelRequests,
		budget.TPMLimit, budget.RPMLimit, string(modelMaxBudgetJSON),
		string(budget.BudgetDuration), budget.BudgetResetAt,
		budget.CreatedAt, budget.UpdatedAt, budget.CreatedBy, budget.UpdatedBy,
	)
	return err
}

// UpdateBudget updates an existing budget.
func (s *PostgresStore) UpdateBudget(ctx context.Context, budget *Budget) error {
	modelMaxBudgetJSON, _ := json.Marshal(budget.ModelMaxBudget)

	query := `
		UPDATE budgets SET
			max_budget = $1, soft_budget = $2, max_parallel_requests = $3, tpm_limit = $4, rpm_limit = $5,
			model_max_budget = $6, budget_duration = $7, budget_reset_at = $8,
			updated_at = $9, updated_by = $10
		WHERE id = $11`

	_, err := s.db.ExecContext(ctx, query,
		budget.MaxBudget, budget.SoftBudget, budget.MaxParallelRequests,
		budget.TPMLimit, budget.RPMLimit, string(modelMaxBudgetJSON),
		string(budget.BudgetDuration), budget.BudgetResetAt,
		time.Now(), budget.UpdatedBy, budget.ID,
	)
	return err
}

// DeleteBudget deletes a budget.
func (s *PostgresStore) DeleteBudget(ctx context.Context, budgetID string) error {
	query := `DELETE FROM budgets WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, budgetID)
	return err
}

// ========================================================================
// Team Operations - Missing Methods
// ========================================================================

// UpdateTeam updates a team.
func (s *PostgresStore) UpdateTeam(ctx context.Context, team *Team) error {
	modelsJSON, _ := json.Marshal(team.Models)
	modelMaxBudgetJSON, _ := json.Marshal(team.ModelMaxBudget)
	modelSpendJSON, _ := json.Marshal(team.ModelSpend)
	metadataJSON, _ := json.Marshal(team.Metadata)

	query := `
		UPDATE teams SET
			team_alias = $1, organization_id = $2, max_budget = $3, spend = $4,
			model_max_budget = $5, model_spend = $6, budget_duration = $7, budget_reset_at = $8,
			tpm_limit = $9, rpm_limit = $10, models = $11, metadata = $12,
			updated_at = $13, is_active = $14, blocked = $15
		WHERE id = $16`

	_, err := s.db.ExecContext(ctx, query,
		team.Alias, team.OrganizationID, team.MaxBudget, team.SpentBudget,
		string(modelMaxBudgetJSON), string(modelSpendJSON), string(team.BudgetDuration), team.BudgetResetAt,
		team.TPMLimit, team.RPMLimit, string(modelsJSON), string(metadataJSON),
		time.Now(), team.IsActive, team.Blocked, team.ID,
	)
	return err
}

// UpdateTeamModelSpent updates the model-specific spend for a team.
func (s *PostgresStore) UpdateTeamModelSpent(ctx context.Context, teamID, model string, amount float64) error {
	query := `
		UPDATE teams 
		SET model_spend = COALESCE(model_spend, '{}'::jsonb) || jsonb_build_object($1, 
			COALESCE((model_spend->>$1)::numeric, 0) + $2)
		WHERE id = $3`
	_, err := s.db.ExecContext(ctx, query, model, amount, teamID)
	return err
}

// ResetTeamBudget resets the budget for a team.
func (s *PostgresStore) ResetTeamBudget(ctx context.Context, teamID string) error {
	query := `
		UPDATE teams 
		SET spend = 0, 
		    model_spend = '{}'::jsonb,
		    budget_reset_at = CASE 
		        WHEN budget_duration = '1d' THEN NOW() + INTERVAL '1 day'
		        WHEN budget_duration = '7d' THEN NOW() + INTERVAL '7 days'
		        WHEN budget_duration = '30d' THEN NOW() + INTERVAL '30 days'
		        ELSE NULL
		    END,
		    updated_at = NOW()
		WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, teamID)
	return err
}

// BlockTeam blocks or unblocks a team.
func (s *PostgresStore) BlockTeam(ctx context.Context, teamID string, blocked bool) error {
	query := `UPDATE teams SET blocked = $1, updated_at = $2 WHERE id = $3`
	_, err := s.db.ExecContext(ctx, query, blocked, time.Now(), teamID)
	return err
}

// ========================================================================
// User Operations - Missing Methods
// ========================================================================

// GetUserBySSOID retrieves a user by SSO ID.
func (s *PostgresStore) GetUserBySSOID(ctx context.Context, ssoID string) (*User, error) {
	query := `
		SELECT id, user_alias, user_email, team_id, organization_id, user_role,
		       max_budget, spend, models, metadata, is_active, created_at, updated_at
		FROM users
		WHERE sso_id = $1 AND is_active = true`

	var user User
	var alias, email, teamID, orgID sql.NullString
	var modelsJSON, metadataJSON sql.NullString
	var createdAt, updatedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, ssoID).Scan(
		&user.ID, &alias, &email, &teamID, &orgID, &user.Role,
		&user.MaxBudget, &user.Spend, &modelsJSON, &metadataJSON,
		&user.IsActive, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user by sso id: %w", err)
	}

	if alias.Valid {
		user.Alias = &alias.String
	}
	if email.Valid {
		user.Email = &email.String
	}
	if teamID.Valid {
		user.TeamID = &teamID.String
	}
	if orgID.Valid {
		user.OrganizationID = &orgID.String
	}
	if createdAt.Valid {
		user.CreatedAt = &createdAt.Time
	}
	if updatedAt.Valid {
		user.UpdatedAt = &updatedAt.Time
	}
	if modelsJSON.Valid && modelsJSON.String != "" {
		_ = json.Unmarshal([]byte(modelsJSON.String), &user.Models)
	}
	if metadataJSON.Valid && metadataJSON.String != "" {
		_ = json.Unmarshal([]byte(metadataJSON.String), &user.Metadata)
	}

	return &user, nil
}

// UpdateUser updates a user.
func (s *PostgresStore) UpdateUser(ctx context.Context, user *User) error {
	modelsJSON, _ := json.Marshal(user.Models)
	modelMaxBudgetJSON, _ := json.Marshal(user.ModelMaxBudget)
	modelSpendJSON, _ := json.Marshal(user.ModelSpend)
	metadataJSON, _ := json.Marshal(user.Metadata)

	query := `
		UPDATE users SET
			user_alias = $1, user_email = $2, team_id = $3, organization_id = $4, user_role = $5,
			max_budget = $6, spend = $7, model_max_budget = $8, model_spend = $9,
			budget_duration = $10, budget_reset_at = $11, tpm_limit = $12, rpm_limit = $13,
			models = $14, metadata = $15, is_active = $16, updated_at = $17
		WHERE id = $18`

	_, err := s.db.ExecContext(ctx, query,
		user.Alias, user.Email, user.TeamID, user.OrganizationID, user.Role,
		user.MaxBudget, user.Spend, string(modelMaxBudgetJSON), string(modelSpendJSON),
		string(user.BudgetDuration), user.BudgetResetAt, user.TPMLimit, user.RPMLimit,
		string(modelsJSON), string(metadataJSON), user.IsActive, time.Now(), user.ID,
	)
	return err
}

// UpdateUserSpent updates the spent amount for a user.
func (s *PostgresStore) UpdateUserSpent(ctx context.Context, userID string, amount float64) error {
	query := `UPDATE users SET spend = spend + $1, updated_at = $2 WHERE id = $3`
	_, err := s.db.ExecContext(ctx, query, amount, time.Now(), userID)
	return err
}

// ResetUserBudget resets the budget for a user.
func (s *PostgresStore) ResetUserBudget(ctx context.Context, userID string) error {
	query := `
		UPDATE users 
		SET spend = 0, 
		    model_spend = '{}'::jsonb,
		    budget_reset_at = CASE 
		        WHEN budget_duration = '1d' THEN NOW() + INTERVAL '1 day'
		        WHEN budget_duration = '7d' THEN NOW() + INTERVAL '7 days'
		        WHEN budget_duration = '30d' THEN NOW() + INTERVAL '30 days'
		        ELSE NULL
		    END,
		    updated_at = NOW()
		WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}

// ListUsers returns users with pagination and filtering.
func (s *PostgresStore) ListUsers(ctx context.Context, filter UserFilter) ([]*User, int64, error) {
	// Build query with filters
	query := `
		SELECT id, user_alias, user_email, team_id, organization_id, user_role,
		       max_budget, spend, is_active, created_at
		FROM users
		WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM users WHERE 1=1`

	args := []interface{}{}
	argIdx := 1

	// Apply filters
	if filter.IsActive != nil {
		query += fmt.Sprintf(" AND is_active = $%d", argIdx)
		countQuery += fmt.Sprintf(" AND is_active = $%d", argIdx)
		args = append(args, *filter.IsActive)
		argIdx++
	}
	if filter.TeamID != nil {
		query += fmt.Sprintf(" AND team_id = $%d", argIdx)
		countQuery += fmt.Sprintf(" AND team_id = $%d", argIdx)
		args = append(args, *filter.TeamID)
		argIdx++
	}
	if filter.OrganizationID != nil {
		query += fmt.Sprintf(" AND organization_id = $%d", argIdx)
		countQuery += fmt.Sprintf(" AND organization_id = $%d", argIdx)
		args = append(args, *filter.OrganizationID)
		argIdx++
	}
	if filter.Role != nil {
		query += fmt.Sprintf(" AND user_role = $%d", argIdx)
		countQuery += fmt.Sprintf(" AND user_role = $%d", argIdx)
		args = append(args, string(*filter.Role))
		argIdx++
	}

	// Get total count
	var total int64
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	// Add pagination
	query += " ORDER BY created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*User
	for rows.Next() {
		var user User
		var alias, email, teamID, orgID sql.NullString
		var createdAt sql.NullTime

		if err := rows.Scan(
			&user.ID, &alias, &email, &teamID, &orgID, &user.Role,
			&user.MaxBudget, &user.Spend, &user.IsActive, &createdAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}

		if alias.Valid {
			user.Alias = &alias.String
		}
		if email.Valid {
			user.Email = &email.String
		}
		if teamID.Valid {
			user.TeamID = &teamID.String
		}
		if orgID.Valid {
			user.OrganizationID = &orgID.String
		}
		if createdAt.Valid {
			user.CreatedAt = &createdAt.Time
		}
		users = append(users, &user)
	}

	return users, total, rows.Err()
}

// ========================================================================
// Organization Operations - Missing Methods
// ========================================================================

// GetOrganization retrieves an organization by ID.
func (s *PostgresStore) GetOrganization(ctx context.Context, orgID string) (*Organization, error) {
	query := `
		SELECT id, organization_alias, budget_id, models, max_budget, spend, model_spend,
		       metadata, created_at, updated_at
		FROM organizations
		WHERE id = $1`

	var org Organization
	var budgetID sql.NullString
	var modelsJSON, modelSpendJSON, metadataJSON sql.NullString

	err := s.db.QueryRowContext(ctx, query, orgID).Scan(
		&org.ID, &org.Alias, &budgetID, &modelsJSON, &org.MaxBudget, &org.Spend,
		&modelSpendJSON, &metadataJSON, &org.CreatedAt, &org.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query organization: %w", err)
	}

	if budgetID.Valid {
		org.BudgetID = &budgetID.String
	}
	if modelsJSON.Valid && modelsJSON.String != "" {
		_ = json.Unmarshal([]byte(modelsJSON.String), &org.Models)
	}
	if modelSpendJSON.Valid && modelSpendJSON.String != "" {
		_ = json.Unmarshal([]byte(modelSpendJSON.String), &org.ModelSpend)
	}
	if metadataJSON.Valid && metadataJSON.String != "" {
		_ = json.Unmarshal([]byte(metadataJSON.String), &org.Metadata)
	}

	return &org, nil
}

// CreateOrganization creates a new organization.
func (s *PostgresStore) CreateOrganization(ctx context.Context, org *Organization) error {
	modelsJSON, _ := json.Marshal(org.Models)
	modelSpendJSON, _ := json.Marshal(org.ModelSpend)
	metadataJSON, _ := json.Marshal(org.Metadata)

	query := `
		INSERT INTO organizations (id, organization_alias, budget_id, models, max_budget, spend,
		                           model_spend, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := s.db.ExecContext(ctx, query,
		org.ID, org.Alias, org.BudgetID, string(modelsJSON), org.MaxBudget, org.Spend,
		string(modelSpendJSON), string(metadataJSON), org.CreatedAt, org.UpdatedAt,
	)
	return err
}

// UpdateOrganization updates an organization.
func (s *PostgresStore) UpdateOrganization(ctx context.Context, org *Organization) error {
	modelsJSON, _ := json.Marshal(org.Models)
	modelSpendJSON, _ := json.Marshal(org.ModelSpend)
	metadataJSON, _ := json.Marshal(org.Metadata)

	query := `
		UPDATE organizations SET
			organization_alias = $1, budget_id = $2, models = $3, max_budget = $4, spend = $5,
			model_spend = $6, metadata = $7, updated_at = $8
		WHERE id = $9`

	_, err := s.db.ExecContext(ctx, query,
		org.Alias, org.BudgetID, string(modelsJSON), org.MaxBudget, org.Spend,
		string(modelSpendJSON), string(metadataJSON), time.Now(), org.ID,
	)
	return err
}

// UpdateOrganizationSpent updates the spent amount for an organization.
func (s *PostgresStore) UpdateOrganizationSpent(ctx context.Context, orgID string, amount float64) error {
	query := `UPDATE organizations SET spend = spend + $1, updated_at = $2 WHERE id = $3`
	_, err := s.db.ExecContext(ctx, query, amount, time.Now(), orgID)
	return err
}

// DeleteOrganization deletes an organization.
func (s *PostgresStore) DeleteOrganization(ctx context.Context, orgID string) error {
	query := `DELETE FROM organizations WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, orgID)
	return err
}

// ListOrganizations returns organizations with pagination.
func (s *PostgresStore) ListOrganizations(ctx context.Context, limit, offset int) ([]*Organization, int64, error) {
	countQuery := `SELECT COUNT(*) FROM organizations`
	var total int64
	err := s.db.QueryRowContext(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count organizations: %w", err)
	}

	query := `
		SELECT id, organization_alias, budget_id, max_budget, spend, created_at, updated_at
		FROM organizations
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query organizations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var orgs []*Organization
	for rows.Next() {
		var org Organization
		var budgetID sql.NullString

		if err := rows.Scan(
			&org.ID, &org.Alias, &budgetID, &org.MaxBudget, &org.Spend,
			&org.CreatedAt, &org.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan organization: %w", err)
		}

		if budgetID.Valid {
			org.BudgetID = &budgetID.String
		}
		orgs = append(orgs, &org)
	}

	return orgs, total, rows.Err()
}

// Continue in next part due to length limits...
