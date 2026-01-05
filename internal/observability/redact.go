// Package observability provides logging utilities with sensitive data redaction.
package observability

import (
	"regexp"
	"strings"
)

// Redactor handles sensitive data masking in logs.
type Redactor struct {
	patterns []*redactPattern
}

type redactPattern struct {
	regex       *regexp.Regexp
	replacement string
	name        string
}

// NewRedactor creates a new redactor with default patterns.
func NewRedactor() *Redactor {
	r := &Redactor{}
	r.addDefaultPatterns()
	return r
}

func (r *Redactor) addDefaultPatterns() {
	// API Keys - various formats
	r.AddPattern(`sk-[a-zA-Z0-9]{20,}`, "[REDACTED_OPENAI_KEY]", "openai_key")
	r.AddPattern(`sk-proj-[a-zA-Z0-9\-_]{20,}`, "[REDACTED_OPENAI_PROJECT_KEY]", "openai_project_key")
	r.AddPattern(`sk-ant-[a-zA-Z0-9\-_]{20,}`, "[REDACTED_ANTHROPIC_KEY]", "anthropic_key")
	r.AddPattern(`AIza[a-zA-Z0-9\-_]{35}`, "[REDACTED_GOOGLE_KEY]", "google_key")
	r.AddPattern(`[a-f0-9]{32}`, "[REDACTED_API_KEY]", "generic_api_key")

	// Bearer tokens
	r.AddPattern(`Bearer\s+[a-zA-Z0-9\-_\.]+`, "Bearer [REDACTED]", "bearer_token")

	// Authorization headers
	r.AddPattern(`Authorization:\s*[^\s]+`, "Authorization: [REDACTED]", "auth_header")

	// Email addresses
	r.AddPattern(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`, "[REDACTED_EMAIL]", "email")

	// Phone numbers (various formats)
	r.AddPattern(`\+?[0-9]{1,3}[-.\s]?\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}`, "[REDACTED_PHONE]", "phone")

	// Credit card numbers (basic pattern)
	r.AddPattern(`\b[0-9]{4}[-\s]?[0-9]{4}[-\s]?[0-9]{4}[-\s]?[0-9]{4}\b`, "[REDACTED_CARD]", "credit_card")

	// SSN (US format)
	r.AddPattern(`\b[0-9]{3}-[0-9]{2}-[0-9]{4}\b`, "[REDACTED_SSN]", "ssn")

	// IP addresses (optional - may want to keep for debugging)
	// r.AddPattern(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`, "[REDACTED_IP]", "ip_address")
}

// AddPattern adds a custom redaction pattern.
func (r *Redactor) AddPattern(pattern, replacement, name string) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return // Skip invalid patterns
	}
	r.patterns = append(r.patterns, &redactPattern{
		regex:       regex,
		replacement: replacement,
		name:        name,
	})
}

// Redact applies all redaction patterns to the input string.
func (r *Redactor) Redact(input string) string {
	result := input
	for _, p := range r.patterns {
		result = p.regex.ReplaceAllString(result, p.replacement)
	}
	return result
}

// RedactMap redacts sensitive values in a map.
func (r *Redactor) RedactMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = r.redactValue(k, v)
	}
	return result
}

func (r *Redactor) redactValue(key string, value any) any {
	// Check if key itself suggests sensitive data
	lowerKey := strings.ToLower(key)
	sensitiveKeys := []string{"key", "token", "secret", "password", "auth", "credential", "api_key", "apikey"}
	for _, sk := range sensitiveKeys {
		if strings.Contains(lowerKey, sk) {
			return "[REDACTED]"
		}
	}

	switch v := value.(type) {
	case string:
		return r.Redact(v)
	case map[string]any:
		return r.RedactMap(v)
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = r.redactValue("", item)
		}
		return result
	default:
		return value
	}
}

// RedactHeaders redacts sensitive HTTP headers.
func (r *Redactor) RedactHeaders(headers map[string][]string) map[string][]string {
	sensitiveHeaders := map[string]bool{
		"authorization":   true,
		"x-api-key":       true,
		"api-key":         true,
		"x-auth-token":    true,
		"cookie":          true,
		"set-cookie":      true,
		"x-openai-api-key": true,
	}

	result := make(map[string][]string, len(headers))
	for k, v := range headers {
		if sensitiveHeaders[strings.ToLower(k)] {
			result[k] = []string{"[REDACTED]"}
		} else {
			result[k] = v
		}
	}
	return result
}
