package auth

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/goccy/go-json"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// PostgresStore implements Store using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// PostgresConfig contains PostgreSQL connection settings.
type PostgresConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	Database     string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
	ConnLifetime time.Duration
}

// DefaultPostgresConfig returns sensible defaults.
func DefaultPostgresConfig() *PostgresConfig {
	return &PostgresConfig{
		Host:         "localhost",
		Port:         5432,
		Database:     "llmux",
		SSLMode:      "disable",
		MaxOpenConns: 25,
		MaxIdleConns: 5,
		ConnLifetime: 5 * time.Minute,
	}
}

// NewPostgresStore creates a new PostgreSQL store.
func NewPostgresStore(cfg *PostgresConfig) (*PostgresStore, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnLifetime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

// Ping checks database connectivity.
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// Close closes the database connection.
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// GetAPIKeyByHash retrieves an API key by its hash.
func (s *PostgresStore) GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error) {
	query := `
		SELECT id, key_hash, key_prefix, name, key_alias, team_id, user_id, organization_id,
		       allowed_models, tpm_limit, rpm_limit, max_budget, soft_budget, spent_budget,
		       model_max_budget, model_spend, budget_duration, budget_reset_at,
		       metadata, created_at, updated_at, expires_at, last_used_at, is_active, blocked
		FROM api_keys
		WHERE key_hash = $1`

	var key APIKey
	var allowedModels, modelMaxBudget, modelSpend, metadataJSON sql.NullString
	var keyAlias, teamID, userID, orgID sql.NullString
	var tpmLimit, rpmLimit sql.NullInt64
	var softBudget sql.NullFloat64
	var budgetDuration sql.NullString
	var budgetResetAt, expiresAt, lastUsedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, hash).Scan(
		&key.ID, &key.KeyHash, &key.KeyPrefix, &key.Name, &keyAlias,
		&teamID, &userID, &orgID, &allowedModels, &tpmLimit, &rpmLimit,
		&key.MaxBudget, &softBudget, &key.SpentBudget,
		&modelMaxBudget, &modelSpend, &budgetDuration, &budgetResetAt,
		&metadataJSON, &key.CreatedAt, &key.UpdatedAt, &expiresAt, &lastUsedAt,
		&key.IsActive, &key.Blocked,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query api key: %w", err)
	}

	// Handle nullable fields
	if keyAlias.Valid {
		key.KeyAlias = &keyAlias.String
	}
	if teamID.Valid {
		key.TeamID = &teamID.String
	}
	if userID.Valid {
		key.UserID = &userID.String
	}
	if orgID.Valid {
		key.OrganizationID = &orgID.String
	}
	if tpmLimit.Valid {
		key.TPMLimit = &tpmLimit.Int64
	}
	if rpmLimit.Valid {
		key.RPMLimit = &rpmLimit.Int64
	}
	if softBudget.Valid {
		key.SoftBudget = &softBudget.Float64
	}
	if budgetDuration.Valid {
		key.BudgetDuration = BudgetDuration(budgetDuration.String)
	}
	if budgetResetAt.Valid {
		key.BudgetResetAt = &budgetResetAt.Time
	}
	if expiresAt.Valid {
		key.ExpiresAt = &expiresAt.Time
	}
	if lastUsedAt.Valid {
		key.LastUsedAt = &lastUsedAt.Time
	}

	// Parse JSON fields
	if allowedModels.Valid && allowedModels.String != "" {
		if err := json.Unmarshal([]byte(allowedModels.String), &key.AllowedModels); err != nil {
			// Log but don't fail - use empty slice
			key.AllowedModels = nil
		}
	}
	if modelMaxBudget.Valid && modelMaxBudget.String != "" {
		if err := json.Unmarshal([]byte(modelMaxBudget.String), &key.ModelMaxBudget); err != nil {
			key.ModelMaxBudget = nil
		}
	}
	if modelSpend.Valid && modelSpend.String != "" {
		if err := json.Unmarshal([]byte(modelSpend.String), &key.ModelSpend); err != nil {
			key.ModelSpend = nil
		}
	}
	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &key.Metadata); err != nil {
			key.Metadata = nil
		}
	}

	return &key, nil
}

