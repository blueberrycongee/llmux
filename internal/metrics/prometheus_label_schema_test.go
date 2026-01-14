package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func describeLabels(t *testing.T, c prometheus.Collector) []string {
	t.Helper()

	descCh := make(chan *prometheus.Desc, 8)
	c.Describe(descCh)
	close(descCh)

	var desc *prometheus.Desc
	for d := range descCh {
		desc = d
		break
	}
	if desc == nil {
		t.Fatalf("no descriptor returned")
	}

	s := desc.String()
	start := strings.Index(s, "variableLabels: {")
	if start < 0 {
		return nil
	}
	start += len("variableLabels: {")
	end := strings.Index(s[start:], "}")
	if end < 0 {
		t.Fatalf("failed to parse descriptor: %s", s)
	}
	raw := strings.TrimSpace(s[start : start+end])
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func assertLabelsEqual(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("labels mismatch\ngot:  %v\nwant: %v", got, want)
	}
}

func TestPrometheusLabelSchema_LowCardinality(t *testing.T) {
	assertLabelsEqual(t, describeLabels(t, ProxyTotalRequests), []string{
		"model", "model_group", "api_provider", "status_code",
	})

	assertLabelsEqual(t, describeLabels(t, ProxyFailedRequests), []string{
		"model", "model_group", "api_provider", "exception_status", "exception_class",
	})

	assertLabelsEqual(t, describeLabels(t, RequestTotalLatency), []string{
		"model", "model_group", "api_provider",
	})

	assertLabelsEqual(t, describeLabels(t, TotalTokens), []string{
		"model", "model_group", "api_provider",
	})

	assertLabelsEqual(t, describeLabels(t, TotalSpend), []string{
		"model", "model_group", "api_provider",
	})
}
