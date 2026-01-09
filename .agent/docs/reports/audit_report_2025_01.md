# LLMux vs LiteLLM: 全功能版图覆盖与代码深度审计报告

## 1. 总体评价 (Executive Summary)

*   **成熟度定级：** **Beta (Text-Only Production Ready)**
    *   *注：原定级为“企业级”，但因发现关键的可观测性与安全模块未集成（Feature Islands），故降级为 Beta。*
*   **一句话结论：** **核心文本交互与流式治理（Chat/Stream/RateLimit）已超越竞品，具备卓越的鲁棒性；但多模态（Image/Audio）完全缺失，且安全/监控模块处于“代码已写好但未插电”的状态，距离全能型企业网关仍有“最后一公里”的集成差距。**

`llmux` 在 Go 语言特有的并发优势上发挥得淋漓尽致，特别是在 **流式断点恢复 (Stream Recovery)** 和 **分布式限流** 上表现出极高的工程素养。然而，**“功能孤岛”** 现象严重——高质量的脱敏和监控代码静静地躺在 `internal/` 目录中，却未在主链路中生效，这是目前最大的风险点。

---

## 2. 架构健康度与代码异味 (Architecture & Code Quality)

### **[A] 架构设计 (Architecture)**
*   **优点：** 采用了清晰的 **Clean Architecture**。`Provider` (适配器) -> `Router` (策略) -> `Client` (门面) 的分层比 `litellm` 的混合式设计更易维护。
*   **并发模型：** Go 的 Goroutine 模型在处理高并发流式请求时，比 `litellm` 的 Python `asyncio` 具有天然的性能优势，且内存占用更低。

### **[B-] 代码异味 (Code Smells)**
*   **🔴 功能孤岛 (Feature Islands)：**
    *   `internal/observability/redact.go` 实现了非常完善的 PII 脱敏（正则匹配 API Key、Email 等），但**未被任何中间件调用**。
    *   `internal/observability/otel_metrics.go` 实现了标准的 GenAI Semantic Conventions，但在 `client.go` 或 `main.go` 中**未初始化**。
    *   **后果：** 用户以为开启了安全与监控，实则系统在“裸奔”。
*   **⚠️ 粗粒度的错误映射 (Coarse-grained Error Mapping)：**
    *   **LiteLLM**: 能精确区分 `ContextWindowExceeded` (可截断重试) vs `ContentPolicyViolation` (不可重试)。
    *   **LLMux**: `providers/openailike` 仅依赖 HTTP 状态码 (400/401/429)。如果上游返回 400 且 Body 含 `content_filter`，LLMux 无法识别，可能导致错误的重试。

---

## 3. 全功能版图深度审计 (Feature Parity Matrix)

### **A. 模型交互核心 (Core I/O)**

| 功能模块            | LiteLLM (Python) | LLMux (Go)     | 差距评级   | 审计证据/备注                                                                      |
| :------------------ | :--------------- | :------------- | :--------- | :--------------------------------------------------------------------------------- |
| **Chat Completion** | ✅ 完整支持       | ✅ **完整实现** | 🟢 Parity   | 支持完整 OpenAI 协议。                                                             |
| **Stream Recovery** | ⚠️ 基础重试       | 🌟 **卓越**     | 🔵 Superior | `stream.go` 实现了 `tryRecover`，能自动拼接已收到的 Chunk 并换节点续传，非常强悍。 |
| **Embeddings**      | ✅ 完整支持       | ✅ **完整实现** | 🟢 Parity   | 已修复类型兼容性问题，支持多态输入。                                               |
| **Image Gen**       | ✅ 完整支持       | ❌ **缺失**     | 🔴 Critical | 代码库中无任何图片生成定义。                                                       |
| **Audio (STT/TTS)** | ✅ 完整支持       | ❌ **缺失**     | 🔴 Critical | 代码库中无任何音频处理定义。                                                       |

### **B. 流量治理与韧性 (Traffic & Governance)**

| 功能模块          | LiteLLM (Python) | LLMux (Go)     | 差距评级 | 审计证据/备注                                                                     |
| :---------------- | :--------------- | :------------- | :------- | :-------------------------------------------------------------------------------- |
| **Rate Limiting** | ✅ Redis Lua      | ✅ **完整实现** | 🟢 Parity | `internal/resilience/redis_limiter.go` 成功移植了 LiteLLM 的 Lua 脚本，行为一致。 |
| **Routing**       | ✅ 丰富策略       | ✅ **核心覆盖** | 🟢 Parity | 支持 Latency/Cost/Random 策略。Go 在并发路由选择上性能更好。                      |
| **Cost Calc**     | ✅ 动态+硬编码    | ✅ **注册表**   | 🟢 Parity | `pkg/pricing` 结构清晰，但目前缺乏热更新机制 (Hot Reload)。                       |

### **C. 可观测性与安全 (Observability & Security)**

| 功能模块        | LiteLLM (Python)  | LLMux (Go)   | 差距评级 | 审计证据/备注                                                 |
| :-------------- | :---------------- | :----------- | :------- | :------------------------------------------------------------ |
| **PII Masking** | ✅ 完整集成        | ⚠️ **未集成** | 🟠 High   | 代码已在 `redact.go` 中完成，但未接入 Request/Response 管道。 |
| **Metrics**     | ✅ Prometheus/OTel | ⚠️ **未集成** | 🟠 High   | OTel 代码已就绪，但 Client 未调用 `RecordRequest`。           |

---

## 4. 关键缺陷清单 (Top Findings)

### 1. [P0] 功能孤岛：安全与监控未生效 (Feature Islands)
*   **现象**：代码库里有高质量的 `Redactor` 和 `OTelMetricsProvider`，但在 `Client` 初始化和请求处理链路中找不到它们的调用点。
*   **风险**：生产环境无指标监控，且日志中可能泄露敏感 API Key。
*   **建议**：立即在 `client.go` 的 `sendRequest` 前后插入 `ObservabilityManager` 的 Hook。

### 2. [P1] 错误处理粒度不足 (Coarse Error Handling)
*   **现象**：`providers/openailike/openailike.go` 中的 `MapError` 仅根据 HTTP Status Code 分类。
*   **对比**：LiteLLM 会解析 JSON Body 寻找 `error.code` (e.g. `context_length_exceeded`)。
*   **建议**：引入 `LLMError` 的子类型，并解析 Response Body。

### 3. [P1] 多模态能力缺失 (Zero Multimodal)
*   **现象**：完全不支持 Image/Audio。
*   **建议**：如果目标是全能网关，需尽快规划接口；如果是文本网关，需明确声明不支持。

---

## 5. 总结与建议

`llmux` 是一个**代码质量极高但完成度尚欠火候**的项目。它的核心骨架（Router, Streamer, Limiter）比 `litellm` 更健壮、更符合云原生标准。

**如果我是架构师，我会建议：**
1.  **Stop New Features**: 暂停开发新功能（如 Image/Audio）。
2.  **Focus on Integration**: 集中精力把现有的 `observability` 和 `security` 模块“插上电”，消除功能孤岛。
3.  **Refine Error Handling**: 提升错误处理的精细度，确保重试策略的准确性。

**结论：** 这是一个**潜力巨大**的准企业级项目，只要补齐“集成”这一课，它将是 `litellm` 的强力替代者。
