# SQL 扫描错误修复记录

**日期**: 2026-05-07
**修复人**: SuperPowers全栈开发工程师

## 问题

两个接口返回 SQL 扫描错误：
- `/api/v1/agents/{id}/commands` - "sql: expected 5 destination arguments in Scan, not 6"
- `/api/v1/agents/{id}/subagents` - "sql: expected 5 destination arguments in Scan, not 6"

## 原因

`scanCommand` 和 `scanSubagent` 函数扫描 **6 个字段**（含 `supported_agents`），但 SQL 查询只选择 **5 列**（缺少 `supported_agents`）。

## 修复

| 文件 | 修改 |
|------|------|
| `internal/repo/agent_subagent_binding.go:60` | SQL 查询添加 `s.supported_agents` 列 |
| `internal/repo/agent_command_binding.go:61` | SQL 查询添加 `c.supported_agents` 列 |

## 验证

- Go 编译通过：`go build ./internal/repo/...`