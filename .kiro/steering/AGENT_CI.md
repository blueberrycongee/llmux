<!------------------------------------------------------------------------------------
   LLMux 项目 CI 测试完整指南
   
   本文档用于指导 AI Agent 如何运行完整的 CI 测试流程
   包括：代码格式化、静态检查、Lint、单元测试、构建验证
-------------------------------------------------------------------------------------> 

# LLMux CI 测试指南

## 项目环境要求

- **Go 版本**: 1.24.0+ (项目使用 Go 1.24，不要降级到 1.23)
- **golangci-lint**: v1.64.2+ (支持 Go 1.24)
- **操作系统**: Windows/Linux/Mac

### ⚠️ 重要：为什么必须使用 Go 1.24？
项目依赖 OpenTelemetry v1.39.0，该版本强制要求 `go >= 1.24.0`。降级会破坏 observability 功能。

---

## 完整 CI 测试流程（一键运行）

### Windows (Git Bash)
```bash
gofmt -w . && go vet ./... && /c/Users/10758/go/bin/golangci-lint.exe run ./... && go test ./... -v -cover && go build -o llmux.exe ./cmd/server
```

### Linux/Mac
```bash
gofmt -w . && go vet ./... && golangci-lint run ./... && go test ./... -v -cover && go build -o llmux ./cmd/server
```

---

## 分步测试命令

### 1. 代码格式化

```bash
# 检查哪些文件需要格式化
gofmt -l .

# 自动格式化所有代码
gofmt -w .
```

**说明**: 必须先格式化代码，否则 CI 会失败。

---

### 2. 静态检查 (go vet)

```bash
go vet ./...
```

**说明**: Go 内置的静态分析工具，检查常见错误。

---

### 3. Lint 检查 (golangci-lint)

#### 安装 golangci-lint v1.64.2+

**✅ 推荐方法：使用官方脚本**

```bash
# Linux/Mac
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.64.2

# Windows (Git Bash)
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /c/Users/10758/go/bin v1.64.2
```

**备选方法：使用 go install**
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.2
```

#### 验证安装

```bash
# Windows
/c/Users/10758/go/bin/golangci-lint.exe version

# Linux/Mac
golangci-lint version

# 应该显示: golangci-lint has version 1.64.2 built with go1.24.0
```

#### 运行 Lint

```bash
# Windows
/c/Users/10758/go/bin/golangci-lint.exe run ./...

# Linux/Mac
golangci-lint run ./...

# GitHub Actions 格式输出
golangci-lint run --out-format=github-actions

# 只显示新问题
golangci-lint run --new-from-rev=HEAD~1
```

**⚠️ 重要**: 必须使用 v1.64.2+，旧版本不支持 Go 1.24。参考：[golangci-lint#5225](https://github.com/golangci/golangci-lint/issues/5225)

---

### 4. 单元测试

```bash
# 运行所有测试（简洁输出）
go test ./...

# 运行所有测试（详细输出）
go test ./... -v

# 运行测试并显示覆盖率
go test ./... -cover

# 生成覆盖率报告
go test ./... -coverprofile=coverage.out

# 在浏览器中查看覆盖率
go tool cover -html=coverage.out

# 查看覆盖率详情（按函数）
go tool cover -func=coverage.out

# 查看总体覆盖率
go tool cover -func=coverage.out | grep total
```

#### 运行特定包的测试

```bash
# 测试 auth 包
go test ./internal/auth/... -v

# 测试 router 包
go test ./internal/router/... -v

# 测试 observability 包
go test ./internal/observability/... -v

# 测试 metrics 包
go test ./internal/metrics/... -v

# 测试 cache 包
go test ./internal/cache/... -v
```

---

### 5. 竞态检测 (Race Detector)

```bash
# Linux/Mac
go test ./... -race

# Windows: 不推荐（有已知问题）
# 在 Windows 上 race detector 可能失败，建议在 CI 环境运行
```

**注意**: Windows 平台的 race detector 有已知限制（exit status 0xc0000139），建议在 Linux/Mac CI 环境中运行。

---

### 6. 构建验证

```bash
# Windows
go build -o llmux.exe ./cmd/server

