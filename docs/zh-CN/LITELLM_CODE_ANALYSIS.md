# LiteLLM 代码深度分析报告

## 一、项目整体架构

```
litellm/
├── litellm/                    # 核心库
│   ├── llms/                   # 🔥 Provider 适配层 (100+ 个)
│   │   ├── base_llm/           # 基类定义
│   │   ├── openai/             # OpenAI 实现
│   │   ├── anthropic/          # Anthropic 实现
│   │   ├── azure/              # Azure OpenAI
│   │   ├── gemini/             # Google Gemini
│   │   └── ...                 # 其他 90+ providers
│   ├── types/                  # 类型定义 (Pydantic models)
│   ├── proxy/                  # Proxy Server (LLM Gateway)
│   ├── router.py               # 路由 & 负载均衡
│   ├── router_strategy/        # 路由策略实现
│   ├── router_utils/           # 路由工具函数
│   ├── exceptions.py           # 统一异常定义
│   ├── utils.py                # 工具函数
│   └── cost_calculator.py      # 成本计算
├── model_prices_and_context_window.json  # 💰 价格表 (直接可用)
└── tests/                      # 测试用例
```

## 二、核心设计模式分析

### 1. Provider Adapter 模式

LiteLLM 使用 BaseConfig 抽象基类定义统一接口，每个 Provider 继承并实现：

```python
# litellm/llms/base_llm/chat/transformation.py
class BaseConfig(ABC):
    
    @abstractmethod
    def get_supported_openai_params(self, model: str) -> list:
        """返回该 Provider 支持的 OpenAI 参数"""
        pass

    @abstractmethod
    def map_openai_params(self, non_default_params, optional_params, model, drop_params) -> dict:
        """将 OpenAI 格式参数映射为 Provider 特定格式"""
        pass

    @abstractmethod
    def transform_request(self, model, messages, optional_params, litellm_params, headers) -> dict:
        """转换请求体"""
        pass

    @abstractmethod
    def transform_response(self, model, raw_response, model_response, ...) -> ModelResponse:
        """转换响应体为统一格式"""
        pass

    @abstractmethod
    def validate_environment(self, headers, model, messages, ...) -> dict:
        """验证环境变量、设置 headers"""
        pass

    @abstractmethod
    def get_error_class(self, error_message, status_code, headers) -> BaseLLMException:
        """返回对应的异常类"""
        pass
```

**Go 转换策略**：这个接口设计可以直接映射为 Go interface：

```go
type ProviderConfig interface {
    GetSupportedOpenAIParams(model string) []string
    MapOpenAIParams(params map[string]any, model string) map[string]any
    TransformRequest(model string, messages []Message, params map[string]any) (*Request, error)
    TransformResponse(raw *http.Response, model string) (*ModelResponse, error)
    ValidateEnvironment(headers map[string]string, apiKey string) error
    GetErrorClass(statusCode int, message string) error
}
```

### 2. 类型系统分析

**核心类型定义 (litellm/types/utils.py)**：

```python
# 请求类型
class AllMessageValues(TypedDict):  # OpenAI 消息格式
    role: str
    content: Union[str, List[ContentBlock]]
    
# 响应类型  
class ModelResponse:
    id: str
    object: str = "chat.completion"
    created: int
    model: str
    choices: List[Choices]
    usage: Usage

class Choices:
    finish_reason: str
    index: int
    message: Message

class Usage:
    prompt_tokens: int
    completion_tokens: int
    total_tokens: int

# 流式响应
class GenericStreamingChunk(TypedDict):
    text: str
    tool_use: Optional[ChatCompletionToolCallChunk]
    is_finished: bool
    finish_reason: str
    usage: Optional[ChatCompletionUsageBlock]
```

**Go 转换**：这些 TypedDict 可以直接转为 Go struct：

```go
type ModelResponse struct {
    ID      string   `json:"id"`
    Object  string   `json:"object"`
    Created int64    `json:"created"`
    Model   string   `json:"model"`
    Choices []Choice `json:"choices"`
    Usage   Usage    `json:"usage"`
}

type Choice struct {
    FinishReason string  `json:"finish_reason"`
    Index        int     `json:"index"`
    Message      Message `json:"message"`
}

type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}
```

### 3. 异常体系

**统一异常类型 (litellm/exceptions.py)**：

| 异常类 | HTTP Status | 说明 |
|--------|-------------|------|
| AuthenticationError | 401 | API Key 无效 |
| NotFoundError | 404 | 模型不存在 |
| BadRequestError | 400 | 请求参数错误 |
| RateLimitError | 429 | 触发限流 |
| Timeout | 408 | 请求超时 |
| ServiceUnavailableError | 503 | 服务不可用 |
| InternalServerError | 500 | 内部错误 |
| ContextWindowExceededError | 400 | 上下文超限 |
| ContentPolicyViolationError | 400 | 内容违规 |

