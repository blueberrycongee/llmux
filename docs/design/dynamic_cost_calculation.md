# 动态成本计算与定价系统设计文档 (Dynamic Cost Calculation & Pricing System)

## 进度 (Progress)
- [x] Phase 1: 基础架构 (Atomic Step 1) - `pkg/pricing` implemented with tests.
- [x] Phase 2: 集成 Router (Atomic Step 2) - `routers/cost.go` integrated with `PriceRegistry`.
- [ ] Phase 3: 配置与扩展 (Atomic Step 3)


## 1. 开发目标 (Development Goal)

当前 `llmux` 的成本计算依赖于硬编码的 `DefaultCostPerToken = 5.0` (Magic Number) 以及用户在配置文件中手动填写的 `InputCostPerToken`。这种方式维护成本高、易出错且无法适应 LLM 价格频繁变动的现状。

本设计的目标是：
1.  **消除 Magic Number**：移除所有硬编码的默认价格。
2.  **引入数据驱动定价**：建立统一的 `PriceRegistry`，支持从 JSON/YAML 数据源加载主流模型的最新价格。
3.  **对齐业界标准**：参考 `litellm` 的实现，支持细粒度的计费模型（Input/Output/Cache/Time-based）。
4.  **保持高性能**：在热路径（Router Pick）中实现零延迟的价格查询。

---

## 2. 其他项目的优秀实现 (Reference: litellm)

通过阅读 `litellm` 源码 (`litellm/cost_calculator.py` 和 `model_prices_and_context_window.json`)，我们总结出以下最佳实践：

### 2.1 数据源管理
`litellm` 维护了一个庞大的 `model_prices_and_context_window.json` 文件，包含数千个模型的元数据。
结构示例：
```json
"gpt-4o": {
    "litellm_provider": "openai",
    "mode": "chat",
    "input_cost_per_token": 0.000005,
    "output_cost_per_token": 0.000015,
    "max_tokens": 4096,
    "max_input_tokens": 128000
}
```

### 2.2 计算逻辑 (`cost_calculator.py`)
*   **多维计费**：不仅支持 Token 计费，还支持：
    *   **Per Character**: Vertex AI 等。
    *   **Per Second**: Audio/Speech 模型。
    *   **Per Image**: DALL-E 等。
*   **动态匹配**：支持 `provider/model` 格式的精确匹配，也支持通配符或前缀匹配。
*   **Tier 支持**：支持 OpenAI 的 Service Tier (Scale/Standard) 定价差异。

---

## 3. 我们的架构背景 (Architecture Background)

### 3.1 现状 (`routers/cost.go`)
目前 `llmux` 的 `CostRouter` 逻辑非常原始：
```go
// routers/cost.go
const DefaultCostPerToken = 5.0 // [Code Smell] Magic Number

func (r *CostRouter) Pick(...) {
    // ...
    if inputCost == 0 { inputCost = DefaultCostPerToken }
    // 简单的加法，未考虑 Input/Output 比例
    totalCost := inputCost + outputCost 
    // ...
}
```

### 3.2 痛点
1.  **配置繁琐**：用户必须为每个 Deployment 手动查价格并填入 Config，否则就会回退到错误的 `5.0`。
2.  **路由不准**：`input + output` 的简单求和无法反映真实场景（通常 Input >> Output，或者 Output 价格 >> Input）。
3.  **无法扩展**：无法支持 Image/Audio 等非 Token 计费模型。

---

## 4. 我们的设计思路 (Design Strategy)

我们将采用 **"内置默认库 + 外部覆盖 + 自动更新"** 的三层架构。

### 4.1 核心组件：`PriceRegistry`
在 `pkg/pricing` 包中实现一个单例或注入式的 `Registry`。

