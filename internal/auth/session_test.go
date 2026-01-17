package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionManager_RoundTrip(t *testing.T) {
	manager, err := NewSessionManager(SessionManagerConfig{
		Secret:       "test-secret",
		CookieSecure: true,
	})
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	original := &Session{
		UserID:    "user-1",
		Email:     "user@example.com",
		Role:      UserRoleProxyAdmin,
		TeamID:    "team-1",
		TeamIDs:   []string{"team-1", "team-2"},
		SSOUserID: "sso-1",
	}

	recorder := httptest.NewRecorder()
	if err = manager.Set(recorder, original); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	for _, cookie := range recorder.Result().Cookies() {
		req.AddCookie(cookie)
	}

	loaded, err := manager.Get(req)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if loaded.UserID != original.UserID {
		t.Fatalf("UserID = %q, want %q", loaded.UserID, original.UserID)
	}
	if loaded.Email != original.Email {
		t.Fatalf("Email = %q, want %q", loaded.Email, original.Email)
	}
	if loaded.Role != original.Role {
		t.Fatalf("Role = %q, want %q", loaded.Role, original.Role)
	}
	if loaded.TeamID != original.TeamID {
		t.Fatalf("TeamID = %q, want %q", loaded.TeamID, original.TeamID)
	}
	if loaded.SSOUserID != original.SSOUserID {
		t.Fatalf("SSOUserID = %q, want %q", loaded.SSOUserID, original.SSOUserID)
	}
}

func TestSessionManager_ExpiredSession(t *testing.T) {
	manager, err := NewSessionManager(SessionManagerConfig{
		Secret:       "test-secret",
		CookieSecure: true,
	})
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	expired := &Session{
		UserID:    "user-1",
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	value, err := manager.codec.encode(expired)
	if err != nil {
		t.Fatalf("encode() error = %v", err)
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.AddCookie(&http.Cookie{
		Name:  manager.cookieName,
		Value: value,
	})

	_, err = manager.Get(req)
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("Get() error = %v, want %v", err, ErrSessionExpired)
	}
}

func TestSessionManager_StateRoundTrip(t *testing.T) {
	manager, err := NewSessionManager(SessionManagerConfig{
		Secret:       "test-secret",
		CookieSecure: true,
	})
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	state := &OIDCState{
		State:        "state-1",
		Nonce:        "nonce-1",
		CodeVerifier: "code-verifier",
		Redirect:     "/dashboard",
	}

	recorder := httptest.NewRecorder()
	if err = manager.SetState(recorder, state); err != nil {
		t.Fatalf("SetState() error = %v", err)
	}

	req := httptest.NewRequest("GET", "http://example.com", nil)
	for _, cookie := range recorder.Result().Cookies() {
		req.AddCookie(cookie)
	}

	loaded, err := manager.GetState(req)
	if err != nil {
		t.Fatalf("GetState() error = %v", err)
	}
	if loaded.State != state.State {
		t.Fatalf("State = %q, want %q", loaded.State, state.State)
	}
	if loaded.Nonce != state.Nonce {
		t.Fatalf("Nonce = %q, want %q", loaded.Nonce, state.Nonce)
	}
	if loaded.Redirect != state.Redirect {
		t.Fatalf("Redirect = %q, want %q", loaded.Redirect, state.Redirect)
	}
}
