# 修复报告：Embedding API 类型安全增强（Bifrost 对齐版）

## 📋 执行摘要

本次修复对 LLMux 的 Embedding API 进行了**重大架构改进**，对齐了 Bifrost 的类型安全设计模式，彻底解决了之前代码审计中发现的所有问题。

| 问题                                                      | 状态           | 修复方式                              |
| --------------------------------------------------------- | -------------- | ------------------------------------- |
| **P2 [中] - Missing configuration for embedding support** | ✅ 已修复       | `SupportsEmbedding` 配置字段          |
| **P3 [低] - Loose typing for EmbeddingRequest.Input**     | ✅ **彻底重构** | 专用 `EmbeddingInput` 类型            |
| **[新发现] - `[]interface{}` 问题**                       | ✅ 已修复       | 自定义 `UnmarshalJSON`                |
| **[新发现] - Validate() 未自动调用**                      | ✅ 已修复       | 在 `BuildEmbeddingRequest` 中强制验证 |

---

## 🏗️ 架构改进：对齐 Bifrost 设计

### 核心变更：引入 `EmbeddingInput` 类型

**之前（存在问题）：**
```go
type EmbeddingRequest struct {
    Model string      `json:"model"`
    Input interface{} `json:"input"`  // ← 类型不安全，JSON 反序列化为 []interface{}
    // ...
}
```

**之后（Bifrost 风格）：**
```go
type EmbeddingInput struct {
    Text       *string   `json:"-"` // 单个字符串
    Texts      []string  `json:"-"` // 字符串数组
    Tokens     []int     `json:"-"` // Token ID 数组
    TokensList [][]int   `json:"-"` // 多个 Token ID 数组（批量）
}

type EmbeddingRequest struct {
    Model string          `json:"model"`
    Input *EmbeddingInput `json:"input"`  // ← 类型安全，自动类型推断
    // ...
}
```

### 自定义 JSON 序列化/反序列化

```go
// UnmarshalJSON - 自动类型推断
func (e *EmbeddingInput) UnmarshalJSON(data []byte) error {
    // Reject null
    if string(data) == "null" {
        return fmt.Errorf("input cannot be null")
    }

    // Try string -> []string -> []int -> [][]int
    var s string
    if err := json.Unmarshal(data, &s); err == nil {
        e.Text = &s
        return nil
    }
    // ... 依次尝试其他类型
}

// MarshalJSON - 强制 one-of 约束
func (e *EmbeddingInput) MarshalJSON() ([]byte, error) {
    // 确保恰好设置一个字段
    // ...
}
```

---

## 🔧 具体修复

### 1. 新增 `EmbeddingInput` 类型 (`pkg/types/embedding.go`)

| 方法            | 功能                                              |
| --------------- | ------------------------------------------------- |
| `UnmarshalJSON` | 自动类型推断：string → []string → []int → [][]int |
| `MarshalJSON`   | 强制 one-of 约束，确保恰好一个字段被设置          |
| `Validate()`    | 验证输入非空、数组元素有效                        |
| `IsEmpty()`     | 检查是否有输入                                    |

### 2. 辅助构造函数

```go
types.NewEmbeddingInputFromString("Hello, world!")
types.NewEmbeddingInputFromStrings([]string{"Hello", "World"})
types.NewEmbeddingInputFromTokens([]int{1234, 5678})
```

### 3. 自动验证集成

在所有 `BuildEmbeddingRequest` 方法中添加了自动验证：

```go
func (p *Provider) BuildEmbeddingRequest(ctx context.Context, req *types.EmbeddingRequest) (*http.Request, error) {
    // Validate input before sending to API
    if err := req.Validate(); err != nil {
        return nil, fmt.Errorf("invalid embedding request: %w", err)
    }
    // ...
}
```

---

## ✅ 测试验证

### 单元测试覆盖

