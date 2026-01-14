package auth

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestPostgresInvitationLinkStore_CreateGetUpdateDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	store := &PostgresStore{db: db}

	now := time.Date(2026, 1, 14, 12, 0, 0, 0, time.UTC)
	teamID := "team-uuid"
	link := &InvitationLink{
		ID:        "inv-uuid",
		Token:     "token-hash",
		TeamID:    &teamID,
		Role:      "member",
		MaxUses:   5,
		IsActive:  true,
		CreatedBy: "creator",
		CreatedAt: now,
		UpdatedAt: now,
	}

	mock.ExpectExec(`INSERT INTO invitation_links`).
		WithArgs(
			link.ID,
			link.Token,
			sqlmock.AnyArg(), // team_id
			sqlmock.AnyArg(), // organization_id
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
			sqlmock.AnyArg(), // metadata
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, store.CreateInvitationLink(context.Background(), link))

	rows := sqlmock.NewRows([]string{
		"id",
		"token_hash",
		"team_id",
		"organization_id",
		"role",
		"max_uses",
		"current_uses",
		"max_budget",
		"expires_at",
		"is_active",
		"created_by",
		"created_at",
		"updated_at",
		"description",
		"metadata",
	}).AddRow(
		link.ID,
		link.Token,
		teamID,
		nil,
		link.Role,
		link.MaxUses,
		link.CurrentUses,
		nil,
		nil,
		link.IsActive,
		link.CreatedBy,
		link.CreatedAt,
		link.UpdatedAt,
		link.Description,
		`{}`,
	)
	mock.ExpectQuery(`SELECT .* FROM invitation_links WHERE id = \$1`).
		WithArgs(link.ID).
		WillReturnRows(rows)

	got, err := store.GetInvitationLink(context.Background(), link.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, link.ID, got.ID)
	require.Equal(t, link.Token, got.Token)
	require.NotNil(t, got.TeamID)
	require.Equal(t, teamID, *got.TeamID)
	require.True(t, got.IsActive)

	link.IsActive = false
	link.UpdatedAt = now.Add(time.Minute)
	mock.ExpectExec(`UPDATE invitation_links SET`).
		WithArgs(
			link.Token,
			sqlmock.AnyArg(), // team_id
			sqlmock.AnyArg(), // organization_id
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
			sqlmock.AnyArg(), // metadata
			link.ID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, store.UpdateInvitationLink(context.Background(), link))

	mock.ExpectExec(`DELETE FROM invitation_links WHERE id = \$1`).
		WithArgs(link.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, store.DeleteInvitationLink(context.Background(), link.ID))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresInvitationLinkStore_GetByTokenListAndIncrement(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	store := &PostgresStore{db: db}

	now := time.Date(2026, 1, 14, 12, 0, 0, 0, time.UTC)
	orgID := "org-uuid"
	linkID := "inv-uuid"
	tokenHash := "token-hash"

	rows := sqlmock.NewRows([]string{
		"id", "token_hash", "team_id", "organization_id", "role",
		"max_uses", "current_uses", "max_budget", "expires_at",
		"is_active", "created_by", "created_at", "updated_at", "description", "metadata",
	}).AddRow(
		linkID,
		tokenHash,
		nil,
		orgID,
		"admin",
		0,
		0,
		nil,
		nil,
		true,
		"creator",
		now,
		now,
		"desc",
		`{}`,
	)
	mock.ExpectQuery(`SELECT .* FROM invitation_links WHERE token_hash = \$1`).
		WithArgs(tokenHash).
		WillReturnRows(rows)

	got, err := store.GetInvitationLinkByToken(context.Background(), tokenHash)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.OrganizationID)
	require.Equal(t, orgID, *got.OrganizationID)

	filter := InvitationLinkFilter{
		OrganizationID: &orgID,
		IsActive:       ptr(true),
		Limit:          10,
		Offset:         5,
	}

	listRows := sqlmock.NewRows([]string{
		"id", "token_hash", "team_id", "organization_id", "role",
		"max_uses", "current_uses", "max_budget", "expires_at",
		"is_active", "created_by", "created_at", "updated_at", "description", "metadata",
	}).AddRow(
		linkID,
		tokenHash,
		nil,
		orgID,
		"admin",
		0,
		0,
		nil,
		nil,
		true,
		"creator",
		now,
		now,
		"desc",
		`{}`,
	)

	mock.ExpectQuery(`SELECT .* FROM invitation_links WHERE .* ORDER BY created_at DESC LIMIT \$[0-9]+ OFFSET \$[0-9]+`).
		WithArgs(orgID, true, filter.Limit, filter.Offset).
		WillReturnRows(listRows)

	list, err := store.ListInvitationLinks(context.Background(), filter)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, linkID, list[0].ID)

	mock.ExpectExec(`UPDATE invitation_links SET current_uses = current_uses \+ 1, updated_at = NOW\(\) WHERE id = \$1`).
		WithArgs(linkID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, store.IncrementInvitationLinkUses(context.Background(), linkID))
	require.NoError(t, mock.ExpectationsWereMet())
}

func ptr[T any](v T) *T { return &v }
