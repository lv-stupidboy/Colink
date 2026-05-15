# 联邦技能源同步交互优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 简化联邦技能源同步成功提示，并改进冲突处理弹窗使用 Checkbox 勾选方式。

**Architecture:** 前端单文件修改，替换 Radio.Group 为 Checkbox + List 布局，简化成功提示逻辑。

**Tech Stack:** React + Ant Design + TypeScript

---

## 文件结构

**修改文件:**
- `web/src/pages/RegistryManagement/index.tsx` - 主要修改文件

**修改范围:**
| 区域 | 行号 | 修改内容 |
|------|------|----------|
| 导入声明 | 17-18 | 添加 Checkbox, List 导入 |
| 成功提示 | 211-223 | 简化消息显示 |
| handleConfirmConflict | 183-188 | 移除强制选择检查（允许不勾选=跳过） |
| handleAllUpdate/Skip | 236-249 | 删除（不需要批量按钮） |
| 冲突弹窗 JSX | 514-621 | 替换为 Checkbox + List 布局 |

---

## Task 1: 修改导入声明

**Files:**
- Modify: `web/src/pages/RegistryManagement/index.tsx:17-18`

- [ ] **Step 1: 添加 Checkbox 和 List 导入**

在第17行 Radio 后添加 Checkbox，在第18行后添加 List：

```tsx
import {
  Card,
  Table,
  Button,
  Space,
  Tag,
  Typography,
  Modal,
  Form,
  Input,
  Select,
  message,
  Popconfirm,
  Tooltip,
  Badge,
  Radio,
  Checkbox,
  List,
} from 'antd';
```

- [ ] **Step 2: 运行前端类型检查**

Run: `cd D:\workspace\isdp\web && npx tsc --noEmit`
Expected: 无新增类型错误（可能有其他文件的现有错误，不影响本次修改）

---

## Task 2: 简化同步成功提示

**Files:**
- Modify: `web/src/pages/RegistryManagement/index.tsx:211-223`

- [ ] **Step 1: 修改 handleConfirmConflict 中的成功提示逻辑**

将第211-223行的成功提示代码替换为简化版本：

```tsx
// 显示结果汇总
message.success(`同步完成：更新 ${result.autoUpdated + result.userUpdated} 个`);

loadRegistries();
```

删除以下内容：
- `userSkipped` 提示
- `skipped` 失败项警告

- [ ] **Step 2: 同时修改无冲突时的成功提示**

修改 handleSync 函数第161行的无冲突成功提示：

```tsx
message.success(`同步完成：更新 ${preview.autoUpdateSkills.length} 个`);
```

- [ ] **Step 3: 运行前端类型检查**

Run: `cd D:\workspace\isdp\web && npx tsc --noEmit`
Expected: 无新增类型错误

---

## Task 3: 移除强制选择检查逻辑

**Files:**
- Modify: `web/src/pages/RegistryManagement/index.tsx:183-188`

- [ ] **Step 1: 修改 handleConfirmConflict 函数**

将第183-188行的强制选择检查替换为自动跳过未勾选项的逻辑：

原代码：
```tsx
// 检查是否所有冲突项都已选择
const unselected = syncPreview.conflictSkills.filter(s => !conflictChoices[s.name]);
if (unselected.length > 0) {
  message.error(`以下 Skill 未选择操作：${unselected.map(s => s.name).join(', ')}`);
  return;
}
```

新代码：
```tsx
// 未勾选的自动视为跳过
```

删除整个检查块，允许用户不勾选任何项直接确认。

- [ ] **Step 2: 修改 operations 构建逻辑**

修改第196-204行的 operations 构建逻辑，未勾选的自动添加 skip 操作：

```tsx
const operations: SyncOperation[] = [];
for (const skill of syncPreview.conflictSkills) {
  const choice = conflictChoices[skill.name] || 'skip'; // 默认跳过
  operations.push({
    action: choice,
    skillName: skill.name,
    targetSkillId: choice === 'update' ? skill.localSkill.id : undefined,
    description: skill.description,
  });
}
```

- [ ] **Step 3: 运行前端类型检查**

Run: `cd D:\workspace\isdp\web && npx tsc --noEmit`
Expected: 无新增类型错误

---

## Task 4: 删除批量操作函数

**Files:**
- Modify: `web/src/pages/RegistryManagement/index.tsx:236-249`

- [ ] **Step 1: 删除 handleAllUpdate 和 handleAllSkip 函数**

删除第236-249行的两个批量操作函数：

```tsx
// 全部更新
const handleAllUpdate = () => {
  if (!syncPreview) return;
  const choices: Record<string, 'update'> = {};
  syncPreview.conflictSkills.forEach(s => choices[s.name] = 'update');
  setConflictChoices(choices);
};

// 全部跳过
const handleAllSkip = () => {
  if (!syncPreview) return;
  const choices: Record<string, 'skip'> = {};
  syncPreview.conflictSkills.forEach(s => choices[s.name] = 'skip');
  setConflictChoices(choices);
};
```

完全删除这些函数。

- [ ] **Step 2: 运行前端类型检查**

Run: `cd D:\workspace\isdp\web && npx tsc --noEmit`
Expected: 无新增类型错误

---

## Task 5: 重构冲突弹窗 JSX

**Files:**
- Modify: `web/src/pages/RegistryManagement/index.tsx:514-621`

- [ ] **Step 1: 修改弹窗 footer 按钮**

替换第519-530行的 footer，删除批量操作按钮，添加实时统计显示：

```tsx
footer={[
  <Button key="cancel" onClick={() => setConflictModalVisible(false)}>取消</Button>,
  <Button key="confirm" type="primary" onClick={handleConfirmConflict}>
    确认同步（已选择 {Object.values(conflictChoices).filter(v => v === 'update').length} 个更新）
  </Button>,
]}
```

