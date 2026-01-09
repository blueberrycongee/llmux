# llmux vs litellm：全功能版图覆盖与代码深度审计报告

## 1. 总体评价 (Executive Summary)

*   **成熟度定级：** **高级 MVP (Advanced MVP) / 早期企业级**
*   **一句话结论：** **核心交互层（Chat/Stream/Embedding）已达生产级水准，架构设计优于 `litellm`（Go vs Python），但周边生态（多模态、可观测性、分布式治理）仍处于“荒原”状态。**

`llmux` 展示了极高的工程素养，特别是在流式处理和故障恢复方面甚至超越了部分成熟竞品。然而，作为网关，它在“广度”上严重缺失，目前更像是一个“高可用的 Chat Completion 代理”，而非全能的 LLM Gateway。

---

## 2. 架构健康度与代码异味 (Architecture & Code Quality)

### **[A-] 架构设计**
*   **优点：** 采用了清晰的 **Clean Architecture**。`Provider` (适配器) -> `Router` (策略) -> `Client` (门面) -> `Pipeline` (插件) 分层清晰。
*   **并发模型：** Go 的 Goroutine 模型在处理高并发流式请求时，比 `litellm` 的 Python `asyncio` 具有天然的性能优势和更低的调试复杂度。

### **[B] 耦合度分析**
*   **Client 膨胀风险：** `client.go` (`Client` 结构体) 正在成为上帝对象 (God Object)。它同时管理着 `providers`, `deployments`, `router`, `cache`, `httpClient`, `pipeline`。虽然目前代码量 (~750行) 尚可控，但随着功能增加，急需拆分。
*   **Router 独立性：** `routers/` 包设计优秀，策略（Cost, Latency, TPM/RPM）完全解耦，易于扩展。

### **[B-] 代码异味 (Code Smells)**
*   **硬编码的成本计算：** `routers/cost.go` 中 `DefaultCostPerToken = 5.0` 是典型的 Magic Number。且缺乏动态价格更新机制。
*   **错误处理的“吞没”：** 在 `client.go` 的 `executeWithRetry` 中，虽然有重试逻辑，但部分错误类型判断依赖于 `prov.MapError` 的字符串匹配或状态码，缺乏统一的错误码枚举标准。

---

## 3. 全功能版图深度审计 (Feature Parity Matrix)

### **A. 模型交互核心 (Core I/O)**

| 功能模块             | llmux 现状     | 评级       | 证据/备注                                                                                                                     |
| :------------------- | :------------- | :--------- | :---------------------------------------------------------------------------------------------------------------------------- |
| **Chat Completion**  | ✅ **完整实现** | **生产级** | 支持完整的 OpenAI 协议，且具备高级的 **Mid-Stream Recovery** (流式中断恢复) 功能 (`stream.go`)。                              |
| **Function Calling** | ⚠️ **透传支持** | **骨架**   | `pkg/types/request.go` 定义了 `Tools` 结构，但网关层仅做 JSON 透传，无参数校验或回调处理逻辑。                                |
| **Embeddings**       | ✅ **完整实现** | **生产级** | **已修复 [P0] 缺陷。** `client.go` 现已支持完整的 Embedding 路由、重试和 Provider 调用。支持多态输入 (`[]string`/`[][]int`)。 |
| **Image Gen**        | ❌ **缺失**     | **缺失**   | 代码库中无任何图片生成相关定义。                                                                                              |
| **Audio (STT/TTS)**  | ❌ **缺失**     | **缺失**   | 代码库中无任何音频处理相关定义。                                                                                              |

### **B. 流量治理与韧性 (Traffic & Governance)**

