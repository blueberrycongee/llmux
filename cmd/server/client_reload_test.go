package main

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	llmux "github.com/blueberrycongee/llmux"
	"github.com/blueberrycongee/llmux/internal/api"
	"github.com/blueberrycongee/llmux/internal/config"
	"github.com/stretchr/testify/require"
)

func TestClientReloaderSwapsClientOnSuccess(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{}))

	initial, err := llmux.New()
	require.NoError(t, err)

	next, err := llmux.New()
	require.NoError(t, err)

	swapper := api.NewClientSwapper(initial)
	t.Cleanup(swapper.Close)

	reloader := newClientReloader(logger, swapper, func(*config.Config) (*llmux.Client, error) {
		return next, nil
	})

	reloader.Reload(&config.Config{})

	require.Same(t, next, swapper.Current())
}

func TestClientReloaderKeepsClientOnFailure(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{}))

	initial, err := llmux.New()
	require.NoError(t, err)

	swapper := api.NewClientSwapper(initial)
	t.Cleanup(swapper.Close)

	reloader := newClientReloader(logger, swapper, func(*config.Config) (*llmux.Client, error) {
		return nil, errTestReload
	})

	reloader.Reload(&config.Config{})

	require.Same(t, initial, swapper.Current())
}

var errTestReload = errors.New("reload failed")
