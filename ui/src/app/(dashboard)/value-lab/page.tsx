"use client";

import { useEffect, useMemo, useState } from "react";
import {
  Bot,
  BrainCircuit,
  Clock3,
  Coins,
  Cpu,
  FlaskConical,
  GitBranch,
  Send,
  Sparkles,
  Wrench,
} from "lucide-react";
import { apiClient } from "@/lib/api/client";
import type { ConversationLabResponse, ConversationLabRun } from "@/types/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { useI18n } from "@/i18n/locale-provider";

type ScenarioId = "grounded-repo-qa" | "speed-chat" | "memory-follow-up";

function getValueLabScenarios(locale: "cn" | "i18n") {
  if (locale === "cn") {
    return [
      {
        id: "grounded-repo-qa" as const,
        title: "仓库取证问答",
        description: "强制做 repo-grounded 对比，观察优化链路如何通过 tools 与路由降低幻觉风险。",
        prompt: "请把这个网关项目分析成一个 memory / team / agent / model 路由系统。先用 filesystem 工具确认顶层结构，再解释优化链路相对普通直聊的价值。",
        trafficType: "analysis",
        requireTeam: false,
        requireFilesystem: true,
      },
      {
        id: "speed-chat" as const,
        title: "极速问答",
        description: "测试低复杂度 prompt，看看 baseline 是否更适合简单直接的总结场景。",
        prompt: "请用五条简洁要点总结这个 gateway 项目对开发者的价值。",
        trafficType: "interactive",
        requireTeam: false,
        requireFilesystem: false,
      },
      {
        id: "memory-follow-up" as const,
        title: "记忆追问",
        description: "预置 route/team memory，观察优化链路是否会在 follow-up 问题里复用已有经验。",
        prompt: "继续上一次架构讨论。如果我们想提升 memory-aware routing 的质量，应该优先改哪个子系统？",
        trafficType: "value-demo",
        requireTeam: false,
        requireFilesystem: false,
      },
    ];
  }
  return [
    {
      id: "grounded-repo-qa" as const,
      title: "Grounded Repo QA",
      description: "Force a repo-grounded comparison so the optimized chain can show tool-aware routing and reduce hallucination risk.",
      prompt: "Analyze this gateway project as a memory/team/agent/model routing system. Use filesystem tools to inspect the top-level project structure, then explain how the optimized chain creates value over a plain chat response.",
      trafficType: "analysis",
      requireTeam: false,
      requireFilesystem: true,
    },
    {
      id: "speed-chat" as const,
      title: "Speed Chat",
      description: "Test a low-complexity prompt where the baseline path should often win on raw latency.",
      prompt: "Summarize what this gateway project does for developers in five concise bullets.",
      trafficType: "interactive",
      requireTeam: false,
      requireFilesystem: false,
    },
    {
      id: "memory-follow-up" as const,
      title: "Memory Follow-up",
      description: "Prime route memory and evaluate whether the optimized chain can reuse previous routing knowledge on a follow-up architectural question.",
      prompt: "Continue the earlier architecture discussion and tell me which subsystem should change first if we want to improve memory-aware routing quality.",
      trafficType: "value-demo",
      requireTeam: false,
      requireFilesystem: false,
    },
  ];
}

function formatDelta(value: number, suffix = "") {
  if (value === 0) return `0${suffix}`;
  return `${value > 0 ? "+" : ""}${value}${suffix}`;
}

function formatCost(value: number | undefined) {
  return typeof value === "number" ? value.toFixed(6) : "0.000000";
}

function formatTokens(value: number | undefined) {
  return typeof value === "number" ? value.toLocaleString() : "0";
}

function formatScore(value: number | undefined) {
  return typeof value === "number" ? value.toFixed(2) : "0.00";
}

function variantForDelta(value: number, inverse = false) {
  if (value === 0) return "outline" as const;
  const positive = inverse ? value < 0 : value > 0;
  return positive ? "success" as const : "warning" as const;
}

function winnerVariant(winner: string) {
  switch (winner) {
    case "optimized":
      return "success" as const;
    case "baseline":
      return "warning" as const;
    default:
      return "outline" as const;
  }
}

