"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { Activity, BrainCircuit, Network, Play, ShieldCheck, Sparkles } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { apiClient } from "@/lib/api/client";
import type { TrafficSimulationResponse } from "@/types/api";
import { gatewayLayerDefinitions } from "./layer-definitions";

const layerMeta = {
  L1: { accent: "from-sky-500/20 to-cyan-500/10", border: "border-sky-400/20", icon: Activity, label: "入口" },
  L2: { accent: "from-violet-500/20 to-fuchsia-500/10", border: "border-violet-400/20", icon: Sparkles, label: "适配" },
  L3: { accent: "from-cyan-500/20 to-emerald-500/10", border: "border-cyan-400/20", icon: BrainCircuit, label: "调度" },
  L4: { accent: "from-emerald-500/25 to-lime-500/10", border: "border-emerald-400/30", icon: Network, label: "执行" },
  L5: { accent: "from-amber-500/20 to-orange-500/10", border: "border-amber-400/20", icon: ShieldCheck, label: "治理" },
  L6: { accent: "from-pink-500/20 to-fuchsia-500/10", border: "border-pink-400/20", icon: Sparkles, label: "优化" },
} as const;

export default function GatewayVisualizerPage() {
  const [simulation, setSimulation] = useState<TrafficSimulationResponse | null>(null);
  const [selectedLayer, setSelectedLayer] = useState("L4");
  const [simulating, setSimulating] = useState(false);

  const runSimulation = async () => {
    setSimulating(true);
    try {
      const result = await apiClient.simulateTraffic({
        prompt: "请模拟一条对低延迟敏感的交互式请求经过异构网关的完整链路。",
        traffic_type: "interactive",
      });
      setSimulation(result);
      setSelectedLayer("L4");
    } finally {
      setSimulating(false);
    }
  };

  const selected = useMemo(
    () => simulation?.layers.find((layer) => layer.layer === selectedLayer) ?? null,
    [simulation, selectedLayer]
  );

  return (
    <div className="space-y-8">
      <section className="relative overflow-hidden rounded-[32px] border border-cyan-500/20 bg-[radial-gradient(circle_at_top_left,rgba(34,211,238,0.18),transparent_28%),radial-gradient(circle_at_top_right,rgba(52,211,153,0.16),transparent_22%),linear-gradient(135deg,rgba(10,14,28,0.98),rgba(15,22,40,0.96))] p-8 text-white shadow-2xl">
        <div className="absolute inset-0 opacity-20 bg-[linear-gradient(rgba(255,255,255,0.05)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.05)_1px,transparent_1px)] bg-[size:26px_26px]" />
        <div className="relative flex flex-col gap-6 xl:flex-row xl:items-end xl:justify-between">
          <div className="max-w-4xl space-y-4">
            <Badge variant="info" className="w-fit">Gateway Visualizer</Badge>
            <div>
              <h1 className="text-4xl font-black tracking-tight">六层异构网关链路观察台</h1>
              <p className="mt-3 text-sm leading-7 text-slate-200">
                这个页面不再把六层架构当说明卡片，而是把它变成一个真正的可视化观察台：上方触发模拟流量，
                中部看请求逐层穿过网关，右侧看当前层的细节，底部看本次调度与 memory 对执行的影响。
              </p>
            </div>
          </div>
          <Button onClick={runSimulation} disabled={simulating} className="bg-emerald-400 text-black hover:bg-emerald-300">
            <Play className="mr-2 h-4 w-4" />
            {simulating ? "正在发起模拟流量..." : "发起模拟流量"}
          </Button>
        </div>
      </section>

      <section className="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
        <Card className="glass-card border-cyan-500/20">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-xl"><Network className="h-5 w-5 text-cyan-400" />六层链路总览</CardTitle>
            <CardDescription>点击任意一层，查看该层在本次请求中的具体动作。</CardDescription>
          </CardHeader>
          <CardContent>
            {simulation ? (
              <div className="space-y-4">
                {simulation.layers.map((layer, index) => {
                  const meta = layerMeta[layer.layer as keyof typeof layerMeta];
                  const Icon = meta.icon;
                  const active = selectedLayer === layer.layer;
                  const layerRoute = gatewayLayerDefinitions.find((item) => item.code === layer.layer);
                  return (
                    <Link
                      key={layer.layer}
                      href={layerRoute ? `/gateway-visualizer/${layerRoute.slug}` : "/gateway-visualizer"}
                      onClick={() => setSelectedLayer(layer.layer)}
                      className={`block w-full rounded-3xl border p-5 text-left transition-all ${active ? `${meta.border} bg-gradient-to-r ${meta.accent} shadow-[0_0_0_1px_rgba(34,211,238,0.1)]` : "border-border/60 bg-secondary/20 hover:bg-secondary/35"}`}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="flex items-start gap-4">
                          <div className="flex h-12 w-12 items-center justify-center rounded-2xl border border-white/10 bg-black/15">
                            <Icon className="h-5 w-5 text-white" />
                          </div>
                          <div>
                            <div className="text-xs uppercase tracking-[0.22em] text-muted-foreground">Step {index + 1} · {layer.layer}</div>
                            <div className="mt-1 text-xl font-black">{layer.title}</div>
                            <p className="mt-2 text-sm leading-7 text-muted-foreground">{layer.summary}</p>
                            <div className="mt-3 text-xs font-semibold uppercase tracking-[0.18em] text-cyan-300">点击进入本层详情页</div>
                          </div>
                        </div>
                        <div className="flex flex-col items-end gap-2">
                          <Badge variant={layer.touches_llm ? "success" : "info"}>{layer.touches_llm ? "真实 LLM" : meta.label}</Badge>
                          <Badge variant={active ? "default" : "outline"}>{layer.status}</Badge>
                        </div>
                      </div>
                    </Link>
                  );
                })}
              </div>
            ) : (
              <div className="rounded-3xl border border-dashed p-8 text-sm text-muted-foreground">
                还没有链路数据。点击上方“发起模拟流量”后，这里会按六层展示一次请求如何穿过网关。
              </div>
            )}
          </CardContent>
        </Card>

        <div className="space-y-6">
          <Card className="glass-card border-emerald-500/20">
            <CardHeader>
              <CardTitle className="text-xl">当前层细节</CardTitle>
              <CardDescription>右侧始终显示你当前选中层的执行细节。</CardDescription>
            </CardHeader>
            <CardContent>
              {selected ? (
                <div className="space-y-4">
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <div className="text-xs uppercase tracking-[0.2em] text-muted-foreground">{selected.layer}</div>
                      <div className="mt-1 text-2xl font-black">{selected.title}</div>
                    </div>
                    <Badge variant={selected.touches_llm ? "success" : "info"}>{selected.touches_llm ? "真实大模型触点" : selected.status}</Badge>
                  </div>
                  <div className="rounded-3xl border border-border/60 bg-secondary/20 p-5 text-sm leading-7 text-muted-foreground">
                    {selected.summary}
                  </div>
                  <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-1">
                    <InfoMetric label="当前状态" value={selected.status} />
                    <InfoMetric label="是否触达 LLM" value={selected.touches_llm ? "是，发生在该层" : "否，属于支撑层"} />
                  </div>
                </div>
              ) : (
                <div className="rounded-3xl border border-dashed p-6 text-sm text-muted-foreground">先发起一次模拟流量，再选择一层查看细节。</div>
              )}
            </CardContent>
          </Card>

          <Card className="glass-card border-fuchsia-500/20">
            <CardHeader>
              <CardTitle className="text-xl">本次请求结果摘要</CardTitle>
              <CardDescription>把调度、执行与 memory 影响收敛成一组可讲解的数据。</CardDescription>
            </CardHeader>
            <CardContent>
              {simulation ? (
                <div className="space-y-4">
                  <div className="grid gap-3 md:grid-cols-2">
                    <InfoMetric label="调度策略" value={simulation.recommended_strategy} />
                    <InfoMetric label="执行 Provider" value={simulation.recommended_provider} />
                    <InfoMetric label="执行模型" value={simulation.recommended_model} />
                    <InfoMetric label="执行模式" value={simulation.execution_mode} />
                  </div>
                  <div className="rounded-3xl border border-border/60 bg-secondary/20 p-5 text-sm leading-7 text-muted-foreground">
                    <div><span className="font-semibold text-foreground">Memory 影响：</span>{simulation.memory_influence}</div>
                    <div className="mt-2"><span className="font-semibold text-foreground">最终结论：</span>{simulation.final_summary}</div>
                  </div>
                </div>
              ) : (
                <div className="rounded-3xl border border-dashed p-6 text-sm text-muted-foreground">发起模拟流量后，这里会显示 provider、model、strategy 和 memory 影响。</div>
              )}
            </CardContent>
          </Card>
        </div>
      </section>
    </div>
  );
}

function InfoMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-border/60 bg-secondary/20 p-4">
      <div className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{label}</div>
      <div className="mt-2 text-sm font-bold leading-6">{value}</div>
    </div>
  );
}
