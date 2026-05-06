# Launcher 智能体检测功能实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 installer-tauri 的 LauncherDashboard 中新增智能体环境检测卡片，显示 Claude CLI 和 OpenCode 的安装状态。

**Architecture:** 修改 Rust 后端 `check_all_dependencies()` 只检测智能体，前端新增 Card 组件调用现有 API，使用 CSS 变量适配深色模式。

**Tech Stack:** Tauri 2 (Rust) + React + Ant Design + TypeScript

---

## File Structure

| 文件 | 操作 | 说明 |
|------|------|------|
| `installer-tauri/src-tauri/src/services/dependency.rs` | 修改 | 第77-80行，`check_all_dependencies()` 只检测智能体 |
| `installer-tauri/src/renderer/src/pages/LauncherDashboard.tsx` | 修改 | 新增"智能体环境"Card |
| `installer-tauri/src/lib/api/types.ts` | 无修改 | `DependencyInfo` 已定义 |
| `installer-tauri/src/lib/api/dependency.ts` | 无修改 | `checkAll()` 已实现 |

---

### Task 1: 修改 Rust 后端检测范围

**Files:**
- Modify: `installer-tauri/src-tauri/src/services/dependency.rs:77-80`

- [ ] **Step 1: 修改 `check_all_dependencies()` 函数**

修改第 77-80 行，只检测智能体（移除 node 和 git）：

```rust
/// Check all dependencies (only agents)
pub fn check_all_dependencies() -> Vec<DependencyInfo> {
    let keys = ["claude", "opencode"];
    keys.iter().map(|k| check_dependency(k)).collect()
}
```

- [ ] **Step 2: 验证 Rust 代码编译通过**

Run: `cd installer-tauri && cargo check --manifest-path src-tauri/Cargo.toml`
Expected: 编译成功，无错误

- [ ] **Step 3: Commit Rust backend change**

```bash
cd installer-tauri
git add src-tauri/src/services/dependency.rs
git commit -m "feat: check_all_dependencies only checks agents (claude, opencode)"
```

---

### Task 2: 新增智能体环境 Card

**Files:**
- Modify: `installer-tauri/src/renderer/src/pages/LauncherDashboard.tsx`

- [ ] **Step 1: 导入 dependencyApi 和 Spin 组件**

在第 21 行添加导入：

```typescript
import { serviceApi, launcherApi, modeApi, installApi, dependencyApi } from '../../../lib/api';
```

在第 12 行 `Alert` 后添加 `Spin`：

```typescript
import {
  Button,
  Card,
  Typography,
  Space,
  Table,
  Tag,
  Divider,
  Alert,
  message,
  Spin,
} from 'antd';
```

- [ ] **Step 2: 导入 RefreshOutlined 图标**

在第 20 行 `GlobalOutlined` 后添加：

```typescript
import {
  PlayCircleOutlined,
  StopOutlined,
  SettingOutlined,
  FileTextOutlined,
  FolderOutlined,
  GlobalOutlined,
  RefreshOutlined,
} from '@ant-design/icons';
```

- [ ] **Step 3: 导入 DependencyInfo 类型**

在第 22 行添加类型导入：

```typescript
import type { RunningAgentInstance, DependencyInfo } from '../../../lib/api/types';
```

- [ ] **Step 4: 添加智能体检测状态变量**

在第 81 行 `installDir` 状态后添加（约第 81 行后）：

```typescript
const [agentDependencies, setAgentDependencies] = useState<DependencyInfo[]>([]);
const [loadingAgents, setLoadingAgents] = useState(true);
```

- [ ] **Step 5: 添加智能体检测函数**

在 `handleOpenConfig` 函数后添加（约第 192 行后）：

```typescript
const checkAgentDependencies = async () => {
  setLoadingAgents(true);
  try {
    const deps = await dependencyApi.checkAll();
    setAgentDependencies(deps);
  } catch (err) {
    console.error('Failed to check agent dependencies:', err);
    setAgentDependencies([]);
  } finally {
    setLoadingAgents(false);
  }
};
```

- [ ] **Step 6: 在 useEffect 中调用智能体检测**

修改第 83-88 行的 useEffect，添加智能体检测调用：

```typescript
useEffect(() => {
  checkStatus();
  loadInstallDir();
  checkAgentDependencies();
  const interval = setInterval(checkStatus, 5000);
  return () => clearInterval(interval);
}, []);
```

- [ ] **Step 7: 在服务状态 Card 后添加智能体环境 Card**

在第 287 行（服务状态 Card 结束）后添加智能体环境 Card：

