package api

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/internal/auth"
)

type noopLogger struct{}

func (noopLogger) Error(string, ...any) {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Info(string, ...any)  {}

func TestInvitationCreate_DoesNotTrustXUserIDHeader(t *testing.T) {
	authStore := auth.NewMemoryStore()
	inviteStore := auth.NewMemoryInvitationLinkStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	service := auth.NewInvitationService(inviteStore, authStore, logger)

	handler := NewInvitationHandler(service, inviteStore, noopLogger{})

	teamID := "team-1"
	body, err := json.Marshal(CreateInvitationRequest{
		TeamID: &teamID,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/invitation/new", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "evil-user")

	ctx := context.WithValue(req.Context(), auth.AuthContextKey, &auth.AuthContext{
		User: &auth.User{ID: "real-user"},
	})
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.CreateInvitation(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	links, err := inviteStore.ListInvitationLinks(context.Background(), auth.InvitationLinkFilter{})
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.Equal(t, "real-user", links[0].CreatedBy)
}

func TestInvitationCreate_DefaultsToSystemWhenNoAuthContext(t *testing.T) {
	authStore := auth.NewMemoryStore()
	inviteStore := auth.NewMemoryInvitationLinkStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	service := auth.NewInvitationService(inviteStore, authStore, logger)

	handler := NewInvitationHandler(service, inviteStore, noopLogger{})

	teamID := "team-1"
	body, err := json.Marshal(CreateInvitationRequest{
		TeamID: &teamID,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/invitation/new", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "evil-user")

	rr := httptest.NewRecorder()
	handler.CreateInvitation(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	links, err := inviteStore.ListInvitationLinks(context.Background(), auth.InvitationLinkFilter{})
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.Equal(t, "system", links[0].CreatedBy)
}
