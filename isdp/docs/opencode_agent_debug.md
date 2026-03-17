# OpenCode Agent 调试修复说明

## 问题描述
OpenCode Agent 执行时只能看到"执行完成"，看不到实际的输出信息。

## 修复内容

### 1. `extractTextFromJSON` 方法增强 (`open_code_adapter.go:187-293`)

**问题原因：**
- 原始代码在 JSON 解析失败或没有可提取文本时返回空字符串
- 导致输出丢失，用户看不到任何内容

**修复方案：**
- 增加对 JSON 数组标记的过滤（`[`、`]`、`,`）
- 增强事件类型处理，支持更多字段：
  - `assistant` 类型增加 `Text` 字段支持
  - `result` 类型同时支持 `Result` 和 `Text` 字段
  - `error` 类型增加空值检查
  - 新增 `user` 类型支持
- 增加 fallback 逻辑：尝试从原始 JSON 中提取 `text`/`content`/`result` 字段
- **最终保底**：如果解析了 JSON 但没有可提取的文本，返回原始行而不是空字符串

### 2. `ExecuteWithStream` 方法增强 (`open_code_adapter.go:79-162`)

- 增加 stderr 异步捕获（第 121-137 行）
- 添加调试日志：显示前 5 行和每 10 行的摘要（第 145-148 行）
- 统计并输出总行数（第 161 行）

### 3. `Execute` 方法增强 (`open_code_adapter.go:46-76`)

- 添加执行前后的日志输出

### 4. 新增辅助函数 (`open_code_adapter.go:165-170`)

- `truncateString`：截断长字符串用于日志显示

## 测试验证

### 测试结果
```
=== ExecuteWithStream（流式执行）===
[OpenCode] line 1: {"type":"error",...}
[CHUNK] {"type":"error",...}  <- 可以正常看到输出
[OpenCode] total lines: 1

=== Execute（普通执行）===
[OpenCode] executing with model: xxx
[OpenCode] execution completed, output length: 0  <- 日志正常
```

### 日志输出示例
```
[OpenCode] executing with model: alibaba-cn/qwen3-coder-plus
[OpenCode] line 1: {"type":"error","timestamp":...}
[OpenCode] total lines: 1
[OpenCode] execution completed, output length: 1109
```

## 配置说明

### 环境变量
```bash
# OpenCode API 配置
export OPENCODE_API_URL="https://dashscope.aliyuncs.com/compatible-mode/v1"
export OPENCODE_API_KEY="your-api-key-here"
export OPENCODE_MODEL="alibaba-cn/qwen3-coder-plus"
```

### BaseAgent 配置
通过 API 创建 BaseAgent:
```json
{
  "name": "OpenCode Agent",
  "type": "open_code",
  "api_url": "https://dashscope.aliyuncs.com/compatible-mode/v1",
  "api_token": "your-api-key",
  "default_model": "alibaba-cn/qwen3-coder-plus",
  "cli_path": "opencode",
  "max_tokens": 4096,
  "timeout_minutes": 30
}
```

## 可用模型列表
运行以下命令查看可用模型：
```bash
opencode models
```

## 调试日志说明

修复后的日志输出：
- `[OpenCode] executing with model: xxx` - 执行开始
- `[OpenCode] line N: ...` - 接收到的输出行（前 5 行 + 每 10 行）
- `[OpenCode stderr] ...` - 错误输出
- `[OpenCode] total lines: N` - 总行数
- `[OpenCode] execution completed, output length: N` - 执行完成

## 下一步

如果有有效的 API Key，可以测试完整的执行流程。
