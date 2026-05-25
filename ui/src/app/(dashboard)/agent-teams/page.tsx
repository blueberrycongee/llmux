"use client";

import { useEffect, useMemo, useState } from "react";
import { Bot, GitBranch, Plus, Save, Sparkles, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { apiClient } from "@/lib/api/client";
import type { AgentTeam, AgentTeamRehearsalResponse, ConversationAgent, TeamMemoryRecord } from "@/types/api";

type Draft = { id: string; name: string; description: string; mode: string; agent_ids: string; enabled: boolean };
const emptyDraft = (): Draft => ({ id: "", name: "", description: "", mode: "collaborative", agent_ids: "research-strategist, writer-translator", enabled: true });

export default function AgentTeamsPage() {
  const [teams, setTeams] = useState<AgentTeam[]>([]);
  const [agents, setAgents] = useState<ConversationAgent[]>([]);
  const [memory, setMemory] = useState<TeamMemoryRecord[]>([]);
  const [rehearsal, setRehearsal] = useState<AgentTeamRehearsalResponse | null>(null);
  const [rehearsalTeamId, setRehearsalTeamId] = useState("");
  const [prompt, setPrompt] = useState("帮我围绕异构大模型网关论文做一次团队演练：先提炼研究问题，再给出实现路线和写作建议。");
  const [open, setOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft>(emptyDraft());

  const load = async () => {
    const [teamsRes, agentsRes, memoryRes] = await Promise.all([
      apiClient.listAgentTeams(),
      apiClient.listConversationAgents(),
      apiClient.getTeamMemory(),
    ]);
    setTeams(teamsRes.data ?? []);
    setAgents(agentsRes.data ?? []);
    setMemory(memoryRes.data ?? []);
    if (!rehearsalTeamId && (teamsRes.data ?? []).length) setRehearsalTeamId((teamsRes.data ?? [])[0].id);
  };

  useEffect(() => { void load(); }, []);

  const agentMap = useMemo(() => Object.fromEntries(agents.map((agent) => [agent.id, agent])), [agents]);

  const startCreate = () => { setEditingId(null); setDraft(emptyDraft()); setOpen(true); };
  const startEdit = (team: AgentTeam) => {
    setEditingId(team.id);
    setDraft({ id: team.id, name: team.name, description: team.description, mode: team.mode, agent_ids: team.agent_ids.join(", "), enabled: team.enabled });
    setOpen(true);
  };

  const save = async () => {
    const payload = { ...draft, agent_ids: draft.agent_ids.split(",").map((item) => item.trim()).filter(Boolean) };
    editingId ? await apiClient.updateAgentTeam(payload) : await apiClient.createAgentTeam(payload);
    setOpen(false);
    await load();
  };

  const remove = async (id: string) => { await apiClient.deleteAgentTeam(id); await load(); };

  const rehearse = async () => {
    if (!rehearsalTeamId || !prompt.trim()) return;
    const result = await apiClient.rehearseAgentTeam({ team_id: rehearsalTeamId, prompt });
    setRehearsal(result);
    const memoryRes = await apiClient.getTeamMemory();
    setMemory(memoryRes.data ?? []);
  };

  return <div className="space-y-8">
    <section className="rounded-[32px] border border-fuchsia-500/20 bg-[radial-gradient(circle_at_top_left,rgba(217,70,239,0.18),transparent_28%),linear-gradient(135deg,rgba(9,10,22,0.98),rgba(22,18,38,0.94))] p-8 text-white shadow-2xl">
      <div className="flex items-start justify-between gap-6">
        <div>
          <Badge variant="info" className="w-fit">Expert Agent Team</Badge>
          <h1 className="mt-4 text-4xl font-black tracking-tight">团队级专家路由与演练台</h1>
          <p className="mt-3 max-w-4xl text-sm leading-7 text-slate-200">在单 expert agent 之上，引入可选的专家团队层。复杂任务可先路由到 team，再由 team 内部选择 lead agent；每次演练会沉淀 team memory，用于后续复用。</p>
        </div>
        <Button onClick={startCreate} className="bg-fuchsia-300 text-black hover:bg-fuchsia-200"><Plus className="mr-2 h-4 w-4" />新增 Team</Button>
      </div>
    </section>

    <section className="grid gap-6 xl:grid-cols-[1fr_0.95fr]">
      <Card className="glass-card border-fuchsia-500/20">
        <CardHeader><CardTitle className="flex items-center gap-2 text-xl"><GitBranch className="h-5 w-5 text-fuchsia-400" />Agent Teams</CardTitle><CardDescription>复杂任务可先路由到 Team，再由 Team 挑选 lead expert agent。</CardDescription></CardHeader>
        <CardContent className="space-y-4">
          {teams.map((team) => <div key={team.id} className="rounded-3xl border border-border/60 bg-secondary/15 p-4">
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="text-lg font-black">{team.name}</div>
                <div className="mt-1 text-sm text-muted-foreground">{team.description}</div>
              </div>
              <div className="flex gap-2">
                <Badge variant={team.enabled ? "success" : "destructive"}>{team.enabled ? "enabled" : "disabled"}</Badge>
                <Badge variant="outline">{team.mode}</Badge>
              </div>
            </div>
            <div className="mt-3 flex flex-wrap gap-2">{team.agent_ids.map((id) => <Badge key={id} variant="outline">{agentMap[id]?.name ?? id}</Badge>)}</div>
            <div className="mt-4 flex gap-2"><Button variant="outline" onClick={() => startEdit(team)}>编辑</Button><Button variant="destructive" onClick={() => remove(team.id)}><Trash2 className="h-4 w-4" /></Button></div>
          </div>)}
        </CardContent>
      </Card>

      <Card className="glass-card border-emerald-500/20">
        <CardHeader><CardTitle className="flex items-center gap-2 text-xl"><Sparkles className="h-5 w-5 text-emerald-400" />Team Rehearsal</CardTitle><CardDescription>通过演练去打磨团队组合，并沉淀可复用的 team memory。</CardDescription></CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2"><Label>选择 Team</Label><Select value={rehearsalTeamId} onValueChange={setRehearsalTeamId}><SelectTrigger><SelectValue placeholder="选择一个 team" /></SelectTrigger><SelectContent>{teams.map((team) => <SelectItem key={team.id} value={team.id}>{team.name}</SelectItem>)}</SelectContent></Select></div>
          <div className="space-y-2"><Label>演练任务</Label><textarea value={prompt} onChange={(e) => setPrompt(e.target.value)} className="min-h-32 w-full rounded-2xl border border-input bg-background px-3 py-3 text-sm outline-none" /></div>
          <Button onClick={rehearse} className="bg-emerald-400 text-black hover:bg-emerald-300">开启团队演练</Button>
          {rehearsal && <div className="rounded-3xl border border-border/60 bg-secondary/15 p-4">
            <div className="text-sm font-semibold">最近演练：{rehearsal.selected_team.name}</div>
            <div className="mt-3 space-y-3">{rehearsal.steps.map((step) => <div key={step.agent_id} className="rounded-2xl border border-border/60 bg-background/60 p-3">
              <div className="flex flex-wrap items-center gap-2"><Badge variant={step.succeeded ? "success" : "destructive"}>{step.succeeded ? "success" : "failed"}</Badge><Badge variant="outline">{step.agent_name}</Badge>{step.selected_candidate && <Badge variant="warning">{step.selected_candidate.provider}/{step.selected_candidate.model}</Badge>}</div>
              <div className="mt-2 text-sm text-muted-foreground">{step.output_preview || step.error_message || "暂无输出"}</div>
            </div>)}</div>
          </div>}
        </CardContent>
      </Card>
    </section>

    <Card className="glass-card border-cyan-500/20">
      <CardHeader><CardTitle className="flex items-center gap-2 text-xl"><Bot className="h-5 w-5 text-cyan-400" />Team Memory</CardTitle><CardDescription>沉淀团队级协作路径，供后续复杂任务直接 consulted / reused。</CardDescription></CardHeader>
      <CardContent className="space-y-3">
        {memory.length ? memory.slice(0, 10).map((item) => <div key={item.id} className="rounded-3xl border border-border/60 bg-secondary/15 p-4">
          <div className="flex flex-wrap items-center gap-2"><Badge variant={item.succeeded ? "success" : "destructive"}>{item.succeeded ? "有效路径" : "失败路径"}</Badge><Badge variant="outline">{item.intent}</Badge><Badge variant="warning">{item.selected_team_id}</Badge></div>
          <div className="mt-3 text-sm font-semibold leading-6">{item.query}</div>
          <div className="mt-2 text-sm text-muted-foreground">{item.summary}</div>
          {item.compressed_summary && item.compressed_summary !== item.summary && <div className="mt-2 rounded-2xl border border-border/50 bg-background/40 px-3 py-2 text-sm text-foreground">{item.compressed_summary}</div>}
          {item.distilled_learnings?.length ? <div className="mt-3 space-y-2">
            {item.distilled_learnings.slice(0, 3).map((learning) => <div key={learning} className="rounded-2xl border border-border/50 bg-background/60 px-3 py-2 text-sm text-muted-foreground">{learning}</div>)}
          </div> : null}
          {item.representative_queries?.length ? <div className="mt-3 rounded-2xl border border-border/50 bg-background/40 p-3">
            <div className="text-xs font-semibold uppercase tracking-[0.16em] text-cyan-400">Representative Queries</div>
            <div className="mt-2 space-y-2 text-sm text-muted-foreground">
              {item.representative_queries.slice(0, 2).map((query) => <div key={query}>{query}</div>)}
            </div>
          </div> : null}
          <div className="mt-3 flex flex-wrap gap-3 text-xs uppercase tracking-[0.18em] text-cyan-400">
            <span>score {item.outcome_score.toFixed(2)}</span>
            {item.memory_stage && <span>{item.memory_stage}</span>}
            {typeof item.source_count === "number" && item.source_count > 0 && <span>{item.source_count} episodes</span>}
            {typeof item.compression_ratio === "number" && <span>compression {(item.compression_ratio * 100).toFixed(0)}%</span>}
            {typeof item.consult_count === "number" && <span>consult {item.consult_count}</span>}
            {typeof item.reuse_count === "number" && <span>reuse {item.reuse_count}</span>}
          </div>
        </div>) : <div className="rounded-3xl border border-dashed p-4 text-sm text-muted-foreground">还没有 team memory。先跑一次团队演练。</div>}
      </CardContent>
    </Card>

    <Dialog open={open} onOpenChange={setOpen}><DialogContent className="max-w-2xl"><DialogHeader><DialogTitle>{editingId ? "编辑 Team" : "新增 Team"}</DialogTitle><DialogDescription>输入 agent id 列表，逗号分隔。</DialogDescription></DialogHeader><div className="grid gap-4 md:grid-cols-2"><div className="space-y-2"><Label>ID</Label><Input value={draft.id} disabled={!!editingId} onChange={(e) => setDraft({ ...draft, id: e.target.value })} /></div><div className="space-y-2"><Label>Name</Label><Input value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} /></div><div className="space-y-2 md:col-span-2"><Label>Description</Label><Input value={draft.description} onChange={(e) => setDraft({ ...draft, description: e.target.value })} /></div><div className="space-y-2"><Label>Mode</Label><Input value={draft.mode} onChange={(e) => setDraft({ ...draft, mode: e.target.value })} /></div><div className="space-y-2"><Label>Status</Label><Select value={draft.enabled ? "enabled" : "disabled"} onValueChange={(v) => setDraft({ ...draft, enabled: v === "enabled" })}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="enabled">enabled</SelectItem><SelectItem value="disabled">disabled</SelectItem></SelectContent></Select></div><div className="space-y-2 md:col-span-2"><Label>Agent IDs</Label><Input value={draft.agent_ids} onChange={(e) => setDraft({ ...draft, agent_ids: e.target.value })} /></div></div><DialogFooter><Button variant="outline" onClick={() => setOpen(false)}>取消</Button><Button onClick={save}><Save className="mr-2 h-4 w-4" />保存 Team</Button></DialogFooter></DialogContent></Dialog>
  </div>;
}
