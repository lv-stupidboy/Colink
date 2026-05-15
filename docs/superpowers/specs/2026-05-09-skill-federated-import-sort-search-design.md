---
name: skill-federated-import-sort-search
description: 联邦源扫描弹窗添加排序和搜索框
type: project
---

# 联邦源扫描弹窗排序和搜索功能设计

**日期**: 2026-05-09
**状态**: Approved
**版本**: 1.0

---

## 1. 背景

### 1.1 当前问题

联邦源扫描弹窗（`scanModalVisible`）存在以下问题：
- **无搜索框**：当联邦源返回大量 skill 时，用户无法快速筛选
- **无排序**：skill 列表按后端返回顺序展示，不便于查找

### 1.2 参考实现

冲突弹窗（`conflictModalVisible`）已实现类似功能：
- 搜索框：`Input.Search` 组件，支持按名称/路径搜索
- 排序：`.sort((a, b) => a.name.localeCompare(b.name))` 按名称字母排序

---

## 2. 设计方案

### 2.1 新增状态

```typescript
// 扫描弹窗搜索状态（已有 conflictSearchText，新增 scanSearchText）
const [scanSearchText, setScanSearchText] = useState('');
```

### 2.2 UI 变更

扫描弹窗顶部添加搜索框：

```
┌─────────────────────────────────────────────────────┐
│ 从联邦源导入 Skill                              [×] │
├─────────────────────────────────────────────────────┤
│ 联邦源：my-github-skills                            │
│ URL：https://github.com/owner/skills-repo           │
│                                                     │
│ [🔍 搜索 Skill 名称/路径/描述...        ]          │
│                                                     │
│ ┌──────────────────────────────────────────────────┤
│ │ [✓] java-coding-standards     来自: 我的联邦源   │
│ │     Java 代码规范检查技能                         │
│ │ ...                                             │
│ └───────────────────────────────────────────────────┤
│                                                     │
│                    [取消]      [确认导入]           │
└─────────────────────────────────────────────────────┘
```

### 2.3 功能逻辑

**搜索过滤**：
- 支持按 skill 名称、路径、描述搜索
- 使用 `.includes()` 进行模糊匹配

**排序**：
- 默认按名称字母排序（`a.name.localeCompare(b.name)`）
- 与冲突弹窗保持一致

---

## 3. 实施范围

### 3.1 修改文件

| 文件 | 变更 |
|------|------|
| `web/src/pages/SkillLibrary/index.tsx` | 新增 `scanSearchText` 状态、搜索框组件、排序逻辑 |

### 3.2 代码变更点

1. **新增状态**（约第133行后）：
   ```typescript
   const [scanSearchText, setScanSearchText] = useState('');
   ```

2. **添加搜索框**（约第1376行，`scanResult.registryUrl` 之后）：
   ```tsx
   <Input.Search
     placeholder="搜索 Skill 名称/路径/描述"
     allowClear
     style={{ marginBottom: 12 }}
     value={scanSearchText}
     onChange={(e) => setScanSearchText(e.target.value)}
   />
   ```

3. **修改 List 数据源**（约第1381行）：
   ```tsx
   dataSource={scanResult.skills
     .sort((a, b) => a.name.localeCompare(b.name))
     .filter(skill =>
       skill.name.includes(scanSearchText) ||
       (skill.path && skill.path.includes(scanSearchText)) ||
       (skill.description && skill.description.includes(scanSearchText))
     )}
   ```

4. **关闭弹窗时清理**（第1366行 `onCancel`）：
   ```tsx
   onCancel={() => {
     setScanModalVisible(false);
     setScanSearchText('');
   }}
   ```

---

## 4. 验收标准

1. 扫描弹窗顶部显示搜索框
2. 输入搜索词后列表实时过滤
3. 列表按名称字母排序
4. 关闭弹窗后搜索状态清空
5. 搜索/排序逻辑与冲突弹窗一致

---

## 修订历史

| 版本 | 日期 | 变更 |
|------|------|------|
| v1.0 | 2026-05-09 | 初版设计 |