package api //nolint:revive // package name is intentional

const (
	// DefaultMaxBodySize is the default maximum request body size (10MB).
	// This accommodates large context windows while preventing abuse.
	DefaultMaxBodySize = 10 * 1024 * 1024
)
