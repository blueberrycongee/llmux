# LLMux Benchmark

本目录包含 LLMux 的性能测试工具，用于测量代理吞吐量、延迟和资源使用情况。

## 测试模式

### Mock 模式（推荐）

使用 Mock LLM Server 模拟 API 响应，**不需要真实 API Key，零成本**。

```bash
# 一键运行 benchmark
go run ./bench/cmd/runner

# 或分步运行
# 1. 启动 Mock Server
go run ./bench/cmd/mock &

# 2. 启动 LLMux（另一个终端）
go run ./cmd/server

# 3. 运行压测（另一个终端）
go run ./bench/cmd/runner --target http://localhost:3000
```

## 目录结构

```
bench/
├── cmd/
│   ├── mock/           # Mock LLM Server
│   │   └── main.go
│   └── runner/         # Benchmark 运行器
│       └── main.go
├── internal/
│   ├── mock/           # Mock 服务器逻辑
│   │   └── server.go
│   ├── runner/         # Benchmark 运行逻辑
│   │   └── runner.go
│   └── report/         # 报告生成
│       └── report.go
├── scenarios/          # 测试场景
│   └── basic.go
├── results/            # 测试结果（不提交）
└── README.md
```

## Benchmark 指标

| 指标 | 说明 |
|------|------|
| **RPS** | Requests Per Second，每秒请求数 |
| **Latency P50** | 50% 请求的延迟 |
| **Latency P95** | 95% 请求的延迟 |
| **Latency P99** | 99% 请求的延迟 |
| **Memory** | 内存使用量 |
| **Errors** | 错误率 |

## 与 LiteLLM 对比

```bash
# 1. 启动 Mock Server
go run ./bench/cmd/mock &

# 2. 测试 LLMux
go run ./bench/cmd/runner --target http://localhost:3000 --name llmux

# 3. 启动 LiteLLM (另开终端)
# litellm --model openai/gpt-4o --api_base http://localhost:8080 --port 4000

# 4. 测试 LiteLLM
go run ./bench/cmd/runner --target http://localhost:4000 --name litellm

# 5. 对比结果
go run ./bench/cmd/runner --compare results/llmux.json results/litellm.json
```

## 配置参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--target` | `http://localhost:3000` | 目标服务器地址 |
| `--requests` | `10000` | 总请求数 |
| `--concurrency` | `100` | 并发数 |
| `--duration` | `0` | 持续时间（秒），0 表示按请求数 |
| `--name` | `benchmark` | 测试名称 |
| `--output` | `results/` | 结果输出目录 |

## 测试场景

### 1. Basic Chat Completion

```bash
go run ./bench/cmd/runner --scenario basic
```

### 2. Streaming Response

```bash
go run ./bench/cmd/runner --scenario streaming
```

### 3. High Concurrency

```bash
go run ./bench/cmd/runner --concurrency 1000
```
