# 开发策略文档

## 核心理念

**让 AI 做翻译官，你做架构师。**

我们不是从零造轮子，而是站在 LiteLLM 的肩膀上，用 Go 重写其核心逻辑。脏活累活交给 AI 批量生成，精细活由人把控。

---

## 第一步：Clone LiteLLM 作为参考

```bash
git clone https://github.com/BerriAI/litellm.git ./reference/litellm
```

### 重点关注的目录

```
litellm/
├── llms/                    # 🔥 各 Provider 的适配实现
│   ├── openai.py
│   ├── anthropic.py
│   ├── azure.py
│   ├── vertex_ai.py
│   └── ...
├── utils.py                 # Token 计算、model 映射
├── cost_calculator.py       # 成本计算逻辑
├── model_prices_and_context_window.json  # 💰 价格表，直接抄
└── exceptions.py            # 错误类型定义
```

---

## 工作分类

### 🤖 脏活累活（AI 批量生成）

| 任务 | 来源文件 | 输出 |
|------|----------|------|
| Request/Response 结构体 | `llms/*.py` 的 dataclass | Go struct + json tag |
| 参数映射表 | 各 provider 的 `transform_request()` | Go map/switch |
| 错误码映射 | `exceptions.py` | Go error types |
| Model 名称标准化 | `model_prices_and_context_window.json` | Go const + map |
| Token 计算映射 | `utils.py` 的 encoding 选择逻辑 | Go map |
| 单元测试 cases | 根据接口定义 | table-driven tests |

**执行方式**：写 prompt 模板，喂 Python 代码，让 AI 输出 Go 代码

### 🧠 精细活（人工把控）

| 任务 | 为什么不能让 AI 做 |
|------|-------------------|
| SSE 流式转发 | 背压、连接检测、buffer 复用，Python 实现参考价值低 |
| 熔断器调参 | 需要结合实际流量模式 |
| 配置热重载 | 原子性、一致性是架构决策 |
| Metrics label 设计 | cardinality 控制需要经验 |
| 超时层级设计 | 搞错会卡死或泄漏 |
| 核心接口定义 | 决定整体架构走向 |

---

## 开发阶段

### Phase 1: 骨架搭建（1-2 周）

**目标**：能跑通一个 OpenAI 请求，有 metrics 输出

```
1. 初始化 Go 项目结构
2. 定义核心接口 (Provider, Router, Config)
3. 实现 HTTP Server (net/http 或 fasthttp)
4. 实现配置加载 + fsnotify 热重载
5. 集成 Prometheus metrics
6. 集成 slog 结构化日志
7. 实现 OpenAI adapter（手写，作为模板）
8. 跑通 /v1/chat/completions（非流式）
```

**产出**：一个能用的最小网关 + 作为模板的 OpenAI adapter

### Phase 2: AI 批量生成（1 周）

**目标**：覆盖主流 Provider

```
1. 从 LiteLLM 提取各 provider 的结构定义
2. 编写 prompt 模板
3. AI 生成 Anthropic/Azure/Gemini adapter
4. AI 生成 model 映射表、价格表
5. AI 生成各 adapter 的单元测试
6. 人工 review + 修正
```

**产出**：支持 4-5 个主流 Provider

### Phase 3: 流式支持（1 周）

**目标**：SSE 流式转发稳定可靠

```
1. 实现 SSE 转发核心逻辑
2. sync.Pool buffer 复用
3. client 断开检测 + 上游 cancel
4. 各 provider 的流式响应适配
5. 编写流式相关的测试用例
```

**产出**：流式请求可用，无 goroutine 泄漏

### Phase 4: 高可用（1-2 周）

**目标**：生产级稳定性

```
1. 集成 gobreaker 熔断器
2. 实现 per-provider semaphore（舱壁模式）
3. 实现 token bucket 限流
4. 实现优雅关闭 (drain mode)
5. 健康检查端点 (/health/live, /health/ready)
6. 压测 + 调参
```

**产出**：能扛住流量突增，上游挂了不会雪崩

### Phase 5: 可观测性增强（1 周）

**目标**：成为 AI 流量的显微镜

```
1. 集成 OpenTelemetry tracing
2. 实现日志脱敏 (API Key, PII)
3. 完善 metrics（token 用量、成本归因）
4. 集成 tiktoken-go 估算 token
```

**产出**：完整的可观测性三件套

### Phase 6: 云原生打包（3 天）

```
1. 多阶段 Dockerfile (distroless, <20MB)
2. Helm Chart
3. K8s manifests (Deployment, Service, ConfigMap)
4. CI/CD pipeline (GitHub Actions)
```

---

## 测试策略

### 不测或轻测

- AI 生成的 struct 定义（跑一下 marshal/unmarshal 就行）
- 纯映射表（错了用户会报 400，自然发现）
- 简单 CRUD 逻辑

### 重点测试

| 模块 | 测试方式 | 关注点 |
|------|----------|--------|
| SSE 转发 | mock server + 异常注入 | 内存泄漏、goroutine 泄漏、边界 case |
| 熔断器 | mock clock + 状态验证 | 状态机转换正确性 |
| 并发控制 | race detector + 压测 | 死锁、permit 泄漏 |
| 热重载 | 临时文件 + 并发读写 | 原子性、错误回滚 |
| 优雅关闭 | 集成测试 | 请求不丢、超时强杀 |

### 性能测试

```go
// 必须 benchmark
BenchmarkSSEForward      // 流式吞吐
BenchmarkTokenCount      // tiktoken 速度
BenchmarkRouterSelect    // 路由延迟

// 关注指标
- ns/op
- B/op (越小越好)
- allocs/op (越少越好)
```

---

## Prompt 模板示例

用于让 AI 批量生成 adapter：

```
你是一个 Go 专家。我会给你 LiteLLM 中某个 provider 的 Python 实现，
请将其转换为 Go 代码。

要求：
1. 生成 Request/Response 的 Go struct，带正确的 json tag
2. 生成参数转换函数 TransformRequest()
3. 生成响应转换函数 TransformResponse()
4. 遵循 Go 命名规范 (CamelCase)
5. 添加 GoDoc 注释（英文）

以下是 Python 代码：
---
{paste python code here}
---

请生成对应的 Go 代码。
```

---

## 里程碑检查点

| 里程碑 | 验收标准 |
|--------|----------|
| M1: 骨架完成 | curl 能打通 OpenAI 非流式请求，Prometheus 有数据 |
| M2: 多 Provider | 支持 OpenAI/Claude/Gemini，可切换 |
| M3: 流式可用 | SSE 正常工作，client 断开能 cancel 上游 |
| M4: 高可用 | 压测 1000 QPS 不崩，熔断生效 |
| M5: 可观测 | Grafana 能看到 token 用量、延迟分布、错误率 |
| M6: 可部署 | docker pull 就能跑，镜像 <20MB |

---

## 风险与应对

| 风险 | 应对 |
|------|------|
| AI 生成的代码有 bug | 每个 adapter 至少跑一个真实请求验证 |
| Provider API 变更 | 关注 changelog，adapter 设计要易于修改 |
| 流式实现复杂度高 | 先做 OpenAI，它最标准；其他参考它改 |
| 性能不达预期 | 早期就加 benchmark，持续监控 |

---

## 现在开始

```bash
# 1. Clone 参考项目
git clone https://github.com/BerriAI/litellm.git ./reference/litellm

# 2. 看看 OpenAI adapter 长什么样
cat ./reference/litellm/litellm/llms/openai.py

# 3. 开始写 Go 骨架...
```
