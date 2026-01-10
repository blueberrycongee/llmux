package auth

import "context"

// ModelAccess evaluates model permissions across key/team/user/org scopes.
type ModelAccess struct {
	apiKey       *APIKey
	team         *Team
	user         *User
	organization *Organization
}

// NewModelAccess builds a model access evaluator from the auth context.
// It loads user/org details from the store when IDs are present.
func NewModelAccess(ctx context.Context, store Store, authCtx *AuthContext) (*ModelAccess, error) {
	if authCtx == nil {
		return nil, nil
	}

	access := &ModelAccess{
		apiKey: authCtx.APIKey,
		team:   authCtx.Team,
		user:   authCtx.User,
	}

	if access.user == nil && authCtx.APIKey != nil && authCtx.APIKey.UserID != nil && store != nil {
		user, err := store.GetUser(ctx, *authCtx.APIKey.UserID)
		if err != nil {
			return nil, err
		}
		access.user = user
	}

	orgID := ""
	if authCtx.APIKey != nil && authCtx.APIKey.OrganizationID != nil {
		orgID = *authCtx.APIKey.OrganizationID
	}
	if orgID == "" && access.user != nil && access.user.OrganizationID != nil {
		orgID = *access.user.OrganizationID
	}

	if orgID != "" && store != nil {
		org, err := store.GetOrganization(ctx, orgID)
		if err != nil {
			return nil, err
		}
		access.organization = org
	}

	return access, nil
}

// Allows returns true if all configured scopes allow the model.
func (a *ModelAccess) Allows(model string) bool {
	if a == nil || model == "" {
		return true
	}
	if a.apiKey != nil && !a.apiKey.CanAccessModel(model) {
		return false
	}
	if a.team != nil && !a.team.CanAccessModel(model) {
		return false
	}
	if a.user != nil && !a.user.CanAccessModel(model) {
		return false
	}
	if a.organization != nil && !a.organization.CanAccessModel(model) {
		return false
	}
	return true
}
