// Package auth provides API key authentication and multi-tenant support.
package auth

import (
	"context"
	"log/slog"
	"time"
)

// UserTeamSyncConfig contains configuration for user-team synchronization.
type UserTeamSyncConfig struct {
	// Enable automatic sync on SSO login
	Enabled bool `json:"enabled"`

	// Create users if they don't exist
	AutoCreateUsers bool `json:"auto_create_users"`

	// Create teams if they don't exist
	AutoCreateTeams bool `json:"auto_create_teams"`

	// Remove user from teams not in JWT
	RemoveFromUnlistedTeams bool `json:"remove_from_unlisted_teams"`

	// Sync user role from JWT
	SyncUserRole bool `json:"sync_user_role"`

	// Default role for new users
	DefaultRole string `json:"default_role"`

	// Default organization for new users
	DefaultOrganizationID string `json:"default_organization_id"`
}

// UserTeamSyncer handles synchronization of user roles and team memberships
// from JWT claims. This aligns with LiteLLM's sync_user_role_and_teams feature.
type UserTeamSyncer struct {
	store  Store
	config UserTeamSyncConfig
	logger *slog.Logger
}

// NewUserTeamSyncer creates a new user-team syncer.
func NewUserTeamSyncer(store Store, config UserTeamSyncConfig, logger *slog.Logger) *UserTeamSyncer {
	if logger == nil {
		logger = slog.Default()
	}
	return &UserTeamSyncer{
		store:  store,
		config: config,
		logger: logger,
	}
}

// SyncRequest contains the data needed to sync a user's teams and role.
type SyncRequest struct {
	// User identification
	UserID    string  // SSO user ID (subject)
	Email     *string // User email
	SSOUserID string  // SSO provider's user ID

	// Role information
	Role     string   // Resolved role from JWT
	JWTRoles []string // Raw roles from JWT

	// Team information
	TeamIDs []string // Team IDs from JWT

	// Organization information
	OrganizationID *string // Organization ID from JWT

	// Additional metadata
	Metadata map[string]any
}

// SyncResult contains the result of a sync operation.
type SyncResult struct {
	// User operations
	UserCreated bool   `json:"user_created"`
	UserUpdated bool   `json:"user_updated"`
	UserID      string `json:"user_id"`

	// Team operations
	TeamsAdded   []string `json:"teams_added"`
	TeamsRemoved []string `json:"teams_removed"`
	TeamsCreated []string `json:"teams_created"`

	// Role operations
	RoleUpdated bool   `json:"role_updated"`
	OldRole     string `json:"old_role,omitempty"`
	NewRole     string `json:"new_role,omitempty"`

	// Errors (non-fatal)
	Warnings []string `json:"warnings,omitempty"`
}

// SyncUserTeams synchronizes a user's team memberships and role based on JWT claims.
func (s *UserTeamSyncer) SyncUserTeams(ctx context.Context, req *SyncRequest) (*SyncResult, error) {
	if !s.config.Enabled {
		return &SyncResult{}, nil
	}

	result := &SyncResult{
		TeamsAdded:   make([]string, 0),
		TeamsRemoved: make([]string, 0),
		TeamsCreated: make([]string, 0),
		Warnings:     make([]string, 0),
	}

	// Step 1: Ensure user exists
	user, err := s.ensureUser(ctx, req, result)
	if err != nil {
		return result, err
	}
	result.UserID = user.ID

	// Step 2: Sync user role
	if s.config.SyncUserRole && req.Role != "" {
		s.syncUserRole(ctx, user, req.Role, result)
	}

	// Step 3: Sync team memberships
	if len(req.TeamIDs) > 0 {
		s.syncTeamMemberships(ctx, user, req.TeamIDs, result)
	}

	// Step 4: Sync organization membership
	if req.OrganizationID != nil && *req.OrganizationID != "" {
		s.syncOrganizationMembership(ctx, user, *req.OrganizationID, result)
	}

	return result, nil
}

