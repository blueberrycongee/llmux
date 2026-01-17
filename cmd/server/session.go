package main

import (
	"fmt"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
)

func buildSessionManager(cfg *config.Config) (*auth.SessionManager, error) {
	if cfg == nil {
		return nil, errNilConfig
	}
	if !cfg.Auth.Session.Enabled {
		return nil, nil
	}

	manager, err := auth.NewSessionManager(auth.SessionManagerConfig{
		Secret:          cfg.Auth.Session.Secret,
		CookieName:      cfg.Auth.Session.CookieName,
		StateCookieName: cfg.Auth.Session.StateCookieName,
		CookieDomain:    cfg.Auth.Session.CookieDomain,
		CookiePath:      cfg.Auth.Session.CookiePath,
		CookieSecure:    cfg.Auth.Session.CookieSecure,
		CookieSameSite:  cfg.Auth.Session.CookieSameSite,
		TTL:             cfg.Auth.Session.TTL,
		StateTTL:        cfg.Auth.Session.StateTTL,
	})
	if err != nil {
		return nil, fmt.Errorf("init session manager: %w", err)
	}

	return manager, nil
}
