package api //nolint:revive // package name is intentional

import (
	"fmt"
	"net/http"
	"time"

	"github.com/goccy/go-json"

	llmux "github.com/blueberrycongee/llmux"
)

type TrafficSimulationRequest struct {
	Prompt      string `json:"prompt"`
	TrafficType string `json:"traffic_type"`
}

type TrafficSimulationLayer struct {
	Layer       string `json:"layer"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Summary     string `json:"summary"`
	TouchesLLM  bool   `json:"touches_llm"`
	Highlighted bool   `json:"highlighted"`
}

type TrafficSimulationResponse struct {
	RequestID           string                   `json:"request_id"`
	Prompt              string                   `json:"prompt"`
	TrafficType         string                   `json:"traffic_type"`
	RecommendedStrategy string                   `json:"recommended_strategy"`
	RecommendedProvider string                   `json:"recommended_provider"`
	RecommendedModel    string                   `json:"recommended_model"`
	ExecutionMode       string                   `json:"execution_mode"`
	MemoryInfluence     string                   `json:"memory_influence"`
	Layers              []TrafficSimulationLayer `json:"layers"`
	FinalSummary        string                   `json:"final_summary"`
	GeneratedAt         string                   `json:"generated_at"`
}

type RealTrafficRunRequest struct {
	Prompt      string `json:"prompt"`
	TrafficType string `json:"traffic_type"`
}

type RealTrafficRunResponse struct {
	RequestID           string `json:"request_id"`
	Prompt              string `json:"prompt"`
	TrafficType         string `json:"traffic_type"`
	RecommendedStrategy string `json:"recommended_strategy"`
	RecommendedProvider string `json:"recommended_provider"`
	RecommendedModel    string `json:"recommended_model"`
	ExecutionMode       string `json:"execution_mode"`
	MemoryInfluence     string `json:"memory_influence"`
	SchedulerInput      string `json:"scheduler_input"`
	SchedulerDecision   string `json:"scheduler_decision"`
	GatewayRequest      any    `json:"gateway_request"`
	GatewayResponse     any    `json:"gateway_response,omitempty"`
	LLMOutputText       string `json:"llm_output_text,omitempty"`
	ProviderReported    string `json:"provider_reported,omitempty"`
	ErrorMessage        string `json:"error_message,omitempty"`
	Succeeded           bool   `json:"succeeded"`
	GeneratedAt         string `json:"generated_at"`
}

func (h *ManagementHandler) SimulateTraffic(w http.ResponseWriter, r *http.Request) {
	var req TrafficSimulationRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Prompt == "" {
		req.Prompt = "请总结这个网关如何完成一次低延迟请求调度。"
	}
	if req.TrafficType == "" {
		req.TrafficType = "interactive"
	}

	memory, scores := buildRuntimeRoutingMemory(h)
	recommended := selectRecommendedProvider(scores)
	providerName := "deepseek-primary"
	modelName := "deepseek-chat"
	strategy := "lowest-latency"
	memoryInfluence := "routing memory marks recent latency and stability signals as favorable for interactive traffic"
	if recommended != nil {
		providerName = recommended.Provider
		strategy = recommended.RecommendedStrategy
		memoryInfluence = fmt.Sprintf("routing memory currently biases traffic toward %s because its recent runtime profile is more stable", providerName)
	}

	client, release := h.acquireClient()
	defer release()
	executionMode := "simulated-gateway-trace"
	if client != nil {
		if prov, ok := client.GetProvider(providerName); ok && prov != nil {
			models := prov.SupportedModels()
			if len(models) > 0 {
				modelName = models[0]
			}
			executionMode = "real-provider-configured"
		}
	}

	layers := []TrafficSimulationLayer{
		{Layer: "L1", Title: "请求接入层", Status: "completed", Summary: fmt.Sprintf("Gateway received a %s request with prompt length %d.", req.TrafficType, len(req.Prompt)), TouchesLLM: false, Highlighted: false},
		{Layer: "L2", Title: "协议适配层", Status: "completed", Summary: "The request is normalized into the gateway's unified OpenAI-style request schema.", TouchesLLM: false, Highlighted: false},
		{Layer: "L3", Title: "AI 流量调度层", Status: "completed", Summary: fmt.Sprintf("Scheduling selected %s using %s strategy after considering runtime memory and provider signals.", providerName, strategy), TouchesLLM: false, Highlighted: true},
		{Layer: "L4", Title: "请求执行层", Status: "completed", Summary: fmt.Sprintf("Execution targets provider=%s model=%s. This is the layer that actually touches the LLM backend.", providerName, modelName), TouchesLLM: true, Highlighted: true},
		{Layer: "L5", Title: "治理控制层", Status: "completed", Summary: "Governance checks budgets, quotas, API key policy, and auditability around the request path.", TouchesLLM: false, Highlighted: false},
		{Layer: "L6", Title: "观测优化层", Status: "completed", Summary: fmt.Sprintf("The run is converted into routing memory so later traffic can reuse this signal; memory items available=%d.", len(memory)), TouchesLLM: false, Highlighted: false},
	}

	resp := TrafficSimulationResponse{
		RequestID:           fmt.Sprintf("sim-%d", time.Now().UnixNano()),
		Prompt:              req.Prompt,
		TrafficType:         req.TrafficType,
		RecommendedStrategy: strategy,
		RecommendedProvider: providerName,
		RecommendedModel:    modelName,
		ExecutionMode:       executionMode,
		MemoryInfluence:     memoryInfluence,
		Layers:              layers,
		FinalSummary:        fmt.Sprintf("The simulated request was routed through all six gateway layers and would finally execute on %s/%s.", providerName, modelName),
		GeneratedAt:         time.Now().Format(time.RFC3339),
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *ManagementHandler) RealTrafficRun(w http.ResponseWriter, r *http.Request) {
	var req RealTrafficRunRequest
	if err := decodeJSONBody(r, &req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Prompt == "" {
		req.Prompt = "请用一段简洁说明总结这个异构网关如何为低延迟交互式请求选择模型并执行。"
	}
	if req.TrafficType == "" {
		req.TrafficType = "interactive"
	}

	memory, scores := buildRuntimeRoutingMemory(h)
	recommended := selectRecommendedProvider(scores)
	providerName := "deepseek-primary"
	modelName := "deepseek-chat"
	strategy := "lowest-latency"
	memoryInfluence := "routing memory marks recent latency and stability signals as favorable for interactive traffic"
	if recommended != nil {
		providerName = recommended.Provider
		strategy = recommended.RecommendedStrategy
		memoryInfluence = fmt.Sprintf("routing memory currently biases traffic toward %s because its recent runtime profile is more stable", providerName)
	}

	client, release := h.acquireClient()
	defer release()
	if client == nil {
		h.writeError(w, r, http.StatusServiceUnavailable, "client not available")
		return
	}
	if prov, ok := client.GetProvider(providerName); ok && prov != nil {
		models := prov.SupportedModels()
		if len(models) > 0 {
			modelName = models[0]
		}
	}

	gatewayReq := &llmux.ChatRequest{
		Model: modelName,
		Messages: []llmux.ChatMessage{{
			Role:    "user",
			Content: json.RawMessage(fmt.Sprintf("%q", req.Prompt)),
		}},
		Temperature: float64Ptr(0.2),
		MaxTokens:   256,
		Tags:        []string{"gateway-visualizer", req.TrafficType, strategy},
		User:        "gateway-visualizer",
	}

	resp, err := client.ChatCompletion(r.Context(), gatewayReq)
	result := RealTrafficRunResponse{
		RequestID:           fmt.Sprintf("real-%d", time.Now().UnixNano()),
		Prompt:              req.Prompt,
		TrafficType:         req.TrafficType,
		RecommendedStrategy: strategy,
		RecommendedProvider: providerName,
		RecommendedModel:    modelName,
		ExecutionMode:       "real-chat-completion",
		MemoryInfluence:     memoryInfluence,
		SchedulerInput:      fmt.Sprintf("traffic_type=%s prompt_length=%d memory_items=%d", req.TrafficType, len(req.Prompt), len(memory)),
		SchedulerDecision:   fmt.Sprintf("selected provider=%s model=%s strategy=%s", providerName, modelName, strategy),
		GatewayRequest: map[string]any{
			"model":       gatewayReq.Model,
			"messages":    gatewayReq.Messages,
			"max_tokens":  gatewayReq.MaxTokens,
			"temperature": gatewayReq.Temperature,
			"tags":        gatewayReq.Tags,
			"user":        gatewayReq.User,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
	}
	if err != nil {
		result.Succeeded = false
		result.ErrorMessage = err.Error()
		h.writeJSON(w, http.StatusOK, result)
		return
	}

	result.Succeeded = true
	result.ProviderReported = resp.Usage.Provider
	result.GatewayResponse = resp
	if len(resp.Choices) > 0 {
		result.LLMOutputText = string(resp.Choices[0].Message.Content)
	}
	if result.ProviderReported == "" {
		result.ProviderReported = providerName
	}
	h.writeJSON(w, http.StatusOK, result)
}

func decodeJSONBody(r *http.Request, v any) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func float64Ptr(v float64) *float64 { return &v }
