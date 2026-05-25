# LLMux 项目核心理论、目标与实验数据提炼

## 文档目的

本文档基于仓库内现有 README、规范文档、路线图、Benchmark 文档与论文草稿，
对 LLMux 的核心定位、理论框架、工程目标和已公开实验数据进行集中提炼。

这是一份项目级摘要，不替代原始规格文档与论文原文。

## 提炼来源

- `README.md`
- `docs/ROADMAP.md`
- `docs/specs/GATEWAY_SPEC.md`
- `docs/specs/STATE_ADAPTERS.md`
- `routers/README.md`
- `bench/README.md`
- `docs/AGENT_HETEROGENEOUS_GATEWAY_THESIS_PAPER.md`

## 1. 项目核心定位

LLMux 当前实际上有两层定位。

### 1.1 基础产品定位

在基础产品层，LLMux 是一个用 Go 编写的高性能 LLM Gateway。它的核心目的
不是单纯做请求转发，而是把模型接入、路由、治理、审计、观测这些能力上收
为统一的平台层能力，而不是散落在业务系统里。

这一层的稳定目标是：

- 提供 OpenAI 兼容接口，如 chat、responses、embeddings、models。
- 支持多 Provider 接入与统一路由策略控制。
- 提供认证、预算、限流、审计等治理能力。
- 支持单机部署与分布式部署。
- 提供 metrics、tracing、health check、dashboard 等运维与分析能力。

### 1.2 研究演进定位

在研究与演进层，项目正在从“多 Provider 模型网关”继续扩展为“面向 Agent 的
异构路由中枢”。

论文草稿把问题定义为一个多层决策系统：

- 团队层路由：是否由 Expert Agent Team 接管任务。
- Agent 层路由：由哪个 lead Agent 负责主执行。
- 模型层调度：在候选模型池中选出当前最合适的模型。
- 工具增强执行：通过 MCP 给 Agent 注入真实外部工具能力。

因此，项目目标已经不只是“把请求发给哪个 Provider”，而是在动态约束下回答：
一个复杂任务应该如何在团队、Agent、工具和模型之间被更合理地调度。

## 2. 项目核心工程目标

根据 `docs/ROADMAP.md`，项目的核心工程目标可以概括为：

- 交付企业级治理网关，同时支持微服务模式和单体模式。
- 设计保持务实：高性能、运维友好、不做过度工程化。
- 对齐关键 OpenAI 风格接口与 LiteLLM 重要参数。

路线图体现出的核心实施方向包括：

- 状态接口抽象与外部化。
- 统一治理内核。
- 路由与弹性能力标准化。
- 控制面与可观测能力建设。
- OpenAI / LiteLLM 兼容与开发体验优化。

## 3. 核心理论与方法论

## 3.1 网关即平台控制层

项目把 LLM Gateway 视为“平台控制层”，而不是“薄代理”。

在这个模型中，网关负责：

- 协议归一化。
- 治理评估。
- 路由选择。
- 上游执行。
- 使用量与费用记账。
- 可观测性输出。

`docs/specs/GATEWAY_SPEC.md` 中定义的请求生命周期是：

1. Ingress
2. Validation
3. Governance Evaluate
4. Route Selection
5. Upstream Execution
6. Response Handling
7. Governance Account
8. Observability

这体现了项目的一个核心思想：治理逻辑不应零散地长在 handler 和 router 里，
而应由网关提供确定性、可复用的统一执行流水线。

## 3.2 治理内核理论

路线图明确提出要把治理逻辑从 handler 与 router 中剥离出来，统一到治理内核。

在工程上，治理被建模为两段：

- 请求前评估：认证、租户解析、限流、预算、策略判定。
- 请求后记账：使用量记录、费用更新、审计日志。

同时支持：

- 尽可能异步的 accounting。
- 幂等写入。
- 运行时配置热更新。

这说明项目把“治理”视为执行主流程中的一等阶段，而不是附属功能。

## 3.3 状态外部化理论

项目的一个关键架构原则是：有状态能力必须接口化，并在需要时迁移到进程外。

`docs/specs/STATE_ADAPTERS.md` 中定义的状态拆分如下：

- 路由统计：内存或 Redis。
- Round Robin 计数器：内存或 Redis。
- 限流：本地 fallback 或 Redis 分布式限流器。
- 预算 / 租户 / 使用量：内存或 Postgres。
- 审计日志：内存或 Postgres。

这一原则背后的理论是：
分布式模式不应改变 API 语义，而只应更换状态后端和路由统计行为。

## 3.4 自适应路由理论

在基础网关层，LLMux 不是靠固定规则做死板转发，而是基于运行态反馈做自适应路由。

`routers/README.md` 体现了三点核心思想：

- 支持多种路由策略，包括最低时延、最少繁忙度、最低 TPM/RPM、最低成本等。
- 使用 EWMA 跟踪 latency、TTFT 和 success rate。
- 在 latency router 中引入动态权重：
  `Weight = BaseWeight * (SuccessRate^2) / Latency`

这个理论说明项目认为：
路由决策应该对近期 Provider 质量变化敏感，而不是只依赖静态权重。

