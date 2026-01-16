# Auth Package

This package provides authentication and authorization for LLMux.

## Casbin RBAC

LLMux integrates [Casbin](https://casbin.org/) for flexible Role-Based Access Control (RBAC) and Attribute-Based Access Control (ABAC).

### Model Definition

The Casbin model used in LLMux is defined as follows:

```ini
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
```

### Subjects (sub)

- `key:<key_id>`: Represents an API key.
- `user:<user_id>`: Represents an internal user.
- `team:<team_id>`: Represents a team.
- `org:<org_id>`: Represents an organization.
- `role:<role_name>`: Represents a role (e.g., `role:proxy_admin`, `role:read_only`).

### Objects (obj)

- URL Paths: e.g., `/v1/chat/completions`, `/v1/models`.
- Model Resources: e.g., `model:gpt-4`, `model:claude-3-opus`.
- Wildcard: `*` matches everything.

### Actions (act)

- HTTP Methods: `GET`, `POST`, `PUT`, `DELETE`, etc.
- LLM Usage: `use`.
- Wildcard: `*` matches all actions.

### Default Policies

LLMux comes with default policies for standard roles:

| Subject | Object | Action | Description |
|---------|--------|--------|-------------|
| `role:proxy_admin` | `*` | `*` | Full access |
| `role:proxy_admin_viewer` | `*` | `GET`, `HEAD` | Read-only access to all resources |
| `role:management` | `*` | `*` | Full access (management keys) |
| `role:read_only` | `/v1/models` | `GET` | Can only list models |
| `role:llm_api` | `/v1/chat/completions` | `POST` | Can call chat completions |
| `role:llm_api` | `/v1/completions` | `POST` | Can call completions |
| `role:llm_api` | `/v1/embeddings` | `POST` | Can call embeddings |

### Configuration

You can enable and configure Casbin in your LLMux configuration file:

```yaml
auth:
  enabled: true
  casbin:
    enabled: true
    # Optional: Path to a CSV policy file
    # policy_path: /path/to/policy.csv
```

If `policy_path` is not provided, LLMux uses an in-memory enforcer with default policies.

### Dynamic Policy Management

LLMux supports dynamic policy management. You can add policies at runtime or load them from a database using a Casbin adapter.

For model access control, LLMux automatically maps the `allowed_models` field of API keys to Casbin policies if Casbin is enabled.
