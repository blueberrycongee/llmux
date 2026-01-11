package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/blueberrycongee/llmux/internal/config"
)

func corsMiddleware(cfg config.CORSConfig, next http.Handler) http.Handler {
	if !cfg.Enabled {
		return next
	}

	allowMethods := strings.Join(cfg.AllowMethods, ", ")
	allowHeaders := strings.Join(cfg.AllowHeaders, ", ")
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ", ")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		policy := cfg.DataOrigins
		if isAdminPath(r.URL.Path, cfg.AdminPathPrefixes) {
			policy = cfg.AdminOrigins
		}

		if !isOriginAllowed(origin, policy, cfg.AllowAllOrigins) {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		allowOrigin := origin
		if cfg.AllowAllOrigins && !cfg.AllowCredentials {
			allowOrigin = "*"
		} else {
			w.Header().Add("Vary", "Origin")
		}

		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		if cfg.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if allowMethods != "" {
			w.Header().Set("Access-Control-Allow-Methods", allowMethods)
		}
		if allowHeaders != "" {
			w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
		}
		if exposeHeaders != "" {
			w.Header().Set("Access-Control-Expose-Headers", exposeHeaders)
		}
		if cfg.MaxAge > 0 {
			w.Header().Set("Access-Control-Max-Age", strconv.FormatInt(int64(cfg.MaxAge.Seconds()), 10))
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isAdminPath(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func isOriginAllowed(origin string, policy config.CORSOrigins, allowAll bool) bool {
	if isOriginDenied(origin, policy.Denylist) {
		return false
	}
	if allowAll {
		return true
	}
	if len(policy.Allowlist) == 0 {
		return false
	}
	for _, allowed := range policy.Allowlist {
		if origin == allowed {
			return true
		}
	}
	return false
}

func isOriginDenied(origin string, denylist []string) bool {
	for _, denied := range denylist {
		if denied == "*" || denied == origin {
			return true
		}
	}
	return false
}
