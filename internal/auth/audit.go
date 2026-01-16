// Package auth provides API key authentication and multi-tenant support.
package auth

import (
	"time"

	"github.com/google/uuid"
)

// AuditAction represents the type of action being audited.
type AuditAction string

const (
	// Entity CRUD actions
	AuditActionCreate AuditAction = "create"
	AuditActionUpdate AuditAction = "update"
	AuditActionDelete AuditAction = "delete"
	AuditActionRead   AuditAction = "read"

	// Authentication actions
	AuditActionLogin        AuditAction = "login"
	AuditActionLogout       AuditAction = "logout"
	AuditActionLoginFailed  AuditAction = "login_failed"
	AuditActionTokenRefresh AuditAction = "token_refresh"

	// API Key actions
	AuditActionAPIKeyCreate  AuditAction = "api_key_create" // #nosec G101 -- audit action name, not a credential.
	AuditActionAPIKeyRevoke  AuditAction = "api_key_revoke" // #nosec G101 -- audit action name, not a credential.
	AuditActionAPIKeyBlock   AuditAction = "api_key_block" // #nosec G101 -- audit action name, not a credential.
	AuditActionAPIKeyUnblock AuditAction = "api_key_unblock" // #nosec G101 -- audit action name, not a credential.

	// Team actions
	AuditActionTeamCreate       AuditAction = "team_create"
	AuditActionTeamUpdate       AuditAction = "team_update"
	AuditActionTeamDelete       AuditAction = "team_delete"
	AuditActionTeamMemberAdd    AuditAction = "team_member_add"
	AuditActionTeamMemberRemove AuditAction = "team_member_remove"
	AuditActionTeamBlock        AuditAction = "team_block"

	// Organization actions
	AuditActionOrgCreate       AuditAction = "org_create"
	AuditActionOrgUpdate       AuditAction = "org_update"
	AuditActionOrgDelete       AuditAction = "org_delete"
	AuditActionOrgMemberAdd    AuditAction = "org_member_add"
	AuditActionOrgMemberRemove AuditAction = "org_member_remove"

	// User actions
	AuditActionUserCreate     AuditAction = "user_create"
	AuditActionUserUpdate     AuditAction = "user_update"
	AuditActionUserDelete     AuditAction = "user_delete"
	AuditActionUserRoleChange AuditAction = "user_role_change"

	// Budget actions
	AuditActionBudgetExceeded AuditAction = "budget_exceeded"
	AuditActionBudgetReset    AuditAction = "budget_reset"
	AuditActionBudgetUpdate   AuditAction = "budget_update"

	// Configuration actions
	AuditActionConfigUpdate AuditAction = "config_update"
	AuditActionSSOUpdate    AuditAction = "sso_update"
)

// AuditObjectType represents the type of object being audited.
type AuditObjectType string

const (
	AuditObjectAPIKey       AuditObjectType = "api_key"
	AuditObjectTeam         AuditObjectType = "team"
	AuditObjectOrganization AuditObjectType = "organization"
	AuditObjectUser         AuditObjectType = "user"
	AuditObjectEndUser      AuditObjectType = "end_user"
	AuditObjectBudget       AuditObjectType = "budget"
	AuditObjectConfig       AuditObjectType = "config"
	AuditObjectSSO          AuditObjectType = "sso"
	AuditObjectModel        AuditObjectType = "model"
	AuditObjectMembership   AuditObjectType = "membership"
)

// AuditLog represents an audit log entry for compliance and security tracking.
// Aligned with LiteLLM's LiteLLM_AuditLog table.
type AuditLog struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`

	// Who performed the action
	ActorID    string `json:"actor_id"`              // User ID or API Key ID
	ActorType  string `json:"actor_type"`            // "user", "api_key", "system"
	ActorEmail string `json:"actor_email,omitempty"` // For user actors
	ActorIP    string `json:"actor_ip,omitempty"`    // Client IP address

	// What action was performed
	Action AuditAction `json:"action"`

	// What object was affected
	ObjectType AuditObjectType `json:"object_type"`
	ObjectID   string          `json:"object_id"`

	// Context
	TeamID         *string `json:"team_id,omitempty"`
	OrganizationID *string `json:"organization_id,omitempty"`

	// Changes
	BeforeValue map[string]any `json:"before_value,omitempty"` // State before change
	AfterValue  map[string]any `json:"after_value,omitempty"`  // State after change
	Diff        map[string]any `json:"diff,omitempty"`         // Only changed fields

	// Request context
	RequestID  string `json:"request_id,omitempty"`
	UserAgent  string `json:"user_agent,omitempty"`
	RequestURI string `json:"request_uri,omitempty"`

	// Status
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`

	// Metadata
	Metadata Metadata `json:"metadata,omitempty"`
}

// AuditLogFilter contains filter options for querying audit logs.
type AuditLogFilter struct {
	ActorID        *string
	ActorType      *string
	Action         *AuditAction
	ObjectType     *AuditObjectType
	ObjectID       *string
	TeamID         *string
	OrganizationID *string
	StartTime      time.Time
	EndTime        time.Time
	Success        *bool
	Limit          int
	Offset         int
}

// AuditLogStats contains aggregated audit statistics.
type AuditLogStats struct {
	TotalEvents      int64            `json:"total_events"`
	SuccessCount     int64            `json:"success_count"`
	FailureCount     int64            `json:"failure_count"`
	UniqueActors     int              `json:"unique_actors"`
	ActionCounts     map[string]int64 `json:"action_counts"`
	ObjectTypeCounts map[string]int64 `json:"object_type_counts"`
}

