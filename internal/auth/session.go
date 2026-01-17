// Package auth provides API key authentication and multi-tenant support.
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultSessionCookieName = "llmux_session"
	defaultStateCookieName   = "llmux_oidc_state"
	defaultSessionTTL        = 12 * time.Hour
	defaultStateTTL          = 10 * time.Minute
)

// Session represents the authenticated user context stored in a cookie.
type Session struct {
	UserID         string    `json:"user_id"`
	Email          string    `json:"email,omitempty"`
	Role           UserRole  `json:"role,omitempty"`
	TeamID         string    `json:"team_id,omitempty"`
	TeamIDs        []string  `json:"team_ids,omitempty"`
	OrganizationID string    `json:"organization_id,omitempty"`
	EndUserID      string    `json:"end_user_id,omitempty"`
	SSOUserID      string    `json:"sso_user_id,omitempty"`
	IssuedAt       time.Time `json:"issued_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// OIDCState stores temporary state for the OIDC auth code flow.
type OIDCState struct {
	State        string    `json:"state"`
	Nonce        string    `json:"nonce"`
	CodeVerifier string    `json:"code_verifier,omitempty"`
	Redirect     string    `json:"redirect,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// SessionManagerConfig configures the session cookie behavior.
type SessionManagerConfig struct {
	Secret          string
	CookieName      string
	StateCookieName string
	CookieDomain    string
	CookiePath      string
	CookieSecure    bool
	CookieSameSite  string
	TTL             time.Duration
	StateTTL        time.Duration
}

// SessionManager encodes and decodes session cookies.
type SessionManager struct {
	codec           *cookieCodec
	cookieName      string
	stateCookieName string
	domain          string
	path            string
	secure          bool
	sameSite        http.SameSite
	ttl             time.Duration
	stateTTL        time.Duration
}

// Session errors for control flow decisions.
var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
	ErrSessionInvalid  = errors.New("session invalid")
	ErrStateNotFound   = errors.New("state not found")
	ErrStateExpired    = errors.New("state expired")
	ErrStateInvalid    = errors.New("state invalid")
)

// NewSessionManager creates a new session manager.
func NewSessionManager(cfg SessionManagerConfig) (*SessionManager, error) {
	if strings.TrimSpace(cfg.Secret) == "" {
		return nil, errors.New("session secret is required")
	}

	codec, err := newCookieCodec(cfg.Secret)
	if err != nil {
		return nil, err
	}

	cookieName := strings.TrimSpace(cfg.CookieName)
	if cookieName == "" {
		cookieName = defaultSessionCookieName
	}

	stateCookieName := strings.TrimSpace(cfg.StateCookieName)
	if stateCookieName == "" {
		stateCookieName = defaultStateCookieName
	}

	cookiePath := strings.TrimSpace(cfg.CookiePath)
	if cookiePath == "" {
		cookiePath = "/"
	}

	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}

	stateTTL := cfg.StateTTL
	if stateTTL <= 0 {
		stateTTL = defaultStateTTL
	}

	sameSite := parseSameSite(cfg.CookieSameSite)

	return &SessionManager{
		codec:           codec,
		cookieName:      cookieName,
		stateCookieName: stateCookieName,
		domain:          strings.TrimSpace(cfg.CookieDomain),
		path:            cookiePath,
		secure:          cfg.CookieSecure,
		sameSite:        sameSite,
		ttl:             ttl,
		stateTTL:        stateTTL,
	}, nil
}

// Set writes a session cookie.
func (m *SessionManager) Set(w http.ResponseWriter, session *Session) error {
	if w == nil {
		return errors.New("response writer is nil")
	}
	if session == nil {
		return errors.New("session is nil")
	}

	now := time.Now()
	if session.IssuedAt.IsZero() {
		session.IssuedAt = now
	}
	if session.ExpiresAt.IsZero() {
		session.ExpiresAt = now.Add(m.ttl)
	}

	value, err := m.codec.encode(session)
	if err != nil {
		return err
	}

	http.SetCookie(w, m.buildCookie(m.cookieName, value, session.ExpiresAt))
	return nil
}

