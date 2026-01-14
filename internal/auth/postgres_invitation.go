package auth

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/goccy/go-json"
)

func (s *PostgresStore) CreateInvitationLink(ctx context.Context, link *InvitationLink) error {
	metadataJSON, err := json.Marshal(link.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	query := `
		INSERT INTO invitation_links (
			id, token_hash, team_id, organization_id,
			role, max_uses, current_uses, max_budget,
			expires_at, is_active,
			created_by, created_at, updated_at,
			description, metadata
		)
		VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10,
			$11, $12, $13,
			$14, $15
		)`

	_, err = s.db.ExecContext(ctx, query,
		link.ID,
		link.Token,
		link.TeamID,
		link.OrganizationID,
		link.Role,
		link.MaxUses,
		link.CurrentUses,
		link.MaxBudget,
		link.ExpiresAt,
		link.IsActive,
		link.CreatedBy,
		link.CreatedAt,
		link.UpdatedAt,
		link.Description,
		string(metadataJSON),
	)
	return err
}

func (s *PostgresStore) GetInvitationLink(ctx context.Context, id string) (*InvitationLink, error) {
	query := `
		SELECT id, token_hash, team_id, organization_id,
		       role, max_uses, current_uses, max_budget,
		       expires_at, is_active,
		       created_by, created_at, updated_at,
		       description, metadata
		FROM invitation_links
		WHERE id = $1`

	return s.scanInvitationLinkRow(ctx, query, id)
}

func (s *PostgresStore) GetInvitationLinkByToken(ctx context.Context, tokenHash string) (*InvitationLink, error) {
	query := `
		SELECT id, token_hash, team_id, organization_id,
		       role, max_uses, current_uses, max_budget,
		       expires_at, is_active,
		       created_by, created_at, updated_at,
		       description, metadata
		FROM invitation_links
		WHERE token_hash = $1`

	return s.scanInvitationLinkRow(ctx, query, tokenHash)
}

func (s *PostgresStore) scanInvitationLinkRow(ctx context.Context, query string, arg any) (*InvitationLink, error) {
	var link InvitationLink
	var teamID, orgID sql.NullString
	var maxBudget sql.NullFloat64
	var expiresAt sql.NullTime
	var description, metadataJSON sql.NullString

	err := s.db.QueryRowContext(ctx, query, arg).Scan(
		&link.ID,
		&link.Token,
		&teamID,
		&orgID,
		&link.Role,
		&link.MaxUses,
		&link.CurrentUses,
		&maxBudget,
		&expiresAt,
		&link.IsActive,
		&link.CreatedBy,
		&link.CreatedAt,
		&link.UpdatedAt,
		&description,
		&metadataJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query invitation link: %w", err)
	}

	if teamID.Valid {
		link.TeamID = &teamID.String
	}
	if orgID.Valid {
		link.OrganizationID = &orgID.String
	}
	if maxBudget.Valid {
		link.MaxBudget = &maxBudget.Float64
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		link.ExpiresAt = &t
	}
	if description.Valid {
		link.Description = description.String
	}
	if metadataJSON.Valid && metadataJSON.String != "" {
		_ = json.Unmarshal([]byte(metadataJSON.String), &link.Metadata)
	}

	return &link, nil
}

func (s *PostgresStore) UpdateInvitationLink(ctx context.Context, link *InvitationLink) error {
	metadataJSON, err := json.Marshal(link.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	query := `
		UPDATE invitation_links
		SET token_hash = $1,
		    team_id = $2,
		    organization_id = $3,
		    role = $4,
		    max_uses = $5,
		    current_uses = $6,
		    max_budget = $7,
		    expires_at = $8,
		    is_active = $9,
		    created_by = $10,
		    created_at = $11,
		    updated_at = $12,
		    description = $13,
		    metadata = $14
		WHERE id = $15`

	_, err = s.db.ExecContext(ctx, query,
		link.Token,
		link.TeamID,
		link.OrganizationID,
		link.Role,
		link.MaxUses,
		link.CurrentUses,
		link.MaxBudget,
		link.ExpiresAt,
		link.IsActive,
		link.CreatedBy,
		link.CreatedAt,
		link.UpdatedAt,
		link.Description,
		string(metadataJSON),
		link.ID,
	)
	return err
}

func (s *PostgresStore) DeleteInvitationLink(ctx context.Context, id string) error {
	query := `DELETE FROM invitation_links WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

func (s *PostgresStore) ListInvitationLinks(ctx context.Context, filter InvitationLinkFilter) ([]*InvitationLink, error) {
	query := `
		SELECT id, token_hash, team_id, organization_id,
		       role, max_uses, current_uses, max_budget,
		       expires_at, is_active,
		       created_by, created_at, updated_at,
		       description, metadata
		FROM invitation_links
		WHERE 1=1`

	args := []any{}
	argIdx := 1

	if filter.TeamID != nil {
		query += fmt.Sprintf(" AND team_id = $%d", argIdx)
		args = append(args, *filter.TeamID)
		argIdx++
	}
	if filter.OrganizationID != nil {
		query += fmt.Sprintf(" AND organization_id = $%d", argIdx)
		args = append(args, *filter.OrganizationID)
		argIdx++
	}
	if filter.CreatedBy != nil {
		query += fmt.Sprintf(" AND created_by = $%d", argIdx)
		args = append(args, *filter.CreatedBy)
		argIdx++
	}
	if filter.IsActive != nil {
		query += fmt.Sprintf(" AND is_active = $%d", argIdx)
		args = append(args, *filter.IsActive)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list invitation links: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*InvitationLink
	for rows.Next() {
		var link InvitationLink
		var teamID, orgID sql.NullString
		var maxBudget sql.NullFloat64
		var expiresAt sql.NullTime
		var description, metadataJSON sql.NullString

		if err := rows.Scan(
			&link.ID,
			&link.Token,
			&teamID,
			&orgID,
			&link.Role,
			&link.MaxUses,
			&link.CurrentUses,
			&maxBudget,
			&expiresAt,
			&link.IsActive,
			&link.CreatedBy,
			&link.CreatedAt,
			&link.UpdatedAt,
			&description,
			&metadataJSON,
		); err != nil {
			return nil, fmt.Errorf("scan invitation link: %w", err)
		}

		if teamID.Valid {
			link.TeamID = &teamID.String
		}
		if orgID.Valid {
			link.OrganizationID = &orgID.String
		}
		if maxBudget.Valid {
			link.MaxBudget = &maxBudget.Float64
		}
		if expiresAt.Valid {
			t := expiresAt.Time
			link.ExpiresAt = &t
		}
		if description.Valid {
			link.Description = description.String
		}
		if metadataJSON.Valid && metadataJSON.String != "" {
			_ = json.Unmarshal([]byte(metadataJSON.String), &link.Metadata)
		}

		results = append(results, &link)
	}

	return results, rows.Err()
}

func (s *PostgresStore) IncrementInvitationLinkUses(ctx context.Context, id string) error {
	query := `UPDATE invitation_links SET current_uses = current_uses + 1, updated_at = NOW() WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

var _ InvitationLinkStore = (*PostgresStore)(nil)
