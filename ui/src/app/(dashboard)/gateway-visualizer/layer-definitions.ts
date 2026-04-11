export type GatewayLayerSlug = 'l1-ingress' | 'l2-protocol' | 'l3-scheduling' | 'l4-execution' | 'l5-governance' | 'l6-observability';

export interface GatewayLayerDefinition {
  slug: GatewayLayerSlug;
  code: string;
  title: string;
  summary: string;
  focus: string;
}

export const gatewayLayerDefinitions: GatewayLayerDefinition[] = [
  {
    slug: 'l1-ingress',
    code: 'L1',
    title: '请求接入层',
    summary: '统一接收应用、Agent 与管理端发起的请求，建立入口上下文。',
    focus: '这里应该看到请求从哪里来、prompt 多长、流量类型是什么。',
  },
  {
    slug: 'l2-protocol',
    code: 'L2',
    title: '协议适配层',
    summary: '把不同来源请求归一成网关内部统一的 OpenAI 风格结构。',
    focus: '这里应该看到归一化后的 model、messages、tags 等协议结果。',
  },
  {
    slug: 'l3-scheduling',
    code: 'L3',
    title: 'AI 流量调度层',
    summary: '根据 routing memory、runtime stats 和策略选择 provider/model。',
    focus: '这里需要看到调度输入、调度决策，以及 AI/策略如何选择执行目标。',
  },
  {
    slug: 'l4-execution',
    code: 'L4',
    title: '请求执行层',
    summary: '真正把请求打到底层 provider，例如 DeepSeek。',
    focus: '这里需要看到真实请求体、真实响应、报错信息，以及是否真的触发了 DeepSeek。',
  },
  {
    slug: 'l5-governance',
    code: 'L5',
    title: '治理控制层',
    summary: '围绕预算、配额、访问控制、审计形成治理闭环。',
    focus: '这里应该看到调度之外的约束条件，例如 budget、policy、audit。',
  },
  {
    slug: 'l6-observability',
    code: 'L6',
    title: '观测优化层',
    summary: '将一次运行沉淀为 routing memory，形成之后的优化依据。',
    focus: '这里应该看到 memory 如何生成，以及它如何反过来影响下一次调度。',
  },
];

export function getGatewayLayerDefinition(slug: string) {
  return gatewayLayerDefinitions.find((item) => item.slug === slug) ?? null;
}