## 3.5 面向 Agent 的三层路由理论

论文草稿把项目从 Provider 路由扩展成三层任务路由：

- 第 1 层：Team Routing，依赖 Team Registry + Team Memory。
- 第 2 层：Agent Routing，依赖 Agent Registry + Route Memory + 工具绑定。
- 第 3 层：Candidate Model Scheduling，依赖 RPM/TPM、cooldown 与预测压力。

其主要理论增量包括：

- Route Memory：历史路径与执行结果成为可复用路由知识。
- Team Memory：历史团队协作成为可复用的团队级经验。
- Tool-Augmented Expert Agent：Agent 不再只是 prompt 里的角色，而是绑定真实工具。
- Predictive Scheduling：在请求发送前规避高风险候选模型，而不是失败后才 fallback。

## 3.6 研究层的优化目标

论文草稿把研究问题进一步形式化为四个优化目标：

- 最大化请求成功率。
- 最小化限流碰撞概率。
- 最小化平均响应时延。
- 在安全约束下最大化模型资源利用率。

这说明项目的扩展方向不是纯语义路由，而是一个同时考虑成功率、稳定性、
时延与资源利用率的约束优化问题。

## 4. 核心研究问题

论文中明确提出了四个核心研究问题：

- RQ1：如何利用请求语义与 Route Memory 提升 Agent 路由的合理性与一致性？
- RQ2：如何在复杂任务中引入 Expert Agent Team，并用 Team Memory 提升团队级复用能力？
- RQ3：如何在多候选模型、多工具绑定场景下做主动调度，而不是只在失败后被动切换？
- RQ4：如何把 Team 路由、Agent 路由、候选模型调度、工具增强执行与多租户治理整合为统一原型系统？

这四个问题基本构成了项目“上层研究目标”的完整定义。

## 5. 当前已公开实验数据

仓库里的实验输出目前分为两类：

- 基础网关层的量化 Benchmark 数据。
- Agent 路由原型的功能性验证结果。

这两类证据不能混在一起理解。

## 5.1 基础网关量化 Benchmark

README 中公开了一组 LLMux 与 LiteLLM 的性能对比结果。

### Benchmark 条件

- 硬件：4 CPU cores。
- 后端条件：本地 Mock LLM Server，固定 50 ms 延迟。
- 请求总数：10,000。
- 并发数：100。

### 已公开结果

| 指标 | LLMux (Go) | LiteLLM (Python) | 结论 |
| --- | ---: | ---: | --- |
| Throughput (RPS) | 1943.35 | 246.52 | 吞吐约 8 倍 |
| Mean Latency | 51.29 ms | 403.94 ms | 平均额外开销约低 8 倍 |
| P99 Latency | 91.71 ms | 845.37 ms | 尾延迟显著更稳 |

### 提炼结论

在 README 所给测试环境下，LLMux 的 Go 网关内核相对于 Python 对照基线，
表现出显著更低的框架开销和更稳定的尾延迟。

### Benchmark 工具支持的指标

根据 `bench/README.md`，压测工具还支持统计：

- RPS
- Latency P50
- Latency P95
- Latency P99
- Memory
- Errors

但当前 README 实际公开的对比数据只有：

- RPS
- Mean Latency
- P99 Latency

## 5.2 Agent 路由原型的功能验证结果

论文草稿没有给出大规模统计实验表，而是给出了原型级、功能级验证结论。

### 已验证能力

论文中声称系统已经能够：

- 在对话接口返回 `selected_team`、`team_participants`、`selected_agent`、
  `selected_candidate`、`candidate_failovers`、`candidate_states` 等字段。
- 在团队演练接口返回 `participating_agents`、`steps`、`recorded_memory` 等字段。
- 记录 `consulted` / `reused` Route Memory 与 Team Memory。
- 在候选模型调度层基于近一分钟 usage 和预测压力计算 `selection_score`。
- 在候选模型接近上限时提前跳过 `predicted_saturated` 候选模型。
- 通过 MCP 工具注入，使 Expert Agent 具备真实外部工具调用能力。

### 提炼结论

当前仓库已经证明“三层路由 + 记忆增强 + 工具增强 + 候选模型主动调度”
这套思路具备可实现性和初步有效性。

但这仍然属于“原型验证成立”，还不能等同于“大规模统计研究已经完成”。

## 5.3 证据边界与当前局限

论文草稿也明确指出了当前证据边界：

- 当前验证仍以原型系统和功能性实验为主。
- 候选模型 usage 主要还是本地内存态。
- 调度 score 目前主要考虑 weight、RPM/TPM 压力和 cooldown。
- latency、cost、success rate 等因素尚未完整纳入候选模型综合评分。
- 团队协作目前仍偏轻量，还不是成熟的并行自治多 Agent 执行系统。

因此，当前项目在研究层面的定位更准确地说是：

- 一个已实现、可验证的研究原型。
- 而不是一个已经完成大规模量化研究闭环的最终调度系统。

## 5.4 真实端到端实验补充数据

除仓库中原有 README Benchmark 与论文功能验证外，本地还额外执行了两组真实实验：

- `Value Lab`：10 次真实调用
- `Agent Router`：10 次真实调用

