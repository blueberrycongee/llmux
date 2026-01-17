// Package auth provides API key authentication and multi-tenant support.
package auth

import (
	"errors"
	"net/http"
	"strings"
)

// SessionMiddleware attaches AuthContext derived from the session cookie.
func SessionMiddleware(manager *SessionManager) func(http.Handler) http.Handler {
	if manager == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		if next == nil {
			return nil
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if GetAuthContext(r.Context()) != nil {
				next.ServeHTTP(w, r)
				return
			}
			if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
				next.ServeHTTP(w, r)
				return
			}

			session, err := manager.Get(r)
			if err != nil {
				if errors.Is(err, ErrSessionExpired) || errors.Is(err, ErrSessionInvalid) {
					manager.Clear(w)
				}
				next.ServeHTTP(w, r)
				return
			}
			if session.UserID == "" {
				next.ServeHTTP(w, r)
				return
			}

			user := &User{
				ID:             session.UserID,
				Email:          strPtr(session.Email),
				Role:           string(session.Role),
				Teams:          session.TeamIDs,
				OrganizationID: strPtr(session.OrganizationID),
			}
			if session.TeamID != "" {
				user.TeamID = strPtr(session.TeamID)
			}

			authCtx := &AuthContext{
				User:       user,
				UserRole:   session.Role,
				EndUserID:  session.EndUserID,
				SSOUserID:  session.SSOUserID,
				JWTTeamIDs: session.TeamIDs,
				JWTOrgID:   session.OrganizationID,
			}

			ctx := WithAuthContext(r.Context(), authCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
