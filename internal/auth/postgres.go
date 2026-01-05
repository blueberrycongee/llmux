package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
		SELECT id, key_hash, key_prefix, name, team_id, user_id, 
		       allowed_models, rate_limit, max_budget, spent_budget,
		       metadata, created_at, expires_at, last_used_at, is_active
		FROM api_keys
		WHERE key_hash = $1`

	var key APIKey
	var allowedModels, metadataJSON sql.NullString
	var teamID, userID sql.NullString
	var expiresAt, lastUsedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, hash).Scan(
		&key.ID, &key.KeyHash, &key.KeyPrefix, &key.Name,
		&teamID, &userID, &allowedModels, &key.RateLimit,
		&key.MaxBudget, &key.SpentBudget, &metadataJSON,
		&key.CreatedAt, &expiresAt, &lastUsedAt, &key.IsActive,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query api key: %w", err)
	}

	// Handle nullable fields
	if teamID.Valid {
		key.TeamID = &teamID.String
	}
	if userID.Valid {
		key.UserID = &userID.String
	}
	if expiresAt.Valid {
		key.ExpiresAt = &expiresAt.Time
	}
	if lastUsedAt.Valid {
		key.LastUsedAt = &lastUsedAt.Time
	}

	// Parse JSON arrays
	if allowedModels.Valid && allowedModels.String != "" {
		if err := json.Unmarshal([]byte(allowedModels.String), &key.AllowedModels); err != nil {
			return nil, fmt.Errorf("parse allowed_models: %w", err)
		}
	}
	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &key.Metadata); err != nil {
			return nil, fmt.Errorf("parse metadata: %w", err)
		}
	}

	return &key, nil
}

// CreateAPIKey inserts a new API key.
func (s *PostgresStore) CreateAPIKey(ctx context.Context, key *APIKey) error {
	allowedModelsJSON, _ := json.Marshal(key.AllowedModels)
	metadataJSON, _ := json.Marshal(key.Metadata)

	query := `
		INSERT INTO api_keys (id, key_hash, key_prefix, name, team_id, user_id,
		                      allowed_models, rate_limit, max_budget, spent_budget,
		                      metadata, created_at, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	_, err := s.db.ExecContext(ctx, query,
		key.ID, key.KeyHash, key.KeyPrefix, key.Name, key.TeamID, key.UserID,
		string(allowedModelsJSON), key.RateLimit, key.MaxBudget, key.SpentBudget,
		string(metadataJSON), key.CreatedAt, key.ExpiresAt, key.IsActive,
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
		SELECT id, key_prefix, name, team_id, rate_limit, max_budget, 
		       spent_budget, created_at, expires_at, last_used_at, is_active
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
		var expiresAt, lastUsedAt sql.NullTime

		if err := rows.Scan(
			&key.ID, &key.KeyPrefix, &key.Name, &teamIDVal,
			&key.RateLimit, &key.MaxBudget, &key.SpentBudget,
			&key.CreatedAt, &expiresAt, &lastUsedAt, &key.IsActive,
		); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}

		if teamIDVal.Valid {
			key.TeamID = &teamIDVal.String
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
		SELECT id, name, max_budget, spent_budget, rate_limit, 
		       metadata, created_at, is_active
		FROM teams
		WHERE id = $1`

	var team Team
	var metadataJSON sql.NullString

	err := s.db.QueryRowContext(ctx, query, teamID).Scan(
		&team.ID, &team.Name, &team.MaxBudget, &team.SpentBudget,
		&team.RateLimit, &metadataJSON, &team.CreatedAt, &team.IsActive,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query team: %w", err)
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &team.Metadata); err != nil {
			return nil, fmt.Errorf("parse metadata: %w", err)
		}
	}

	return &team, nil
}

// CreateTeam inserts a new team.
func (s *PostgresStore) CreateTeam(ctx context.Context, team *Team) error {
	metadataJSON, _ := json.Marshal(team.Metadata)

	query := `
		INSERT INTO teams (id, name, max_budget, spent_budget, rate_limit, 
		                   metadata, created_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := s.db.ExecContext(ctx, query,
		team.ID, team.Name, team.MaxBudget, team.SpentBudget,
		team.RateLimit, string(metadataJSON), team.CreatedAt, team.IsActive,
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
		SELECT id, name, max_budget, spent_budget, rate_limit, created_at, is_active
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
		if err := rows.Scan(
			&team.ID, &team.Name, &team.MaxBudget, &team.SpentBudget,
			&team.RateLimit, &team.CreatedAt, &team.IsActive,
		); err != nil {
			return nil, fmt.Errorf("scan team: %w", err)
		}
		teams = append(teams, &team)
	}
	return teams, rows.Err()
}

// GetUser retrieves a user by ID.
func (s *PostgresStore) GetUser(ctx context.Context, userID string) (*User, error) {
	query := `
		SELECT id, email, name, team_id, role, created_at, is_active
		FROM users
		WHERE id = $1`

	var user User
	var teamID sql.NullString

	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID, &user.Email, &user.Name, &teamID,
		&user.Role, &user.CreatedAt, &user.IsActive,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	if teamID.Valid {
		user.TeamID = &teamID.String
	}
	return &user, nil
}

// GetUserByEmail retrieves a user by email.
func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, name, team_id, role, created_at, is_active
		FROM users
		WHERE email = $1 AND is_active = true`

	var user User
	var teamID sql.NullString

	err := s.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Name, &teamID,
		&user.Role, &user.CreatedAt, &user.IsActive,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	if teamID.Valid {
		user.TeamID = &teamID.String
	}
	return &user, nil
}

// CreateUser inserts a new user.
func (s *PostgresStore) CreateUser(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (id, email, name, team_id, role, created_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := s.db.ExecContext(ctx, query,
		user.ID, user.Email, user.Name, user.TeamID,
		user.Role, user.CreatedAt, user.IsActive,
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
	query := `
		INSERT INTO usage_logs (api_key_id, team_id, model, provider, 
		                        input_tokens, output_tokens, total_tokens,
		                        cost, latency_ms, status_code, request_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := s.db.ExecContext(ctx, query,
		log.APIKeyID, log.TeamID, log.Model, log.Provider,
		log.InputTokens, log.OutputTokens, log.TotalTokens,
		log.Cost, log.LatencyMs, log.StatusCode, log.RequestID, log.CreatedAt,
	)
	return err
}

// GetUsageStats returns aggregated usage statistics.
func (s *PostgresStore) GetUsageStats(ctx context.Context, filter UsageFilter) (*UsageStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_requests,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COALESCE(SUM(cost), 0) as total_cost,
			COALESCE(AVG(latency_ms), 0) as avg_latency_ms,
			COALESCE(AVG(CASE WHEN status_code < 400 THEN 1.0 ELSE 0.0 END), 0) as success_rate,
			COUNT(DISTINCT model) as unique_models,
			COUNT(DISTINCT provider) as unique_providers
		FROM usage_logs
		WHERE created_at >= $1 AND created_at <= $2
			AND ($3::text IS NULL OR api_key_id = $3)
			AND ($4::text IS NULL OR team_id = $4)
			AND ($5::text IS NULL OR model = $5)
			AND ($6::text IS NULL OR provider = $6)`

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
