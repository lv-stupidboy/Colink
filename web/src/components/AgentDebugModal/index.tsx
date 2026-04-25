import React, { useState, useEffect, useRef, useCallback } from 'react';
import { Input, Button, message, Typography, Space, Empty, Tooltip, Tag, Modal } from 'antd';
import { SendOutlined, PlayCircleOutlined, StopOutlined, ReloadOutlined, ExpandOutlined, FolderOutlined, LoadingOutlined } from '@ant-design/icons';
import api from '@/api/client';
import { useWebSocket } from '@/hooks/useWebSocket';
import type { AgentConfig } from '@/types';
import './index.css';

const { Text } = Typography;

interface AgentDebugModalProps {
  open: boolean;
  agent: AgentConfig | null;
  onClose: () => void;
}

interface SandboxServer {
  id: string;
  threadId: string;
  projectPath: string;
  port: number;
  url: string;
  status: string;
}

const AgentDebugModal: React.FC<AgentDebugModalProps> = ({ open, agent, onClose }) => {
  const [input, setInput] = useState('');
  const [output, setOutput] = useState('');
  const [loading, setLoading] = useState(false);
  const [projectPath, setProjectPath] = useState('');
  const [sandboxServer, setSandboxServer] = useState<SandboxServer | null>(null);
  const [sandboxLoading, setSandboxLoading] = useState(false);
  const [currentInvocationId, setCurrentInvocationId] = useState<string | null>(null);
  const [invocationStatus, setInvocationStatus] = useState<string>('pending');
  const [debugThreadId, setDebugThreadId] = useState<string | null>(null);
  const outputRef = useRef<HTMLPreElement>(null);
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // WebSocket 消息处理
  const handleWSMessage = useCallback((msg: { type: string; payload: Record<string, unknown> }) => {
    if (msg.type === 'agent_output_chunk') {
      // 实时输出块
      const chunk = msg.payload.chunk as string;
      const invocationId = msg.payload.invocationId as string;
      if (chunk && invocationId === currentInvocationId) {
        setOutput(prev => prev + chunk);
      }
    } else if (msg.type === 'agent_status') {
      const status = msg.payload.status as string;
      const invocationId = msg.payload.invocationId as string;
      if (invocationId === currentInvocationId) {
        setInvocationStatus(status);
        if (status === 'completed' || status === 'failed' || status === 'cancelled') {
          setLoading(false);
          stopPolling();
          if (status === 'failed') {
            // 展示详细错误信息（来自 errorDetails）
            const errorDetails = msg.payload.errorDetails as string;
            setOutput(prev => prev + `\n${errorDetails || '执行失败'}\n`);
          } else if (status === 'cancelled') {
            setOutput(prev => prev + '\n执行已取消\n');
          }
        }
      }
    }
  }, [currentInvocationId]);

  // WebSocket 连接
  const { connected: wsConnected } = useWebSocket(debugThreadId, {
    onMessage: handleWSMessage,
  });

  // 清理轮询
  const stopPolling = useCallback(() => {
    if (pollingRef.current) {
      clearInterval(pollingRef.current);
      pollingRef.current = null;
    }
  }, []);

  // 轮询获取调用状态
  const pollInvocationStatus = useCallback(async (invocationId: string) => {
    try {
      const invocation = await api.invocations.get(invocationId);
      setInvocationStatus(invocation.status);

      if (invocation.status === 'completed') {
        stopPolling();
        setLoading(false);
        setCurrentInvocationId(null);
        if (invocation.output) {
          setOutput(prev => prev + `\n${invocation.output}\n`);
        }
      } else if (invocation.status === 'failed' || invocation.status === 'cancelled') {
        stopPolling();
        setLoading(false);
        setCurrentInvocationId(null);
        setOutput(prev => prev + `\n执行${invocation.status === 'failed' ? '失败' : '已取消'}: ${invocation.output || '未知错误'}\n`);
      }
    } catch (error: any) {
      console.error('Failed to poll invocation status:', error);
    }
  }, [stopPolling]);

  // 开始轮询
  const startPolling = useCallback((invocationId: string) => {
    setCurrentInvocationId(invocationId);
    setInvocationStatus('pending');

    // 立即检查一次
    pollInvocationStatus(invocationId);

    // 每500ms轮询一次，更实时
    pollingRef.current = setInterval(() => {
      pollInvocationStatus(invocationId);
    }, 500);
  }, [pollInvocationStatus]);

  // 清理
  useEffect(() => {
    return () => {
      stopPolling();
    };
  }, [stopPolling]);

  useEffect(() => {
    if (open && agent) {
      setOutput('');
      setInput('');
      setProjectPath('');
      setSandboxServer(null);
      stopPolling();
      setLoading(false);
      setCurrentInvocationId(null);
      setInvocationStatus('pending');
      // 为调试创建一个临时 threadId（使用固定前缀）
      setDebugThreadId(`debug-${agent.id}`);
    }
  }, [open, agent, stopPolling]);

  const handleSend = async () => {
    if (!input.trim() || !agent) {
      return;
    }

    setLoading(true);
    setOutput(prev => prev + `\n> ${input}\n`);
    const currentInput = input;
    setInput('');

    try {
      const result = await api.agents.debug(agent.id, currentInput, projectPath || undefined);

      if (result.invocationId) {
        setCurrentInvocationId(result.invocationId);
        // 显示执行中状态
        setOutput(prev => prev + `\n⏳ Agent正在执行...\n`);

        // 如果 WebSocket 未连接，使用轮询作为后备
        if (!wsConnected) {
          startPolling(result.invocationId);
        }
      } else if (result.output) {
        // 如果直接返回了输出（向后兼容）
        setLoading(false);
        setOutput(prev => prev + `\n${result.output}\n`);
      }
    } catch (error: any) {
      setLoading(false);
      setOutput(prev => prev + `\n错误: ${error.message || '请求失败'}\n`);
    }
  };

  const handleRunToSandbox = async () => {
    if (!agent) return;

    if (!projectPath.trim()) {
      message.warning('请先输入工作目录');
      return;
    }

    setSandboxLoading(true);
    try {
      // 先运行项目
      const server = await api.sandbox.runProject('debug-thread', projectPath);
      setSandboxServer(server);
      message.success('项目已启动');
    } catch (error: any) {
      message.error(`启动失败: ${error.message || '未知错误'}`);
    } finally {
      setSandboxLoading(false);
    }
  };

  const handleStopSandbox = async () => {
    if (!sandboxServer) return;

    try {
      await api.sandbox.stopServer(sandboxServer.id);
      setSandboxServer(null);
      message.success('已停止');
    } catch (error: any) {
      message.error('停止失败');
    }
  };

  const handleRefreshPreview = () => {
    const iframe = document.querySelector('#sandbox-preview') as HTMLIFrameElement;
    if (iframe) {
      iframe.src = iframe.src;
    }
  };

  const handleOpenInNewWindow = () => {
    if (sandboxServer?.url) {
      window.open(sandboxServer.url, '_blank');
    }
  };

  // 取消当前执行
  const handleCancelExecution = async () => {
    if (!currentInvocationId) return;

    try {
      await api.invocations.cancel(currentInvocationId);
      stopPolling();
      setLoading(false);
      setCurrentInvocationId(null);
      setOutput(prev => prev + '\n⚠️ 执行已取消\n');
    } catch (error: any) {
      message.error('取消失败');
    }
  };

  // 获取状态标签
  const getStatusTag = () => {
    if (!loading && !currentInvocationId) return null;

    const statusColors: Record<string, string> = {
      pending: 'default',
      running: 'processing',
      completed: 'success',
      failed: 'error',
      cancelled: 'warning',
    };

    const statusTexts: Record<string, string> = {
      pending: '等待中',
      running: '执行中',
      completed: '已完成',
      failed: '失败',
      cancelled: '已取消',
    };

    return (
      <Space>
        <Tag color={statusColors[invocationStatus] || 'default'} icon={loading ? <LoadingOutlined spin /> : undefined}>
          {statusTexts[invocationStatus] || invocationStatus}
        </Tag>
        {wsConnected && (
          <Tag color="green">实时输出</Tag>
        )}
      </Space>
    );
  };

  return (
    <Modal
      title={`调试: ${agent?.name || 'Agent'}`}
      open={open}
      onCancel={onClose}
      width={1400}
      footer={null}
      styles={{
        body: { height: 700, padding: 0 }
      }}
    >
      <div className="agent-debug-modal">
        {/* 左侧：输入输出 */}
        <div className="agent-debug-modal-left">
          <div className="agent-debug-modal-header">
            <Space direction="vertical" style={{ width: '100%' }} size="small">
              <Input
                placeholder="工作目录（可选，如: D:/projects/my-app）"
                value={projectPath}
                onChange={e => setProjectPath(e.target.value)}
                prefix={<FolderOutlined />}
                style={{ marginBottom: 8 }}
              />
              <Space.Compact style={{ width: '100%' }}>
                <Input
                  placeholder="输入测试消息..."
                  value={input}
                  onChange={e => setInput(e.target.value)}
                  onPressEnter={handleSend}
                  disabled={loading}
                />
                {loading && currentInvocationId ? (
                  <Button danger icon={<StopOutlined />} onClick={handleCancelExecution}>
                    取消
                  </Button>
                ) : (
                  <Button type="primary" icon={<SendOutlined />} onClick={handleSend} disabled={loading}>
                    发送
                  </Button>
                )}
                <Tooltip title="在沙箱中运行项目并预览">
                  <Button icon={<PlayCircleOutlined />} onClick={handleRunToSandbox} loading={sandboxLoading}>
                    运行到沙箱
                  </Button>
                </Tooltip>
              </Space.Compact>
              {getStatusTag() && (
                <div style={{ marginTop: 8 }}>
                  {getStatusTag()}
                </div>
              )}
            </Space>
          </div>

          <div className="agent-debug-modal-output">
            <pre
              ref={outputRef}
              className="agent-debug-modal-pre"
            >
              {output || '暂无输出，请输入测试消息...'}
            </pre>
          </div>
        </div>

        {/* 右侧：沙箱预览 */}
        <div className="agent-debug-modal-right">
          <div className="agent-debug-modal-preview-header">
            <Space>
              <Text strong>沙箱预览</Text>
              {sandboxServer && (
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {sandboxServer.url}
                </Text>
              )}
            </Space>
          </div>

          <div className="agent-debug-modal-preview-actions">
            <Space>
              <Button
                size="small"
                icon={<ReloadOutlined />}
                onClick={handleRefreshPreview}
                disabled={!sandboxServer}
              >
                刷新
              </Button>
              <Button
                size="small"
                icon={<ExpandOutlined />}
                onClick={handleOpenInNewWindow}
                disabled={!sandboxServer}
              >
                新窗口
              </Button>
              <Button
                size="small"
                icon={<StopOutlined />}
                danger
                onClick={handleStopSandbox}
                disabled={!sandboxServer}
              >
                停止
              </Button>
            </Space>
          </div>

          <div className="agent-debug-modal-preview-content">
            {sandboxServer ? (
              <iframe
                id="sandbox-preview"
                src={sandboxServer.url}
                style={{
                  width: '100%',
                  height: '100%',
                  border: 'none',
                }}
                title="沙箱预览"
              />
            ) : (
              <div className="agent-debug-modal-empty">
                <Empty
                  description="点击'运行到沙箱'启动项目预览"
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                />
              </div>
            )}
          </div>
        </div>
      </div>
    </Modal>
  );
};

export default AgentDebugModal;