// Get reads a session cookie.
func (m *SessionManager) Get(r *http.Request) (*Session, error) {
	if r == nil {
		return nil, ErrSessionNotFound
	}

	cookie, err := r.Cookie(m.cookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return nil, ErrSessionNotFound
		}
		return nil, ErrSessionInvalid
	}

	var session Session
	if err := m.codec.decode(cookie.Value, &session); err != nil {
		return nil, ErrSessionInvalid
	}

	if session.ExpiresAt.IsZero() {
		return nil, ErrSessionInvalid
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	return &session, nil
}

// Clear removes the session cookie.
func (m *SessionManager) Clear(w http.ResponseWriter) {
	if w == nil {
		return
	}
	cookie := m.buildCookie(m.cookieName, "", time.Time{})
	cookie.MaxAge = -1
	http.SetCookie(w, cookie)
}

// SetState writes the OIDC state cookie.
func (m *SessionManager) SetState(w http.ResponseWriter, state *OIDCState) error {
	if w == nil {
		return errors.New("response writer is nil")
	}
	if state == nil {
		return errors.New("state is nil")
	}

	if state.ExpiresAt.IsZero() {
		state.ExpiresAt = time.Now().Add(m.stateTTL)
	}

	value, err := m.codec.encode(state)
	if err != nil {
		return err
	}

	http.SetCookie(w, m.buildCookie(m.stateCookieName, value, state.ExpiresAt))
	return nil
}

// GetState reads the OIDC state cookie.
func (m *SessionManager) GetState(r *http.Request) (*OIDCState, error) {
	if r == nil {
		return nil, ErrStateNotFound
	}

	cookie, err := r.Cookie(m.stateCookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return nil, ErrStateNotFound
		}
		return nil, ErrStateInvalid
	}

	var state OIDCState
	if err := m.codec.decode(cookie.Value, &state); err != nil {
		return nil, ErrStateInvalid
	}

	if state.ExpiresAt.IsZero() {
		return nil, ErrStateInvalid
	}
	if time.Now().After(state.ExpiresAt) {
		return nil, ErrStateExpired
	}

	return &state, nil
}

// ClearState removes the OIDC state cookie.
func (m *SessionManager) ClearState(w http.ResponseWriter) {
	if w == nil {
		return
	}
	cookie := m.buildCookie(m.stateCookieName, "", time.Time{})
	cookie.MaxAge = -1
	http.SetCookie(w, cookie)
}

func (m *SessionManager) buildCookie(name, value string, expiresAt time.Time) *http.Cookie {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     m.path,
		Domain:   m.domain,
		Secure:   m.secure,
		HttpOnly: true,
		SameSite: m.sameSite,
	}
	if !expiresAt.IsZero() {
		cookie.Expires = expiresAt
		cookie.MaxAge = int(time.Until(expiresAt).Seconds())
		if cookie.MaxAge < 0 {
			cookie.MaxAge = -1
		}
	}
	return cookie
}

func parseSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

type cookieCodec struct {
	aead cipher.AEAD
}

func newCookieCodec(secret string) (*cookieCodec, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &cookieCodec{aead: aead}, nil
}

func (c *cookieCodec) encode(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode payload: %w", err)
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := c.aead.Seal(nil, nonce, payload, nil)
	token := make([]byte, 0, len(nonce)+len(ciphertext))
	token = append(token, nonce...)
	token = append(token, ciphertext...)
	return base64.RawURLEncoding.EncodeToString(token), nil
}

func (c *cookieCodec) decode(token string, out any) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("empty token")
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return fmt.Errorf("decode token: %w", err)
	}
	if len(raw) < c.aead.NonceSize() {
		return errors.New("token too short")
	}

	nonce := raw[:c.aead.NonceSize()]
	ciphertext := raw[c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decrypt token: %w", err)
	}

	if err := json.Unmarshal(plaintext, out); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	return nil
}
