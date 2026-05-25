# 模块实现 PlantUML 图集

本文档基于当前代码实现，整理了 5 个核心模块的 PlantUML 图。

为避免论文或 Markdown 预览中图片过于拥挤，以下图均采用“简化展示版”，只保留模块职责与主链路，不再展开函数级细节。

每个模块包含两类图：
- 架构图
- 链路时序图

## 1. 记忆管理模块

相关源码：
- `internal/api/conversation_endpoints.go`
- `internal/api/route_memory.go`
- `internal/api/team_memory.go`
- `internal/api/routing_profile.go`
- `internal/api/agent_team_endpoints.go`

### 1.1 架构图

```plantuml
@startuml
title 记忆管理模块架构图
skinparam componentStyle rectangle
skinparam defaultFontName "Microsoft YaHei"

[会话入口\nConversationChat] as Chat
[团队演练\nRehearseAgentTeam] as Rehearse
[路由记忆服务\n查询 / 统计 / 持久化] as RouteMemory
[团队记忆服务\n查询 / 统计 / 持久化] as TeamMemory
database "路由记忆存储\nconversationMemoryStore" as RouteStore
database "团队记忆存储\nteamMemoryStore" as TeamStore

Chat --> RouteMemory : 查询与写入
Chat --> TeamMemory : 团队场景查询与写入
Rehearse --> TeamMemory : 演练结果沉淀

RouteMemory --> RouteStore
TeamMemory --> TeamStore
@enduml
```

### 1.2 链路时序图

```plantuml
@startuml
title 记忆管理模块链路时序图
skinparam defaultFontName "Microsoft YaHei"

actor 用户 as User
participant "会话路由接口\nConversationChat" as Chat
participant "路由画像构建器\nbuildRoutingProfile" as Profile
participant "路由记忆服务" as RouteMemory
participant "团队记忆服务" as TeamMemory
database "路由记忆存储\nconversationMemoryStore" as RouteStore
database "团队记忆存储\nteamMemoryStore" as TeamStore

User -> Chat : POST /control/conversation/chat
Chat -> Profile : 构建 RoutingProfile
Profile --> Chat : RoutingProfile

Chat -> RouteMemory : 查询相似路由记忆
RouteMemory -> RouteStore : 读取并筛选
RouteStore --> RouteMemory : 候选记忆
RouteMemory --> Chat : 路由经验

opt 需要团队协作
  Chat -> TeamMemory : 查询相似团队记忆
  TeamMemory -> TeamStore : 读取并筛选
  TeamStore --> TeamMemory : 候选记忆
  TeamMemory --> Chat : 团队经验
end

Chat -> RouteMemory : 写入本次路由结果
RouteMemory -> RouteStore : 追加并整理记忆

opt 命中团队执行链路
  Chat -> TeamMemory : 写入团队执行结果
  TeamMemory -> TeamStore : 追加并整理记忆
end

opt 显式触发团队演练
  User -> TeamMemory : 提交演练结果
  TeamMemory -> TeamStore : 写入 rehearsal 记忆
end
@enduml
```

## 2. 分层路由模块

相关源码：
- `internal/api/conversation_endpoints.go`
- `internal/api/routing_profile.go`
- `internal/api/agent_team_endpoints.go`
- `internal/api/token_optimization.go`

### 2.1 架构图

```plantuml
@startuml
title 分层路由模块架构图
skinparam componentStyle rectangle
skinparam defaultFontName "Microsoft YaHei"

[请求理解\n意图识别 + 路由画像] as Profile
[单 Agent 路由\n显式 / 快路 / 评分 / 回退] as SingleRoute
[团队路由\n团队筛选 + lead 选择] as TeamRoute
[路由约束\n记忆 / 工具 / 鉴权] as Constraints
[执行准备] as Prepare
[候选模型执行] as Execute
[llmux Client] as Client

Profile --> SingleRoute
Profile --> TeamRoute
SingleRoute --> Constraints
TeamRoute --> Constraints
SingleRoute --> Prepare
TeamRoute --> Prepare
Prepare --> Execute
Execute --> Client
@enduml
```

### 2.2 链路时序图

