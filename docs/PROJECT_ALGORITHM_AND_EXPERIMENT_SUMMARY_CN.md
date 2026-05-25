# LLMux 核心算法原理与实验数据总述

## 1. 文档目标

本文档用于集中总结两类内容：

1. 项目的核心算法、核心原理与研究价值主张
2. 目前已经完成的所有关键实验对比数据与结论

它是一份“总述版”文档，适合直接用于：

- 论文实验章节总览
- 中期汇报 / 答辩 PPT
- 项目总述
- 后续写摘要与结论时的统一引用材料

## 2. 项目核心价值主张

LLMux 的论文主张可以概括为 5 个核心创新：

1. 记忆增强的分层路由  
   Route Memory + Team Memory

2. 预测式调度算法  
   基于负载预测规避 RPM / TPM 限流碰撞

3. 工具增强 Expert Agent  
   通过 MCP 集成真实外部工具能力

4. 团队演练与经验沉淀  
   通过 rehearsal + Team Memory 实现持续优化

5. 多约束多目标协同优化  
   在能力约束、限流约束、权限约束和运行态约束下，同时优化多个目标

## 3. 项目核心原理与算法框架

## 3.1 网关即平台控制层

LLMux 不把自己定位为“简单代理”，而是定位为“平台控制层”。

它的核心执行流水线是：

1. Ingress
2. Validation
3. Governance Evaluate
4. Route Selection
5. Upstream Execution
6. Response Handling
7. Governance Account
8. Observability

这一点意味着：

- 协议兼容
- 治理控制
- 路由决策
- 使用量记账
- 可观测性

都不是散落在业务层，而是统一由网关承担。

## 3.2 分层路由原理

项目在研究层的关键扩展，是把“模型路由”提升为“三层路由”：

### 第一层：Team Routing

根据请求语义复杂度、任务类型、Team Registry 与 Team Memory，判断是否需要由某个 Expert Agent Team 接管任务。

### 第二层：Agent Routing

在团队内部或单 Agent 场景中，结合：

- Agent Registry
- Route Memory
- 工具绑定
- 能力匹配
- 时延预算

选择最合适的 lead Agent。

### 第三层：Candidate Model Scheduling

在候选模型池中，根据：

- 静态权重
- 当前 RPM / TPM
- 预测 RPM / TPM
- cooldown 状态
- latency fit

选择当前最合适的底层模型。

## 3.3 记忆增强机制

### Route Memory

把历史 query、路径、结果沉淀为可复用的路由经验。

作用：

- 提升同类请求路由一致性
- 降低重复决策成本
- 为高质量历史路径提供优先复用依据

### Team Memory

把历史团队协作过程与结果沉淀为团队级经验。

作用：

- 为复杂任务的 team 选择提供经验支持
- 降低团队协作的试错开销
- 支持 rehearsal 结果沉淀

## 3.4 预测式调度算法

项目当前的候选模型调度不是单纯轮询，而是基于运行态的预测式调度。

核心思想：

- 统计近一段时间内的 RPM / TPM 使用量
- 预测未来窗口内的负载压力
- 结合安全边际、时延预算和候选权重做综合评分
- 在请求发送前主动规避高风险候选模型

这使系统从：

- “失败后再切换”

演进为：

- “发送前规避高风险候选”

## 3.5 工具增强 Expert Agent

项目通过 MCP 给 Agent 注入真实工具能力，Agent 不再只是 prompt 里声明“我会做什么”，
而是真正能：

- 读文件
- 列目录
- 抓取网页
- 调数据库
- 调浏览器
- 组合外部工具

这使 Agent 从“语言层代理”变成“工具增强执行体”。

## 3.6 团队演练与经验沉淀

系统引入 rehearsal 机制，让团队可以用演练任务不断沉淀经验。

这意味着：

- team routing 不再只靠即时决策
- 历史高分团队路径可以被复用
- 团队协作有机会从一次次真实请求中持续改进

## 3.7 多约束多目标协同优化

项目不是做单目标最优化，而是同时关注：

- 请求成功率
- 限流碰撞概率
- 平均响应时延
- 模型资源利用率

这使其研究目标更接近一个多目标、受约束的调度问题，而不是简单策略匹配。

---

## 4. 已完成实验总览

目前已经完成并得到实测数据的实验主要有 6 组：

