import React, { useState, useEffect, useRef, useCallback } from 'react';
import { Input, Button, message, Typography, Space, Empty, Tooltip, Tag } from 'antd';
import { SendOutlined, StopOutlined, ReloadOutlined, ExpandOutlined, FolderOutlined, LoadingOutlined, ArrowLeftOutlined, CloudServerOutlined, DesktopOutlined } from '@ant-design/icons';
import { useParams, useNavigate } from 'react-router-dom';
import api from '@/api/client';
import { useWebSocket } from '@/hooks/useWebSocket';
import { useDebugThreadStore } from '@/store/debugThread';
import { MessageCard } from '@/components/thread';
import type { AgentConfig, Message, MessageContentBlock } from '@/types';
import './AgentDebug.css';

const { Text, Title } = Typography;

interface SandboxServer {
  id: string;
  threadId: string;
  projectPath: string;
  mode: string;
  port: number;
  url: string;
  status: string;
  containerId?: string;
}

const AgentDebugPage: React.FC = () => {
  const { agentId } = useParams<{ agentId: string }>();
  const navigate = useNavigate();

  const [agent, setAgent] = useState<AgentConfig | null>(null);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [sandboxServer, setSandboxServer] = useState<SandboxServer | null>(null);
  const [sandboxLoading, setSandboxLoading] = useState(false);
  const [dockerAvailable, setDockerAvailable] = useState<boolean>(false);

  // 使用 Zustand store
  const {
    threadId,
    messages,
    streamingContent,
    status,
    projectPath,
    setThreadId,
    addMessage,
    appendStreamChunk,
    clearStreamContent,
    setStatus,
    setSandboxUrl: setStoreSandboxUrl,
    setProjectPath,
    clearAll,
  } = useDebugThreadStore();

  const outputRef = useRef<HTMLDivElement>(null);
  const threadIdRef = useRef<string | null>(null);

  // 同步 threadId 到 ref
  useEffect(() => {
    threadIdRef.current = threadId;
  }, [threadId]);

  // 加载Agent信息
  useEffect(() => {
    if (agentId) {
      // 切换角色时清除旧的调试会话
      clearAll();
      api.agents.get(agentId).then(setAgent).catch(err => {
        message.error('加载Agent失败');
        console.error(err);
      });
    }
  }, [agentId]);

  // 检查Docker可用性
  useEffect(() => {
    api.sandbox.checkDocker().then(res => {
      setDockerAvailable(res.available);
    }).catch(() => {
      setDockerAvailable(false);
    });
  }, []);

  // 自动滚动到底部
  useEffect(() => {
    if (outputRef.current) {
      outputRef.current.scrollTop = outputRef.current.scrollHeight;
    }
  }, [messages, streamingContent]);

  // WebSocket 消息处理 - 使用 ref 避免不必要的依赖更新
  const handleWSMessage = useCallback((msg: { type: string; payload: Record<string, unknown>; timestamp?: number }) => {
    console.log('[AgentDebug] Received WS message:', msg.type);

    switch (msg.type) {
      case 'agent_output_chunk':
        const chunk = msg.payload.chunk as string;
        if (chunk) {
          appendStreamChunk(chunk);
        }
        break;

      case 'agent_message':
        clearStreamContent();
        const agentMsg: Message = {
          id: (msg.payload.messageId as string) || Date.now().toString(),
          threadId: threadIdRef.current || '',
          role: 'agent',
          agentId: msg.payload.agentId as string,
          agentName: msg.payload.agentName as string,
          content: msg.payload.content as string,
          contentBlocks: msg.payload.contentBlocks as MessageContentBlock[] | undefined,
          messageType: 'text',
          createdAt: msg.timestamp ? new Date(msg.timestamp * 1000).toISOString() : new Date().toISOString(),
        };
        addMessage(agentMsg);
        setStatus('idle');
        setLoading(false);
        break;

      case 'system_message':
        const sysMsg: Message = {
          id: Date.now().toString(),
          threadId: threadIdRef.current || '',
          role: 'system',
          content: msg.payload.content as string,
          messageType: 'text',
          createdAt: new Date().toISOString(),
        };
        addMessage(sysMsg);
        break;

      case 'agent_status':
        const status = msg.payload.status as string;
        setStatus(status === 'running' ? 'running' : status === 'completed' ? 'completed' : status === 'failed' ? 'error' : 'idle');
        if (status === 'completed' || status === 'failed' || status === 'cancelled') {
          setLoading(false);
        }
        break;

      case 'sandbox_ready':
        setStoreSandboxUrl(msg.payload.url as string);
        break;

      case 'thread_expired':
        clearAll();
        message.warning('调试会话已过期，请重新开始');
        break;
    }
  // 注意：threadIdRef 是 ref，不需要作为依赖
  // store 方法是稳定的，不需要作为依赖
  }, [appendStreamChunk, clearStreamContent, addMessage, setStatus, setStoreSandboxUrl, clearAll]);

  // WebSocket 连接
  const { connected: wsConnected } = useWebSocket(threadId, {
    onMessage: handleWSMessage,
  });

  // 等待 WebSocket 连接的 Promise
  const wsConnectedRef = useRef(false);
  const waitForConnection = useCallback((timeout = 5000): Promise<void> => {
    return new Promise((resolve, reject) => {
      const startTime = Date.now();
      const check = () => {
        if (wsConnectedRef.current) {
          resolve();
        } else if (Date.now() - startTime > timeout) {
          reject(new Error('WebSocket connection timeout'));
        } else {
          setTimeout(check, 100);
        }
      };
      check();
    });
  }, []);

  // 同步 wsConnected 到 ref
  useEffect(() => {
    wsConnectedRef.current = wsConnected;
  }, [wsConnected]);

  // 发送调试请求
  const handleSend = async () => {
    if (!input.trim() || !agent) {
      return;
    }

    const currentInput = input;
    setInput('');
    setLoading(true);

    // 添加用户消息
    const userMsg: Message = {
      id: Date.now().toString(),
      threadId: threadId || '',
      role: 'user',
      content: currentInput,
      messageType: 'text',
      createdAt: new Date().toISOString(),
    };
    addMessage(userMsg);
    setStatus('running');

    try {
      if (threadId && wsConnected) {
        // 已有会话且WebSocket已连接，继续发送消息
        await api.agents.continueDebug(threadId, currentInput);
      } else {
        // 新会话：先创建 thread，等待 WebSocket 连接，再调用 debug
        // 1. 创建 thread
        const threadResult = await api.agents.createDebugThread(projectPath || undefined);
        const newThreadId = threadResult.threadId;
        console.log('[AgentDebug] Created thread:', newThreadId);

        // 2. 设置 threadId，触发 WebSocket 连接
        setThreadId(newThreadId);

        // 3. 等待 WebSocket 连接成功（通过轮询 wsConnected ref）
        await waitForConnection();
        console.log('[AgentDebug] WebSocket connected');

        // 4. 调用 debug API
        const result = await api.agents.debug(agent.id, currentInput, projectPath || undefined, newThreadId);
        console.log('[AgentDebug] Debug started:', result);
      }
    } catch (error: any) {
      setLoading(false);
      setStatus('error');
      const errorMsg: Message = {
        id: Date.now().toString(),
        threadId: threadIdRef.current || '',
        role: 'system',
        content: `错误: ${error.message || '请求失败'}`,
        messageType: 'text',
        createdAt: new Date().toISOString(),
      };
      addMessage(errorMsg);
      console.error('[AgentDebug] Error:', error);
    }
  };

  const handleRunToSandbox = async (mode: 'local' | 'docker') => {
    if (!agent) return;

    if (!projectPath.trim()) {
      message.warning('请先输入工作目录');
      return;
    }

    if (mode === 'docker' && !dockerAvailable) {
      message.warning('Docker不可用，请确保Docker已启动');
      return;
    }

    setSandboxLoading(true);
    try {
      const server = await api.sandbox.runProject(threadId || undefined, projectPath, mode);
      setSandboxServer(server);
      message.success(`项目已在${mode === 'docker' ? '容器' : '本地'}沙箱中启动`);
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

  const handleNewSession = () => {
    clearAll();
    setLoading(false);
  };

  const getStatusTag = () => {
    const statusColors: Record<string, string> = {
      idle: 'default',
      running: 'processing',
      completed: 'success',
      error: 'error',
    };

    const statusTexts: Record<string, string> = {
      idle: '空闲',
      running: '执行中',
      completed: '已完成',
      error: '失败',
    };

    return (
      <Space>
        <Tag color={statusColors[status] || 'default'} icon={loading ? <LoadingOutlined spin /> : undefined}>
          {statusTexts[status] || status}
        </Tag>
        {wsConnected && (
          <Tag color="green">实时输出</Tag>
        )}
        {threadId && (
          <Tag color="blue">会话中</Tag>
        )}
      </Space>
    );
  };

  return (
    <div className="agent-debug">
      {/* 顶部标题栏 */}
      <div className="debug-header">
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/agents')}>
          返回
        </Button>
        <Title level={4} style={{ margin: 0 }}>
          调试: {agent?.name || 'Agent'}
        </Title>
        {getStatusTag()}
        {threadId && (
          <Button size="small" onClick={handleNewSession}>
            新会话
          </Button>
        )}
      </div>

      {/* 主内容区 - 左右对半分 */}
      <div className="debug-content">
        {/* 左侧：消息列表 */}
        <div className="left-panel">
          <div style={{ padding: 12, borderBottom: '1px solid #f0f0f0' }}>
            <Space direction="vertical" style={{ width: '100%' }} size="small">
              <Input
                placeholder="工作目录（如: D:/projects/my-app）"
                value={projectPath}
                onChange={e => setProjectPath(e.target.value)}
                prefix={<FolderOutlined />}
                style={{ marginBottom: 8 }}
              />
              <Space.Compact style={{ width: '100%' }}>
                <Input
                  placeholder={threadId ? "输入回复消息..." : "输入测试消息..."}
                  value={input}
                  onChange={e => setInput(e.target.value)}
                  onPressEnter={handleSend}
                  disabled={loading}
                />
                <Button type="primary" icon={<SendOutlined />} onClick={handleSend} disabled={loading}>
                  发送
                </Button>
                <Tooltip title="在本地进程沙箱中运行项目并预览">
                  <Button
                    icon={<DesktopOutlined />}
                    onClick={() => handleRunToSandbox('local')}
                    loading={sandboxLoading}
                  >
                    运行到本地沙箱
                  </Button>
                </Tooltip>
                <Tooltip title={dockerAvailable ? "在Docker容器沙箱中运行项目并预览" : "Docker不可用"}>
                  <Button
                    icon={<CloudServerOutlined />}
                    onClick={() => handleRunToSandbox('docker')}
                    loading={sandboxLoading}
                    disabled={!dockerAvailable}
                  >
                    运行到容器沙箱
                  </Button>
                </Tooltip>
              </Space.Compact>
            </Space>
          </div>

          <div className="messages-area" ref={outputRef}>
            {messages.length === 0 && !streamingContent ? (
              <div style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                height: '100%',
                color: '#999'
              }}>
                <Empty
                  description="暂无消息，请输入测试消息..."
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                />
              </div>
            ) : (
              <>
                {messages.map((msg, idx) => (
                  <MessageCard
                    key={msg.id || idx}
                    message={msg}
                    agentName={msg.agentName || msg.agentId}
                    isStreaming={false}
                  />
                ))}
                {streamingContent && (
                  <MessageCard
                    message={{
                      id: 'streaming',
                      threadId: threadId || '',
                      role: 'agent',
                      content: streamingContent,
                      messageType: 'text',
                      createdAt: new Date().toISOString(),
                    }}
                    isStreaming={true}
                  />
                )}
              </>
            )}
          </div>
        </div>

        {/* 右侧：沙箱预览 */}
        <div className="right-panel">
          <div style={{ padding: '8px 12px', borderBottom: '1px solid #f0f0f0', background: '#fafafa' }}>
            <Space>
              <Text strong>沙箱预览</Text>
              {sandboxServer && (
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {sandboxServer.url}
                </Text>
              )}
            </Space>
          </div>

          <div style={{ padding: 8, borderBottom: '1px solid #f0f0f0' }}>
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

          <div style={{ flex: 1, position: 'relative', background: '#fff' }}>
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
              <div style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                height: '100%',
                color: '#999'
              }}>
                <Empty
                  description="点击'运行到沙箱'启动项目预览"
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                />
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default AgentDebugPage;