// ensureUser ensures the user exists, creating if necessary.
func (s *UserTeamSyncer) ensureUser(ctx context.Context, req *SyncRequest, result *SyncResult) (*User, error) {
	// Try to find existing user
	user, err := s.store.GetUser(ctx, req.UserID)
	if err != nil {
		return nil, err
	}

	if user != nil {
		return user, nil
	}

	// User doesn't exist - create if enabled
	if !s.config.AutoCreateUsers {
		return nil, ErrUserNotFound
	}

	now := time.Now()
	user = &User{
		ID:        req.UserID,
		Email:     req.Email,
		Role:      s.getDefaultRole(req.Role),
		IsActive:  true,
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	if req.OrganizationID != nil {
		user.OrganizationID = req.OrganizationID
	} else if s.config.DefaultOrganizationID != "" {
		user.OrganizationID = &s.config.DefaultOrganizationID
	}

	if err := s.store.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	result.UserCreated = true
	s.logger.Info("created user from SSO",
		"user_id", user.ID,
		"email", user.Email,
		"role", user.Role,
	)

	return user, nil
}

// getDefaultRole returns the appropriate default role.
func (s *UserTeamSyncer) getDefaultRole(jwtRole string) string {
	if jwtRole != "" {
		return jwtRole
	}
	if s.config.DefaultRole != "" {
		return s.config.DefaultRole
	}
	return string(UserRoleInternalUser)
}

// syncUserRole updates the user's role if it has changed.
func (s *UserTeamSyncer) syncUserRole(ctx context.Context, user *User, newRole string, result *SyncResult) {
	if user.Role == newRole {
		return
	}

	oldRole := user.Role
	user.Role = newRole
	now := time.Now()
	user.UpdatedAt = &now

	if err := s.store.UpdateUser(ctx, user); err != nil {
		s.logger.Warn("failed to update user role",
			"user_id", user.ID,
			"error", err,
		)
		result.Warnings = append(result.Warnings, "failed to update role: "+err.Error())
		return
	}

	result.RoleUpdated = true
	result.OldRole = oldRole
	result.NewRole = newRole
	result.UserUpdated = true

	s.logger.Info("updated user role from SSO",
		"user_id", user.ID,
		"old_role", oldRole,
		"new_role", newRole,
	)
}

// syncTeamMemberships synchronizes the user's team memberships.
func (s *UserTeamSyncer) syncTeamMemberships(ctx context.Context, user *User, jwtTeamIDs []string, result *SyncResult) {
	// Get current team memberships
	currentMemberships, err := s.store.ListUserTeamMemberships(ctx, user.ID)
	if err != nil {
		s.logger.Warn("failed to get current team memberships",
			"user_id", user.ID,
			"error", err,
		)
		result.Warnings = append(result.Warnings, "failed to get current memberships: "+err.Error())
		return
	}

	// Build set of current team IDs
	currentTeamIDs := make(map[string]bool)
	for _, m := range currentMemberships {
		currentTeamIDs[m.TeamID] = true
	}

	// Build set of JWT team IDs
	jwtTeamSet := make(map[string]bool)
	for _, id := range jwtTeamIDs {
		jwtTeamSet[id] = true
	}

	// Add user to teams in JWT but not in current memberships
	for _, teamID := range jwtTeamIDs {
		if currentTeamIDs[teamID] {
			continue // Already a member
		}

		// Ensure team exists
		if err := s.ensureTeam(ctx, teamID, result); err != nil {
			result.Warnings = append(result.Warnings, "failed to ensure team "+teamID+": "+err.Error())
			continue
		}

		// Create membership
		now := time.Now()
		membership := &TeamMembership{
			UserID:   user.ID,
			TeamID:   teamID,
			Role:     "member",
			JoinedAt: &now,
		}

		if err := s.store.CreateTeamMembership(ctx, membership); err != nil {
			s.logger.Warn("failed to add user to team",
				"user_id", user.ID,
				"team_id", teamID,
				"error", err,
			)
			result.Warnings = append(result.Warnings, "failed to add to team "+teamID+": "+err.Error())
			continue
		}

		result.TeamsAdded = append(result.TeamsAdded, teamID)
		s.logger.Info("added user to team from SSO",
			"user_id", user.ID,
			"team_id", teamID,
		)
	}

	// Remove user from teams not in JWT (if configured)
	if s.config.RemoveFromUnlistedTeams {
		for teamID := range currentTeamIDs {
			if jwtTeamSet[teamID] {
				continue // Still in JWT
			}

			if err := s.store.DeleteTeamMembership(ctx, user.ID, teamID); err != nil {
				s.logger.Warn("failed to remove user from team",
					"user_id", user.ID,
					"team_id", teamID,
					"error", err,
				)
				result.Warnings = append(result.Warnings, "failed to remove from team "+teamID+": "+err.Error())
				continue
			}

			result.TeamsRemoved = append(result.TeamsRemoved, teamID)
			s.logger.Info("removed user from team (not in JWT)",
				"user_id", user.ID,
				"team_id", teamID,
			)
		}
	}
}

// ensureTeam ensures a team exists, creating if necessary.
func (s *UserTeamSyncer) ensureTeam(ctx context.Context, teamID string, result *SyncResult) error {
	team, err := s.store.GetTeam(ctx, teamID)
	if err != nil {
		return err
	}

	if team != nil {
		return nil
	}

	if !s.config.AutoCreateTeams {
		return ErrTeamNotFound
	}

	now := time.Now()
	team = &Team{
		ID:        teamID,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.store.CreateTeam(ctx, team); err != nil {
		return err
	}

	result.TeamsCreated = append(result.TeamsCreated, teamID)
	s.logger.Info("created team from SSO", "team_id", teamID)

	return nil
}

// syncOrganizationMembership syncs the user's organization membership.
func (s *UserTeamSyncer) syncOrganizationMembership(ctx context.Context, user *User, orgID string, result *SyncResult) {
	// Check if already a member
	membership, err := s.store.GetOrganizationMembership(ctx, user.ID, orgID)
	if err != nil {
		s.logger.Warn("failed to check organization membership",
			"user_id", user.ID,
			"org_id", orgID,
			"error", err,
		)
		return
	}

	if membership != nil {
		return // Already a member
	}

	// Create membership
	now := time.Now()
	newMembership := &OrganizationMembership{
		UserID:         user.ID,
		OrganizationID: orgID,
		UserRole:       "member",
		JoinedAt:       &now,
	}

	if err := s.store.CreateOrganizationMembership(ctx, newMembership); err != nil {
		s.logger.Warn("failed to add user to organization",
			"user_id", user.ID,
			"org_id", orgID,
			"error", err,
		)
		result.Warnings = append(result.Warnings, "failed to add to organization: "+err.Error())
		return
	}

	s.logger.Info("added user to organization from SSO",
		"user_id", user.ID,
		"org_id", orgID,
	)
}

// Error definitions
var (
	ErrUserNotFound = &SyncError{Code: "USER_NOT_FOUND", Message: "user not found and auto-create is disabled"}
	ErrTeamNotFound = &SyncError{Code: "TEAM_NOT_FOUND", Message: "team not found and auto-create is disabled"}
)

// SyncError represents a sync operation error.
type SyncError struct {
	Code    string
	Message string
}

func (e *SyncError) Error() string {
	return e.Message
}
