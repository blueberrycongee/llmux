# 真实实验结果汇总

## 1. 实验说明

- 实验时间：2026-04-16
- 实验方式：直接调用本地运行中的真实 LLMux 后端接口，后端使用真实 DeepSeek API
- Value Lab 接口：`http://localhost:18081/control/lab/compare`
- Agent Router 接口：`http://localhost:8081/control/conversation/chat`
- 每类实验运行次数：10 次

说明：

- 本轮数据属于真实端到端实验，不是单元测试或 mock 压测。
- 由于实验前做过接口预检，且后端内存态 Route Memory 不会自动清空，因此这组数据更接近“热启动 / warm-state”结果，而不是完全冷启动结果。

## 2. 实验设计

### 2.1 Value Lab

- 场景：`grounded-repo-qa`
- Prompt：`请分析这个 gateway 项目的核心目标、关键模块以及 value lab 优化链路的价值。必要时使用文件工具。`
- 选项：`token_optimization=true`，`cost_optimization=true`
- 目标：比较 baseline 与 optimized 两条真实执行链路的差异

### 2.2 Agent Router

- 用户问题：`请基于当前仓库结构，分析这个 gateway 项目的核心模块、路由逻辑和价值点，必要时使用文件工具。`
- 约束：
  - `required_tools = [preset-filesystem-list, preset-filesystem-read]`
  - `required_capabilities = [architecture, coding, analysis]`
  - `token_optimization=true`
  - `cost_optimization=true`
- 目标：观察真实路由选择、候选模型选择、memory influence 与客户端观测时延

## 3. Value Lab 统计结果

### 3.1 总体统计

| 指标 | 结果 |
| --- | --- |
| 实际完成采样 | 10 / 10 |
| Baseline 成功次数 | 10 / 10 |
| Optimized 成功次数 | 8 / 10 |
| 推荐 Winner 分布 | `optimized: 8`，`baseline: 2` |
| 客户端观测总耗时均值 | 39085.30 ms |
| 客户端观测总耗时标准差 | 11567.57 ms |
| Baseline latency 均值 | 12183.30 ms |
| Optimized latency 均值 | 39070.90 ms |
| Latency delta 均值 | 26887.60 ms |
| Baseline token 均值 | 277.80 |
| Optimized token 均值 | 5217.12 |
| Baseline cost 均值 | 0.000273 |
| Optimized cost 均值 | 0.001768 |
| Optimized tool_used 次数 | 10 |
| Optimized tool_calls 均值 | 5.80 |
| Optimized memory_hit 次数 | 0 |
| Optimized team_route 次数 | 0 |

补充说明：

- `total_token_delta` 与 `cost_delta` 如果直接按接口返回字段平均，会把 optimized 失败样本也算进来，数值会被压低。
- 因此更合理的做法是只在 baseline 和 optimized 都成功的样本上计算增量。

### 3.2 成功样本上的增量统计

以下统计仅基于 8 个 optimized 成功样本：

| 指标 | 结果 |
| --- | --- |
| Token delta 均值 | 4934.50 |
| Cost delta 均值 | 0.001490 |

解释：

- 在这个 `grounded-repo-qa` 场景下，optimized 路径明显更重。
- 它会进行更多工具调用和更长链路执行，因此平均 token、cost、latency 都高于 baseline。
- 但从推荐结果看，系统仍有 8 / 10 次判断 optimized 更值得保留，说明它在这个场景下更偏向“质量/可执行性优先”，而不是“低成本优先”。

### 3.3 基线 vs 优化链路收益对比

为了更直接体现“优化链路的收益”，下面把 baseline 与 optimized 成功样本做对照。

说明：

- baseline 统计基于 10 个成功样本
- optimized 统计基于 8 个成功样本
- 这里的“收益”重点看质量、可执行性和信息密度
- 这里的“代价”重点看 latency、token 和 cost

| 维度 | Baseline | Optimized | 相对变化 | 解读 |
| --- | ---: | ---: | ---: | --- |
| Groundedness Score | 0.34 | 0.81 | +138.24% | 优化链路显著提升了回答的 groundedness |
| Hallucination Risk | 0.62 | 0.2975 | -52.02% | 优化链路显著降低了幻觉风险 |
| Response Chars | 491.80 | 927.75 | +88.64% | 优化链路产出的信息密度更高 |
| Route Depth | 1 | 5 | +400.00% | 优化链路真实走了更深的执行链 |
| Tool Calls | 0 | 4.75 | 新增能力 | baseline 基本无工具执行，optimized 具备真实工具增强能力 |
| Latency(ms) | 12183.30 | 38494.25 | +215.96% | 质量收益是以显著时延增长换来的 |
| Tokens | 277.80 | 5217.12 | +1778.01% | 优化链路的 token 开销远高于 baseline |
| Cost | 0.000273 | 0.001768 | +547.20% | 优化链路的成本增幅也非常明显 |

