package router

import "context"

type tenantScopeKey struct{}

// WithTenantScope returns a context carrying the tenant scope used for routing stats isolation.
// The scope should be a stable, low-cardinality identifier (e.g. API key ID, team ID, org ID).
func WithTenantScope(ctx context.Context, scope string) context.Context {
	if scope == "" {
		return ctx
	}
	return context.WithValue(ctx, tenantScopeKey{}, scope)
}

// TenantScopeFromContext returns the tenant scope stored in the context, if any.
func TenantScopeFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(tenantScopeKey{}).(string); ok {
		return v
	}
	return ""
}