// CreateAPIKey inserts a new API key.
func (s *PostgresStore) CreateAPIKey(ctx context.Context, key *APIKey) error {
	allowedModelsJSON, err := json.Marshal(key.AllowedModels)
	if err != nil {
		allowedModelsJSON = []byte("[]")
	}
	modelMaxBudgetJSON, err := json.Marshal(key.ModelMaxBudget)
	if err != nil {
		modelMaxBudgetJSON = []byte("{}")
	}
	modelSpendJSON, err := json.Marshal(key.ModelSpend)
	if err != nil {
		modelSpendJSON = []byte("{}")
	}
	metadataJSON, err := json.Marshal(key.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	query := `
		INSERT INTO api_keys (id, key_hash, key_prefix, name, key_alias, team_id, user_id, organization_id,
		                      allowed_models, tpm_limit, rpm_limit, max_budget, soft_budget, spent_budget,
		                      model_max_budget, model_spend, budget_duration, budget_reset_at,
		                      metadata, created_at, updated_at, expires_at, is_active, blocked)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)`

	_, err = s.db.ExecContext(ctx, query,
		key.ID, key.KeyHash, key.KeyPrefix, key.Name, key.KeyAlias,
		key.TeamID, key.UserID, key.OrganizationID,
		string(allowedModelsJSON), key.TPMLimit, key.RPMLimit,
		key.MaxBudget, key.SoftBudget, key.SpentBudget,
		string(modelMaxBudgetJSON), string(modelSpendJSON),
		string(key.BudgetDuration), key.BudgetResetAt,
		string(metadataJSON), key.CreatedAt, key.UpdatedAt, key.ExpiresAt,
		key.IsActive, key.Blocked,
	)
	if err != nil {
		return fmt.Errorf("insert api key: %w", err)
	}
	return nil
}

// UpdateAPIKeyLastUsed updates the last_used_at timestamp.
func (s *PostgresStore) UpdateAPIKeyLastUsed(ctx context.Context, keyID string, lastUsed time.Time) error {
	query := `UPDATE api_keys SET last_used_at = $1 WHERE id = $2`
	_, err := s.db.ExecContext(ctx, query, lastUsed, keyID)
	return err
}

// UpdateAPIKeySpent adds to the spent_budget.
func (s *PostgresStore) UpdateAPIKeySpent(ctx context.Context, keyID string, amount float64) error {
	query := `UPDATE api_keys SET spent_budget = spent_budget + $1 WHERE id = $2`
	_, err := s.db.ExecContext(ctx, query, amount, keyID)
	return err
}

// DeleteAPIKey soft-deletes an API key.
func (s *PostgresStore) DeleteAPIKey(ctx context.Context, keyID string) error {
	query := `UPDATE api_keys SET is_active = false WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, keyID)
	return err
}

// ListAPIKeys returns API keys with pagination.
func (s *PostgresStore) ListAPIKeys(ctx context.Context, teamID *string, limit, offset int) ([]*APIKey, error) {
	query := `
		SELECT id, key_prefix, name, team_id, tpm_limit, rpm_limit, max_budget, 
		       spent_budget, created_at, expires_at, last_used_at, is_active, blocked
		FROM api_keys
		WHERE is_active = true AND ($1::text IS NULL OR team_id = $1)
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, teamID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query api keys: %w", err)
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		var key APIKey
		var teamIDVal sql.NullString
		var tpmLimit, rpmLimit sql.NullInt64
		var expiresAt, lastUsedAt sql.NullTime

		if err := rows.Scan(
			&key.ID, &key.KeyPrefix, &key.Name, &teamIDVal,
			&tpmLimit, &rpmLimit, &key.MaxBudget, &key.SpentBudget,
			&key.CreatedAt, &expiresAt, &lastUsedAt, &key.IsActive, &key.Blocked,
		); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}

		if teamIDVal.Valid {
			key.TeamID = &teamIDVal.String
		}
		if tpmLimit.Valid {
			key.TPMLimit = &tpmLimit.Int64
		}
		if rpmLimit.Valid {
			key.RPMLimit = &rpmLimit.Int64
		}
		if expiresAt.Valid {
			key.ExpiresAt = &expiresAt.Time
		}
		if lastUsedAt.Valid {
			key.LastUsedAt = &lastUsedAt.Time
		}
		keys = append(keys, &key)
	}
	return keys, rows.Err()
}

