"use client";

import { useEffect, useMemo, useState } from "react";
import { BrainCircuit, DatabaseZap, Search, Sparkles, Target } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiClient } from "@/lib/api/client";
import type { RouteMemoryRecord } from "@/types/api";

export default function RouteMemoryPage() {
  const [items, setItems] = useState<RouteMemoryRecord[]>([]);
  const [keyword, setKeyword] = useState("");
  const [source, setSource] = useState("all");

  useEffect(() => { void apiClient.getConversationMemory().then((res) => setItems(res.data ?? [])); }, []);

  const filtered = useMemo(() => items.filter((item) => {
    const matchesKeyword = !keyword || [item.query, item.selected_agent_id, item.selected_model, item.route_source, item.answer_preview].join(" ").toLowerCase().includes(keyword.toLowerCase());
    const matchesSource = source === "all" || item.route_source === source;
    return matchesKeyword && matchesSource;
  }), [items, keyword, source]);

  const stats = useMemo(() => ({
    total: items.length,
    success: items.filter((item) => item.succeeded).length,
    avgScore: items.length ? (items.reduce((sum, item) => sum + item.outcome_score, 0) / items.length).toFixed(2) : "0.00",
    sources: Array.from(new Set(items.map((item) => item.route_source))).filter(Boolean),
  }), [items]);

  return <div className="space-y-8">
    <section className="rounded-[32px] border border-violet-500/20 bg-[radial-gradient(circle_at_top_left,rgba(168,85,247,0.2),transparent_30%),linear-gradient(135deg,rgba(12,10,24,0.98),rgba(25,18,48,0.94))] p-8 text-white shadow-2xl">
      <div className="flex items-start justify-between gap-6">
        <div>
          <Badge variant="info" className="w-fit">Conversation Route Memory</Badge>
          <h1 className="mt-4 text-4xl font-black tracking-tight">Route Memory 管理页</h1>
          <p className="mt-3 max-w-4xl text-sm leading-7 text-slate-200">查看最近对话的选路结果、命中的 agent、路径来源与效果分数，方便你观察自主路由、显式指定、记忆复用是否符合预期。</p>
        </div>
      </div>
    </section>

    <section className="grid gap-4 md:grid-cols-4">
      <Card className="glass-card border-white/10 md:col-span-4">
        <CardContent className="grid gap-3 p-5 md:grid-cols-3">
          <div className="rounded-2xl border border-border/60 bg-secondary/20 p-4"><div className="text-sm font-semibold">autonomous-router</div><div className="mt-2 text-sm text-muted-foreground">Gateway 结合 agent registry 与相似历史 memory 自主选路。</div></div>
          <div className="rounded-2xl border border-border/60 bg-secondary/20 p-4"><div className="text-sm font-semibold">memory-hit</div><div className="mt-2 text-sm text-muted-foreground">自主路由失败后，直接复用高相似、高分历史路径。</div></div>
          <div className="rounded-2xl border border-border/60 bg-secondary/20 p-4"><div className="text-sm font-semibold">explicit-agent / intent-router</div><div className="mt-2 text-sm text-muted-foreground">用户显式指定优先；否则再退回到轻量意图规则路由。</div></div>
        </CardContent>
      </Card>
      <Card className="glass-card border-violet-500/20"><CardHeader className="pb-3"><CardTitle className="flex items-center gap-2 text-sm"><DatabaseZap className="h-4 w-4 text-violet-400" />Total Memories</CardTitle></CardHeader><CardContent><div className="text-3xl font-black">{stats.total}</div></CardContent></Card>
      <Card className="glass-card border-emerald-500/20"><CardHeader className="pb-3"><CardTitle className="flex items-center gap-2 text-sm"><Sparkles className="h-4 w-4 text-emerald-400" />Succeeded</CardTitle></CardHeader><CardContent><div className="text-3xl font-black">{stats.success}</div></CardContent></Card>
      <Card className="glass-card border-amber-500/20"><CardHeader className="pb-3"><CardTitle className="flex items-center gap-2 text-sm"><Target className="h-4 w-4 text-amber-400" />Avg Score</CardTitle></CardHeader><CardContent><div className="text-3xl font-black">{stats.avgScore}</div></CardContent></Card>
      <Card className="glass-card border-cyan-500/20"><CardHeader className="pb-3"><CardTitle className="flex items-center gap-2 text-sm"><BrainCircuit className="h-4 w-4 text-cyan-400" />Sources</CardTitle></CardHeader><CardContent><div className="flex flex-wrap gap-2">{stats.sources.length ? stats.sources.map((s) => <Badge key={s} variant="outline">{s}</Badge>) : <span className="text-sm text-muted-foreground">No route sources</span>}</div></CardContent></Card>
    </section>

    <Card className="glass-card border-violet-500/20">
      <CardHeader>
        <CardTitle>Memory Records</CardTitle>
        <CardDescription>支持按 query、agent、model、route source 搜索，也可以按来源筛选。</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-4 md:grid-cols-[1fr_220px]">
          <div className="relative"><Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" /><Input value={keyword} onChange={(e) => setKeyword(e.target.value)} placeholder="搜索 query / agent / model / route source" className="pl-9" /></div>
          <select value={source} onChange={(e) => setSource(e.target.value)} className="h-10 rounded-xl border border-input bg-background px-3 text-sm outline-none">
            <option value="all">All Sources</option>
            {stats.sources.map((s) => <option key={s} value={s}>{s}</option>)}
          </select>
        </div>

        <div className="rounded-2xl border border-border/60 overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Query</TableHead>
                <TableHead>Selected Agent</TableHead>
                <TableHead>Path</TableHead>
                <TableHead>Route Source</TableHead>
                <TableHead>Score</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((item) => <TableRow key={item.id}>
                <TableCell className="max-w-[360px] align-top"><div className="font-medium leading-6">{item.query}</div><div className="mt-2 text-xs text-muted-foreground line-clamp-2">{item.answer_preview || "No preview"}</div></TableCell>
                <TableCell className="align-top"><div className="font-semibold">{item.selected_agent_id}</div><div className="text-xs text-muted-foreground">{item.selected_model}</div></TableCell>
                <TableCell className="align-top text-xs text-muted-foreground">{item.selected_path}</TableCell>
                <TableCell className="align-top"><Badge variant="outline">{item.route_source}</Badge></TableCell>
                <TableCell className="align-top"><span className="font-semibold">{item.outcome_score.toFixed(2)}</span></TableCell>
                <TableCell className="align-top"><Badge variant={item.succeeded ? "success" : "destructive"}>{item.succeeded ? "success" : "failed"}</Badge></TableCell>
                <TableCell className="align-top text-xs text-muted-foreground">{new Date(item.created_at).toLocaleString()}</TableCell>
              </TableRow>)}
              {!filtered.length && <TableRow><TableCell colSpan={7} className="py-10 text-center text-sm text-muted-foreground">当前没有匹配的 Route Memory 记录。</TableCell></TableRow>}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  </div>;
}
