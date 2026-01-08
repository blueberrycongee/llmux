// Package auth provides API key authentication and multi-tenant support.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"
)

// InvitationLink represents an invitation link for users to join a team or organization.
// This aligns with LiteLLM's LiteLLM_InvitationLink table.
type InvitationLink struct {
	ID        string `json:"id"`
	Token     string `json:"token"` // Unique invitation token (used in URL)
	TokenHash string `json:"-"`     // Hash of token for secure storage

	// Target entity
	TeamID         *string `json:"team_id,omitempty"`
	OrganizationID *string `json:"organization_id,omitempty"`

	// Permissions
	Role string `json:"role,omitempty"` // Role to assign to invited user

	// Limits
	MaxUses     int      `json:"max_uses,omitempty"`   // 0 = unlimited
	CurrentUses int      `json:"current_uses"`         // Number of times used
	MaxBudget   *float64 `json:"max_budget,omitempty"` // Budget for invited user

	// Validity
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	IsActive  bool       `json:"is_active"`

	// Creator info
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Metadata
	Description string   `json:"description,omitempty"`
	Metadata    Metadata `json:"metadata,omitempty"`
}

// IsValid checks if the invitation link is still valid.
func (i *InvitationLink) IsValid() bool {
	if !i.IsActive {
		return false
	}

	if i.ExpiresAt != nil && time.Now().After(*i.ExpiresAt) {
		return false
	}

	if i.MaxUses > 0 && i.CurrentUses >= i.MaxUses {
		return false
	}

	return true
}

// InvitationLinkStore defines the interface for invitation link storage.
type InvitationLinkStore interface {
	// CreateInvitationLink creates a new invitation link.
	CreateInvitationLink(ctx context.Context, link *InvitationLink) error

	// GetInvitationLink retrieves an invitation link by ID.
	GetInvitationLink(ctx context.Context, id string) (*InvitationLink, error)

	// GetInvitationLinkByToken retrieves an invitation link by token.
	GetInvitationLinkByToken(ctx context.Context, token string) (*InvitationLink, error)

	// UpdateInvitationLink updates an invitation link.
	UpdateInvitationLink(ctx context.Context, link *InvitationLink) error

	// DeleteInvitationLink deletes an invitation link.
	DeleteInvitationLink(ctx context.Context, id string) error

	// ListInvitationLinks lists invitation links with optional filters.
	ListInvitationLinks(ctx context.Context, filter InvitationLinkFilter) ([]*InvitationLink, error)

	// IncrementInvitationLinkUses increments the use count of an invitation link.
	IncrementInvitationLinkUses(ctx context.Context, id string) error
}

// InvitationLinkFilter contains filter options for listing invitation links.
type InvitationLinkFilter struct {
	TeamID         *string
	OrganizationID *string
	CreatedBy      *string
	IsActive       *bool
	Limit          int
	Offset         int
}

// InvitationService handles invitation link operations.
type InvitationService struct {
	store     InvitationLinkStore
	authStore Store
	logger    *slog.Logger
}

// NewInvitationService creates a new invitation service.
func NewInvitationService(store InvitationLinkStore, authStore Store, logger *slog.Logger) *InvitationService {
	return &InvitationService{
		store:     store,
		authStore: authStore,
		logger:    logger,
	}
}

// CreateInvitationRequest contains parameters for creating an invitation link.
type CreateInvitationRequest struct {
	TeamID         *string  `json:"team_id,omitempty"`
	OrganizationID *string  `json:"organization_id,omitempty"`
	Role           string   `json:"role,omitempty"`
	MaxUses        int      `json:"max_uses,omitempty"`
	MaxBudget      *float64 `json:"max_budget,omitempty"`
	ExpiresIn      int      `json:"expires_in,omitempty"` // Hours until expiration
	Description    string   `json:"description,omitempty"`
	CreatedBy      string   `json:"created_by"`
}

