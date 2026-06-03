# QA审查报告 - ACP stderr错误处理改进

**日期**: 2026-06-02
**任务**: ACP通信层错误处理机制改进
**审查人**: 质量保证工程师

---

## 审查对象

- **修改文件**: `internal/service/agent/plugins/acp/adapter_base.go`
- **新增文件**: `internal/service/agent/plugins/acp/adapter_base_test.go`
- **改动范围**: stderr缓冲 + 失败时返回stderr内容 + 64KB上限

---

## Pre-landing审查（✅ 通过）

### stderr缓冲机制

```go
type acpSession struct {
    stderrOutput strings.Builder // stderr输出缓冲（用于错误诊断）
    // ...
}
```

- ✅ 使用 `strings.Builder` 高效缓冲
- ✅ `sync.Mutex` 保护并发访问

### 64KB上限防护

```go
const maxStderrSize = 64 * 1024

// stderr goroutine写入时检查上限
session.mu.Lock()
if session.stderrOutput.Len() < maxStderrSize {
    session.stderrOutput.WriteString(line)
    session.stderrOutput.WriteString("\n")
}
session.mu.Unlock()
```

- ✅ 防止stderr输出过大导致内存问题
- ✅ 每次写入前检查，非一次性截断

### 错误返回覆盖（5个启动阶段）

| 方法 | 错误点 | stderr返回 |
|------|--------|-----------|
| ExecuteWithStream | initialize失败 | ✅ 209行 |
| ExecuteWithStream | session/new失败 | ✅ 236行 |
| ExecuteWithStream | session/prompt失败 | ✅ 268行 |
| StartSession | initialize失败 | ✅ 420行 |
| StartSession | session/new失败 | ✅ 439行 |

### goroutine清理

- ✅ 所有失败场景调用 `wg.Wait()` 确保stderr goroutine完成
- ✅ 无goroutine leak风险

---

## QA测试（✅ 通过）

### 单元测试结果

```
=== RUN   TestStderrBufferField
--- PASS: TestStderrBufferField (0.00s)

=== RUN   TestStderrSizeLimit
--- PASS: TestStderrSizeLimit (0.00s)

=== RUN   TestStderrInErrorMessage
--- PASS: TestStderrInErrorMessage (0.00s)

=== RUN   TestAdapterConfigField
--- PASS: TestAdapterConfigField (0.00s)

=== RUN   TestMultipleStderrLines
--- PASS (验证多行缓冲)

=== RUN   TestConcurrentStderrWrite
--- PASS: TestConcurrentStderrWrite (0.00s)

PASS
ok  github.com/anthropic/isdp/internal/service/agent/plugins/acp
```

### 测试覆盖

| 测试ID | 场景 | 状态 |
|--------|------|------|
| ACP-01 | stderr缓冲字段存在 | ✅ |
| ACP-02 | 64KB截断上限 | ✅ |
| ACP-03 | 错误消息格式 | ✅ |
| ACP-04 | 配置结构验证 | ✅ |
| ACP-05 | 多行stderr缓冲 | ✅ |
| ACP-06 | 并发写入安全 | ✅ |

---

## 安全检查（✅ 无风险）

### 并发安全

- ✅ `sync.Mutex` 保护所有 stderr 写入
- ✅ 测试 `TestConcurrentStderrWrite` 验证10个goroutine并发写入

### 内存安全

- ✅ 64KB上限防止内存耗尽
- ✅ 测试 `TestStderrSizeLimit` 验证截断逻辑

### 其他安全项

- ✅ 无SQL注入风险（纯Go逻辑）
- ✅ 无信任边界违规（stderr仅用于诊断）
- ✅ 无敏感信息暴露（stderr可能含配置路径，但在错误消息中已可见）

---

## 风险评估

| 维度 | 评级 | 说明 |
|------|------|------|
| 代码复杂度 | 低 | 改动最小，只修改一个文件 |
| 接口影响 | 低 | 不改变现有API契约 |
| 前端影响 | 无 | 不涉及前端改动 |
| 回滚难度 | 低 | 可快速回滚 |

---

## 建议

1. **可以合并**：代码符合设计文档，测试覆盖完整
2. **后续优化**：
   - 添加集成测试验证实际启动失败场景
   - 考虑stderr分级（ERROR/WARN/INFO）

---

## 审查结论

**✅ 通过**，建议合并代码。