# Linux/Mac
go build -o llmux ./cmd/server

# 验证构建
./llmux.exe --help  # Windows
./llmux --help      # Linux/Mac
```

---

### 7. 依赖管理

```bash
# 更新依赖
go mod tidy

# 下载依赖
go mod download

# 验证依赖
go mod verify

# 查看依赖树
go mod graph
```

---

## 测试报告生成

### 生成 JSON 格式测试报告

```bash
go test ./... -v -json > test-report.json
```

### 生成覆盖率报告

```bash
# 生成覆盖率文件
go test ./... -coverprofile=coverage.out

# 查看总体覆盖率
go tool cover -func=coverage.out | grep total

# 生成 HTML 报告
go tool cover -html=coverage.out -o coverage.html
```

---

## CI/CD 配置示例

### GitHub Actions

```yaml
name: CI Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      
      - name: Format check
        run: |
          gofmt -l . | grep . && exit 1 || exit 0
      
      - name: Go vet
        run: go vet ./...
      
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: v1.64.2
          args: --out-format=github-actions
      
      - name: Run tests
        run: go test ./... -v -race -cover -coverprofile=coverage.out
      
      - name: Build
        run: go build -o llmux ./cmd/server
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.out
```

---

## 常见问题排查

### Q1: golangci-lint 报错 "go version mismatch"
**原因**: golangci-lint 版本太旧，不支持 Go 1.24  
**解决**: 升级到 v1.64.2+
```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.64.2
```

### Q2: 测试失败 "go.mod requires go >= 1.24.0"
**原因**: Go 版本太旧  
**解决**: 升级 Go 到 1.24+
```bash
go version  # 检查当前版本
```

### Q3: Windows 上 race detector 失败
**原因**: Windows 平台的已知限制  
**解决**: 在 Linux/Mac CI 环境运行，或跳过 race detector

### Q4: golangci-lint 发现很多问题
**原因**: 代码质量问题  
**解决**: 根据报告逐个修复，优先修复 errcheck 和 govet 问题

### Q5: 项目能否降级到 Go 1.23？
**答案**: 不能！OpenTelemetry v1.39.0 强制要求 Go 1.24，降级会破坏功能。

---

## 测试覆盖率目标

- **当前覆盖率**: 30.8%
- **目标覆盖率**: >50%
- **优秀模块**: internal/resilience (97.6%), pkg/errors (93.8%)
- **需要改进**: internal/observability (8.7%), internal/api (0.0%)

---

## Agent 执行检查清单

当 Agent 需要运行 CI 测试时，按以下顺序执行：

- [ ] 1. 检查 Go 版本 (必须 >= 1.24)
- [ ] 2. 运行 `gofmt -w .` 格式化代码
- [ ] 3. 运行 `go vet ./...` 静态检查
- [ ] 4. 检查 golangci-lint 是否安装且版本 >= v1.64.2
- [ ] 5. 运行 `golangci-lint run ./...` (如果已安装)
- [ ] 6. 运行 `go test ./... -v -cover` 单元测试
- [ ] 7. 运行 `go build -o llmux.exe ./cmd/server` 构建验证
- [ ] 8. 检查所有步骤是否通过
- [ ] 9. 生成测试报告（可选）

---

## 快速参考

### 本地开发（Windows）
```bash
cd d:\Desktop\LLMux
gofmt -w . && go vet ./... && /c/Users/10758/go/bin/golangci-lint.exe run ./... && go test ./... -v
```

### 提交前检查
```bash
gofmt -w . && go vet ./... && golangci-lint run ./... && go test ./... -cover && go build -o llmux.exe ./cmd/server
```

### 查看覆盖率
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

## 参考资源

- [golangci-lint Go 1.24 支持](https://github.com/golangci/golangci-lint/issues/5225)
- [golangci-lint 配置文件](../.golangci.yml)
- [Go 测试文档](https://golang.org/pkg/testing/)
- [完整测试结果](../COMPLETE_CI_TEST_RESULTS.md)