```go
type ModelPrice struct {
    Provider          string  `json:"provider"`
    InputCostPerToken float64 `json:"input_cost_per_token"`
    OutputCostPerToken float64 `json:"output_cost_per_token"`
    // 支持缓存命中价格 (Anthropic/DeepSeek)
    CacheReadCostPerToken float64 `json:"cache_read_cost_per_token"`
    CacheWriteCostPerToken float64 `json:"cache_write_cost_per_token"`
    // 支持非 Token 计费
    CostPerRequest    float64 `json:"cost_per_request"`
}

type Registry struct {
    prices map[string]ModelPrice // Key: "model_name" or "provider/model_name"
    mu     sync.RWMutex
}
```

### 4.2 数据加载策略
1.  **Embedded Default**: 将精简版的 `model_prices.json` (包含 Top 50 主流模型) 编译进二进制文件 (`embed`).
2.  **Local Override**: 启动时检查 `ConfigDir/model_prices.json`，如有则覆盖内置配置。
3.  **Remote Fetch (Future)**: 预留接口，允许后台任务定期从 GitHub (如 `litellm` 的仓库) 拉取最新 JSON。

### 4.3 路由算法优化
改造 `CostRouter` 的比较逻辑。不再简单相加，而是基于**加权预估**：

```go
// 假设一个典型的 Chat 交互比例，或者使用 RequestContext 中的 EstimatedInputTokens
const DefaultInputOutputRatio = 3.0 // Input 是 Output 的 3 倍

func CalculateScore(price ModelPrice) float64 {
    // 归一化评分，越低越好
    return price.InputCostPerToken * 3.0 + price.OutputCostPerToken * 1.0
}
```

---

## 5. 我们的测试具体需要考虑哪些 Edge Case

1.  **Unknown Model**: 当用户请求一个 Registry 中不存在的模型（如私有微调模型 `my-gpt-4-finetune`）时，应如何处理？
    *   *策略*：回退到 `DefaultUnknownPrice` (但不要是 5.0，而是一个合理的“昂贵”值以避免被优先选中)，或者允许 Config 中显式指定。
2.  **Provider Alias**: `azure/gpt-4` 和 `openai/gpt-4` 价格可能不同。
    *   *策略*：Registry Key 优先使用 `provider/model`，未命中则尝试 `model`。
3.  **Zero Cost Models**: Ollama / Local 模型价格为 0。
    *   *策略*：确保排序算法能正确处理 0 值（最优先）。
4.  **Currency Precision**: 浮点数精度问题（Go `float64` 对极小金额如 `0.0000001` 的处理）。
    *   *策略*：在内部计算时保持 `float64`，展示或计费时再做截断。

---

## 6. 我们的开发具体实施路径 (Implementation Path)

### Phase 1: 基础架构 (Atomic Step 1)
1.  创建 `pkg/pricing` 包。
2.  定义 `ModelPrice` 结构体。
3.  从 `litellm` 仓库复制一份 `model_prices.json`，精简后放入 `pkg/pricing/data/defaults.json`。
4.  实现 `Load()` 和 `GetPrice(model, provider)` 方法。

### Phase 2: 集成 Router (Atomic Step 2)
1.  修改 `routers/cost.go`。
2.  注入 `PriceRegistry`。
3.  移除 `DefaultCostPerToken = 5.0`。
4.  更新 `Pick` 逻辑使用 Registry 查询价格。

### Phase 3: 配置与扩展 (Atomic Step 3)
1.  在 `config.yaml` 中支持 `pricing_file` 路径配置。
2.  实现本地文件覆盖逻辑。

---

## 7. 验收标准 (Acceptance Criteria)

1.  **No Magic Numbers**: 代码库中不再出现 `DefaultCostPerToken = 5.0`。
2.  **Accurate Pricing**: 对于 `gpt-4o`, `claude-3-5-sonnet` 等主流模型，Router 能自动获取正确价格，无需人工配置。
3.  **Local Override**: 用户可以通过提供自定义 JSON 文件修改特定模型价格，且生效。
4.  **Fallback Safety**: 对于未知模型，系统不会 Panic，且能给出合理的默认行为（如日志警告）。
