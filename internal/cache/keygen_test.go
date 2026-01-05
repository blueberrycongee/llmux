package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultKeyGenerator_Generate(t *testing.T) {
	gen := NewKeyGenerator("llmux")

	t.Run("basic key generation", func(t *testing.T) {
		params := KeyParams{
			Model:    "gpt-4",
			Messages: []byte(`[{"role":"user","content":"hello"}]`),
		}

		key := gen.Generate(params)
		assert.NotEmpty(t, key)
		assert.Contains(t, key, "llmux:")
		// SHA-256 produces 64 hex characters
		assert.Len(t, key, len("llmux:")+64)
	})

	t.Run("same params produce same key", func(t *testing.T) {
		params := KeyParams{
			Model:    "gpt-4",
			Messages: []byte(`[{"role":"user","content":"hello"}]`),
		}

		key1 := gen.Generate(params)
		key2 := gen.Generate(params)
		assert.Equal(t, key1, key2)
	})

	t.Run("different params produce different keys", func(t *testing.T) {
		params1 := KeyParams{
			Model:    "gpt-4",
			Messages: []byte(`[{"role":"user","content":"hello"}]`),
		}
		params2 := KeyParams{
			Model:    "gpt-4",
			Messages: []byte(`[{"role":"user","content":"world"}]`),
		}

		key1 := gen.Generate(params1)
		key2 := gen.Generate(params2)
		assert.NotEqual(t, key1, key2)
	})

	t.Run("temperature affects key", func(t *testing.T) {
		temp1 := 0.7
		temp2 := 0.9

		params1 := KeyParams{
			Model:       "gpt-4",
			Messages:    []byte(`[{"role":"user","content":"hello"}]`),
			Temperature: &temp1,
		}
		params2 := KeyParams{
			Model:       "gpt-4",
			Messages:    []byte(`[{"role":"user","content":"hello"}]`),
			Temperature: &temp2,
		}

		key1 := gen.Generate(params1)
		key2 := gen.Generate(params2)
		assert.NotEqual(t, key1, key2)
	})

	t.Run("namespace in key", func(t *testing.T) {
		params := KeyParams{
			Model:     "gpt-4",
			Messages:  []byte(`[{"role":"user","content":"hello"}]`),
			Namespace: "tenant-123",
		}

		key := gen.Generate(params)
		assert.Contains(t, key, "llmux:tenant-123:")
	})

	t.Run("no prefix", func(t *testing.T) {
		genNoPrefix := NewKeyGenerator("")
		params := KeyParams{
			Model:    "gpt-4",
			Messages: []byte(`[{"role":"user","content":"hello"}]`),
		}

		key := genNoPrefix.Generate(params)
		assert.NotContains(t, key, ":")
		assert.Len(t, key, 64) // Just the hash
	})

	t.Run("with tools", func(t *testing.T) {
		params1 := KeyParams{
			Model:    "gpt-4",
			Messages: []byte(`[{"role":"user","content":"hello"}]`),
		}
		params2 := KeyParams{
			Model:    "gpt-4",
			Messages: []byte(`[{"role":"user","content":"hello"}]`),
			Tools:    []byte(`[{"type":"function","function":{"name":"test"}}]`),
		}

		key1 := gen.Generate(params1)
		key2 := gen.Generate(params2)
		assert.NotEqual(t, key1, key2)
	})

	t.Run("with max_tokens", func(t *testing.T) {
		params1 := KeyParams{
			Model:     "gpt-4",
			Messages:  []byte(`[{"role":"user","content":"hello"}]`),
			MaxTokens: 100,
		}
		params2 := KeyParams{
			Model:     "gpt-4",
			Messages:  []byte(`[{"role":"user","content":"hello"}]`),
			MaxTokens: 200,
		}

		key1 := gen.Generate(params1)
		key2 := gen.Generate(params2)
		assert.NotEqual(t, key1, key2)
	})
}

func TestDefaultKeyGenerator_GenerateFromRaw(t *testing.T) {
	gen := NewKeyGenerator("llmux")

	t.Run("basic raw key", func(t *testing.T) {
		key := gen.GenerateFromRaw("", "test content")
		assert.NotEmpty(t, key)
		assert.Contains(t, key, "llmux:")
	})

	t.Run("with namespace", func(t *testing.T) {
		key := gen.GenerateFromRaw("tenant-123", "test content")
		assert.Contains(t, key, "llmux:tenant-123:")
	})

	t.Run("same content same key", func(t *testing.T) {
		key1 := gen.GenerateFromRaw("ns", "content")
		key2 := gen.GenerateFromRaw("ns", "content")
		assert.Equal(t, key1, key2)
	})
}

func BenchmarkKeyGenerator_Generate(b *testing.B) {
	gen := NewKeyGenerator("llmux")
	params := KeyParams{
		Model:    "gpt-4",
		Messages: []byte(`[{"role":"user","content":"hello world, this is a test message"}]`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen.Generate(params)
	}
}
