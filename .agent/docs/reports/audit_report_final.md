# 审计追踪日志 (Audit Trace Log)

## 1. 断链与死代码 (Broken Links & Dead Code)

### [严重] 模块名/功能：下游限流 (Downstream Rate Limiting)
*   **定义位置：** `internal/auth/ratelimiter.go` (TenantRateLimiter)
*   **追踪结果：**
    *   **定义：** `TenantRateLimiter` 及其方法 `RateLimitMiddleware` 在 `internal/auth` 包中被完整定义。
    *   **断链：** 在 `cmd/server/main.go` 的初始化流程中，虽然初始化了 `authMiddleware`，但**从未调用** `RateLimitMiddleware`。
    *   **证据：** `main.go` 第 248 行仅调用了 `authMiddleware.Authenticate(httpHandler)`。而在 `internal/auth/middleware.go` 的 `Authenticate` 方法中（第 50-134 行），仅检查了 `IsOverBudget`，**完全没有调用** `RateLimiter.Check()` 或任何限流逻辑。
    *   **结论：** 所谓的“网关层限流”是**虚假的**。代码存在但未实装，流量可以无限制地穿透到应用层（虽然 `client.go` 有应用层限流，但网关层防御失效）。

### [严重] 模块名/功能：路由实现分裂 (Zombie Router Implementation)
*   **定义位置：** `internal/router/` (simple_shuffle.go, least_busy.go 等)
*   **追踪结果：**
    *   **定义：** `internal/router` 包中包含了一整套路由策略实现。
    *   **断链：** `client.go` (第 895 行) 使用的是 `routers` 包 (`github.com/blueberrycongee/llmux/routers`) 来创建路由器。
    *   **证据：** `main.go` 仅使用了 `internal/router` 中的 `NewRedisStatsStore` (第 418 行)。`internal/router` 中的所有具体路由算法文件（如 `simple_shuffle.go`）在生产路径中**从未被引用**。
    *   **结论：** `internal/router` 是**僵尸代码**库，不仅增加了维护负担，还可能导致测试环境（如果使用了它）与生产环境行为不一致。

## 2. 数据流完整性分析 (Data Flow Integrity)

### [漏洞] 参数/功能：Anthropic Stream Options (流式选项)
*   **证据：**
    *   **输入：** `pkg/types/request.go` 中 `ChatRequest` 定义了 `StreamOptions` 字段 (第 24 行)。
    *   **转换：** 在 `providers/anthropic/anthropic.go` 的 `transformRequest` 函数 (第 208 行) 中。
    *   **丢失：** `anthropicRequest` 结构体 (第 104 行) **没有定义** `stream_options` 字段，且转换逻辑完全忽略了 `req.StreamOptions`。
    *   **后果：** 用户请求中的 `include_usage: true` 等流式选项在传递给 Claude 模型时会被**静默丢弃**，导致客户端无法获取 Token 使用量。

### [隐患] 参数/功能：多模态支持 (Multimodal Support)
*   **证据：**
    *   `pkg/types/request.go` 中 `ChatMessage.Content` 被定义为 `json.RawMessage`。
    *   虽然这允许透传任意 JSON（包括图片数组），但代码库中**缺乏显式的 Image/Audio 类型定义和校验逻辑**。
    *   这意味着 `llmux` 对多模态的支持是“盲目”的——它不理解也不验证多模态数据，只是作为字节流透传。这在需要对图片进行计费或处理（如压缩、审核）时将成为重大阻碍。

## 3. 全链路逻辑验证总结

*   **已打通的链路 (Solid Links)：**
    *   **核心对话流程 (Chat Completion)：** `main` -> `handler` -> `client` -> `router` -> `openai provider` 的链路是畅通的。
    *   **流式响应 (Streaming)：** `stream.go` 实现了健壮的 `StreamReader`，包含 `tryRecover` 重试机制，能正确处理连接中断和 `[DONE]` 信号。
    *   **上游限流 (Upstream Rate Limit)：** `client.go` 中的 `checkRateLimit` (第 679 行) 被正确调用，且与 `resilience` 包打通。

*   **未打通/虚假的链路 (Broken/Hollow Links)：**
    *   **网关防御 (Downstream Rate Limit)：** 完全断开，代码存在但未接入。
    *   **非 OpenAI 提供商的完整性：** 如 Anthropic 的 `StreamOptions` 丢失，表明非 OpenAI 提供商的适配层存在数据丢失风险。
    *   **路由一致性：** 生产代码与旧的 `internal/router` 实现共存，存在混淆风险。

## 4. 最终判决

基于**控制流分析**，`llmux` 的完成度目前属于 **“空心化 (Hollow)”** 状态。

虽然其核心的 OpenAI 转发路径是实心的，但在**企业级网关**所必须的“防御层（限流）”和“多模态适配层”上存在明显的**断链**和**数据丢失**。它更像是一个“具备重试功能的 HTTP 代理”，而非一个严谨的“AI 网关”。

**建议立即行动：**
1.  **Wire Up Rate Limiter:** 在 `main.go` 中显式调用 `TenantRateLimiter.RateLimitMiddleware`。
2.  **Purge Zombie Code:** 删除 `internal/router` 中除 `StatsStore` 以外的所有代码，或将其合并到 `routers` 包。
3.  **Fix Data Mutation:** 修复 `providers/anthropic` (及其他提供商) 的参数映射，确保 `StreamOptions` 等字段正确透传。