// AuditLogStore defines the interface for audit log storage.
type AuditLogStore interface {
	// CreateAuditLog records a new audit log entry.
	CreateAuditLog(log *AuditLog) error

	// GetAuditLog retrieves a single audit log by ID.
	GetAuditLog(id string) (*AuditLog, error)

	// ListAuditLogs returns audit logs matching the filter.
	ListAuditLogs(filter AuditLogFilter) ([]*AuditLog, int64, error)

	// GetAuditLogStats returns aggregated audit statistics.
	GetAuditLogStats(filter AuditLogFilter) (*AuditLogStats, error)

	// DeleteAuditLogs deletes audit logs older than the specified time.
	// Used for log rotation/retention policies.
	DeleteAuditLogs(olderThan time.Time) (int64, error)
}

// AuditLogger provides a high-level API for recording audit events.
type AuditLogger struct {
	store   AuditLogStore
	enabled bool
}

// NewAuditLogger creates a new audit logger.
func NewAuditLogger(store AuditLogStore, enabled bool) *AuditLogger {
	return &AuditLogger{
		store:   store,
		enabled: enabled,
	}
}

// Log records an audit event.
func (al *AuditLogger) Log(log *AuditLog) error {
	if !al.enabled || al.store == nil {
		return nil
	}
	return al.store.CreateAuditLog(log)
}

// LogAction is a convenience method for logging common actions.
func (al *AuditLogger) LogAction(
	actorID, actorType string,
	action AuditAction,
	objectType AuditObjectType,
	objectID string,
	success bool,
	beforeValue, afterValue map[string]any,
) error {
	if !al.enabled {
		return nil
	}

	log := &AuditLog{
		ID:          generateAuditID(),
		Timestamp:   time.Now().UTC(),
		ActorID:     actorID,
		ActorType:   actorType,
		Action:      action,
		ObjectType:  objectType,
		ObjectID:    objectID,
		Success:     success,
		BeforeValue: beforeValue,
		AfterValue:  afterValue,
	}

	// Calculate diff if both before and after values are provided
	if beforeValue != nil && afterValue != nil {
		log.Diff = calculateDiff(beforeValue, afterValue)
	}

	return al.Log(log)
}

// LogAPIKeyAction logs an API key related action.
func (al *AuditLogger) LogAPIKeyAction(
	actorID, actorType string,
	action AuditAction,
	keyID string,
	success bool,
	metadata map[string]any,
) error {
	log := &AuditLog{
		ID:         generateAuditID(),
		Timestamp:  time.Now().UTC(),
		ActorID:    actorID,
		ActorType:  actorType,
		Action:     action,
		ObjectType: AuditObjectAPIKey,
		ObjectID:   keyID,
		Success:    success,
		Metadata:   metadata,
	}
	return al.Log(log)
}

// LogTeamAction logs a team related action.
func (al *AuditLogger) LogTeamAction(
	actorID, actorType string,
	action AuditAction,
	teamID string,
	orgID *string,
	success bool,
	beforeValue, afterValue map[string]any,
) error {
	log := &AuditLog{
		ID:             generateAuditID(),
		Timestamp:      time.Now().UTC(),
		ActorID:        actorID,
		ActorType:      actorType,
		Action:         action,
		ObjectType:     AuditObjectTeam,
		ObjectID:       teamID,
		OrganizationID: orgID,
		Success:        success,
		BeforeValue:    beforeValue,
		AfterValue:     afterValue,
	}
	return al.Log(log)
}

// LogUserAction logs a user related action.
func (al *AuditLogger) LogUserAction(
	actorID, actorType string,
	action AuditAction,
	userID string,
	teamID, orgID *string,
	success bool,
	beforeValue, afterValue map[string]any,
) error {
	log := &AuditLog{
		ID:             generateAuditID(),
		Timestamp:      time.Now().UTC(),
		ActorID:        actorID,
		ActorType:      actorType,
		Action:         action,
		ObjectType:     AuditObjectUser,
		ObjectID:       userID,
		TeamID:         teamID,
		OrganizationID: orgID,
		Success:        success,
		BeforeValue:    beforeValue,
		AfterValue:     afterValue,
	}
	return al.Log(log)
}

// LogLoginAttempt logs an authentication attempt.
func (al *AuditLogger) LogLoginAttempt(
	userID, email, ip, userAgent string,
	success bool,
	errorMsg string,
) error {
	action := AuditActionLogin
	if !success {
		action = AuditActionLoginFailed
	}

	log := &AuditLog{
		ID:         generateAuditID(),
		Timestamp:  time.Now().UTC(),
		ActorID:    userID,
		ActorType:  "user",
		ActorEmail: email,
		ActorIP:    ip,
		Action:     action,
		ObjectType: AuditObjectUser,
		ObjectID:   userID,
		Success:    success,
		Error:      errorMsg,
		UserAgent:  userAgent,
	}
	return al.Log(log)
}

// generateAuditID generates a unique ID for audit logs.
func generateAuditID() string {
	return uuid.New().String()
}

// calculateDiff computes the difference between two maps.
func calculateDiff(before, after map[string]any) map[string]any {
	diff := make(map[string]any)

	// Find changed and added fields
	for k, afterVal := range after {
		if beforeVal, exists := before[k]; exists {
			if beforeVal != afterVal {
				diff[k] = map[string]any{
					"before": beforeVal,
					"after":  afterVal,
				}
			}
		} else {
			diff[k] = map[string]any{
				"before": nil,
				"after":  afterVal,
			}
		}
	}

	// Find removed fields
	for k, beforeVal := range before {
		if _, exists := after[k]; !exists {
			diff[k] = map[string]any{
				"before": beforeVal,
				"after":  nil,
			}
		}
	}

	return diff
}
