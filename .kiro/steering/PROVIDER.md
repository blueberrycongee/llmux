LiteLLM Provider 实现架构参考文档
概述
LiteLLM 通过统一的接口支持 100+ LLM providers。本文档分析其架构设计，为 LLMux 快速实现多 provider 支持提供参考。

一、核心架构设计
1.1 目录结构
litellm/llms/
├── base.py                    # 基础 BaseLLM 类（较老，逐渐废弃）
├── base_llm/                  # 新的基础抽象层
│   ├── chat/
│   │   └── transformation.py  # BaseConfig - 核心配置基类
│   ├── embedding/
│   │   └── transformation.py  # BaseEmbeddingConfig
│   ├── audio_transcription/
│   ├── image_generation/
│   └── ...
├── openai/                    # OpenAI 实现
│   ├── chat/
│   │   └── gpt_transformation.py  # OpenAIGPTConfig
│   └── ...
├── openai_like/               # OpenAI 兼容层基类
│   └── chat/
│       └── transformation.py  # OpenAILikeChatConfig
├── anthropic/                 # Anthropic 实现
├── groq/                      # Groq 实现（继承 OpenAILike）
├── deepseek/                  # DeepSeek 实现
└── ... (100+ providers)
1.2 继承体系
BaseConfig (ABC)                           # 抽象基类
    │
    └── OpenAIGPTConfig                    # OpenAI 完整实现
            │
            └── OpenAILikeChatConfig       # OpenAI 兼容层
                    │
                    ├── GroqChatConfig     # Groq（OpenAI兼容）
                    ├── DeepSeekChatConfig # DeepSeek（OpenAI兼容）
                    ├── TogetherAIConfig   # Together AI
                    ├── FireworksAIConfig  # Fireworks AI
                    └── ... (大多数 OpenAI 兼容 provider)

BaseConfig (ABC)
    │
    └── AnthropicConfig                    # Anthropic（独立实现）
            │
            └── BedrockAnthropicConfig     # AWS Bedrock Anthropic
            └── VertexAIAnthropicConfig    # Google Vertex Anthropic
二、核心接口定义
2.1 BaseConfig 抽象基类
文件: 
litellm/llms/base_llm/chat/transformation.py

python
class BaseConfig(ABC):
    """所有 Provider 配置的抽象基类"""
    
    # ========== 必须实现的抽象方法 ==========
    
    @abstractmethod
    def get_supported_openai_params(self, model: str) -> list:
        """返回该 provider 支持的 OpenAI 参数列表"""
        pass
    
    @abstractmethod
    def map_openai_params(
        self,
        non_default_params: dict,    # 用户传入的非默认参数
        optional_params: dict,        # 可选参数累积器
        model: str,
        drop_params: bool,            # 是否丢弃不支持的参数
    ) -> dict:
        """将 OpenAI 格式参数映射为 provider 特定格式"""
        pass
    
    @abstractmethod
    def validate_environment(
        self,
        headers: dict,
        model: str,
        messages: List[AllMessageValues],
        optional_params: dict,
        litellm_params: dict,
        api_key: Optional[str] = None,
        api_base: Optional[str] = None,
    ) -> dict:
        """验证环境配置，返回处理后的 headers"""
        pass
    
    @abstractmethod
    def transform_request(
        self,
        model: str,
        messages: List[AllMessageValues],
        optional_params: dict,
        litellm_params: dict,
        headers: dict,
    ) -> dict:
        """将统一请求格式转换为 provider 特定的请求体"""
        pass
    
    @abstractmethod
    def transform_response(
        self,
        model: str,
        raw_response: httpx.Response,
        model_response: ModelResponse,
        logging_obj: LiteLLMLoggingObj,
        request_data: dict,
        messages: List[AllMessageValues],
        optional_params: dict,
        litellm_params: dict,
        encoding: Any,
        api_key: Optional[str] = None,
        json_mode: Optional[bool] = None,
    ) -> ModelResponse:
        """将 provider 响应转换为统一的 ModelResponse 格式"""
        pass
    
    @abstractmethod
    def get_error_class(
        self, 
        error_message: str, 
        status_code: int, 
        headers: Union[dict, httpx.Headers]
    ) -> BaseLLMException:
        """将 provider 错误转换为统一异常"""
        pass
    
    # ========== 可选的重写方法 ==========
    
    def get_complete_url(
        self,
        api_base: Optional[str],
        api_key: Optional[str],
        model: str,
        optional_params: dict,
        litellm_params: dict,
        stream: Optional[bool] = None,
    ) -> str:
        """构建完整的 API URL"""
        if api_base is None:
            raise ValueError("api_base is required")
        return api_base
    
    def sign_request(
        self,
        headers: dict,
        optional_params: dict,
        request_data: dict,
        api_base: str,
        ...
    ) -> Tuple[dict, Optional[bytes]]:
        """请求签名（如 AWS Bedrock 需要 SigV4 签名）"""
        return headers, None
    
    def should_fake_stream(
        self,
        model: Optional[str],
        stream: Optional[bool],
        custom_llm_provider: Optional[str] = None,
    ) -> bool:
        """是否需要模拟流式响应"""
        return False
    
    def get_model_response_iterator(
        self,
        streaming_response: Union[Iterator[str], AsyncIterator[str], ModelResponse],
        sync_stream: bool,
        json_mode: Optional[bool] = False,
    ) -> Any:
        """获取流式响应迭代器"""
        pass
    
    @property
    def custom_llm_provider(self) -> Optional[str]:
        """返回 provider 标识符"""
        return None
