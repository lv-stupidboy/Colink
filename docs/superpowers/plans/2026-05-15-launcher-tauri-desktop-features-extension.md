# Tauri Launcher 桌面应用功能扩展实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Tauri Launcher 的 Web 控制台通用设置页面中补充 3 个功能模块：系统配置编辑、智能体检测管理、快捷操作。

**Architecture:** Web 控制台通过 iframe postMessage 调用 Tauri 命令。WebUIContainer.tsx 监听消息并调用 invoke()，Rust 命令执行原生操作。

**Tech Stack:** Tauri 2 (Rust)、React (Ant Design)、postMessage IPC

---

## 文件结构

### Tauri Launcher 修改

| 文件 | 责任 |
|------|------|
| `installer-tauri/src-tauri/src/services/dependency.rs` | 扩展支持 4 个智能体检测和安装 |
| `installer-tauri/src-tauri/src/commands/mod.rs` | 导入新命令 |
| `installer-tauri/src/launcher/WebUIContainer.tsx` | 扩展 postMessage 处理（config、dependency、open 类型） |

### Web 控制台新增

| 文件 | 责任 |
|------|------|
| `web/src/utils/tauriBridge.ts` | iframe-to-Tauri postMessage 通信工具 |
| `web/src/components/settings/SystemConfigCard.tsx` | 系统配置编辑器卡片 |
| `web/src/components/settings/AgentManagementCard.tsx` | 智能体管理卡片 |
| `web/src/components/settings/QuickAccessCard.tsx` | 快捷操作卡片 |
| `web/src/pages/Settings/GeneralSettings.tsx` | 集成新组件 |

---

## Task 1: 扩展 dependency.rs 支持 Hermes 和 OpenClaw

**Files:**
- Modify: `installer-tauri/src-tauri/src/services/dependency.rs:21-114`

- [ ] **Step 1: 添加 Hermes 和 OpenClaw 检测**

在 `check_dependency` 函数的 match 语句中添加：

```rust
/// Check if a dependency is installed
pub fn check_dependency(key: &str) -> DependencyInfo {
    let (name, tool) = match key {
        "node" => ("Node.js", "node"),
        "git" => ("Git", "git"),
        "claude" => ("Claude CLI", "claude"),
        "opencode" => ("OpenCode", "opencode"),
        "hermes" => ("Hermes", "hermes"),
        "openclaw" => ("OpenClaw", "openclaw"),
        _ => ("Unknown", key),
    };

    let version = get_tool_version(tool);

    DependencyInfo {
        key: key.to_string(),
        name: name.to_string(),
        installed: version.is_some(),
        version,
    }
}
```

- [ ] **Step 2: 修改 check_all_dependencies**

修改 `check_all_dependencies` 函数：

```rust
/// Check all dependencies (all 4 agents)
pub fn check_all_dependencies() -> Vec<DependencyInfo> {
    let keys = ["claude", "opencode", "hermes", "openclaw"];
    keys.iter().map(|k| check_dependency(k)).collect()
}
```

- [ ] **Step 3: 扩展 install_dependency**

修改 `install_dependency` 函数：

```rust
/// Install a dependency (npm package globally)
pub fn install_dependency(key: &str) -> Result<()> {
    #[cfg(target_os = "windows")]
    {
        let package = match key {
            "claude" => "@anthropic-ai/claude-code",
            "opencode" => "opencode",
            "openclaw" => "openclaw",
            "hermes" => return Err(InstallerError::DependencyNotFound(
                "Hermes 需在 WSL2 中安装，Windows 原生不支持".to_string()
            )),
            _ => return Err(InstallerError::DependencyNotFound(key.to_string())),
        };

        let output = Command::new("npm")
            .args(["install", "-g", package])
            .creation_flags(CREATE_NO_WINDOW)
            .output();

        match output {
            Ok(o) => {
                if !o.status.success() {
                    return Err(InstallerError::Process(
                        String::from_utf8_lossy(&o.stderr).to_string(),
                    ));
                }
                Ok(())
            }
            Err(e) => Err(InstallerError::Process(e.to_string())),
        }
    }

    #[cfg(not(target_os = "windows"))]
    {
        Ok(())
    }
}
```