```plantuml
@startuml
title 分层路由模块链路时序图
skinparam defaultFontName "Microsoft YaHei"

actor 用户 as User
participant "会话路由接口\nConversationChat" as Chat
participant "意图识别器\ninferConversationIntent" as Intent
participant "路由画像构建器\nbuildRoutingProfile" as Profile
participant "路由决策器" as Router
participant "团队路由器" as TeamRoute
participant "执行器" as Execute

User -> Chat : POST /control/conversation/chat
Chat -> Intent : 识别用户意图
Intent --> Chat : intent
Chat -> Profile : 构建 RoutingProfile
Profile --> Chat : RoutingProfile

Chat -> Router : 选择基础 Agent
alt 简单任务或已有明确目标
  Router --> Chat : 单 Agent 结果
else 需要团队协作
  Chat -> TeamRoute : 选择团队与 lead Agent
  TeamRoute --> Chat : 团队路由结果
end

Chat -> Execute : 执行选中的 Agent / Team
Execute --> Chat : response / failover trail
Chat --> User : AgentChatResponse
@enduml
```

## 3. 预测式调度模块

相关源码：
- `internal/api/conversation_endpoints.go`
- `internal/api/routing_profile.go`
- `internal/api/value_lab_endpoints.go`

### 3.1 架构图

```plantuml
@startuml
title 预测式调度模块架构图
skinparam componentStyle rectangle
skinparam defaultFontName "Microsoft YaHei"

[调度输入\n画像 + 候选模型 + 访问控制] as Input
[候选排序与筛选] as Rank
[预测式调度器] as Scheduler
database "调度状态存储\n使用量 / 冷却 / 健康" as StateStore
[模型调用] as ExecLLM
[llmux Client] as Client

Input --> Rank
Rank --> Scheduler
Scheduler --> StateStore : 读取 / 回写状态
Scheduler --> ExecLLM
ExecLLM --> Client
@enduml
```

### 3.2 链路时序图

```plantuml
@startuml
title 预测式调度模块链路时序图
skinparam defaultFontName "Microsoft YaHei"

participant "会话执行层\nConversationChat" as Chat
participant "预测式调度器" as Scheduler
database "调度状态存储" as StateStore
participant "模型调用器" as ExecLLM
participant "底层 llmux Client" as Client

Chat -> Scheduler : 传入候选模型与路由画像
Scheduler -> StateStore : 读取负载 / 冷却 / 健康状态
StateStore --> Scheduler : 候选状态

loop 按优先级尝试候选模型
  alt 候选不可用或预测超限
    Scheduler -> StateStore : 记录跳过原因
  else 候选可执行
    Scheduler -> ExecLLM : 发起模型调用
    ExecLLM -> Client : ChatCompletion
    Client --> ExecLLM : response or error
    alt 成功
      Scheduler -> StateStore : 回写使用量与健康状态
      Scheduler --> Chat : 返回响应
    else 限流或失败
      Scheduler -> StateStore : 写入冷却并切换下一个候选
    end
  end
end
@enduml
```

## 4. 工具增强模块

相关源码：
- `internal/api/conversation_endpoints.go`
- `internal/api/tool_presets.go`
- `internal/api/conversation_execution.go`
- `internal/api/token_optimization.go`
- `internal/mcp/config.go`
- `internal/mcp/interface.go`
- `internal/mcp/manager.go`
- `internal/mcp/tools.go`
- `internal/mcp/transport.go`
- `cmd/mcp-fetch/main.go`

### 4.1 架构图

```plantuml
@startuml
title 工具增强模块架构图
skinparam componentStyle rectangle
skinparam defaultFontName "Microsoft YaHei"

database "工具市场\n预设与工具定义" as Marketplace
[工具解析\n按画像筛选可用工具] as ResolveTools
[MCP 管理器\n列举工具 / 执行调用] as Manager
[LLM + Tool 执行闭环] as ExecLoop
[MCP 客户端] as MCPClients
[llmux Client] as Client

Marketplace --> ResolveTools
ResolveTools --> Manager
ExecLoop --> Manager
Manager --> MCPClients
ExecLoop --> Client
@enduml
```

### 4.2 链路时序图

