package auth

import (
	"fmt"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
)

const casbinModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && (r.obj == p.obj || p.obj == "*" || keyMatch(r.obj, p.obj)) && (r.act == p.act || p.act == "*")
`

// CasbinEnforcer wraps Casbin enforcer for llmux.
type CasbinEnforcer struct {
	enforcer *casbin.Enforcer
}

// NewCasbinEnforcer creates a new CasbinEnforcer.
func NewCasbinEnforcer(adapter persist.Adapter) (*CasbinEnforcer, error) {
	m, err := model.NewModelFromString(casbinModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin model: %w", err)
	}

	var e *casbin.Enforcer
	if adapter != nil {
		e, err = casbin.NewEnforcer(m, adapter)
	} else {
		e, err = casbin.NewEnforcer(m)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create casbin enforcer: %w", err)
	}

	// Add default roles if any
	return &CasbinEnforcer{enforcer: e}, nil
}

// NewFileCasbinEnforcer creates a CasbinEnforcer from a policy file.
func NewFileCasbinEnforcer(policyPath string) (*CasbinEnforcer, error) {
	m, err := model.NewModelFromString(casbinModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin model: %w", err)
	}

	e, err := casbin.NewEnforcer(m, policyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin enforcer: %w", err)
	}

	return &CasbinEnforcer{enforcer: e}, nil
}

// Enforce checks if a subject has permission to access an object with an action.
func (ce *CasbinEnforcer) Enforce(sub, obj, act string) (bool, error) {
	return ce.enforcer.Enforce(sub, obj, act)
}

// AddPolicy adds a policy to the enforcer.
func (ce *CasbinEnforcer) AddPolicy(sub, obj, act string) (bool, error) {
	return ce.enforcer.AddPolicy(sub, obj, act)
}

// AddRoleForUser adds a role for a user.
func (ce *CasbinEnforcer) AddRoleForUser(user, role string) (bool, error) {
	return ce.enforcer.AddGroupingPolicy(user, role)
}

// AddGroupingPolicy adds a grouping policy to the enforcer.
func (ce *CasbinEnforcer) AddGroupingPolicy(sub, group string) (bool, error) {
	return ce.enforcer.AddGroupingPolicy(sub, group)
}

// LoadPolicy reloads the policy from storage.
func (ce *CasbinEnforcer) LoadPolicy() error {
	return ce.enforcer.LoadPolicy()
}

// AddDefaultPolicies adds basic llmux policies.
func (ce *CasbinEnforcer) AddDefaultPolicies() error {
	// Proxy Admin: can do everything
	_, _ = ce.enforcer.AddPolicy(RoleSub(string(UserRoleProxyAdmin)), "*", "*")

	// Proxy Admin Viewer: can read everything
	_, _ = ce.enforcer.AddPolicy(RoleSub(string(UserRoleProxyAdminViewer)), "*", "GET")
	_, _ = ce.enforcer.AddPolicy(RoleSub(string(UserRoleProxyAdminViewer)), "*", "HEAD")

	// Management Key: can do everything (usually for administrative tasks)
	_, _ = ce.enforcer.AddPolicy(RoleSub(string(KeyTypeManagement)), "*", "*")

	// ReadOnly Key: can only list models
	_, _ = ce.enforcer.AddPolicy(RoleSub(string(KeyTypeReadOnly)), "/v1/models", "GET")

	// LLM API Key: can call model endpoints
	_, _ = ce.enforcer.AddPolicy(RoleSub(string(KeyTypeLLMAPI)), "/v1/chat/completions", "POST")
	_, _ = ce.enforcer.AddPolicy(RoleSub(string(KeyTypeLLMAPI)), "/v1/completions", "POST")
	_, _ = ce.enforcer.AddPolicy(RoleSub(string(KeyTypeLLMAPI)), "/v1/embeddings", "POST")
	_, _ = ce.enforcer.AddPolicy(RoleSub(string(KeyTypeLLMAPI)), "/embeddings", "POST")

	return nil
}

// Helper methods to format subjects and objects

func KeySub(keyID string) string   { return "key:" + keyID }
func TeamSub(teamID string) string { return "team:" + teamID }
func UserSub(userID string) string { return "user:" + userID }
func OrgSub(orgID string) string   { return "org:" + orgID }
func RoleSub(role string) string   { return "role:" + role }

func ModelObj(modelName string) string { return "model:" + modelName }
func PathObj(path string) string       { return path }

func ActionUse() string                 { return "use" }
func ActionMethod(method string) string { return strings.ToUpper(method) }
