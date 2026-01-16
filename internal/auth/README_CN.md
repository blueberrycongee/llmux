# Auth 模块 (鉴权与授权)

此包为 LLMux 提供身份验证与授权功能。

## Casbin RBAC (基于角色的访问控制)

LLMux 集成了 [Casbin](https://casbin.org/)，以提供灵活的角色访问控制 (RBAC) 和基于属性的访问控制 (ABAC)。

### 模型定义 (Model Definition)

LLMux 中使用的 Casbin 模型定义如下：

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

### 主体 (sub)

- `key:<key_id>`: 代表一个 API key。
- `user:<user_id>`: 代表内部用户。
- `team:<team_id>`: 代表团队。
- `org:<org_id>`: 代表组织。
- `role:<role_name>`: 代表角色 (例如 `role:proxy_admin`, `role:read_only`)。

### 对象 (obj)

- URL 路径: 例如 `/v1/chat/completions`, `/v1/models`。
- 模型资源: 例如 `model:gpt-4`, `model:claude-3-opus`。
- 通配符: `*` 匹配所有内容。

### 动作 (act)

- HTTP 方法: `GET`, `POST`, `PUT`, `DELETE` 等。
- LLM 使用: `use`。
- 通配符: `*` 匹配所有动作。

### 默认策略 (Default Policies)

LLMux 为标准角色预置了以下策略：

| 主体 | 对象 | 动作 | 描述 |
|---------|--------|--------|-------------|
| `role:proxy_admin` | `*` | `*` | 完全访问权限 |
| `role:proxy_admin_viewer` | `*` | `GET`, `HEAD` | 对所有资源的只读访问权限 |
| `role:management` | `*` | `*` | 完全访问权限 (管理 Key) |
| `role:read_only` | `/v1/models` | `GET` | 仅能列出模型 |
| `role:llm_api` | `/v1/chat/completions` | `POST` | 可调用对话补全 |
| `role:llm_api` | `/v1/completions` | `POST` | 可调用文本补全 |
| `role:llm_api` | `/v1/embeddings` | `POST` | 可调用向量化接口 |

### 配置

您可以在 LLMux 配置文件中启用并配置 Casbin：

```yaml
auth:
  enabled: true
  casbin:
    enabled: true
    # 可选：CSV 策略文件的路径
    # policy_path: /path/to/policy.csv
```

如果未提供 `policy_path`，LLMux 将使用带默认策略的内存 Enforcer。

### 动态策略管理

LLMux 支持动态策略管理。您可以在运行时添加策略，或使用 Casbin 适配器从数据库加载策略。

对于模型访问控制，如果启用了 Casbin，LLMux 会自动将 API Key 的 `allowed_models` 字段映射为 Casbin 策略。
