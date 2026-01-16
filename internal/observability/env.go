package observability

import (
	"os"
	"strings"
)

func envBool(key string, defaultValue bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	if strings.EqualFold(value, "true") || value == "1" {
		return true
	}
	if strings.EqualFold(value, "false") || value == "0" {
		return false
	}
	return defaultValue
}
