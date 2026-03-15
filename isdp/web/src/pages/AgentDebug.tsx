import React, { useState, useEffect, useRef, useCallback } from 'react';
import { Input, Button, message, Typography, Space, Empty, Tooltip, Tag } from 'antd';
import { SendOutlined, StopOutlined, ReloadOutlined, ExpandOutlined, FolderOutlined, LoadingOutlined, ArrowLeftOutlined, CloudServerOutlined, DesktopOutlined } from '@ant-design/icons';
import { useParams, useNavigate } from 'react-router-dom';
import api from '@/api/client';
import { useWebSocket } from '@/hooks/useWebSocket';
import type { AgentConfig } from '@/types';

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

// 辅助函数：等待 WebSocket 连接成功
function waitForWsConnect(threadId: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/v1/ws?threadId=${threadId}`;

    console.log('[WS-Helper] Connecting to:', wsUrl);
    const ws = new WebSocket(wsUrl);
    let done = false;

    ws.onopen = () => {
      if (!done) {
        done = true;
        console.log('[WS-Helper] Connected');
        ws.close();
        resolve();
      }
    };

    ws.onerror = (err) => {
      if (!done) {
        done = true;
        console.error('[WS-Helper] Error:', err);
        reject(err);
      }
    };

    setTimeout(() => {
      if (!done) {
        done = true;
        ws.close();
        reject(new Error('WebSocket timeout'));
      }
    }, 5000);
  });
}

const AgentDebugPage: React.FC = () => {
  const { agentId } = useParams<{ agentId: string }>();
  const navigate = useNavigate();

  const [agent, setAgent] = useState<AgentConfig | null>(null);
  const [input, setInput] = useState('');
  const [output, setOutput] = useState('');
  const [loading, setLoading] = useState(false);
  const [projectPath, setProjectPath] = useState('');
  const [sandboxServer, setSandboxServer] = useState<SandboxServer | null>(null);
  const [sandboxLoading, setSandboxLoading] = useState(false);
  const [currentThreadId, setCurrentThreadId] = useState<string | null>(null);
  const [invocationStatus, setInvocationStatus] = useState<string>('idle');
  const [dockerAvailable, setDockerAvailable] = useState<boolean>(false);
  const outputRef = useRef<HTMLPreElement>(null);

  // 加载Agent信息
  useEffect(() => {
    if (agentId) {
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

  // WebSocket 消息处理
  const handleWSMessage = useCallback((msg: { type: string; payload: Record<string, unknown> }) => {
    console.log('[AgentDebug] Received WS message:', msg.type);
    if (msg.type === 'agent_output_chunk') {
      const chunk = msg.payload.chunk as string;
      if (chunk) {
        setOutput(prev => prev + chunk);
      }
    } else if (msg.type === 'agent_status') {
      const status = msg.payload.status as string;
      setInvocationStatus(status);
      if (status === 'completed' || status === 'failed' || status === 'cancelled') {
        setLoading(false);
        if (status === 'completed') {
          setOutput(prev => prev + '\n✅ 执行完成\n');
        } else if (status === 'failed') {
          setOutput(prev => prev + '\n❌ 执行失败\n');
        } else if (status === 'cancelled') {
          setOutput(prev => prev + '\n⚠️ 执行已取消\n');
        }
      }
    }
  }, []);

  // WebSocket 连接
  const { connected: wsConnected } = useWebSocket({
    threadId: currentThreadId,
    onMessage: handleWSMessage,
  });

  // 发送调试请求
  const handleSend = async () => {
    if (!input.trim() || !agent) {
      return;
    }

    const currentInput = input;
    setInput('');
    setOutput(prev => prev + `\n> ${currentInput}\n`);
    setLoading(true);

    try {
      if (currentThreadId && wsConnected) {
        // 已有会话且WebSocket已连接，继续发送消息
        setOutput(prev => prev + `\n⏳ 继续会话...\n`);
        await api.agents.continueDebug(currentThreadId, currentInput);
      } else {
        // 新会话：先创建 thread，等待 WebSocket 连接，再调用 debug
        setOutput(prev => prev + `\n⏳ 创建会话...\n`);

        // 1. 创建 thread
        const threadResult = await api.agents.createDebugThread(projectPath || undefined);
        const threadId = threadResult.threadId;
        console.log('[AgentDebug] Created thread:', threadId);

        // 2. 设置 threadId，触发 WebSocket 连接
        setCurrentThreadId(threadId);

        // 3. 等待 WebSocket 连接成功
        setOutput(prev => prev + `\n⏳ 等待连接...\n`);
        await waitForWsConnect(threadId);
        console.log('[AgentDebug] WebSocket connected');

        // 4. 调用 debug API
        setOutput(prev => prev + `\n⏳ 启动Agent...\n`);
        setInvocationStatus('running');

        const result = await api.agents.debug(agent.id, currentInput, projectPath || undefined, threadId);
        console.log('[AgentDebug] Debug started:', result);
      }
    } catch (error: any) {
      setLoading(false);
      setOutput(prev => prev + `\n错误: ${error.message || '请求失败'}\n`);
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
      // threadId 是可选的，用于关联会话；没有会话时不传
      const server = await api.sandbox.runProject(currentThreadId || undefined, projectPath, mode);
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
    setCurrentThreadId(null);
    setOutput('');
    setInvocationStatus('idle');
    setLoading(false);
  };

  const getStatusTag = () => {
    const statusColors: Record<string, string> = {
      idle: 'default',
      pending: 'default',
      running: 'processing',
      completed: 'success',
      failed: 'error',
      cancelled: 'warning',
    };

    const statusTexts: Record<string, string> = {
      idle: '空闲',
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
        {currentThreadId && (
          <Tag color="blue">会话中</Tag>
        )}
      </Space>
    );
  };

  return (
    <div style={{ height: 'calc(100vh - 120px)', display: 'flex', flexDirection: 'column' }}>
      {/* 顶部标题栏 */}
      <div style={{ padding: '12px 16px', borderBottom: '1px solid #f0f0f0', display: 'flex', alignItems: 'center', gap: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/agents')}>
          返回
        </Button>
        <Title level={4} style={{ margin: 0 }}>
          调试: {agent?.name || 'Agent'}
        </Title>
        {getStatusTag()}
        {currentThreadId && (
          <Button size="small" onClick={handleNewSession}>
            新会话
          </Button>
        )}
      </div>

      {/* 主内容区 - 左右对半分 */}
      <div style={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
        {/* 左侧：输入输出 */}
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', borderRight: '1px solid #f0f0f0' }}>
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
                  placeholder={currentThreadId ? "输入回复消息..." : "输入测试消息..."}
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

          <div style={{ flex: 1, overflow: 'auto', padding: 12 }}>
            <pre
              ref={outputRef}
              style={{
                margin: 0,
                padding: 12,
                background: '#f5f5f5',
                borderRadius: 8,
                minHeight: '100%',
                fontSize: 12,
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
              }}
            >
              {output || '暂无输出，请输入测试消息...'}
            </pre>
          </div>
        </div>

        {/* 右侧：沙箱预览 */}
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
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