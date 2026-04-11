"use client";

import { useEffect, useMemo, useState } from "react";
import { Bot, BrainCircuit, CheckCircle2, Cpu, MessageSquare, Route, Send, Sparkles } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { apiClient } from "@/lib/api/client";
import type { AgentChatResponse, ConversationAgent, RouteMemoryRecord } from "@/types/api";

type ChatMessage = { role: "user" | "assistant"; content: string; meta?: AgentChatResponse };

const suggestedPrompts = [
  "帮我分析这个 Go 网关项目应该如何演进成 memory-driven 的多 agent 对话系统。",
  "我想写一篇关于异构大模型网关的论文摘要，应该怎么组织研究问题和贡献点？",
  "请帮我把一段中文产品介绍润色成适合英文官网首页的版本。",
];

export default function AgentRouterPage() {
  const [agents, setAgents] = useState<ConversationAgent[]>([]);
  const [memory, setMemory] = useState<RouteMemoryRecord[]>([]);
  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      role: "assistant",
      content: "你好，我是多 Agent 对话路由台。你可以问代码、论文、写作或通用问题，我会自动路由到最合适的 Agent。",
    },
  ]);
  const [input, setInput] = useState("请帮我分析这个 Go 网关项目应该如何演进成 memory-driven 的多 agent 对话系统。 ");
  const [sending, setSending] = useState(false);
  const [sessionId] = useState(() => `session-${Date.now()}`);

  useEffect(() => {
    let active = true;
    const load = async () => {
      const [agentsRes, memoryRes] = await Promise.all([
        apiClient.listConversationAgents(),
        apiClient.getConversationMemory(),
      ]);
      if (!active) return;
      setAgents(agentsRes.data ?? []);
      setMemory(memoryRes.data ?? []);
    };
    void load();
    return () => {
      active = false;
    };
  }, []);

  const latestMeta = useMemo(() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i].meta) return messages[i].meta ?? null;
    }
    return null;
  }, [messages]);

  const sendMessage = async () => {
    const trimmed = input.trim();
    if (!trimmed || sending) return;
    const nextMessages = [...messages, { role: "user" as const, content: trimmed }];
    setMessages(nextMessages);
    setInput("");
    setSending(true);
    try {
      const payload = nextMessages
        .filter((item) => item.role === "user" || item.role === "assistant")
        .map((item) => ({ role: item.role, content: item.content }));
      const result = await apiClient.conversationChat({ session_id: sessionId, messages: payload });
      setMessages((prev) => [...prev, { role: "assistant", content: result.assistant_message || result.error_message || "暂无回复", meta: result }]);
      const memoryRes = await apiClient.getConversationMemory();
      setMemory(memoryRes.data ?? []);
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="space-y-8">
      <section className="relative overflow-hidden rounded-[32px] border border-cyan-500/20 bg-[radial-gradient(circle_at_top_left,rgba(34,211,238,0.18),transparent_30%),radial-gradient(circle_at_top_right,rgba(251,191,36,0.14),transparent_22%),linear-gradient(135deg,rgba(8,13,24,0.98),rgba(15,22,40,0.94))] p-8 text-white shadow-2xl">
        <div className="absolute inset-0 opacity-20 bg-[linear-gradient(rgba(255,255,255,0.06)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.06)_1px,transparent_1px)] bg-[size:28px_28px]" />
        <div className="relative grid gap-6 xl:grid-cols-[1.15fr_0.85fr]">
          <div className="space-y-4">
            <Badge variant="info" className="w-fit">Conversation Agent Router</Badge>
            <div>
              <h1 className="text-4xl font-black tracking-tight">记忆增强的对话式多 Agent 路由台</h1>
              <p className="mt-3 max-w-4xl text-sm leading-7 text-slate-200">
                统一接入多个 Expert Agent，由 Gateway 先做语义理解，再结合 route memory、tool bindings 与当前 agent registry 自动选路。每次问答都会把 query → selected agent → outcome 沉淀成可复用路径。
              </p>
            </div>
            <div className="flex flex-wrap gap-3 text-sm text-slate-200">
              <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">Session: {sessionId}</div>
              <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">Agents: {agents.length}</div>
              <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">Route Memory: {memory.length}</div>
            </div>
          </div>
          <Card className="border-white/10 bg-white/5 text-white backdrop-blur">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-xl"><Route className="h-5 w-5 text-cyan-300" />当前路由解释</CardTitle>
              <CardDescription className="text-slate-300">展示最近一轮问答被路由到哪个 Agent，以及为什么。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-slate-200">
              {latestMeta ? (
                <>
                  <div className="flex flex-wrap gap-2">
                    <Badge variant={latestMeta.succeeded ? "success" : "destructive"}>{latestMeta.succeeded ? "执行成功" : "执行失败"}</Badge>
                    <Badge variant="info">intent: {latestMeta.intent}</Badge>
                    <Badge variant="warning">{latestMeta.route_source}</Badge>
                    <Badge variant="outline">memory: {latestMeta.memory_influence ?? "none"}</Badge>
                    {!!latestMeta.consulted_memories?.length && <Badge variant="outline">consulted {latestMeta.consulted_memories.length}</Badge>}
                  </div>
                  <div className="rounded-2xl border border-white/10 bg-white/5 p-4">
                    {latestMeta.selected_team && (
                      <div className="mb-3 rounded-2xl border border-fuchsia-400/20 bg-fuchsia-400/10 p-3 text-sm text-fuchsia-50">
                        <div className="font-semibold">team route: {latestMeta.selected_team.name}</div>
                        <div className="mt-1 text-fuchsia-100/80">mode: {latestMeta.selected_team.mode}</div>
                        {!!latestMeta.team_participants?.length && <div className="mt-2 flex flex-wrap gap-2">{latestMeta.team_participants.map((item) => <Badge key={item.id} variant="outline">{item.name}</Badge>)}</div>}
                      </div>
                    )}
                    <div className="font-semibold">{latestMeta.selected_agent.name}</div>
                    <div className="mt-1 text-slate-300">{latestMeta.selected_agent.provider} / {latestMeta.selected_agent.model}</div>
                    {latestMeta.selected_candidate && <div className="mt-2 text-xs text-cyan-200">actual candidate: {latestMeta.selected_candidate.provider}/{latestMeta.selected_candidate.model}</div>}
                    {!!latestMeta.selected_agent.candidate_models?.length && <div className="mt-3 flex flex-wrap gap-2">{latestMeta.selected_agent.candidate_models.map((item) => <Badge key={`${item.provider}/${item.model}`} variant="outline">{item.provider}/{item.model}</Badge>)}</div>}
                    {!!latestMeta.selected_agent.candidate_states?.length && <div className="mt-3 grid gap-2">{latestMeta.selected_agent.candidate_states.map((item) => <div key={`${item.provider}/${item.model}`} className="rounded-xl border border-white/10 bg-black/10 px-3 py-2 text-xs"><div className="font-semibold">{item.provider}/{item.model}</div><div className="mt-1 flex flex-wrap gap-2"><Badge variant="outline">{item.status}</Badge><Badge variant="outline">score {item.selection_score?.toFixed(1) ?? "0.0"}</Badge><Badge variant="outline">rpm {item.current_rpm}/{item.rpm_limit ?? "∞"}</Badge><Badge variant="outline">tpm {item.current_tpm}/{item.tpm_limit ?? "∞"}</Badge><Badge variant="outline">selected {item.selected_count}</Badge><Badge variant="outline">failover {item.failover_count}</Badge></div>{item.cooldown_until && <div className="mt-1 text-amber-200">cooldown until {item.cooldown_until}</div>}{item.last_error && <div className="mt-1 text-rose-200 line-clamp-2">{item.last_error}</div>}</div>)}</div>}
                  </div>
                  <div className="space-y-2">
                    {latestMeta.routing_reasoning.map((item) => (
                      <div key={item} className="rounded-2xl border border-white/10 bg-white/5 p-3">{item}</div>
                    ))}
                  </div>
                  {!!latestMeta.candidate_failovers?.length && (
                    <div className="rounded-2xl border border-cyan-400/20 bg-cyan-400/10 p-4 text-sm text-cyan-50">
                      <div className="font-semibold">Candidate execution trail</div>
                      <div className="mt-3 space-y-2">{latestMeta.candidate_failovers.map((item, index) => <div key={`${item.provider}/${item.model}/${index}`} className="rounded-xl border border-white/10 bg-black/10 px-3 py-2"><div>{item.provider}/{item.model} · {item.outcome}</div>{item.reason && <div className="mt-1 text-xs text-cyan-100/80">{item.reason}</div>}</div>)}</div>
                    </div>
                  )}
                  {latestMeta.memory_hit && (
                    <div className="rounded-2xl border border-emerald-400/20 bg-emerald-400/10 p-4 text-sm text-emerald-50">
                      <div className="flex items-center gap-2 font-semibold"><CheckCircle2 className="h-4 w-4 text-emerald-300" />本轮复用了相似历史路径</div>
                      <div className="mt-2 leading-6">历史 query：{latestMeta.memory_hit.query}</div>
                      <div className="mt-1 text-emerald-100/80">历史 agent：{latestMeta.memory_hit.selected_agent_id} · score {latestMeta.memory_hit.outcome_score.toFixed(2)}</div>
                    </div>
                  )}
                  {latestMeta.team_memory_hit && (
                    <div className="rounded-2xl border border-fuchsia-400/20 bg-fuchsia-400/10 p-4 text-sm text-fuchsia-50">
                      <div className="font-semibold">本轮复用了 team memory</div>
                      <div className="mt-2 leading-6">历史 team：{latestMeta.team_memory_hit.selected_team_id}</div>
                      <div className="mt-1 text-fuchsia-100/80">score {latestMeta.team_memory_hit.outcome_score.toFixed(2)}</div>
                    </div>
                  )}
                  {!latestMeta.memory_hit && !!latestMeta.consulted_memories?.length && (
                    <div className="rounded-2xl border border-amber-400/20 bg-amber-400/10 p-4 text-sm text-amber-50">
                      <div className="font-semibold">本轮参考了历史 memory，但没有直接复用旧路径</div>
                      <div className="mt-2 text-amber-100/80">consulted memories: {latestMeta.consulted_memories.length}</div>
                    </div>
                  )}
                </>
              ) : (
                <div className="rounded-2xl border border-dashed border-white/15 p-4 text-slate-300">发送第一条消息后，这里会显示意图识别、选中的 Agent、路由原因与 memory 命中情况。</div>
              )}
            </CardContent>
          </Card>
        </div>
      </section>

      <section className="grid gap-6 xl:grid-cols-[1.15fr_0.85fr]">
        <Card className="glass-card border-cyan-500/20">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-xl"><MessageSquare className="h-5 w-5 text-cyan-400" />对话主面板</CardTitle>
            <CardDescription>同一个入口统一承接问答，再自动路由到底层最优 Agent。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="max-h-[560px] space-y-4 overflow-auto rounded-3xl border border-border/60 bg-secondary/15 p-4">
              {messages.map((message, index) => (
                <div key={`${message.role}-${index}`} className={`flex ${message.role === "user" ? "justify-end" : "justify-start"}`}>
                  <div className={`max-w-[86%] rounded-3xl px-5 py-4 text-sm leading-7 ${message.role === "user" ? "bg-cyan-500 text-black" : "border border-border/60 bg-background/70"}`}>
                    <div className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] opacity-80">
                      {message.role === "user" ? <Send className="h-3.5 w-3.5" /> : <Bot className="h-3.5 w-3.5" />}
                      {message.role === "user" ? "User" : "Assistant"}
                    </div>
                    <div>{message.content}</div>
                    {message.meta && (
                      <div className="mt-4 flex flex-wrap gap-2">
                        <Badge variant="outline">{message.meta.selected_agent.id}</Badge>
                        <Badge variant="info">{message.meta.selected_candidate?.model ?? message.meta.selected_agent.model}</Badge>
                        <Badge variant="warning">{message.meta.route_source}</Badge>
                        <Badge variant="outline">memory: {message.meta.memory_influence ?? "none"}</Badge>
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
            <div className="rounded-3xl border border-border/60 bg-secondary/10 p-4">
              <textarea
                value={input}
                onChange={(event) => setInput(event.target.value)}
                placeholder="输入你的问题，系统会自动路由到最优 Agent..."
                className="min-h-28 w-full resize-none rounded-2xl border border-border/60 bg-background/70 px-4 py-3 text-sm outline-none"
              />
              <div className="mt-3 flex flex-wrap gap-2">
                {suggestedPrompts.map((prompt) => (
                  <button
                    key={prompt}
                    type="button"
                    onClick={() => setInput(prompt)}
                    className="rounded-full border border-border/60 bg-background/70 px-3 py-1.5 text-xs text-muted-foreground transition hover:border-cyan-400/60 hover:text-foreground"
                  >
                    {prompt}
                  </button>
                ))}
              </div>
              <div className="mt-3 flex justify-end">
                <Button onClick={sendMessage} disabled={sending || !input.trim()} className="bg-emerald-400 text-black hover:bg-emerald-300">
                  <Send className="mr-2 h-4 w-4" />
                  {sending ? "正在路由并执行..." : "发送并自动路由"}
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        <div className="space-y-6">
          <Card className="glass-card border-emerald-500/20">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-xl"><BrainCircuit className="h-5 w-5 text-emerald-400" />Agent Registry</CardTitle>
              <CardDescription>系统当前已注册的可路由 Agent。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {agents.map((agent) => (
                <div key={agent.id} className="rounded-3xl border border-border/60 bg-secondary/20 p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="text-lg font-black">{agent.name}</div>
                      <div className="mt-1 text-sm text-muted-foreground">{agent.description}</div>
                    </div>
                    <Badge variant="info">{agent.category}</Badge>
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    <Badge variant="outline">{agent.provider}</Badge>
                    <Badge variant="warning">{agent.model}</Badge>
                    <Badge variant="success">{agent.strategy}</Badge>
                  </div>
                </div>
              ))}
            </CardContent>
          </Card>

          <Card className="glass-card border-fuchsia-500/20">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-xl"><Sparkles className="h-5 w-5 text-fuchsia-400" />Route Memory</CardTitle>
              <CardDescription>最近沉淀的 query → agent → outcome 路径。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {memory.length > 0 ? memory.slice(0, 8).map((item) => (
                <div key={item.id} className="rounded-3xl border border-border/60 bg-secondary/20 p-4">
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge variant={item.succeeded ? "success" : "destructive"}>{item.succeeded ? "有效路径" : "失败路径"}</Badge>
                    <Badge variant="outline">{item.intent}</Badge>
                    <Badge variant="warning">{item.selected_agent_id}</Badge>
                  </div>
                  <div className="mt-3 text-sm font-semibold leading-6">{item.query}</div>
                  <div className="mt-2 text-sm leading-6 text-muted-foreground">{item.answer_preview}</div>
                  <div className="mt-3 text-xs uppercase tracking-[0.18em] text-cyan-400">score {item.outcome_score.toFixed(2)} · {item.route_source}</div>
                </div>
              )) : (
                <div className="rounded-3xl border border-dashed p-4 text-sm text-muted-foreground">还没有 memory。发送几轮不同类型的问题后，这里会逐渐沉淀最优 agent 路径。</div>
              )}
            </CardContent>
          </Card>
        </div>
      </section>

      {latestMeta && (
        <section className="grid gap-6 xl:grid-cols-2">
          <Card className="glass-card border-amber-500/20">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-xl"><Cpu className="h-5 w-5 text-amber-400" />本轮执行请求</CardTitle>
              <CardDescription>展示路由后发给底层 Agent / 模型的请求元信息。</CardDescription>
            </CardHeader>
            <CardContent>
              <pre className="overflow-x-auto whitespace-pre-wrap break-words rounded-3xl border border-border/60 bg-black/80 p-5 text-xs leading-6 text-emerald-200">{JSON.stringify(latestMeta.gateway_request, null, 2)}</pre>
            </CardContent>
          </Card>
          <Card className="glass-card border-sky-500/20">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-xl"><Bot className="h-5 w-5 text-sky-400" />本轮返回结果</CardTitle>
              <CardDescription>展示最近一轮 Agent 执行后的网关响应。</CardDescription>
            </CardHeader>
            <CardContent>
              <pre className="overflow-x-auto whitespace-pre-wrap break-words rounded-3xl border border-border/60 bg-black/80 p-5 text-xs leading-6 text-sky-200">{JSON.stringify(latestMeta.gateway_response ?? { error: latestMeta.error_message }, null, 2)}</pre>
            </CardContent>
          </Card>
        </section>
      )}
    </div>
  );
}
