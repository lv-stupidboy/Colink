// isdp/web/src/components/thread/SandboxPanel.tsx
import React from 'react';
import {
  Button,
  Space,
  Typography,
  Empty,
  Tooltip,
} from 'antd';
import {
  ReloadOutlined,
  ExpandOutlined,
  StopOutlined,
  DesktopOutlined,
  CloudServerOutlined,
} from '@ant-design/icons';
import type { SandboxServer } from '@/types';
import './SandboxPanel.css';

const { Text } = Typography;

interface SandboxPanelProps {
  // 关闭侧边栏
  onClose?: () => void;
  // 调试模式还是团队模式
  isDebugMode: boolean;
  // 是否有工作目录（调试模式需要）
  hasProjectPath: boolean;
  // 沙箱服务器状态
  sandboxServer: SandboxServer | null;
  sandboxLoading: boolean;
  dockerAvailable: boolean;
  // 运行沙箱
  onRunSandbox: (mode: 'local' | 'docker') => void;
  // 停止沙箱
  onStopSandbox: () => void;
  // 自定义宽度
  width?: number;
}

export const SandboxPanel: React.FC<SandboxPanelProps> = ({
  onClose,
  isDebugMode,
  hasProjectPath,
  sandboxServer,
  sandboxLoading,
  dockerAvailable,
  onRunSandbox,
  onStopSandbox,
  width,
}) => {
  // 刷新预览
  const handleRefresh = () => {
    const iframe = document.querySelector('#sandbox-preview-iframe') as HTMLIFrameElement;
    if (iframe) {
      iframe.src = iframe.src;
    }
  };

  // 新窗口打开
  const handleOpenNewWindow = () => {
    if (sandboxServer?.url) {
      window.open(sandboxServer.url, '_blank');
    }
  };

  // 判断是否可以运行沙箱
  const canRunSandbox = isDebugMode ? hasProjectPath : true;

  return (
    <div
      className="right-sidebar sandbox-panel"
      style={width ? { width, minWidth: width, maxWidth: width } : undefined}
    >
      {/* 标题栏 */}
      <div className="right-sidebar-header">
        <span style={{ fontWeight: 500 }}>沙箱预览</span>
        <Tooltip title="关闭侧边栏">
          <Button
            type="text"
            size="small"
            onClick={onClose}
          >
            ✕
          </Button>
        </Tooltip>
      </div>

      {/* 控制栏 */}
      <div className="sandbox-control-bar">
        <Space>
          <Tooltip title="在本地进程沙箱中运行项目并预览">
            <Button
              icon={<DesktopOutlined />}
              onClick={() => onRunSandbox('local')}
              loading={sandboxLoading}
              disabled={!canRunSandbox}
              size="small"
            >
              本地沙箱
            </Button>
          </Tooltip>
          <Tooltip title={dockerAvailable ? "在Docker容器沙箱中运行项目并预览" : "Docker不可用"}>
            <Button
              icon={<CloudServerOutlined />}
              onClick={() => onRunSandbox('docker')}
              loading={sandboxLoading}
              disabled={!dockerAvailable || !canRunSandbox}
              size="small"
            >
              容器沙箱
            </Button>
          </Tooltip>
        </Space>
      </div>

      {/* 预览控制栏 */}
      {sandboxServer && (
        <div className="sandbox-preview-bar">
          <Text type="secondary" style={{ fontSize: 12 }}>
            {sandboxServer.url}
          </Text>
          <Space size="small">
            <Button
              size="small"
              icon={<ReloadOutlined />}
              onClick={handleRefresh}
            />
            <Button
              size="small"
              icon={<ExpandOutlined />}
              onClick={handleOpenNewWindow}
            />
            <Button
              size="small"
              icon={<StopOutlined />}
              danger
              onClick={onStopSandbox}
            />
          </Space>
        </div>
      )}

      {/* 预览区域 */}
      <div className="sandbox-iframe-container">
        {sandboxServer ? (
          <iframe
            id="sandbox-preview-iframe"
            src={sandboxServer.url}
            style={{
              width: '100%',
              height: '100%',
              border: 'none',
            }}
            title="沙箱预览"
          />
        ) : (
          <div className="sandbox-empty">
            <Empty
              description={isDebugMode ? "输入工作目录后点击运行按钮启动项目预览" : "点击运行按钮启动项目预览"}
              image={Empty.PRESENTED_IMAGE_SIMPLE}
            />
          </div>
        )}
      </div>
    </div>
  );
};