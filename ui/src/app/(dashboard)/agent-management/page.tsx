"use client";

import { useEffect, useState } from "react";
import { Bot, Plus, Save, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiClient } from "@/lib/api/client";
import type { CandidateModel, CandidateModelState, ConversationAgent, ToolMarketplaceItem } from "@/types/api";

type Draft = { id: string; name: string; description: string; category: string; provider: string; model: string; candidate_models: string; strategy: string; capabilities: string; system_prompt: string; accent: string; tools: string[]; enabled: boolean; };
const emptyDraft = (): Draft => ({ id: "", name: "", description: "", category: "general", provider: "deepseek-primary", model: "deepseek-chat", candidate_models: "deepseek-primary/deepseek-chat | 1 | 24 | 24000\ndeepseek-primary/deepseek-reasoner | 0.7 | 12 | 12000", strategy: "lowest-latency", capabilities: "", system_prompt: "", accent: "cyan", tools: [], enabled: true });

function parseCandidateModels(value: string): CandidateModel[] {
  return value.split(/\r?\n/).map((line, index) => {
    const [pair, weightText, rpmText, tpmText] = line.split("|").map((item) => item.trim());
    const [provider, model] = (pair ?? "").split("/").map((item) => item.trim());
    const weight = Number(weightText || (index === 0 ? 1 : 0.8));
    const rpm = Number(rpmText || 0);
    const tpm = Number(tpmText || 0);
    return provider && model ? {
      provider,
      model,
      weight: Number.isFinite(weight) ? weight : undefined,
      rpm_limit: Number.isFinite(rpm) && rpm > 0 ? rpm : undefined,
      tpm_limit: Number.isFinite(tpm) && tpm > 0 ? tpm : undefined,
    } : null;
  }).filter(Boolean) as CandidateModel[];
}

function formatCandidateModels(items?: CandidateModel[]): string {
  return (items ?? []).map((item) => `${item.provider}/${item.model}${item.weight ? ` | ${item.weight}` : ""}${item.rpm_limit ? ` | ${item.rpm_limit}` : ""}${item.tpm_limit ? ` | ${item.tpm_limit}` : ""}`).join("\n");
}

function renderCandidateBadge(item: CandidateModel | CandidateModelState) {
  const state = "status" in item ? item.status : undefined;
  const rpm = item.rpm_limit ? ` rpm ${"current_rpm" in item ? item.current_rpm : 0}/${item.rpm_limit}` : "";
  const tpm = item.tpm_limit ? ` tpm ${"current_tpm" in item ? item.current_tpm : 0}/${item.tpm_limit}` : "";
  return `${item.provider}/${item.model}${state ? ` · ${state}` : ""}${rpm}${tpm}`;
}