// CreateInvitationLink creates a new invitation link.
func (s *InvitationService) CreateInvitationLink(ctx context.Context, req *CreateInvitationRequest) (*InvitationLink, string, error) {
	// Generate secure token
	token, err := generateInvitationToken()
	if err != nil {
		return nil, "", err
	}

	now := time.Now()
	link := &InvitationLink{
		ID:             GenerateUUID(),
		Token:          hashInvitationToken(token),
		TeamID:         req.TeamID,
		OrganizationID: req.OrganizationID,
		Role:           req.Role,
		MaxUses:        req.MaxUses,
		MaxBudget:      req.MaxBudget,
		IsActive:       true,
		CreatedBy:      req.CreatedBy,
		CreatedAt:      now,
		UpdatedAt:      now,
		Description:    req.Description,
	}

	if req.ExpiresIn > 0 {
		expiresAt := now.Add(time.Duration(req.ExpiresIn) * time.Hour)
		link.ExpiresAt = &expiresAt
	}

	if err := s.store.CreateInvitationLink(ctx, link); err != nil {
		return nil, "", err
	}

	return link, token, nil
}

// AcceptInvitationRequest contains parameters for accepting an invitation.
type AcceptInvitationRequest struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
	Email  string `json:"email,omitempty"`
}

// AcceptInvitationResult contains the result of accepting an invitation.
type AcceptInvitationResult struct {
	Success        bool    `json:"success"`
	TeamID         *string `json:"team_id,omitempty"`
	OrganizationID *string `json:"organization_id,omitempty"`
	Role           string  `json:"role,omitempty"`
	Message        string  `json:"message,omitempty"`
}

// AcceptInvitation processes an invitation acceptance.
func (s *InvitationService) AcceptInvitation(ctx context.Context, req *AcceptInvitationRequest) (*AcceptInvitationResult, error) {
	// Find invitation by token
	tokenHash := hashInvitationToken(req.Token)
	link, err := s.store.GetInvitationLinkByToken(ctx, tokenHash)
	if err != nil {
		return nil, err
	}

	if link == nil {
		return &AcceptInvitationResult{
			Success: false,
			Message: "invitation not found",
		}, nil
	}

	// Validate invitation
	if !link.IsValid() {
		return &AcceptInvitationResult{
			Success: false,
			Message: "invitation is expired or has reached maximum uses",
		}, nil
	}

	result := &AcceptInvitationResult{
		Success:        true,
		TeamID:         link.TeamID,
		OrganizationID: link.OrganizationID,
		Role:           link.Role,
	}

	// Add user to team
	if link.TeamID != nil {
		now := time.Now()
		membership := &TeamMembership{
			UserID:   req.UserID,
			TeamID:   *link.TeamID,
			Role:     link.Role,
			JoinedAt: &now,
		}

		// Check for existing membership
		existing, _ := s.authStore.GetTeamMembership(ctx, req.UserID, *link.TeamID)
		if existing == nil {
			if err := s.authStore.CreateTeamMembership(ctx, membership); err != nil {
				return nil, err
			}
		}
	}

	// Add user to organization
	if link.OrganizationID != nil {
		now := time.Now()
		membership := &OrganizationMembership{
			UserID:         req.UserID,
			OrganizationID: *link.OrganizationID,
			UserRole:       link.Role,
			JoinedAt:       &now,
		}

		// Check for existing membership
		existing, _ := s.authStore.GetOrganizationMembership(ctx, req.UserID, *link.OrganizationID)
		if existing == nil {
			if err := s.authStore.CreateOrganizationMembership(ctx, membership); err != nil {
				return nil, err
			}
		}
	}

	// Increment use count
	if err := s.store.IncrementInvitationLinkUses(ctx, link.ID); err != nil {
		// Log error but don't fail the invitation
		s.logger.Error("failed to increment invitation link uses", "error", err, "invitation_id", link.ID)
	}

	result.Message = "invitation accepted successfully"
	return result, nil
}

