// isdp/web/src/pages/thread/ThreadHeader.tsx
import React, { memo } from 'react';
import { Button, Space, Tag, Badge, Tooltip } from 'antd';
import {
  ArrowLeftOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  FullscreenOutlined,
  UnorderedListOutlined,
  DesktopOutlined,
} from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';

interface ThreadHeaderProps {
  wsConnected: boolean;
  isDebugMode: boolean;
  fileSidebarVisible: boolean;
  artifactsSidebarVisible: boolean;
  rightPanelVisible: boolean;
  hasSandboxServer: boolean;
  soloMode: boolean;
  isRunning: boolean;
  debugAgentName?: string;
  activeAgentCount: number;
  onToggleFileSidebar: () => void;
  onToggleArtifactsSidebar: () => void;
  onToggleRightPanel: () => void;
  onToggleSoloMode: () => void;
}

/**
 * ThreadView 顶部控制栏组件
 * 只订阅需要的状态，避免整体重渲染
 */
export const ThreadHeader: React.FC<ThreadHeaderProps> = memo(({
  wsConnected,
  isDebugMode,
  fileSidebarVisible,
  artifactsSidebarVisible,
  rightPanelVisible,
  hasSandboxServer,
  soloMode,
  isRunning,
  debugAgentName,
  activeAgentCount,
  onToggleFileSidebar,
  onToggleArtifactsSidebar,
  onToggleRightPanel,
  onToggleSoloMode,
}) => {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();

  return (
    <div className="intervention-bar">
      <Space style={{ width: '100%', justifyContent: 'space-between' }}>
        <Space>
          <Tooltip title={fileSidebarVisible ? '隐藏文件树' : '显示文件树'}>
            <Button
              icon={fileSidebarVisible ? <MenuFoldOutlined /> : <MenuUnfoldOutlined />}
              onClick={onToggleFileSidebar}
              size="small"
            />
          </Tooltip>
          <Button
            icon={<ArrowLeftOutlined />}
            onClick={() => isDebugMode ? navigate('/agents') : navigate(`/projects/${projectId}`)}
            size="small"
          >
            {isDebugMode ? '返回 Agent 列表' : '返回项目'}
          </Button>
          <Tag color={wsConnected ? 'green' : 'red'}>
            {wsConnected ? '已连接' : '未连接'}
          </Tag>
          {isDebugMode && debugAgentName && (
            <Tag color="purple">调试: {debugAgentName}</Tag>
          )}
          {isRunning && (
            <Badge status="processing" text={`${activeAgentCount} 个 Agent 运行中`} />
          )}
        </Space>
        <Space>
          <Tooltip title="进入 Solo 模式">
            <Button
              icon={<FullscreenOutlined />}
              onClick={onToggleSoloMode}
              size="small"
              type={soloMode ? 'primary' : 'default'}
            >
              Solo
            </Button>
          </Tooltip>
          <Tooltip title={artifactsSidebarVisible ? '隐藏产物' : '查看产物列表'}>
            <Button
              icon={<UnorderedListOutlined />}
              onClick={onToggleArtifactsSidebar}
              size="small"
              type={artifactsSidebarVisible ? 'primary' : 'default'}
            >
              产物
            </Button>
          </Tooltip>
          <Tooltip title={rightPanelVisible ? '隐藏面板' : '打开代码/沙箱面板'}>
            <Button
              icon={<DesktopOutlined />}
              onClick={onToggleRightPanel}
              size="small"
              type={rightPanelVisible || hasSandboxServer ? 'primary' : 'default'}
            >
              面板
            </Button>
          </Tooltip>
        </Space>
      </Space>
    </div>
  );
});

ThreadHeader.displayName = 'ThreadHeader';