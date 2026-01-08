package config

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Manager handles configuration loading and hot-reload.
// It uses atomic pointer swaps to ensure thread-safe config updates.
type Manager struct {
	config   atomic.Pointer[Config]
	path     string
	watcher  *fsnotify.Watcher
	onChange []func(*Config)
	logger   *slog.Logger
}

// NewManager creates a new configuration manager.
func NewManager(path string, logger *slog.Logger) (*Manager, error) {
	cfg, err := LoadFromFile(path)
	if err != nil {
		return nil, err
	}

	m := &Manager{
		path:   path,
		logger: logger,
	}
	m.config.Store(cfg)

	return m, nil
}

// Get returns the current configuration.
// This is safe to call concurrently from multiple goroutines.
func (m *Manager) Get() *Config {
	return m.config.Load()
}

// OnChange registers a callback to be invoked when configuration changes.
func (m *Manager) OnChange(fn func(*Config)) {
	m.onChange = append(m.onChange, fn)
}

// Watch starts watching the configuration file for changes.
// It debounces rapid changes and reloads configuration atomically.
func (m *Manager) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	m.watcher = watcher

	if err := watcher.Add(m.path); err != nil {
		_ = watcher.Close()
		return err
	}

	go m.watchLoop(ctx)
	return nil
}

func (m *Manager) watchLoop(ctx context.Context) {
	// Debounce timer to avoid rapid reloads
	const debounceDelay = 500 * time.Millisecond
	var debounceTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			_ = m.watcher.Close()
			return

		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}

			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				// Reset debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(debounceDelay, func() {
					m.reload()
				})
			}

		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			m.logger.Error("config watcher error", "error", err)
		}
	}
}

func (m *Manager) reload() {
	newCfg, err := LoadFromFile(m.path)
	if err != nil {
		m.logger.Error("failed to reload config, keeping current",
			"error", err,
		)
		return
	}

	// Atomic swap
	m.config.Store(newCfg)
	m.logger.Info("configuration reloaded successfully")

	// Notify listeners
	for _, fn := range m.onChange {
		fn(newCfg)
	}
}

// Close stops the configuration watcher.
func (m *Manager) Close() error {
	if m.watcher != nil {
		return m.watcher.Close()
	}
	return nil
}
