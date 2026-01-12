package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// Manager handles configuration loading and hot-reload.
// It uses atomic pointer swaps to ensure thread-safe config updates.
type Manager struct {
	config      atomic.Pointer[Config]
	path        string
	watcher     *fsnotify.Watcher
	onChange    []func(*Config)
	logger      *slog.Logger
	checksum    atomic.Value
	loadedAt    atomic.Value
	reloadCount atomic.Uint64
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
	if err := m.storeConfig(cfg); err != nil {
		return nil, err
	}

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

// ConfigStatus contains the current config metadata.
type ConfigStatus struct {
	Path        string    `json:"path"`
	Checksum    string    `json:"checksum"`
	LoadedAt    time.Time `json:"loaded_at"`
	ReloadCount uint64    `json:"reload_count"`
}

// Status returns metadata about the active configuration.
func (m *Manager) Status() ConfigStatus {
	status := ConfigStatus{
		Path:        m.path,
		ReloadCount: m.reloadCount.Load(),
	}
	if value, ok := m.checksum.Load().(string); ok {
		status.Checksum = value
	}
	if value, ok := m.loadedAt.Load().(time.Time); ok {
		status.LoadedAt = value
	}
	return status
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
					if err := m.Reload(); err != nil {
						m.logger.Error("failed to reload config, keeping current", "error", err)
					}
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

// Reload forces a configuration reload from disk.
func (m *Manager) Reload() error {
	newCfg, err := LoadFromFile(m.path)
	if err != nil {
		return err
	}

	// Atomic swap
	if err := m.storeConfig(newCfg); err != nil {
		return err
	}
	m.logger.Info("configuration reloaded successfully")

	// Notify listeners
	for _, fn := range m.onChange {
		fn(newCfg)
	}
	return nil
}

// Close stops the configuration watcher.
func (m *Manager) Close() error {
	if m.watcher != nil {
		return m.watcher.Close()
	}
	return nil
}

func (m *Manager) storeConfig(cfg *Config) error {
	checksum, err := configChecksum(cfg)
	if err != nil {
		return err
	}
	m.config.Store(cfg)
	m.checksum.Store(checksum)
	m.loadedAt.Store(time.Now().UTC())
	m.reloadCount.Add(1)
	return nil
}

func configChecksum(cfg *Config) (string, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