function getValueLabCopy(locale: "cn" | "i18n") {
  if (locale === "cn") {
    return {
      badge: "价值实验室",
      title: "场景化价值实验台",
      subtitle: "并行触发普通基线对话与优化路由链路，对比速度、grounding、工具执行、route depth、memory reuse 与 team escalation。",
      baselineSummary: "baseline = 非 grounded 的直接回答",
      optimizedSummary: "optimized = memory + team + agent + model + tools",
      controlsTitle: "实验控制",
      controlsRun: "开始对比实验",
      controlsRunning: "实验运行中...",
      controlsPromptPlaceholder: "输入要比较的测试问题...",
      toggleForceTeam: "强制 team route",
      toggleRequireFilesystem: "要求 filesystem 工具",
      toggleTokenOptimization: "token / 成本优化",
      metricLatencyDelta: "延迟差值",
      metricTokenDelta: "Token 差值",
      metricCostDelta: "成本差值",
      metricWinningDimensions: "胜出维度",
      metricWinner: "推荐赢家",
      winnerOptimized: "优化链路更有价值",
      winnerBaseline: "简单直聊更合适",
      winnerInspect: "需要继续观察",
      baselineTitle: "Baseline",
      baselineSubtitle: "非 grounded 的直接回答",
      optimizedTitle: "Optimized",
      optimizedSubtitle: "Memory + team + agent + model + tools",
      fieldProviderModel: "Provider / Model",
      fieldEstimatedCost: "预估成本",
      fieldGroundedness: "Groundedness",
      fieldHallucinationRisk: "幻觉风险",
      fieldToolUse: "工具使用",
      fieldRouteDepth: "路由深度",
      fieldMemory: "Memory",
      fieldTeamRoute: "Team Route",
      fieldTokenOptimization: "Token 优化",
      fieldRequiredTools: "要求工具",
      fieldBoundTools: "绑定工具",
      fieldObservedToolCalls: "实际工具调用",
      fieldCandidateTrail: "候选执行轨迹",
      fieldRoutingReasoning: "路由解释",
      fieldQualityNotes: "质量说明",
      fieldResponse: "回答内容",
      fieldScenario: "实验场景",
      fieldPrompt: "测试问题",
      fieldMemoryState: "Memory",
      fieldTeamAgent: "Team / Agent",
      fieldCandidateModel: "候选模型",
      fieldToolExecution: "工具执行",
      valueNarrativeTitle: "价值解释",
      valueNarrativeDesc: "这些是根据本次运行自动生成的讲解要点。",
      whyTitle: "为什么优化链路有价值",
      whyDesc: "这些能力是普通直聊路径天然不具备的。",
      emptyTitle: "运行实验，生成可讲解的价值对比。",
      emptyDesc: "实验会并行执行普通直聊与优化链路，并自动比较速度、grounding、幻觉风险、工具执行、路由深度、memory reuse 与 team route。",
      statusSuccess: "成功",
      statusFailed: "失败",
      finish: "结束原因",
      enabled: "已开启",
      disabled: "已关闭",
      yes: "是",
      no: "否",
      none: "无",
      responseNone: "暂无输出",
      routeValueVisible: "已体现优化链路价值",
      keepItSimple: "简单场景保留直聊",
      sameLatency: "延迟持平",
      optimizedFaster: "优化链路更快",
      optimizedHeavier: "优化链路更重",
      sameTokenLoad: "Token 持平",
      optimizedLeaner: "优化链路更省",
      optimizedRicher: "优化链路更丰富",
      sameCost: "成本持平",
      optimizedCheaper: "优化链路更省钱",
      optimizedInvestsMore: "优化链路投入更多",
      memoryReused: "复用",
      memoryNotUsed: "本轮未使用",
      noToolCallsObserved: "未观察到工具调用",
      directAnswer: "普通直聊",
      noOutput: "暂无输出",
    };
  }
  return {
    badge: "Value Lab",
    title: "Scenario-Based Value Lab",
    subtitle: "Trigger a plain baseline chat path and the optimized routing chain in parallel, then compare speed, grounding, tool execution, route depth, memory reuse, team escalation, and hallucination risk.",
    baselineSummary: "baseline = ungrounded direct answer",
    optimizedSummary: "optimized = memory + team + agent + model + tools",
    controlsTitle: "Lab Controls",
    controlsRun: "Run comparison",
    controlsRunning: "Running lab...",
    controlsPromptPlaceholder: "Describe the comparison you want to run...",
    toggleForceTeam: "force team route",
    toggleRequireFilesystem: "require filesystem tools",
    toggleTokenOptimization: "token / cost optimization",
    metricLatencyDelta: "Latency Delta",
    metricTokenDelta: "Token Delta",
    metricCostDelta: "Cost Delta",
    metricWinningDimensions: "Winning Dimensions",
    metricWinner: "Recommended Winner",
    winnerOptimized: "route value visible",
    winnerBaseline: "keep it simple",
    winnerInspect: "inspect this run",
    baselineTitle: "Baseline",
    baselineSubtitle: "Ungrounded direct answer",
    optimizedTitle: "Optimized",
    optimizedSubtitle: "Memory + team + agent + model + tools",
    fieldProviderModel: "Provider / Model",
    fieldEstimatedCost: "Estimated Cost",
    fieldGroundedness: "Groundedness",
    fieldHallucinationRisk: "Hallucination Risk",
    fieldToolUse: "Tool Use",
    fieldRouteDepth: "Route Depth",
    fieldMemory: "Memory",
    fieldTeamRoute: "Team Route",
    fieldTokenOptimization: "Token Optimization",
    fieldRequiredTools: "Required Tools",
    fieldBoundTools: "Bound Tools",
    fieldObservedToolCalls: "Observed Tool Calls",
    fieldCandidateTrail: "Candidate Trail",
    fieldRoutingReasoning: "Routing Reasoning",
    fieldQualityNotes: "Quality Notes",
    fieldResponse: "Response",
    fieldScenario: "Scenario",
    fieldPrompt: "Prompt Under Test",
    fieldMemoryState: "Memory",
    fieldTeamAgent: "Team / Agent",
    fieldCandidateModel: "Candidate Model",
    fieldToolExecution: "Tool Execution",
    valueNarrativeTitle: "Value Narrative",
    valueNarrativeDesc: "These are the demo talking points generated from the actual run.",
    whyTitle: "Why The Optimized Chain Matters",
    whyDesc: "Concrete capabilities the direct baseline path does not naturally provide.",
    emptyTitle: "Run the lab to generate a live value comparison.",
    emptyDesc: "The lab will execute a plain baseline chat path and the optimized routing chain in parallel, then score the run on speed, grounding, hallucination risk, tool execution, route depth, memory reuse, and team escalation.",
    statusSuccess: "success",
    statusFailed: "failed",
    finish: "finish",
    enabled: "enabled",
    disabled: "disabled",
    yes: "yes",
    no: "no",
    none: "none",
    responseNone: "No output",
    routeValueVisible: "route value visible",
    keepItSimple: "keep it simple",
    sameLatency: "same latency",
    optimizedFaster: "optimized faster",
    optimizedHeavier: "optimized heavier",
    sameTokenLoad: "same token load",
    optimizedLeaner: "optimized leaner",
    optimizedRicher: "optimized richer",
    sameCost: "same cost",
    optimizedCheaper: "optimized cheaper",
    optimizedInvestsMore: "optimized invests more",
    memoryReused: "reused",
    memoryNotUsed: "not used in this run",
    noToolCallsObserved: "no tool calls observed",
    directAnswer: "ungrounded direct answer",
    noOutput: "No output",
  };
}