每个异常都包含：
- status_code
- message
- llm_provider
- model
- litellm_debug_info
- max_retries / num_retries

**Go 转换**：定义统一 error 类型：

```go
type LLMError struct {
    StatusCode int
    Message    string
    Provider   string
    Model      string
    Retries    int
}

func (e *LLMError) Error() string { 
    return fmt.Sprintf("provider=%s model=%s code=%d: %s", 
        e.Provider, e.Model, e.StatusCode, e.Message)
}
```

### 4. Router 路由策略

**支持的路由策略 (litellm/router_strategy/)**：

| 策略 | 文件 | 说明 |
|------|------|------|
| Simple Shuffle | simple_shuffle.py | 随机选择 |
| Least Busy | least_busy.py | 最少并发 |
| Lowest Latency | lowest_latency.py | 最低延迟 |
| Lowest Cost | lowest_cost.py | 最低成本 |
| Lowest TPM/RPM | lowest_tpm_rpm.py | 最低 token/请求使用率 |
| Budget Limiter | budget_limiter.py | 预算限制 |
| Tag Based | tag_based_routing.py | 标签路由 |

**Router 核心逻辑**：

```python
class Router:
    def __init__(self, model_list, routing_strategy="simple-shuffle", ...):
        self.model_list = model_list  # 部署列表
        self.routing_strategy = routing_strategy
        self.cooldown_cache = CooldownCache()  # 冷却缓存（熔断）
        
    async def acompletion(self, model, messages, **kwargs):
        # 1. 获取可用部署
        deployments = self._get_deployments(model)
        # 2. 过滤冷却中的部署
        deployments = await _async_get_cooldown_deployments(deployments)
        # 3. 根据策略选择
        deployment = self._pick_deployment(deployments)
        # 4. 调用
        return await litellm.acompletion(...)
```

## 三、关键文件清单（AI 批量转换）

### 🤖 可直接让 AI 转换的文件

| 文件 | 内容 | 转换难度 |
|------|------|----------|
| types/utils.py | 核心类型定义 | ⭐ 简单 |
| types/llms/openai.py | OpenAI 类型 | ⭐ 简单 |
| types/llms/anthropic.py | Anthropic 类型 | ⭐ 简单 |
| exceptions.py | 异常定义 | ⭐ 简单 |
| model_prices_and_context_window.json | 价格表 | ⭐ 直接复制 |
| llms/openai/chat/gpt_transformation.py | OpenAI 适配 | ⭐⭐ 中等 |
| llms/anthropic/chat/transformation.py | Anthropic 适配 | ⭐⭐ 中等 |
| llms/azure/chat/gpt_transformation.py | Azure 适配 | ⭐⭐ 中等 |
| llms/gemini/chat/transformation.py | Gemini 适配 | ⭐⭐ 中等 |

### 🧠 需要人工重写的部分

| 模块 | 原因 |
|------|------|
| SSE 流式处理 | Python 的 async generator 模式在 Go 中完全不同 |
| HTTP Client | Go 用 net/http 或 fasthttp，需要重新设计连接池 |
| Router 并发控制 | Go 用 channel/semaphore，不是 Python asyncio |
| 配置热重载 | Go 用 fsnotify + atomic pointer |
| Metrics 埋点 | Go 用 prometheus/client_golang |

## 四、Provider 参数映射表

### OpenAI → Anthropic 参数映射
从 anthropic/chat/transformation.py 提取：

| OpenAI 参数 | Anthropic 参数 | 说明 |
|-------------|----------------|------|
| max_tokens | max_tokens | 直接映射 |
| max_completion_tokens | max_tokens | 别名 |
| temperature | temperature | 直接映射 |
| top_p | top_p | 直接映射 |
| stop | stop_sequences | 需要转为数组 |
| tools | tools | 需要格式转换 |
| tool_choice | tool_choice | 需要格式转换 |
| response_format | 转为 tool call | Anthropic 不直接支持 |
| stream | stream | 直接映射 |
| user | metadata.user_id | 嵌套 |

### Tool Choice 映射
```python
# OpenAI → Anthropic
"auto"     → {"type": "auto"}
"required" → {"type": "any"}
"none"     → {"type": "none"}
{"function": {"name": "xxx"}} → {"type": "tool", "name": "xxx"}
```

## 五、价格表结构

`model_prices_and_context_window.json` 结构：

```json
{
  "gpt-4o": {
    "litellm_provider": "openai",
    "mode": "chat",
    "max_input_tokens": 128000,
    "max_output_tokens": 16384,
    "input_cost_per_token": 0.0000025,
    "output_cost_per_token": 0.00001,
    "supports_function_calling": true,
    "supports_vision": true,
    "supports_response_schema": true,
    "supports_parallel_function_calling": true
  }
}
```

**Go 结构体**：

