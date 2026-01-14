package provider

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateBaseURL validates a provider base URL.
//
// This is intentionally conservative: it rejects userinfo/query/fragment and, unless
// explicitly allowed, loopback/private/link-local hosts (common SSRF targets).
func ValidateBaseURL(raw string, allowPrivate bool) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid base_url: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid base_url scheme %q (must be http or https)", u.Scheme)
	}

	if u.Hostname() == "" {
		return fmt.Errorf("invalid base_url host %q", u.Host)
	}

	if u.User != nil {
		return fmt.Errorf("base_url must not contain userinfo")
	}

	if u.RawQuery != "" {
		return fmt.Errorf("base_url must not contain query")
	}

	if u.Fragment != "" {
		return fmt.Errorf("base_url must not contain fragment")
	}

	if !allowPrivate && isPrivateOrLoopbackHost(u.Hostname()) {
		return fmt.Errorf("base_url host %q is private/loopback (set allow_private_base_url to override)", u.Hostname())
	}

	return nil
}

func isPrivateOrLoopbackHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return true
	}

	ip := net.ParseIP(h)
	if ip == nil {
		return false
	}

	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}

	// Reject other non-global unicast ranges (e.g. multicast).
	return !ip.IsGlobalUnicast()
}