function RunCard({ title, subtitle, run, accent }: { title: string; subtitle: string; run: ConversationLabRun; accent: string }) {
  const { locale } = useI18n();
  const copy = getValueLabCopy(locale);
  return (
    <Card className="border-border/60 bg-background/70">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-xl">
          <span className={`inline-flex h-10 w-10 items-center justify-center rounded-2xl ${accent}`}>
            {title === "Baseline" ? <Bot className="h-5 w-5" /> : <BrainCircuit className="h-5 w-5" />}
          </span>
          <div>
            <div>{title}</div>
            <div className="text-sm font-normal text-muted-foreground">{subtitle}</div>
          </div>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4 text-sm">
        <div className="flex flex-wrap gap-2">
          <Badge variant={run.succeeded ? "success" : "destructive"}>{run.succeeded ? copy.statusSuccess : copy.statusFailed}</Badge>
          <Badge variant="outline">{run.latency_ms} ms</Badge>
          <Badge variant="outline">{formatTokens(run.total_tokens)} tokens</Badge>
          {run.finish_reason && <Badge variant="info">{copy.finish}: {run.finish_reason}</Badge>}
          {run.route_source && <Badge variant="warning">{run.route_source}</Badge>}
        </div>

        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <div className="rounded-2xl border border-border/60 bg-secondary/10 p-3">
            <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldProviderModel}</div>
            <div className="mt-2 font-semibold">{run.provider ?? "unknown"} / {run.model ?? "unknown"}</div>
          </div>
          <div className="rounded-2xl border border-border/60 bg-secondary/10 p-3">
            <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldEstimatedCost}</div>
            <div className="mt-2 font-semibold">{formatCost(run.estimated_cost)}</div>
          </div>
          <div className="rounded-2xl border border-border/60 bg-secondary/10 p-3">
            <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldGroundedness}</div>
            <div className="mt-2 font-semibold">{formatScore(run.quality.groundedness_score)}</div>
          </div>
          <div className="rounded-2xl border border-border/60 bg-secondary/10 p-3">
            <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldHallucinationRisk}</div>
            <div className="mt-2 font-semibold">{formatScore(run.quality.hallucination_risk)}</div>
          </div>
        </div>

        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <div className="rounded-2xl border border-border/60 bg-secondary/10 p-3">
            <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldToolUse}</div>
            <div className="mt-2 font-semibold">{run.quality.tool_used ? `${copy.yes} (${run.quality.tool_calls})` : copy.no}</div>
          </div>
          <div className="rounded-2xl border border-border/60 bg-secondary/10 p-3">
            <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldRouteDepth}</div>
            <div className="mt-2 font-semibold">{run.quality.route_depth}</div>
          </div>
          <div className="rounded-2xl border border-border/60 bg-secondary/10 p-3">
            <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldMemory}</div>
            <div className="mt-2 font-semibold">{run.quality.memory_hit ? copy.memoryReused : copy.none}</div>
          </div>
          <div className="rounded-2xl border border-border/60 bg-secondary/10 p-3">
            <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldTeamRoute}</div>
            <div className="mt-2 font-semibold">{run.quality.team_route ? copy.yes : copy.no}</div>
          </div>
        </div>

        <div className="rounded-2xl border border-border/60 bg-secondary/10 p-3">
          <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldTokenOptimization}</div>
          <div className="mt-2 font-semibold">{run.token_optimization_enabled ? copy.enabled : copy.disabled}</div>
          {!!run.optimization_notes?.length && (
            <div className="mt-2 space-y-1 text-xs leading-6 text-muted-foreground">
              {run.optimization_notes.map((item) => <div key={item}>{item}</div>)}
            </div>
          )}
        </div>

        {(run.selected_team_id || run.selected_agent_id || run.selected_candidate) && (
          <div className="rounded-2xl border border-cyan-500/20 bg-cyan-500/5 p-3 text-xs leading-6">
            {run.selected_team_id && <div>team: {run.selected_team_id}</div>}
            {run.selected_agent_id && <div>agent: {run.selected_agent_id}</div>}
            {run.selected_candidate && <div>candidate: {run.selected_candidate}</div>}
            {run.memory_influence && <div>memory: {run.memory_influence}</div>}
          </div>
        )}

        {!!run.required_tools?.length && (
          <div className="space-y-2">
            <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldRequiredTools}</div>
            <div className="flex flex-wrap gap-2">
              {run.required_tools.map((item) => <Badge key={item} variant="outline">{item}</Badge>)}
            </div>
          </div>
        )}

        {!!run.bound_tools?.length && (
          <div className="space-y-2">
            <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldBoundTools}</div>
            <div className="flex flex-wrap gap-2">
              {run.bound_tools.map((item) => <Badge key={item} variant="outline">{item}</Badge>)}
            </div>
          </div>
        )}

        {!!run.quality.tool_names?.length && (
          <div className="space-y-2">
            <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldObservedToolCalls}</div>
            <div className="flex flex-wrap gap-2">
              {run.quality.tool_names.map((item) => <Badge key={item} variant="outline">{item}</Badge>)}
            </div>
          </div>
        )}

        {!!run.candidate_failovers?.length && (
          <div className="space-y-2">
            <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldCandidateTrail}</div>
            <div className="space-y-2">
              {run.candidate_failovers.map((item, index) => (
                <div key={`${item.provider}-${item.model}-${index}`} className="rounded-2xl border border-border/60 bg-secondary/10 p-3 text-xs leading-6">
                  <div>{item.provider}/{item.model} {"->"} {item.outcome}</div>
                  {item.reason && <div className="text-muted-foreground">{item.reason}</div>}
                </div>
              ))}
            </div>
          </div>
        )}

        {!!run.routing_reasoning?.length && (
          <div className="space-y-2">
            <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldRoutingReasoning}</div>
            <div className="space-y-2">
              {run.routing_reasoning.map((item) => (
                <div key={item} className="rounded-2xl border border-border/60 bg-secondary/10 p-3 text-xs leading-6">{item}</div>
              ))}
            </div>
          </div>
        )}

        {!!run.quality.quality_notes?.length && (
          <div className="space-y-2">
            <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldQualityNotes}</div>
            <div className="space-y-2">
              {run.quality.quality_notes.map((item) => (
                <div key={item} className="rounded-2xl border border-border/60 bg-secondary/10 p-3 text-xs leading-6">{item}</div>
              ))}
            </div>
          </div>
        )}

        <div className="space-y-2">
          <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.fieldResponse}</div>
          <pre className="max-h-72 overflow-auto whitespace-pre-wrap rounded-2xl border border-border/60 bg-black/80 p-4 text-xs leading-6 text-slate-200">
            {run.response_text || run.error_message || copy.noOutput}
          </pre>
        </div>
      </CardContent>
    </Card>
  );
}

