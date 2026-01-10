package auth

import "fmt"

// NewPostgresStores initializes the primary auth store and audit log store.
func NewPostgresStores(cfg *PostgresConfig) (Store, AuditLogStore, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("postgres config is nil")
	}

	store, err := NewPostgresStore(cfg)
	if err != nil {
		return nil, nil, err
	}

	auditStore := NewPostgresAuditLogStore(store.db)
	return store, auditStore, nil
}
