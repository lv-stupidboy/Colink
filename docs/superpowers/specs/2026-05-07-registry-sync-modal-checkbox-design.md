---
name: registry-sync-modal-checkbox-design
description: 联邦技能源同步冲突弹窗改用 Checkbox 勾选方式
type: project
---

# 联邦技能源同步交互优化设计

## 背景

用户反馈联邦技能源页面同步功能有两个优化需求：
1. 同步成功提示过于冗长（显示跳过数量和名称）
2. 冲突处理弹窗交互不够直观（每行 Radio 选择）

## 需求

### 1. 同步成功提示简化

**当前行为**：
```
同步完成：自动更新 X 个，更新 Y 个，跳过 Z 个
跳过 N 个失败项：skill-a, skill-b, ...
```

**优化后**：
```
同步完成：更新 X 个
```

不显示跳过数量和名称。

### 2. 冲突弹窗交互改进

**当前设计**：
- 表格每行使用 Radio.Group 选择"跳过"或"更新"
- 默认无选择，用户必须逐行操作
- 批量操作按钮："全部跳过"、"全部更新"

**目标设计**（参考联邦源导入界面）：
- 使用 Checkbox 勾选需要更新的 Skill
- 未勾选 = 跳过
- 默认全部不勾选（保护本地数据）
- 实时显示选择统计（已选择 X 个更新，Y 个跳过）

## 设计详情

### 冲突弹窗布局

```
标题: 同步冲突处理

提示文字: 勾选需要更新的 Skill，未勾选的将被跳过

[自动更新提示] X 个同源 Skill 将自动更新

┌────────────────────────────────────────────┐
│ ☐ skill-a                                   │
│   [平台] → [联邦源A]                         │
│   远程描述: 处理A类任务...                    │
│                                      将跳过  │
├────────────────────────────────────────────┤
│ ☐ skill-b                                   │
│   [联邦源B] → [联邦源A]                      │
│   远程描述: 处理B类任务...                    │
│                                      将跳过  │
├────────────────────────────────────────────┤
│ ☐ skill-c                                   │
│   [个人] → [联邦源A]                         │
│   远程描述: 处理C类任务...                    │
│                                      将跳过  │
└────────────────────────────────────────────┘

已选择 0 个更新，3 个跳过

[取消]                        [确认同步]
```

### 默认勾选策略

**策略**: 默认全部不勾选

**Why**: 保护本地数据，用户主动选择要更新的项，避免意外覆盖。

**How to apply**: 初始化 `conflictChoices` 为空对象，所有 Checkbox 默认 unchecked。

### 成功提示简化

**当前代码** (`RegistryManagement/index.tsx` 211-222行)：
```tsx
let successMsg = `同步完成：自动更新 ${result.autoUpdated} 个`;
if (result.userUpdated > 0) {
  successMsg += `，更新 ${result.userUpdated} 个`;
}
if (result.userSkipped > 0) {
  successMsg += `，跳过 ${result.userSkipped} 个`;
}
message.success(successMsg);

if (result.skipped.length > 0) {
  message.warning(`跳过 ${result.skipped.length} 个失败项：${result.skipped.map(s => s.name).join(', ')}`);
}
```

**优化后**：
```tsx
let successMsg = `同步完成：更新 ${result.autoUpdated + result.userUpdated} 个`;
message.success(successMsg);
// 不显示跳过信息
```

## 技术实现

### 前端修改范围

**文件**: `web/src/pages/RegistryManagement/index.tsx`

**修改点**:

1. **成功提示简化** (211-222行)
   - 删除 `userSkipped` 提示
   - 删除 `skipped` 失败项警告
   - 合并 `autoUpdated + userUpdated` 显示总更新数

2. **冲突弹窗改用 Checkbox** (514-621行)
   - 将 Radio.Group 改为 Checkbox
   - 删除"全部跳过"、"全部更新"批量按钮
   - 添加实时统计显示（已选择 X 个更新，Y 个跳过）

3. **数据结构调整**
   - `conflictChoices` 从 `Record<string, 'update' | 'skip'>` 改为 `Record<string, boolean>`（勾选状态）
   - 或保持原结构，Checkbox checked = choices[name] === 'update'

### 参考实现

联邦源导入的选择界面 (`SkillLibrary/index.tsx` 1339-1390行)：
- 使用 `<Checkbox>` + `<List.Item>` 布局
- 实时统计选中数量
- 底部显示"确认导入（已选择 X 个）"

## 影响范围

| 文件 | 修改类型 | 影响范围 |
|------|----------|----------|
| `RegistryManagement/index.tsx` | UI + 逻辑 | 冲突弹窗、成功提示 |
| `types/index.ts` | 可能不需要 | 如果保持原 choices 结构 |

## 成功标准

1. 同步成功提示简洁，只显示更新数量
2. 冲突弹窗使用 Checkbox 勾选方式
3. 默认全部不勾选
4. 实时显示选择统计
5. 与导入界面风格一致