- [ ] **Step 4: 验证 Rust 编译**

Run: `cd installer-tauri && cargo check --manifest-path src-tauri/Cargo.toml`
Expected: 编译成功无错误

- [ ] **Step 5: Commit**

```bash
git add installer-tauri/src-tauri/src/services/dependency.rs
git commit -m "feat: extend dependency.rs to support hermes and openclaw"
```

---

## Task 2: 扩展 WebUIContainer.tsx 处理新消息类型

**Files:**
- Modify: `installer-tauri/src/launcher/WebUIContainer.tsx:14-96`

- [ ] **Step 1: 扩展消息类型定义**

修改 `IFrameMessageType` 类型：

```typescript
// iframe 与 Tauri 的消息类型
type IFrameMessageType =
  | 'check-notification-permission'
  | 'request-notification-permission'
  | 'send-notification'
  | 'get-tauri-environment'
  | 'config:read'
  | 'config:save'
  | 'dependency:check'
  | 'dependency:install'
  | 'dependency:check-all'
  | 'open:log_dir'
  | 'open:data_dir'
  | 'open:config_dir';
```

- [ ] **Step 2: 添加 switch case 处理新消息**

在 `handleIFrameMessage` 函数的 switch 语句中添加新 case：

```typescript
case 'config:read':
  const configContent = await invoke<{ success: boolean; content?: string; error?: string }>('read_config_file');
  sendMessage({ type: 'config:read:result', payload: configContent });
  break;

case 'config:save':
  const yamlContent = message.payload?.yaml as string;
  const saveResult = await invoke<{ success: boolean; error?: string }>('save_config', { yaml: yamlContent });
  sendMessage({ type: 'config:save:result', payload: saveResult });
  break;

case 'dependency:check':
  const agentKey = message.payload?.agent as string;
  const depInfo = await invoke<{ key: string; name: string; installed: boolean; version?: string }>('check_dependency', { key: agentKey });
  sendMessage({ type: 'dependency:check:result', payload: depInfo });
  break;

case 'dependency:install':
  const installKey = message.payload?.agent as string;
  const installResult = await invoke<{ success: boolean; error?: string }>('install_dependency', { key: installKey });
  sendMessage({ type: 'dependency:install:result', payload: installResult });
  break;

case 'dependency:check-all':
  const allDeps = await invoke<Array<{ key: string; name: string; installed: boolean; version?: string }>>('check_all_dependencies');
  sendMessage({ type: 'dependency:check-all:result', payload: allDeps });
  break;

case 'open:log_dir':
  await invoke('open_logs');
  sendMessage({ type: 'open:log_dir:result', payload: { success: true } });
  break;

case 'open:data_dir':
  await invoke('open_data_dir');
  sendMessage({ type: 'open:data_dir:result', payload: { success: true } });
  break;

case 'open:config_dir':
  await invoke('open_config');
  sendMessage({ type: 'open:config_dir:result', payload: { success: true } });
  break;
```

- [ ] **Step 3: 验证 TypeScript 编译**

Run: `cd installer-tauri && pnpm typecheck`
Expected: 编译成功无错误

- [ ] **Step 4: Commit**

```bash
git add installer-tauri/src/launcher/WebUIContainer.tsx
git commit -m "feat: extend WebUIContainer to handle config/dependency/open messages"
```

---

## Task 3: 创建 tauriBridge.ts 通信工具

**Files:**
- Create: `web/src/utils/tauriBridge.ts`

- [ ] **Step 1: 创建通信工具文件**

创建文件内容：