三、Provider 实现模式
3.1 模式一：OpenAI 兼容 Provider（最简单）
适用于：Groq、DeepSeek、Together AI、Fireworks AI、OpenRouter 等

特点：API 格式与 OpenAI 基本一致，只需少量适配

示例 - DeepSeek 实现（126行代码）：

python
# litellm/llms/deepseek/chat/transformation.py

from ...openai.chat.gpt_transformation import OpenAIGPTConfig

class DeepSeekChatConfig(OpenAIGPTConfig):
    
    def get_supported_openai_params(self, model: str) -> list:
        """添加 DeepSeek 特有的参数支持"""
        params = super().get_supported_openai_params(model)
        params.extend(["thinking", "reasoning_effort"])  # DeepSeek 支持思考模式
        return params
    
    def map_openai_params(
        self,
        non_default_params: dict,
        optional_params: dict,
        model: str,
        drop_params: bool,
    ) -> dict:
        """处理 DeepSeek 特有的参数映射"""
        optional_params = super().map_openai_params(
            non_default_params, optional_params, model, drop_params
        )
        
        # 处理 thinking 参数 - DeepSeek 只支持 {"type": "enabled"}
        thinking_value = optional_params.pop("thinking", None)
        reasoning_effort = optional_params.pop("reasoning_effort", None)
        
        if thinking_value is not None:
            if isinstance(thinking_value, dict) and thinking_value.get("type") == "enabled":
                optional_params["thinking"] = {"type": "enabled"}
        elif reasoning_effort is not None and reasoning_effort != "none":
            optional_params["thinking"] = {"type": "enabled"}
        
        return optional_params
    
    def _transform_messages(
        self, messages: List[AllMessageValues], model: str, is_async: bool = False
    ):
        """DeepSeek 不支持 list 格式的 content"""
        messages = handle_messages_with_content_list_to_str_conversion(messages)
        return super()._transform_messages(messages=messages, model=model, is_async=is_async)
    
    def _get_openai_compatible_provider_info(
        self, api_base: Optional[str], api_key: Optional[str]
    ) -> Tuple[Optional[str], Optional[str]]:
        """返回 provider 的 API 基础 URL 和 API Key"""
        api_base = (
            api_base
            or get_secret_str("DEEPSEEK_API_BASE")
            or "https://api.deepseek.com/beta"
        )
        dynamic_api_key = api_key or get_secret_str("DEEPSEEK_API_KEY")
        return api_base, dynamic_api_key
    
    def get_complete_url(
        self,
        api_base: Optional[str],
        api_key: Optional[str],
        model: str,
        optional_params: dict,
        litellm_params: dict,
        stream: Optional[bool] = None,
    ) -> str:
        """构建完整 URL"""
        if not api_base:
            api_base = "https://api.deepseek.com/beta"
        if not api_base.endswith("/chat/completions"):
            api_base = f"{api_base}/chat/completions"
        return api_base
3.2 模式二：OpenAI-Like 中间层
适用于：需要额外处理但仍是 OpenAI 兼容的 provider

示例 - Groq 实现（327行代码）：

python
# litellm/llms/groq/chat/transformation.py

from ...openai_like.chat.transformation import OpenAILikeChatConfig

