// Package api provides HTTP handlers for the LLM gateway API.
package api //nolint:revive // package name is intentional

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/goccy/go-json"
	"golang.org/x/oauth2"

	"github.com/blueberrycongee/llmux/internal/auth"
)

// AuthHandler exposes session-based authentication endpoints.
type AuthHandler struct {
	logger         *slog.Logger
	sessionManager *auth.SessionManager
	oidcConfig     auth.OIDCConfig
	provider       *oidc.Provider
	verifier       *oidc.IDTokenVerifier
	oauthConfig    oauth2.Config
	syncer         *auth.UserTeamSyncer
}

// NewAuthHandler creates a new authentication handler.
func NewAuthHandler(oidcConfig auth.OIDCConfig, sessionManager *auth.SessionManager, syncer *auth.UserTeamSyncer, logger *slog.Logger) (*AuthHandler, error) {
	if logger == nil {
		logger = slog.Default()
	}

	handler := &AuthHandler{
		logger:         logger,
		sessionManager: sessionManager,
		oidcConfig:     oidcConfig,
		syncer:         syncer,
	}

	if oidcConfig.IssuerURL == "" {
		return handler, nil
	}
	if oidcConfig.ClientID == "" {
		return nil, errors.New("oidc client id is required")
	}

	provider, err := oidc.NewProvider(context.Background(), oidcConfig.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("init oidc provider: %w", err)
	}

	handler.provider = provider
	handler.verifier = provider.Verifier(&oidc.Config{ClientID: oidcConfig.ClientID})
	handler.oauthConfig = oauth2.Config{
		ClientID:     oidcConfig.ClientID,
		ClientSecret: oidcConfig.ClientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	return handler, nil
}

// RegisterRoutes registers authentication endpoints.
func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	if mux == nil {
		return
	}

	mux.HandleFunc("GET /auth/me", h.AuthMe)
	mux.HandleFunc("GET /api/auth/me", h.AuthMe)
	mux.HandleFunc("POST /auth/logout", h.Logout)
	mux.HandleFunc("POST /api/auth/logout", h.Logout)
	mux.HandleFunc("GET /auth/logout", h.Logout)
	mux.HandleFunc("GET /api/auth/logout", h.Logout)

	mux.HandleFunc("GET /auth/oidc/login", h.OIDCLogin)
	mux.HandleFunc("GET /api/auth/oidc/login", h.OIDCLogin)
	mux.HandleFunc("GET /auth/oidc/callback", h.OIDCCallback)
	mux.HandleFunc("GET /api/auth/oidc/callback", h.OIDCCallback)
}

// OIDCLogin initiates the auth code flow.
func (h *AuthHandler) OIDCLogin(w http.ResponseWriter, r *http.Request) {
	if h.provider == nil || h.verifier == nil {
		h.writeError(w, r, http.StatusNotImplemented, "oidc not configured", "configuration_error")
		return
	}
	if h.sessionManager == nil {
		h.writeError(w, r, http.StatusNotImplemented, "session auth not configured", "configuration_error")
		return
	}

	stateToken, err := randomToken(32)
	if err != nil {
		h.writeError(w, r, http.StatusInternalServerError, "failed to generate state", "server_error")
		return
	}
	nonceToken, err := randomToken(32)
	if err != nil {
		h.writeError(w, r, http.StatusInternalServerError, "failed to generate nonce", "server_error")
		return
	}

	redirect := sanitizeRedirect(r.URL.Query().Get("redirect"))
	if err := h.sessionManager.SetState(w, &auth.OIDCState{
		State:    stateToken,
		Nonce:    nonceToken,
		Redirect: redirect,
	}); err != nil {
		h.writeError(w, r, http.StatusInternalServerError, "failed to persist state", "server_error")
		return
	}

	oauthCfg := h.oauthConfigForRequest(r)
	if oauthCfg == nil {
		h.writeError(w, r, http.StatusInternalServerError, "oidc configuration missing", "server_error")
		return
	}

	authURL := oauthCfg.AuthCodeURL(stateToken, oidc.Nonce(nonceToken))
	http.Redirect(w, r, authURL, http.StatusFound)
}

