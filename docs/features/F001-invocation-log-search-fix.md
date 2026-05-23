---
id: F001
name: 调用日志Modal搜索框修复
status: in_progress
created: 2026-05-23
owner: Colink架构&开发工程师
source: internal
evolved_from: null
blocked_by: null
related: []
---

## Why

全屏 Modal 里的搜索框不可用，用户无法快速筛选调用记录。

## What

修复 AgentInvocationLogPanel 全屏 Modal 中搜索框的交互问题，确保用户可以正常输入、搜索和过滤调用日志记录。

## Acceptance Criteria

- [ ] AC-A1: 搜索框可以正常输入文本
- [ ] AC-A2: 输入文本后列表实时过滤
- [ ] AC-A3: 清空搜索框后恢复完整列表
- [ ] AC-A4: 状态过滤（Segmented）与搜索组合使用正常
- [ ] AC-A5: 搜索框的 allowClear 功能正常

## 需求点 Checklist

| 需求点 | 来源 | 优先级 | 状态 |
|-------|------|--------|------|
| 搜索框可以输入 | 用户反馈"不可用" | P0 | pending |
| 搜索过滤生效 | 功能正常需求 | P0 | pending |
| 状态组合过滤 | 现有功能应保持 | P1 | pending |

## Dependencies

- 无外部依赖

## Root Cause Analysis（初步诊断）

经代码审查发现：
1. **代码逻辑正常**：`onChange={(e) => setSearchQuery(e.target.value)}` 正确绑定
2. **CSS样式无阻止**：未发现 `pointer-events: none` 等阻止交互的规则
3. **组件结构正常**：Input 在正确的位置，无遮挡

**疑似原因**：
- 可能是 Ant Design Modal 的遮罩层或事件处理问题
- 可能是 Input 组件在某些场景下的渲染问题
- 需要实际运行验证具体表现

## Open Questions

- [ ] 搜索框"不可用"的具体表现是什么？（无法输入？输入后无过滤？其他？）
- [ ] 是所有情况都不可用，还是特定场景下？

## Investigation Plan

1. 启动前端开发服务器
2. 打开对话页面，触发调用日志
3. 打开全屏 Modal，尝试使用搜索框
4. 根据实际表现定位问题根因

## Timeline

- Phase 1: 问题诊断 + 修复（预计 0.5 天）

## Links

- 相关组件：`isdp/web/src/components/thread/StatusPanel/AgentInvocationLogPanel.tsx`
- 相关样式：`isdp/web/src/components/thread/StatusPanel/StatusPanel.css`