class GroqChatConfig(OpenAILikeChatConfig):
    # 定义支持的参数作为类属性
    frequency_penalty: Optional[int] = None
    max_tokens: Optional[int] = None
    temperature: Optional[int] = None
    response_format: Optional[dict] = None
    tools: Optional[list] = None
    tool_choice: Optional[Union[str, dict]] = None
    
    @property
    def custom_llm_provider(self) -> Optional[str]:
        return "groq"
    
    def _get_openai_compatible_provider_info(
        self, api_base: Optional[str], api_key: Optional[str]
    ) -> Tuple[Optional[str], Optional[str]]:
        api_base = (
            api_base
            or get_secret_str("GROQ_API_BASE")
            or "https://api.groq.com/openai/v1"
        )
        dynamic_api_key = api_key or get_secret_str("GROQ_API_KEY")
        return api_base, dynamic_api_key
    
    def _should_fake_stream(self, optional_params: dict) -> bool:
        """Groq 不支持流式 + response_format 组合"""
        if optional_params.get("response_format") is not None:
            return True
        return False
    
    def map_openai_params(
        self,
        non_default_params: dict,
        optional_params: dict,
        model: str,
        drop_params: bool = False,
        replace_max_completion_tokens_with_max_tokens: bool = False,
    ) -> dict:
        """处理 Groq 特有的 JSON 模式"""
        _response_format = non_default_params.get("response_format")
        
        if self._should_fake_stream(non_default_params):
            optional_params["fake_stream"] = True
        
        if _response_format is not None and isinstance(_response_format, dict):
            json_schema = self._extract_json_schema(_response_format)
            
            if json_schema is not None:
                # 不支持原生 json_schema 的模型，使用 tool calling 模拟
                if not litellm.supports_response_schema(model=model, custom_llm_provider="groq"):
                    _tool = self._create_json_tool_call_for_response_format(json_schema)
                    optional_params["tools"] = [_tool]
                    optional_params["tool_choice"] = {"type": "function", "function": {"name": "json_tool_call"}}
                    optional_params["json_mode"] = True
                    non_default_params.pop("response_format", None)
        
        return super().map_openai_params(non_default_params, optional_params, model, drop_params)
    
    def get_model_response_iterator(
        self,
        streaming_response: Union[Iterator[str], AsyncIterator[str], ModelResponse],
        sync_stream: bool,
        json_mode: Optional[bool] = False,
    ) -> Any:
        """自定义流式响应处理器"""
        return GroqChatCompletionStreamingHandler(
            streaming_response=streaming_response,
            sync_stream=sync_stream,
            json_mode=json_mode,
        )


class GroqChatCompletionStreamingHandler(OpenAIChatCompletionStreamingHandler):
    """自定义流式解析器，处理 Groq 特有的错误格式"""
    def chunk_parser(self, chunk: dict) -> ModelResponseStream:
        error = chunk.get("error")
        if error:
            raise OpenAIError(status_code=error.get("code"), message=error.get("message"))
        return super().chunk_parser(chunk)
3.3 模式三：完全独立实现
适用于：Anthropic、Google Gemini 等 API 格式完全不同的 provider

示例 - Anthropic 关键实现（1538行代码）：

python
# litellm/llms/anthropic/chat/transformation.py

