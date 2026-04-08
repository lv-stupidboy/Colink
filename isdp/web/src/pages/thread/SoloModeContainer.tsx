// isdp/web/src/pages/thread/SoloModeContainer.tsx
import React, { memo } from 'react';
import { Typography } from 'antd';
import { RobotOutlined } from '@ant-design/icons';
import { ChatMessageList } from '@/components/thread/ChatMessageList';
import { TaskList } from '@/components/thread';
import { StatusPanel } from '@/components/thread/StatusPanel';
import { RightPanel } from '@/components/thread/RightPanel';
import type { Message, Thread, AgentConfig, ToolEvent } from '@/types';
import type { FileChange } from '@/types/content';

const { Title, Text } = Typography;

interface SoloModeContainerProps {
  // 消息相关
  messages: Message[];
  toolEvents: Record<string, ToolEvent[]>;
  agentConfigs: AgentConfig[];
  projectPath: string;

  // 任务相关
  soloTasks: Thread[];
  soloActiveTask: Thread | null;
  isRunning: boolean;

  // 面板相关
  rightPanelVisible: boolean;
  rightPanelWidth: number;
  rightPanelActiveTab: 'code' | 'sandbox';
  codeFiles: FileChange[];
  expandedFiles: Set<string>;
  sandboxServer: { url: string; status: string } | null;
  sandboxLoading: boolean;
  dockerAvailable: boolean;
  isResizing: boolean;

  // 状态栏
  threadId?: string;
  taskDrawerOpen: boolean;

  // 回调
  onStopAgent: (invocationId: string) => void;
  onRetryAgent: (message: Message) => void;
  onOpenCodePanel: (files: FileChange[]) => void;
  onSelectTask: (task: Thread) => void;
  onCreateTask: () => void;
  onDeleteTask?: (taskId: string) => void;
  onClosePanel: () => void;
  onToggleFile: (fileId: string) => void;
  onResizeStart: (e: React.MouseEvent) => void;
  onRunSandbox: (mode: 'local' | 'docker') => void;
  onStopSandbox: () => void;
}

/**
 * Solo 模式容器组件
 * 流式消息由 StreamingMessage 组件独立处理
 */
export const SoloModeContainer: React.FC<SoloModeContainerProps> = memo(({
  messages,
  toolEvents,
  agentConfigs,
  projectPath,
  soloTasks,
  soloActiveTask,
  isRunning,
  rightPanelVisible,
  rightPanelWidth,
  rightPanelActiveTab,
  codeFiles,
  expandedFiles,
  sandboxServer,
  sandboxLoading,
  dockerAvailable,
  isResizing,
  threadId,
  taskDrawerOpen,
  onStopAgent,
  onRetryAgent,
  onOpenCodePanel,
  onSelectTask,
  onCreateTask,
  onDeleteTask,
  onClosePanel,
  onToggleFile,
  onResizeStart,
  onRunSandbox,
  onStopSandbox,
}) => {
  // 流式消息由 StreamingMessage 组件独立处理，这里只检查消息列表
  const hasContent = messages.length > 0;

  return (
    <div className="solo-mode-body">
      {/* 任务抽屉 */}
      <div className={`solo-task-drawer ${!taskDrawerOpen ? 'collapsed' : ''}`}>
        <TaskList
          tasks={soloTasks}
          activeThreadId={soloActiveTask?.id || null}
          onSelectTask={onSelectTask}
          onCreateTask={onCreateTask}
          onDeleteTask={onDeleteTask}
          isRunning={isRunning}
        />
      </div>

      {/* 消息区 */}
      <div className="solo-mode-content">
        <div className="thread-view">
          {/* 消息区域 */}
          <div className="thread-messages">
            {!hasContent ? (
              <div className="solo-mode-welcome">
                <RobotOutlined className="solo-mode-welcome-icon" />
                <Title level={3} type="secondary" className="solo-mode-welcome-title">
                  开始您的开发任务
                </Title>
                <Text type="secondary" className="solo-mode-welcome-desc">
                  在下方输入您的需求，全栈工程师将协助您完成开发
                </Text>
              </div>
            ) : (
              <ChatMessageList
                messages={messages}
                agentConfigs={agentConfigs}
                projectPath={projectPath}
                toolEvents={toolEvents}
                onStopAgent={onStopAgent}
                onRetryAgent={onRetryAgent}
                onOpenCodePanel={onOpenCodePanel}
                autoScroll={true}
              />
            )}
          </div>
        </div>
      </div>

      {/* Solo 模式下的右侧面板 */}
      {rightPanelVisible && (
        <>
          <div
            className={`resize-handle ${isResizing ? 'resizing' : ''}`}
            onMouseDown={onResizeStart}
            style={{ width: isResizing ? 3 : 6 }}
          />
          <div style={{ position: 'relative', display: 'flex' }}>
            {isResizing && <div className="resize-overlay" />}
            <RightPanel
              visible={rightPanelVisible}
              onClose={onClosePanel}
              activeTab={rightPanelActiveTab}
              onTabChange={() => {}}
              codeFiles={codeFiles}
              expandedFiles={expandedFiles}
              onToggleFile={onToggleFile}
              sandboxServer={sandboxServer as any}
              sandboxLoading={sandboxLoading}
              dockerAvailable={dockerAvailable}
              hasProjectPath={Boolean(projectPath)}
              isDebugMode={true}
              onRunSandbox={onRunSandbox}
              onStopSandbox={onStopSandbox}
              width={rightPanelWidth}
            />
          </div>
        </>
      )}

      {/* Solo 模式下的状态栏 */}
      <StatusPanel width={320} threadId={threadId} />
    </div>
  );
});

SoloModeContainer.displayName = 'SoloModeContainer';