export default function AgentManagementPage() {
  const [agents, setAgents] = useState<ConversationAgent[]>([]);
  const [marketplace, setMarketplace] = useState<ToolMarketplaceItem[]>([]);
  const [open, setOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft>(emptyDraft());
  const load = async () => { const [a, m] = await Promise.all([apiClient.listConversationAgents(), apiClient.listToolMarketplace()]); setAgents(a.data ?? []); setMarketplace(m.data ?? []); };
  useEffect(() => { void load(); }, []);
  const startCreate = () => { setEditingId(null); setDraft(emptyDraft()); setOpen(true); };
  const startEdit = (a: ConversationAgent) => { setEditingId(a.id); setDraft({ id: a.id, name: a.name, description: a.description, category: a.category, provider: a.provider, model: a.model, candidate_models: formatCandidateModels(a.candidate_models), strategy: a.strategy, capabilities: (a.capabilities ?? []).join(", "), system_prompt: a.system_prompt, accent: a.accent, tools: a.tools ?? [], enabled: a.enabled }); setOpen(true); };
  const save = async () => { const payload = { ...draft, candidate_models: parseCandidateModels(draft.candidate_models), capabilities: draft.capabilities.split(",").map(v => v.trim()).filter(Boolean) }; editingId ? await apiClient.updateConversationAgent(payload) : await apiClient.createConversationAgent(payload); setOpen(false); await load(); };
  const remove = async (id: string) => { await apiClient.deleteConversationAgent(id); await load(); };
  const toggleTool = (id: string) => setDraft((p) => ({ ...p, tools: p.tools.includes(id) ? p.tools.filter((v) => v !== id) : [...p.tools, id] }));
  return <div className="space-y-8">
    <section className="rounded-[32px] border border-emerald-500/20 bg-[radial-gradient(circle_at_top_left,rgba(16,185,129,0.18),transparent_28%),linear-gradient(135deg,rgba(7,14,20,0.98),rgba(16,24,37,0.94))] p-8 text-white shadow-2xl"><div className="flex items-start justify-between gap-6"><div><Badge variant="success" className="w-fit">Managed Agent Registry</Badge><h1 className="mt-4 text-4xl font-black tracking-tight">Agent 管理页</h1><p className="mt-3 max-w-4xl text-sm leading-7 text-slate-200">每个 expert agent 现在可以挂多个候选模型；网关会结合 weight、近一分钟 RPM/TPM 估算和 cooldown，优先挑最不容易撞限流的模型。</p></div><Button onClick={startCreate} className="bg-emerald-400 text-black hover:bg-emerald-300"><Plus className="mr-2 h-4 w-4" />新增 Agent</Button></div></section>
    <Card className="glass-card border-emerald-500/20"><CardHeader><CardTitle className="flex items-center gap-2 text-xl"><Bot className="h-5 w-5 text-emerald-400" />Agent Registry</CardTitle><CardDescription>候选模型支持 weight、RPM、TPM 配置，并在运行时显示调度状态。</CardDescription></CardHeader><CardContent><Table><TableHeader><TableRow><TableHead>Agent</TableHead><TableHead>Category</TableHead><TableHead>Primary Model</TableHead><TableHead>Candidate Models</TableHead><TableHead>Marketplace Tools</TableHead><TableHead>Status</TableHead><TableHead>Actions</TableHead></TableRow></TableHeader><TableBody>{agents.map((a) => <TableRow key={a.id}><TableCell><div className="font-semibold">{a.name}</div><div className="text-xs text-muted-foreground">{a.id}</div></TableCell><TableCell><Badge variant="info">{a.category}</Badge></TableCell><TableCell><div className="text-sm">{a.provider}</div><div className="text-xs text-muted-foreground">{a.model}</div></TableCell><TableCell><div className="flex max-w-[360px] flex-wrap gap-2">{(a.candidate_states?.length ? a.candidate_states : a.candidate_models ?? []).length ? (a.candidate_states?.length ? a.candidate_states : a.candidate_models ?? []).map((item) => <Badge key={`${item.provider}/${item.model}`} variant="outline">{renderCandidateBadge(item)}</Badge>) : <span className="text-xs text-muted-foreground">No candidates</span>}</div></TableCell><TableCell><div className="flex flex-wrap gap-2">{(a.tools ?? []).length ? (a.tools ?? []).map((t) => <Badge key={t} variant="outline">{t}</Badge>) : <span className="text-xs text-muted-foreground">No marketplace tools</span>}</div></TableCell><TableCell><Badge variant={a.enabled ? "success" : "destructive"}>{a.enabled ? "enabled" : "disabled"}</Badge></TableCell><TableCell><div className="flex gap-2"><Button variant="outline" onClick={() => startEdit(a)}>编辑</Button><Button variant="destructive" onClick={() => remove(a.id)}><Trash2 className="h-4 w-4" /></Button></div></TableCell></TableRow>)}</TableBody></Table></CardContent></Card>
    <Dialog open={open} onOpenChange={setOpen}><DialogContent className="max-w-3xl"><DialogHeader><DialogTitle>{editingId ? "编辑 Agent" : "新增 Agent"}</DialogTitle><DialogDescription>候选模型格式：provider/model | weight | rpm_limit | tpm_limit</DialogDescription></DialogHeader><div className="grid gap-4 md:grid-cols-2"><div className="space-y-2"><Label>ID</Label><Input value={draft.id} disabled={!!editingId} onChange={(e) => setDraft({ ...draft, id: e.target.value })} /></div><div className="space-y-2"><Label>Name</Label><Input value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} /></div><div className="space-y-2"><Label>Category</Label><Select value={draft.category} onValueChange={(v) => setDraft({ ...draft, category: v })}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="general">general</SelectItem><SelectItem value="coding">coding</SelectItem><SelectItem value="research">research</SelectItem><SelectItem value="writing">writing</SelectItem></SelectContent></Select></div><div className="space-y-2"><Label>Strategy</Label><Input value={draft.strategy} onChange={(e) => setDraft({ ...draft, strategy: e.target.value })} /></div><div className="space-y-2"><Label>Primary Provider</Label><Input value={draft.provider} onChange={(e) => setDraft({ ...draft, provider: e.target.value })} /></div><div className="space-y-2"><Label>Primary Model</Label><Input value={draft.model} onChange={(e) => setDraft({ ...draft, model: e.target.value })} /></div><div className="space-y-2 md:col-span-2"><Label>Candidate Models</Label><textarea value={draft.candidate_models} onChange={(e) => setDraft({ ...draft, candidate_models: e.target.value })} className="min-h-32 w-full rounded-2xl border border-input bg-background px-3 py-3 text-sm outline-none" /></div><div className="space-y-2 md:col-span-2"><Label>Description</Label><Input value={draft.description} onChange={(e) => setDraft({ ...draft, description: e.target.value })} /></div><div className="space-y-2 md:col-span-2"><Label>Capabilities</Label><Input value={draft.capabilities} onChange={(e) => setDraft({ ...draft, capabilities: e.target.value })} /></div><div className="space-y-2 md:col-span-2"><Label>System Prompt</Label><textarea value={draft.system_prompt} onChange={(e) => setDraft({ ...draft, system_prompt: e.target.value })} className="min-h-28 w-full rounded-2xl border border-input bg-background px-3 py-3 text-sm outline-none" /></div><div className="space-y-2"><Label>Accent</Label><Input value={draft.accent} onChange={(e) => setDraft({ ...draft, accent: e.target.value })} /></div><div className="space-y-2"><Label>Status</Label><Select value={draft.enabled ? "enabled" : "disabled"} onValueChange={(v) => setDraft({ ...draft, enabled: v === "enabled" })}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="enabled">enabled</SelectItem><SelectItem value="disabled">disabled</SelectItem></SelectContent></Select></div><div className="space-y-2 md:col-span-2"><Label>Tool Marketplace Bindings</Label><div className="grid gap-2 rounded-2xl border border-border/60 bg-secondary/10 p-4 md:grid-cols-2">{marketplace.length ? marketplace.map((t) => <button key={t.id} type="button" onClick={() => toggleTool(t.id)} className={`rounded-2xl border px-3 py-3 text-left ${draft.tools.includes(t.id) ? "border-emerald-400 bg-emerald-400/10" : "border-border/60 bg-background/60"}`}><div className="font-medium">{t.name}</div><div className="mt-1 text-xs text-muted-foreground">{t.id} → {t.source_client_id}/{t.source_tool_name}</div></button>) : <div className="text-sm text-muted-foreground">请先去 Tool Marketplace 上架工具。</div>}</div></div></div><DialogFooter><Button variant="outline" onClick={() => setOpen(false)}>取消</Button><Button onClick={save}><Save className="mr-2 h-4 w-4" />保存 Agent</Button></DialogFooter></DialogContent></Dialog>
  </div>;
}
