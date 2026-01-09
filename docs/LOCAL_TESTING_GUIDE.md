# LLMux 本地功能测试指南

本文档帮助您在本地环境中测试 LLMux 的所有核心功能。

## 📋 测试前准备

### 环境要求
- Go 1.23+
- Node.js 18+
- (可选) Docker Desktop
- (可选) 一个真实的 LLM API Key (OpenAI/Anthropic/等)

### 启动服务

**终端 1 - 后端 Gateway:**
```bash
cd llmux
./llmux.exe --config config/config.dev.yaml
# 或者带真实 API Key:
# OPENAI_API_KEY=sk-xxx ./llmux.exe --config config/config.yaml
```

**终端 2 - 前端 Dashboard:**
```bash
cd llmux/ui
npm run dev
```

**访问地址:**
- Gateway API: http://localhost:8080
- Dashboard: http://localhost:3000

---

## 🧪 测试清单

### 一、健康检查 (基础)

| #   | 测试项          | 命令/操作                                 | 预期结果                 |
| --- | --------------- | ----------------------------------------- | ------------------------ |
| 1.1 | 存活检查        | `curl http://localhost:8080/health/live`  | `{"status":"ok"}`        |
| 1.2 | 就绪检查        | `curl http://localhost:8080/health/ready` | `{"status":"ok"}`        |
| 1.3 | Prometheus 指标 | `curl http://localhost:8080/metrics`      | 返回 Prometheus 格式指标 |

---

### 二、API Key 管理

#### 2.1 通过 API 测试

```bash
# 生成 API Key
curl -X POST http://localhost:8080/key/generate \
  -H "Content-Type: application/json" \
  -d '{
    "key_name": "test-key-1",
    "max_budget": 100.0
  }'
# 预期: 返回 key_id 和 key (只显示一次)

# 列出所有 Key
curl http://localhost:8080/key/list
# 预期: 返回 key 列表

# 获取单个 Key 信息
curl "http://localhost:8080/key/info?key=<返回的key>"
# 预期: 返回 key 详情

# 封禁 Key
curl -X POST http://localhost:8080/key/block \
  -H "Content-Type: application/json" \
  -d '{"key_ids": ["<key_id>"]}'
# 预期: Key 被封禁

# 解封 Key
curl -X POST http://localhost:8080/key/unblock \
  -H "Content-Type: application/json" \
  -d '{"key_ids": ["<key_id>"]}'
# 预期: Key 恢复正常

# 删除 Key
curl -X POST http://localhost:8080/key/delete \
  -H "Content-Type: application/json" \
  -d '{"key_ids": ["<key_id>"]}'
# 预期: Key 被删除
```

#### 2.2 通过 Dashboard 测试

| #     | 测试项        | 操作步骤                            | 预期结果               |
| ----- | ------------- | ----------------------------------- | ---------------------- |
| 2.2.1 | 查看 Key 列表 | 点击左侧 "API Keys"                 | 显示 Key 列表页        |
| 2.2.2 | 创建 Key      | 点击 "Create Key" → 填写名称 → 确认 | Key 创建成功，显示密钥 |
| 2.2.3 | 复制 Key      | 点击复制按钮                        | 密钥复制到剪贴板       |
| 2.2.4 | 封禁 Key      | 点击 Key 行的 "Block"               | 状态变为 Blocked       |
| 2.2.5 | 解封 Key      | 点击 "Unblock"                      | 状态恢复 Active        |
| 2.2.6 | 删除 Key      | 点击 "Delete" → 确认                | Key 从列表消失         |
| 2.2.7 | 搜索 Key      | 在搜索框输入关键词                  | 列表实时过滤           |

---

### 三、用户管理

#### 3.1 通过 API 测试

```bash
# 创建用户
curl -X POST http://localhost:8080/user/new \
  -H "Content-Type: application/json" \
  -d '{
    "user_email": "alice@example.com",
    "user_alias": "Alice",
    "user_role": "internal_user",
    "max_budget": 50.0
  }'
# 预期: 返回 user_id

# 列出用户
curl "http://localhost:8080/user/list?limit=10"
# 预期: 返回用户列表

# 搜索用户
curl "http://localhost:8080/user/list?search=alice"
# 预期: 返回匹配用户

# 获取用户信息
curl "http://localhost:8080/user/info?user_id=<user_id>"
# 预期: 返回用户详情

# 更新用户
curl -X POST http://localhost:8080/user/update \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "<user_id>",
    "user_alias": "Alice Wang",
    "max_budget": 100.0
  }'
# 预期: 用户信息更新

# 删除用户
curl -X POST http://localhost:8080/user/delete \
  -H "Content-Type: application/json" \
  -d '{"user_ids": ["<user_id>"]}'
# 预期: 用户被删除
```

