// Package auth provides API key authentication and multi-tenant support.
package auth

import (
	"context"
	"log/slog"
	"time"
)

// ============================================================================
// Background Jobs for Budget Reset and Key Rotation
// ============================================================================

// JobRunner manages background jobs for budget reset and key rotation.
type JobRunner struct {
	store    Store
	logger   *slog.Logger
	interval time.Duration
	stopCh   chan struct{}
}

// JobRunnerConfig contains configuration for the job runner.
type JobRunnerConfig struct {
	Store    Store
	Logger   *slog.Logger
	Interval time.Duration // How often to run jobs (default: 1 hour)
}

// NewJobRunner creates a new job runner.
func NewJobRunner(cfg *JobRunnerConfig) *JobRunner {
	interval := cfg.Interval
	if interval <= 0 {
		interval = time.Hour
	}

	return &JobRunner{
		store:    cfg.Store,
		logger:   cfg.Logger,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start starts the background job runner.
func (j *JobRunner) Start() {
	go j.run()
}

// Stop stops the background job runner.
func (j *JobRunner) Stop() {
	close(j.stopCh)
}

func (j *JobRunner) run() {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	// Run immediately on start
	j.runJobs()

	for {
		select {
		case <-ticker.C:
			j.runJobs()
		case <-j.stopCh:
			j.logger.Info("job runner stopped")
			return
		}
	}
}

func (j *JobRunner) runJobs() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	j.logger.Debug("running background jobs")

	// Run budget reset job
	j.resetBudgets(ctx)

	// Run key rotation job
	if err := j.rotateKeys(ctx); err != nil {
		j.logger.Error("key rotation job failed", "error", err)
	}
}

// ============================================================================
// Budget Reset Job
// ============================================================================

func (j *JobRunner) resetBudgets(ctx context.Context) {
	j.logger.Debug("starting budget reset job")

	// Reset API key budgets
	if err := j.resetKeyBudgets(ctx); err != nil {
		j.logger.Error("failed to reset key budgets", "error", err)
	}

	// Reset team budgets
	if err := j.resetTeamBudgets(ctx); err != nil {
		j.logger.Error("failed to reset team budgets", "error", err)
	}

	// Reset user budgets
	if err := j.resetUserBudgets(ctx); err != nil {
		j.logger.Error("failed to reset user budgets", "error", err)
	}
}

func (j *JobRunner) resetKeyBudgets(ctx context.Context) error {
	keys, err := j.store.GetKeysNeedingBudgetReset(ctx)
	if err != nil {
		return err
	}

	j.logger.Info("found keys needing budget reset", "count", len(keys))

	for _, key := range keys {
		if err := j.store.ResetAPIKeyBudget(ctx, key.ID); err != nil {
			j.logger.Warn("failed to reset key budget", "key_id", key.ID, "error", err)
			continue
		}
		j.logger.Debug("reset budget for key", "key_id", key.ID, "key_name", key.Name)
	}

	return nil
}

func (j *JobRunner) resetTeamBudgets(ctx context.Context) error {
	teams, err := j.store.GetTeamsNeedingBudgetReset(ctx)
	if err != nil {
		return err
	}

	j.logger.Info("found teams needing budget reset", "count", len(teams))

	for _, team := range teams {
		if err := j.store.ResetTeamBudget(ctx, team.ID); err != nil {
			j.logger.Warn("failed to reset team budget", "team_id", team.ID, "error", err)
			continue
		}
		j.logger.Debug("reset budget for team", "team_id", team.ID)
	}

	return nil
}

func (j *JobRunner) resetUserBudgets(ctx context.Context) error {
	users, err := j.store.GetUsersNeedingBudgetReset(ctx)
	if err != nil {
		return err
	}

	j.logger.Info("found users needing budget reset", "count", len(users))

	for _, user := range users {
		if err := j.store.ResetUserBudget(ctx, user.ID); err != nil {
			j.logger.Warn("failed to reset user budget", "user_id", user.ID, "error", err)
			continue
		}
		j.logger.Debug("reset budget for user", "user_id", user.ID)
	}

	return nil
}

// ============================================================================
// Key Rotation Job
// ============================================================================

func (j *JobRunner) rotateKeys(ctx context.Context) error {
	j.logger.Debug("starting key rotation job")

	// Find keys that need rotation
	keys, err := j.findKeysNeedingRotation(ctx)
	if err != nil {
		return err
	}

	if len(keys) == 0 {
		j.logger.Debug("no keys need rotation")
		return nil
	}

	j.logger.Info("found keys needing rotation", "count", len(keys))

	for _, key := range keys {
		if err := j.rotateKey(ctx, key); err != nil {
			j.logger.Error("failed to rotate key", "key_id", key.ID, "error", err)
			continue
		}
		j.logger.Info("rotated key", "key_id", key.ID, "key_name", key.Name)
	}

	return nil
}

func (j *JobRunner) findKeysNeedingRotation(ctx context.Context) ([]*APIKey, error) {
	// Get all active keys
	keys, _, err := j.store.ListAPIKeys(ctx, APIKeyFilter{
		IsActive: boolPtr(true),
		Limit:    1000,
	})
	if err != nil {
		return nil, err
	}

	// Filter keys that need rotation
	now := time.Now()
	needsRotation := make([]*APIKey, 0)

	for _, key := range keys {
		if key.Metadata == nil {
			continue
		}

		// Check if auto_rotate is enabled
		autoRotate, ok := key.Metadata["auto_rotate"].(bool)
		if !ok || !autoRotate {
			continue
		}

		// Check rotation time
		rotationAtStr, ok := key.Metadata["key_rotation_at"].(string)
		if !ok {
			// No rotation time set, needs rotation
			needsRotation = append(needsRotation, key)
			continue
		}

		rotationAt, err := time.Parse(time.RFC3339, rotationAtStr)
		if err != nil {
			continue
		}

		if now.After(rotationAt) {
			needsRotation = append(needsRotation, key)
		}
	}

	return needsRotation, nil
}

func (j *JobRunner) rotateKey(ctx context.Context, oldKey *APIKey) error {
	// Generate new key credentials
	fullKey, keyHash, err := GenerateAPIKey()
	if err != nil {
		return err
	}
	keyPrefix := ExtractKeyPrefix(fullKey)

	// Create new key with same settings
	now := time.Now()
	newKey := &APIKey{
		ID:                  GenerateUUID(),
		KeyHash:             keyHash,
		KeyPrefix:           keyPrefix,
		Name:                oldKey.Name,
		KeyAlias:            oldKey.KeyAlias,
		TeamID:              oldKey.TeamID,
		UserID:              oldKey.UserID,
		OrganizationID:      oldKey.OrganizationID,
		AllowedModels:       oldKey.AllowedModels,
		KeyType:             oldKey.KeyType,
		TPMLimit:            oldKey.TPMLimit,
		RPMLimit:            oldKey.RPMLimit,
		MaxParallelRequests: oldKey.MaxParallelRequests,
		MaxBudget:           oldKey.MaxBudget,
		SoftBudget:          oldKey.SoftBudget,
		SpentBudget:         0, // Reset spend on rotation
		BudgetDuration:      oldKey.BudgetDuration,
		BudgetResetAt:       oldKey.BudgetResetAt,
		ModelMaxBudget:      oldKey.ModelMaxBudget,
		ModelTPMLimit:       oldKey.ModelTPMLimit,
		ModelRPMLimit:       oldKey.ModelRPMLimit,
		Metadata:            copyMetadata(oldKey.Metadata),
		IsActive:            true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	// Update rotation metadata
	rotationCount := 0
	if v, ok := oldKey.Metadata["rotation_count"].(float64); ok {
		rotationCount = int(v)
	}
	newKey.Metadata["rotation_count"] = rotationCount + 1
	newKey.Metadata["last_rotation_at"] = now.Format(time.RFC3339)

	// Calculate next rotation time
	if interval, ok := oldKey.Metadata["rotation_interval"].(string); ok {
		rotationAt := CalculateRotationTime(interval)
		if rotationAt != nil {
			newKey.Metadata["key_rotation_at"] = rotationAt.Format(time.RFC3339)
		}
	}

	// Deactivate old key (soft delete)
	if err := j.store.DeleteAPIKey(ctx, oldKey.ID); err != nil {
		return err
	}

	// Create new key
	if err := j.store.CreateAPIKey(ctx, newKey); err != nil {
		return err
	}

	return nil
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

func copyMetadata(m Metadata) Metadata {
	if m == nil {
		return make(Metadata)
	}
	result := make(Metadata)
	for k, v := range m {
		result[k] = v
	}
	return result
}
