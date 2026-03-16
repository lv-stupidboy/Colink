import React, { useEffect, useRef, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Input,
  Button,
  Space,
  Tag,
  Spin,
  message,
  Avatar,
  Tooltip,
  Drawer,
  Modal,
  Card,
  Typography,
  Collapse,
  Alert,
  List,
  Divider,
  Badge,
  Empty,
} from 'antd';
import {
  SendOutlined,
  UserOutlined,
  RobotOutlined,
  ArrowLeftOutlined,
  FileTextOutlined,
  CodeOutlined,
  FileOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  PaperClipOutlined,
  FileSearchOutlined,
  StopOutlined,
  ReloadOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
} from '@ant-design/icons';
import { useAppStore } from '@/store';
import type { Message, Artifact, ReviewIssue, MergeCheckResult, AgentRole } from '@/types';
import { PhaseLabels, PhaseColors, AgentRoleLabels, ArtifactTypeLabels } from '@/types';
import { InterventionControls } from '@/components/InterventionControls';
import { ReviewReport } from '@/components/ReviewReport';
import FileTree from '@/components/FileTree';
import api from '@/api/client';
import './ThreadView.css';

const { TextArea } = Input;
const { Title, Text } = Typography;
const { Panel } = Collapse;

/**
 * 开发工作台 (ThreadView)
 * PRD Section 2.1.3 - 开发工作台设计
 *
 * 界面组成：
 * - 顶部进度条（可折叠）
 * - 对话消息区（统一消息流）
 * - 侧边快捷面板（产物列表）
 * - 底部控制栏
 */
