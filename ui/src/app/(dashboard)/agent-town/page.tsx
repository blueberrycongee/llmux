"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { Clock3, MessageSquare, Network, Save, ScrollText, Sparkles, Users } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { apiClient } from "@/lib/api/client";
import {
  buildRoutingMemoryBundleFromSandbox,
  clearActiveRoutingMemory,
  getActiveRoutingMemory,
  saveRoutingMemoryBundle,
} from "@/lib/routing-memory-store";
import type { SandboxAgent, SandboxState } from "@/types/api";

export default function Page() {
  const [prompt, setPrompt] = useState("生成一个资源紧张但信息流通很快的港口社会沙盘");
  const [sandbox, setSandbox] = useState<SandboxState | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [saveMessage, setSaveMessage] = useState("");
  const [activeMemoryTitle, setActiveMemoryTitle] = useState<string | null>(() => getActiveRoutingMemory()?.title ?? null);

  const selected = useMemo(
    () => sandbox?.agents.find((a) => a.id === selectedId) ?? sandbox?.agents[0] ?? null,
    [sandbox, selectedId]
  );

  const generate = async () => {
    setLoading(true);
    setSaveMessage("");
    try {
      const activeMemory = getActiveRoutingMemory();
      setActiveMemoryTitle(activeMemory?.title ?? null);
      const nextPrompt = activeMemory
        ? `${prompt}\n\n请复用以下历史 routing memory 作为本次推演的先验经验：\n标题：${activeMemory.title}\n场景：${activeMemory.prompt_summary}\n记忆摘要：${activeMemory.items.slice(0, 3).map((item) => `${item.signal} / ${item.decision} / ${item.outcome}`).join("；")}`
        : prompt;
      const next = await apiClient.generateSandbox({ prompt: nextPrompt });
      setSandbox(next);
      setSelectedId(next.agents[0]?.id ?? null);
    } finally {
      setLoading(false);
    }
  };

  const step = async () => {
    if (!sandbox) return;
    setLoading(true);
    setSaveMessage("");
    try {
      const next = await apiClient.stepSandbox(sandbox.id);
      setSandbox(next);
      if (!selectedId && next.agents[0]) setSelectedId(next.agents[0].id);
    } finally {
      setLoading(false);
    }
  };

  const saveMemory = () => {
    if (!sandbox) return;
    const bundle = buildRoutingMemoryBundleFromSandbox(sandbox);
    saveRoutingMemoryBundle(bundle);
    setSaveMessage(`已保存演练 memory：${bundle.title}`);
  };

  const pos = (name: string) => {
    const d = sandbox!.districts.find((x) => x.name === name)!;
    return { x: d.x + d.w / 2, y: d.y + d.h / 2 };
  };

  const movePos = (a: SandboxAgent) => {
    const s = pos(a.from);
    const e = pos(a.to);
    return { left: s.x + (e.x - s.x) * a.progress, top: s.y + (e.y - s.y) * a.progress };
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <Badge variant="info" className="mb-3">Social Sandbox Studio</Badge>
          <h1 className="text-4xl font-black tracking-tight">Agent 社会沙盘生成与演练平台</h1>
          <p className="mt-2 max-w-4xl text-muted-foreground">
            先生成沙盘，再推进时间。完成演练后，可将本次 logs 与 archive 抽取为 routing memory 并保存到浏览器本地。
          </p>
          {activeMemoryTitle ? (
            <div className="mt-3 flex flex-wrap items-center gap-3 text-sm">
              <span className="text-cyan-400">当前复用 Memory：{activeMemoryTitle}</span>
              <Button size="sm" variant="outline" onClick={() => { clearActiveRoutingMemory(); setActiveMemoryTitle(null); }}>清除当前复用</Button>
            </div>
          ) : null}
          {saveMessage ? (
            <div className="mt-3 flex flex-wrap items-center gap-3 text-sm">
              <span className="text-emerald-400">{saveMessage}</span>
              <Button asChild size="sm" variant="outline">
                <Link href="/demo-center">跳转查看 Memory</Link>
              </Button>
            </div>
          ) : null}
        </div>
        <div className="flex flex-wrap gap-2">
          <Button onClick={generate} disabled={loading}><Sparkles className="mr-2 h-4 w-4" />{loading ? "处理中..." : "生成沙盘"}</Button>
          <Button variant="outline" onClick={step} disabled={!sandbox || loading}><Clock3 className="mr-2 h-4 w-4" />推进时间</Button>
          <Button variant="outline" onClick={saveMemory} disabled={!sandbox || loading}><Save className="mr-2 h-4 w-4" />保存本次演练 Memory</Button>
        </div>
      </div>

      {!sandbox && (
        <Card>
          <CardContent className="p-8">
            <div className="grid gap-6 xl:grid-cols-[0.9fr_1.1fr]">
              <div className="space-y-4">
                <div className="flex items-center gap-2 text-sm font-black uppercase tracking-[0.16em] text-slate-500"><MessageSquare className="h-4 w-4" />先生成沙盘</div>
                <textarea value={prompt} onChange={(e) => setPrompt(e.target.value)} className="min-h-40 w-full rounded-2xl border bg-background px-4 py-3 text-sm outline-none" />
                <div className="rounded-2xl border border-dashed p-4 text-sm text-muted-foreground">示例：生成一个失业严重、工厂停摆、社会情绪紧张的工业小镇</div>
              </div>
              <div className="rounded-[28px] border-4 border-dashed border-slate-300 bg-slate-50 p-8 text-center text-slate-500">点击“生成沙盘”后，这里才会展示 GUI 社会沙盘与完整配置。</div>
            </div>
          </CardContent>
        </Card>
      )}

      {sandbox && (
        <>
          <div className="grid gap-6 xl:grid-cols-[1.15fr_0.85fr]">
            <Card className="overflow-hidden border-[#6d7f96] bg-[#cadf9a]">
              <CardContent className="p-4">
                <div className={`relative h-[620px] overflow-hidden rounded-[24px] border-4 border-[#8ea4bb] ${sandbox.theme === "industry" ? "bg-[#d8cf9f]" : "bg-[#caea8d]"}`}>
                  <div className="absolute inset-0 opacity-40 bg-[radial-gradient(circle_at_20%_20%,rgba(255,255,255,0.18)_0_2px,transparent_2px)] bg-[size:28px_28px]" />
                  {sandbox.districts.map((d) => (
                    <div key={d.name} className="absolute rounded-[18px] border-4 border-[#6f7c68]" style={{ left: `${d.x}%`, top: `${d.y}%`, width: `${d.w}%`, height: `${d.h}%`, background: d.color }}>
                      <div className="absolute left-2 top-2 rounded bg-black/20 px-2 py-1 text-[11px] font-bold text-white">{d.name}</div>
                    </div>
                  ))}
                  <div className="absolute left-[30%] top-[28%] h-[10%] w-[40%] rounded-[14px] bg-[#f0de92]" />
                  <div className="absolute left-[16%] top-[44%] h-[12%] w-[56%] rounded-[14px] bg-[#f0de92]" />
                  {sandbox.agents.map((a) => {
                    const p = movePos(a);
                    return <button key={a.id} onClick={() => setSelectedId(a.id)} className={`absolute z-30 h-5 w-5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 ${selected?.id === a.id ? "border-amber-100 bg-amber-300 shadow-[0_0_20px_rgba(252,211,77,0.95)]" : "border-white bg-sky-400 shadow-[0_0_18px_rgba(56,189,248,0.95)]"}`} style={{ left: `${p.left}%`, top: `${p.top}%` }} title={a.name} />;
                  })}
                  <div className="absolute left-[4%] top-[4%] rounded-[16px] border-4 border-[#8ea4bb] bg-[#5d6877]/95 p-3 text-white">
                    <div className="text-[12px] font-black">{sandbox.name}</div>
                    <div className="mt-2 rounded-[10px] bg-[#f4f4ef] px-3 py-2 text-[12px] text-slate-800">{sandbox.premise}</div>
                  </div>
                </div>
              </CardContent>
            </Card>

            <div className="space-y-6">
              <Card className="border-[#8ea4bb] bg-[#4e5f75] text-white">
                <CardContent className="p-5">
                  <div className="mb-3 flex items-center gap-2 text-sm font-semibold uppercase tracking-[0.18em] text-sky-200"><Users className="h-4 w-4" />移动中的 Agent</div>
                  {selected && (
                    <div className="space-y-3">
                      <div className="text-2xl font-black">{selected.name}</div>
                      <div className="text-sm text-slate-200">{selected.role}</div>
                      <InfoLine label="from" value={selected.from} />
                      <InfoLine label="to" value={selected.to} />
                      <InfoLine label="status" value={selected.status} />
                      <div className="rounded-[16px] border-2 border-white/15 bg-[#36465b] p-4 text-sm leading-6 text-slate-100">{selected.action}</div>
                      <div className="rounded-[16px] border-2 border-white/15 bg-[#36465b] p-4 text-sm leading-6 text-slate-100">{selected.memory}</div>
                    </div>
                  )}
                </CardContent>
              </Card>
              <Card><CardContent className="p-5"><div className="mb-3 text-sm font-black uppercase tracking-[0.16em] text-slate-500">当前社会协议</div><div className="rounded-xl border p-3 text-sm">{sandbox.protocol}</div><div className="mt-3 space-y-2">{sandbox.institutions.map((i) => <div key={i} className="rounded-xl border p-3 text-sm">{i}</div>)}</div></CardContent></Card>
              <Card><CardContent className="p-5"><div className="mb-3 flex items-center gap-2 text-sm font-black uppercase tracking-[0.16em] text-slate-500"><ScrollText className="h-4 w-4" />过程留痕</div><div className="space-y-2">{sandbox.logs.map((l, i) => <div key={`${l.tick}-${i}`} className="rounded-xl border p-3 text-sm"><div className="font-semibold">T{l.tick}</div><div className="mt-1">{l.text}</div></div>)}</div></CardContent></Card>
            </div>
          </div>

          <div className="grid gap-6 xl:grid-cols-3">
            <Card><CardContent className="p-5"><div className="mb-3 flex items-center gap-2 text-sm font-black uppercase tracking-[0.16em] text-slate-500"><Network className="h-4 w-4" />Governance Rules</div><div className="space-y-2">{sandbox.rules.map((r) => <div key={r} className="rounded-xl border p-3 text-sm">{r}</div>)}</div></CardContent></Card>
            <Card><CardContent className="p-5"><div className="mb-3 text-sm font-black uppercase tracking-[0.16em] text-slate-500">Event Templates</div><div className="space-y-2">{sandbox.templates.map((t) => <div key={t} className="rounded-xl border p-3 text-sm">{t}</div>)}</div></CardContent></Card>
            <Card><CardContent className="p-5"><div className="mb-3 text-sm font-black uppercase tracking-[0.16em] text-slate-500">Memory Archive</div><div className="space-y-2">{sandbox.archive.map((a) => <div key={a} className="rounded-xl border p-3 text-sm">{a}</div>)}</div></CardContent></Card>
          </div>
        </>
      )}
    </div>
  );
}

function InfoLine({ label, value }: { label: string; value: string }) {
  return <div className="flex items-center justify-between rounded-xl border border-white/15 bg-white/5 px-3 py-2 text-sm"><span className="font-mono text-sky-200">{label}</span><span>{value}</span></div>;
}