1. Exp 1.1：基础网关 Benchmark
2. Exp 2.1：记忆增强路由实验
3. Exp 2.2：预测式调度实验
4. Value Lab：优化链路对照实验
5. Agent Router：真实稳定性实验
6. Exp 2.3：工具增强机制贡献实验

## 4.1 论文可直接引用的总表

| 实验 | 类型 | 对照关系 | 样本规模 | 关键实测结果 | 当前能证明什么 |
| --- | --- | --- | --- | --- | --- |
| Exp 1.1 基础网关 Benchmark | 基础网关层 | LLMux vs LiteLLM | README 已公开基准 | 吞吐约 8 倍优势，平均延迟约 7.9 倍优势，P99 约 9.2 倍优势 | Go 网关底座具备显著性能优势 |
| Exp 2.1 记忆增强路由 | 核心创新验证 | 无记忆路由 vs 记忆增强路由 | 8 条相似/相关请求 | 两组成功率均为 100%；基线 `memory_influence=none:8`；增强组 `reused:7/8`；平均 consulted memories `0 -> 0.875` | 记忆增强机制已经真实参与路由，但当前数据更能证明“记忆被用上了”，还不足以严格证明“端到端更快” |
| Exp 2.2 预测式调度 | 核心创新验证 | passive vs greedy-current vs predictive | 8 条调度相关请求 | passive 成功率 100%，平均时延 14854.62 ms；greedy-current 成功率 100%，平均时延 16273.38 ms；predictive 成功率 87.5%，平均时延 8824.62 ms | 预测式调度在当前样本下展现出更低平均时延，但成功率被单次 tool loop 失败拖低，说明调度收益已出现但还需更强样本验证 |
| Value Lab 真实对照 | 优化链路收益验证 | Baseline vs Optimized | 10 次真实调用 | optimized 8/10 被推荐；Groundedness `0.34 -> 0.81`；Hallucination Risk `0.62 -> 0.2975`；Route Depth `1 -> 5`；Latency `+215.96%`；Tokens `+1778.01%`；Cost `+547.20%` | 优化链路有真实质量收益，但不是低成本低时延路径 |
| Agent Router 真实稳定性实验 | 路由稳定性验证 | 热态连续调用 | 10 次真实调用 | `code-specialist` 10/10 命中；`score-router` 10/10 命中；`memory_influence=reused` 10/10；业务成功 6/10 | 路由层已经稳定，Route Memory 已真实参与路由 |
| Exp 2.3 工具增强机制贡献实验 | 核心创新验证 | 无工具 Agent vs 工具增强 Agent | 5 任务对照 | 语义成功率 `40% -> 60%`；关键词覆盖均值 `0.40 -> 0.60`；平均 Tool Calls `0 -> 5.2`；Tool 使用率 `0% -> 100%` | MCP 工具增强真实改变了 Agent 的执行行为，并提升了任务完成质量 |

---

## 5. 各实验详细结论

## 5.1 Exp 1.1：基础网关 Benchmark

### 实测数据

- 吞吐：LLMux 相比 LiteLLM 约 `8x`
- 平均延迟：LLMux 相比 LiteLLM 约 `7.9x` 更低
- P99：LLMux 相比 LiteLLM 约 `9.2x` 更优

### 结论

这一组实验已经可以稳定证明：

- Go 网关底座的工程性能优势成立
- 在高并发代理场景下，LLMux 的基础框架开销远低于 Python 对照基线

### 局限

这一组实验不能直接证明：

- 记忆增强路由的价值
- 预测式调度的价值
- 工具增强 Agent 的价值

它证明的是“底座性能”，不是“智能路由能力”。

## 5.2 Exp 2.1：记忆增强路由实验

### 实测设置

- 样本规模：8 条语义相近或相关请求
- 对照组：`baseline-memoryless-routing`
- 实验组：`our-memory-augmented-routing`
- 运行接口：`/control/conversation/chat`

### 实测结果

#### 基线组

- 成功率：`100%`
- 平均时延：`5275.63 ms`
- `memory_influence = none: 8`
- 平均 consulted memories：`0`

#### 记忆增强组

- 成功率：`100%`
- 平均时延：`7965.00 ms`
- `memory_influence = reused: 7 / 8`
- 平均 consulted memories：`0.875`

### 结论

这组实验已经可以明确证明：

