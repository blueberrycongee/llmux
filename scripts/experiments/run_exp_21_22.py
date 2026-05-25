import argparse
import json
import statistics
import time
import urllib.request
from collections import Counter
from pathlib import Path


DEFAULT_DATASET = [
    "请分析当前仓库的核心模块划分、路由逻辑和关键价值点，必要时使用文件工具。",
    "请结合当前代码结构说明这个 gateway 的治理内核、路由层和 MCP 工具增强分别解决了什么问题。",
    "请从架构角度解释 route memory、candidate scheduling 和 tool injection 在这个项目里的关系。",
    "请基于仓库代码说明为什么这个项目既是 LLM gateway，又在向 agent routing hub 演进。",
    "请用工程角度分析这个项目的 baseline gateway 能力和优化链路能力分别是什么。",
]


def bool_ptr(value):
    return value


def post_json(url, payload, timeout):
    data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    start = time.time()
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        body = resp.read().decode("utf-8")
    latency_ms = int((time.time() - start) * 1000)
    return json.loads(body), latency_ms


def run_group(name, experiment_payload, prompts, endpoint, timeout):
    runs = []
    for idx, prompt in enumerate(prompts, start=1):
        payload = {
            "session_id": f"{name}-{idx}",
            "messages": [{"role": "user", "content": prompt}],
            "complexity_hint": 2,
            "required_tools": ["preset-filesystem-list", "preset-filesystem-read"],
            "required_capabilities": ["architecture", "coding", "analysis"],
            "token_optimization": True,
            "cost_optimization": True,
            "experiment": experiment_payload,
        }
        response, latency_ms = post_json(endpoint, payload, timeout)
        selected_candidate = response.get("selected_candidate") or {}
        runs.append(
            {
                "run": idx,
                "prompt": prompt,
                "client_latency_ms": latency_ms,
                "succeeded": response.get("succeeded"),
                "route_source": response.get("route_source"),
                "selected_agent_id": response.get("selected_agent", {}).get("id"),
                "selected_candidate": (
                    f"{selected_candidate.get('provider','')}/{selected_candidate.get('model','')}"
                    if selected_candidate
                    else "-"
                ),
                "memory_influence": response.get("memory_influence"),
                "consulted_memories": len(response.get("consulted_memories") or []),
                "candidate_failovers": len(response.get("candidate_failovers") or []),
                "error_message": response.get("error_message"),
                "response": response,
            }
        )
        print(
            f"[{name}] run {idx}/{len(prompts)} "
            f"succeeded={response.get('succeeded')} "
            f"latency_ms={latency_ms} "
            f"agent={response.get('selected_agent', {}).get('id')}"
        )
    return runs


def summarize_runs(name, runs):
    latencies = [item["client_latency_ms"] for item in runs]
    success = [item["succeeded"] for item in runs]
    memory = Counter(item["memory_influence"] for item in runs)
    route = Counter(item["route_source"] for item in runs)
    agent = Counter(item["selected_agent_id"] for item in runs)
    failover = [item["candidate_failovers"] for item in runs]
    consulted = [item["consulted_memories"] for item in runs]
    return {
        "name": name,
        "run_count": len(runs),
        "success_count": sum(1 for item in success if item),
        "success_rate": sum(1 for item in success if item) / len(runs) if runs else 0,
        "latency_mean_ms": statistics.mean(latencies) if latencies else None,
        "latency_stdev_ms": statistics.stdev(latencies) if len(latencies) >= 2 else None,
        "memory_influence": dict(memory),
        "route_source": dict(route),
        "selected_agent": dict(agent),
        "candidate_failover_mean": statistics.mean(failover) if failover else None,
        "consulted_memories_mean": statistics.mean(consulted) if consulted else None,
    }


