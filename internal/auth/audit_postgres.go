package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/goccy/go-json"
)

// PostgresAuditLogStore implements AuditLogStore using PostgreSQL.
type PostgresAuditLogStore struct {
	db *sql.DB
}

// NewPostgresAuditLogStore creates a new PostgreSQL audit log store.
func NewPostgresAuditLogStore(db *sql.DB) *PostgresAuditLogStore {
	return &PostgresAuditLogStore{db: db}
}

// CreateAuditLog records a new audit log entry.
func (s *PostgresAuditLogStore) CreateAuditLog(log *AuditLog) error {
	// Set ID if not provided
	if log.ID == "" {
		log.ID = generateAuditID()
	}

	// Set timestamp if not provided
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now().UTC()
	}

	beforeValueJSON, _ := json.Marshal(log.BeforeValue)
	afterValueJSON, _ := json.Marshal(log.AfterValue)
	diffJSON, _ := json.Marshal(log.Diff)
	metadataJSON, _ := json.Marshal(log.Metadata)

	query := `
		INSERT INTO audit_logs (
			id, timestamp, actor_id, actor_type, actor_email, actor_ip,
			action, object_type, object_id, team_id, organization_id,
			before_value, after_value, diff, request_id, user_agent, request_uri,
			success, error, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`

	_, err := s.db.ExecContext(context.Background(), query,
		log.ID, log.Timestamp, log.ActorID, log.ActorType, log.ActorEmail, log.ActorIP,
		string(log.Action), string(log.ObjectType), log.ObjectID, log.TeamID, log.OrganizationID,
		string(beforeValueJSON), string(afterValueJSON), string(diffJSON),
		log.RequestID, log.UserAgent, log.RequestURI,
		log.Success, log.Error, string(metadataJSON),
	)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}

	return nil
}

// GetAuditLog retrieves a single audit log by ID.
func (s *PostgresAuditLogStore) GetAuditLog(id string) (*AuditLog, error) {
	query := `
		SELECT id, timestamp, actor_id, actor_type, actor_email, actor_ip,
		       action, object_type, object_id, team_id, organization_id,
		       before_value, after_value, diff, request_id, user_agent, request_uri,
		       success, error, metadata
		FROM audit_logs
		WHERE id = $1`

	var log AuditLog
	var actorEmail, actorIP sql.NullString
	var teamID, orgID sql.NullString
	var beforeValue, afterValue, diff, metadata sql.NullString
	var requestID, userAgent, requestURI sql.NullString
	var errorMsg sql.NullString

	err := s.db.QueryRowContext(context.Background(), query, id).Scan(
		&log.ID, &log.Timestamp, &log.ActorID, &log.ActorType, &actorEmail, &actorIP,
		&log.Action, &log.ObjectType, &log.ObjectID, &teamID, &orgID,
		&beforeValue, &afterValue, &diff, &requestID, &userAgent, &requestURI,
		&log.Success, &errorMsg, &metadata,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query audit log: %w", err)
	}

	// Handle nullable fields
	if actorEmail.Valid {
		log.ActorEmail = actorEmail.String
	}
	if actorIP.Valid {
		log.ActorIP = actorIP.String
	}
	if teamID.Valid {
		log.TeamID = &teamID.String
	}
	if orgID.Valid {
		log.OrganizationID = &orgID.String
	}
	if requestID.Valid {
		log.RequestID = requestID.String
	}
	if userAgent.Valid {
		log.UserAgent = userAgent.String
	}
	if requestURI.Valid {
		log.RequestURI = requestURI.String
	}
	if errorMsg.Valid {
		log.Error = errorMsg.String
	}

	// Parse JSON fields
	if beforeValue.Valid && beforeValue.String != "" && beforeValue.String != "null" {
		_ = json.Unmarshal([]byte(beforeValue.String), &log.BeforeValue)
	}
	if afterValue.Valid && afterValue.String != "" && afterValue.String != "null" {
		_ = json.Unmarshal([]byte(afterValue.String), &log.AfterValue)
	}
	if diff.Valid && diff.String != "" && diff.String != "null" {
		_ = json.Unmarshal([]byte(diff.String), &log.Diff)
	}
	if metadata.Valid && metadata.String != "" && metadata.String != "null" {
		_ = json.Unmarshal([]byte(metadata.String), &log.Metadata)
	}

	return &log, nil
}