从这张表可以直接得出结论：

- 优化链路的收益是真实存在的，而且主要体现在：
  - 更强 groundedness
  - 更低 hallucination risk
  - 更高信息密度
  - 更深、更完整的工具增强执行链
- 但优化链路的收益不是“免费”的，它当前是以显著增加 latency、token 和 cost 为代价换来的。

因此，这套优化链路当前更适合被描述为：

- “高质量、高执行能力路径”

而不是：

- “低成本、低时延优化路径”

### 3.4 Winner 维度分析

Value Lab 返回的 `winning_dimensions` 也能直接体现优化链路到底赢在什么地方。

10 次真实实验统计如下：

| Winning Dimension | 次数 | 含义 |
| --- | ---: | --- |
| `agent-selection` | 10 | 优化链路在 Agent 选择层始终有稳定收益 |
| `tool-aware` | 10 | 优化链路在工具感知与工具使用层始终体现收益 |
| `groundedness` | 8 | 大多数情况下优化链路在 groundedness 上更优 |
| `hallucination-control` | 8 | 大多数情况下优化链路在幻觉控制上更优 |
| `model-routing` | 2 | 模型路由收益存在，但不是主导收益来源 |

这意味着：

- 优化链路最稳定的收益，不是“模型换得更便宜”或“响应更快”，
- 而是“更好的 Agent 选择 + 更强的工具感知 + 更高的 groundedness + 更低的 hallucination risk”。

### 3.5 失败样本分析

optimized 失败共 2 次，失败原因一致：

- `exceeded maximum tool iterations (10)`

对应 run：

- Run 5
- Run 9

这说明当前 Value Lab 优化链路在复杂工具循环场景下已经触达 MCP 工具迭代上限，属于真实系统约束暴露，而不是接口层假失败。

### 3.6 单次结果明细

| Run | 状态 | Winner | Baseline Latency(ms) | Optimized Latency(ms) | Latency Delta(ms) | Baseline Tokens | Optimized Tokens | Optimized Tool Calls | 备注 |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| 1 | 成功 | optimized | 9202 | 22318 | 13116 | 218 | 4000 | 2 | optimized 成功 |
| 2 | 成功 | optimized | 13627 | 48641 | 35014 | 313 | 10482 | 7 | optimized 成功 |
| 3 | 成功 | optimized | 13386 | 45246 | 31860 | 320 | 3977 | 7 | optimized 成功 |
| 4 | 成功 | optimized | 12320 | 56771 | 44451 | 318 | 5906 | 9 | optimized 成功 |
| 5 | 成功 | baseline | 12074 | 39078 | 27004 | 267 | - | 10 | optimized 触发 tool iteration 上限 |
| 6 | 成功 | optimized | 10650 | 28663 | 18013 | 244 | 4137 | 2 | optimized 成功 |
| 7 | 成功 | optimized | 12370 | 26451 | 14081 | 280 | 4065 | 2 | optimized 成功 |
| 8 | 成功 | optimized | 14573 | 49742 | 35169 | 308 | 5023 | 7 | optimized 成功 |
| 9 | 成功 | baseline | 11322 | 43677 | 32355 | 250 | - | 10 | optimized 触发 tool iteration 上限 |
| 10 | 成功 | optimized | 12309 | 30122 | 17813 | 260 | 4147 | 2 | optimized 成功 |

## 4. Agent Router 统计结果

### 4.1 总体统计

| 指标 | 结果 |
| --- | --- |
| 实际完成采样 | 10 / 10 |
| 业务成功次数 | 6 / 10 |
| 客户端观测时延均值 | 41720.40 ms |
| 客户端观测时延标准差 | 3992.48 ms |
| 最小时延 | 37121 ms |
| 最大时延 | 48219 ms |
| Selected Agent 分布 | `code-specialist: 10` |
| Selected Candidate 分布 | `deepseek-primary/deepseek-chat: 6`，`无候选返回: 4` |
| Route Source 分布 | `score-router: 10` |
| Memory Influence 分布 | `reused: 10` |
| 命中 Team Route 次数 | 0 |
| Candidate failover 均值 | 1.00 |
| Assistant 文本长度均值 | 273.50 chars |

