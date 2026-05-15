# Setup 升级流程简化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 简化 Setup 安装器升级流程，取消智能体检测和磁盘空间检测

**Architecture:** 修改前端 React 组件，调整步骤跳转逻辑和移除磁盘检测功能

**Tech Stack:** React + TypeScript + Ant Design + Electron

---

## Files

| 文件 | 操作 | 说明 |
|------|------|------|
| `installer/src/renderer/src/App.tsx` | Modify | 升级流程逻辑调整 |
| `installer/src/renderer/src/pages/DirectorySelect.tsx` | Modify | 移除磁盘空间检测 |

---

### Task 1: 修改 App.tsx 升级流程逻辑

**Files:**
- Modify: `installer/src/renderer/src/App.tsx`

**修改点1：升级步骤标签（第28行）**

- [ ] **Step 1: 修改 UPGRADE_STEP_LABELS**

将第28行从：
```tsx
const UPGRADE_STEP_LABELS = ['欢迎', '智能体检测', '系统配置']
```
改为：
```tsx
const UPGRADE_STEP_LABELS = ['欢迎', '系统配置']
```

---

**修改点2：handleNext 升级跳转逻辑（第153-157行）**

- [ ] **Step 2: 修改 handleNext 函数升级逻辑**

将第153-157行从：
```tsx
    if (isUpgrade) {
      // 升级跳过目录选择(步骤2)：1->3->4
      if (currentStep === 1) nextStep = 3  // Welcome -> DependencyCheck
      else if (currentStep === 3) nextStep = 4  // DependencyCheck -> SystemConfig
    }
```
改为：
```tsx
    if (isUpgrade) {
      // 升级跳过目录选择和智能体检测：1->4
      if (currentStep === 1) nextStep = 4  // Welcome -> SystemConfig
    }
```

---

**修改点3：handlePrev 升级跳转逻辑（第164-168行）**

- [ ] **Step 3: 修改 handlePrev 函数升级逻辑**

将第164-168行从：
```tsx
    if (isUpgrade) {
      // 升级跳过目录选择(步骤2)：4->3->1
      if (currentStep === 4) prevStep = 3  // SystemConfig -> DependencyCheck
      else if (currentStep === 3) prevStep = 1  // DependencyCheck -> Welcome
    }
```
改为：
```tsx
    if (isUpgrade) {
      // 升级跳过目录选择和智能体检测：4->1
      if (currentStep === 4) prevStep = 1  // SystemConfig -> Welcome
    }
```

---

**修改点4：stepIndex 计算逻辑（第322-324行）**

- [ ] **Step 4: 修改 stepIndex 计算逻辑**

将第321-324行注释和代码从：
```tsx
  // 升级跳过目录选择(步骤2)：1->0(欢迎), 3->1(智能体检测), 4->2(系统配置)
  const stepIndex = isUpgrade
    ? (currentStep === 1 ? 0 : currentStep === 3 ? 1 : 2)
    : currentStep - 1
```
改为：
```tsx
  // 升级跳过目录选择和智能体检测：1->0(欢迎), 4->1(系统配置)
  const stepIndex = isUpgrade
    ? (currentStep === 1 ? 0 : 1)
    : currentStep - 1
```

---

**修改点5：底部按钮显示逻辑（第346-360行）**

- [ ] **Step 5: 调整底部按钮条件判断**

升级流程只有步骤1和步骤4，需要调整"上一步"按钮的条件判断。

将第346行从：
```tsx
          {currentStep > 1 && !(isUpgrade && currentStep === 3) && !(isUpgrade && currentStep === 2) && (
```
改为：
```tsx
          {currentStep > 1 && !(isUpgrade && currentStep === 4) && (
```

说明：升级时步骤4（系统配置）是最后一步，不需要显示"上一步"按钮（因为用户可以直接从欢迎开始升级）。

实际上，升级流程应该显示"上一步"按钮让用户从系统配置回到欢迎页。修正：

改为：
```tsx
          {currentStep > 1 && (
```

因为升级流程只有两个步骤（1和4），currentStep=4时应该显示"上一步"按钮返回步骤1。

---

- [ ] **Step 6: 提交 App.tsx 修改**

```bash
git add installer/src/renderer/src/App.tsx
git commit -m "refactor(installer): simplify upgrade flow to 2 steps

- Remove dependency check from upgrade flow
- Change upgrade steps: Welcome -> SystemConfig (skip DependencyCheck)
- Update handleNext/handlePrev navigation logic
- Fix stepIndex calculation for 2-step upgrade"
```

