import argparse
import json
import statistics
import time
import urllib.request
from collections import Counter
from pathlib import Path


DEFAULT_DATASET = Path(__file__).resolve().parent / "data" / "exp23_default_tasks.json"


def load_tasks(path):
    dataset_path = Path(path) if path else DEFAULT_DATASET
    tasks = json.loads(dataset_path.read_text(encoding="utf-8"))
    if not isinstance(tasks, list):
        raise ValueError("task dataset must be a JSON array")
    return tasks


def post_json(url, payload, timeout):
    req = urllib.request.Request(
        url,
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    start = time.time()
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        body = resp.read().decode("utf-8")
    return json.loads(body), int((time.time() - start) * 1000)


def keyword_coverage(text, keywords):
    if not keywords:
        return 1.0, []
    lowered = (text or "").lower()
    matched = []
    for keyword in keywords:
        if keyword.lower() in lowered:
            matched.append(keyword)
    return len(matched) / len(keywords), matched


def make_payload(task, experiment):
    return {
        "session_id": f"{experiment['experiment_group']}-{task['id']}",
        "messages": [{"role": "user", "content": task["prompt"]}],
        "complexity_hint": task.get("complexity_hint", 2),
        "required_tools": task.get("required_tools", []),
        "required_capabilities": task.get("required_capabilities", []),
        "token_optimization": False,
        "cost_optimization": False,
        "disable_tool_inference": True,
        "experiment": experiment,
    }


def run_group(name, experiment, tasks, endpoint, timeout, coverage_threshold):
    results = []
    for idx, task in enumerate(tasks, start=1):
        response, latency_ms = post_json(endpoint, make_payload(task, experiment), timeout)
        trace = response.get("execution_trace") or {}
        assistant = response.get("assistant_message") or ""
        coverage, matched = keyword_coverage(assistant, task.get("expected_keywords", []))
        api_success = bool(response.get("succeeded"))
        semantic_success = api_success and coverage >= coverage_threshold
        results.append(
            {
                "task_id": task.get("id", f"task-{idx}"),
                "label": task.get("label", f"Task {idx}"),
                "client_latency_ms": latency_ms,
                "api_success": api_success,
                "semantic_success": semantic_success,
                "keyword_coverage": coverage,
                "matched_keywords": matched,
                "selected_agent_id": response.get("selected_agent", {}).get("id"),
                "route_source": response.get("route_source"),
                "memory_influence": response.get("memory_influence"),
                "tool_calls": trace.get("tool_calls", 0),
                "tool_names": trace.get("tool_names", []),
                "iterations": trace.get("iterations", 0),
                "max_iterations_hit": bool(trace.get("max_iterations_hit")),
                "assistant_chars": len(assistant),
                "error_message": response.get("error_message"),
                "response": response,
            }
        )
        print(
            f"[{name}] {task.get('id')} api_success={api_success} semantic_success={semantic_success} "
            f"coverage={coverage:.2f} tool_calls={trace.get('tool_calls', 0)} latency_ms={latency_ms}"
        )
    return results


def summarize(name, results):
    latencies = [r["client_latency_ms"] for r in results]
    coverage = [r["keyword_coverage"] for r in results]
    tool_calls = [r["tool_calls"] for r in results]
    assistant_chars = [r["assistant_chars"] for r in results]
    return {
        "name": name,
        "run_count": len(results),
        "api_success_rate": sum(1 for r in results if r["api_success"]) / len(results) if results else 0.0,
        "semantic_success_rate": sum(1 for r in results if r["semantic_success"]) / len(results) if results else 0.0,
        "keyword_coverage_mean": statistics.mean(coverage) if coverage else 0.0,
        "latency_mean_ms": statistics.mean(latencies) if latencies else 0.0,
        "latency_stdev_ms": statistics.stdev(latencies) if len(latencies) >= 2 else 0.0,
        "tool_calls_mean": statistics.mean(tool_calls) if tool_calls else 0.0,
        "tool_usage_rate": sum(1 for r in results if r["tool_calls"] > 0) / len(results) if results else 0.0,
        "max_iterations_hit_count": sum(1 for r in results if r["max_iterations_hit"]),
        "assistant_chars_mean": statistics.mean(assistant_chars) if assistant_chars else 0.0,
        "selected_agent_distribution": dict(Counter(r["selected_agent_id"] for r in results)),
    }


def format_distribution(mapping):
    if not mapping:
        return "-"
    return ", ".join(f"{k}:{v}" for k, v in mapping.items())


def render_markdown(tasks, baseline_results, enhanced_results, baseline_summary, enhanced_summary):
    lines = []
    lines.append("# Exp 2.3 工具增强机制贡献实验")
    lines.append("")
    lines.append("## 1. 实验目标")
    lines.append("")
    lines.append("对比“无工具 Agent”与“工具增强 Agent”，观察 MCP 工具注入是否显著提升复杂仓库分析任务的完成质量。")
    lines.append("")
    lines.append("说明：本轮默认任务集基于当前环境稳定可用的文件系统工具设计，重点验证工具增强机制本身，而不是所有外部工具类别。")
    lines.append("")
    lines.append("## 2. 总体统计")
    lines.append("")
    lines.append("| 组别 | API 成功率 | 语义成功率 | 关键词覆盖均值 | 平均时延(ms) | 平均 Tool Calls | Tool 使用率 | 命中最大迭代次数 | 平均回答长度(chars) | Selected Agent 分布 |")
    lines.append("| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |")
    for summary in [baseline_summary, enhanced_summary]:
        lines.append(
            "| {name} | {api_success_rate:.2%} | {semantic_success_rate:.2%} | {keyword_coverage_mean:.2f} | {latency_mean_ms:.2f} | {tool_calls_mean:.2f} | {tool_usage_rate:.2%} | {max_iterations_hit_count} | {assistant_chars_mean:.2f} | {selected_agent_distribution} |".format(
                **{
                    **summary,
                    "selected_agent_distribution": format_distribution(summary["selected_agent_distribution"]),
                }
            )
        )
    lines.append("")
    lines.append("## 3. 收益对比")
    lines.append("")
    lines.append("| 指标 | 无工具基线 | 工具增强链路 | 差异解读 |")
    lines.append("| --- | ---: | ---: | --- |")
    lines.append(f"| API 成功率 | {baseline_summary['api_success_rate']:.2%} | {enhanced_summary['api_success_rate']:.2%} | 反映接口层执行是否完成 |")
    lines.append(f"| 语义成功率 | {baseline_summary['semantic_success_rate']:.2%} | {enhanced_summary['semantic_success_rate']:.2%} | 反映回答是否覆盖任务要求的关键事实 |")
    lines.append(f"| 关键词覆盖均值 | {baseline_summary['keyword_coverage_mean']:.2f} | {enhanced_summary['keyword_coverage_mean']:.2f} | 反映回答对仓库真实信息的命中程度 |")
    lines.append(f"| 平均 Tool Calls | {baseline_summary['tool_calls_mean']:.2f} | {enhanced_summary['tool_calls_mean']:.2f} | 直接反映 MCP 工具使用情况 |")
    lines.append(f"| Tool 使用率 | {baseline_summary['tool_usage_rate']:.2%} | {enhanced_summary['tool_usage_rate']:.2%} | 反映任务执行是否真正触发工具链 |")
    lines.append(f"| 平均回答长度(chars) | {baseline_summary['assistant_chars_mean']:.2f} | {enhanced_summary['assistant_chars_mean']:.2f} | 可近似反映信息密度 |")
    lines.append("")
    lines.append("## 4. 单任务明细")
    lines.append("")
    lines.append("| Task | 标签 | 基线 API 成功 | 基线语义成功 | 基线关键词覆盖 | 基线 Tool Calls | 增强 API 成功 | 增强语义成功 | 增强关键词覆盖 | 增强 Tool Calls |")
    lines.append("| --- | --- | --- | --- | ---: | ---: | --- | --- | ---: | ---: |")
    by_task_baseline = {item["task_id"]: item for item in baseline_results}
    by_task_enhanced = {item["task_id"]: item for item in enhanced_results}
    for task in tasks:
        task_id = task["id"]
        left = by_task_baseline[task_id]
        right = by_task_enhanced[task_id]
        lines.append(
            f"| {task_id} | {task.get('label','')} | {left['api_success']} | {left['semantic_success']} | {left['keyword_coverage']:.2f} | {left['tool_calls']} | {right['api_success']} | {right['semantic_success']} | {right['keyword_coverage']:.2f} | {right['tool_calls']} |"
        )
    lines.append("")
    lines.append("## 5. 结论模板")
    lines.append("")
    lines.append("- 如果“工具增强链路”在语义成功率、关键词覆盖、Tool 使用率上显著优于基线，就可以证明 MCP 工具集成确实扩展了 Agent 的实际能力边界。")
    lines.append("- 如果两组 API 成功率接近，但工具增强链路语义成功率更高，则说明工具增强的价值主要体现在答案真实性与完整性，而不是单纯的接口可用性。")
    lines.append("- 如果工具增强链路出现较多 `max_iterations_hit`，则说明后续优化重点应放在 tool loop 控制，而不是否定工具增强思路本身。")
    lines.append("")
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser(description="Run Exp 2.3 tool-augmented agent comparison.")
    parser.add_argument("--endpoint", default="http://localhost:8081/control/conversation/chat")
    parser.add_argument("--dataset", help="JSON file containing task objects")
    parser.add_argument("--timeout", type=int, default=300)
    parser.add_argument("--output-dir", default="docs/experiment_runs")
    parser.add_argument("--coverage-threshold", type=float, default=0.6)
    args = parser.parse_args()

    tasks = load_tasks(args.dataset)
    baseline_experiment = {
        "experiment_id": "exp23",
        "experiment_group": "baseline",
        "baseline_name": "no-tools-agent",
        "memory_reuse_enabled": False,
        "team_routing_enabled": False,
        "candidate_strategy": "predictive",
        "tool_injection_enabled": False,
        "tool_scope_pruning_enabled": False,
        "max_tool_iterations": 2,
    }
    enhanced_experiment = {
        "experiment_id": "exp23",
        "experiment_group": "our-method",
        "baseline_name": "tool-augmented-agent",
        "memory_reuse_enabled": False,
        "team_routing_enabled": False,
        "candidate_strategy": "predictive",
        "tool_injection_enabled": True,
        "tool_scope_pruning_enabled": True,
        "max_tool_iterations": 6,
    }

    baseline_results = run_group("baseline-no-tools", baseline_experiment, tasks, args.endpoint, args.timeout, args.coverage_threshold)
    enhanced_results = run_group("our-tool-augmented", enhanced_experiment, tasks, args.endpoint, args.timeout, args.coverage_threshold)

    baseline_summary = summarize("baseline-no-tools", baseline_results)
    enhanced_summary = summarize("our-tool-augmented", enhanced_results)

    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    timestamp = time.strftime("%Y%m%d_%H%M%S")
    json_path = output_dir / f"exp23_{timestamp}.json"
    md_path = output_dir / f"exp23_{timestamp}.md"
    json_path.write_text(
        json.dumps(
            {
                "experiment": "Exp 2.3 工具增强机制贡献实验",
                "tasks": tasks,
                "baseline_summary": baseline_summary,
                "enhanced_summary": enhanced_summary,
                "baseline_results": baseline_results,
                "enhanced_results": enhanced_results,
            },
            ensure_ascii=False,
            indent=2,
        ),
        encoding="utf-8",
    )
    md_path.write_text(
        render_markdown(tasks, baseline_results, enhanced_results, baseline_summary, enhanced_summary),
        encoding="utf-8",
    )

    print(f"wrote {json_path}")
    print(f"wrote {md_path}")


if __name__ == "__main__":
    main()