export default function ValueLabPage() {
  const { locale } = useI18n();
  const copy = getValueLabCopy(locale);
  const scenarios = useMemo(() => getValueLabScenarios(locale), [locale]);
  const initialScenario = scenarios[0];
  const [scenario, setScenario] = useState<ScenarioId>(initialScenario.id);
  const [prompt, setPrompt] = useState<string>(initialScenario.prompt);
  const [trafficType, setTrafficType] = useState<string>(initialScenario.trafficType);
  const [requireTeam, setRequireTeam] = useState<boolean>(initialScenario.requireTeam);
  const [requireFilesystem, setRequireFilesystem] = useState<boolean>(initialScenario.requireFilesystem);
  const [tokenOptimization, setTokenOptimization] = useState<boolean>(true);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<ConversationLabResponse | null>(null);

  const sessionId = useMemo(() => `value-lab-${Date.now()}`, []);
  const activeScenario = scenarios.find((item) => item.id === scenario) ?? initialScenario;

  useEffect(() => {
    setPrompt((prev) => (prev.trim() ? prev : activeScenario.prompt));
  }, [activeScenario.prompt]);

  const applyScenario = (nextScenario: (typeof scenarios)[number]) => {
    setScenario(nextScenario.id);
    setPrompt(nextScenario.prompt);
    setTrafficType(nextScenario.trafficType);
    setRequireTeam(nextScenario.requireTeam);
    setRequireFilesystem(nextScenario.requireFilesystem);
    setResult(null);
    setError(null);
  };

  const runLab = async () => {
    const trimmed = prompt.trim();
    if (!trimmed || running) return;
    setRunning(true);
    setError(null);
    try {
      const response = await apiClient.runConversationLab({
        scenario,
        prompt: trimmed,
        traffic_type: trafficType,
        session_id: sessionId,
        complexity_hint: requireTeam ? 3 : 1,
        require_team: requireTeam,
        required_tools: requireFilesystem ? ["preset-filesystem-list", "preset-filesystem-read"] : [],
        required_capabilities: ["architecture", "coding", "analysis"],
        token_optimization: tokenOptimization,
        cost_optimization: tokenOptimization,
      });
      setResult(response);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to run value lab");
    } finally {
      setRunning(false);
    }
  };

  return (
    <div className="space-y-8">
      <section className="relative overflow-hidden rounded-[32px] border border-cyan-500/20 bg-[radial-gradient(circle_at_top_left,rgba(34,211,238,0.18),transparent_28%),radial-gradient(circle_at_top_right,rgba(251,191,36,0.14),transparent_24%),linear-gradient(135deg,rgba(8,13,24,0.98),rgba(15,22,40,0.94))] p-8 text-white shadow-2xl">
        <div className="absolute inset-0 opacity-20 bg-[linear-gradient(rgba(255,255,255,0.06)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.06)_1px,transparent_1px)] bg-[size:30px_30px]" />
        <div className="relative grid gap-6 xl:grid-cols-[1.15fr_0.85fr]">
          <div className="space-y-4">
            <Badge variant="info" className="w-fit">{copy.badge}</Badge>
            <div>
              <h1 className="text-4xl font-black tracking-tight">{copy.title}</h1>
              <p className="mt-3 max-w-4xl text-sm leading-7 text-slate-200">
                {copy.subtitle}
              </p>
            </div>
            <div className="flex flex-wrap gap-3 text-sm text-slate-200">
              <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">session: {sessionId}</div>
              <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">scenario: {activeScenario.title}</div>
              <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">{copy.baselineSummary}</div>
              <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">{copy.optimizedSummary}</div>
            </div>
          </div>

          <Card className="border-white/10 bg-white/5 text-white backdrop-blur">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-xl"><FlaskConical className="h-5 w-5 text-cyan-300" />{copy.controlsTitle}</CardTitle>
              <CardDescription className="text-slate-300">{activeScenario.description}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-2">
                {scenarios.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => applyScenario(item)}
                    className={`rounded-3xl border px-4 py-4 text-left transition ${scenario === item.id ? "border-cyan-300 bg-cyan-300/10 text-white" : "border-white/10 bg-white/5 text-slate-200 hover:border-cyan-400/60 hover:text-white"}`}
                  >
                    <div className="font-semibold">{item.title}</div>
                    <div className="mt-1 text-sm leading-6 text-slate-300">{item.description}</div>
                  </button>
                ))}
              </div>

              <textarea
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
                className="min-h-40 w-full resize-none rounded-3xl border border-white/10 bg-black/25 px-4 py-4 text-sm leading-7 text-white outline-none"
                placeholder={copy.controlsPromptPlaceholder}
              />

              <div className="flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={() => setRequireTeam((value) => !value)}
                  className={`rounded-full px-3 py-1.5 text-xs transition ${requireTeam ? "bg-emerald-400 text-black" : "border border-white/10 bg-white/5 text-slate-200"}`}
                >
                  {copy.toggleForceTeam}
                </button>
                <button
                  type="button"
                  onClick={() => setRequireFilesystem((value) => !value)}
                  className={`rounded-full px-3 py-1.5 text-xs transition ${requireFilesystem ? "bg-amber-300 text-black" : "border border-white/10 bg-white/5 text-slate-200"}`}
                >
                  {copy.toggleRequireFilesystem}
                </button>
                <button
                  type="button"
                  onClick={() => setTokenOptimization((value) => !value)}
                  className={`rounded-full px-3 py-1.5 text-xs transition ${tokenOptimization ? "bg-violet-300 text-black" : "border border-white/10 bg-white/5 text-slate-200"}`}
                >
                  {copy.toggleTokenOptimization}
                </button>
                <Badge variant="outline">{trafficType}</Badge>
              </div>

              <Button onClick={runLab} disabled={running || !prompt.trim()} className="w-full bg-emerald-400 text-black hover:bg-emerald-300">
                <Send className="mr-2 h-4 w-4" />
                {running ? copy.controlsRunning : copy.controlsRun}
              </Button>
              {error && <div className="rounded-2xl border border-rose-400/30 bg-rose-400/10 px-4 py-3 text-sm text-rose-50">{error}</div>}
            </CardContent>
          </Card>
        </div>
      </section>

      {result && (
        <>
          <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
            <Card className="glass-card border-cyan-500/20">
              <CardContent className="pt-6">
                <div className="flex items-center gap-3">
                  <Clock3 className="h-5 w-5 text-cyan-400" />
                  <div>
                    <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.metricLatencyDelta}</div>
                    <div className="mt-1 text-2xl font-black">{formatDelta(result.latency_delta_ms, " ms")}</div>
                  </div>
                </div>
                <Badge className="mt-4" variant={variantForDelta(result.latency_delta_ms, true)}>
                  {result.latency_delta_ms < 0 ? copy.optimizedFaster : result.latency_delta_ms > 0 ? copy.optimizedHeavier : copy.sameLatency}
                </Badge>
              </CardContent>
            </Card>
            <Card className="glass-card border-emerald-500/20">
              <CardContent className="pt-6">
                <div className="flex items-center gap-3">
                  <Cpu className="h-5 w-5 text-emerald-400" />
                  <div>
                    <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.metricTokenDelta}</div>
                    <div className="mt-1 text-2xl font-black">{formatDelta(result.total_token_delta)}</div>
                  </div>
                </div>
                <Badge className="mt-4" variant={variantForDelta(result.total_token_delta, true)}>
                  {result.total_token_delta < 0 ? copy.optimizedLeaner : result.total_token_delta > 0 ? copy.optimizedRicher : copy.sameTokenLoad}
                </Badge>
              </CardContent>
            </Card>
            <Card className="glass-card border-amber-500/20">
              <CardContent className="pt-6">
                <div className="flex items-center gap-3">
                  <Coins className="h-5 w-5 text-amber-400" />
                  <div>
                    <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.metricCostDelta}</div>
                    <div className="mt-1 text-2xl font-black">{result.cost_delta > 0 ? "+" : ""}{result.cost_delta.toFixed(6)}</div>
                  </div>
                </div>
                <Badge className="mt-4" variant={variantForDelta(result.cost_delta, true)}>
                  {result.cost_delta < 0 ? copy.optimizedCheaper : result.cost_delta > 0 ? copy.optimizedInvestsMore : copy.sameCost}
                </Badge>
              </CardContent>
            </Card>
            <Card className="glass-card border-fuchsia-500/20">
              <CardContent className="pt-6">
                <div className="flex items-center gap-3">
                  <Sparkles className="h-5 w-5 text-fuchsia-400" />
                  <div>
                    <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.metricWinningDimensions}</div>
                    <div className="mt-1 text-2xl font-black">{result.winning_dimensions.length}</div>
                  </div>
                </div>
                <div className="mt-4 flex flex-wrap gap-2">
                  {result.winning_dimensions.map((item) => <Badge key={item} variant="outline">{item}</Badge>)}
                </div>
              </CardContent>
            </Card>
            <Card className="glass-card border-white/20">
              <CardContent className="pt-6">
                <div className="flex items-center gap-3">
                  <FlaskConical className="h-5 w-5 text-white" />
                  <div>
                    <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{copy.metricWinner}</div>
                    <div className="mt-1 text-2xl font-black">{result.recommended_winner}</div>
                  </div>
                </div>
                <Badge className="mt-4" variant={winnerVariant(result.recommended_winner)}>
                  {result.recommended_winner === "optimized" ? copy.routeValueVisible : result.recommended_winner === "baseline" ? copy.keepItSimple : copy.winnerInspect}
                </Badge>
              </CardContent>
            </Card>
          </section>

          <section className="grid gap-6 xl:grid-cols-2">
            <RunCard title={copy.baselineTitle} subtitle={copy.baselineSubtitle} run={result.baseline} accent="bg-cyan-400/15 text-cyan-200" />
            <RunCard title={copy.optimizedTitle} subtitle={copy.optimizedSubtitle} run={result.optimized} accent="bg-emerald-400/15 text-emerald-200" />
          </section>

          <section className="grid gap-6 xl:grid-cols-[1.05fr_0.95fr]">
            <Card className="glass-card border-cyan-500/20">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-xl"><Sparkles className="h-5 w-5 text-cyan-400" />{copy.valueNarrativeTitle}</CardTitle>
                <CardDescription>{copy.valueNarrativeDesc}</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                {result.value_summary.map((item) => (
                  <div key={item} className="rounded-2xl border border-border/60 bg-secondary/10 p-4 text-sm leading-7">{item}</div>
                ))}
              </CardContent>
            </Card>

            <Card className="glass-card border-fuchsia-500/20">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-xl"><GitBranch className="h-5 w-5 text-fuchsia-400" />{copy.whyTitle}</CardTitle>
                <CardDescription>{copy.whyDesc}</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3 text-sm leading-7">
                <div className="rounded-2xl border border-border/60 bg-secondary/10 p-4">
                  <div className="font-semibold">{copy.fieldScenario}</div>
                  <div className="mt-2 text-muted-foreground">{result.scenario}</div>
                </div>
                <div className="rounded-2xl border border-border/60 bg-secondary/10 p-4">
                  <div className="font-semibold">{copy.fieldMemoryState}</div>
                  <div className="mt-2 text-muted-foreground">{result.optimized.quality.memory_hit ? `${copy.memoryReused} (${result.optimized.memory_influence})` : copy.memoryNotUsed}</div>
                </div>
                <div className="rounded-2xl border border-border/60 bg-secondary/10 p-4">
                  <div className="font-semibold">{copy.fieldTeamAgent}</div>
                  <div className="mt-2 text-muted-foreground">team={result.optimized.selected_team_id || "none"} / agent={result.optimized.selected_agent_id || "none"}</div>
                </div>
                <div className="rounded-2xl border border-border/60 bg-secondary/10 p-4">
                  <div className="font-semibold">{copy.fieldCandidateModel}</div>
                  <div className="mt-2 text-muted-foreground">{result.optimized.selected_candidate || `${result.optimized.provider}/${result.optimized.model}`}</div>
                </div>
                <div className="rounded-2xl border border-border/60 bg-secondary/10 p-4">
                  <div className="font-semibold">{copy.fieldToolExecution}</div>
                  <div className="mt-2 text-muted-foreground">{result.optimized.quality.tool_used ? `${result.optimized.quality.tool_calls} tool call(s)` : copy.noToolCallsObserved}</div>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {(result.optimized.quality.tool_names || []).map((item) => <Badge key={item} variant="outline">{item}</Badge>)}
                  </div>
                </div>
                <div className="rounded-2xl border border-border/60 bg-secondary/10 p-4">
                  <div className="font-semibold">{copy.fieldPrompt}</div>
                  <p className="mt-2 text-muted-foreground">{result.prompt}</p>
                </div>
              </CardContent>
            </Card>
          </section>
        </>
      )}

      {!result && (
        <Card className="glass-card border-dashed border-cyan-500/30">
          <CardContent className="flex flex-col items-center justify-center gap-4 py-16 text-center">
            <div className="inline-flex h-16 w-16 items-center justify-center rounded-3xl bg-cyan-400/10 text-cyan-300">
              <FlaskConical className="h-8 w-8" />
            </div>
            <div className="space-y-2">
              <div className="text-xl font-semibold">{copy.emptyTitle}</div>
              <p className="max-w-2xl text-sm leading-7 text-muted-foreground">
                {copy.emptyDesc}
              </p>
            </div>
            <div className="flex flex-wrap justify-center gap-2">
              <Badge variant="outline"><Bot className="mr-1 h-3.5 w-3.5" />{copy.directAnswer}</Badge>
              <Badge variant="outline"><BrainCircuit className="mr-1 h-3.5 w-3.5" />agent routing</Badge>
              <Badge variant="outline"><GitBranch className="mr-1 h-3.5 w-3.5" />team route</Badge>
              <Badge variant="outline"><Wrench className="mr-1 h-3.5 w-3.5" />tool-aware</Badge>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