```bash
$ go test ./pkg/types/... -run "Embedding" -v
=== RUN   TestEmbeddingInput_UnmarshalJSON_String
--- PASS
=== RUN   TestEmbeddingInput_UnmarshalJSON_StringArray
--- PASS
=== RUN   TestEmbeddingInput_UnmarshalJSON_IntArray
--- PASS
=== RUN   TestEmbeddingInput_UnmarshalJSON_IntArrayList
--- PASS
=== RUN   TestEmbeddingInput_UnmarshalJSON_Invalid
--- PASS
=== RUN   TestEmbeddingInput_MarshalJSON_String
--- PASS
=== RUN   TestEmbeddingInput_MarshalJSON_StringArray
--- PASS
=== RUN   TestEmbeddingInput_Validate_*
--- PASS (all)
=== RUN   TestEmbeddingRequest_*
--- PASS (all)
PASS
```

### 关键测试场景

| 测试场景                           | 之前                    | 之后               |
| ---------------------------------- | ----------------------- | ------------------ |
| JSON `["hello", "world"]` 反序列化 | `[]interface{}` ❌       | `[]string` ✅       |
| `null` 输入                        | 静默通过 ❌              | 返回错误 ✅         |
| 无效类型 (int, map, bool)          | 运行时可能失败          | 立即返回明确错误 ✅ |
| 空字符串/空数组                    | 取决于手动调用 Validate | 自动验证 ✅         |

### 编译验证

```bash
$ go build ./...
✓ 编译成功，无错误

$ go test ./...
PASS
```

---

## 📊 对比分析

| 维度                     | 之前                    | 之后              | Bifrost           |
| ------------------------ | ----------------------- | ----------------- | ----------------- |
| **Input 类型**           | `interface{}`           | `*EmbeddingInput` | `*EmbeddingInput` |
| **类型推断**             | 无（用户负责）          | 自动              | 自动              |
| **验证时机**             | 手动调用                | 自动              | 自动              |
| **`[]interface{}` 问题** | 存在                    | 已消除            | 不存在            |
| **支持的输入类型**       | string, []string, []int | + [][]int         | + [][]int         |

---

## 📝 修改文件清单

### 核心重构
- `pkg/types/embedding.go` - 新增 `EmbeddingInput` 类型及方法
- `pkg/types/embedding_test.go` - 全面更新的测试覆盖

### Provider 更新
- `providers/openai/embedding.go` - 添加自动验证
- `providers/openai/embedding_test.go` - 更新测试使用新类型
- `providers/openailike/embedding.go` - 添加自动验证

---

## 🚀 使用示例

### SDK 模式（Go 代码直接构造）

```go
// 字符串输入
req := &types.EmbeddingRequest{
    Model: "text-embedding-3-small",
    Input: types.NewEmbeddingInputFromString("Hello, world!"),
}

// 字符串数组输入
req := &types.EmbeddingRequest{
    Model: "text-embedding-3-small",
    Input: types.NewEmbeddingInputFromStrings([]string{"Hello", "World"}),
}

// Token ID 输入
req := &types.EmbeddingRequest{
    Model: "text-embedding-3-small",
    Input: types.NewEmbeddingInputFromTokens([]int{1234, 5678}),
}
```

### Gateway 模式（HTTP JSON 请求）

```json
// 自动识别为 []string
{"model": "text-embedding-3-small", "input": ["hello", "world"]}

// 自动识别为 string
{"model": "text-embedding-3-small", "input": "hello world"}

// 自动识别为 []int
{"model": "text-embedding-3-small", "input": [1234, 5678]}
```

---

## ✨ 总结

本次修复通过对齐 Bifrost 的设计模式，实现了：

1. **彻底消除 `[]interface{}` 问题** - 使用专用类型和自定义 JSON 反序列化
2. **自动类型推断** - 无需用户干预，JSON 自动解析为正确的 Go 类型
3. **自动验证** - 在 API 调用前强制验证，无需手动调用
4. **类型安全的 API** - 编译时类型检查，清晰的构造函数
5. **完整的 OpenAI API 兼容** - 支持所有文档规定的输入格式

这使得 LLMux 的 Embedding API 达到了与 Bifrost 同等的设计水准，适合作为高性能网关框架使用。
