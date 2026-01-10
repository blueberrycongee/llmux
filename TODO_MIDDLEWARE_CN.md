# 中间件与数据获取重构待办清单

本文档记录了需要将硬编码数据或逻辑替换为真实数据获取实现的位置。

## 1. 认证存储层 (Store) 连接
**位置:** `cmd/server/main.go`
**状态:** ⚠️ 硬编码为 `MemoryStore` (内存存储)
**任务:**
- [ ] 当 `cfg.Database.Enabled` 为 true 时，初始化 `PostgresStore`。
- [ ] 将持久化存储实例传递给 `auth.NewMiddleware` 及其他消费者。
- [ ] 确保正确的数据库连接管理（连接池、关闭操作）。

## 2. 模型访问控制中间件
**位置:** `internal/auth/middleware.go` -> `ModelAccessMiddleware`
**状态:** ❌ 空白 / 占位符
**任务:**
- [ ] 实现部分解析请求体 (Body) 的逻辑，以提取请求的模型名称。
- [ ] 查询 `Store`（或检查 `AuthContext`）以验证 API Key/用户是否有权访问所请求的模型。
- [ ] 如果拒绝访问，返回 `403 Forbidden`。

## 3. 列出模型端点 (List Models)
**位置:** `internal/api/handler.go` -> `ListModels`
**状态:** ❌ 硬编码的空列表
**任务:**
- [ ] 遍历 `h.registry` 中所有注册的提供商 (Provider)。
- [ ] 聚合所有提供商的可用模型。
- [ ] (可选) 根据已认证用户的权限过滤模型（如果集成了 `ModelAccessMiddleware` 逻辑）。
- [ ] 返回符合 OpenAI 兼容格式的真实模型列表。

## 4. 记忆系统集成 (Memory System Integration)
**位置:** `cmd/server/main.go` 及 `internal/memory/inmem/*`
**状态:** ❌ 未集成 / 仅有内存实现
**任务:**
- [ ] **系统集成**: 在 `cmd/server/main.go` 中初始化 `MemoryManager` 并将其挂载到 API 处理流程中。
- [ ] **真实向量数据库**: 替换 `MemoryVectorStore` (map) 为真实的向量数据库客户端 (如 Qdrant, Milvus, 或 pgvector)。
- [ ] **真实 LLM 客户端**: 替换 `RealLLMClientSimulator` 为真实的 OpenAI/Anthropic API 客户端，用于智能提取和决议。
- [ ] **真实 Embedding**: 替换 `SimpleEmbedder` (SHA256) 为真实的 Embedding 模型调用 (如 `text-embedding-3-small`)。