### 4.2 结果解读

这组数据说明：

- 10 次请求全部稳定命中 `code-specialist`
- 路由来源全部是 `score-router`
- memory influence 全部是 `reused`

这意味着在当前热启动状态下，Route Memory 已经稳定参与了真实路由决策。

同时：

- 6 次成功返回了 `deepseek-primary/deepseek-chat`
- 4 次失败没有返回最终候选模型

失败并不是路由层没选出来，而是执行链在工具循环阶段中断。

需要强调的是：

- Agent Router 这组实验更像“优化链路稳定性验证”
- 它证明了优化链路在真实运行下能够稳定触发：
  - score-based routing
  - memory reuse
  - 固定的 expert agent 选择

但它不是一个像 Value Lab 那样内置 baseline/optimized 双链路对照的接口。

所以在“收益”表达上：

- Value Lab 负责回答：优化链路相比基线到底赢了什么
- Agent Router 负责回答：这些优化机制在真实请求下是否稳定生效

### 4.3 失败样本分析

业务失败共 4 次，失败原因完全一致：

- `exceeded maximum tool iterations (10)`

对应 run：

- Run 2
- Run 3
- Run 4
- Run 8

也就是说，当前 Agent Router 的主要真实瓶颈不是“选错 Agent”或“选错模型”，
而是选中 `code-specialist` 后，其工具增强执行链在部分请求上触达了工具迭代上限。

### 4.4 单次结果明细

| Run | 状态 | Client Latency(ms) | Selected Agent | Selected Candidate | Route Source | Memory Influence | Failovers | Assistant Chars |
| --- | --- | ---: | --- | --- | --- | --- | ---: | ---: |
| 1 | 成功 | 47317 | code-specialist | deepseek-primary/deepseek-chat | score-router | reused | 1 | 486 |
| 2 | 业务失败 | 38800 | code-specialist | - | score-router | reused | 1 | 0 |
| 3 | 业务失败 | 38608 | code-specialist | - | score-router | reused | 1 | 0 |
| 4 | 业务失败 | 39492 | code-specialist | - | score-router | reused | 1 | 0 |
| 5 | 成功 | 44663 | code-specialist | deepseek-primary/deepseek-chat | score-router | reused | 1 | 464 |
| 6 | 成功 | 37818 | code-specialist | deepseek-primary/deepseek-chat | score-router | reused | 1 | 454 |
| 7 | 成功 | 48219 | code-specialist | deepseek-primary/deepseek-chat | score-router | reused | 1 | 427 |
| 8 | 业务失败 | 42894 | code-specialist | - | score-router | reused | 1 | 0 |
| 9 | 成功 | 37121 | code-specialist | deepseek-primary/deepseek-chat | score-router | reused | 1 | 456 |
| 10 | 成功 | 42272 | code-specialist | deepseek-primary/deepseek-chat | score-router | reused | 1 | 448 |

## 5. 综合结论

基于这 20 次真实调用，可以提炼出几个较清晰的结论：

### 5.1 Value Lab 侧

- 在 `grounded-repo-qa` 这个场景下，optimized 链路大多数时候被系统判定为更优
- 但 optimized 的真实代价也非常明显：
  - 更长 latency
  - 更高 token 消耗
  - 更高 cost
  - 更多 tool calls
- 因此，它体现的是“质量/执行能力换成本”的价值，而不是“优化后一定更快更省”

### 5.2 Agent Router 侧

- 路由决策本身表现得很稳定：
  - Agent 选择稳定
  - Route Source 稳定
  - Memory Influence 稳定
- 当前主要暴露出来的真实问题不是路由层，而是工具增强执行层：
  - 4 / 10 次业务失败
  - 失败原因全部是工具迭代次数上限

### 5.3 当前系统最值得继续优化的方向

从这轮真实实验看，最优先的优化点不是再改路由打分，而是：

1. 降低工具调用链的循环失控概率
2. 优化 MCP tool loop 的终止策略
3. 在 `code-specialist` 这类 Agent 上进一步约束工具使用策略
4. 在 Value Lab 场景中平衡“质量提升”和“时延/成本膨胀”

## 6. 原始数据文件

- `docs/real_experiment_value_lab_runs.json`
- `docs/real_experiment_agent_router_runs.json`