// GetTeam retrieves a team by ID.
func (s *PostgresStore) GetTeam(ctx context.Context, teamID string) (*Team, error) {
	query := `
		SELECT id, team_alias, organization_id, max_budget, spend, 
		       tpm_limit, rpm_limit, models, metadata, created_at, updated_at, is_active, blocked
		FROM teams
		WHERE id = $1`

	var team Team
	var alias, orgID sql.NullString
	var tpmLimit, rpmLimit sql.NullInt64
	var models, metadataJSON sql.NullString

	err := s.db.QueryRowContext(ctx, query, teamID).Scan(
		&team.ID, &alias, &orgID, &team.MaxBudget, &team.SpentBudget,
		&tpmLimit, &rpmLimit, &models, &metadataJSON,
		&team.CreatedAt, &team.UpdatedAt, &team.IsActive, &team.Blocked,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query team: %w", err)
	}

	if alias.Valid {
		team.Alias = &alias.String
	}
	if orgID.Valid {
		team.OrganizationID = &orgID.String
	}
	if tpmLimit.Valid {
		team.TPMLimit = &tpmLimit.Int64
	}
	if rpmLimit.Valid {
		team.RPMLimit = &rpmLimit.Int64
	}
	if models.Valid && models.String != "" {
		if err := json.Unmarshal([]byte(models.String), &team.Models); err != nil {
			team.Models = nil
		}
	}
	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &team.Metadata); err != nil {
			team.Metadata = nil
		}
	}

	return &team, nil
}

// CreateTeam inserts a new team.
func (s *PostgresStore) CreateTeam(ctx context.Context, team *Team) error {
	modelsJSON, err := json.Marshal(team.Models)
	if err != nil {
		modelsJSON = []byte("[]")
	}
	metadataJSON, err := json.Marshal(team.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	query := `
		INSERT INTO teams (id, team_alias, organization_id, max_budget, spend, 
		                   tpm_limit, rpm_limit, models, metadata, created_at, updated_at, is_active, blocked)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err = s.db.ExecContext(ctx, query,
		team.ID, team.Alias, team.OrganizationID, team.MaxBudget, team.SpentBudget,
		team.TPMLimit, team.RPMLimit, string(modelsJSON), string(metadataJSON),
		team.CreatedAt, team.UpdatedAt, team.IsActive, team.Blocked,
	)
	return err
}

// UpdateTeamSpent adds to the team's spent_budget.
func (s *PostgresStore) UpdateTeamSpent(ctx context.Context, teamID string, amount float64) error {
	query := `UPDATE teams SET spent_budget = spent_budget + $1 WHERE id = $2`
	_, err := s.db.ExecContext(ctx, query, amount, teamID)
	return err
}

// DeleteTeam soft-deletes a team.
func (s *PostgresStore) DeleteTeam(ctx context.Context, teamID string) error {
	query := `UPDATE teams SET is_active = false WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, teamID)
	return err
}

