---
name: Launcher智能体检测快捷操作
description: 将智能体检测功能从独立卡片改为快捷操作按钮入口，点击后弹出检测结果对话框
type: project
---

# Launcher 智能体检测快捷操作设计规格

## 概述

将 installer-tauri LauncherDashboard 中的"智能体环境"独立卡片改为快捷操作按钮入口，点击后弹出 Modal 对话框显示检测结果。

## 背景

- **Why**: 精简 Launcher 页面布局，将智能体检测功能整合到快捷操作区域
- **Reference**: 现有"智能体环境"卡片实现可复用至 Modal 内容
- **Consistency**: 按钮风格与快捷操作区现有按钮保持一致

## 设计变更

### 移除内容

删除 `LauncherDashboard.tsx` 中的"智能体环境"独立卡片（原306-369行）。

### 新增内容

在快捷操作区域添加"智能体检测"按钮，与"系统配置"同级。

**按钮顺序**：
```
[系统配置] [智能体检测] [查看日志] [数据目录]
```

**按钮图标**：使用 `CheckCircleOutlined` 或类似图标。

## UI 设计

### 快捷操作按钮

```tsx
<Button
  icon={<CheckCircleOutlined />}
  onClick={() => setAgentModalVisible(true)}
>
  智能体检测
</Button>
```

### Modal 对话框

```tsx
<Modal
  title="智能体环境"
  open={agentModalVisible}
  onCancel={() => setAgentModalVisible(false)}
  footer={[
    <Button key="refresh" icon={<ReloadOutlined />} onClick={checkAgentDependencies} loading={loadingAgents}>
      刷新检测
    </Button>,
    <Button key="close" onClick={() => setAgentModalVisible(false)}>
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
        description="Colink 平台支持 Claude CLI 和 OpenCode 等智能体，安装后即可使用对应的 Agent 类型。如未安装，请访问官方文档获取安装指南。"
      />
    </>
  )}
</Modal>
```

## 交互流程

1. **页面加载**：自动调用 `checkAgentDependencies()`，缓存检测结果
2. **点击按钮**：打开 Modal，展示缓存数据（即时响应）
3. **刷新检测**：点击 Modal 内"刷新检测"按钮，重新调用 API 并更新展示
4. **关闭 Modal**：点击"关闭"或 Modal 外部区域

## 数据缓存策略

- 页面加载时已执行检测（`useEffect` 中的 `checkAgentDependencies`）
- Modal 打开时直接展示缓存数据 `agentDependencies`
- 避免每次打开 Modal 都触发 API 调用

## 深色模式适配

使用 CSS 变量（遵循项目规范）：
- `var(--bg-container)` - 结果列表背景
- `var(--text-primary)` - 文本颜色
- `var(--border-color)` - 分隔线

Modal 本身由 Ant Design 控制，已支持深色模式。

## 新增 State

```tsx
const [agentModalVisible, setAgentModalVisible] = useState(false);
```

## 文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `src/renderer/src/pages/LauncherDashboard.tsx` | 修改 | 移除独立卡片，添加按钮和 Modal |

## 验收标准

- [ ] 快捷操作区域显示"智能体检测"按钮
- [ ] 点击按钮弹出 Modal 对话框
- [ ] Modal 正确展示 Claude CLI 和 OpenCode 检测状态
- [ ] 已安装显示版本号，未安装显示"未安装"
- [ ] "刷新检测"按钮可重新检测
- [ ] "关闭"按钮可关闭 Modal
- [ ] 深色模式下样式正常

## 约束

- 需使用 CSS 变量适配深色模式
- 新增 import: `Modal` from antd, `CheckCircleOutlined` from icons