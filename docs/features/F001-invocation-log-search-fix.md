---
id: F001
name: 调用日志Modal搜索框修复
status: in_review
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

## Root Cause Analysis（诊断结果）

经代码审查和CSS分析，发现根本原因：

**CSS选择器错误**：Ant Design Input 组件的 `className` 属性会直接添加到 `.ant-input-affix-wrapper` 元素上，而不是嵌套结构。

错误的选择器：
```css
.sidebar-search .ant-input-affix-wrapper  /* 嵌套选择器，不会匹配 */
```

正确的选择器：
```css
.sidebar-search.ant-input-affix-wrapper  /* 同元素多类选择器 */
```

**DOM结构**：
```html
<span class="ant-input-affix-wrapper sidebar-search">
  <span class="ant-input-prefix">🔍</span>
  <input class="ant-input" />
</span>
```

`.sidebar-search` 和 `.ant-input-affix-wrapper` 是同一元素的class组合，不是父子嵌套关系。

## Open Questions

- ✅ ~~搜索框"不可用"的具体表现是什么？~~ → 已诊断：CSS选择器错误导致样式未应用
- ✅ ~~是所有情况都不可用，还是特定场景下？~~ → 全场景，因为CSS选择器根本不匹配

## Fix Applied

**修改文件**：`isdp/web/src/components/thread/StatusPanel/StatusPanel.css`

**修改内容**：
- 将嵌套选择器 `.sidebar-search .ant-input-affix-wrapper` 改为同元素选择器 `.sidebar-search.ant-input-affix-wrapper`
- 添加 `pointer-events: auto` 确保交互
- 删除无效的嵌套结构注释

## Investigation Plan

1. 启动前端开发服务器
2. 打开对话页面，触发调用日志
3. 打开全屏 Modal，尝试使用搜索框
4. 根据实际表现定位问题根因

## Timeline

- Phase 1: 问题诊断 + CSS修复（✅ 2026-05-23 完成）
- Phase 2: 用户验证（待进行）

## Links

- 相关组件：`isdp/web/src/components/thread/StatusPanel/AgentInvocationLogPanel.tsx`
- 相关样式：`isdp/web/src/components/thread/StatusPanel/StatusPanel.css`