- 记忆增强机制不是“摆设”
- Route Memory 已经真实参与了路由决策
- 在 8 个样本里，增强组有 7 个样本出现了 `reused`

也就是说，当前系统已经具备：

- 对相似请求复用历史路径的真实能力

### 当前边界

这组实验还不能严格证明：

- 记忆增强后的端到端时延更低

原因是当前测到的是：

- 整体端到端耗时

而不是：

- 纯路由决策耗时

增强组里还出现了一个 `28170 ms` 的大 outlier，导致平均值被明显拉高。

因此，这组实验当前最适合支撑的论点是：

- “记忆被真实用上了”

而不是：

- “记忆已经严格证明能降低端到端总时延”

## 5.3 Exp 2.2：预测式调度实验

### 实测设置

- 样本规模：8 条调度相关请求
- 三组对照：
  - `baseline-passive-fallback`
  - `baseline-greedy-current`
  - `our-predictive-scheduling`
- 运行接口：`/control/conversation/chat`

### 实测结果

#### Passive Fallback

- 成功率：`100%`
- 平均时延：`14854.62 ms`

#### Greedy Current

- 成功率：`100%`
- 平均时延：`16273.38 ms`

#### Predictive Scheduling

- 成功率：`87.5%`
- 平均时延：`8824.62 ms`

### 结论

这组实验呈现出一个比较有价值但也必须谨慎解释的现象：

- 预测式调度在当前样本上表现出最低的平均时延
- 相比 passive 和 greedy-current，有明显时延优势

这说明：

- 预测式调度至少已经展现出“潜在收益”
- 它不是纯概念设计，而是能在真实请求中改变模型选择结果

### 当前边界

预测组的成功率低于两个 baseline，原因不是限流碰撞，而是：

- 其中 1 次失败为 `exceeded maximum tool iterations (10)`

这意味着当前实验还不能写成：

- “预测式调度在成功率和时延上都全面优于 baseline”

更准确的表达应是：

- 预测式调度已经在当前样本上展现出更低时延
- 但其最终成功率仍然受到工具增强执行层瓶颈影响

也就是说，这组实验当前证明的是：

- 调度收益已经出现

但尚未完全隔离：

- 路由 / 调度层收益
- 与工具执行链失败之间的耦合

## 5.4 Value Lab：优化链路对照实验

### 实测设置

- 场景：`grounded-repo-qa`
- 样本规模：10 次真实调用
- 对照方式：Baseline vs Optimized

### 实测结果

#### 推荐结果

- optimized 被推荐为 winner：`8 / 10`
- baseline 被推荐为 winner：`2 / 10`

#### 收益项

- Groundedness Score：`0.34 -> 0.81`
- Hallucination Risk：`0.62 -> 0.2975`
- Response Chars：`491.80 -> 927.75`
- Route Depth：`1 -> 5`
- `agent-selection` 在 winning dimensions 中出现 `10` 次
- `tool-aware` 在 winning dimensions 中出现 `10` 次
- `groundedness` 在 winning dimensions 中出现 `8` 次
- `hallucination-control` 在 winning dimensions 中出现 `8` 次

#### 代价项

- Latency：`+215.96%`
- Tokens：`+1778.01%`
- Cost：`+547.20%`

### 结论

这组实验已经足以说明：

- 优化链路的收益是真实存在的
- 其收益主要体现在：
  - 更好的 Agent 选择
  - 更强的工具感知与工具使用
  - 更高的 groundedness
  - 更低的 hallucination risk
  - 更高的信息密度

但是，这条链路当前并不是“更快更便宜”的优化，而是：

- 用更高的执行成本换更高的质量和更强的可执行性

### 当前暴露的问题

optimized 失败 2 次，失败原因完全一致：

- `exceeded maximum tool iterations (10)`

说明当前最大瓶颈不是路由失效，而是：

- 工具增强链过长时容易触达 MCP tool loop 上限

## 5.5 Agent Router：真实稳定性实验

### 实测设置

- 样本规模：10 次真实调用
- 场景：真实 `conversation/chat` 接口

### 实测结果

- 业务成功：`6 / 10`
- 平均客户端时延：`41720.40 ms`
- Selected Agent：`code-specialist`，`10 / 10`
- Route Source：`score-router`，`10 / 10`
- Memory Influence：`reused`，`10 / 10`
- 成功样本中的 Selected Candidate：全部为 `deepseek-primary/deepseek-chat`