def render_markdown(exp_name, summaries):
    lines = [f"# {exp_name} 实验结果", ""]
    lines.append("| 组别 | 成功率 | 平均时延(ms) | Memory Influence | Route Source | Selected Agent | 平均 Failover | 平均 Consulted Memories |")
    lines.append("| --- | ---: | ---: | --- | --- | --- | ---: | ---: |")
    for item in summaries:
        lines.append(
            "| {name} | {success_rate:.2%} | {latency} | {memory} | {route} | {agent} | {failover} | {consulted} |".format(
                name=item["name"],
                success_rate=item["success_rate"],
                latency=f"{item['latency_mean_ms']:.2f}" if item["latency_mean_ms"] is not None else "-",
                memory=", ".join(f"{k}:{v}" for k, v in item["memory_influence"].items()) or "-",
                route=", ".join(f"{k}:{v}" for k, v in item["route_source"].items()) or "-",
                agent=", ".join(f"{k}:{v}" for k, v in item["selected_agent"].items()) or "-",
                failover=f"{item['candidate_failover_mean']:.2f}" if item["candidate_failover_mean"] is not None else "-",
                consulted=f"{item['consulted_memories_mean']:.2f}" if item["consulted_memories_mean"] is not None else "-",
            )
        )
    lines.append("")
    return "\n".join(lines) + "\n"


def load_dataset(path):
    if not path:
        return DEFAULT_DATASET
    data = json.loads(Path(path).read_text(encoding="utf-8"))
    if not isinstance(data, list) or not all(isinstance(item, str) for item in data):
        raise ValueError("dataset file must be a JSON array of prompt strings")
    return data


def main():
    parser = argparse.ArgumentParser(description="Run LLMux experiment groups for Exp 2.1 and Exp 2.2.")
    parser.add_argument("--experiment", choices=["exp21", "exp22"], required=True)
    parser.add_argument("--endpoint", default="http://localhost:8081/control/conversation/chat")
    parser.add_argument("--dataset", help="JSON file containing an array of prompt strings")
    parser.add_argument("--timeout", type=int, default=300)
    parser.add_argument("--output-dir", default="docs/experiment_runs")
    args = parser.parse_args()

    prompts = load_dataset(args.dataset)
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    if args.experiment == "exp21":
        groups = [
            (
                "baseline-memoryless-routing",
                {
                    "experiment_id": "exp21",
                    "experiment_group": "baseline",
                    "baseline_name": "memoryless-routing",
                    "memory_reuse_enabled": False,
                    "team_routing_enabled": True,
                    "candidate_strategy": "predictive",
                    "tool_injection_enabled": True,
                },
            ),
            (
                "our-memory-augmented-routing",
                {
                    "experiment_id": "exp21",
                    "experiment_group": "our-method",
                    "baseline_name": "memory-augmented-routing",
                    "memory_reuse_enabled": True,
                    "team_routing_enabled": True,
                    "candidate_strategy": "predictive",
                    "tool_injection_enabled": True,
                },
            ),
        ]
        exp_name = "Exp 2.1 记忆增强路由"
    else:
        groups = [
            (
                "baseline-passive-fallback",
                {
                    "experiment_id": "exp22",
                    "experiment_group": "baseline",
                    "baseline_name": "passive-fallback",
                    "memory_reuse_enabled": True,
                    "candidate_strategy": "passive",
                },
            ),
            (
                "baseline-greedy-current",
                {
                    "experiment_id": "exp22",
                    "experiment_group": "baseline",
                    "baseline_name": "greedy-current",
                    "memory_reuse_enabled": True,
                    "candidate_strategy": "greedy-current",
                },
            ),
            (
                "our-predictive-scheduling",
                {
                    "experiment_id": "exp22",
                    "experiment_group": "our-method",
                    "baseline_name": "predictive-scheduling",
                    "memory_reuse_enabled": True,
                    "candidate_strategy": "predictive",
                },
            ),
        ]
        exp_name = "Exp 2.2 预测式调度"

    all_results = {}
    summaries = []
    for name, payload in groups:
        runs = run_group(name, payload, prompts, args.endpoint, args.timeout)
        all_results[name] = runs
        summaries.append(summarize_runs(name, runs))

    timestamp = time.strftime("%Y%m%d_%H%M%S")
    json_path = output_dir / f"{args.experiment}_{timestamp}.json"
    md_path = output_dir / f"{args.experiment}_{timestamp}.md"
    json_path.write_text(json.dumps({"experiment": exp_name, "summaries": summaries, "results": all_results}, ensure_ascii=False, indent=2), encoding="utf-8")
    md_path.write_text(render_markdown(exp_name, summaries), encoding="utf-8")

    print(f"wrote {json_path}")
    print(f"wrote {md_path}")


if __name__ == "__main__":
    main()
