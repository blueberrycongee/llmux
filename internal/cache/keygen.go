package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// DefaultKeyGenerator implements KeyGenerator using SHA-256 hashing.
type DefaultKeyGenerator struct {
	// Prefix is prepended to all generated keys.
	Prefix string
}

// NewKeyGenerator creates a new DefaultKeyGenerator with optional prefix.
func NewKeyGenerator(prefix string) *DefaultKeyGenerator {
	return &DefaultKeyGenerator{Prefix: prefix}
}

// Generate creates a SHA-256 hash key from the request parameters.
// The key format is: [prefix:]namespace:sha256(params)
func (g *DefaultKeyGenerator) Generate(params KeyParams) string {
	var sb strings.Builder

	// Model is always included
	sb.WriteString(fmt.Sprintf("model:%s", params.Model))

	// Messages content
	if len(params.Messages) > 0 {
		sb.WriteString(fmt.Sprintf("|messages:%s", string(params.Messages)))
	}

	// Temperature (only if set)
	if params.Temperature != nil {
		sb.WriteString(fmt.Sprintf("|temp:%.2f", *params.Temperature))
	}

	// MaxTokens (only if set)
	if params.MaxTokens > 0 {
		sb.WriteString(fmt.Sprintf("|max_tokens:%d", params.MaxTokens))
	}

	// TopP (only if set)
	if params.TopP != nil {
		sb.WriteString(fmt.Sprintf("|top_p:%.2f", *params.TopP))
	}

	// Tools (only if set)
	if len(params.Tools) > 0 {
		sb.WriteString(fmt.Sprintf("|tools:%s", string(params.Tools)))
	}

	// Extra provider-specific params
	for k, v := range params.Extra {
		sb.WriteString(fmt.Sprintf("|%s:%s", k, string(v)))
	}

	// Generate SHA-256 hash
	hash := sha256.Sum256([]byte(sb.String()))
	hashHex := hex.EncodeToString(hash[:])

	// Build final key with namespace
	var key strings.Builder
	if g.Prefix != "" {
		key.WriteString(g.Prefix)
		key.WriteString(":")
	}
	if params.Namespace != "" {
		key.WriteString(params.Namespace)
		key.WriteString(":")
	}
	key.WriteString(hashHex)

	return key.String()
}

// GenerateFromRaw creates a cache key from raw string content.
// Useful for simple caching scenarios.
func (g *DefaultKeyGenerator) GenerateFromRaw(namespace, content string) string {
	hash := sha256.Sum256([]byte(content))
	hashHex := hex.EncodeToString(hash[:])

	var key strings.Builder
	if g.Prefix != "" {
		key.WriteString(g.Prefix)
		key.WriteString(":")
	}
	if namespace != "" {
		key.WriteString(namespace)
		key.WriteString(":")
	}
	key.WriteString(hashHex)

	return key.String()
}