```tsx
{/* 智能体环境 */}
<Card 
  title="智能体环境" 
  size="small" 
  style={{ marginBottom: 16 }}
  extra={
    <Button 
      size="small" 
      icon={<RefreshOutlined />} 
      onClick={checkAgentDependencies}
      loading={loadingAgents}
    >
      刷新
    </Button>
  }
>
  {loadingAgents ? (
    <div style={{ textAlign: 'center', padding: 20 }}>
      <Spin size="small" />
    </div>
  ) : (
    <>
      <div style={{
        background: 'var(--bg-container, #fafafa)',
        borderRadius: 8,
        padding: 12,
      }}>
        {agentDependencies.map(dep => (
          <div
            key={dep.key}
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '8px 12px',
              borderBottom: dep.key === agentDependencies[agentDependencies.length - 1]?.key ? 'none' : '1px solid var(--border-color, #e8e8e8)',
            }}
          >
            <span style={{ fontWeight: 500 }}>{dep.name}</span>
            <Tag color={dep.installed ? 'success' : 'warning'}>
              {dep.installed ? `已安装 ${dep.version || ''}` : '未安装'}
            </Tag>
          </div>
        ))}
      </div>
      <Alert
        type="info"
        showIcon
        style={{ marginTop: 12 }}
        message="智能体说明"
        description={
          <div style={{ fontSize: 12 }}>
            <p style={{ marginBottom: 4 }}>
              Colink 平台支持 Claude CLI 和 OpenCode 等智能体，安装后即可使用对应的 Agent 类型。
            </p>
            <p style={{ marginBottom: 0 }}>
              如未安装，请访问官方文档获取安装指南。
            </p>
          </div>
        }
      />
    </>
  )}
</Card>
```

- [ ] **Step 8: 验证 TypeScript 类型检查**

Run: `cd installer-tauri && pnpm typecheck`
Expected: 类型检查通过，无错误

- [ ] **Step 9: Commit frontend change**

```bash
cd installer-tauri
git add src/renderer/src/pages/LauncherDashboard.tsx
git commit -m "feat: add agent environment card to LauncherDashboard"
```

---

### Task 3: 测试验证

**Files:**
- 无文件修改，仅测试验证

- [ ] **Step 1: 启动 Launcher 开发模式**

Run: `cd installer-tauri && pnpm dev:launcher`
Expected: Launcher 窗口正常打开

- [ ] **Step 2: 验证智能体环境卡片显示**

手动验证：
1. Launcher 页面显示"智能体环境"Card
2. Claude CLI 和 OpenCode 检测状态正确显示
3. 已安装时显示版本号
4. 未安装时显示"未安装"标签
5. 点击刷新按钮可重新检测

- [ ] **Step 3: 验证深色模式样式（如支持）**

手动验证：
1. 切换深色模式（如有）
2. 智能体环境 Card 样式正常显示
3. CSS 变量正常生效

---

## Self-Review

**1. Spec Coverage:**
- ✓ 检测范围：只检测 Claude CLI 和 OpenCode - Task 1
- ✓ UI 实现：新增智能体环境 Card - Task 2
- ✓ 自动检测：useEffect 调用 checkAgentDependencies - Task 2 Step 6
- ✓ 刷新按钮：Card extra 包含刷新按钮 - Task 2 Step 7
- ✓ 状态展示：Tag 显示已安装/未安装 + 版本 - Task 2 Step 7
- ✓ 说明提示：Alert 组件显示智能体说明 - Task 2 Step 7
- ✓ 深色模式：使用 CSS 变量 - Task 2 Step 7
- ✓ 测试验证：Task 3

**2. Placeholder Scan:**
- ✓ 无 TBD、TODO、implement later
- ✓ 无 "Add appropriate error handling"
- ✓ 无 "Write tests for the above"
- ✓ 所有代码步骤包含完整代码

**3. Type Consistency:**
- ✓ `DependencyInfo` 类型定义在 `types.ts`，前端导入使用一致
- ✓ `dependencyApi.checkAll()` 返回 `Promise<DependencyInfo[]>`
- ✓ Rust `DependencyInfo` struct 与 TypeScript interface 字段匹配（key, name, installed, version）

---

## Acceptance Criteria

- [ ] Rust 后端 `check_all_dependencies()` 只检测 claude 和 opencode
- [ ] Launcher 页面显示"智能体环境"Card
- [ ] 检测状态正确显示（已安装/未安装 + 版本）
- [ ] 刷新按钮可重新检测
- [ ] 深色模式下样式正常（CSS 变量生效）
- [ ] TypeScript 类型检查通过
- [ ] Cargo 编译通过

---

## Notes

- 此计划基于现有代码结构，遵循项目规范（CSS 变量、camelCase）
- 无需修改 API 层和类型定义，复用现有实现
- 智能体检测与 installer 模块保持一致（只检测 Claude CLI 和 OpenCode）