// DeactivateInvitation deactivates an invitation link.
func (s *InvitationService) DeactivateInvitation(ctx context.Context, id string) error {
	link, err := s.store.GetInvitationLink(ctx, id)
	if err != nil {
		return err
	}
	if link == nil {
		return nil
	}

	link.IsActive = false
	link.UpdatedAt = time.Now()

	return s.store.UpdateInvitationLink(ctx, link)
}

// ListInvitations lists invitation links for a team or organization.
func (s *InvitationService) ListInvitations(ctx context.Context, filter InvitationLinkFilter) ([]*InvitationLink, error) {
	return s.store.ListInvitationLinks(ctx, filter)
}

// generateInvitationToken generates a secure random token.
func generateInvitationToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// hashInvitationToken hashes an invitation token for secure storage.
func hashInvitationToken(token string) string {
	// Use the same hashing as API keys for consistency
	return HashKey(token)
}

// MemoryInvitationLinkStore implements InvitationLinkStore using in-memory storage.
type MemoryInvitationLinkStore struct {
	links map[string]*InvitationLink
	mu    sync.RWMutex
}

// NewMemoryInvitationLinkStore creates a new in-memory invitation link store.
func NewMemoryInvitationLinkStore() *MemoryInvitationLinkStore {
	return &MemoryInvitationLinkStore{
		links: make(map[string]*InvitationLink),
	}
}

// CreateInvitationLink creates a new invitation link.
func (s *MemoryInvitationLinkStore) CreateInvitationLink(_ context.Context, link *InvitationLink) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	linkCopy := *link
	s.links[link.ID] = &linkCopy
	return nil
}

// GetInvitationLink retrieves an invitation link by ID.
func (s *MemoryInvitationLinkStore) GetInvitationLink(_ context.Context, id string) (*InvitationLink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	link, ok := s.links[id]
	if !ok {
		return nil, nil
	}
	linkCopy := *link
	return &linkCopy, nil
}

// GetInvitationLinkByToken retrieves an invitation link by token.
func (s *MemoryInvitationLinkStore) GetInvitationLinkByToken(_ context.Context, tokenHash string) (*InvitationLink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, link := range s.links {
		if link.Token == tokenHash {
			linkCopy := *link
			return &linkCopy, nil
		}
	}
	return nil, nil
}

// UpdateInvitationLink updates an invitation link.
func (s *MemoryInvitationLinkStore) UpdateInvitationLink(_ context.Context, link *InvitationLink) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.links[link.ID]; !ok {
		return nil
	}
	linkCopy := *link
	s.links[link.ID] = &linkCopy
	return nil
}

// DeleteInvitationLink deletes an invitation link.
func (s *MemoryInvitationLinkStore) DeleteInvitationLink(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.links, id)
	return nil
}

// ListInvitationLinks lists invitation links with optional filters.
func (s *MemoryInvitationLinkStore) ListInvitationLinks(_ context.Context, filter InvitationLinkFilter) ([]*InvitationLink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var results []*InvitationLink

	for _, link := range s.links {
		// Apply filters
		if filter.TeamID != nil && (link.TeamID == nil || *link.TeamID != *filter.TeamID) {
			continue
		}
		if filter.OrganizationID != nil && (link.OrganizationID == nil || *link.OrganizationID != *filter.OrganizationID) {
			continue
		}
		if filter.CreatedBy != nil && link.CreatedBy != *filter.CreatedBy {
			continue
		}
		if filter.IsActive != nil && link.IsActive != *filter.IsActive {
			continue
		}

		linkCopy := *link
		results = append(results, &linkCopy)
	}

	// Apply pagination
	if filter.Offset > 0 && filter.Offset < len(results) {
		results = results[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results, nil
}

// IncrementInvitationLinkUses increments the use count of an invitation link.
func (s *MemoryInvitationLinkStore) IncrementInvitationLinkUses(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	link, ok := s.links[id]
	if !ok {
		return nil
	}
	link.CurrentUses++
	link.UpdatedAt = time.Now()
	return nil
}

// Ensure interface is satisfied
var _ InvitationLinkStore = (*MemoryInvitationLinkStore)(nil)
