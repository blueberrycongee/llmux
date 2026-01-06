// Package observability provides content filtering utilities for logging.
package observability

import (
	"encoding/base64"
	"regexp"
	"strings"
)

// ContentFilter provides content filtering for logging payloads.
type ContentFilter struct {
	// FilterBase64 removes base64 encoded content (images, etc.)
	FilterBase64 bool
	// Base64Placeholder is the replacement text for base64 content
	Base64Placeholder string
	// MaxContentLength truncates content longer than this
	MaxContentLength int
	// RedactPatterns are regex patterns to redact
	RedactPatterns []*regexp.Regexp
	// RedactPlaceholder is the replacement text for redacted content
	RedactPlaceholder string
}

// DefaultContentFilter returns a filter with sensible defaults.
func DefaultContentFilter() *ContentFilter {
	return &ContentFilter{
		FilterBase64:      true,
		Base64Placeholder: "[base64_content_filtered]",
		MaxContentLength:  10000,
		RedactPlaceholder: "[REDACTED]",
	}
}

// FilterPayload filters sensitive content from a StandardLoggingPayload.
func (f *ContentFilter) FilterPayload(payload *StandardLoggingPayload) *StandardLoggingPayload {
	if payload == nil {
		return nil
	}

	// Create a copy to avoid modifying the original
	filtered := *payload

	// Filter messages
	if filtered.Messages != nil {
		filtered.Messages = f.filterContent(filtered.Messages)
	}

	// Filter response
	if filtered.Response != nil {
		filtered.Response = f.filterContent(filtered.Response)
	}

	return &filtered
}

// filterContent recursively filters content.
func (f *ContentFilter) filterContent(content any) any {
	if content == nil {
		return nil
	}

	switch v := content.(type) {
	case string:
		return f.filterString(v)
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = f.filterContent(item)
		}
		return result
	case map[string]any:
		return f.filterMap(v)
	case []map[string]any:
		result := make([]map[string]any, len(v))
		for i, item := range v {
			result[i] = f.filterMap(item)
		}
		return result
	default:
		return content
	}
}

// filterString filters a string value.
func (f *ContentFilter) filterString(s string) string {
	// Filter base64 content
	if f.FilterBase64 && f.isBase64Content(s) {
		return f.Base64Placeholder
	}

	// Apply redaction patterns
	for _, pattern := range f.RedactPatterns {
		s = pattern.ReplaceAllString(s, f.RedactPlaceholder)
	}

	// Truncate if too long
	if f.MaxContentLength > 0 && len(s) > f.MaxContentLength {
		s = s[:f.MaxContentLength] + "...[truncated]"
	}

	return s
}

// filterMap filters a map value.
func (f *ContentFilter) filterMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))

	for k, v := range m {
		// Check for image content in OpenAI format
		if k == "type" && v == "image_url" {
			result[k] = v
			continue
		}

		// Filter image_url content
		if k == "image_url" {
			if imgMap, ok := v.(map[string]any); ok {
				if url, ok := imgMap["url"].(string); ok {
					if f.FilterBase64 && strings.HasPrefix(url, "data:") {
						imgMap["url"] = f.Base64Placeholder
					}
				}
				result[k] = imgMap
				continue
			}
		}

		// Recursively filter other content
		result[k] = f.filterContent(v)
	}

	return result
}

// isBase64Content checks if a string appears to be base64 encoded content.
func (f *ContentFilter) isBase64Content(s string) bool {
	// Check for data URI scheme
	if strings.HasPrefix(s, "data:") {
		return true
	}

	// Check if it looks like base64 (long string with base64 chars)
	if len(s) < 100 {
		return false
	}

	// Quick heuristic: check if string is mostly base64 characters
	base64Chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/="
	validCount := 0
	for _, c := range s {
		if strings.ContainsRune(base64Chars, c) {
			validCount++
		}
	}

	// If >90% base64 chars and length is divisible by 4, likely base64
	ratio := float64(validCount) / float64(len(s))
	if ratio > 0.9 && len(s)%4 == 0 {
		// Try to decode to confirm
		_, err := base64.StdEncoding.DecodeString(s)
		return err == nil
	}

	return false
}

// LabelFilter provides label filtering for metrics.
type LabelFilter struct {
	// IncludeLabels specifies which labels to include (empty = all)
	IncludeLabels []string
	// ExcludeLabels specifies which labels to exclude
	ExcludeLabels []string
}

// MetricsLabelConfig defines label configuration for a metric group.
type MetricsLabelConfig struct {
	Group         string   `yaml:"group" json:"group"`
	Metrics       []string `yaml:"metrics" json:"metrics"`
	IncludeLabels []string `yaml:"include_labels" json:"include_labels"`
	ExcludeLabels []string `yaml:"exclude_labels" json:"exclude_labels"`
}

// LabelFilterManager manages label filters for different metric groups.
type LabelFilterManager struct {
	configs map[string]*LabelFilter // metric name -> filter
	default_ *LabelFilter
}

// NewLabelFilterManager creates a new label filter manager.
func NewLabelFilterManager(configs []MetricsLabelConfig) *LabelFilterManager {
	mgr := &LabelFilterManager{
		configs: make(map[string]*LabelFilter),
		default_: &LabelFilter{}, // No filtering by default
	}

	for _, cfg := range configs {
		filter := &LabelFilter{
			IncludeLabels: cfg.IncludeLabels,
			ExcludeLabels: cfg.ExcludeLabels,
		}
		for _, metric := range cfg.Metrics {
			mgr.configs[metric] = filter
		}
	}

	return mgr
}

// GetFilter returns the label filter for a metric.
func (m *LabelFilterManager) GetFilter(metricName string) *LabelFilter {
	if filter, ok := m.configs[metricName]; ok {
		return filter
	}
	return m.default_
}

// FilterLabels filters labels based on the configuration.
func (f *LabelFilter) FilterLabels(labels map[string]string) map[string]string {
	if len(f.IncludeLabels) == 0 && len(f.ExcludeLabels) == 0 {
		return labels
	}

	result := make(map[string]string)

	// If include list is specified, only include those
	if len(f.IncludeLabels) > 0 {
		includeSet := make(map[string]bool)
		for _, l := range f.IncludeLabels {
			includeSet[l] = true
		}
		for k, v := range labels {
			if includeSet[k] {
				result[k] = v
			}
		}
	} else {
		// Otherwise, start with all labels
		for k, v := range labels {
			result[k] = v
		}
	}

	// Remove excluded labels
	for _, l := range f.ExcludeLabels {
		delete(result, l)
	}

	return result
}

// ShouldIncludeLabel checks if a label should be included.
func (f *LabelFilter) ShouldIncludeLabel(label string) bool {
	// Check exclude list first
	for _, l := range f.ExcludeLabels {
		if l == label {
			return false
		}
	}

	// If include list is empty, include all (except excluded)
	if len(f.IncludeLabels) == 0 {
		return true
	}

	// Check include list
	for _, l := range f.IncludeLabels {
		if l == label {
			return true
		}
	}

	return false
}
