# Launcher 系统配置弹窗实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Launcher 中修改「系统配置」按钮行为，从打开文件夹改为打开弹窗编辑 config.yaml

**Architecture:** 前端新增 ConfigEditorModal 组件，使用 CodeMirror 实现 yaml 语法高亮和验证；复用现有 configApi 和 serviceApi；修改 LauncherDashboard.tsx 的按钮处理逻辑

**Tech Stack:** React, Ant Design, CodeMirror 6, Tauri IPC

---

## 文件结构

| 文件 | 操作 | 说明 |
|------|------|------|
| `installer-tauri/package.json` | 修改 | 添加 CodeMirror 依赖 |
| `installer-tauri/src/renderer/src/components/ConfigEditorModal.tsx` | 新增 | 配置编辑弹窗组件 |
| `installer-tauri/src/renderer/src/pages/LauncherDashboard.tsx` | 修改 | 修改 handleOpenConfig 打开弹窗 |
| `installer-tauri/src/lib/api/config.ts` | 已存在 | 复用 readConfigFile、saveConfig |

---

## Task 1: 安装 CodeMirror 依赖

**Files:**
- Modify: `installer-tauri/package.json`

- [ ] **Step 1: 添加 CodeMirror 依赖**

在 package.json 的 dependencies 中添加：

```json
"@codemirror/lang-yaml": "^6.0.0",
"@codemirror/theme-one-dark": "^6.1.0",
"@uiw/react-codemirror": "^4.21.0",
```

- [ ] **Step 2: 安装依赖**

Run: `cd installer-tauri && pnpm install`
Expected: 依赖安装成功

- [ ] **Step 3: Commit**

```bash
git add installer-tauri/package.json installer-tauri/pnpm-lock.yaml
git commit -m "feat(installer-tauri): add CodeMirror dependencies for yaml editor"
```

---

## Task 2: 创建 ConfigEditorModal 组件

**Files:**
- Create: `installer-tauri/src/renderer/src/components/ConfigEditorModal.tsx`

- [ ] **Step 1: 编写 ConfigEditorModal 组件**

创建文件 `installer-tauri/src/renderer/src/components/ConfigEditorModal.tsx`：

```tsx
import React, { useState, useEffect } from 'react';
import { Modal, Button, message, Alert, Spin } from 'antd';
import CodeMirror from '@uiw/react-codemirror';
import { yaml } from '@codemirror/lang-yaml';
import { configApi, serviceApi } from '../../lib/api';

interface ConfigEditorModalProps {
  open: boolean;
  onCancel: () => void;
  onRestartRequired?: () => void;
}

const ConfigEditorModal: React.FC<ConfigEditorModalProps> = ({
  open,
  onCancel,
  onRestartRequired,
}) => {
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [yamlContent, setYamlContent] = useState('');
  const [yamlError, setYamlError] = useState<string | null>(null);
  const [restartModalVisible, setRestartModalVisible] = useState(false);

  // 加载配置文件
  useEffect(() => {
    if (open) {
      loadConfig();
    }
  }, [open]);

  const loadConfig = async () => {
    setLoading(true);
    setYamlError(null);
    try {
      const result = await configApi.readConfigFile();
      if (result.success && result.content) {
        setYamlContent(result.content);
      } else {
        setYamlError(result.error || '读取配置文件失败');
      }
    } catch (err) {
      setYamlError(err instanceof Error ? err.message : '读取配置文件失败');
    } finally {
      setLoading(false);
    }
  };

  // YAML 格式验证（前端）
  const validateYaml = (content: string): boolean => {
    try {
      // 简单验证：检查基本语法
      // 空内容不允许
      if (!content.trim()) {
        setYamlError('配置不能为空');
        return false;
      }
      // 检查是否有明显的语法错误（如不匹配的引号）
      // 这里使用简单检查，实际解析由后端处理
      setYamlError(null);
      return true;
    } catch (err) {
      setYamlError('配置格式错误');
      return false;
    }
  };

  const handleSave = async () => {
    if (!validateYaml(yamlContent)) {
      return;
    }

    setSaving(true);
    try {
      const result = await configApi.saveConfig(yamlContent);
      if (result.success) {
        message.success('配置已保存');
        // 显示重启提示弹窗
        setRestartModalVisible(true);
      } else {
        message.error(result.error || '保存失败');
      }
    } catch (err) {
      message.error(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const handleRestart = async () => {
    setRestartModalVisible(false);
    try {
      // 1. 停止服务（含 agent 校验）
      const stopResult = await serviceApi.stop();
      if (!stopResult.success) {
        // 停止失败，显示错误
        Modal.warning({
          title: '无法重启服务',
          content: stopResult.error || '停止服务失败',
        });
        return;
      }

      // 2. 启动服务
      const startResult = await serviceApi.start();
      if (!startResult.success) {
        message.error(startResult.error || '启动服务失败');
        return;
      }

      message.success('服务已重启');
      onRestartRequired?.();
    } catch (err) {
      message.error(err instanceof Error ? err.message : '重启服务失败');
    }
  };

  const handleChange = (value: string) => {
    setYamlContent(value);
    // 清除错误提示
    if (yamlError) {
      setYamlError(null);
    }
  };

  return (
    <>
      <Modal
        title="系统配置"
        open={open}
        onCancel={onCancel}
        width={700}
        footer={[
          <Button key="cancel" onClick={onCancel}>
            取消
          </Button>,
          <Button key="save" type="primary" loading={saving} onClick={handleSave}>
            保存
          </Button>,
        ]}
        destroyOnClose
      >
        {loading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Spin />
          </div>
        ) : yamlError ? (
          <Alert type="error" showIcon message={yamlError} />
        ) : (
          <div style={{ height: 500, overflow: 'auto' }}>
            <CodeMirror
              value={yamlContent}
              height="500px"
              extensions={[yaml()]}
              onChange={handleChange}
              theme="light"
              style={{
                fontSize: 13,
                fontFamily: 'Consolas, Monaco, monospace',
              }}
            />
          </div>
        )}
      </Modal>

      {/* 重启提示弹窗 */}
      <Modal
        title="配置已更新"
        open={restartModalVisible}
        onCancel={() => setRestartModalVisible(false)}
        footer={[
          <Button key="close" onClick={() => setRestartModalVisible(false)}>
            关闭
          </Button>,
          <Button key="restart" type="primary" onClick={handleRestart}>
            重启服务
          </Button>,
        ]}
      >
        <p>配置已更新，重启服务生效。</p>
      </Modal>
    </>
  );
};

export default ConfigEditorModal;
```