```plantuml
@startuml
title 工具增强模块链路时序图
skinparam defaultFontName "Microsoft YaHei"

actor 用户 as User
participant "会话路由接口\nConversationChat" as Chat
participant "路由画像构建器\nbuildRoutingProfile" as Profile
participant "工具解析器" as Resolve
participant "MCP 管理器\nMCPManager" as Manager
participant "LLM + Tool 闭环执行器\nexecuteConversationChatCompletionWithTrace" as ExecLoop
participant "底层 llmux Client" as Client
participant "目标 MCP 客户端" as MCPClient

User -> Chat : POST /control/conversation/chat
Chat -> Profile : 推断 required tools
Profile --> Chat : RoutingProfile
Chat -> Resolve : 解析工具市场项
Resolve --> Chat : 可用工具清单
Chat -> Manager : 获取过滤后的工具定义
Manager -> MCPClient : 列举已连接工具
MCPClient --> Manager : 工具定义

Chat -> ExecLoop : 携带工具清单执行
loop 直到没有 tool call 或达到最大迭代数
  ExecLoop -> Client : ChatCompletion(messages, tools)
  Client --> ExecLoop : assistant response
  alt 返回 tool_calls
    ExecLoop -> Manager : ExecuteToolCalls(tool_calls)
    Manager -> MCPClient : CallTool(arguments)
    MCPClient --> Manager : ToolExecutionResult
  else 返回最终回复
    ExecLoop --> Chat : final response + trace
  end
end

Chat --> User : AgentChatResponse
@enduml
```

## 5. 团队演练与经验沉淀模块

相关源码：
- `internal/api/agent_team_endpoints.go`
- `internal/api/team_memory.go`
- `internal/api/conversation_endpoints.go`
- `internal/api/routing_profile.go`

### 5.1 架构图

```plantuml
@startuml
title 团队演练与经验沉淀模块架构图
skinparam componentStyle rectangle
skinparam defaultFontName "Microsoft YaHei"

[团队演练入口\nRehearseAgentTeam] as Rehearse
[团队装载\n读取团队与成员] as TeamLoader
[演练执行\n逐个 Agent 运行] as Execute
[结果评分与记忆沉淀] as PersistTeam
database "团队注册表\nagentTeamStore / conversationAgentStore" as TeamStore
database "团队记忆存储\nteamMemoryStore" as MemoryStore
[llmux Client] as Client

Rehearse --> TeamLoader
TeamLoader --> TeamStore
Rehearse --> Execute
Execute --> Client
Rehearse --> PersistTeam
PersistTeam --> MemoryStore
@enduml
```

### 5.2 链路时序图

```plantuml
@startuml
title 团队演练与经验沉淀模块链路时序图
skinparam defaultFontName "Microsoft YaHei"

actor 用户 as User
participant "团队演练接口\nRehearseAgentTeam" as Rehearse
participant "团队装载器" as TeamLoader
participant "候选执行器\nexecuteConversationWithCandidates" as Execute
participant "团队结果评分器\nscoreTeamOutcome" as Score
participant "团队记忆持久化器" as PersistTeam
database "团队注册表" as TeamStore
database "团队记忆存储\nteamMemoryStore" as MemoryStore
participant "底层 llmux Client" as Client

User -> Rehearse : POST /control/conversation/rehearse-team
Rehearse -> TeamLoader : 加载团队与参与 Agent
TeamLoader -> TeamStore : 读取 team / agents
TeamStore --> TeamLoader : 团队配置
TeamLoader --> Rehearse : Agent 列表

loop 对每个参与 Agent 逐个演练
  Rehearse -> Execute : 执行候选模型链路
  Execute -> Client : ChatCompletion / failover
  Client --> Execute : response or error
  Execute --> Rehearse : TeamRehearsalStep
end

Rehearse -> Score : 聚合结果并评分
Score --> Rehearse : outcome score
Rehearse -> PersistTeam : 沉淀本次团队经验
PersistTeam -> MemoryStore : 写入并整理团队记忆
PersistTeam --> Rehearse : 返回最佳匹配团队记忆
Rehearse --> User : AgentTeamRehearsalResponse
@enduml
```
