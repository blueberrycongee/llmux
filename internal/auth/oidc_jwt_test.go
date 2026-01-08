package auth

import "testing"

func TestIsLikelyJWT(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		// API Key formats - should return false
		{
			name:     "sk- prefix API Key",
			token:    "sk-1234567890abcdef",
			expected: false,
		},
		{
			name:     "pk- prefix API Key",
			token:    "pk-1234567890abcdef",
			expected: false,
		},
		{
			name:     "api- prefix API Key",
			token:    "api-1234567890abcdef",
			expected: false,
		},
		{
			name:     "key- prefix API Key",
			token:    "key-1234567890abcdef",
			expected: false,
		},
		{
			name:     "ak- prefix API Key",
			token:    "ak-1234567890abcdef",
			expected: false,
		},
		{
			name:     "test- prefix API Key",
			token:    "test-1234567890abcdef",
			expected: false,
		},

		// JWT formats - should return true
		{
			name:     "valid JWT format",
			token:    "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0.signature",
			expected: true,
		},
		{
			name:     "minimal JWT format with 3 segments",
			token:    "header.payload.signature",
			expected: true,
		},

		// Invalid formats - should return false
		{
			name:     "empty string",
			token:    "",
			expected: false,
		},
		{
			name:     "single segment",
			token:    "justsometoken",
			expected: false,
		},
		{
			name:     "two segments",
			token:    "header.payload",
			expected: false,
		},
		{
			name:     "four segments",
			token:    "a.b.c.d",
			expected: false,
		},
		{
			name:     "empty segment in middle",
			token:    "header..signature",
			expected: false,
		},
		{
			name:     "empty first segment",
			token:    ".payload.signature",
			expected: false,
		},
		{
			name:     "empty last segment",
			token:    "header.payload.",
			expected: false,
		},

		// Edge cases
		{
			name:     "sk- with dots (should be API Key)",
			token:    "sk-abc.def.ghi",
			expected: false, // API Key prefix takes precedence
		},
		{
			name:     "random 3-segment token",
			token:    "abc.def.ghi",
			expected: true, // Looks like JWT format
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLikelyJWT(tt.token)
			if result != tt.expected {
				t.Errorf("isLikelyJWT(%q) = %v, want %v", tt.token, result, tt.expected)
			}
		})
	}
}
