package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// KeyPrefixLength is the number of characters to show as key prefix.
	KeyPrefixLength = 8
	// KeyLength is the total length of generated API keys (before prefix).
	KeyLength = 32
	// DefaultKeyPrefix is the prefix for generated keys.
	DefaultKeyPrefix = "llmux_"
)

// GenerateAPIKey creates a new random API key with the format: llmux_<random>
// Returns the full key (to show user once) and the hash (to store).
func GenerateAPIKey() (fullKey, hash string, err error) {
	// Generate random bytes
	randomBytes := make([]byte, KeyLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}

	// Encode to base64 URL-safe format
	randomPart := base64.RawURLEncoding.EncodeToString(randomBytes)
	fullKey = DefaultKeyPrefix + randomPart

	// Hash the key for storage
	hash = HashKey(fullKey)

	return fullKey, hash, nil
}

// HashKey creates a SHA-256 hash of the API key.
func HashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// VerifyKey checks if a key matches a hash using constant-time comparison.
func VerifyKey(key, hash string) bool {
	keyHash := HashKey(key)
	return subtle.ConstantTimeCompare([]byte(keyHash), []byte(hash)) == 1
}

// ExtractKeyPrefix returns the first N characters of a key for identification.
func ExtractKeyPrefix(key string) string {
	if len(key) <= KeyPrefixLength {
		return key
	}
	return key[:KeyPrefixLength]
}

// ParseAuthHeader extracts the API key from an Authorization header.
// Supports formats: "Bearer <key>" or just "<key>"
func ParseAuthHeader(header string) (string, error) {
	if header == "" {
		return "", fmt.Errorf("authorization header is empty")
	}

	// Handle "Bearer <key>" format
	if strings.HasPrefix(header, "Bearer ") {
		key := strings.TrimPrefix(header, "Bearer ")
		key = strings.TrimSpace(key)
		if key == "" {
			return "", fmt.Errorf("bearer token is empty")
		}
		return key, nil
	}

	// Handle plain key format
	return strings.TrimSpace(header), nil
}

// MaskKey returns a masked version of the key for logging.
// Example: "llmux_abc...xyz"
func MaskKey(key string) string {
	if len(key) <= 12 {
		return "***"
	}
	return key[:8] + "..." + key[len(key)-4:]
}
