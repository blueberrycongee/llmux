package llmux

import (
	"context"
	"strings"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/internal/auth"
)

func TestGenerateCacheKey_IsolatedByTenantAPIKeyID(t *testing.T) {
	c := &Client{}

	req := &ChatRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
		},
	}

	ctxA := context.WithValue(context.Background(), auth.AuthContextKey, &auth.AuthContext{
		APIKey: &auth.APIKey{ID: "key-a"},
	})
	ctxB := context.WithValue(context.Background(), auth.AuthContextKey, &auth.AuthContext{
		APIKey: &auth.APIKey{ID: "key-b"},
	})

	keyA, err := c.generateCacheKey(ctxA, req)
	require.NoError(t, err)
	keyB, err := c.generateCacheKey(ctxB, req)
	require.NoError(t, err)

	require.NotEqual(t, keyA, keyB)
}

func TestGenerateCacheKey_ChangesWhenTopPChanges(t *testing.T) {
	c := &Client{}
	ctx := context.WithValue(context.Background(), auth.AuthContextKey, &auth.AuthContext{
		APIKey: &auth.APIKey{ID: "key-a"},
	})

	topP1 := 0.1
	topP2 := 0.9

	req1 := &ChatRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
		},
		TopP: &topP1,
	}
	req2 := &ChatRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
		},
		TopP: &topP2,
	}

	key1, err := c.generateCacheKey(ctx, req1)
	require.NoError(t, err)
	key2, err := c.generateCacheKey(ctx, req2)
	require.NoError(t, err)

	require.NotEqual(t, key1, key2)
}

func TestGenerateCacheKey_UsesSHA256Length(t *testing.T) {
	c := &Client{}
	ctx := context.WithValue(context.Background(), auth.AuthContextKey, &auth.AuthContext{
		APIKey: &auth.APIKey{ID: "key-a"},
	})

	req := &ChatRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
		},
	}

	key, err := c.generateCacheKey(ctx, req)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(key, "chat:"), "expected chat: prefix, got %q", key)
	require.Len(t, strings.TrimPrefix(key, "chat:"), 64, "expected sha256 hex length")
}

func TestGenerateCacheKey_DoesNotExposeTenantIDInPlaintext(t *testing.T) {
	c := &Client{}
	ctx := context.WithValue(context.Background(), auth.AuthContextKey, &auth.AuthContext{
		APIKey: &auth.APIKey{ID: "key-a"},
	})

	req := &ChatRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
		},
	}

	key, err := c.generateCacheKey(ctx, req)
	require.NoError(t, err)
	require.NotContains(t, key, "key-a")
}

func TestGenerateCacheKey_StableAcrossExtraMapOrder(t *testing.T) {
	c := &Client{}
	ctx := context.WithValue(context.Background(), auth.AuthContextKey, &auth.AuthContext{
		APIKey: &auth.APIKey{ID: "key-a"},
	})

	req1 := &ChatRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
		},
		Extra: map[string]json.RawMessage{
			"foo": json.RawMessage(`"bar"`),
			"baz": json.RawMessage(`123`),
		},
	}
	req2 := &ChatRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
		},
		Extra: map[string]json.RawMessage{
			"baz": json.RawMessage(`123`),
			"foo": json.RawMessage(`"bar"`),
		},
	}

	key1, err := c.generateCacheKey(ctx, req1)
	require.NoError(t, err)
	key2, err := c.generateCacheKey(ctx, req2)
	require.NoError(t, err)

	require.Equal(t, key1, key2)
}

func TestGenerateCacheKey_ChangesWhenExtraChanges(t *testing.T) {
	c := &Client{}
	ctx := context.WithValue(context.Background(), auth.AuthContextKey, &auth.AuthContext{
		APIKey: &auth.APIKey{ID: "key-a"},
	})

	req1 := &ChatRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
		},
		Extra: map[string]json.RawMessage{
			"seed": json.RawMessage(`1`),
		},
	}
	req2 := &ChatRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
		},
		Extra: map[string]json.RawMessage{
			"seed": json.RawMessage(`2`),
		},
	}

	key1, err := c.generateCacheKey(ctx, req1)
	require.NoError(t, err)
	key2, err := c.generateCacheKey(ctx, req2)
	require.NoError(t, err)

	require.NotEqual(t, key1, key2)
}