#### 3.2 通过 Dashboard 测试

| #     | 测试项       | 操作步骤                   | 预期结果       |
| ----- | ------------ | -------------------------- | -------------- |
| 3.2.1 | 查看用户列表 | 点击左侧 "Users"           | 显示用户列表   |
| 3.2.2 | 创建用户     | 点击 "Add User" → 填写信息 | 用户创建成功   |
| 3.2.3 | 编辑用户     | 点击 "Edit" → 修改 → 保存  | 信息更新成功   |
| 3.2.4 | 搜索用户     | 搜索框输入邮箱或名称       | 服务端过滤     |
| 3.2.5 | 分页         | 点击下一页                 | 加载下一批用户 |
| 3.2.6 | 删除用户     | 点击 "Delete" → 确认       | 用户从列表消失 |

---

### 四、团队管理

#### 4.1 通过 API 测试

```bash
# 创建团队
curl -X POST http://localhost:8080/team/new \
  -H "Content-Type: application/json" \
  -d '{
    "team_alias": "Frontend Team",
    "max_budget": 200.0
  }'
# 预期: 返回 team_id

# 列出团队
curl http://localhost:8080/team/list
# 预期: 返回团队列表

# 添加成员
curl -X POST http://localhost:8080/team/member_add \
  -H "Content-Type: application/json" \
  -d '{
    "team_id": "<team_id>",
    "member": [{"user_id": "<user_id>", "role": "member"}]
  }'
# 预期: 成员添加成功

# 获取团队成员
curl "http://localhost:8080/team/members?team_id=<team_id>"
# 预期: 返回成员列表

# 更新团队
curl -X POST http://localhost:8080/team/update \
  -H "Content-Type: application/json" \
  -d '{
    "team_id": "<team_id>",
    "team_alias": "Frontend Dev Team",
    "max_budget": 300.0
  }'
# 预期: 团队信息更新

# 删除团队
curl -X POST http://localhost:8080/team/delete \
  -H "Content-Type: application/json" \
  -d '{"team_ids": ["<team_id>"]}'
# 预期: 团队被删除
```

#### 4.2 通过 Dashboard 测试

| #     | 测试项       | 操作步骤                | 预期结果     |
| ----- | ------------ | ----------------------- | ------------ |
| 4.2.1 | 查看团队列表 | 点击左侧 "Teams"        | 显示团队列表 |
| 4.2.2 | 创建团队     | 点击 "Create Team"      | 团队创建成功 |
| 4.2.3 | 查看团队详情 | 点击团队卡片            | 显示成员列表 |
| 4.2.4 | 添加成员     | 详情页点击 "Add Member" | 成员添加成功 |
| 4.2.5 | 移除成员     | 点击成员旁 "Remove"     | 成员被移除   |

---

### 五、组织管理

#### 5.1 通过 API 测试

```bash
# 创建组织
curl -X POST http://localhost:8080/organization/new \
  -H "Content-Type: application/json" \
  -d '{
    "organization_alias": "Acme Corp",
    "max_budget": 1000.0
  }'
# 预期: 返回 organization_id

# 列出组织
curl http://localhost:8080/organization/list
# 预期: 返回组织列表

# 添加成员
curl -X POST http://localhost:8080/organization/member_add \
  -H "Content-Type: application/json" \
  -d '{
    "organization_id": "<org_id>",
    "members": [{"user_id": "<user_id>", "user_role": "org_member"}]
  }'
# 预期: 成员添加成功

# 获取组织成员
curl "http://localhost:8080/organization/members?organization_id=<org_id>"
# 预期: 返回成员列表
```

#### 5.2 通过 Dashboard 测试

| #     | 测试项       | 操作步骤                   | 预期结果     |
| ----- | ------------ | -------------------------- | ------------ |
| 5.2.1 | 查看组织列表 | 点击左侧 "Organizations"   | 显示组织列表 |
| 5.2.2 | 创建组织     | 点击 "Create Organization" | 组织创建成功 |
| 5.2.3 | 管理成员     | 点击组织 → 管理成员        | 可添加/移除  |

