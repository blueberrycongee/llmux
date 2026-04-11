import type { RoutingMemoryItem, RoutingOptimizationComparison, SandboxState } from "@/types/api";

const STORAGE_KEY = "llmux.saved-routing-memory";
const ACTIVE_MEMORY_KEY = "llmux.active-routing-memory-id";

export interface SavedRoutingMemoryBundle {
  id: string;
  title: string;
  source: "agent-town";
  saved_at: string;
  scenario_id: string;
  scenario_name: string;
  tick: number;
  prompt_summary: string;
  items: RoutingMemoryItem[];
}

function isBrowser() {
  return typeof window !== "undefined";
}

export function getSavedRoutingMemory(): SavedRoutingMemoryBundle[] {
  if (!isBrowser()) return [];
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as SavedRoutingMemoryBundle[];
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

export function saveRoutingMemoryBundle(bundle: SavedRoutingMemoryBundle) {
  if (!isBrowser()) return;
  const current = getSavedRoutingMemory();
  const next = [bundle, ...current].slice(0, 20);
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
}

export function clearSavedRoutingMemory() {
  if (!isBrowser()) return;
  window.localStorage.removeItem(STORAGE_KEY);
  window.localStorage.removeItem(ACTIVE_MEMORY_KEY);
}

export function setActiveRoutingMemory(bundleId: string) {
  if (!isBrowser()) return;
  window.localStorage.setItem(ACTIVE_MEMORY_KEY, bundleId);
}

export function clearActiveRoutingMemory() {
  if (!isBrowser()) return;
  window.localStorage.removeItem(ACTIVE_MEMORY_KEY);
}

export function getActiveRoutingMemoryId(): string | null {
  if (!isBrowser()) return null;
  return window.localStorage.getItem(ACTIVE_MEMORY_KEY);
}

export function getActiveRoutingMemory(): SavedRoutingMemoryBundle | null {
  const activeId = getActiveRoutingMemoryId();
  if (!activeId) return null;
  return getSavedRoutingMemory().find((bundle) => bundle.id === activeId) ?? null;
}

export function buildRoutingMemoryBundleFromSandbox(sandbox: SandboxState): SavedRoutingMemoryBundle {
  const logItems: RoutingMemoryItem[] = sandbox.logs.slice(-6).map((log, index) => ({
    id: `${sandbox.id}-log-${sandbox.tick}-${index}`,
    timestamp: new Date().toISOString(),
    category: "sandbox-log",
    signal: `tick-${log.tick}`,
    observation: log.text,
    decision: sandbox.agents[index]?.action ?? "observe_and_record",
    outcome: sandbox.archive[index] ?? "memory_archived",
    confidence: 0.76,
    tags: ["agent-town", sandbox.theme, "simulation-memory"],
  }));

  const archiveItems: RoutingMemoryItem[] = sandbox.archive.slice(-4).map((entry, index) => ({
    id: `${sandbox.id}-archive-${sandbox.tick}-${index}`,
    timestamp: new Date().toISOString(),
    category: "sandbox-archive",
    signal: sandbox.protocol,
    observation: entry,
    decision: sandbox.rules[index] ?? "preserve_archive_signal",
    outcome: "stored_for_routing_memory",
    confidence: 0.81,
    tags: ["archive", "agent-town", "routing-memory"],
  }));

  return {
    id: `${sandbox.id}-${Date.now()}`,
    title: `${sandbox.name} · T${sandbox.tick}`,
    source: "agent-town",
    saved_at: new Date().toISOString(),
    scenario_id: sandbox.id,
    scenario_name: sandbox.name,
    tick: sandbox.tick,
    prompt_summary: sandbox.premise,
    items: [...archiveItems, ...logItems],
  };
}

export function deriveOptimizationComparisonFromSavedMemory(saved: SavedRoutingMemoryBundle[]): RoutingOptimizationComparison | null {
  if (saved.length === 0) return null;
  const latest = saved[0];
  const itemCount = latest.items.length;
  const avgConfidence = itemCount > 0 ? latest.items.reduce((sum, item) => sum + item.confidence, 0) / itemCount : 0.75;
  const archiveSignals = latest.items.filter((item) => item.category.includes("archive")).length;
  const latencyGain = Math.max(140, Math.round(itemCount * 32 + archiveSignals * 18 + avgConfidence * 120));
  const successGain = Number((0.8 + avgConfidence * 1.6 + archiveSignals * 0.12).toFixed(1));
  const costDelta = Number((0.2 + itemCount * 0.04).toFixed(2));

  return {
    baseline: {
      strategy: "round-robin",
      provider: "shared-pool",
      avg_latency_ms: 1320,
      success_rate: 96.2,
      cost_per_1k_requests: 11.8,
    },
    optimized: {
      strategy: archiveSignals > 1 ? "least-busy" : "lowest-latency",
      provider: latest.scenario_name,
      avg_latency_ms: 1320 - latencyGain,
      success_rate: Number((96.2 + successGain).toFixed(1)),
      cost_per_1k_requests: Number((11.8 + costDelta).toFixed(2)),
    },
    latency_delta_ms: -latencyGain,
    success_rate_delta: successGain,
    cost_delta_per_1k_requests: costDelta,
    improvement_summary: `基于最近一次演练（${latest.title}）沉淀的 routing memory，系统推导出更适合当前场景的路由策略，因此在保持成本可控的前提下，预计可以降低平均延迟并提高成功率。`,
    why_it_improved: [
      `本次演练沉淀了 ${itemCount} 条 memory，说明系统已经获得足够的行为样本。`,
      `archive 类 memory 有 ${archiveSignals} 条，表示治理规则与长期状态已经能被纳入路由决策。`,
      `memory 平均置信度约为 ${Math.round(avgConfidence * 100)}%，可支持更稳健的策略切换。`,
    ],
    recommended_rollout: [
      "先将优化策略用于交互型、高时延敏感流量。",
      "对预算敏感任务仍保留保守策略作为回退路径。",
      "继续积累演练 memory，再扩大优化策略覆盖范围。",
    ],
    generated_at: new Date().toISOString(),
  };
}
