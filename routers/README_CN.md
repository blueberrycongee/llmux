# 路由模块 (Routers)

LLMux 支持多种路由策略，以实现各 LLM 供应商之间的负载均衡和高可用性。

## 路由策略

- **简单随机 (Simple Shuffle)**: 从可用部署中随机选择。支持权重选择。
- **轮询 (Round Robin)**: 按严格的轮询顺序选择部署。
- **最低延迟 (Lowest Latency)**: 选择平均延迟或首字延迟 (TTFT) 最低的部署。**已通过 EWMA 和动态权重算法进行增强。**
- **最小负载 (Least Busy)**: 选择当前活跃请求数最少的部署。
- **最小 TPM/RPM**: 选择 Token 或请求消耗最低的部署。
- **最低成本 (Lowest Cost)**: 选择每 Token 成本最低的部署。
- **基于标签 (Tag-Based)**: 根据请求级别的标签过滤部署。

## EWMA (指数加权移动平均)

LLMux 使用 EWMA 算法实时跟踪每个部署的性能和质量。与简单移动平均不同，EWMA 赋予近期观测值更高的权重，使路由器能快速适应供应商性能或可用性的变化。

### 工作原理

EWMA 的计算公式如下：
`Value_new = Alpha * Observation + (1 - Alpha) * Value_old`

其中 `Alpha` 是平滑因子 (0 < Alpha <= 1)。Alpha 值越高，平均值对近期变化的响应就越灵敏。

LLMux 跟踪以下指标的 EWMA：
- **延时 (Latency)**: 请求总时长。
- **首字延时 (TTFT)**: 流式请求的首字时间。
- **成功率 (Success Rate)**: 成功 (1.0) 和失败 (0.0) 的移动平均。

### 配置

您可以在路由配置中调整平滑因子：

```yaml
routing:
  strategy: lowest-latency
  ewma_alpha: 0.1  # 默认值
```

## LatencyRouter 中的动态权重

`Lowest Latency` 策略利用 EWMA 值进行动态反馈。它在可配置的延迟缓冲范围内，为所有健康的候选者计算动态权重。

### 权重计算

对于每个候选部署，其动态权重计算如下：
`Weight = BaseWeight * (SuccessRate^2) / Latency`

- **BaseWeight (基础权重)**: 部署配置的静态权重（默认为 1.0）。
- **SuccessRate (成功率)**: EWMA 成功率 (0.0 到 1.0)。将其平方后，即便供应商有微小的失败率，也会受到更严厉的惩罚。
- **Latency (延时)**: EWMA 延时 (或 TTFT)。作为分母，较低的延时会显著增加被选中的概率。

这种方法确保流量能自动从缓慢或失败的供应商转移，即使这些供应商在技术上仍处于“健康”状态（尚未触发断路器）。
