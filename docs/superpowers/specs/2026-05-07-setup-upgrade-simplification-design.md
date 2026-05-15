# Setup 安装器升级流程简化设计

## 需求概述

简化 Setup 安装器的升级流程，减少不必要的检测步骤和显示内容：

1. 升级时取消智能体检测页面
2. 所有安装步骤取消磁盘空间检测
3. 取消展示"所需空间：约 500 MB"文字

## 现有流程分析

### 新安装流程（4步）
```
步骤1: 欢迎 → 步骤2: 目录选择 → 步骤3: 智能体检测 → 步骤4: 系统配置
```

### 升级流程（当前3步）
```
步骤1: 欢迎 → 步骤3: 智能体检测 → 步骤4: 系统配置
```
（已跳过目录选择）

## 修改后流程

### 新安装流程（保持4步，但简化目录选择页面）
```
步骤1: 欢迎 → 步骤2: 目录选择(简化) → 步骤3: 智能体检测 → 步骤4: 系统配置
```

### 升级流程（减少为2步）
```
步骤1: 欢迎 → 步骤4: 系统配置
```

## 详细修改

### 1. App.tsx - 升级流程调整

**文件**: `installer/src/renderer/src/App.tsx`

#### 修改点1：升级步骤标签（第28行）
```tsx
// 当前
const UPGRADE_STEP_LABELS = ['欢迎', '智能体检测', '系统配置']

// 修改后
const UPGRADE_STEP_LABELS = ['欢迎', '系统配置']
```

#### 修改点2：handleNext 升级跳转逻辑（第155-156行）
```tsx
// 当前
if (isUpgrade) {
  if (currentStep === 1) nextStep = 3  // Welcome -> DependencyCheck
  else if (currentStep === 3) nextStep = 4  // DependencyCheck -> SystemConfig
}

// 修改后
if (isUpgrade) {
  if (currentStep === 1) nextStep = 4  // Welcome -> SystemConfig
}
```

#### 修改点3：handlePrev 升级跳转逻辑（第166-167行）
```tsx
// 当前
if (isUpgrade) {
  if (currentStep === 4) prevStep = 3  // SystemConfig -> DependencyCheck
  else if (currentStep === 3) prevStep = 1  // DependencyCheck -> Welcome
}

// 修改后
if (isUpgrade) {
  if (currentStep === 4) prevStep = 1  // SystemConfig -> Welcome
}
```

#### 修改点4：stepIndex 计算逻辑（第322-324行）
```tsx
// 当前
const stepIndex = isUpgrade
  ? (currentStep === 1 ? 0 : currentStep === 3 ? 1 : 2)
  : currentStep - 1

// 修改后
const stepIndex = isUpgrade
  ? (currentStep === 1 ? 0 : 1)
  : currentStep - 1
```

#### 修改点5：底部按钮条件判断（第346-360行）
```tsx
// 当前：步骤3显示"下一步"按钮（智能体检测）
{currentStep === 3 && (
  <Button type="primary" onClick={handleNext}>下一步</Button>
)}

// 修改后：升级时步骤3不存在，需要调整判断条件
// 升级流程只有步骤1和步骤4，无需步骤3的按钮逻辑
```

### 2. DirectorySelect.tsx - 移除磁盘空间检测

**文件**: `installer/src/renderer/src/pages/DirectorySelect.tsx`

#### 移除内容

| 行号 | 内容 | 说明 |
|------|------|------|
| 15 | `const [freeSpace, setFreeSpace] = useState<number>(0)` | 状态定义 |
| 18-45 | `checkDiskSpace` 函数 | 磁盘检测逻辑 |
| 47-49 | `useEffect` 调用 | 自动检测触发 |
| 58-62 | `formatSize` 函数 | 格式化显示 |
| 88-90 | 空间显示区域 | "所需空间：约 500 MB" 和可用空间 |

#### 保留内容

- 目录输入框和浏览按钮（第69-86行）
- 提示文字："目录不存在时将自动创建"（第83-85行）
- `onValidationChange?.(true)` 简化验证回调

#### 修改后代码结构
```tsx
export default function DirectorySelect({ config, onConfigUpdate, onValidationChange }: DirectorySelectProps) {
  useEffect(() => {
    // 始终允许继续，无需验证磁盘空间
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

## 影响范围

### 不受影响的功能
- 新安装流程的智能体检测页面（步骤3）保持不变
- `DependencyCheck.tsx` 文件保留，仅升级流程跳过
- 磁盘空间检测 API `getDiskSpace` 保留（preload/index.ts）
- 其他安装步骤（欢迎、系统配置）无变化

### 受影响的功能
- 升级用户不再看到智能体检测结果
- 新安装用户不再看到磁盘空间信息
- 步骤指示器在升级时显示2步而非3步

## 测试要点

1. **升级流程测试**
   - 已安装状态下启动 Setup，验证流程：欢迎 → 系统配置
   - 步骤指示器显示 "步骤1/2" → "步骤2/2"
   - 上一步按钮在系统配置页面直接返回欢迎页

2. **新安装流程测试**
   - 未安装状态下启动 Setup，验证流程：欢迎 → 目录选择 → 智能体检测 → 系统配置
   - 目录选择页面无磁盘空间显示
   - "下一步"按钮始终可用（无空间验证）

3. **边界情况**
   - 目录选择页面输入无效路径时，不阻止用户继续
   - 升级时点击"开始升级"后直接进入系统配置

## 风险评估

| 风险 | 等级 | 说明 |
|------|------|------|
| 用户选择磁盘空间不足的安装目录 | 低 | 安装时会失败并提示错误，用户可重新选择 |
| 升级用户不知道智能体安装状态 | 低 | 智能体检测仅为信息展示，不影响安装 |
| 步骤跳转逻辑错误 | 中 | 需充分测试升级和新安装两种流程 |

## 实施计划概要

1. 修改 App.tsx 升级流程逻辑（约10处修改点）
2. 简化 DirectorySelect.tsx 移除磁盘检测（删除约40行代码）
3. 手动测试升级和新安装两种流程
4. 验证步骤指示器显示正确