# Router 包重构实施方案

## 一、现状分析

### 1.1 当前目录结构
```
pkg/router/           ← 公共接口（Router, Config, Strategy）
    └── router.go

internal/router/      ← 旧实现 + StatsStore 接口（混乱）
    ├── interface.go  ← Router 接口（重复）+ StatsStore 接口
    ├── types.go      ← 类型定义（重复）
    ├── base.go       ← 旧 BaseRouter
    ├── simple_shuffle.go   ← 旧策略 ❌ 废弃
    ├── least_busy.go       ← 旧策略 ❌ 废弃
    ├── lowest_latency.go   ← 旧策略 ❌ 废弃
    ├── lowest_cost.go      ← 旧策略 ❌ 废弃
    ├── lowest_tpm_rpm.go   ← 旧策略 ❌ 废弃
    ├── tag_based.go        ← 旧策略 ❌ 废弃
    ├── factory.go          ← 旧工厂 ❌ 废弃
    ├── simple.go           ← 旧简单路由 ❌ 废弃
    └── *_test.go           ← 旧测试

routers/              ← 新实现（生产用）
    ├── base.go       ← 新 BaseRouter（支持 StatsStore）
    ├── shuffle.go
    ├── leastbusy.go
    ├── latency.go
    ├── cost.go
    ├── tpmrpm.go
    ├── tagbased.go
    ├── factory.go    ← 依赖 internalRouter.StatsStore
    ├── stats_store.go         ← StatsStore 接口（重复定义）
    ├── memory_stats_store.go
    ├── redis_stats_store.go
    └── types.go      ← Re-export pkg/router 类型
```

### 1.2 问题清单

| # | 问题 | 影响 |
|---|------|------|
| 1 | `StatsStore` 接口定义在 `internal/router/interface.go`，但 `routers/stats_store.go` 又定义了一遍 | 维护混乱 |
| 2 | `Router` 接口在 `pkg/router/` 和 `internal/router/` 重复定义 | 语义不清 |
| 3 | `internal/router/` 下的策略实现已废弃但未删除 | 代码冗余 |
| 4 | `routers/factory.go` 依赖 `internal/router.StatsStore` | 依赖倒置 |
| 5 | 类型定义分散在三处 | 难以维护 |

---

## 二、目标架构

```
pkg/router/                    ← 公共接口层（对外暴露）
    ├── router.go              ← Router 接口 + Config + Strategy
    ├── stats_store.go         ← StatsStore 接口（新增）
    └── types.go               ← 所有公共类型

routers/                       ← 实现层（对外暴露）
    ├── base.go                ← BaseRouter 实现
    ├── shuffle.go             ← ShuffleRouter
    ├── leastbusy.go           ← LeastBusyRouter
    ├── latency.go             ← LatencyRouter
    ├── cost.go                ← CostRouter
    ├── tpmrpm.go              ← TPMRPMRouter
    ├── tagbased.go            ← TagBasedRouter
    ├── factory.go             ← 工厂函数
    ├── memory_stats_store.go  ← MemoryStatsStore 实现
    ├── redis_stats_store.go   ← RedisStatsStore 实现
    └── redis_scripts.go       ← Redis Lua 脚本

internal/router/               ← 删除或仅保留内部工具
    └── (清空或删除整个目录)
```

---

## 三、实施步骤

### Phase 1: 接口迁移（无破坏性）

#### Step 1.1: 创建 `pkg/router/stats_store.go`
将 `StatsStore` 接口从 `internal/router/interface.go` 迁移到 `pkg/router/`。

```go
// pkg/router/stats_store.go
package router

import (
    "context"
    "time"
)

// StatsStore defines the interface for distributed deployment statistics.
type StatsStore interface {
    GetStats(ctx context.Context, deploymentID string) (*DeploymentStats, error)
    IncrementActiveRequests(ctx context.Context, deploymentID string) error
    DecrementActiveRequests(ctx context.Context, deploymentID string) error
    RecordSuccess(ctx context.Context, deploymentID string, metrics *ResponseMetrics) error
    RecordFailure(ctx context.Context, deploymentID string, err error) error
    SetCooldown(ctx context.Context, deploymentID string, until time.Time) error
    GetCooldownUntil(ctx context.Context, deploymentID string) (time.Time, error)
    ListDeployments(ctx context.Context) ([]string, error)
    DeleteStats(ctx context.Context, deploymentID string) error
    Close() error
}
```

#### Step 1.2: 更新 `routers/` 包的 import
将 `routers/factory.go` 和 `routers/base.go` 中的：
```go
import internalRouter "github.com/blueberrycongee/llmux/internal/router"
```
改为：
```go
import "github.com/blueberrycongee/llmux/pkg/router"
```