// OIDCCallback completes the auth code flow and issues a session cookie.
func (h *AuthHandler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	if h.provider == nil || h.verifier == nil {
		h.writeError(w, r, http.StatusNotImplemented, "oidc not configured", "configuration_error")
		return
	}
	if h.sessionManager == nil {
		h.writeError(w, r, http.StatusNotImplemented, "session auth not configured", "configuration_error")
		return
	}

	if errCode := r.URL.Query().Get("error"); errCode != "" {
		message := errCode
		if desc := r.URL.Query().Get("error_description"); desc != "" {
			message = fmt.Sprintf("%s: %s", errCode, desc)
		}
		h.writeError(w, r, http.StatusUnauthorized, message, "authentication_error")
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		h.sessionManager.ClearState(w)
		h.writeError(w, r, http.StatusBadRequest, "missing auth code or state", "request_error")
		return
	}

	stored, err := h.sessionManager.GetState(r)
	if err != nil {
		h.sessionManager.ClearState(w)
		h.writeError(w, r, http.StatusBadRequest, "invalid oidc state", "authentication_error")
		return
	}
	if stored.State != state {
		h.sessionManager.ClearState(w)
		h.writeError(w, r, http.StatusBadRequest, "invalid oidc state", "authentication_error")
		return
	}

	oauthCfg := h.oauthConfigForRequest(r)
	if oauthCfg == nil {
		h.writeError(w, r, http.StatusInternalServerError, "oidc configuration missing", "server_error")
		return
	}

	token, err := oauthCfg.Exchange(r.Context(), code)
	if err != nil {
		h.writeError(w, r, http.StatusUnauthorized, "failed to exchange auth code", "authentication_error")
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		h.writeError(w, r, http.StatusUnauthorized, "missing id token", "authentication_error")
		return
	}

	idToken, err := h.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		h.writeError(w, r, http.StatusUnauthorized, "invalid id token", "authentication_error")
		return
	}
	if stored.Nonce != "" && idToken.Nonce != stored.Nonce {
		h.writeError(w, r, http.StatusUnauthorized, "invalid nonce", "authentication_error")
		return
	}

	var rawClaims map[string]interface{}
	if err := idToken.Claims(&rawClaims); err != nil {
		h.writeError(w, r, http.StatusInternalServerError, "failed to parse claims", "server_error")
		return
	}

	identity := auth.ExtractOIDCIdentity(h.oidcConfig, rawClaims, idToken.Subject)
	if !auth.IsEmailDomainAllowed(h.oidcConfig, identity.Email) {
		h.writeError(w, r, http.StatusForbidden, fmt.Sprintf("email domain not allowed: expected @%s", h.oidcConfig.UserAllowedEmailDomain), "authentication_error")
		return
	}

	if h.syncer != nil {
		syncReq := &auth.SyncRequest{
			UserID:         identity.UserID,
			Email:          stringPtr(identity.Email),
			SSOUserID:      identity.SSOUserID,
			Role:           string(identity.Role),
			TeamIDs:        identity.TeamIDs,
			OrganizationID: stringPtr(identity.OrgID),
		}
		_, _ = h.syncer.SyncUserTeams(r.Context(), syncReq)
	}

	session := &auth.Session{
		UserID:         identity.UserID,
		Email:          identity.Email,
		Role:           identity.Role,
		TeamIDs:        identity.TeamIDs,
		OrganizationID: identity.OrgID,
		EndUserID:      identity.EndUserID,
		SSOUserID:      identity.SSOUserID,
	}
	if identity.User != nil && identity.User.TeamID != nil {
		session.TeamID = *identity.User.TeamID
	}

	if err := h.sessionManager.Set(w, session); err != nil {
		h.writeError(w, r, http.StatusInternalServerError, "failed to create session", "server_error")
		return
	}
	h.sessionManager.ClearState(w)

	redirect := sanitizeRedirect(stored.Redirect)
	if redirect == "" {
		redirect = "/"
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

// AuthMe returns the current authenticated principal.
func (h *AuthHandler) AuthMe(w http.ResponseWriter, r *http.Request) {
	authCtx := auth.GetAuthContext(r.Context())
	if authCtx == nil {
		h.writeError(w, r, http.StatusUnauthorized, "authentication required", "authentication_error")
		return
	}

	response := map[string]any{
		"role":        string(authCtx.UserRole),
		"team_ids":    authCtx.JWTTeamIDs,
		"org_id":      authCtx.JWTOrgID,
		"end_user_id": authCtx.EndUserID,
		"sso_user_id": authCtx.SSOUserID,
	}
	if authCtx.User != nil {
		response["user"] = authCtx.User
		if len(authCtx.JWTTeamIDs) == 0 && len(authCtx.User.Teams) > 0 {
			response["team_ids"] = authCtx.User.Teams
		}
		if authCtx.JWTOrgID == "" && authCtx.User.OrganizationID != nil {
			response["org_id"] = *authCtx.User.OrganizationID
		}
	}
	if authCtx.APIKey != nil {
		response["api_key"] = authCtx.APIKey
	}

	h.writeJSON(w, http.StatusOK, response)
}

// Logout clears the session cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if h.sessionManager != nil {
		h.sessionManager.Clear(w)
		h.sessionManager.ClearState(w)
	}
	h.writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *AuthHandler) oauthConfigForRequest(r *http.Request) *oauth2.Config {
	if h.provider == nil {
		return nil
	}
	cfg := h.oauthConfig
	callbackURL := h.callbackURL(r)
	if callbackURL != "" {
		cfg.RedirectURL = callbackURL
	}
	return &cfg
}

func (h *AuthHandler) callbackURL(r *http.Request) string {
	if h.oidcConfig.RedirectURL != "" {
		return h.oidcConfig.RedirectURL
	}
	if r == nil {
		return ""
	}
	host := r.Host
	if host == "" {
		return ""
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + host + h.callbackPath(r)
}

func (h *AuthHandler) callbackPath(r *http.Request) string {
	if r != nil && strings.HasPrefix(r.URL.Path, "/api/") {
		return "/api/auth/oidc/callback"
	}
	return "/auth/oidc/callback"
}

func (h *AuthHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode JSON response", "error", err)
	}
}

func (h *AuthHandler) writeError(w http.ResponseWriter, r *http.Request, status int, message, typ string) {
	message = localizeManagementMessage(detectLocaleFromRequest(r), message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"message": message,
			"type":    typ,
		},
	}); err != nil {
		h.logger.Error("failed to encode error response", "error", err)
	}
}

func sanitizeRedirect(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "//") {
		return ""
	}
	if strings.HasPrefix(value, "/") {
		return value
	}
	return ""
}

func randomToken(size int) (string, error) {
	if size <= 0 {
		return "", errors.New("size must be positive")
	}
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
