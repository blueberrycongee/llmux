package main

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/blueberrycongee/llmux/internal/auth"
	"github.com/blueberrycongee/llmux/internal/config"
)

type fakeJobRunner struct {
	started bool
	stopped bool
}

func (f *fakeJobRunner) Start() {
	f.started = true
}

func (f *fakeJobRunner) Stop() {
	f.stopped = true
}

func TestStartJobRunner_EnabledStarts(t *testing.T) {
	cfg := &config.Config{
		Governance: config.GovernanceConfig{Enabled: true},
	}
	store := auth.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	runner := &fakeJobRunner{}
	var gotCfg *auth.JobRunnerConfig
	newRunner := func(cfg *auth.JobRunnerConfig) jobRunner {
		gotCfg = cfg
		return runner
	}

	job := startJobRunner(cfg, store, logger, newRunner)
	require.Equal(t, runner, job)
	require.NotNil(t, gotCfg)
	require.Equal(t, store, gotCfg.Store)
	require.NotNil(t, gotCfg.Logger)
	require.True(t, runner.started)
}

func TestStartJobRunner_DisabledOrNilSkips(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := auth.NewMemoryStore()

	cases := []struct {
		name string
		cfg  *config.Config
		st   auth.Store
	}{
		{name: "nil config", cfg: nil, st: store},
		{name: "governance disabled", cfg: &config.Config{Governance: config.GovernanceConfig{Enabled: false}}, st: store},
		{name: "nil store", cfg: &config.Config{Governance: config.GovernanceConfig{Enabled: true}}, st: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			newRunner := func(cfg *auth.JobRunnerConfig) jobRunner {
				called = true
				return &fakeJobRunner{}
			}
			job := startJobRunner(tc.cfg, tc.st, logger, newRunner)
			require.Nil(t, job)
			require.False(t, called)
		})
	}
}
