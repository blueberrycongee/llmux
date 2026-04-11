"use client";

import Link from "next/link";
import { notFound, useParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { ArrowLeft, BrainCircuit, Network, Play, ShieldCheck, Sparkles } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { apiClient } from "@/lib/api/client";
import type { RealTrafficRunResponse, TrafficSimulationResponse } from "@/types/api";
import { gatewayLayerDefinitions, getGatewayLayerDefinition } from "../layer-definitions";

export default function GatewayLayerDetailPage() {
  const params = useParams<{ layer: string }>();
  const layer = getGatewayLayerDefinition(params.layer);
  const [simulation, setSimulation] = useState<TrafficSimulationResponse | null>(null);
  const [realRun, setRealRun] = useState<RealTrafficRunResponse | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!layer) return;
    const load = async () => {
      setLoading(true);
      try {
        const sim = await apiClient.simulateTraffic({
          prompt: "请模拟一条对低延迟敏感的交互式请求经过异构网关的完整链路。",
          traffic_type: "interactive",
        });
        setSimulation(sim);
        if (layer.code === "L3" || layer.code === "L4") {
          const real = await apiClient.realRun({
            prompt: "请简洁说明这次真实请求为何被路由到当前 provider，并给出一句结果摘要。",
            traffic_type: "interactive",
          });
          setRealRun(real);
        }
      } finally {
        setLoading(false);
      }
    };
    void load();
  }, [layer]);

  const selectedLayer = useMemo(
    () => simulation?.layers.find((item) => item.layer === layer?.code) ?? null,
    [layer, simulation]
  );

  if (!layer) notFound();

  return (
    <div className="space-y-8">
      <section className="rounded-[30px] border border-cyan-500/20 bg-[radial-gradient(circle_at_top_left,rgba(34,211,238,0.18),transparent_30%),linear-gradient(135deg,rgba(10,14,28,0.98),rgba(15,22,40,0.96))] p-8 text-white shadow-2xl">
        <div className="flex flex-col gap-5 xl:flex-row xl:items-end xl:justify-between">
          <div className="space-y-4">
            <Button asChild variant="outline" className="border-white/15 bg-white/5 text-white hover:bg-white/10">
              <Link href="/gateway-visualizer">
                <ArrowLeft className="mr-2 h-4 w-4" />
                返回 Gateway Visualizer
              </Link>
            </Button>
            <div>
              <div className="text-xs uppercase tracking-[0.26em] text-cyan-200">{layer.code}</div>
              <h1 className="mt-2 text-4xl font-black tracking-tight">{layer.title}</h1>
              <p className="mt-3 max-w-3xl text-sm leading-7 text-slate-200">{layer.summary}</p>
            </div>
            <div className="rounded-3xl border border-white/10 bg-white/5 p-4 text-sm leading-7 text-slate-200">
              <span className="font-bold text-white">本层重点：</span>{layer.focus}
            </div>
          </div>
          <Button onClick={() => window.location.reload()} disabled={loading} className="bg-emerald-400 text-black hover:bg-emerald-300">
            <Play className="mr-2 h-4 w-4" />
            {loading ? "正在刷新本层数据..." : "重新试跑"}
          </Button>
        </div>
      </section>

      <section className="grid gap-6 xl:grid-cols-[0.9fr_1.1fr]">
        <Card className="glass-card border-cyan-500/20">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-xl"><BrainCircuit className="h-5 w-5 text-cyan-400" />六层导航</CardTitle>
            <CardDescription>每一层都是一个独立路径，可以单独讲解。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {gatewayLayerDefinitions.map((item) => (
              <Link key={item.slug} href={`/gateway-visualizer/${item.slug}`} className={`block rounded-2xl border p-4 transition-colors ${item.slug === layer.slug ? "border-cyan-400/40 bg-cyan-500/10" : "border-border/60 bg-secondary/20 hover:bg-secondary/35"}`}>
                <div className="text-xs uppercase tracking-[0.2em] text-muted-foreground">{item.code}</div>
                <div className="mt-1 text-lg font-black">{item.title}</div>
                <div className="mt-2 text-sm leading-6 text-muted-foreground">{item.summary}</div>
              </Link>
            ))}
          </CardContent>
        </Card>

        <div className="space-y-6">
          <Card className="glass-card border-fuchsia-500/20">
            <CardHeader>
              <CardTitle className="text-xl">本层模拟链路详情</CardTitle>
              <CardDescription>这部分来自六层 simulation，用于说明本层在网关中的职责。</CardDescription>
            </CardHeader>
            <CardContent>
              {selectedLayer ? (
                <div className="space-y-4">
                  <div className="flex items-center gap-3">
                    <Badge variant="info">{selectedLayer.layer}</Badge>
                    <Badge variant={selectedLayer.touches_llm ? "success" : "outline"}>{selectedLayer.touches_llm ? "触达 LLM" : "支撑层"}</Badge>
                    <Badge variant="default">{selectedLayer.status}</Badge>
                  </div>
                  <div className="rounded-3xl border border-border/60 bg-secondary/20 p-5 text-sm leading-7 text-muted-foreground">
                    {selectedLayer.summary}
                  </div>
                </div>
              ) : (
                <div className="text-sm text-muted-foreground">正在加载本层 simulation 详情...</div>
              )}
            </CardContent>
          </Card>

          {(layer.code === "L3" || layer.code === "L4") && (
            <Card className="glass-card border-emerald-500/20">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-xl"><Network className="h-5 w-5 text-emerald-400" />真实试跑数据</CardTitle>
                <CardDescription>
                  {layer.code === "L3" ? "这里重点看调度输入与调度决策。" : "这里重点看真实请求、真实响应，以及是否真的触发到底层 provider。"}
                </CardDescription>
              </CardHeader>
              <CardContent>
                {realRun ? (
                  <div className="space-y-4">
                    <div className="flex flex-wrap gap-2">
                      <Badge variant={realRun.succeeded ? "success" : "destructive"}>{realRun.succeeded ? "真实调用成功" : "真实调用失败"}</Badge>
                      <Badge variant="info">{realRun.recommended_provider}</Badge>
                      <Badge variant="outline">{realRun.recommended_model}</Badge>
                      <Badge variant="warning">{realRun.recommended_strategy}</Badge>
                    </div>
                    <InfoBlock title="调度输入" icon={<Sparkles className="h-4 w-4 text-cyan-400" />}>{realRun.scheduler_input}</InfoBlock>
                    <InfoBlock title="调度决策" icon={<BrainCircuit className="h-4 w-4 text-emerald-400" />}>{realRun.scheduler_decision}</InfoBlock>
                    {!realRun.succeeded && realRun.error_message && (
                      <div className="rounded-3xl border border-rose-500/30 bg-rose-500/10 p-5 text-sm leading-7 text-rose-200">
                        <div className="mb-2 text-sm font-semibold text-rose-100">真实调用失败原因</div>
                        <div>{realRun.error_message}</div>
                      </div>
                    )}
                    {layer.code === "L4" && (
                      <>
                        <CodeBlock title="真实请求体" value={JSON.stringify(realRun.gateway_request, null, 2)} />
                        <CodeBlock title="真实响应" value={realRun.gateway_response ? JSON.stringify(realRun.gateway_response, null, 2) : (realRun.error_message ?? "暂无响应") } />
                        <InfoBlock title="模型文本输出" icon={<ShieldCheck className="h-4 w-4 text-fuchsia-400" />}>{realRun.llm_output_text || realRun.error_message || "暂无输出"}</InfoBlock>
                      </>
                    )}
                  </div>
                ) : (
                  <div className="text-sm text-muted-foreground">正在发起真实试跑...</div>
                )}
              </CardContent>
            </Card>
          )}
        </div>
      </section>
    </div>
  );
}

function InfoBlock({ title, icon, children }: { title: string; icon: React.ReactNode; children: string }) {
  return (
    <div className="rounded-3xl border border-border/60 bg-secondary/20 p-5">
      <div className="mb-3 flex items-center gap-2 text-sm font-semibold">{icon}{title}</div>
      <div className="text-sm leading-7 text-muted-foreground">{children}</div>
    </div>
  );
}

function CodeBlock({ title, value }: { title: string; value: string }) {
  return (
    <div className="rounded-3xl border border-border/60 bg-black/80 p-5">
      <div className="mb-3 text-sm font-semibold text-slate-200">{title}</div>
      <pre className="overflow-x-auto whitespace-pre-wrap break-words text-xs leading-6 text-emerald-200">{value}</pre>
    </div>
  );
}
