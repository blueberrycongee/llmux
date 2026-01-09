# LLMux 流式容错机制 (Smart Stream Recovery) 实施文档

## 1. 项目背景与目标
LLMux 旨在构建一个企业级的高可用 LLM 网关。目前，LLMux 已经实现了基础的 429 限流重试和连接建立阶段的故障转移。然而，在流式传输（Streaming）过程中，如果发生网络中断（如 TCP Reset 或 EOF），当前的实现会直接断开连接，导致用户接收到截断的内容。

**目标**：实现“智能断点续传”（Smart Stream Recovery），即在流式传输中断时，自动捕获异常，保留已生成的内容，并向备用节点发起新的请求以“续写”剩余内容，从而对用户屏蔽底层故障，提供无缝的流式体验。

---

## 2. LiteLLM 源码深度解析
通过对 LiteLLM 源码（特别是 `litellm/router.py` 和 `litellm/litellm_core_utils/streaming_handler.py`）的分析，我们总结了其核心实现逻辑：

### 2.1 核心机制：`MidStreamFallbackError`
LiteLLM 定义了一个专门的异常 `MidStreamFallbackError`（在 `exceptions.py` 中），用于在流式传输过程中捕获错误。该异常携带了关键的上下文信息：
- `generated_content`: 故障前已生成的文本内容。
- `original_exception`: 原始的底层错误。
- `is_pre_first_chunk`: 标记是否在收到第一个 Chunk 之前就失败了。

### 2.2 捕获与重试逻辑 (`streaming_handler.py`)
在 `CustomStreamWrapper.__anext__` 迭代器中：
1.  **Chunk 处理**：正常迭代并累积已生成的内容到 `self.response_uptil_now`。
2.  **异常捕获**：如果底层生成器抛出异常（非 `StopAsyncIteration`），捕获该异常。
3.  **抛出信号**：将捕获的异常封装为 `MidStreamFallbackError` 并抛出，同时将 `self.response_uptil_now` 传入异常对象。

### 2.3 恢复与续写逻辑 (`router.py`)
在 `_acompletion_streaming_iterator` 中：
1.  **捕获信号**：`try...except MidStreamFallbackError` 捕获流中断信号。
2.  **构造新请求**：
    -   保留原始 `messages`。
    -   追加一个 `Assistant` 消息，内容为 `e.generated_content`（已生成部分）。
    -   追加一个 `System` 提示（可选），引导模型“请继续完成上文的回答，不要重复”。
3.  **故障转移**：调用 `async_function_with_fallbacks_common_utils`，利用 Router 的 Fallback 机制选择新的健康节点。
4.  **无缝拼接**：获取新请求的流，继续 `yield` 新的 Chunk。对于用户而言，流只是暂停了一下，然后继续输出。

---

## 3. LLMux 实施方案

基于 Go 语言特性和 LLMux 现有架构，我们将采用类似的设计思路，但更符合 Go 的惯用模式（Composition over Inheritance）。

### 3.1 核心组件设计

#### A. `ResilientStreamReader` (新组件)
这是对现有 `StreamReader` 的一层封装，负责状态管理和自动恢复。

```go
type ResilientStreamReader struct {
    // 内部状态
    currentReader *StreamReader
    accumulated   strings.Builder // 缓冲区，记录已收到的完整文本
    
    // 恢复所需的上下文
    ctx           context.Context
    client        *Client
    originalReq   *ChatRequest
    
    // 配置
    maxRetries    int
    retryCount    int
}
```

#### B. 增强 `StreamReader`
现有的 `StreamReader` 需要暴露底层的读取错误，而不是吞掉。

### 3.2 详细交互流程