class AnthropicConfig(AnthropicModelInfo, BaseConfig):
    """
    Reference: https://docs.anthropic.com/claude/reference/messages_post
    """
    
    max_tokens: Optional[int] = None
    stop_sequences: Optional[list] = None
    temperature: Optional[int] = None
    top_p: Optional[int] = None
    top_k: Optional[int] = None
    metadata: Optional[dict] = None
    system: Optional[str] = None
    
    @property
    def custom_llm_provider(self) -> Optional[str]:
        return "anthropic"
    
    def get_supported_openai_params(self, model: str):
        """Anthropic 支持的参数（需要完全自定义映射）"""
        params = [
            "stream", "stop", "temperature", "top_p",
            "max_tokens", "max_completion_tokens",
            "tools", "tool_choice", "extra_headers",
            "parallel_tool_calls", "response_format", "user",
            "web_search_options",
        ]
        
        # 带思考能力的模型额外支持
        if supports_reasoning(model=model, custom_llm_provider=self.custom_llm_provider):
            params.append("thinking")
            params.append("reasoning_effort")
        
        return params
    
    def map_openai_params(
        self,
        non_default_params: dict,
        optional_params: dict,
        model: str,
        drop_params: bool,
    ) -> dict:
        """OpenAI 参数 -> Anthropic 参数的完整映射"""
        is_thinking_enabled = self.is_thinking_enabled(non_default_params)
        
        for param, value in non_default_params.items():
            if param == "max_tokens":
                optional_params["max_tokens"] = value
            elif param == "max_completion_tokens":
                optional_params["max_tokens"] = value  # Anthropic 使用 max_tokens
            elif param == "tools":
                # OpenAI tools 格式 -> Anthropic tools 格式
                anthropic_tools, mcp_servers = self._map_tools(value)
                optional_params = self._add_tools_to_optional_params(
                    optional_params=optional_params, tools=anthropic_tools
                )
                if mcp_servers:
                    optional_params["mcp_servers"] = mcp_servers
            elif param == "tool_choice":
                # OpenAI tool_choice -> Anthropic tool_choice
                _tool_choice = self._map_tool_choice(
                    tool_choice=non_default_params.get("tool_choice"),
                    parallel_tool_use=non_default_params.get("parallel_tool_calls"),
                )
                if _tool_choice is not None:
                    optional_params["tool_choice"] = _tool_choice
            elif param == "stop":
                # OpenAI stop -> Anthropic stop_sequences
                _value = self._map_stop_sequences(value)
                if _value is not None:
                    optional_params["stop_sequences"] = _value
            elif param == "response_format" and isinstance(value, dict):
                # 转换为 tool 调用来实现 JSON 模式
                _tool = self.map_response_format_to_anthropic_tool(value, optional_params, is_thinking_enabled)
                if _tool is not None:
                    optional_params = self._add_tools_to_optional_params(
                        optional_params=optional_params, tools=[_tool]
                    )
                optional_params["json_mode"] = True
            elif param == "user":
                # OpenAI user -> Anthropic metadata.user_id
                optional_params["metadata"] = {"user_id": value}
            elif param == "reasoning_effort":
                # 映射 reasoning_effort -> thinking 参数
                optional_params["thinking"] = self._map_reasoning_effort(value)
        
        return optional_params
    
    def _map_tool_choice(
        self, tool_choice: Optional[str], parallel_tool_use: Optional[bool]
    ) -> Optional[AnthropicMessagesToolChoice]:
        """OpenAI tool_choice 到 Anthropic 格式的映射"""
        if tool_choice == "auto":
            return AnthropicMessagesToolChoice(type="auto")
        elif tool_choice == "required":
            return AnthropicMessagesToolChoice(type="any")  # Anthropic 用 "any" 表示必须使用工具
        elif tool_choice == "none":
            return AnthropicMessagesToolChoice(type="none")
        elif isinstance(tool_choice, dict):
            _tool_name = tool_choice.get("function", {}).get("name")
            _tool_choice = AnthropicMessagesToolChoice(type="tool")
            if _tool_name is not None:
                _tool_choice["name"] = _tool_name
            return _tool_choice
        return None
    
    def _map_tool_helper(self, tool: ChatCompletionToolParam) -> AllAnthropicToolsValues:
        """单个 OpenAI tool -> Anthropic tool 的转换"""
        if tool["type"] == "function":
            _input_schema = tool["function"].get("parameters", {
                "type": "object",
                "properties": {},
            })
            
            return AnthropicMessagesTool(
                name=tool["function"]["name"],
                input_schema=AnthropicInputSchema(**_input_schema),
                description=tool["function"].get("description"),
            )
        elif tool["type"].startswith("computer_"):
            # Anthropic Computer Use 工具
            return AnthropicComputerTool(
                type=tool["type"],
                name=tool["function"].get("name", "computer"),
                display_width_px=tool["function"]["parameters"]["display_width_px"],
                display_height_px=tool["function"]["parameters"]["display_height_px"],
            )
        # ... 更多工具类型处理
    
    def transform_request(
        self,
        model: str,
        messages: List[AllMessageValues],
        optional_params: dict,
        litellm_params: dict,
        headers: dict,
    ) -> dict:
        """构建 Anthropic API 请求体"""
        # 1. 提取并转换 system message
        system_prompt, messages = self.translate_system_message(messages)
        
        # 2. 转换消息格式
        anthropic_messages = self._transform_messages(messages)
        
        # 3. 构建请求
        data = {
            "model": model,
            "messages": anthropic_messages,
            **optional_params,
        }
        
        if system_prompt:
            data["system"] = system_prompt
        
        # 4. 确保 max_tokens 存在（Anthropic 强制要求）
        if "max_tokens" not in data:
            data["max_tokens"] = self.get_max_tokens_for_model(model)
        
        return data
    
    def transform_response(
        self,
        model: str,
        raw_response: httpx.Response,
        model_response: ModelResponse,
        ...
    ) -> ModelResponse:
        """Anthropic 响应 -> 统一 ModelResponse 格式"""
        response_json = raw_response.json()
        
        # 转换 content 块
        content_str = ""
        tool_calls = []
        thinking_blocks = []
        
        for idx, content in enumerate(response_json.get("content", [])):
            if content["type"] == "text":
                content_str += content["text"]
            elif content["type"] == "tool_use":
                # Anthropic tool_use -> OpenAI tool_calls 格式
                tool_calls.append(self.convert_tool_use_to_openai_format(content, idx))
            elif content["type"] == "thinking":
                thinking_blocks.append(ChatCompletionThinkingBlock(
                    type="thinking",
                    thinking=content["thinking"],
                ))
        
        # 构建 Message
        message = LitellmMessage(
            content=content_str or None,
            role="assistant",
            tool_calls=tool_calls if tool_calls else None,
            thinking_blocks=thinking_blocks if thinking_blocks else None,
        )
        
        # 转换 usage
        usage = Usage(
            prompt_tokens=response_json["usage"]["input_tokens"],
            completion_tokens=response_json["usage"]["output_tokens"],
            total_tokens=response_json["usage"]["input_tokens"] + response_json["usage"]["output_tokens"],
        )
        
        # 转换 finish_reason
        finish_reason = map_finish_reason(response_json["stop_reason"])
        
        model_response.choices = [Choices(
            message=message,
            index=0,
            finish_reason=finish_reason,
        )]
        model_response.usage = usage
        
        return model_response