#### Step 1.3: 删除 `routers/stats_store.go` 中的重复定义
该文件中的 `StatsStore` 接口已迁移到 `pkg/router/`，删除重复定义，保留错误变量。

---

### Phase 2: 清理旧实现

#### Step 2.1: 删除 `internal/router/` 下的废弃文件

**待删除文件清单：**
```
internal/router/simple_shuffle.go
internal/router/least_busy.go
internal/router/lowest_latency.go
internal/router/lowest_cost.go
internal/router/lowest_tpm_rpm.go
internal/router/tag_based.go
internal/router/factory.go
internal/router/simple.go
internal/router/base.go
internal/router/types.go
internal/router/interface.go
```

**待删除测试文件：**
```
internal/router/router_test.go
internal/router/simple_test.go
internal/router/stats_store_test.go
```

#### Step 2.2: 删除整个 `internal/router/` 目录
确认无其他包依赖后，删除整个目录。

---

### Phase 3: 更新依赖方

#### Step 3.1: 全局搜索 `internal/router` 引用
```bash
grep -r "internal/router" --include="*.go" .
```

#### Step 3.2: 逐一更新 import 路径
将所有 `internal/router` 引用改为 `pkg/router` 或 `routers`。

---

### Phase 4: 测试验证

#### Step 4.1: 运行 CI 流程
```bash
gofmt -w .
go vet ./...
golangci-lint run ./...
go test ./... -v -cover
go build -o llmux.exe ./cmd/server
```

#### Step 4.2: 验证分布式路由功能
确保 `RedisStatsStore` 正常工作。

---

## 四、文件变更清单

### 新增文件
| 文件 | 说明 |
|------|------|
| `pkg/router/stats_store.go` | StatsStore 接口定义 |

### 修改文件
| 文件 | 变更内容 |
|------|----------|
| `routers/factory.go` | 更新 import，使用 `router.StatsStore` |
| `routers/base.go` | 更新 import，使用 `router.StatsStore` |
| `routers/stats_store.go` | 删除重复的 StatsStore 接口定义 |
| `routers/memory_stats_store.go` | 更新 import |
| `routers/redis_stats_store.go` | 更新 import |

### 删除文件
| 文件 | 原因 |
|------|------|
| `internal/router/simple_shuffle.go` | 已被 `routers/shuffle.go` 替代 |
| `internal/router/least_busy.go` | 已被 `routers/leastbusy.go` 替代 |
| `internal/router/lowest_latency.go` | 已被 `routers/latency.go` 替代 |
| `internal/router/lowest_cost.go` | 已被 `routers/cost.go` 替代 |
| `internal/router/lowest_tpm_rpm.go` | 已被 `routers/tpmrpm.go` 替代 |
| `internal/router/tag_based.go` | 已被 `routers/tagbased.go` 替代 |
| `internal/router/factory.go` | 已被 `routers/factory.go` 替代 |
| `internal/router/simple.go` | 废弃 |
| `internal/router/base.go` | 已被 `routers/base.go` 替代 |
| `internal/router/types.go` | 已迁移到 `pkg/router/router.go` |
| `internal/router/interface.go` | 已迁移到 `pkg/router/` |
| `internal/router/router_test.go` | 旧测试 |
| `internal/router/simple_test.go` | 旧测试 |
| `internal/router/stats_store_test.go` | 旧测试 |

---

## 五、风险评估

| 风险 | 等级 | 缓解措施 |
|------|------|----------|
| 外部包依赖 `internal/router` | 低 | `internal/` 包按 Go 约定不应被外部引用 |
| 测试覆盖不足 | 中 | 确保 `routers/` 下有完整测试 |
| Redis 功能回归 | 中 | 手动测试分布式场景 |

---

## 六、执行顺序

```
1. [Phase 1] 创建 pkg/router/stats_store.go
2. [Phase 1] 更新 routers/ 包的 import
3. [Phase 1] 清理 routers/stats_store.go 重复定义
4. [Phase 2] 删除 internal/router/ 废弃文件
5. [Phase 3] 全局搜索并更新依赖
6. [Phase 4] 运行 CI 验证
7. [Phase 4] 功能测试
```

---

## 七、回滚方案

如果出现问题，可通过 Git 回滚：
```bash
git checkout HEAD -- internal/router/
git checkout HEAD -- routers/
git checkout HEAD -- pkg/router/
```

---

## 八、预计工时

| 阶段 | 预计时间 |
|------|----------|
| Phase 1: 接口迁移 | 30 分钟 |
| Phase 2: 清理旧实现 | 15 分钟 |
| Phase 3: 更新依赖 | 15 分钟 |
| Phase 4: 测试验证 | 30 分钟 |
| **总计** | **~1.5 小时** |
