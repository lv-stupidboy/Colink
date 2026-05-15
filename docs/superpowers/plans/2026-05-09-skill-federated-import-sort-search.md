# 联邦源扫描弹窗排序和搜索功能实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为联邦源扫描弹窗添加搜索框和排序功能，复用冲突弹窗的已有实现模式。

**Architecture:** 单文件修改，新增一个 useState 状态，修改 Modal 的 onCancel 和 List 的 dataSource。

**Tech Stack:** React + Ant Design (Input.Search, List)

---

## 文件结构

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `web/src/pages/SkillLibrary/index.tsx` | Modify | 新增状态、搜索框、排序逻辑 |

---

### Task 1: 新增搜索状态

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx:134`（在 `conflictSearchText` 状态后）

- [ ] **Step 1: 添加 scanSearchText 状态**

在 `web/src/pages/SkillLibrary/index.tsx` 第 134 行后，添加新状态：

```typescript
  // 冲突弹窗搜索状态
  const [conflictSearchText, setConflictSearchText] = useState('');
  // 扫描弹窗搜索状态
  const [scanSearchText, setScanSearchText] = useState('');
```

- [ ] **Step 2: 验证修改**

检查文件状态：
```bash
git diff web/src/pages/SkillLibrary/index.tsx
```
Expected: 显示新增的 `scanSearchText` 状态行

---

### Task 2: 修改 onCancel 清理搜索状态

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx:1365`

- [ ] **Step 1: 修改扫描弹窗 onCancel**

将第 1365 行的单行 `onCancel` 改为多行清理：

```tsx
      {/* 联邦源扫描弹窗 */}
      <Modal
        title="从联邦源导入 Skill"
        open={scanModalVisible}
        onCancel={() => {
          setScanModalVisible(false);
          setScanSearchText('');
        }}
        width={600}
```

- [ ] **Step 2: 验证修改**

```bash
git diff web/src/pages/SkillLibrary/index.tsx
```
Expected: 显示 onCancel 从单行变为多行，包含 `setScanSearchText('')`

---

### Task 3: 添加搜索框组件

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx:1380`（在 `registryUrl` 显示后）

- [ ] **Step 1: 在联邦源信息后添加搜索框**

在第 1380 行 `<Text type="secondary">{scanResult.registryUrl}</Text>` 后，添加搜索框：

```tsx
            <div style={{ marginBottom: 12 }}>
              <Text strong>联邦源：</Text>{scanResult.registryName}
              <br />
              <Text type="secondary">{scanResult.registryUrl}</Text>
            </div>
            <Input.Search
              placeholder="搜索 Skill 名称/路径/描述"
              allowClear
              style={{ marginBottom: 12 }}
              value={scanSearchText}
              onChange={(e) => setScanSearchText(e.target.value)}
            />
            <List
```

- [ ] **Step 2: 验证修改**

```bash
git diff web/src/pages/SkillLibrary/index.tsx
```
Expected: 显示新增的 Input.Search 组件

---

### Task 4: 添加排序和过滤逻辑

**Files:**
- Modify: `web/src/pages/SkillLibrary/index.tsx:1388`（List 的 dataSource）

- [ ] **Step 1: 修改 List dataSource**

将第 1388 行的 `dataSource={scanResult.skills}` 改为带排序和过滤：

```tsx
            <List
              dataSource={scanResult.skills
                .sort((a, b) => a.name.localeCompare(b.name))
                .filter(skill =>
                  skill.name.includes(scanSearchText) ||
                  (skill.path && skill.path.includes(scanSearchText)) ||
                  (skill.description && skill.description.includes(scanSearchText))
                )}
              renderItem={(skill) => {
```

- [ ] **Step 2: 验证修改**

```bash
git diff web/src/pages/SkillLibrary/index.tsx
```
Expected: 显示 dataSource 从直接使用 `scanResult.skills` 变为带 `.sort().filter()` 链

---

### Task 5: 验证功能并提交

**Files:**
- Test: 手动测试前端功能

- [ ] **Step 1: 启动前端开发服务器**

```bash
cd web && npm run dev
```
Expected: 前端在 http://localhost:26306 启动

- [ ] **Step 2: 手动测试功能**

测试步骤：
1. 打开 Skill 管理页面
2. 点击「新建 Skill」
3. 选择来源「联邦」
4. 选择「联邦源下载」
5. 选择一个联邦源，点击「导入」
6. 验证：
   - 扫描弹窗顶部有搜索框
   - skill 列表按名称字母排序
   - 输入搜索词后列表实时过滤
   - 关闭弹窗后再次打开，搜索框为空

- [ ] **Step 3: 提交代码**

```bash
git add web/src/pages/SkillLibrary/index.tsx
git commit -m "feat(skill): add search and sort to federated scan modal"
```

---

## 自审清单

| 检查项 | 状态 |
|--------|------|
| 覆盖规格所有要求 | ✓ 搜索框 + 排序 + 状态清理 |
| 无占位符 | ✓ 所有代码完整 |
| 类型一致 | ✓ 使用已有类型 RemoteSkill |
| 与冲突弹窗一致 | ✓ 相同的 sort/filter 模式 |

---

## 验收标准

1. ✓ 扫描弹窗顶部显示搜索框
2. ✓ 输入搜索词后列表实时过滤
3. ✓ 列表按名称字母排序
4. ✓ 关闭弹窗后搜索状态清空
5. ✓ 搜索/排序逻辑与冲突弹窗一致