---

### Task 2: 简化 DirectorySelect.tsx 移除磁盘检测

**Files:**
- Modify: `installer/src/renderer/src/pages/DirectorySelect.tsx`

- [ ] **Step 1: 重写 DirectorySelect 组件，移除磁盘检测**

将整个文件从第14行开始替换为简化版本：

```tsx
export default function DirectorySelect({ config, onConfigUpdate, onValidationChange }: DirectorySelectProps) {
  // 始终允许继续，无需验证磁盘空间
  useEffect(() => {
    onValidationChange?.(true)
  }, [onValidationChange])

  const handleBrowse = async () => {
    const result = await window.electronAPI.selectDirectory()
    if (result) {
      onConfigUpdate({ installDir: result })
    }
  }

  return (
    <div style={{ flex: 1 }}>
      <h2 style={{ fontSize: 22, marginBottom: 8, color: '#333' }}>选择安装位置</h2>
      <p style={{ color: '#666', marginBottom: 30 }}>请选择 Colink 的安装目录</p>

      <div style={{ marginBottom: 20 }}>
        <label style={{ display: 'block', fontSize: 13, color: '#666', marginBottom: 8 }}>
          安装目录
        </label>
        <div style={{ display: 'flex', gap: 12 }}>
          <Input
            value={config.installDir}
            onChange={(e) => onConfigUpdate({ installDir: e.target.value })}
            style={{ flex: 1 }}
          />
          <Button icon={<FolderOpenOutlined />} onClick={handleBrowse}>
            浏览...
          </Button>
        </div>
        <div style={{ color: '#999', fontSize: 12, marginTop: 8 }}>
          目录不存在时将自动创建
        </div>
      </div>
    </div>
  )
}
```

移除的内容：
- `const [freeSpace, setFreeSpace] = useState<number>(0)` 状态
- `checkDiskSpace` 函数（第18-45行）
- 原有 `useEffect` 调用磁盘检测（第47-49行）
- `formatSize` 函数（第58-62行）
- 底部空间显示区域（第88-91行）

---

- [ ] **Step 2: 提交 DirectorySelect.tsx 修改**

```bash
git add installer/src/renderer/src/pages/DirectorySelect.tsx
git commit -m "refactor(installer): remove disk space detection from DirectorySelect

- Remove checkDiskSpace function and freeSpace state
- Remove 'Required space: ~500 MB' and 'Available space' display
- Simplify validation to always allow continue"
```

---

### Task 3: 手动测试验证

**Files:**
- Test: 升级流程和新安装流程

- [ ] **Step 1: 启动开发环境测试升级流程**

```bash
cd installer
npm run dev
```

测试升级流程：
1. 模拟已安装状态（或使用已安装环境）
2. 启动 Setup，应显示选择操作页面
3. 点击"升级"，应直接进入欢迎页面
4. 点击"开始升级"，应直接进入系统配置页面（跳过智能体检测）
5. 步骤指示器应显示"步骤1/2"
6. 点击"上一步"，应返回欢迎页面

---

- [ ] **Step 2: 测试新安装流程**

测试新安装流程：
1. 模拟未安装状态
2. 启动 Setup，应显示欢迎页面
3. 点击"开始安装"，进入目录选择页面
4. 目录选择页面应无磁盘空间显示
5. 点击"下一步"，进入智能体检测页面
6. 点击"下一步"，进入系统配置页面
7. 步骤指示器应显示完整4步

---

- [ ] **Step 3: 构建生产版本验证**

```bash
cd installer
npm run build
```

运行构建后的安装器，重复上述测试流程。

---

## Self-Review Checklist

**1. Spec Coverage:**
- ✅ 升级时取消智能体检测 - Task 1 修改升级流程跳过步骤3
- ✅ 取消磁盘空间检测 - Task 2 移除 checkDiskSpace 函数
- ✅ 取消"所需空间：约 500 MB"文字 - Task 2 移除底部显示

**2. Placeholder Scan:**
- ✅ 无 TBD/TODO 占位符
- ✅ 所有步骤包含完整代码
- ✅ 所有提交命令完整

**3. Type Consistency:**
- ✅ InstallConfig 和 InstalledVersion 类型未改变
- ✅ onValidationChange 回调签名保持一致
- ✅ 页面组件引用名称无变化