// ListAuditLogs returns audit logs matching the filter.
func (s *PostgresAuditLogStore) ListAuditLogs(filter AuditLogFilter) ([]*AuditLog, int64, error) {
	// Build base query with filters
	query := `
		SELECT id, timestamp, actor_id, actor_type, actor_email, actor_ip,
		       action, object_type, object_id, team_id, organization_id,
		       success, request_id, user_agent
		FROM audit_logs
		WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM audit_logs WHERE 1=1`

	args := []interface{}{}
	argIdx := 1

	// Apply filters
	if !filter.StartTime.IsZero() {
		query += fmt.Sprintf(" AND timestamp >= $%d", argIdx)
		countQuery += fmt.Sprintf(" AND timestamp >= $%d", argIdx)
		args = append(args, filter.StartTime)
		argIdx++
	}
	if !filter.EndTime.IsZero() {
		query += fmt.Sprintf(" AND timestamp <= $%d", argIdx)
		countQuery += fmt.Sprintf(" AND timestamp <= $%d", argIdx)
		args = append(args, filter.EndTime)
		argIdx++
	}
	if filter.ActorID != nil {
		query += fmt.Sprintf(" AND actor_id = $%d", argIdx)
		countQuery += fmt.Sprintf(" AND actor_id = $%d", argIdx)
		args = append(args, *filter.ActorID)
		argIdx++
	}
	if filter.ActorType != nil {
		query += fmt.Sprintf(" AND actor_type = $%d", argIdx)
		countQuery += fmt.Sprintf(" AND actor_type = $%d", argIdx)
		args = append(args, *filter.ActorType)
		argIdx++
	}
	if filter.Action != nil {
		query += fmt.Sprintf(" AND action = $%d", argIdx)
		countQuery += fmt.Sprintf(" AND action = $%d", argIdx)
		args = append(args, string(*filter.Action))
		argIdx++
	}
	if filter.ObjectType != nil {
		query += fmt.Sprintf(" AND object_type = $%d", argIdx)
		countQuery += fmt.Sprintf(" AND object_type = $%d", argIdx)
		args = append(args, string(*filter.ObjectType))
		argIdx++
	}
	if filter.ObjectID != nil {
		query += fmt.Sprintf(" AND object_id = $%d", argIdx)
		countQuery += fmt.Sprintf(" AND object_id = $%d", argIdx)
		args = append(args, *filter.ObjectID)
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
	if filter.Success != nil {
		query += fmt.Sprintf(" AND success = $%d", argIdx)
		countQuery += fmt.Sprintf(" AND success = $%d", argIdx)
		args = append(args, *filter.Success)
		argIdx++
	}

	// Get total count
	var total int64
	err := s.db.QueryRowContext(context.Background(), countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	// Add ordering and pagination
	query += " ORDER BY timestamp DESC"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		args = append(args, filter.Limit, filter.Offset)
	}

	rows, err := s.db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query audit logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var logs []*AuditLog
	for rows.Next() {
		var log AuditLog
		var actorEmail, actorIP sql.NullString
		var teamID, orgID sql.NullString
		var requestID, userAgent sql.NullString

		if err := rows.Scan(
			&log.ID, &log.Timestamp, &log.ActorID, &log.ActorType, &actorEmail, &actorIP,
			&log.Action, &log.ObjectType, &log.ObjectID, &teamID, &orgID,
			&log.Success, &requestID, &userAgent,
		); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}

		if actorEmail.Valid {
			log.ActorEmail = actorEmail.String
		}
		if actorIP.Valid {
			log.ActorIP = actorIP.String
		}
		if teamID.Valid {
			log.TeamID = &teamID.String
		}
		if orgID.Valid {
			log.OrganizationID = &orgID.String
		}
		if requestID.Valid {
			log.RequestID = requestID.String
		}
		if userAgent.Valid {
			log.UserAgent = userAgent.String
		}

		logs = append(logs, &log)
	}

	return logs, total, rows.Err()
}