四、Provider 注册机制
4.1 ProviderConfigManager
文件: 
litellm/utils.py

python
class ProviderConfigManager:
    @staticmethod
    def get_provider_chat_config(
        model: str, 
        provider: LlmProviders
    ) -> Optional[BaseConfig]:
        """根据 provider 类型返回对应的配置类实例"""
        
        # 1. 优先检查 JSON 配置的 provider（动态加载）
        if JSONProviderRegistry.exists(provider.value):
            provider_config = JSONProviderRegistry.get(provider.value)
            return create_config_class(provider_config)()
        
        # 2. 硬编码的 provider 映射
        if provider == LlmProviders.OPENAI:
            if openaiOSeriesConfig.is_model_o_series_model(model):
                return openaiOSeriesConfig
            return OpenAIGPTConfig()
        
        elif provider == LlmProviders.DEEPSEEK:
            return DeepSeekChatConfig()
        
        elif provider == LlmProviders.GROQ:
            return GroqChatConfig()
        
        elif provider == LlmProviders.ANTHROPIC:
            return AnthropicConfig()
        
        elif provider == LlmProviders.VERTEX_AI:
            if "gemini" in model:
                return VertexGeminiConfig()
            elif "claude" in model:
                return VertexAIAnthropicConfig()
            else:
                return VertexAILlama3Config()
        
        elif provider == LlmProviders.BEDROCK:
            return get_bedrock_chat_config(model)  # 根据模型动态选择
        
        # ... 100+ providers 的映射
        
        return None
4.2 JSON Provider 动态配置
LiteLLM 支持通过 JSON 配置文件动态添加 OpenAI-like provider：

json
{
  "provider_name": "my_custom_provider",
  "api_base": "https://api.example.com/v1",
  "api_key_env": "MY_PROVIDER_API_KEY",
  "supported_params": [
    "temperature", "max_tokens", "top_p", "stream"
  ],
  "model_prefix": "my-provider/"
}
五、数据类型定义
5.1 统一请求/响应类型
文件: 
litellm/types/llms/openai.py
, 
litellm/types/utils.py

关键类型定义：

python
# 消息类型
AllMessageValues = Union[
    ChatCompletionSystemMessage,
    ChatCompletionUserMessage,
    ChatCompletionAssistantMessage,
    ChatCompletionToolMessage,
    ChatCompletionFunctionMessage,
]

# Tool 参数
class ChatCompletionToolParam(TypedDict):
    type: Literal["function"]
    function: ChatCompletionToolParamFunctionChunk

class ChatCompletionToolParamFunctionChunk(TypedDict):
    name: str
    description: Optional[str]
    parameters: Optional[dict]

# 响应类型
class ModelResponse:
    id: str
    choices: List[Choices]
    created: int
    model: str
    usage: Usage
    
class Choices:
    finish_reason: str
    index: int
    message: Message

class Message:
    content: Optional[str]
    role: str
    tool_calls: Optional[List[ChatCompletionMessageToolCall]]
    function_call: Optional[FunctionCall]
    thinking_blocks: Optional[List[ChatCompletionThinkingBlock]]