1.  **初始化**：用户调用 `client.ChatCompletionStream`。
2.  **首次请求**：`client` 内部发起请求，获得第一个 `StreamReader`。
3.  **封装**：`client` 将 `StreamReader` 包装进 `ResilientStreamReader` 并返回给用户。
4.  **用户读取 (`Recv`)**：
    -   用户调用 `ResilientStreamReader.Recv()`。
    -   `ResilientStreamReader` 调用内部 `currentReader.Recv()`。
    -   **成功**：将 Chunk 内容追加到 `accumulated` 缓冲区，返回 Chunk 给用户。
    -   **失败 (EOF/Error)**：
        -   检查错误是否为可恢复错误（网络中断、超时，而非 400 业务错误）。
        -   检查是否还有重试机会 (`retryCount < maxRetries`)。
        -   **触发恢复 (`tryRecover`)**：
            1.  关闭旧的 `currentReader`。
            2.  构造新请求：复制 `originalReq`，在 `Messages` 末尾追加 `{Role: "assistant", Content: accumulated.String()}`。
            3.  调用 `client.router.Pick()` 获取新节点（利用现有的 Fallback 逻辑）。
            4.  发起新请求，获得新的 `StreamReader`。
            5.  更新 `currentReader`，递归调用 `Recv()`。
        -   **不可恢复**：直接返回错误给用户。

---

## 4. 测试驱动开发 (TDD) 方案

我们将采用 TDD 模式，先编写测试用例模拟故障，再实现功能。

### 4.1 测试场景：`TestStreamRecovery_MidStreamFailure`

**目标**：模拟在流传输一半时连接断开，验证系统能否自动重连并补全剩余内容。

**步骤**：
1.  **Mock Server A (故障节点)**：
    -   接收请求。
    -   发送 "Hello, "。
    -   发送 "this is "。
    -   **强制断开 TCP 连接** (模拟网络故障)。
2.  **Mock Server B (备用节点)**：
    -   接收请求（此时请求应包含 "Hello, this is " 作为上下文）。
    -   发送 "a resilient "。
    -   发送 "system."。
    -   正常结束。
3.  **Client 配置**：
    -   配置 Router 包含 Server A 和 Server B。
    -   开启 Fallback。
4.  **断言**：
    -   用户端 `Recv()` 到的完整内容应为 "Hello, this is a resilient system."。
    -   中间不应收到 error。
    -   验证 Server B 收到的请求中确实包含了 Server A 已生成的内容。

### 4.2 测试场景：`TestStreamRecovery_UnrecoverableError`

**目标**：验证遇到不可恢复错误（如 400 Bad Request）时，不会触发无限重试。

**步骤**：
1.  **Mock Server**：发送部分数据后返回 400 错误。
2.  **断言**：`Recv()` 返回错误，且不进行重试。

---

## 5. 实施计划

| 阶段                      | 任务                                                                                                     | 预计耗时 |
| :------------------------ | :------------------------------------------------------------------------------------------------------- | :------- |
| **Phase 1: 基础重构**     | 修改 `StreamReader` 以支持更细粒度的错误暴露；在 `client.go` 中引入 `ResilientStreamReader` 结构体定义。 | 1 Hour   |
| **Phase 2: TDD 测试编写** | 编写 `client_stream_recovery_test.go`，实现上述 Mock Server 逻辑和断言。此时测试应 Fail。                | 1 Hour   |
| **Phase 3: 核心逻辑实现** | 实现 `ResilientStreamReader.Recv()` 和 `tryRecover()` 方法。打通上下文拼接和 Router 重新选路逻辑。       | 2 Hours  |
| **Phase 4: 验证与优化**   | 运行测试直到 Pass。优化日志记录，确保故障转移过程在 Debug 模式下可见。                                   | 1 Hour   |

---

## 6. 验收标准 (Acceptance Criteria)

1.  **自动化测试通过**：`TestStreamRecovery_MidStreamFailure` 测试用例 100% 通过。
2.  **内容完整性**：在发生一次网络中断的情况下，客户端接收到的最终字符串与预期完全一致，无丢失、无重复。
3.  **透明性**：客户端调用者无需修改任何代码，无需感知底层的重连逻辑。
4.  **无死循环**：确保在所有节点都故障时，重试次数受限，最终正确返回错误。
5.  **资源清理**：确保在重试过程中，旧的 `StreamReader` 和 HTTP 连接被正确关闭，无 Goroutine 泄漏。
