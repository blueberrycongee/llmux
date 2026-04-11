"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import {
  Activity,
  ArrowRight,
  BrainCircuit,
  Building2,
  CheckCircle2,
  FileText,
  GitBranch,
  KeyRound,
  Network,
  ShieldCheck,
  Sparkles,
  Users,
} from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { apiClient } from "@/lib/api/client";
import {
  clearActiveRoutingMemory,
  clearSavedRoutingMemory,
  deriveOptimizationComparisonFromSavedMemory,
  getActiveRoutingMemoryId,
  getSavedRoutingMemory,
  setActiveRoutingMemory,
} from "@/lib/routing-memory-store";
import type { RoutingMemoryItem, RoutingOptimizationComparison, SchedulingAdvisor, TrafficSimulationResponse } from "@/types/api";
import { useDashboardStats } from "@/hooks/use-dashboard-stats";

function formatPercent(value: number) {
  return `${value.toFixed(1)}%`;
}

function formatNumber(value: number) {
  return value.toLocaleString();
}

export default function DemoCenterPage() {
  const [advisor, setAdvisor] = useState<SchedulingAdvisor | null>(null);
  const [memory, setMemory] = useState<RoutingMemoryItem[]>([]);
  const [comparison, setComparison] = useState<RoutingOptimizationComparison | null>(null);
  const [simulation, setSimulation] = useState<TrafficSimulationResponse | null>(null);
  const [simulating, setSimulating] = useState(false);
  const [activeMemoryId, setActiveMemoryId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const loadSavedMemory = () => {
    const saved = getSavedRoutingMemory();
    const flattened = saved.flatMap((bundle) => bundle.items.map((item) => ({ ...item, category: `${bundle.title} · ${item.category}` })));
    setMemory(flattened);
    setActiveMemoryId(getActiveRoutingMemoryId());
    const derived = deriveOptimizationComparisonFromSavedMemory(saved);
    if (derived) setComparison(derived);
  };

  const endDate = useMemo(() => new Date().toISOString().split("T")[0], []);
  const startDate = useMemo(() => {
    const d = new Date();
    d.setDate(d.getDate() - 30);
    return d.toISOString().split("T")[0];
  }, []);

  const { totalRequests, totalTokens, avgLatency, successRate } = useDashboardStats({ startDate, endDate });

  useEffect(() => {
    let active = true;
    const load = async () => {
      setLoading(true);
      try {
        const [advisorData, memoryData, comparisonData] = await Promise.all([
          apiClient.getSchedulingAdvisor(),
          apiClient.getRoutingMemory(),
          apiClient.getRoutingOptimizationComparison(),
        ]);
        if (!active) return;
        setAdvisor(advisorData);
        const saved = getSavedRoutingMemory();
        setActiveMemoryId(getActiveRoutingMemoryId());
        if (saved.length > 0) {
          const flattened = saved.flatMap((bundle) => bundle.items.map((item) => ({ ...item, category: `${bundle.title} · ${item.category}` })));
          setMemory(flattened);
          setComparison(deriveOptimizationComparisonFromSavedMemory(saved) ?? comparisonData);
        } else {
          setMemory(memoryData.data ?? []);
          setComparison(comparisonData);
        }
      } catch {
        if (!active) return;
        setAdvisor(null);
        setMemory([]);
      } finally {
        if (active) setLoading(false);
      }
    };
    void load();
    return () => {
      active = false;
    };
  }, []);

  const demoSteps = [
    "先展示网关 AI 流量调度中枢，说明系统主体是异构网关。",
    "再展示调度建议与优化效果，说明系统如何进行动态流量分发。",
    "进入 Agent Town，展示 routing memory 如何从演化实验层沉淀。",
    "最后展示 API Keys、Teams 与 Audit Logs，说明治理闭环。",
  ];

  const schedulingHighlights = comparison
    ? [
        { label: "调度策略", value: comparison.optimized.strategy },
        { label: "当前 Provider", value: comparison.optimized.provider },
        { label: "延迟改善", value: `${Math.abs(comparison.latency_delta_ms)} ms` },
        { label: "成功率提升", value: `+${comparison.success_rate_delta.toFixed(1)}%` },
      ]
    : [
        { label: "调度策略", value: advisor?.recommended_strategy ?? "least-busy" },
        { label: "当前 Provider", value: advisor?.recommended_provider ?? "shared-pool" },
        { label: "延迟目标", value: `${avgLatency} ms` },
        { label: "成功率", value: formatPercent(successRate) },
      ];

  const runSimulation = async () => {
    setSimulating(true);
    try {
      const result = await apiClient.simulateTraffic({
        prompt: "请模拟一条对低延迟敏感的交互式请求经过异构网关的完整链路。",
        traffic_type: "interactive",
      });
      setSimulation(result);
    } finally {
      setSimulating(false);
    }
  };

  const sixLayers = [
    {
      layer: "L1",
      title: "请求接入层",
      desc: "统一接收上层应用或 Agent 请求，提供兼容 OpenAI 风格的入口。",
      status: "已实现",
      touchpoint: "接收请求，不直接调用 LLM",
      highlight: false,
    },
    {
      layer: "L2",
      title: "协议适配层",
      desc: "屏蔽不同 provider 的协议差异，统一请求、响应和认证语义。",
      status: "已实现",
      touchpoint: "适配协议，不直接调用 LLM",
      highlight: false,
    },
    {
      layer: "L3",
      title: "AI 流量调度层",
      desc: "依据 latency、success rate、cost 与 routing memory 选择 provider 与策略。",
      status: "已实现（memory-informed）",
      touchpoint: "做决策增强，不是最终执行调用",
      highlight: false,
    },
    {
      layer: "L4",
      title: "请求执行层",
      desc: "真正把请求转发到 DeepSeek 等底层模型服务，并返回结果。",
      status: "真实调用已接入",
      touchpoint: "这里才是真正用到大模型的地方",
      highlight: true,
    },
    {
      layer: "L5",
      title: "治理控制层",
      desc: "负责 API Keys、组织团队、预算配额、审计日志与访问控制。",
      status: "已实现",
      touchpoint: "不直接调用 LLM，负责治理约束",
      highlight: false,
    },
    {
      layer: "L6",
      title: "观测优化层",
      desc: "沉淀 routing memory，输出 advisor 和 optimization，对调度效果进行分析。",
      status: "已实现（仍可继续增强）",
      touchpoint: "利用结果反馈优化调度，不直接执行 LLM 请求",
      highlight: false,
    },
  ];

  return (
    <div className="space-y-8">
      <section className="relative overflow-hidden rounded-[32px] border border-cyan-500/20 bg-[radial-gradient(circle_at_top_left,rgba(34,211,238,0.22),transparent_28%),radial-gradient(circle_at_top_right,rgba(16,185,129,0.18),transparent_22%),linear-gradient(135deg,rgba(12,18,30,0.98),rgba(18,26,43,0.94))] p-8 text-white shadow-2xl">
        <div className="absolute inset-0 opacity-30 bg-[linear-gradient(rgba(255,255,255,0.06)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.06)_1px,transparent_1px)] bg-[size:28px_28px]" />
        <div className="relative grid gap-8 xl:grid-cols-[1.15fr_0.85fr]">
          <div className="space-y-5">
            <Badge variant="info" className="w-fit">Defense Demo Center</Badge>
            <div>
              <h1 className="text-4xl font-black tracking-tight">Agent 异构网关完整演示前端</h1>
              <p className="mt-3 max-w-3xl text-sm leading-7 text-slate-200">
                这个页面把答辩所需能力串成一条完整故事：异构网关、LLM 增强调度、routing memory、治理控制，以及 Agent Town 演化实验层。
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <Button asChild>
                <Link href="/">
                  进入总览
                  <ArrowRight className="h-4 w-4" />
                </Link>
              </Button>
              <Button asChild variant="outline">
                <Link href="/agent-town">
                  打开 Agent Town
                  <GitBranch className="h-4 w-4" />
                </Link>
              </Button>
            </div>
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
              <MetricBox label="30 天请求数" value={formatNumber(totalRequests)} icon={Activity} />
              <MetricBox label="30 天 Token" value={formatNumber(totalTokens)} icon={Sparkles} />
              <MetricBox label="平均延迟" value={`${avgLatency}ms`} icon={Network} />
              <MetricBox label="成功率" value={formatPercent(successRate)} icon={CheckCircle2} />
            </div>
          </div>

          <Card className="border-white/10 bg-white/5 text-white backdrop-blur">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-xl"><BrainCircuit className="h-5 w-5 text-cyan-300" />现场讲解提纲</CardTitle>
              <CardDescription className="text-slate-300">适合答辩时边点页面边讲。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-slate-200">
              {demoSteps.map((step, index) => (
                <div key={step} className="flex gap-3 rounded-2xl border border-white/10 bg-white/5 p-3">
                  <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-cyan-400/20 font-black text-cyan-200">{index + 1}</div>
                  <p className="leading-6">{step}</p>
                </div>
              ))}
            </CardContent>
          </Card>
        </div>
      </section>

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <QuickLinkCard href="/" title="异构网关总览" desc="展示全局请求、成本、延迟与调度建议。" icon={Activity} />
        <QuickLinkCard href="/agent-town" title="Agent Town" desc="展示演化实验层与 routing memory 的生成。" icon={GitBranch} />
        <QuickLinkCard href="/api-keys" title="API Keys" desc="展示访问控制、预算与 API Key 治理。" icon={KeyRound} />
        <QuickLinkCard href="/audit-logs" title="Audit Logs" desc="展示系统可审计性与操作留痕。" icon={FileText} />
      </section>

      <section>
        <Card className="border-cyan-500/20 bg-[linear-gradient(135deg,rgba(8,16,30,0.98),rgba(10,29,36,0.94))] text-white shadow-xl">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-xl"><BrainCircuit className="h-5 w-5 text-cyan-300" />AI Traffic Scheduling Hub</CardTitle>
            <CardDescription className="text-slate-300">突出展示异构网关如何用 AI 调度策略把流量按延迟、成功率与成本目标进行分流。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-5">
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              {schedulingHighlights.map((item) => (
                <div key={item.label} className="rounded-2xl border border-white/10 bg-white/5 p-4">
                  <div className="text-xs uppercase tracking-[0.18em] text-slate-300">{item.label}</div>
                  <div className="mt-2 text-2xl font-black">{item.value}</div>
                </div>
              ))}
            </div>
            <div className="grid gap-4 xl:grid-cols-[1.2fr_0.8fr]">
              <div className="rounded-2xl border border-white/10 bg-white/5 p-5">
                <div className="mb-3 text-sm font-semibold text-cyan-200">网关调度链路</div>
                <div className="grid gap-3 md:grid-cols-4">
                  {[
                    "接收统一 OpenAI 风格请求",
                    "识别延迟 / 成本 / 成功率目标",
                    "选择最优 provider 与策略",
                    "回传结果并记录 routing memory",
                  ].map((step, index) => (
                    <div key={step} className="rounded-xl border border-white/10 bg-black/10 p-3 text-sm leading-6 text-slate-200">
                      <div className="mb-2 text-xs font-black uppercase tracking-[0.16em] text-cyan-300">Step {index + 1}</div>
                      {step}
                    </div>
                  ))}
                </div>
              </div>
              <div className="rounded-2xl border border-emerald-500/20 bg-emerald-500/5 p-5">
                <div className="mb-3 text-sm font-semibold text-emerald-300">当前调度结论</div>
                <p className="text-sm leading-7 text-slate-200">
                  {comparison
                    ? `当前网关已根据最新 memory 与策略推导结果，将流量优先分配到 ${comparison.optimized.provider}，并采用 ${comparison.optimized.strategy} 策略，以优先降低延迟并提升成功率。`
                    : advisor?.summary ?? "当前系统正在根据 provider 状态、历史表现和预算目标生成调度建议。"}
                </p>
                <div className="mt-4 flex flex-wrap gap-2">
                  <Badge variant="success">AI Scheduling Active</Badge>
                  <Badge variant="info">Gateway-first</Badge>
                  <Badge variant="warning">Traffic-aware Routing</Badge>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </section>

      <section>
        <Card className="glass-card border-fuchsia-500/20 bg-[radial-gradient(circle_at_top,rgba(192,132,252,0.12),transparent_35%),linear-gradient(135deg,rgba(18,15,35,0.98),rgba(10,18,32,0.96))] text-white shadow-xl">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-xl"><Network className="h-5 w-5 text-fuchsia-300" />六层异构网关可视化</CardTitle>
            <CardDescription className="text-slate-300">这一块专门回答“哪一层真正用到了大模型”——真实 LLM 调用发生在第 4 层，请求执行层。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-5">
            <div className="rounded-2xl border border-fuchsia-400/20 bg-white/5 p-4 text-sm leading-7 text-slate-200">
              <span className="font-bold text-fuchsia-200">真实大模型调用说明：</span>
              当前系统里，真正向 DeepSeek 等底层模型发起请求的是 <span className="font-bold text-white">L4 请求执行层</span>；
              L3 负责“选谁来处理”，L6 负责“把历史结果变成 memory 和优化建议”，但真正把请求发给大模型的是执行层。
            </div>
            <div className="grid gap-4 md:grid-cols-3">
              <div className="rounded-2xl border border-white/10 bg-black/10 p-4 text-sm text-slate-200">
                <div className="text-xs uppercase tracking-[0.18em] text-slate-400">真正用到 LLM</div>
                <div className="mt-2 text-lg font-black text-emerald-300">L4 请求执行层</div>
              </div>
              <div className="rounded-2xl border border-white/10 bg-black/10 p-4 text-sm text-slate-200">
                <div className="text-xs uppercase tracking-[0.18em] text-slate-400">做调度决策</div>
                <div className="mt-2 text-lg font-black text-cyan-300">L3 AI 流量调度层</div>
              </div>
              <div className="rounded-2xl border border-white/10 bg-black/10 p-4 text-sm text-slate-200">
                <div className="text-xs uppercase tracking-[0.18em] text-slate-400">沉淀与复用经验</div>
                <div className="mt-2 text-lg font-black text-fuchsia-300">L6 观测优化层</div>
              </div>
            </div>
            <div className="grid gap-4 xl:grid-cols-2">
              {sixLayers.map((item) => (
                <div
                  key={item.layer}
                  className={`rounded-3xl border p-5 transition-all ${item.highlight
                    ? "border-emerald-400/40 bg-emerald-500/10 shadow-[0_0_0_1px_rgba(52,211,153,0.18)]"
                    : "border-white/10 bg-white/5"
                    }`}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="text-xs font-black uppercase tracking-[0.22em] text-slate-400">{item.layer}</div>
                      <div className="mt-1 text-xl font-black">{item.title}</div>
                    </div>
                    <Badge variant={item.highlight ? "success" : "info"}>{item.status}</Badge>
                  </div>
                  <p className="mt-4 text-sm leading-7 text-slate-200">{item.desc}</p>
                  <div className={`mt-4 rounded-2xl border px-4 py-3 text-sm ${item.highlight
                    ? "border-emerald-400/30 bg-emerald-500/10 text-emerald-100"
                    : "border-white/10 bg-black/10 text-slate-300"
                    }`}>
                    {item.touchpoint}
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </section>

      <section>
        <Card className="glass-card border-emerald-500/20 bg-[radial-gradient(circle_at_top_left,rgba(16,185,129,0.16),transparent_28%),linear-gradient(135deg,rgba(8,18,22,0.98),rgba(11,26,32,0.96))] text-white shadow-xl">
          <CardHeader>
            <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <CardTitle className="flex items-center gap-2 text-xl"><Activity className="h-5 w-5 text-emerald-300" />模拟流量入口</CardTitle>
                <CardDescription className="text-slate-300">点击一次按钮，模拟一条请求走完整个六层网关链路，直观看到调度与执行落点。</CardDescription>
              </div>
              <Button onClick={runSimulation} disabled={simulating} className="bg-emerald-500 text-black hover:bg-emerald-400">
                {simulating ? "正在模拟..." : "发起模拟流量"}
              </Button>
            </div>
          </CardHeader>
          <CardContent className="space-y-5">
            {simulation ? (
              <>
                <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                  <div className="rounded-2xl border border-white/10 bg-white/5 p-4"><div className="text-xs uppercase tracking-[0.18em] text-slate-400">请求 ID</div><div className="mt-2 text-sm font-bold text-white">{simulation.request_id}</div></div>
                  <div className="rounded-2xl border border-white/10 bg-white/5 p-4"><div className="text-xs uppercase tracking-[0.18em] text-slate-400">调度策略</div><div className="mt-2 text-sm font-bold text-cyan-300">{simulation.recommended_strategy}</div></div>
                  <div className="rounded-2xl border border-white/10 bg-white/5 p-4"><div className="text-xs uppercase tracking-[0.18em] text-slate-400">执行 Provider</div><div className="mt-2 text-sm font-bold text-emerald-300">{simulation.recommended_provider}</div></div>
                  <div className="rounded-2xl border border-white/10 bg-white/5 p-4"><div className="text-xs uppercase tracking-[0.18em] text-slate-400">执行模型</div><div className="mt-2 text-sm font-bold text-fuchsia-300">{simulation.recommended_model}</div></div>
                </div>
                <div className="rounded-2xl border border-white/10 bg-black/10 p-4 text-sm leading-7 text-slate-200">
                  <div><span className="font-semibold text-emerald-300">执行模式：</span>{simulation.execution_mode}</div>
                  <div className="mt-2"><span className="font-semibold text-cyan-300">Memory 影响：</span>{simulation.memory_influence}</div>
                  <div className="mt-2"><span className="font-semibold text-white">最终结论：</span>{simulation.final_summary}</div>
                </div>
                <div className="grid gap-4 xl:grid-cols-2">
                  {simulation.layers.map((layer) => (
                    <div key={layer.layer} className={`rounded-3xl border p-5 ${layer.highlighted ? "border-emerald-400/30 bg-emerald-500/10" : "border-white/10 bg-white/5"}`}>
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <div className="text-xs uppercase tracking-[0.2em] text-slate-400">{layer.layer}</div>
                          <div className="mt-1 text-lg font-black text-white">{layer.title}</div>
                        </div>
                        <Badge variant={layer.touches_llm ? "success" : "info"}>{layer.touches_llm ? "真实 LLM 触点" : layer.status}</Badge>
                      </div>
                      <p className="mt-4 text-sm leading-7 text-slate-200">{layer.summary}</p>
                    </div>
                  ))}
                </div>
              </>
            ) : (
              <div className="rounded-2xl border border-dashed border-white/15 p-6 text-sm text-slate-300">还没有模拟记录。点击上方按钮后，这里会展示一次请求如何依次经过六层网关。</div>
            )}
          </CardContent>
        </Card>
      </section>

      <Tabs defaultValue="advisor" className="space-y-4">
        <TabsList className="grid w-full max-w-3xl grid-cols-4">
          <TabsTrigger value="advisor">调度建议</TabsTrigger>
          <TabsTrigger value="optimization">优化效果</TabsTrigger>
          <TabsTrigger value="memory">Routing Memory</TabsTrigger>
          <TabsTrigger value="governance">治理入口</TabsTrigger>
        </TabsList>

        <TabsContent value="advisor">
          <Card className="glass-card">
            <CardHeader>
              <CardTitle className="flex items-center gap-2"><BrainCircuit className="h-5 w-5 text-cyan-400" />LLM 增强调度建议</CardTitle>
              <CardDescription>推荐策略、推荐 provider、风险提示与后续动作。</CardDescription>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="grid gap-4 md:grid-cols-3">{[1, 2, 3].map((n) => <div key={n} className="h-28 animate-pulse rounded-2xl bg-muted" />)}</div>
              ) : advisor ? (
                <div className="grid gap-4 xl:grid-cols-[1.1fr_0.9fr]">
                  <div className="space-y-4">
                    <div className="grid gap-4 md:grid-cols-3">
                      <InfoCard title="推荐策略" value={advisor.recommended_strategy} />
                      <InfoCard title="推荐 Provider" value={advisor.recommended_provider} />
                      <InfoCard title="置信度" value={`${Math.round(advisor.confidence * 100)}%`} />
                    </div>
                    <Block title="摘要">{advisor.summary}</Block>
                  </div>
                  <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-1">
                    <BulletBlock title="原因" items={advisor.reasons} />
                    <BulletBlock title="风险与动作" items={[...advisor.risks, ...advisor.next_actions]} />
                  </div>
                </div>
              ) : (
                <div className="rounded-2xl border border-dashed p-6 text-sm text-muted-foreground">当前未能获取调度建议。</div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="optimization">
          <Card className="glass-card">
            <CardHeader>
              <CardTitle className="flex items-center gap-2"><Activity className="h-5 w-5 text-emerald-400" />流量路由优化效果</CardTitle>
              <CardDescription>对比基线策略与优化策略，展示异构网关在延迟、成功率与成本上的调度收益。</CardDescription>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="grid gap-4 md:grid-cols-2">{[1, 2].map((n) => <div key={n} className="h-40 animate-pulse rounded-2xl bg-muted" />)}</div>
              ) : comparison ? (
                <div className="space-y-4">
                  <div className="grid gap-4 lg:grid-cols-2">
                    <div className="rounded-2xl border p-5">
                      <div className="mb-3 text-xs font-black uppercase tracking-[0.18em] text-muted-foreground">Baseline</div>
                      <div className="space-y-2 text-sm">
                        <div><span className="font-semibold">策略：</span>{comparison.baseline.strategy}</div>
                        <div><span className="font-semibold">Provider：</span>{comparison.baseline.provider}</div>
                        <div><span className="font-semibold">平均延迟：</span>{comparison.baseline.avg_latency_ms} ms</div>
                        <div><span className="font-semibold">成功率：</span>{comparison.baseline.success_rate.toFixed(1)}%</div>
                        <div><span className="font-semibold">每千请求成本：</span>${comparison.baseline.cost_per_1k_requests.toFixed(2)}</div>
                      </div>
                    </div>
                    <div className="rounded-2xl border border-emerald-500/30 bg-emerald-500/5 p-5">
                      <div className="mb-3 text-xs font-black uppercase tracking-[0.18em] text-emerald-400">Optimized</div>
                      <div className="space-y-2 text-sm">
                        <div><span className="font-semibold">策略：</span>{comparison.optimized.strategy}</div>
                        <div><span className="font-semibold">Provider：</span>{comparison.optimized.provider}</div>
                        <div><span className="font-semibold">平均延迟：</span>{comparison.optimized.avg_latency_ms} ms</div>
                        <div><span className="font-semibold">成功率：</span>{comparison.optimized.success_rate.toFixed(1)}%</div>
                        <div><span className="font-semibold">每千请求成本：</span>${comparison.optimized.cost_per_1k_requests.toFixed(2)}</div>
                      </div>
                    </div>
                  </div>
                  <div className="grid gap-4 md:grid-cols-3">
                    <InfoCard title="延迟变化" value={`${comparison.latency_delta_ms} ms`} />
                    <InfoCard title="成功率变化" value={`${comparison.success_rate_delta.toFixed(1)}%`} />
                    <InfoCard title="成本变化" value={`$${comparison.cost_delta_per_1k_requests.toFixed(2)}`} />
                  </div>
                  <Block title="优化结论">{comparison.improvement_summary}</Block>
                  <div className="grid gap-4 lg:grid-cols-2">
                    <BulletBlock title="为什么优化有效" items={comparison.why_it_improved} />
                    <BulletBlock title="建议落地方式" items={comparison.recommended_rollout} />
                  </div>
                </div>
              ) : (
                <div className="rounded-2xl border border-dashed p-6 text-sm text-muted-foreground">当前未能获取优化效果对比。</div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="memory">
          <Card className="glass-card">
            <CardHeader>
              <div className="flex items-center justify-between gap-3">
                <div>
                  <CardTitle className="flex items-center gap-2"><Network className="h-5 w-5 text-emerald-400" />Routing Memory</CardTitle>
                  <CardDescription>展示网关如何将运行经验沉淀为可回看的 memory。</CardDescription>
                </div>
                <div className="flex gap-2">
                  <Button variant="outline" size="sm" onClick={loadSavedMemory}>刷新本地 Memory</Button>
                  <Button variant="outline" size="sm" onClick={() => { clearSavedRoutingMemory(); setMemory([]); setComparison(null); setActiveMemoryId(null); }}>清空本地 Memory</Button>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <div className="grid gap-4 xl:grid-cols-2">
                {memory.length > 0 ? getSavedRoutingMemory().slice(0, 6).map((bundle) => (
                  <div key={bundle.id} className="rounded-2xl border p-4">
                    <div className="mb-3 flex items-start justify-between gap-3">
                      <div>
                        <div className="text-sm font-bold">{bundle.title}</div>
                        <div className="text-xs text-muted-foreground">{bundle.saved_at}</div>
                      </div>
                      <Badge variant={activeMemoryId === bundle.id ? "default" : "success"}>{activeMemoryId === bundle.id ? "当前复用中" : "已保存"}</Badge>
                    </div>
                    <div className="mb-3 text-sm text-muted-foreground">{bundle.prompt_summary}</div>
                    <div className="space-y-2 text-sm">
                      {bundle.items.slice(0, 3).map((item) => (
                        <div key={item.id} className="rounded-xl border p-3">
                          <div><span className="font-semibold">Signal：</span>{item.signal}</div>
                          <div><span className="font-semibold">Decision：</span>{item.decision}</div>
                          <div><span className="font-semibold">Outcome：</span>{item.outcome}</div>
                        </div>
                      ))}
                    </div>
                    <div className="mt-4 flex gap-2">
                      <Button size="sm" onClick={() => { setActiveRoutingMemory(bundle.id); setActiveMemoryId(bundle.id); }}>设为当前复用</Button>
                      {activeMemoryId === bundle.id ? <Button size="sm" variant="outline" onClick={() => { clearActiveRoutingMemory(); setActiveMemoryId(null); }}>取消复用</Button> : null}
                    </div>
                  </div>
                )) : <div className="rounded-2xl border border-dashed p-6 text-sm text-muted-foreground">当前没有可展示的 memory。</div>}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="governance">
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <QuickLinkCard href="/api-keys" title="API Keys" desc="演示访问控制与预算治理。" icon={KeyRound} />
            <QuickLinkCard href="/teams" title="Teams" desc="演示多团队接入。" icon={Users} />
            <QuickLinkCard href="/organizations" title="Organizations" desc="演示多租户治理。" icon={Building2} />
            <QuickLinkCard href="/audit-logs" title="Audit Logs" desc="演示审计追踪与留痕。" icon={ShieldCheck} />
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}

function MetricBox({ label, value, icon: Icon }: { label: string; value: string; icon: React.ComponentType<{ className?: string }> }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 p-4 backdrop-blur">
      <div className="mb-3 flex items-center gap-2 text-xs uppercase tracking-[0.18em] text-slate-300">
        <Icon className="h-4 w-4" />
        {label}
      </div>
      <div className="text-2xl font-black">{value}</div>
    </div>
  );
}

function InfoCard({ title, value }: { title: string; value: string }) {
  return <div className="rounded-2xl border p-4"><div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{title}</div><div className="mt-2 text-lg font-bold">{value}</div></div>;
}

function Block({ title, children }: { title: string; children: React.ReactNode }) {
  return <div className="rounded-2xl border p-4"><div className="mb-2 text-sm font-semibold">{title}</div><div className="text-sm leading-6 text-muted-foreground">{children}</div></div>;
}

function BulletBlock({ title, items }: { title: string; items: string[] }) {
  return <div className="rounded-2xl border p-4"><div className="mb-2 text-sm font-semibold">{title}</div><ul className="list-disc space-y-2 pl-5 text-sm leading-6 text-muted-foreground">{items.map((item) => <li key={item}>{item}</li>)}</ul></div>;
}

function QuickLinkCard({ href, title, desc, icon: Icon }: { href: string; title: string; desc: string; icon: React.ComponentType<{ className?: string }> }) {
  return (
    <Card className="glass-card">
      <CardContent className="p-5">
        <div className="mb-4 flex h-11 w-11 items-center justify-center rounded-2xl border bg-secondary/30"><Icon className="h-5 w-5 text-cyan-400" /></div>
        <div className="text-lg font-bold">{title}</div>
        <p className="mt-2 min-h-12 text-sm leading-6 text-muted-foreground">{desc}</p>
        <Button asChild variant="outline" className="mt-4 w-full"><Link href={href}>打开页面</Link></Button>
      </CardContent>
    </Card>
  );
}
