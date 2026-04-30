# Launcher 智能体检测快捷操作实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将智能体检测从独立卡片改为快捷操作按钮入口，点击弹出 Modal 对话框

**Architecture:** 移除现有"智能体环境"独立卡片，在快捷操作区添加按钮入口，新增 Modal 展示检测结果

**Tech Stack:** React + Ant Design + Tauri IPC

---

## 文件结构

| 文件 | 操作 | 说明 |
|------|------|------|
| `installer-tauri/src/renderer/src/pages/LauncherDashboard.tsx` | 修改 | 移除卡片、添加按钮和 Modal |

---

### Task 1: 添加 import 和 state

**Files:**
- Modify: `installer-tauri/src/renderer/src/pages/LauncherDashboard.tsx:1-26`

- [ ] **Step 1: 添加 Modal 和 CheckCircleOutlined import**

修改 import 块（第14-22行），添加 Modal 和 CheckCircleOutlined：

```tsx
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
  Modal,  // 新增
} from 'antd';
import {
  PlayCircleOutlined,
  StopOutlined,
  SettingOutlined,
  FileTextOutlined,
  FolderOutlined,
  GlobalOutlined,
  ReloadOutlined,
  CheckCircleOutlined,  // 新增
} from '@ant-design/icons';
```

- [ ] **Step 2: 添加 agentModalVisible state**

在组件内 state 定义区（第78-85行之后），添加：

```tsx
const [agentModalVisible, setAgentModalVisible] = useState(false);
```

---

### Task 2: 移除智能体环境独立卡片

**Files:**
- Modify: `installer-tauri/src/renderer/src/pages/LauncherDashboard.tsx:306-369`

- [ ] **Step 1: 删除智能体环境卡片代码块**

删除第306-369行的整个"智能体环境"Card 代码块：

```tsx
// 删除以下整个代码块（306-369行）
{/* 智能体环境 */}
<Card
  title="智能体环境"
  size="small"
  style={{ marginBottom: 16 }}
  extra={
    <Button
      size="small"
      icon={<ReloadOutlined />}
      onClick={checkAgentDependencies}
      loading={loadingAgents}
    >
      刷新
    </Button>
  }
>
  ...  // 整个 Card 内容
</Card>
```

---

### Task 3: 在快捷操作区添加智能体检测按钮

**Files:**
- Modify: `installer-tauri/src/renderer/src/pages/LauncherDashboard.tsx:394-416`

- [ ] **Step 1: 在系统配置按钮后添加智能体检测按钮**

修改快捷操作区（394-416行），在"系统配置"按钮后添加：

```tsx
{/* 快捷操作 */}
<Card title="快捷操作" size="small" style={{ marginBottom: 16 }}>
  <Space wrap>
    <Button
      icon={<SettingOutlined />}
      onClick={handleOpenConfig}
    >
      系统配置
    </Button>
    <Button
      icon={<CheckCircleOutlined />}
      onClick={() => setAgentModalVisible(true)}
    >
      智能体检测
    </Button>
    <Button
      icon={<FileTextOutlined />}
      onClick={handleOpenLogs}
    >
      查看日志
    </Button>
    <Button
      icon={<FolderOutlined />}
      onClick={handleOpenDataDir}
    >
      数据目录
    </Button>
  </Space>
</Card>
```

---

### Task 4: 添加智能体检测 Modal

**Files:**
- Modify: `installer-tauri/src/renderer/src/pages/LauncherDashboard.tsx:424` (在安装信息 Card 之后)

- [ ] **Step 1: 在安装信息 Card 后添加 Modal 组件**

在安装信息 Card 后（第424行之后，return 闭合标签之前）添加：

```tsx
{/* 智能体检测 Modal */}
<Modal
  title="智能体环境"
  open={agentModalVisible}
  onCancel={() => setAgentModalVisible(false)}
  footer={[
    <Button
      key="refresh"
      icon={<ReloadOutlined />}
      onClick={checkAgentDependencies}
      loading={loadingAgents}
    >
      刷新检测
    </Button>,
    <Button
      key="close"
      onClick={() => setAgentModalVisible(false)}
    >
      关闭
    </Button>,
  ]}
  width={480}
>
  {loadingAgents ? (
    <div style={{ textAlign: 'center', padding: 24 }}>
      <Spin />
    </div>
  ) : (
    <>
      <div
        style={{
          background: 'var(--bg-container, #fafafa)',
          borderRadius: 8,
          padding: 12,
        }}
      >
        {agentDependencies.map((dep) => (
          <div
            key={dep.key}
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '8px 12px',
              borderBottom:
                dep.key === agentDependencies[agentDependencies.length - 1]?.key
                  ? 'none'
                  : '1px solid var(--border-color, #e8e8e8)',
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
        description="Colink 平台支持 Claude CLI 和 OpenCode 等智能体，安装后即可使用对应的 Agent 类型。如未安装，请访问官方文档获取安装指南。"
      />
    </>
  )}
</Modal>
```

---

### Task 5: 测试验证

- [ ] **Step 1: 启动 Launcher 开发服务器**

```bash
cd D:/workspace/isdp/installer-tauri
pnpm dev:launcher
```

- [ ] **Step 2: 验证功能**

检查项：
1. Launcher 页面不再显示"智能体环境"独立卡片
2. 快捷操作区显示"智能体检测"按钮（位于"系统配置"后）
3. 点击按钮弹出 Modal 对话框
4. Modal 正确展示 Claude CLI 和 OpenCode 检测状态
5. 点击"刷新检测"按钮可重新检测
6. 点击"关闭"按钮可关闭 Modal
7. 深色模式下样式正常

---

### Task 6: 提交代码

- [ ] **Step 1: 提交变更**

```bash
cd D:/workspace/isdp
git add installer-tauri/src/renderer/src/pages/LauncherDashboard.tsx
git add docs/superpowers/specs/2026-04-30-launcher-agent-detection-design.md
git commit -m "feat: 将智能体检测改为快捷操作按钮入口

- 移除 LauncherDashboard 中的智能体环境独立卡片
- 在快捷操作区添加智能体检测按钮
- 点击按钮弹出 Modal 对话框展示检测结果
- 支持刷新检测和关闭操作"
```

---

## 自审清单

| 检查项 | 状态 |
|--------|------|
| 规格覆盖：移除独立卡片 | Task 2 |
| 规格覆盖：添加按钮 | Task 3 |
| 规格覆盖：添加 Modal | Task 4 |
| 规格覆盖：添加 state | Task 1 |
| 无占位符 | ✓ |
| 类型一致 | ✓ |
| 文件路径精确 | ✓ |