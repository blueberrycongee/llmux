package mcp

import (
	"net/http"
)

// Middleware returns an HTTP middleware that injects the MCP manager into request context.
func Middleware(manager Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithManager(r.Context(), manager)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ToolInjectionMiddleware returns middleware that automatically injects MCP tools into requests.
// This is useful when you want tools to be automatically added without modifying handlers.
func ToolInjectionMiddleware(manager Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Just inject the manager; actual tool injection happens in the handler
			ctx := WithManager(r.Context(), manager)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
