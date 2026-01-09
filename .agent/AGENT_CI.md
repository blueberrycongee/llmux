LLMux 本地 CI/Lint 操作手册
1. 核心原则 (防坑必读)
配置不动原则：
.golangci.yml
 是经过调试的基准文件。如果 Lint 报错，优先修改代码（修复逻辑或添加 //nolint），严禁随意修改配置文件来掩盖问题。
环境一致性：必须使用 Go 1.24+。由于 Windows 环境变量有时混乱，建议直接使用 golangci-lint 的绝对路径。
2. 一键运行 (推荐)
在项目根目录（Git Bash）中，直接复制运行以下“黄金命令”。它会按顺序执行：格式化 -> 静态检查 -> Lint -> 测试 -> 构建。

bash
gofmt -w . && go vet ./... && /c/Users/10758/go/bin/golangci-lint.exe run ./... && go test ./... -v -cover && go build -o llmux.exe ./cmd/server
说明：只要这个命令跑通 (Exit Code 0)，您的代码就 100% 符合 CI 要求。

3. 分步操作详解
如果您想单独排查某个环节，请按以下步骤操作：

第一步：代码格式化 (Format)
bash
gofmt -w .
作用：自动修正代码缩进和空格。
为什么跑：CI 第一步就是检查格式，不跑这个必挂。
第二步：静态分析 (Vet)
bash
go vet ./...
作用：Go 官方检查工具，发现明显的语法/逻辑错误（如 Printf 参数不对）。
第三步：Lint 检查 (最容易出错)
bash
/c/Users/10758/go/bin/golangci-lint.exe run ./...
常见问题处理：
包名警告 (avoid meaningless package names): 不要在配置里关掉规则，而是在文件头 package xxx 后加 //nolint:revive。
废弃函数 (deprecated): 替换为新函数（如 NewSimpleRouter -> 
NewSimpleShuffleRouter
）。
未检查错误 (errcheck): 显式处理错误，或者用 _ = func() 明确忽略。
第四步：单元测试 (Test)
bash
go test ./... -v -cover
作用：运行测试并看覆盖率。
第五步：构建验证 (Build)
bash
go build -o llmux.exe ./cmd/server
作用：确保 main 函数入口能正常编译。
4. 给 Agent 的“防乱改”指令
下次让 Agent 帮忙修 Bug 或跑 CI 时，请附带以下指令，可以有效防止它破坏配置：

⚠️ Agent 操作规范：

运行 CI 流程：请按顺序执行 Format, Vet, Lint, Test, Build。
Lint 修复原则：如果遇到 Lint 报错，只允许修改 Go 代码（如修复逻辑错误、变量名，或在必要时添加 //nolint 注释）。
配置红线：绝对禁止修改 
.golangci.yml
 配置文件。如果你认为报错是因为配置不合理，请先停止执行并向我报告，不要擅自更改规则。