| 功能模块            | llmux 现状     | 评级       | 证据/备注                                                                                                                         |
| :------------------ | :------------- | :--------- | :-------------------------------------------------------------------------------------------------------------------------------- |
| **Rate Limiting**   | ⚠️ **单机版**   | **玩具级** | `internal/resilience/ratelimiter.go` 使用内存 `sync.Mutex` 实现令牌桶。**不支持分布式限流**（如 Redis），多实例部署时限流将失效。 |
| **Caching**         | ✅ **完整实现** | **生产级** | `caches/redis` 封装了成熟的 `go-redis`，支持 Cluster/Sentinel，具备 Pipeline 优化。                                               |
| **Retries**         | ✅ **完整实现** | **生产级** | `client.go` 实现了指数退避 (Exponential Backoff) 重试。                                                                           |
| **Stream Recovery** | 🌟 **超越竞品** | **卓越**   | `stream.go` 实现了自动拼接已接收 Chunk 并重发请求的逻辑，这是 `litellm` 的高级特性，`llmux` 实现得非常扎实。                      |

### **C. 可观测性与计费 (Observability & Cost)**

| 功能模块             | llmux 现状     | 评级     | 证据/备注                                                                                         |
| :------------------- | :------------- | :------- | :------------------------------------------------------------------------------------------------ |
| **Cost Calculation** | ⚠️ **静态配置** | **简陋** | `routers/cost.go` 仅支持基于配置文件的静态价格路由，无动态价格抓取，无累计消费统计。              |
| **Logging/Tracing**  | ❌ **缺失**     | **缺失** | 虽然有 `pkg/plugin` 机制，但**没有任何内置的** Sentry, Datadog, Langfuse 插件。用户必须从零手写。 |

### **D. 安全与鉴权 (Security & Auth)**

| 功能模块           | llmux 现状     | 评级     | 证据/备注                                                |
| :----------------- | :------------- | :------- | :------------------------------------------------------- |
| **Key Management** | ✅ **完整实现** | **标准** | 支持静态 Key 和动态 `TokenSource` (适合 IAM/OIDC 集成)。 |
| **PII Masking**    | ❌ **缺失**     | **缺失** | 无敏感数据脱敏逻辑。                                     |

---

## 4. 关键缺陷清单 (Top Findings)

### 1. [P1] 分布式限流缺失 (Missing Distributed Rate Limiting)
*   **描述：** 当前限流器 `internal/resilience/ratelimiter.go` 仅基于进程内内存。在云原生多副本部署场景下，全局限流将无法生效，导致下游 Provider 被击穿。
*   **建议：** 基于现有的 `caches/redis` 模块，实现 Redis Lua 脚本限流。

### 2. [P1] 可观测性黑洞 (Observability Black Hole)
*   **描述：** 作为网关，缺乏对请求日志、延迟分布、Token 消耗的外部输出能力。`pkg/plugin` 虽有定义但无实现。
*   **建议：** 立即实现至少一个标准 Logger 插件（如 OpenTelemetry 或简单的 Webhook Logger）。

### 3. [P1] 多模态能力缺失 (Zero Multimodal Support)
*   **描述：** 相比 `litellm` 对 Image/Audio 的全面支持，`llmux` 目前仅能处理文本和 Embedding。
*   **建议：** 规划 Image 接口。

### 4. [P2] 静态成本路由 (Static Cost Routing)
*   **描述：** 依赖人工在 Config 中配置 `InputCostPerToken`，维护成本极高且易错。
*   **建议：** 引入类似 `litellm` 的 `model_prices.json` 静态数据库，或定期从网络拉取价格。

---

## 5. 总结与建议

`llmux` 的核心（Chat Client & Router）是一块美玉，代码质量高，并发控制细腻，特别是流式恢复功能的实现令人印象深刻。

**重大进展：** 我们刚刚修复了 **Embedding 入口未打通** 的 [P0] 级缺陷。现在 `llmux` 已经是一个完全合格的文本/向量混合网关。

**下一步行动建议：**

1.  **Go Wide:** 集中精力填充“空壳”，优先实现 **Redis 分布式限流**。
2.  **Plugin First:** 利用已有的 Pipeline 架构，开发官方的 **Logging/Metrics 插件**，解决可观测性问题。
