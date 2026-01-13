package api

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/internal/auth"
)

func TestManagementRegenerateKey_UpdatesInPlace(t *testing.T) {
	store := auth.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	raw, hash, err := auth.GenerateAPIKey()
	require.NoError(t, err)

	now := time.Now()
	key := &auth.APIKey{
		ID:        "key-1",
		KeyHash:   hash,
		KeyPrefix: auth.ExtractKeyPrefix(raw),
		Name:      "k",
		IsActive:  true,
		Metadata: auth.Metadata{
			"rotation_count": float64(2),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, store.CreateAPIKey(context.Background(), key))

	handler := NewManagementHandler(store, nil, logger, nil, nil, nil)

	body, err := json.Marshal(map[string]string{"key": key.ID})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/key/regenerate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.RegenerateKey(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp GenerateKeyResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Equal(t, key.ID, resp.KeyID)
	require.NotEmpty(t, resp.Key)
	require.NotEqual(t, key.KeyPrefix, resp.KeyPrefix)

	updated, err := store.GetAPIKeyByID(context.Background(), key.ID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, key.ID, updated.ID)
	require.Equal(t, resp.KeyPrefix, updated.KeyPrefix)
	require.NotEqual(t, key.KeyHash, updated.KeyHash)
	switch v := updated.Metadata["rotation_count"].(type) {
	case int:
		require.Equal(t, 3, v)
	case int64:
		require.Equal(t, int64(3), v)
	case float64:
		require.Equal(t, float64(3), v)
	default:
		t.Fatalf("unexpected rotation_count type %T", updated.Metadata["rotation_count"])
	}
}