const ThreadView: React.FC = () => {
  const { threadId, projectId } = useParams<{ threadId: string; projectId: string }>();
  const navigate = useNavigate();
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const inputRef = useRef<any>(null);

  const {
    currentThread,
    messages,
    streamingMessages,
    activeAgents,
    loading,
    wsConnected,
    loadThread,
    sendMessage,
    spawnAgent,
    setWsConnected,
    addMessage,
    updateAgentStatus,
    updateStreamingMessage,
    finalizeStreamingMessage,
    loadingProjectContext,
    loadProjectContext,
    loadWorkflowTemplate,
    clearProjectContext,
    getFilteredAgents,
    loadAgentConfigs,
  } = useAppStore();

  const [inputValue, setInputValue] = useState('');
  const [artifactDrawerVisible, setArtifactDrawerVisible] = useState(false);
  const [artifacts, setArtifacts] = useState<Artifact[]>([]);
  const [reviewResult, setReviewResult] = useState<MergeCheckResult | undefined>();
  const [reviewIssues, setReviewIssues] = useState<ReviewIssue[]>([]);
  const [checkpointModalVisible, setCheckpointModalVisible] = useState(false);
  const [currentCheckpoint, setCurrentCheckpoint] = useState<{
    type: 'requirement' | 'design' | 'review' | 'deploy';
    title: string;
    content: string;
  } | null>(null);
  const [mentionListVisible, setMentionListVisible] = useState(false);
  const [mentionFilter, setMentionFilter] = useState('');
  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);
  const [fileSidebarVisible, setFileSidebarVisible] = useState(true);

  useEffect(() => {
    if (threadId) {
      // 连接新的 WebSocket 前先清空旧状态
      // 注意：loadThread 已经会清空状态，这里确保 WebSocket 切换时的隔离
      loadThread(threadId);
      connectWebSocket(threadId);
      loadArtifacts(threadId);
      loadReviewData(threadId);
    }

    return () => {
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [threadId]);

  // Load agent configs for @mention dropdown
  useEffect(() => {
    loadAgentConfigs();
  }, [loadAgentConfigs]);

  // Load workflow template when thread is loaded
  // Priority: Thread.workflowTemplateId > Project.workflowTemplateId
  useEffect(() => {
    const loadWorkflowContext = async () => {
      if (currentThread?.workflowTemplateId) {
        // Thread has workflowTemplateId directly
        await loadWorkflowTemplate(currentThread.workflowTemplateId);
      } else if (currentThread?.projectId) {
        // Fallback: load from project
        await loadProjectContext(currentThread.projectId);
      }
    };

    loadWorkflowContext();

    return () => {
      clearProjectContext();
    };
  }, [currentThread?.workflowTemplateId, currentThread?.projectId, loadWorkflowTemplate, loadProjectContext, clearProjectContext]);

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  // 流式消息更新时也滚动
  useEffect(() => {
    scrollToBottom();
  }, [streamingMessages]);

  const connectWebSocket = (id: string) => {
    const wsUrl = `ws://${window.location.host}/api/v1/ws?threadId=${id}`;
    wsRef.current = new WebSocket(wsUrl);

    wsRef.current.onopen = () => {
      setWsConnected(true);
      console.log('WebSocket connected');
    };

    wsRef.current.onclose = () => {
      setWsConnected(false);
      console.log('WebSocket disconnected');
    };

    wsRef.current.onmessage = (event) => {
      const data = JSON.parse(event.data);
      handleWsMessage(data);
    };
  };

  const handleWsMessage = (data: { type: string; threadId?: string; payload: Record<string, unknown> }) => {
    // 验证消息是否属于当前 thread，防止跨 thread 数据泄露
    const currentThreadId = useAppStore.getState().currentThread?.id;
    if (data.threadId && currentThreadId && data.threadId !== currentThreadId) {
      console.warn('[WebSocket] Received message for different thread, ignoring:', data.threadId);
      return;
    }

    switch (data.type) {
      case 'agent_output_chunk':
        // 流式输出块：实时追加内容
        updateStreamingMessage(
          data.payload.invocationId as string,
          data.payload.chunk as string,
          data.payload.agentId as string || '',
          data.payload.agentName as string
        );
        break;
      case 'agent_message':
        // Agent 完成消息（非流式场景备用）：清除流式缓存，显示最终消息
        // 注意：流式场景下不会收到此消息，由 agent_status/completed 触发 finalizeStreamingMessage
        const invocationId = data.payload.invocationId as string || data.payload.messageId as string;
        // 使用 getState() 避免闭包陷阱
        const currentStreaming = useAppStore.getState().streamingMessages;
        if (invocationId && currentStreaming[invocationId]) {
          finalizeStreamingMessage(invocationId);
        } else {
          // 直接添加消息（非流式场景）
          addMessage({
            id: data.payload.messageId as string,
            threadId: threadId!,
            role: 'agent',
            agentId: data.payload.agentId as string,
            content: data.payload.content as string,
            messageType: 'text',
            metadata: {
              agentName: data.payload.agentName,
              agentRole: data.payload.agentRole,
            },
            createdAt: new Date().toISOString(),
          });
        }
        break;
      case 'system_message':
        addMessage({
          id: `sys-${Date.now()}`,
          threadId: threadId!,
          role: 'system',
          content: data.payload.content as string,
          messageType: 'command',
          metadata: data.payload.metadata as Record<string, unknown>,
          createdAt: new Date().toISOString(),
        });
        // 检查是否需要人工确认
        if (data.payload.checkpoint) {
          showCheckpoint(data.payload.checkpoint as string, data.payload as any);
        }
        break;
      case 'artifact_created':
        if (threadId) loadArtifacts(threadId);
        break;
      case 'agent_status':
        const status = data.payload.status as string;
        const invocId = data.payload.invocationId as string;
        updateAgentStatus(invocId, status);
        // Agent 完成时，如果有流式消息缓存，转为正式消息
        if (status === 'completed' || status === 'failed') {
          // 使用 getState() 避免闭包陷阱
          const currentStreaming = useAppStore.getState().streamingMessages;
          if (currentStreaming[invocId]) {
            finalizeStreamingMessage(invocId);
          }
        }
        break;
    }
  };

  const showCheckpoint = (type: string, data: any) => {
    const checkpointConfig: Record<string, { title: string; getContent: (d: any) => string }> = {
      requirement: {
        title: '需求确认',
        getContent: (d) => d.summary || '请确认需求分析结果是否符合预期',
      },
      design: {
        title: '方案确认',
        getContent: (d) => d.summary || '请确认技术方案是否符合预期',
      },
      review: {
        title: '代码审查放行',
        getContent: (d) => d.summary || '请确认代码审查结果',
      },
      deploy: {
        title: '部署确认',
        getContent: (d) => d.summary || '请确认是否可以部署',
      },
    };

    const config = checkpointConfig[type];
    if (config) {
      setCurrentCheckpoint({
        type: type as any,
        title: config.title,
        content: config.getContent(data),
      });
      setCheckpointModalVisible(true);
    }
  };

  const loadArtifacts = async (id: string) => {
    try {
      const data = await api.artifacts.list(id);
      setArtifacts((data as unknown as Artifact[]) || []);
    } catch (error) {
      console.error('Failed to load artifacts:', error);
      setArtifacts([]);
    }
  };

  const loadReviewData = async (id: string) => {
    try {
      const result = await api.merge.check(id);
      setReviewResult(result as unknown as MergeCheckResult);
      setReviewIssues((result as unknown as MergeCheckResult).unresolved || []);
    } catch (error) {
      console.error('Failed to load review data:', error);
    }
  };

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  /**
   * 处理发送消息
   * PRD: 支持 @mention 触发特定 Agent
   */
  const handleSend = async () => {
    if (!inputValue.trim()) return;

    const content = inputValue.trim();
    setInputValue('');
    setMentionListVisible(false);

    // 检查是否是 @mention 命令
    // PRD Section 2.3.1: 行首 @Agent名 触发路由
    const mentionMatch = content.match(/^@(\S+)\s*(.*)/);
    if (mentionMatch) {
      const agentName = mentionMatch[1].toLowerCase();
      const input = mentionMatch[2] || content;

      // 如果有选中的 Agent ID，直接使用 configId 启动
      if (selectedAgentId) {
        // 先发送用户消息（显示用户输入），skipAgentTrigger: true 避免后端重复触发
        await sendMessage(content, true);
        // 然后触发 Agent
        await spawnAgent('custom', input, selectedAgentId);
        setSelectedAgentId(null);
        return;
      }

      // 没有 selectedAgentId 时，尝试通过名称查找 Agent
      const agentByName = agentOptions.find(opt =>
        opt.name.toLowerCase() === agentName ||
        opt.label.toLowerCase() === agentName
      );
      if (agentByName) {
        // 先发送用户消息（显示用户输入），skipAgentTrigger: true 避免后端重复触发
        await sendMessage(content, true);
        // 然后触发 Agent
        await spawnAgent('custom', input, agentByName.id);
        setSelectedAgentId(null);
        return;
      }

      message.warning(`未找到 Agent: ${agentName}，请从下拉列表中选择`);
      setSelectedAgentId(null);
      return;
    } else {
      await sendMessage(content);
      setSelectedAgentId(null);
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  /**
   * 处理输入变化，检测 @ 符号
   */
  const handleInputChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const value = e.target.value;
    setInputValue(value);

    // 检测 @ 符号
    const lastAtIndex = value.lastIndexOf('@');
    if (lastAtIndex >= 0 && lastAtIndex === value.length - 1) {
      setMentionListVisible(true);
      setMentionFilter('');
      setSelectedAgentId(null); // 清除之前选中的 Agent
    } else if (lastAtIndex >= 0 && value.indexOf(' ', lastAtIndex) === -1) {
      setMentionListVisible(true);
      setMentionFilter(value.substring(lastAtIndex + 1).toLowerCase());
      setSelectedAgentId(null); // 清除之前选中的 Agent（用户正在输入新的 @）
    } else {
      setMentionListVisible(false);
    }
  };

  /**
   * 选择 Agent mention
   */
  const selectMention = (agentId: string, _agentRole: AgentRole, label: string) => {
    const lastAtIndex = inputValue.lastIndexOf('@');
    if (lastAtIndex >= 0) {
      setInputValue(inputValue.substring(0, lastAtIndex) + '@' + label + ' ');
    }
    setSelectedAgentId(agentId);
    setMentionListVisible(false);
    inputRef.current?.focus();
  };

  // 干预操作处理
  const handlePause = async () => {
    try {
      if (currentThread?.abortToken) {
        // 调用取消 API
        message.info('正在暂停当前 Agent...');
      }
    } catch (error) {
      message.error('暂停失败');
    }
  };

  const handleResume = () => {
    message.info('继续执行');
  };

  const handleSkip = async () => {
    try {
      await api.threads.updateStatus(threadId!, 'running');
      message.success('已跳过当前任务，继续执行');
    } catch (error) {
      message.error('跳过失败');
    }
  };

  const handleRetry = async () => {
    try {
      await api.threads.updateStatus(threadId!, 'running');
      message.success('正在重试当前任务');
    } catch (error) {
      message.error('重试失败');
    }
  };

  const handleStop = () => {
    Modal.confirm({
      title: '确认终止？',
      content: '终止后将无法恢复当前进度',
      okText: '确认终止',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          if (currentThread?.abortToken) {
            // 调用终止 API
          }
          await api.threads.updateStatus(threadId!, 'failed');
          message.info('已终止任务');
        } catch (error) {
          message.error('终止失败');
        }
      },
    });
  };

  /**
   * 处理终止Agent
   */
  const handleStopAgent = async (invocationId: string) => {
    try {
      await api.invocations.cancel(invocationId);
      message.info('已终止 Agent');
    } catch (error) {
      message.error('终止失败');
    }
  };

  /**
   * 处理重试Agent
   * 使用相同的Agent配置重新触发
   */
  const handleRetryAgent = async (msg: Message) => {
    if (!currentThread) return;

    // 从消息元数据获取 agentId (即 configId)
    const agentId = msg.agentId;
    if (!agentId) {
      message.warning('无法重试：缺少 Agent 配置信息');
      return;
    }

    try {
      // 重新触发该 Agent，让它重新处理
      await spawnAgent('custom', '请重新处理上一次的任务', agentId);
      message.info('已重新触发 Agent');
    } catch (error) {
      message.error('重试失败');
    }
  };

  /**
   * 处理检查点确认
   */
  const handleCheckpointConfirm = async () => {
    try {
      if (currentCheckpoint?.type === 'review') {
        await api.merge.approve(threadId!);
      }
      setCheckpointModalVisible(false);
      message.success('已确认，继续执行');
    } catch (error) {
      message.error('确认失败');
    }
  };

  /**
   * 处理检查点拒绝
   */
  const handleCheckpointReject = () => {
    setCheckpointModalVisible(false);
    message.info('已拒绝，可以进行修改');
  };

  /**
   * 渲染消息
   * PRD: 支持多种消息类型 - 用户消息、Agent消息、系统消息、产物卡片
   */
  const renderMessage = (msg: Message) => {
    // 系统消息
    if (msg.role === 'system') {
      const alertType = (msg.metadata?.alertType as string) || 'info';
      return (
        <div key={msg.id} className="message message-system">
          <div className="system-message-content">
            <Alert
              type={alertType === 'error' ? 'error' : alertType === 'warning' ? 'warning' : 'info'}
              message={msg.metadata?.title as string || '系统消息'}
              description={msg.content}
              showIcon
              banner
            />
          </div>
        </div>
      );
    }

    // Agent 消息 - 可能包含产物卡片
    if (msg.role === 'agent') {
      const hasArtifact = Boolean(msg.metadata?.artifact);
      const hasReview = Boolean(msg.metadata?.reviewReport);

      // 优先使用 metadata 中的 agentName，其次尝试用 agentRole 映射，最后 fallback 到 agentId
      const agentName = (msg.metadata?.agentName as string) ||
        AgentRoleLabels[(msg.metadata?.agentRole as keyof typeof AgentRoleLabels)] ||
        AgentRoleLabels[msg.agentId as keyof typeof AgentRoleLabels] ||
        msg.agentId ||
        'Agent';

      return (
        <div key={msg.id} className="message-container message-container-agent">
          <Avatar
            className="message-avatar"
            icon={<RobotOutlined />}
            style={{ backgroundColor: '#1890ff' }}
          />
          <div className="message message-agent">
            <div className="message-content">
              <div className="message-header">
                <span className="message-role">
                  {agentName}
                </span>
                <div className="message-header-right">
                  <span className="message-time">
                    {new Date(msg.createdAt).toLocaleString()}
                  </span>
                  {/* 重试按钮 */}
                  <Tooltip title="重试">
                    <Button
                      type="text"
                      size="small"
                      icon={<ReloadOutlined />}
                      className="message-action-btn"
                      onClick={() => handleRetryAgent(msg)}
                    />
                  </Tooltip>
                </div>
              </div>
              <div className="message-body">{msg.content}</div>

              {/* 产物卡片 */}
              {hasArtifact && (
                <Card
                  size="small"
                  className="artifact-card-in-message"
                  style={{ marginTop: 12 }}
                  title={
                    <Space>
                      <FileTextOutlined />
                      <span>产物: {String((msg.metadata?.artifact as Record<string, unknown>)?.name || '产物')}</span>
                    </Space>
                  }
                >
                  <Text type="secondary">{String((msg.metadata?.artifact as Record<string, unknown>)?.description || '点击查看详情')}</Text>
                </Card>
              )}

              {/* 审查报告卡片 */}
              {hasReview && (
                <Card
                  size="small"
                  className="review-card-in-message"
                  style={{ marginTop: 12 }}
                  title={
                    <Space>
                      <ExclamationCircleOutlined />
                      <span>审查报告</span>
                    </Space>
                  }
                >
                  <ReviewReport
                    result={msg.metadata?.reviewReport as any}
                    issues={msg.metadata?.reviewIssues as ReviewIssue[] || []}
                  />
                </Card>
              )}
            </div>
          </div>
        </div>
      );
    }

    // 用户消息 - 微信风格：消息框在右，头像在消息框右边
    return (
      <div key={msg.id} className="message-container message-container-user">
        <div className="message message-user">
          <div className="message-content">
            <div className="message-header">
              <span className="message-role">用户</span>
              <span className="message-time">
                {new Date(msg.createdAt).toLocaleString()}
              </span>
            </div>
            <div className="message-body">{msg.content}</div>
          </div>
        </div>
        <Avatar
          className="message-avatar"
          icon={<UserOutlined />}
          style={{ backgroundColor: '#52c41a' }}
        />
      </div>
    );
  };

  /**
   * 渲染产物列表项
   */
  const renderArtifactItem = (artifact: Artifact) => {
    const iconMap: Record<string, React.ReactNode> = {
      code: <CodeOutlined style={{ color: '#52c41a' }} />,
      document: <FileTextOutlined style={{ color: '#1890ff' }} />,
      review: <FileSearchOutlined style={{ color: '#faad14' }} />,
      test: <CheckCircleOutlined style={{ color: '#722ed1' }} />,
      config: <FileOutlined style={{ color: '#666' }} />,
    };

    return (
      <List.Item
        className="artifact-card-item"
        onClick={() => {
          // TODO: 打开产物预览
          message.info('产物预览功能开发中');
        }}
      >
        <List.Item.Meta
          avatar={iconMap[artifact.type] || <FileOutlined />}
          title={artifact.name}
          description={
            <Space direction="vertical" size={0}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {ArtifactTypeLabels[artifact.type] || artifact.type}
              </Text>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {new Date(artifact.createdAt).toLocaleString()}
              </Text>
            </Space>
          }
        />
      </List.Item>
    );
  };

  /**
   * 处理文件选择
   */
  const handleFileSelect = (path: string, isDir: boolean) => {
    console.log('Selected file:', path, 'isDir:', isDir);
    // TODO: 可以添加文件预览或其他操作
    if (!isDir) {
      message.info(`选中文件: ${path}`);
    }
  };

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%' }}>
        <Spin size="large" />
      </div>
    );
  }

  const isRunning = activeAgents.length > 0;
  const isPaused = currentThread?.status === 'paused';

  // Get agents available for @mention from workflow template
  const mentionableAgents = getFilteredAgents();

  // Create a map of agent id -> display info for @mention
  const agentOptions = mentionableAgents.map(agent => ({
    id: agent.id,
    role: agent.role,
    name: agent.name,
    label: `${agent.name} (${AgentRoleLabels[agent.role as keyof typeof AgentRoleLabels] || agent.role})`,
  }));

  return (
    <div className="thread-view-wrapper">
      {/* 左侧文件树侧边栏 */}
      {fileSidebarVisible && projectId && (
        <div className="file-sidebar">
          <FileTree
            projectId={projectId}
            onFileSelect={handleFileSelect}
          />
        </div>
      )}

      <div className="thread-view">
        {/* 干预控制面板 */}
        <div className="intervention-bar">
          <Space style={{ width: '100%', justifyContent: 'space-between' }}>
            <Space>
              <Tooltip title={fileSidebarVisible ? '隐藏文件树' : '显示文件树'}>
                <Button
                  icon={fileSidebarVisible ? <MenuFoldOutlined /> : <MenuUnfoldOutlined />}
                  onClick={() => setFileSidebarVisible(!fileSidebarVisible)}
                  size="small"
                />
              </Tooltip>
              <Button
                icon={<ArrowLeftOutlined />}
                onClick={() => navigate(`/projects/${projectId}`)}
                size="small"
              >
                返回项目
              </Button>
            <Tag color={wsConnected ? 'green' : 'red'}>
              {wsConnected ? '已连接' : '未连接'}
            </Tag>
            {currentThread && (
              <Tag color={PhaseColors[currentThread.currentPhase]}>
                {PhaseLabels[currentThread.currentPhase]}
              </Tag>
            )}
            {isRunning && (
              <Badge status="processing" text={`${activeAgents.length} 个 Agent 运行中`} />
            )}
          </Space>
          <InterventionControls
            onPause={handlePause}
            onResume={handleResume}
            onSkip={handleSkip}
            onRetry={handleRetry}
            onStop={handleStop}
            onShowArtifacts={() => setArtifactDrawerVisible(true)}
            isPaused={isPaused}
            isRunning={isRunning}
          />
        </Space>
      </div>

      {/* 消息区域 */}
      <div className="thread-messages">
        {messages.length === 0 && Object.keys(streamingMessages).length === 0 ? (
          <div style={{ textAlign: 'center', padding: '60px 20px', color: '#999' }}>
            <RobotOutlined style={{ fontSize: 48, marginBottom: 16 }} />
            <Title level={4} type="secondary">开始您的开发任务</Title>
            <Text type="secondary">
              在下方输入您的需求，或使用 @需求分析师、@架构师、@开发者 等 Agent 协助开发
            </Text>
          </div>
        ) : (
          <>
            {messages.map(renderMessage)}
            {/* 流式消息渲染 */}
            {Object.entries(streamingMessages).map(([invocationId, streamMsg]) => (
              <div key={invocationId} className="message-container message-container-agent">
                <Avatar
                  className="message-avatar"
                  icon={<RobotOutlined />}
                  style={{ backgroundColor: '#1890ff' }}
                />
                <div className="message message-agent streaming">
                  <div className="message-content">
                    <div className="message-header">
                      <span className="message-role">
                        {streamMsg.agentName || 'Agent'}
                      </span>
                      <div className="message-header-right">
                        <Tag color="processing" style={{ marginLeft: 8 }}>
                          生成中...
                        </Tag>
                        {/* 终止按钮 */}
                        <Tooltip title="终止">
                          <Button
                            type="text"
                            size="small"
                            danger
                            icon={<StopOutlined />}
                            className="message-action-btn"
                            onClick={() => handleStopAgent(invocationId)}
                          />
                        </Tooltip>
                      </div>
                    </div>
                    <div className="message-body">
                      {streamMsg.content}
                      <span className="streaming-cursor">▌</span>
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </>
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* 底部输入区 */}
      <div className="thread-input">
        <div style={{ position: 'relative', flex: 1 }}>
          <TextArea
            ref={inputRef}
            value={inputValue}
            onChange={handleInputChange}
            onKeyPress={handleKeyPress}
            placeholder="输入消息或使用 @需求分析师 @架构师 @开发者 等触发 Agent..."
            autoSize={{ minRows: 2, maxRows: 6 }}
          />

          {/* @mention 下拉列表 */}
          {mentionListVisible && (
            <Card
              size="small"
              className="mention-dropdown"
              style={{
                position: 'absolute',
                bottom: '100%',
                left: 0,
                marginBottom: 4,
                minWidth: 200,
                zIndex: 1000,
              }}
            >
              {loadingProjectContext ? (
                <div style={{ padding: 16, textAlign: 'center' }}>
                  <Spin size="small" />
                  <span style={{ marginLeft: 8 }}>加载中...</span>
                </div>
              ) : agentOptions.length === 0 ? (
                <div style={{ padding: 16, textAlign: 'center', color: '#999' }}>
                  当前工作流没有可用的 Agent
                </div>
              ) : (
                <List
                  size="small"
                  dataSource={agentOptions.filter(opt =>
                    !mentionFilter ||
                    opt.label.toLowerCase().includes(mentionFilter.toLowerCase()) ||
                    opt.role.toLowerCase().includes(mentionFilter.toLowerCase())
                  )}
                  renderItem={(opt) => (
                    <List.Item
                      style={{ cursor: 'pointer', padding: '8px 12px' }}
                      onClick={() => selectMention(opt.id, opt.role as AgentRole, opt.name)}
                      onMouseEnter={(e) => {
                        (e.currentTarget as HTMLElement).style.background = '#f5f5f5';
                      }}
                      onMouseLeave={(e) => {
                        (e.currentTarget as HTMLElement).style.background = 'transparent';
                      }}
                    >
                      <Space>
                        <Avatar size="small" icon={<RobotOutlined />} />
                        <span>{opt.label}</span>
                      </Space>
                    </List.Item>
                  )}
                />
              )}
            </Card>
          )}
        </div>
        <Space direction="vertical">
          <Button type="primary" icon={<SendOutlined />} onClick={handleSend}>
            发送
          </Button>
          <Button icon={<PaperClipOutlined />} onClick={() => setArtifactDrawerVisible(true)}>
            产物
          </Button>
        </Space>
      </div>

      {/* 运行中 Agent 显示 */}
      {activeAgents.length > 0 && (
        <div className="active-agents">
          <span>运行中的 Agent: </span>
          {activeAgents.map((agent) => (
            <Tooltip key={agent.id} title={agent.input}>
              <Tag color="processing">
                {AgentRoleLabels[agent.role as keyof typeof AgentRoleLabels] || agent.role}
              </Tag>
            </Tooltip>
          ))}
        </div>
      )}

      {/* 产物抽屉 */}
      <Drawer
        title={
          <Space>
            <FileTextOutlined />
            <span>工作产物</span>
            <Badge count={artifacts.length} />
          </Space>
        }
        placement="right"
        width={400}
        open={artifactDrawerVisible}
        onClose={() => setArtifactDrawerVisible(false)}
      >
        {artifacts.length > 0 ? (
          <List
            dataSource={artifacts}
            renderItem={renderArtifactItem}
            split
          />
        ) : (
          <Empty description="暂无产物" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        )}

        <Divider />

        {/* 审查报告 */}
        {reviewResult && (
          <Collapse defaultActiveKey={['review']}>
            <Panel
              header={
                <Space>
                  <ExclamationCircleOutlined />
                  <span>审查状态</span>
                  {reviewResult.decision === 'allow' ? (
                    <Tag color="green">可以放行</Tag>
                  ) : (
                    <Tag color="red">{reviewResult.p1Issues + reviewResult.p2Issues} 个问题</Tag>
                  )}
                </Space>
              }
              key="review"
            >
              <ReviewReport result={reviewResult} issues={reviewIssues} />
            </Panel>
          </Collapse>
        )}
      </Drawer>

      {/* 检查点确认弹窗 */}
      <Modal
        title={
          <Space>
            <ExclamationCircleOutlined style={{ color: '#faad14' }} />
            <span>{currentCheckpoint?.title || '确认检查点'}</span>
          </Space>
        }
        open={checkpointModalVisible}
        onOk={handleCheckpointConfirm}
        onCancel={handleCheckpointReject}
        okText="确认通过"
        cancelText="需要修改"
        width={600}
      >
        <Alert
          type="info"
          message="请确认以下内容是否符合预期"
          description={currentCheckpoint?.content}
          showIcon
          style={{ marginBottom: 16 }}
        />
        <Text type="secondary">
          确认后将进入下一阶段，如需修改请点击"需要修改"并在对话中描述您的修改要求。
        </Text>
      </Modal>
      </div>
    </div>
  );
};

export default ThreadView;