```typescript
/**
 * Tauri Bridge - iframe 与 Tauri Launcher 通信工具
 * 
 * Web 控制台通过 iframe 嵌入到 Tauri Launcher 中，
 * 使用 postMessage 与 WebUIContainer 通信，调用 Tauri 命令。
 */

// 检测是否运行在 Tauri iframe 环境
export function isInTauriIframe(): boolean {
  // 检查是否在 iframe 中且有父窗口
  try {
    return window.self !== window.top && window.parent !== window;
  } catch {
    return false;
  }
}

// 生成唯一请求 ID
function generateRequestId(): string {
  return `req-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
}

// 发送消息并等待响应
async function sendAndWaitResponse<T>(
  type: string,
  payload?: Record<string, unknown>
): Promise<T> {
  if (!isInTauriIframe()) {
    throw new Error('Not in Tauri iframe environment');
  }

  const requestId = generateRequestId();
  
  return new Promise<T>((resolve, reject) => {
    const timeout = setTimeout(() => {
      window.removeEventListener('message', handler);
      reject(new Error(`Timeout waiting for response: ${type}`));
    }, 30000); // 30 秒超时

    const handler = (event: MessageEvent) => {
      // 只处理来自父窗口的消息
      if (event.source !== window.parent) return;
      
      const response = event.data;
      if (response.requestId === requestId) {
        clearTimeout(timeout);
        window.removeEventListener('message', handler);
        
        if (response.type === 'error') {
          reject(new Error(response.error || 'Unknown error'));
        } else {
          resolve(response.payload as T);
        }
      }
    };

    window.addEventListener('message', handler);
    
    window.parent.postMessage({
      type,
      payload,
      requestId,
    }, '*');
  });
}

// ============== 配置相关 API ==============

export async function readConfigFile(): Promise<{ success: boolean; content?: string; error?: string }> {
  return sendAndWaitResponse('config:read');
}

export async function saveConfig(yaml: string): Promise<{ success: boolean; error?: string }> {
  return sendAndWaitResponse('config:save', { yaml });
}

// ============== 智能体依赖相关 API ==============

export interface DependencyInfo {
  key: string;
  name: string;
  installed: boolean;
  version?: string;
}

export async function checkDependency(agent: string): Promise<DependencyInfo> {
  return sendAndWaitResponse('dependency:check', { agent });
}

export async function installDependency(agent: string): Promise<{ success: boolean; error?: string }> {
  return sendAndWaitResponse('dependency:install', { agent });
}

export async function checkAllDependencies(): Promise<DependencyInfo[]> {
  return sendAndWaitResponse('dependency:check-all');
}

// ============== 目录打开 API ==============

export async function openLogDir(): Promise<{ success: boolean }> {
  return sendAndWaitResponse('open:log_dir');
}

export async function openDataDir(): Promise<{ success: boolean }> {
  return sendAndWaitResponse('open:data_dir');
}