```go
type ModelInfo struct {
    LiteLLMProvider              string   `json:"litellm_provider"`
    Mode                         string   `json:"mode"`
    MaxInputTokens               int      `json:"max_input_tokens"`
    MaxOutputTokens              int      `json:"max_output_tokens"`
    InputCostPerToken            float64  `json:"input_cost_per_token"`
    OutputCostPerToken           float64  `json:"output_cost_per_token"`
    SupportsFunctionCalling      bool     `json:"supports_function_calling"`
    SupportsVision               bool     `json:"supports_vision"`
    SupportsResponseSchema       bool     `json:"supports_response_schema"`
    SupportsParallelFunctionCall bool     `json:"supports_parallel_function_calling"`
}
```

## 六、流式响应处理分析

### Python 实现模式
```python
# litellm/llms/base_llm/base_model_iterator.py
class BaseModelResponseIterator:
    def chunk_parser(self, chunk: dict) -> ModelResponseStream:
        """解析单个 chunk"""
        pass
        
    def __iter__(self):
        for chunk in self.streaming_response:
            yield self.chunk_parser(chunk)
```

### Go 实现建议
```go
type StreamHandler struct {
    reader   *bufio.Reader
    buffer   *sync.Pool  // 复用 buffer
    clientCtx context.Context
}

func (s *StreamHandler) Next() (*StreamChunk, error) {
    select {
    case <-s.clientCtx.Done():
        return nil, s.clientCtx.Err()  // client 断开
    default:
        line, err := s.reader.ReadBytes('\n')
        if err != nil {
            return nil, err
        }
        return s.parseChunk(line)
    }
}
```

## 七、我们需要实现的核心接口

### 1. Provider Interface
```go
type Provider interface {
    // 基础信息
    Name() string
    SupportedModels() []string
    
    // 参数转换
    MapParams(openaiParams map[string]any) map[string]any
    
    // 请求转换
    BuildRequest(ctx context.Context, req *ChatRequest) (*http.Request, error)
    
    // 响应转换
    ParseResponse(resp *http.Response) (*ChatResponse, error)
    ParseStreamChunk(chunk []byte) (*StreamChunk, error)
    
    // 错误处理
    MapError(statusCode int, body []byte) error
}
```

### 2. Router Interface
```go
type Router interface {
    // 选择部署
    Pick(ctx context.Context, model string) (*Deployment, error)
    
    // 标记成功/失败
    ReportSuccess(deployment *Deployment, latency time.Duration)
    ReportFailure(deployment *Deployment, err error)
    
    // 熔断状态
    IsCircuitOpen(deployment *Deployment) bool
}
```

### 3. Config Interface
```go
type ConfigManager interface {
    // 获取配置
    Get() *Config
    
    // 热重载
    Watch(ctx context.Context) <-chan *Config
    
    // 原子更新
    Update(newConfig *Config) error
}
```

## 八、开发优先级建议

### Phase 1: 骨架 (Week 1-2)
- 定义 Go 接口 (Provider, Router, Config)
- 实现 HTTP Server (net/http)
- 实现 OpenAI Provider（作为模板）
- 集成 Prometheus metrics
- 集成 slog 日志

### Phase 2: AI 批量生成 (Week 3)
- 让 AI 转换 types/ 下的所有类型定义
- 让 AI 转换 Anthropic/Azure/Gemini adapter
- 让 AI 生成参数映射表
- 人工 review + 修正

### Phase 3: 流式 & 高可用 (Week 4-5)
- 实现 SSE 流式转发（人工）
- 集成 gobreaker 熔断器
- 实现 per-provider semaphore
- 实现优雅关闭

### Phase 4: 打包 (Week 6)
- Distroless Docker 镜像
- Helm Chart
- CI/CD

## 九、Prompt 模板（用于 AI 批量转换）

### 转换类型定义
```
你是一个 Go 专家。请将以下 Python Pydantic/TypedDict 类型定义转换为 Go struct。

要求：
1. 使用正确的 json tag
2. Optional 字段使用指针类型或 omitempty
3. Union 类型使用 interface{} 或定义多个类型
4. 添加 GoDoc 注释

Python 代码：
---
{paste code}
---
```

### 转换 Provider Adapter
```
你是一个 Go 专家。请将以下 LiteLLM Provider 的 Python 实现转换为 Go。

要求：
1. 实现 Provider interface
2. MapParams 方法处理参数映射
3. BuildRequest 方法构建 HTTP 请求
4. ParseResponse 方法解析响应
5. 保持与原 Python 逻辑一致

Python 代码：
---
{paste code}
---

Go Provider Interface：
---
type Provider interface {
    Name() string
    MapParams(params map[string]any) map[string]any
    BuildRequest(ctx context.Context, req *ChatRequest) (*http.Request, error)
    ParseResponse(resp *http.Response) (*ChatResponse, error)
}
---
```