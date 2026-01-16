# Bug 修复日志

本文档记录了 LLMux 项目在近期深度审计中发现并修复的核心 Bug。

## 1. APIKey 并发安全修复 (Data Race)
- **问题描述**：在 `internal/auth/memory.go` 中，`MemoryStore` 的查询方法返回的是 `APIKey` 结构体的浅拷贝。由于该结构体包含 `ModelSpend` (Map) 等字段，多个协程并发读写该 Map 时会触发竞态导致程序崩溃。
- **修复方案**：
  - 为 `APIKey`、`Team`、`User` 等核心结构体实现了 `Clone()` 深度拷贝方法。
  - 强制 `MemoryStore` 在返回或存储对象前进行深度拷贝，确保内存数据的隔离性。
- **影响模块**：`internal/auth`
- **风险等级**：🚨 高

## 2. 流式传输 Context 泄露修复
- **问题描述**：`internal/streaming/forwarder.go` 中的 `Forward` 方法创建了带取消功能的子 Context，但在方法正常结束或异常退出时未确保调用 `cancel()`。
- **修复方案**：在 `Forward()` 方法入口处添加 `defer f.cancel()`，确保请求生命周期结束后立即释放资源。
- **影响模块**：`internal/streaming`
- **风险等级**：⚠️ 中

## 3. 全局 Panic 恢复机制
- **问题描述**：服务器中间件栈中缺少 Panic 捕获逻辑。如果某个处理程序发生崩溃，会导致请求直接挂断且无标准的 JSON 错误响应。
- **修复方案**：在 `cmd/server/middleware.go` 中新增了 `recoveryMiddleware`，统一捕获 Panic、记录日志并返回标准的 500 JSON 错误。
- **影响模块**：`cmd/server`
- **风险等级**：⚠️ 中

## 4. 编译与兼容性修复
- **问题描述**：由于之前的代码重构，`cmd/server/management_authz_test.go` 中的函数签名与实际实现不一致，导致编译失败。
- **修复方案**：同步更新了测试用例中的函数签名。
- **影响模块**：`cmd/server`
- **风险等级**：✅ 低

---
**验证状态**：所有修复均已通过开启 `-race` 标志的回归测试。