### 结论

这组实验说明：

1. 路由层本身已经非常稳定  
   Agent 选择、Route Source 和 Memory Influence 都没有明显抖动。

2. Route Memory 已经真实参与了路由  
   因为 `memory_influence=reused` 在 10 次中全命中。

3. 当前主要工程瓶颈不在“选错 Agent”或“选错模型”  
   失败主因是工具增强执行层的问题。

### 当前暴露的问题

4 次失败全部是：

- `exceeded maximum tool iterations (10)`

所以这组实验当前最能证明的是：

- 路由稳定性已经成立
- 真正需要优化的是 tool loop 控制

## 5.6 Exp 2.3：工具增强机制贡献实验

### 实测设置

- 样本规模：5 个任务
- 对照组：无工具 Agent
- 实验组：工具增强 Agent
- 任务类型：围绕当前仓库的真实文件系统分析任务

### 实测结果

#### 无工具基线

- API 成功率：`100%`
- 语义成功率：`40%`
- 关键词覆盖均值：`0.40`
- 平均 Tool Calls：`0`
- Tool 使用率：`0%`

#### 工具增强链路

- API 成功率：`60%`
- 语义成功率：`60%`
- 关键词覆盖均值：`0.60`
- 平均 Tool Calls：`5.2`
- Tool 使用率：`100%`
- `max_iterations_hit_count = 2`

### 结论

这组实验已经直接说明：

- 工具增强不是“装饰性能力”
- MCP 工具增强确实改变了 Agent 的执行行为
- 在当前任务集下，工具增强链路在语义成功率和事实命中率上优于无工具基线

也就是说，它已经可以支撑一个明确论断：

- 工具增强带来的不是“更会说”
- 而是“更能做”

### 当前局限

这组任务集目前主要依赖：

- filesystem 工具

因此它能证明“工具增强机制有效”，但还不能完全代表：

- 数据库类工具
- 浏览器自动化
- 跨工具协作

另外，增强组成功率下降的原因并不是工具增强无效，而是：

- tool loop 风险被真实暴露出来了

---

## 6. 当前可以稳定得出的总体判断

综合现有所有实测结果，目前可以稳定得出以下结论：

1. LLMux 的 Go 网关底座已经具有明显性能优势。  
2. Route Memory 已经通过 Exp 2.1 证明会真实参与路由决策。  
3. 预测式调度已通过 Exp 2.2 展现出潜在时延优势，但仍受工具执行层干扰。  
4. 优化链路的收益已经在真实 Value Lab 对照里被观察到。  
5. 这种收益主要体现在质量、groundedness、工具感知和执行深度，而不是成本和时延。  
6. Agent Router 的路由层已经表现出较强稳定性。  
7. MCP 工具增强机制已经通过 Exp 2.3 证明能提升任务完成质量。  
8. 当前最主要的工程瓶颈不是路由算法本身，而是工具增强执行链的迭代控制。  

---

## 7. 当前仍然缺什么

尽管已经有了多组真实数据，但论文目前还没有完成“全套系统性验证”。

当前仍缺：

- Exp 2.4：团队演练与经验沉淀对照实验
- Exp 2.5：多约束多目标协同优化对照实验
- Exp 3.x：长期稳定性与并发压力测试
- Exp 4.1：与主流方案的完整对标

换句话说：

- 论文已经有“真实证据”
- 但还没有形成“完整闭环”

---

## 8. 相关文档与原始数据

### 项目与理论总结

- `docs/PROJECT_CORE_THEORY_AND_EXPERIMENTS_CN.md`

### 实验体系设计

- `docs/EXPERIMENT_SYSTEM_DESIGN_CN.md`

### 实测结果总结

- `docs/REAL_EXPERIMENT_RESULTS_CN.md`

### 原始数据

- `docs/real_experiment_value_lab_runs.json`
- `docs/real_experiment_agent_router_runs.json`
- `docs/experiment_runs/exp21_20260417_011932.json`
- `docs/experiment_runs/exp22_20260417_012306.json`
- `docs/experiment_runs/exp23_20260417_000904.json`

### 工具增强实验报告

- `docs/experiment_runs/exp21_20260417_011932.md`
- `docs/experiment_runs/exp22_20260417_012306.md`
- `docs/experiment_runs/exp23_20260417_000904.md`