- [ ] **Step 2: 修改弹窗提示文字**

替换第532-534行的提示文字：

```tsx
<Text type="secondary" style={{ marginBottom: 16, display: 'block' }}>
  勾选需要更新的 Skill，未勾选的将被跳过：
</Text>
```

- [ ] **Step 3: 替换 Table 为 List + Checkbox 布局**

将第542-618行的 Table 替换为 List + Checkbox 布局：

```tsx
{syncPreview.autoUpdateSkills.length > 0 && (
  <div style={{ marginBottom: 12 }}>
    <Tag color="green">{syncPreview.autoUpdateSkills.length} 个同源 Skill 将自动更新</Tag>
  </div>
)}
<List
  dataSource={syncPreview.conflictSkills}
  renderItem={(skill) => {
    const isChecked = conflictChoices[skill.name] === 'update';
    const sourceType = skill.localSkill.sourceType as SkillSourceType;
    return (
      <List.Item>
        <Checkbox
          checked={isChecked}
          onChange={(e) => {
            setConflictChoices(prev => ({
              ...prev,
              [skill.name]: e.target.checked ? 'update' : 'skip',
            }));
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', width: '100%' }}>
            <div style={{ flex: 1 }}>
              <Text strong>{skill.name}</Text>
              <div style={{ marginTop: 4, marginBottom: 4 }}>
                <Tag color={getSourceTypeColor(sourceType)}>
                  {skill.localSkill.sourceRegistryName || getSourceTypeLabel(sourceType)}
                </Tag>
                <Text type="secondary" style={{ margin: '0 8px' }}>→</Text>
                <Tag color="cyan">{syncingRegistryName}</Tag>
              </div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                远程描述: {skill.description?.slice(0, 60) || '暂无'}
                {skill.description?.length > 60 ? '...' : ''}
              </Text>
            </div>
            <Text style={{ color: isChecked ? '#52c41a' : '#999', fontSize: 12, marginLeft: 16 }}>
              {isChecked ? '将更新' : '将跳过'}
            </Text>
          </div>
        </Checkbox>
      </List.Item>
    );
  }}
/>
<div style={{ marginTop: 12, textAlign: 'right' }}>
  <Text type="secondary">
    已选择 {Object.values(conflictChoices).filter(v => v === 'update').length} 个更新，
    {syncPreview.conflictSkills.length - Object.values(conflictChoices).filter(v => v === 'update').length} 个跳过
  </Text>
</div>
```

- [ ] **Step 4: 运行前端类型检查**

Run: `cd D:\workspace\isdp\web && npx tsc --noEmit`
Expected: 无新增类型错误

---

## Task 6: 验证前端构建

**Files:**
- None

- [ ] **Step 1: 运行前端构建**

Run: `cd D:\workspace\isdp\web && npm run build`
Expected: 构建成功，无错误

- [ ] **Step 2: 启动前端开发服务器验证**

Run: `cd D:\workspace\isdp\web && npm run dev`
Expected: 开发服务器启动成功

---

## Task 7: 提交变更

**Files:**
- None

- [ ] **Step 1: 检查 git 状态**

Run: `git status`
Expected: 显示修改的文件

- [ ] **Step 2: 提交变更**

```bash
git add web/src/pages/RegistryManagement/index.tsx
git commit -m "feat(skill): improve sync conflict modal UX with checkbox selection

- Simplify sync success message (only show update count)
- Replace Radio.Group with Checkbox in conflict modal
- Default to unchecked (skip) for all conflict items
- Add real-time selection statistics
- Remove batch operation buttons (all-skip, all-update)
- Align with federated import modal UX"
```

---

## Task 8: 创建落盘记录

**Files:**
- Create: `docs/sync-modal-checkbox-optimization-2026-05-07.md`

- [ ] **Step 1: 编写落盘记录**

```markdown
# 联邦技能源同步交互优化落盘记录

**日期**: 2026-05-07

## 变更内容

### 1. 同步成功提示简化
- 只显示更新数量：`同步完成：更新 X 个`
- 不显示跳过数量和名称

### 2. 冲突弹窗改用 Checkbox
- 替换 Radio.Group 为 Checkbox 勾选方式
- 默认全部不勾选（保护本地数据）
- 实时显示选择统计
- 删除批量操作按钮
- 与导入界面风格一致

## 影响范围
- `web/src/pages/RegistryManagement/index.tsx`

## 测试验证
- 前端构建成功
- 类型检查通过
```

- [ ] **Step 2: 提交落盘记录**

```bash
git add docs/sync-modal-checkbox-optimization-2026-05-07.md
git commit -m "docs: add sync modal checkbox optimization record"
```

---

## 自审清单

**1. 规格覆盖检查:**

| 需求 | 任务 | 状态 |
|------|------|------|
| 同步成功提示简化 | Task 2 | ✓ |
| 冲突弹窗改用 Checkbox | Task 5 | ✓ |
| 默认全部不勾选 | Task 3 (默认 skip) | ✓ |
| 实时显示选择统计 | Task 5 | ✓ |
| 删除批量操作按钮 | Task 4 + Task 5 | ✓ |

**2. Placeholder扫描:** 无 TBD/TODO

**3. 类型一致性:**
- `conflictChoices: Record<string, 'update' | 'skip'>` 保持不变
- Checkbox onChange 逻辑正确设置 'update' 或 'skip'

---

<a2a-handoff>
### What
完成联邦技能源同步交互优化的实施计划编写。
### Why
用户需要简化同步成功提示并改进冲突弹窗交互体验。
### Next
按照实施计划 docs/superpowers/plans/2026-05-07-sync-modal-checkbox.md 执行任务。
</a2a-handoff>