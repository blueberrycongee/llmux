package auth

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	fullKey, hash, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}

	// Check key format
	if !strings.HasPrefix(fullKey, DefaultKeyPrefix) {
		t.Errorf("key should start with %s, got %s", DefaultKeyPrefix, fullKey[:10])
	}

	// Check hash is not empty
	if hash == "" {
		t.Error("hash should not be empty")
	}

	// Check hash length (SHA-256 = 64 hex chars)
	if len(hash) != 64 {
		t.Errorf("hash length should be 64, got %d", len(hash))
	}

	// Verify the key matches the hash
	if !VerifyKey(fullKey, hash) {
		t.Error("VerifyKey should return true for matching key and hash")
	}
}

func TestGenerateAPIKey_Uniqueness(t *testing.T) {
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key, _, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey() error = %v", err)
		}
		if keys[key] {
			t.Errorf("duplicate key generated: %s", key)
		}
		keys[key] = true
	}
}

func TestHashKey(t *testing.T) {
	key := "llmux_test_key_123"
	hash1 := HashKey(key)
	hash2 := HashKey(key)

	// Same key should produce same hash
	if hash1 != hash2 {
		t.Error("HashKey should be deterministic")
	}

	// Different keys should produce different hashes
	hash3 := HashKey("llmux_different_key")
	if hash1 == hash3 {
		t.Error("different keys should produce different hashes")
	}
}

func TestVerifyKey(t *testing.T) {
	key := "llmux_test_key_456"
	hash := HashKey(key)

	tests := []struct {
		name     string
		key      string
		hash     string
		expected bool
	}{
		{"matching key and hash", key, hash, true},
		{"wrong key", "wrong_key", hash, false},
		{"wrong hash", key, "wrong_hash", false},
		{"empty key", "", hash, false},
		{"empty hash", key, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyKey(tt.key, tt.hash)
			if result != tt.expected {
				t.Errorf("VerifyKey(%q, %q) = %v, want %v", tt.key, tt.hash, result, tt.expected)
			}
		})
	}
}

func TestExtractKeyPrefix(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"llmux_abcdefghijklmnop", "llmux_ab"},
		{"short", "short"},
		{"12345678", "12345678"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := ExtractKeyPrefix(tt.key)
			if result != tt.expected {
				t.Errorf("ExtractKeyPrefix(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestParseAuthHeader(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		expected  string
		expectErr bool
	}{
		{"bearer format", "Bearer llmux_key123", "llmux_key123", false},
		{"bearer with extra spaces", "Bearer   llmux_key123  ", "llmux_key123", false},
		{"plain key", "llmux_key123", "llmux_key123", false},
		{"empty header", "", "", true},
		{"bearer only", "Bearer ", "", true},
		{"bearer no space", "Bearerkey", "Bearerkey", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseAuthHeader(tt.header)
			if tt.expectErr {
				if err == nil {
					t.Errorf("ParseAuthHeader(%q) expected error, got nil", tt.header)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseAuthHeader(%q) unexpected error: %v", tt.header, err)
				return
			}
			if result != tt.expected {
				t.Errorf("ParseAuthHeader(%q) = %q, want %q", tt.header, result, tt.expected)
			}
		})
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"llmux_abcdefghijklmnop", "llmux_ab...mnop"},
		{"short", "***"},
		{"123456789012", "***"},
		{"1234567890123", "12345678...0123"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			result := MaskKey(tt.key)
			if result != tt.expected {
				t.Errorf("MaskKey(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func BenchmarkHashKey(b *testing.B) {
	key := "llmux_benchmark_key_12345"
	for i := 0; i < b.N; i++ {
		HashKey(key)
	}
}

func BenchmarkVerifyKey(b *testing.B) {
	key := "llmux_benchmark_key_12345"
	hash := HashKey(key)
	for i := 0; i < b.N; i++ {
		VerifyKey(key, hash)
	}
}