export async function openConfigDir(): Promise<{ success: boolean }> {
  return sendAndWaitResponse('open:config_dir');
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/utils/tauriBridge.ts
git commit -m "feat: add tauriBridge.ts for iframe-to-Tauri communication"
```

---

## Task 4: 创建 SystemConfigCard 组件

**Files:**
- Create: `web/src/components/settings/SystemConfigCard.tsx`

- [ ] **Step 1: 创建组件目录**

Run: `mkdir -p web/src/components/settings`

- [ ] **Step 2: 创建 SystemConfigCard 组件**

创建文件内容：

```typescript
import React, { useState, useEffect } from 'react';
import { Card, Button, Input, Alert, Space, Spin, message } from 'antd';
import { SaveOutlined, ReloadOutlined } from '@ant-design/icons';
import { isInTauriIframe, readConfigFile, saveConfig } from '@/utils/tauriBridge';

const { TextArea } = Input;

const SystemConfigCard: React.FC = () => {
  const [configContent, setConfigContent] = useState('');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [notInTauri, setNotInTauri] = useState(false);

  useEffect(() => {
    // 检测是否在 Tauri 环境
    if (!isInTauriIframe()) {
      setNotInTauri(true);
      return;
    }
    loadConfig();
  }, []);

  const loadConfig = async () => {
    setLoading(true);
    try {
      const result = await readConfigFile();
      if (result.success && result.content) {
        setConfigContent(result.content);
      } else {
        message.error(result.error || '读取配置失败');
      }
    } catch (error) {
      message.error('读取配置失败，请检查 Tauri 环境');
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    if (!configContent.trim()) {
      message.warning('配置内容不能为空');
      return;
    }
    setSaving(true);
    try {
      const result = await saveConfig(configContent);
      if (result.success) {
        message.success('配置已保存，请重启服务使配置生效');
      } else {
        message.error(result.error || '保存配置失败');
      }
    } catch (error) {
      message.error('保存配置失败');
    } finally {
      setSaving(false);
    }
  };

  if (notInTauri) {
    return (
      <Card title="系统配置" style={{ marginBottom: 16 }}>
        <Alert
          type="warning"
          message="此功能仅在 Launcher 桌面应用中可用"
          description="请在 Tauri Launcher 中打开 Web 控制台以使用此功能"
          showIcon
        />
      </Card>
    );
  }

  return (
    <Card title="系统配置" style={{ marginBottom: 16 }}>
      <Spin spinning={loading}>
        <TextArea
          value={configContent}
          onChange={(e) => setConfigContent(e.target.value)}
          placeholder="配置文件内容..."
          rows={12}
          style={{ fontFamily: 'monospace', marginBottom: 16 }}
        />

        <Space>
          <Button
            type="primary"
            icon={<SaveOutlined />}
            onClick={handleSave}
            loading={saving}
          >
            保存配置
          </Button>
          <Button
            icon={<ReloadOutlined />}
            onClick={loadConfig}
            loading={loading}
          >
            重新加载
          </Button>
        </Space>

        <Alert
          type="info"
          message="配置修改后需要重启服务才能生效"
          showIcon
          style={{ marginTop: 16 }}
        />
      </Spin>
    </Card>
  );
};

export default SystemConfigCard;
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/settings/SystemConfigCard.tsx
git commit -m "feat: add SystemConfigCard component for config.yaml editing"
```

---

## Task 5: 创建 AgentManagementCard 组件

**Files:**
- Create: `web/src/components/settings/AgentManagementCard.tsx`

- [ ] **Step 1: 创建智能体管理组件**

创建文件内容：

```typescript
import React, { useState, useEffect } from 'react';
import { Card, Table, Button, Space, Tag, Alert, Spin, message, Typography } from 'antd';
import { CheckCircleOutlined, CloseCircleOutlined, SyncOutlined, InfoCircleOutlined } from '@ant-design/icons';
import { isInTauriIframe, checkAllDependencies, installDependency, DependencyInfo } from '@/utils/tauriBridge';

const { Text } = Typography;

// 智能体配置
const AGENT_CONFIG = {
  claude: {
    installCmd: 'npm install -g @anthropic-ai/claude-code',
    canInstall: true,
    note: undefined,
  },
  opencode: {
    installCmd: 'npm install -g opencode',
    canInstall: true,
    note: undefined,
  },
  hermes: {
    installCmd: undefined,
    canInstall: false,
    note: 'Hermes 需在 WSL2 中安装，Windows 原生不支持',
  },
  openclaw: {
    installCmd: 'npm install -g openclaw',
    canInstall: true,
    note: '安装后请运行 openclaw onboard 完成初始化配置',
  },
};

const AgentManagementCard: React.FC = () => {
  const [agents, setAgents] = useState<DependencyInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [installing, setInstalling] = useState<string | null>(null);
  const [notInTauri, setNotInTauri] = useState(false);

  useEffect(() => {
    if (!isInTauriIframe()) {
      setNotInTauri(true);
      return;
    }
    loadAgents();
  }, []);

  const loadAgents = async () => {
    setLoading(true);
    try {
      const result = await checkAllDependencies();
      setAgents(result);
    } catch (error) {
      message.error('检测智能体失败');
    } finally {
      setLoading(false);
    }
  };

  const handleInstall = async (agentKey: string) => {
    setInstalling(agentKey);
    try {
      const result = await installDependency(agentKey);
      if (result.success) {
        message.success('安装成功');
        await loadAgents();
      } else {
        message.error(result.error || '安装失败');
      }
    } catch (error) {
      message.error('安装失败');
    } finally {
      setInstalling(null);
    }
  };

  const handleCheck = async (agentKey: string) => {
    setLoading(true);
    try {
      const result = await checkAllDependencies();
      setAgents(result);
      message.success('检测完成');
    } catch (error) {
      message.error('检测失败');
    } finally {
      setLoading(false);
    }
  };

  if (notInTauri) {
    return (
      <Card title="智能体管理" style={{ marginBottom: 16 }}>
        <Alert
          type="warning"
          message="此功能仅在 Launcher 桌面应用中可用"
          description="请在 Tauri Launcher 中打开 Web 控制台以使用此功能"
          showIcon
        />
      </Card>
    );
  }

  const columns = [
    {
      title: '智能体',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '状态',
      dataIndex: 'installed',
      key: 'status',
      render: (installed: boolean, record: DependencyInfo) => (
        installed ? (
          <Space>
            <CheckCircleOutlined style={{ color: '#52c41a' }} />
            <Tag color="green">已安装 {record.version}</Tag>
          </Space>
        ) : (
          <Space>
            <CloseCircleOutlined style={{ color: '#ff4d4f' }} />
            <Tag color="red">未安装</Tag>
          </Space>
        )
      ),
    },
    {
      title: '操作',
      key: 'action',
      render: (_: unknown, record: DependencyInfo) => {
        const config = AGENT_CONFIG[record.key as keyof typeof AGENT_CONFIG];
        return (
          <Space direction="vertical" size="small">
            <Space>
              {config?.canInstall && !record.installed && (
                <Button
                  type="primary"
                  size="small"
                  onClick={() => handleInstall(record.key)}
                  loading={installing === record.key}
                >
                  安装
                </Button>
              )}
              <Button
                size="small"
                icon={<SyncOutlined />}
                onClick={handleCheck}
              >
                重新检测
              </Button>
            </Space>
            {config?.note && (
              <Text type="secondary" style={{ fontSize: 12 }}>
                <InfoCircleOutlined /> {config.note}
              </Text>
            )}
          </Space>
        );
      },
    },
  ];

  return (
    <Card 
      title="智能体管理" 
      style={{ marginBottom: 16 }}
      extra={
        <Button 
          icon={<SyncOutlined />} 
          onClick={loadAgents}
          loading={loading}
        >
          全部重新检测
        </Button>
      }
    >
      <Spin spinning={loading}>
        <Table
          dataSource={agents}
          columns={columns}
          rowKey="key"
          pagination={false}
          size="small"
        />
      </Spin>
    </Card>
  );
};

export default AgentManagementCard;
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/settings/AgentManagementCard.tsx
git commit -m "feat: add AgentManagementCard component for agent detection"
```

---

## Task 6: 创建 QuickAccessCard 组件

**Files:**
- Create: `web/src/components/settings/QuickAccessCard.tsx`

- [ ] **Step 1: 创建快捷操作组件**

创建文件内容：

```typescript
import React, { useState, useEffect } from 'react';
import { Card, Button, Space, Alert, message } from 'antd';
import { FolderOpenOutlined, FileTextOutlined, SettingOutlined } from '@ant-design/icons';
import { isInTauriIframe, openLogDir, openDataDir, openConfigDir } from '@/utils/tauriBridge';

const QuickAccessCard: React.FC = () => {
  const [notInTauri, setNotInTauri] = useState(false);

  useEffect(() => {
    if (!isInTauriIframe()) {
      setNotInTauri(true);
    }
  }, []);

  const handleOpenLogs = async () => {
    try {
      await openLogDir();
      message.success('已打开日志目录');
    } catch (error) {
      message.error('打开日志目录失败');
    }
  };

  const handleOpenDataDir = async () => {
    try {
      await openDataDir();
      message.success('已打开数据目录');
    } catch (error) {
      message.error('打开数据目录失败');
    }
  };

  const handleOpenConfigDir = async () => {
    try {
      await openConfigDir();
      message.success('已打开配置目录');
    } catch (error) {
      message.error('打开配置目录失败');
    }
  };

  if (notInTauri) {
    return (
      <Card title="快捷操作" style={{ marginBottom: 16 }}>
        <Alert
          type="warning"
          message="此功能仅在 Launcher 桌面应用中可用"
          description="请在 Tauri Launcher 中打开 Web 控制台以使用此功能"
          showIcon
        />
      </Card>
    );
  }

  return (
    <Card title="快捷操作" style={{ marginBottom: 16 }}>
      <Space size="middle">
        <Button
          icon={<FileTextOutlined />}
          onClick={handleOpenLogs}
        >
          查看日志
        </Button>
        <Button
          icon={<FolderOpenOutlined />}
          onClick={handleOpenDataDir}
        >
          打开数据目录
        </Button>
        <Button
          icon={<SettingOutlined />}
          onClick={handleOpenConfigDir}
        >
          打开配置目录
        </Button>
      </Space>
    </Card>
  );
};

export default QuickAccessCard;
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/settings/QuickAccessCard.tsx
git commit -m "feat: add QuickAccessCard component for quick directory access"
```

---

## Task 7: 集成组件到 GeneralSettings 页面

**Files:**
- Modify: `web/src/pages/Settings/GeneralSettings.tsx`

- [ ] **Step 1: 添加导入语句**

在文件顶部添加导入：

```typescript
import SystemConfigCard from '@/components/settings/SystemConfigCard';
import AgentManagementCard from '@/components/settings/AgentManagementCard';
import QuickAccessCard from '@/components/settings/QuickAccessCard';
```

- [ ] **Step 2: 在页面底部添加新卡片**

在 `GeneralSettings` 组件的 return 语句中，在治理摘要编辑卡片之后添加：

```typescript
      {/* 系统配置编辑器卡片 */}
      <SystemConfigCard />

      {/* 智能体管理卡片 */}
      <AgentManagementCard />

      {/* 快捷操作卡片 */}
      <QuickAccessCard />
    </div>
  );
};
```

- [ ] **Step 3: 验证 TypeScript 编译**

Run: `cd web && npx tsc --noEmit`
Expected: 编译成功无错误（忽略预先存在的测试文件错误）

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/Settings/GeneralSettings.tsx
git commit -m "feat: integrate SystemConfigCard, AgentManagementCard, QuickAccessCard into GeneralSettings"
```

---

## Task 8: 最终验证和提交

- [ ] **Step 1: 运行前端 lint 检查**

Run: `cd web && npm run lint`
Expected: lint 通过（max-warnings 0）

- [ ] **Step 2: 运行 TypeScript 类型检查**

Run: `cd installer-tauri && pnpm typecheck`
Expected: TypeScript 编译成功

- [ ] **Step 3: 查看所有变更**

Run: `git status`
Expected: 所有文件已提交

- [ ] **Step 4: 查看提交历史**

Run: `git log --oneline -5`
Expected: 5 个提交对应 Task 1-7

---

## 成功标准

1. 用户可在通用设置页面直接编辑 config.yaml
2. 保存配置后显示重启提醒
3. 智能体管理显示 Claude、OpenCode、Hermes、OpenClaw 4 种状态
4. Claude、OpenCode、OpenClaw 提供安装按钮
5. Hermes 显示 WSL2 安装提示
6. 快捷操作按钮可打开对应目录
7. 在浏览器直接访问时显示功能不可用提示