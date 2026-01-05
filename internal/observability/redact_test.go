package observability

import (
	"strings"
	"testing"
)

func TestRedactor_OpenAIKey(t *testing.T) {
	r := NewRedactor()

	tests := []struct {
		input    string
		contains string
	}{
		{"sk-1234567890abcdefghijklmnop", "[REDACTED_OPENAI_KEY]"},
		{"key: sk-proj-abcdefghijklmnopqrstuvwxyz123456", "[REDACTED_OPENAI_PROJECT_KEY]"},
	}

	for _, tt := range tests {
		result := r.Redact(tt.input)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("expected result to contain %q, got %q", tt.contains, result)
		}
	}
}

func TestRedactor_AnthropicKey(t *testing.T) {
	r := NewRedactor()

	input := "key: sk-ant-api03-abcdefghijklmnopqrstuvwxyz"
	result := r.Redact(input)

	if !strings.Contains(result, "[REDACTED_ANTHROPIC_KEY]") {
		t.Errorf("expected anthropic key to be redacted, got %q", result)
	}
}

func TestRedactor_BearerToken(t *testing.T) {
	r := NewRedactor()

	input := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0"
	result := r.Redact(input)

	if !strings.Contains(result, "Bearer [REDACTED]") {
		t.Errorf("expected bearer token to be redacted, got %q", result)
	}
}

func TestRedactor_Email(t *testing.T) {
	r := NewRedactor()

	input := "user email is test@example.com"
	result := r.Redact(input)

	if !strings.Contains(result, "[REDACTED_EMAIL]") {
		t.Errorf("expected email to be redacted, got %q", result)
	}
}

func TestRedactor_Phone(t *testing.T) {
	r := NewRedactor()

	input := "+1-555-123-4567"
	result := r.Redact(input)
	if !strings.Contains(result, "[REDACTED_PHONE]") {
		t.Errorf("expected phone to be redacted, got %q", result)
	}
}

func TestRedactor_CreditCard(t *testing.T) {
	r := NewRedactor()

	tests := []string{
		"4111-1111-1111-1111",
		"4111 1111 1111 1111",
	}

	for _, input := range tests {
		result := r.Redact(input)
		if !strings.Contains(result, "[REDACTED_CARD]") {
			t.Errorf("expected card %q to be redacted, got %q", input, result)
		}
	}
}

func TestRedactor_SSN(t *testing.T) {
	r := NewRedactor()

	input := "SSN: 123-45-6789"
	result := r.Redact(input)

	if !strings.Contains(result, "[REDACTED_SSN]") {
		t.Errorf("expected SSN to be redacted, got %q", result)
	}
}

func TestRedactor_RedactMap(t *testing.T) {
	r := NewRedactor()

	input := map[string]any{
		"api_key":  "sk-1234567890abcdefghijklmnop",
		"username": "testuser",
		"password": "secret123",
		"data": map[string]any{
			"token": "abc123",
		},
	}

	result := r.RedactMap(input)

	if result["api_key"] != "[REDACTED]" {
		t.Errorf("expected api_key to be redacted, got %v", result["api_key"])
	}
	if result["password"] != "[REDACTED]" {
		t.Errorf("expected password to be redacted, got %v", result["password"])
	}
	if result["username"] != "testuser" {
		t.Errorf("expected username to be unchanged, got %v", result["username"])
	}

	nested := result["data"].(map[string]any)
	if nested["token"] != "[REDACTED]" {
		t.Errorf("expected nested token to be redacted, got %v", nested["token"])
	}
}

func TestRedactor_RedactHeaders(t *testing.T) {
	r := NewRedactor()

	headers := map[string][]string{
		"Authorization": {"Bearer token123"},
		"X-Api-Key":     {"sk-secret"},
		"Content-Type":  {"application/json"},
		"Cookie":        {"session=abc123"},
	}

	result := r.RedactHeaders(headers)

	if result["Authorization"][0] != "[REDACTED]" {
		t.Errorf("expected Authorization to be redacted")
	}
	if result["X-Api-Key"][0] != "[REDACTED]" {
		t.Errorf("expected X-Api-Key to be redacted")
	}
	if result["Content-Type"][0] != "application/json" {
		t.Errorf("expected Content-Type to be unchanged")
	}
	if result["Cookie"][0] != "[REDACTED]" {
		t.Errorf("expected Cookie to be redacted")
	}
}

func TestRedactor_AddPattern(t *testing.T) {
	r := NewRedactor()

	// Add custom pattern
	r.AddPattern(`SECRET_[A-Z0-9]+`, "[CUSTOM_REDACTED]", "custom")

	input := "my secret is SECRET_ABC123"
	result := r.Redact(input)

	if !strings.Contains(result, "[CUSTOM_REDACTED]") {
		t.Errorf("expected custom pattern to be redacted, got %q", result)
	}
}

func TestRedactor_InvalidPattern(t *testing.T) {
	r := NewRedactor()

	// Invalid regex should not panic
	r.AddPattern(`[invalid`, "replacement", "invalid")

	// Should still work
	result := r.Redact("test")
	if result != "test" {
		t.Errorf("expected unchanged result, got %q", result)
	}
}

func TestRedactor_RedactArray(t *testing.T) {
	r := NewRedactor()

	input := map[string]any{
		"items": []any{
			"normal text",
			"email: test@example.com",
			map[string]any{"api_key": "secret"},
		},
	}

	result := r.RedactMap(input)
	items := result["items"].([]any)

	if items[0] != "normal text" {
		t.Errorf("expected first item unchanged")
	}
	if !strings.Contains(items[1].(string), "[REDACTED_EMAIL]") {
		t.Errorf("expected email in array to be redacted")
	}
	nested := items[2].(map[string]any)
	if nested["api_key"] != "[REDACTED]" {
		t.Errorf("expected nested api_key to be redacted")
	}
}