详细原始数据与完整分析已整理在：

- `docs/REAL_EXPERIMENT_RESULTS_CN.md`
- `docs/real_experiment_value_lab_runs.json`
- `docs/real_experiment_agent_router_runs.json`

### 5.4.1 Value Lab 真实结果摘要

在 `grounded-repo-qa` 场景下，10 次真实调用得到的核心结果为：

- `optimized` 被推荐为 winner：8 / 10
- `baseline` 成功：10 / 10
- `optimized` 成功：8 / 10
- `baseline` 平均 latency：12183.30 ms
- `optimized` 平均 latency：39070.90 ms
- `optimized` 平均 tool calls：5.80

同时，`optimized` 的 2 次失败原因完全一致：

- `exceeded maximum tool iterations (10)`

进一步看 baseline 与 optimized 成功样本的对比，优化链路的收益主要体现在：

- groundedness 从 `0.34` 提升到 `0.81`，相对提升约 `138%`
- hallucination risk 从 `0.62` 降到 `0.2975`，相对下降约 `52%`
- response chars 从 `491.8` 提升到 `927.75`，相对提升约 `88.6%`
- route depth 从 `1` 提升到 `5`
- `winning_dimensions` 中：
  - `agent-selection` 出现 `10` 次
  - `tool-aware` 出现 `10` 次
  - `groundedness` 出现 `8` 次
  - `hallucination-control` 出现 `8` 次

这说明在当前实验场景中，optimized 链路大多数时候更被系统认为“更有价值”，
而且这种价值主要体现为：

- 更好的 Agent 选择
- 更强的工具感知与工具使用能力
- 更高的 groundedness
- 更低的 hallucination risk
- 更高的信息密度

但它不是“更快、更便宜”的优化，而是偏向“更强执行能力、更高答案质量”的优化。
其代价是：

- 时延显著上升
- token 消耗显著上升
- 成本显著上升
- 工具调用次数显著上升

换句话说，这组真实数据证明了 Value Lab 当前优化链路的价值主张更接近：

- 用额外成本换取更强的任务完成能力与更高的 groundedness

而不是：

- 用更聪明的路由同时降低时延与成本

### 5.4.2 Agent Router 真实结果摘要

对 `control/conversation/chat` 连续进行 10 次真实调用后，得到的核心结果为：

- 业务成功：6 / 10
- 平均客户端时延：41720.40 ms
- Selected Agent：`code-specialist`，10 次全命中
- Route Source：`score-router`，10 次全命中
- Memory Influence：`reused`，10 次全命中
- 成功样本中的 Selected Candidate：全部为 `deepseek-primary/deepseek-chat`

4 次业务失败的原因也完全一致：

- `exceeded maximum tool iterations (10)`

这组结果带来两个很重要的判断：

1. 路由层本身是稳定的  
   Agent 选择、Route Source 和 Memory Influence 都没有明显抖动。

2. 当前主要瓶颈不在路由层，而在工具增强执行层  
   失败并不是“选错 Agent”或“选错模型”，而是进入工具调用循环后触达了最大迭代次数。

因此，Agent Router 这组实验更适合作为“优化链路稳定生效”的证据，而不是
“baseline vs optimized 收益对比”的证据。真正用于收益对比的核心数据仍然是 Value Lab。

因此，如果后续要继续提升真实成功率，优先级更高的方向并不是继续微调路由打分，
而是：

- 优化 MCP tool loop 的终止条件
- 降低工具循环失控概率
- 对 `code-specialist` 这类 Agent 进一步收紧工具调用策略
- 在保证质量的同时压缩工具链长度

## 6. 当前成熟度判断

综合现有材料，可以把项目分成两条成熟度不同的主线：

- “企业级 LLM Gateway”这条线相对成熟：
  兼容接口、治理、分布式状态、路由、弹性、观测、控制面都已经形成明确规格和路线图闭环。
- “面向 Agent 的异构多层路由”这条线更具创新性：
  已经有工作原型和功能验证，但定量评估仍处于早期阶段。

换句话说，仓库同时包含了两种价值：

- 一个可落地的平台型网关方向。
- 一个有研究增量的智能路由方向。

## 7. 关键提炼结论

- LLMux 的根本目标，是把模型调用上升为“可治理、可观测、可路由”的基础设施。
- 它的核心工程理论，是一条确定性网关流水线：
  校验 -> 治理 -> 路由 -> 执行 -> 记账 -> 观测。
- 它的基础路由理论，是根据近期运行态反馈做自适应调度，而不是固定分发。
- 它的主要研究扩展，是 Team -> Agent -> Candidate Model 的三层路由体系。
- 目前最强的量化证据，来自基础网关对 LiteLLM 的性能 Benchmark。
- 目前 Agent 路由方向最强的证据，来自功能性原型验证，而不是大规模统计实验。

## 8. 适用场景

这份提炼文档适合用于：

- 项目介绍。
- 论文 / 汇报材料整理。
- 新成员架构认知同步。
- 区分“基础网关能力”和“Agent 路由研究扩展”两条主线。

若需要实现细节，应回到文首列出的原始文档继续展开阅读。