---

### 六、核心 Gateway 功能 (需要真实 API Key)

> ⚠️ 以下测试需要配置真实的 LLM 提供商 API Key

#### 6.1 基本请求

```bash
# 非流式请求
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer <your-llmux-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Say hello in 3 words"}],
    "stream": false
  }'
# 预期: 返回 AI 响应

# 流式请求
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer <your-llmux-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Count from 1 to 5"}],
    "stream": true
  }'
# 预期: 返回 SSE 流

# 列出可用模型
curl http://localhost:8080/v1/models
# 预期: 返回配置的模型列表
```

#### 6.2 预算/速率限制测试

```bash
# 1. 创建一个预算很小的 Key
curl -X POST http://localhost:8080/key/generate \
  -H "Content-Type: application/json" \
  -d '{"key_name": "budget-test", "max_budget": 0.001}'

# 2. 使用该 Key 发送请求直到超限
# 预期: 第 N 次请求返回 402 Payment Required
```

---

### 七、数据分析 API

```bash
# 消费日志
curl "http://localhost:8080/spend/logs?limit=10"
# 预期: 返回使用日志

# 按 Key 统计消费
curl http://localhost:8080/spend/keys
# 预期: 返回按 Key 的消费汇总

# 按团队统计消费
curl http://localhost:8080/spend/teams
# 预期: 返回按团队的消费汇总

# 全局活动指标
curl http://localhost:8080/global/activity
# 预期: 返回请求量、Token 用量等

# 按模型统计消费
curl http://localhost:8080/global/spend/models
# 预期: 返回按模型的消费分布
```

---

### 八、审计日志

```bash
# 获取审计日志
curl "http://localhost:8080/audit/logs?limit=20"
# 预期: 返回操作审计记录

# 按操作类型过滤
curl "http://localhost:8080/audit/logs?action=create"
# 预期: 返回创建类操作

# 按对象类型过滤
curl "http://localhost:8080/audit/logs?object_type=api_key"
# 预期: 返回 API Key 相关操作
```

---

### 九、Dashboard UI 测试

| #   | 页面     | 测试项        | 预期结果               |
| --- | -------- | ------------- | ---------------------- |
| 9.1 | Overview | 页面加载      | 显示图表骨架屏或数据   |
| 9.2 | Overview | 切换时间范围  | 图表数据更新           |
| 9.3 | Overview | 响应式布局    | 手机端正常显示         |
| 9.4 | API Keys | 创建/删除流程 | Toast 提示正确         |
| 9.5 | Users    | 搜索功能      | 服务端搜索生效         |
| 9.6 | Teams    | 成员管理      | 添加/移除正常          |
| 9.7 | 全局     | 暗色模式切换  | 主题正常切换           |
| 9.8 | 全局     | 错误处理      | 后端关闭时显示错误提示 |

---

## 🐳 Docker 快速测试 (可选)

如果想使用 Docker 启动完整环境：

```bash
# 构建镜像
docker build -t llmux .

# 运行 (使用开发配置)
docker run -p 8080:8080 \
  -v $(pwd)/config:/config \
  llmux --config /config/config.dev.yaml

# 或者带真实 API Key
docker run -p 8080:8080 \
  -e OPENAI_API_KEY=sk-xxx \
  -v $(pwd)/config:/config \
  llmux --config /config/config.yaml
```

---

## ✅ 测试完成检查清单

- [ ] 1. 健康检查通过
- [ ] 2. API Key 创建/封禁/解封/删除
- [ ] 3. 用户 CRUD + 搜索
- [ ] 4. 团队 CRUD + 成员管理
- [ ] 5. 组织 CRUD + 成员管理
- [ ] 6. (可选) LLM 请求转发
- [ ] 7. 数据统计 API
- [ ] 8. 审计日志
- [ ] 9. Dashboard 所有页面

---

## 📝 已知限制

1. **内存模式** - 重启后数据丢失，生产需启用 PostgreSQL
2. **无真实 LLM** - 使用 demo key 时，chat/completions 会失败
3. **统计数据** - 需要实际请求才能生成统计图表

---

## 🆘 常见问题

**Q: Dashboard 显示空白**
A: 检查后端是否运行在 8080 端口

**Q: API 返回 401**
A: 检查 config.yaml 中 `auth.enabled` 设置

**Q: 图表没有数据**
A: 需要发送实际请求才能生成统计数据