class Usage:
    prompt_tokens: int
    completion_tokens: int
    total_tokens: int
六、快速实现新 Provider 的步骤
6.1 OpenAI 兼容 Provider（推荐）
创建目录结构：
litellm/llms/your_provider/
├── __init__.py
└── chat/
    └── transformation.py
实现配置类（继承 
OpenAILikeChatConfig
）：
python
class YourProviderChatConfig(OpenAILikeChatConfig):
    @property
    def custom_llm_provider(self) -> str:
        return "your_provider"
    
    def _get_openai_compatible_provider_info(
        self, api_base: Optional[str], api_key: Optional[str]
    ) -> Tuple[Optional[str], Optional[str]]:
        api_base = api_base or get_secret_str("YOUR_PROVIDER_API_BASE") or "https://api.yourprovider.com/v1"
        api_key = api_key or get_secret_str("YOUR_PROVIDER_API_KEY")
        return api_base, api_key
    
    def get_supported_openai_params(self, model: str) -> list:
        params = super().get_supported_openai_params(model)
        # 添加/移除特有参数
        return params
注册到 ProviderConfigManager
添加到 LlmProviders 枚举
6.2 非 OpenAI 兼容 Provider
需要完整实现 
BaseConfig
 的所有抽象方法，参考 
AnthropicConfig
 的实现。

七、LLMux 实现建议
基于 LiteLLM 的架构，为 LLMux (Go) 建议以下实现方案：

7.1 接口设计（已有）
go
// internal/provider/interface.go
type Provider interface {
    Name() string
    SupportedModels() []string
    SupportsModel(model string) bool
    BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error)
    ParseResponse(resp *http.Response) (*types.ChatResponse, error)
    ParseStreamChunk(data []byte) (*types.StreamChunk, error)
    MapError(statusCode int, body []byte) error
}
7.2 建议的继承体系
go
// 基础 OpenAI-like 实现
type OpenAILikeProvider struct {
    name      string
    apiBase   string
    apiKey    string
    models    []string
}

// Groq 只需重写少量方法
type GroqProvider struct {
    OpenAILikeProvider
}

func (g *GroqProvider) Name() string { return "groq" }

func (g *GroqProvider) GetAPIBase() string {
    return "https://api.groq.com/openai/v1"
}

// Anthropic 需要完全独立实现
type AnthropicProvider struct {
    apiKey string
    apiBase string
}
7.3 参数映射表（核心）
可以将 LiteLLM 的各 provider 参数映射逻辑抽取为配置表：

go
var ProviderParamMappings = map[string]ParamMapping{
    "groq": {
        SupportedParams: []string{
            "temperature", "max_tokens", "top_p", "stream",
            "tools", "tool_choice", "response_format",
        },
        MaxTokensKey: "max_tokens", // some providers use max_completion_tokens
        StopKey: "stop",
    },
    "anthropic": {
        SupportedParams: []string{
            "temperature", "max_tokens", "top_p", "stream",
            "tools", "tool_choice", "stop",
        },
        RenameParams: map[string]string{
            "stop": "stop_sequences",
        },
        TransformFuncs: map[string]TransformFunc{
            "tool_choice": transformAnthropicToolChoice,
            "tools": transformAnthropicTools,
        },
    },
}
八、Provider 列表与复杂度分级
复杂度	Provider	说明
⭐ 简单	Groq, DeepSeek, Together AI, Fireworks AI, Perplexity, OpenRouter, Deepinfra	完全 OpenAI 兼容
⭐⭐ 中等	Mistral, Cohere, Hugging Face, Replicate	需要少量参数映射
⭐⭐⭐ 复杂	Anthropic, Google Gemini, AWS Bedrock, Azure OpenAI, Vertex AI	独立 API 格式，需完整实现
九、总结
LiteLLM 的设计精髓：

分层继承：
BaseConfig
 -> 
OpenAIGPTConfig
 -> 
OpenAILikeChatConfig
 -> 具体 Provider
标准化接口：6 个核心抽象方法定义了完整的请求-响应生命周期
参数映射：统一使用 OpenAI 格式作为输入，各 provider 负责转换
工厂注册：
ProviderConfigManager
 集中管理 provider 实例创建
最小实现：OpenAI 兼容 provider 只需 ~100 行代码
这种架构使得 LLMux 可以快速复用 LiteLLM 的映射逻辑，大幅减少开发工作量。