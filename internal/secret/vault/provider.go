// Package vault implements a secret provider that reads from HashiCorp Vault.
package vault

import (
	"context"
	"fmt"
	"strings"
	"sync"

	vault "github.com/hashicorp/vault/api"
)

// Provider implements the secret.Provider interface for HashiCorp Vault.
type Provider struct {
	client *vault.Client
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// New creates a new Vault provider and authenticates using AppRole.
func New(addr, roleID, secretID string) (*Provider, error) {
	config := vault.DefaultConfig()
	config.Address = addr

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("create vault client: %w", err)
	}

	// Initial login
	secret, err := client.Logical().Write("auth/approle/login", map[string]interface{}{
		"role_id":   roleID,
		"secret_id": secretID,
	})
	if err != nil {
		return nil, fmt.Errorf("vault login: %w", err)
	}

	if secret == nil || secret.Auth == nil {
		return nil, fmt.Errorf("vault login returned no auth info")
	}

	client.SetToken(secret.Auth.ClientToken)

	p := &Provider{
		client: client,
		stopCh: make(chan struct{}),
	}

	// Start token renewer
	p.wg.Add(1)
	go p.startTokenRenewer(secret.Auth)

	return p, nil
}

// Get retrieves a secret from Vault.
// Path format: "path/to/secret#key"
// If #key is omitted, defaults to "value".
func (p *Provider) Get(ctx context.Context, path string) (string, error) {
	// Parse path and key
	secretPath := path
	key := "value"
	if idx := strings.LastIndex(path, "#"); idx != -1 {
		secretPath = path[:idx]
		key = path[idx+1:]
	}

	// Read from Vault
	secret, err := p.client.Logical().ReadWithContext(ctx, secretPath)
	if err != nil {
		return "", fmt.Errorf("read vault secret %q: %w", secretPath, err)
	}
	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("secret %q not found", secretPath)
	}

	// Handle KV v2 "data" wrapper
	data := secret.Data
	if v, ok := data["data"]; ok {
		if nested, ok := v.(map[string]interface{}); ok {
			data = nested
		}
	}

	val, ok := data[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret %q", key, secretPath)
	}

	return fmt.Sprintf("%v", val), nil
}

// Close stops the token renewer and releases resources.
func (p *Provider) Close() error {
	close(p.stopCh)
	p.wg.Wait()
	return nil
}

func (p *Provider) startTokenRenewer(auth *vault.SecretAuth) {
	defer p.wg.Done()

	// If the token is not renewable, just return
	if !auth.Renewable {
		return
	}

	watcher, err := p.client.NewLifetimeWatcher(&vault.LifetimeWatcherInput{
		Secret: &vault.Secret{Auth: auth},
	})
	if err != nil {
		// In a real app, we should log this error
		fmt.Printf("failed to create vault lifetime watcher: %v\n", err)
		return
	}

	go watcher.Start()
	defer watcher.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case err := <-watcher.DoneCh():
			if err != nil {
				fmt.Printf("vault token renewal error: %v\n", err)
			}
			return
		case <-watcher.RenewCh():
			// Token successfully renewed
		}
	}
}