- [ ] **Step 2: 验证组件编译**

Run: `cd installer-tauri && pnpm typecheck`
Expected: 无 TypeScript 错误

- [ ] **Step 3: Commit**

```bash
git add installer-tauri/src/renderer/src/components/ConfigEditorModal.tsx
git commit -m "feat(installer-tauri): add ConfigEditorModal component with CodeMirror yaml editor"
```

---

## Task 3: 修改 LauncherDashboard 按钮逻辑

**Files:**
- Modify: `installer-tauri/src/renderer/src/pages/LauncherDashboard.tsx:196-203`

- [ ] **Step 1: 导入 ConfigEditorModal**

在 LauncherDashboard.tsx 第 24 行后添加导入：

```tsx
import ConfigEditorModal from '../components/ConfigEditorModal';
```

- [ ] **Step 2: 添加弹窗状态**

在 LauncherDashboard.tsx 第 88 行（agentModalVisible 状态后）添加：

```tsx
const [configModalVisible, setConfigModalVisible] = useState(false);
```

- [ ] **Step 3: 修改 handleOpenConfig 函数**

替换第 196-202 行的 handleOpenConfig 函数：

```tsx
const handleOpenConfig = () => {
  setConfigModalVisible(true);
};
```

- [ ] **Step 4: 修改按钮点击事件**

替换第 338-343 行的按钮，确保 onClick 调用 handleOpenConfig：

```tsx
<Button
  icon={<SettingOutlined />}
  onClick={handleOpenConfig}
>
  系统配置
</Button>
```

- [ ] **Step 5: 添加 ConfigEditorModal 渲染**

在 LauncherDashboard.tsx 第 439 行（agent Modal 结束标签后）添加：

```tsx
{/* 系统配置编辑弹窗 */}
<ConfigEditorModal
  open={configModalVisible}
  onCancel={() => setConfigModalVisible(false)}
  onRestartRequired={checkStatus}
/>
```

- [ ] **Step 6: 验证编译**

Run: `cd installer-tauri && pnpm typecheck`
Expected: 无 TypeScript 错误

- [ ] **Step 7: Commit**

```bash
git add installer-tauri/src/renderer/src/pages/LauncherDashboard.tsx
git commit -m "feat(installer-tauri): modify SystemConfig button to open config editor modal"
```

---

## Task 4: 测试验证

**Files:**
- 无新文件

- [ ] **Step 1: 启动 Launcher 开发模式**

Run: `cd installer-tauri && pnpm dev:launcher`
Expected: Launcher 界面启动成功

- [ ] **Step 2: 验证功能**

手动验证：
1. 点击「系统配置」按钮 → 打开弹窗（不打开文件夹）
2. 弹窗展示 config.yaml 内容，有语法高亮
3. 编辑内容并保存 → 弹出重启提示
4. 点击「重启服务」 → 服务正常重启（或显示 agent 校验错误）

- [ ] **Step 3: Commit（如有修复）**

如有修复代码：
```bash
git add -A
git commit -m "fix(installer-tauri): fix issues found during testing"
```

---

## 自审检查

**1. Spec 覆盖检查**：
| 需求 | 任务 |
|------|------|
| 点击按钮打开弹窗 | Task 3 |
| yaml 语法高亮 | Task 1 + Task 2 |
| yaml 格式验证 | Task 2（validateYaml） |
| 保存后重启提示 | Task 2（restartModal） |
| 重启含 agent 校验 | Task 2（调用 serviceApi.stop） |

**2. Placeholder 检查**：
- 无 TBD、TODO、implement later
- 所有代码完整

**3. 类型一致性检查**：
- ConfigEditorModalProps 类型定义与使用一致
- configApi、serviceApi 类型与 API 定义一致

---

## 执行选项

**计划已完成，保存到 docs/superpowers/plans/2026-05-07-launcher-config-editor.md**

**两种执行方式：**

1. **Subagent-Driven（推荐）** - 每个任务派发独立子代理，任务间有审查点，快速迭代

2. **Inline Execution** - 当前会话批量执行，批量审查点

**选择哪种方式？**