// GetAuditLogStats returns aggregated audit statistics.
func (s *PostgresAuditLogStore) GetAuditLogStats(filter AuditLogFilter) (*AuditLogStats, error) {
	// Build base query with filters
	baseWhere := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if !filter.StartTime.IsZero() {
		baseWhere += fmt.Sprintf(" AND timestamp >= $%d", argIdx)
		args = append(args, filter.StartTime)
		argIdx++
	}
	if !filter.EndTime.IsZero() {
		baseWhere += fmt.Sprintf(" AND timestamp <= $%d", argIdx)
		args = append(args, filter.EndTime)
		argIdx++
	}
	if filter.ActorID != nil {
		baseWhere += fmt.Sprintf(" AND actor_id = $%d", argIdx)
		args = append(args, *filter.ActorID)
		argIdx++
	}
	if filter.TeamID != nil {
		baseWhere += fmt.Sprintf(" AND team_id = $%d", argIdx)
		args = append(args, *filter.TeamID)
		argIdx++
	}
	if filter.OrganizationID != nil {
		baseWhere += fmt.Sprintf(" AND organization_id = $%d", argIdx)
		args = append(args, *filter.OrganizationID)
	}

	stats := &AuditLogStats{
		ActionCounts:     make(map[string]int64),
		ObjectTypeCounts: make(map[string]int64),
	}

	// Get basic counts
	basicQuery := fmt.Sprintf(`
		SELECT 
			COUNT(*) as total_events,
			COUNT(*) FILTER (WHERE success = true) as success_count,
			COUNT(*) FILTER (WHERE success = false) as failure_count,
			COUNT(DISTINCT actor_id) as unique_actors
		FROM audit_logs %s`, baseWhere)

	err := s.db.QueryRowContext(context.Background(), basicQuery, args...).Scan(
		&stats.TotalEvents, &stats.SuccessCount, &stats.FailureCount, &stats.UniqueActors,
	)
	if err != nil {
		return nil, fmt.Errorf("query basic stats: %w", err)
	}

	// Get action counts
	actionQuery := fmt.Sprintf(`
		SELECT action, COUNT(*) as count
		FROM audit_logs %s
		GROUP BY action`, baseWhere)

	actionRows, err := s.db.QueryContext(context.Background(), actionQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query action counts: %w", err)
	}
	defer func() { _ = actionRows.Close() }()

	for actionRows.Next() {
		var action string
		var count int64
		if scanErr := actionRows.Scan(&action, &count); scanErr != nil {
			return nil, fmt.Errorf("scan action count: %w", scanErr)
		}
		stats.ActionCounts[action] = count
	}

	// Get object type counts
	objectQuery := fmt.Sprintf(`
		SELECT object_type, COUNT(*) as count
		FROM audit_logs %s
		GROUP BY object_type`, baseWhere)

	objectRows, err := s.db.QueryContext(context.Background(), objectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query object type counts: %w", err)
	}
	defer func() { _ = objectRows.Close() }()

	for objectRows.Next() {
		var objectType string
		var count int64
		if scanErr := objectRows.Scan(&objectType, &count); scanErr != nil {
			return nil, fmt.Errorf("scan object type count: %w", scanErr)
		}
		stats.ObjectTypeCounts[objectType] = count
	}

	return stats, nil
}

// DeleteAuditLogs deletes audit logs older than the specified time.
func (s *PostgresAuditLogStore) DeleteAuditLogs(olderThan time.Time) (int64, error) {
	query := `DELETE FROM audit_logs WHERE timestamp < $1`
	result, err := s.db.ExecContext(context.Background(), query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("delete audit logs: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get rows affected: %w", err)
	}

	return deleted, nil
}

// Ensure PostgresAuditLogStore implements AuditLogStore
var _ AuditLogStore = (*PostgresAuditLogStore)(nil)