// ListTeams returns teams with pagination.
func (s *PostgresStore) ListTeams(ctx context.Context, limit, offset int) ([]*Team, error) {
	query := `
		SELECT id, team_alias, max_budget, spend, tpm_limit, rpm_limit, created_at, is_active, blocked
		FROM teams
		WHERE is_active = true
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query teams: %w", err)
	}
	defer rows.Close()

	var teams []*Team
	for rows.Next() {
		var team Team
		var alias sql.NullString
		var tpmLimit, rpmLimit sql.NullInt64
		if err := rows.Scan(
			&team.ID, &alias, &team.MaxBudget, &team.SpentBudget,
			&tpmLimit, &rpmLimit, &team.CreatedAt, &team.IsActive, &team.Blocked,
		); err != nil {
			return nil, fmt.Errorf("scan team: %w", err)
		}
		if alias.Valid {
			team.Alias = &alias.String
		}
		if tpmLimit.Valid {
			team.TPMLimit = &tpmLimit.Int64
		}
		if rpmLimit.Valid {
			team.RPMLimit = &rpmLimit.Int64
		}
		teams = append(teams, &team)
	}
	return teams, rows.Err()
}

// GetUser retrieves a user by ID.
func (s *PostgresStore) GetUser(ctx context.Context, userID string) (*User, error) {
	query := `
		SELECT id, user_alias, user_email, team_id, organization_id, user_role,
		       max_budget, spend, is_active, created_at, updated_at
		FROM users
		WHERE id = $1`

	var user User
	var alias, email, teamID, orgID sql.NullString
	var createdAt, updatedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID, &alias, &email, &teamID, &orgID, &user.Role,
		&user.MaxBudget, &user.Spend, &user.IsActive, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
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
	return &user, nil
}

// GetUserByEmail retrieves a user by email.
func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, user_alias, user_email, team_id, organization_id, user_role,
		       max_budget, spend, is_active, created_at
		FROM users
		WHERE user_email = $1 AND is_active = true`

	var user User
	var alias, teamID, orgID sql.NullString
	var createdAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &alias, &user.Email, &teamID, &orgID, &user.Role,
		&user.MaxBudget, &user.Spend, &user.IsActive, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	if alias.Valid {
		user.Alias = &alias.String
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
	return &user, nil
}

// CreateUser inserts a new user.
func (s *PostgresStore) CreateUser(ctx context.Context, user *User) error {
	modelsJSON, err := json.Marshal(user.Models)
	if err != nil {
		modelsJSON = []byte("[]")
	}
	metadataJSON, err := json.Marshal(user.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	query := `
		INSERT INTO users (id, user_alias, user_email, team_id, organization_id, user_role,
		                   max_budget, spend, models, metadata, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err = s.db.ExecContext(ctx, query,
		user.ID, user.Alias, user.Email, user.TeamID, user.OrganizationID, user.Role,
		user.MaxBudget, user.Spend, string(modelsJSON), string(metadataJSON),
		user.IsActive, user.CreatedAt, user.UpdatedAt,
	)
	return err
}

// DeleteUser soft-deletes a user.
func (s *PostgresStore) DeleteUser(ctx context.Context, userID string) error {
	query := `UPDATE users SET is_active = false WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}

// LogUsage records API usage.
func (s *PostgresStore) LogUsage(ctx context.Context, log *UsageLog) error {
	tagsJSON, err := json.Marshal(log.RequestTags)
	if err != nil {
		tagsJSON = []byte("[]")
	}
	metadataJSON, err := json.Marshal(log.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	query := `
		INSERT INTO usage_logs (request_id, api_key, team_id, organization_id, "user", end_user,
		                        model, model_group, custom_llm_provider, call_type,
		                        prompt_tokens, completion_tokens, total_tokens, spend,
		                        latency_ms, status_code, status, cache_hit, request_tags,
		                        metadata, "startTime", "endTime")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`

	_, err = s.db.ExecContext(ctx, query,
		log.RequestID, log.APIKeyID, log.TeamID, log.OrganizationID, log.UserID, log.EndUserID,
		log.Model, log.ModelGroup, log.Provider, log.CallType,
		log.InputTokens, log.OutputTokens, log.TotalTokens, log.Cost,
		log.LatencyMs, log.StatusCode, log.Status, log.CacheHit, string(tagsJSON),
		string(metadataJSON), log.StartTime, log.EndTime,
	)
	return err
}

// GetUsageStats returns aggregated usage statistics.
func (s *PostgresStore) GetUsageStats(ctx context.Context, filter UsageFilter) (*UsageStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_requests,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(prompt_tokens), 0) as input_tokens,
			COALESCE(SUM(completion_tokens), 0) as output_tokens,
			COALESCE(SUM(spend), 0) as total_cost,
			COALESCE(AVG(latency_ms), 0) as avg_latency_ms,
			COALESCE(AVG(CASE WHEN status_code IS NULL OR status_code < 400 THEN 1.0 ELSE 0.0 END), 0) as success_rate,
			COUNT(DISTINCT model) as unique_models,
			COUNT(DISTINCT custom_llm_provider) as unique_providers
		FROM usage_logs
		WHERE "startTime" >= $1 AND "startTime" <= $2
			AND ($3::text IS NULL OR api_key = $3)
			AND ($4::text IS NULL OR team_id = $4)
			AND ($5::text IS NULL OR model = $5)
			AND ($6::text IS NULL OR custom_llm_provider = $6)`

	var stats UsageStats
	err := s.db.QueryRowContext(ctx, query,
		filter.StartTime, filter.EndTime,
		filter.APIKeyID, filter.TeamID, filter.Model, filter.Provider,
	).Scan(
		&stats.TotalRequests, &stats.TotalTokens, &stats.InputTokens,
		&stats.OutputTokens, &stats.TotalCost, &stats.AvgLatencyMs,
		&stats.SuccessRate, &stats.UniqueModels, &stats.UniqueProviders,
	)
	if err != nil {
		return nil, fmt.Errorf("query usage stats: %w", err)
	}
	return &stats, nil
}
