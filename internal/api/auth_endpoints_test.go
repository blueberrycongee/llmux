package api_test

import (
	"crypto/sha256"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	apipkg "github.com/blueberrycongee/llmux/internal/api"
	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/tests/testutil"
)

func TestOIDCLogin_SetsPKCEChallenge(t *testing.T) {
	provider, err := testutil.NewMockOIDCProvider()
	if err != nil {
		t.Fatalf("NewMockOIDCProvider() error = %v", err)
	}
	defer provider.Close()

	sessionManager := newSessionManager(t)
	handler := newAuthHandler(t, provider, sessionManager)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/auth/oidc/login", nil)
	handler.OIDCLogin(recorder, req)

	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusFound)
	}

	location := recorder.Header().Get("Location")
	if location == "" {
		t.Fatal("missing Location header")
	}

	redirectURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse redirect url: %v", err)
	}
	query := redirectURL.Query()

	state := query.Get("state")
	if state == "" {
		t.Fatal("missing state")
	}

	codeChallenge := query.Get("code_challenge")
	if codeChallenge == "" {
		t.Fatal("missing code_challenge")
	}
	if method := query.Get("code_challenge_method"); method != "S256" {
		t.Fatalf("code_challenge_method = %q, want S256", method)
	}

	reqWithCookie := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	for _, cookie := range recorder.Result().Cookies() {
		reqWithCookie.AddCookie(cookie)
	}

	stored, err := sessionManager.GetState(reqWithCookie)
	if err != nil {
		t.Fatalf("GetState() error = %v", err)
	}
	if stored.State != state {
		t.Fatalf("state = %q, want %q", stored.State, state)
	}
	if stored.CodeVerifier == "" {
		t.Fatal("missing code verifier in state")
	}

	expectedChallenge := codeChallengeS256(stored.CodeVerifier)
	if codeChallenge != expectedChallenge {
		t.Fatalf("code_challenge = %q, want %q", codeChallenge, expectedChallenge)
	}
}

func TestOIDCCallback_IssuesSession(t *testing.T) {
	provider, err := testutil.NewMockOIDCProvider()
	if err != nil {
		t.Fatalf("NewMockOIDCProvider() error = %v", err)
	}
	defer provider.Close()
	provider.RequireCodeVerifier()

	sessionManager := newSessionManager(t)
	handler := newAuthHandler(t, provider, sessionManager)

	loginRecorder := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodGet, "http://example.com/auth/oidc/login", nil)
	handler.OIDCLogin(loginRecorder, loginReq)

	if loginRecorder.Code != http.StatusFound {
		t.Fatalf("login status = %d, want %d", loginRecorder.Code, http.StatusFound)
	}

	location := loginRecorder.Header().Get("Location")
	redirectURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse redirect url: %v", err)
	}
	state := redirectURL.Query().Get("state")
	if state == "" {
		t.Fatal("missing state")
	}

	stateReq := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	for _, cookie := range loginRecorder.Result().Cookies() {
		stateReq.AddCookie(cookie)
	}
	stored, err := sessionManager.GetState(stateReq)
	if err != nil {
		t.Fatalf("GetState() error = %v", err)
	}

	provider.SetTokenClaims(map[string]interface{}{
		"sub":   "user-123",
		"email": "user@example.com",
		"nonce": stored.Nonce,
	})

	callbackRecorder := httptest.NewRecorder()
	callbackReq := httptest.NewRequest(http.MethodGet, "http://example.com/auth/oidc/callback?code=code123&state="+state, nil)
	for _, cookie := range loginRecorder.Result().Cookies() {
		callbackReq.AddCookie(cookie)
	}

	handler.OIDCCallback(callbackRecorder, callbackReq)

	if callbackRecorder.Code != http.StatusFound {
		t.Fatalf("callback status = %d, want %d", callbackRecorder.Code, http.StatusFound)
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	for _, cookie := range callbackRecorder.Result().Cookies() {
		sessionReq.AddCookie(cookie)
	}
	session, err := sessionManager.Get(sessionReq)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if session.UserID != "user-123" {
		t.Fatalf("UserID = %q, want %q", session.UserID, "user-123")
	}
	if session.Email != "user@example.com" {
		t.Fatalf("Email = %q, want %q", session.Email, "user@example.com")
	}
}

func newSessionManager(t *testing.T) *auth.SessionManager {
	t.Helper()
	manager, err := auth.NewSessionManager(auth.SessionManagerConfig{
		Secret:       "test-secret",
		CookieSecure: true,
	})
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}
	return manager
}

func newAuthHandler(t *testing.T, provider *testutil.MockOIDCProvider, manager *auth.SessionManager) *apipkg.AuthHandler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler, err := apipkg.NewAuthHandler(auth.OIDCConfig{
		IssuerURL:    provider.URL(),
		ClientID:     "llmux-client-id",
		ClientSecret: "secret",
	}, manager, nil, logger)
	if err != nil {
		t.Fatalf("NewAuthHandler() error = %v", err)
	}
	return handler
}

func codeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
