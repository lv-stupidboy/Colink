// isdp/web/src/components/thread/RightPanel.tsx
import React from 'react';
import { Tabs, Empty, Button, Space, Tooltip } from 'antd';
import { CodeOutlined, DesktopOutlined, PlayCircleOutlined, StopOutlined } from '@ant-design/icons';
import { CodePanel } from './CodePanel';
import type { FileChange } from '@/types/content';

interface SandboxServer {
  id: string;
  port: number;
  status: string;
}

interface RightPanelProps {
  visible: boolean;
  onClose: () => void;
  activeTab: 'code' | 'sandbox';
  onTabChange: (tab: 'code' | 'sandbox') => void;
  codeFiles: FileChange[];
  expandedFiles: Set<string>;
  onToggleFile: (fileId: string) => void;
  sandboxServer?: SandboxServer | null;
  sandboxLoading?: boolean;
  dockerAvailable?: boolean;
  hasProjectPath?: boolean;
  isDebugMode?: boolean;
  onRunSandbox: (mode: 'local' | 'docker') => void | Promise<void>;
  onStopSandbox: () => void;
  width?: number;
}

export const RightPanel: React.FC<RightPanelProps> = ({
  visible,
  activeTab,
  onTabChange,
  codeFiles,
  expandedFiles,
  onToggleFile,
  sandboxServer,
  sandboxLoading,
  dockerAvailable,
  hasProjectPath,
  onRunSandbox,
  onStopSandbox,
  width = 520,
}) => {
  if (!visible) return null;

  const items = [
    {
      key: 'code',
      label: (
        <span>
          <CodeOutlined /> 代码预览
        </span>
      ),
      children: (
        <div style={{ height: 'calc(100vh - 120px)', overflow: 'auto' }}>
          {codeFiles.length === 0 ? (
            <Empty description="暂无代码变更" style={{ marginTop: '40px' }} />
          ) : (
            <CodePanel
              isOpen={true}
              isCollapsed={false}
              files={codeFiles}
              expandedFiles={expandedFiles}
              onToggleCollapse={() => {}}
              onClose={() => {}}
              onToggleFile={onToggleFile}
            />
          )}
        </div>
      ),
    },
    {
      key: 'sandbox',
      label: (
        <span>
          <DesktopOutlined /> 沙箱
        </span>
      ),
      children: (
        <div style={{ padding: '16px', height: 'calc(100vh - 120px)', overflow: 'auto' }}>
          {!dockerAvailable ? (
            <Empty description="Docker 不可用，请确保 Docker 已启动" />
          ) : !hasProjectPath ? (
            <Empty description="请先设置项目路径" />
          ) : (
            <Space direction="vertical" style={{ width: '100%' }}>
              {sandboxServer ? (
                <div>
                  <p>沙箱运行中</p>
                  <p>端口: {sandboxServer.port}</p>
                  <p>状态: {sandboxServer.status}</p>
                  <Tooltip title="停止沙箱">
                    <Button
                      danger
                      icon={<StopOutlined />}
                      onClick={onStopSandbox}
                      loading={sandboxLoading}
                    >
                      停止
                    </Button>
                  </Tooltip>
                </div>
              ) : (
                <Space>
                  <Tooltip title="启动沙箱">
                    <Button
                      type="primary"
                      icon={<PlayCircleOutlined />}
                      onClick={() => onRunSandbox('docker')}
                      loading={sandboxLoading}
                    >
                      启动开发模式
                    </Button>
                  </Tooltip>
                  <Button
                    icon={<PlayCircleOutlined />}
                    onClick={() => onRunSandbox('local')}
                    loading={sandboxLoading}
                  >
                    预览模式
                  </Button>
                </Space>
              )}
            </Space>
          )}
        </div>
      ),
    },
  ];

  return (
    <div
      style={{
        width,
        height: '100%',
        backgroundColor: 'var(--bg-container)',
        borderLeft: '1px solid var(--border-color)',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      <Tabs
        activeKey={activeTab}
        onChange={(key) => onTabChange(key as 'code' | 'sandbox')}
        items={items}
        style={{ flex: 1, overflow: 'hidden' }}
        tabBarStyle={{ padding: '0 16px', margin: 0 }}
      />
    </div>
  );
};