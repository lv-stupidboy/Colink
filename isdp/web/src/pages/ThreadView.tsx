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
  FileSearchOutlined,
  StopOutlined,
  ReloadOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  DesktopOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons';
import { useAppStore } from '@/store';
import { useDebugThreadStore } from '@/store/debugThread';
import type { Message, Artifact, ReviewIssue, MergeCheckResult, AgentRole, AgentConfig } from '@/types';
import { PhaseLabels, PhaseColors, AgentRoleLabels, ArtifactTypeLabels } from '@/types';
import { InterventionControls } from '@/components/InterventionControls';
import { ReviewReport } from '@/components/ReviewReport';
import { SandboxPanel } from '@/components/thread';
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
  const { threadId, projectId, agentId } = useParams<{ threadId: string; projectId: string; agentId: string }>();
  const navigate = useNavigate();
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const inputRef = useRef<any>(null);
  const threadIdRef = useRef<string | null>(null);
  const wsConnectedRef = useRef(false);

  // 判断是否为调试模式
  const isDebugMode = Boolean(agentId);

  // 工作流模式的 store
  const {
    currentThread,
    messages: workflowMessages,
    streamingMessages: workflowStreamingMessages,
    activeAgents,
    loading: workflowLoading,
    wsConnected: workflowWsConnected,
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
    // 调试模式状态
    debugAgentConfig,
    debugProjectPath,
    sandboxServer,
    sandboxLoading,
    dockerAvailable,
    // 调试模式 actions
    setDebugMode,
    setDebugAgentConfig,
    setDebugProjectPath,
    // 沙箱 actions
    setSandboxServer,
    setSandboxLoading,
    setDockerAvailable,
    // 当前项目
    currentProject,
  } = useAppStore();

  // 调试模式的独立 store
  const {
    threadId: debugThreadId,
    messages: debugMessages,
    streamingContent: debugStreamingContent,
    status: debugStatus,
    sandboxServer: debugSandboxServer,
    sandboxLoading: debugSandboxLoading,
    setThreadId: setDebugThreadId,
    addMessage: addDebugMessage,
    appendStreamChunk: appendDebugStreamChunk,
    clearStreamContent: clearDebugStreamContent,
    setStatus: setDebugStatus,
    clearAll: clearDebugAll,
    setSandboxServer: setDebugSandboxServer,
    setSandboxLoading: setDebugSandboxLoading,
  } = useDebugThreadStore();

  // 调试模式的本地 WebSocket 连接状态（避免使用全局状态导致重新渲染）
  // 必须在使用之前定义
  const [debugWsConnected, setDebugWsConnected] = useState(false);

  // 根据模式选择使用哪个状态
  const messages = isDebugMode ? debugMessages : workflowMessages;
  const streamingMessages = isDebugMode
    ? (debugStreamingContent ? { 'debug': { content: debugStreamingContent, agentId: agentId || '', agentName: debugAgentConfig?.name } } : {})
    : workflowStreamingMessages;
  // 调试模式下，不使用全屏 loading，只在消息区显示加载状态
  // 因为 debugStatus === 'running' 只是表示 Agent 正在执行，不应该阻止用户交互
  const loading = isDebugMode ? false : workflowLoading;
  // 调试模式使用本地状态，工作流模式使用全局状态
  const wsConnected = isDebugMode ? debugWsConnected : workflowWsConnected;
  // 沙箱状态根据模式选择
  const currentSandboxServer = isDebugMode ? debugSandboxServer : sandboxServer;
  const currentSandboxLoading = isDebugMode ? debugSandboxLoading : sandboxLoading;

  const [inputValue, setInputValue] = useState('');
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
  const [sandboxSidebarVisible, setSandboxSidebarVisible] = useState(false);
  const [artifactsSidebarVisible, setArtifactsSidebarVisible] = useState(false);

  // 调试模式的 WebSocket 连接
  const connectDebugWebSocket = (id: string) => {
    const wsUrl = `ws://${window.location.host}/api/v1/ws?threadId=${id}`;
    wsRef.current = new WebSocket(wsUrl);

    wsRef.current.onopen = () => {
      wsConnectedRef.current = true;
      setDebugWsConnected(true);
      console.log('[Debug] WebSocket connected');
    };

    wsRef.current.onclose = () => {
      wsConnectedRef.current = false;
      setDebugWsConnected(false);
      console.log('[Debug] WebSocket disconnected');
    };

    wsRef.current.onmessage = (event) => {
      const data = JSON.parse(event.data);
      handleDebugWsMessage(data);
    };
  };

  // 调试模式的 WebSocket 消息处理
  const handleDebugWsMessage = (data: { type: string; payload: Record<string, unknown> }) => {
    console.log('[Debug] WS message:', data.type);

    switch (data.type) {
      case 'agent_output_chunk':
        const chunk = data.payload.chunk as string;
        if (chunk) {
          appendDebugStreamChunk(chunk);
        }
        break;

      case 'agent_message':
        clearDebugStreamContent();
        const agentMsg: Message = {
          id: (data.payload.messageId as string) || Date.now().toString(),
          threadId: debugThreadId || '',
          role: 'agent',
          agentId: data.payload.agentId as string,
          content: data.payload.content as string,
          messageType: 'text',
          createdAt: new Date().toISOString(),
        };
        addDebugMessage(agentMsg);
        setDebugStatus('idle');
        break;

      case 'system_message':
        const sysMsg: Message = {
          id: Date.now().toString(),
          threadId: debugThreadId || '',
          role: 'system',
          content: data.payload.content as string,
          messageType: 'text',
          createdAt: new Date().toISOString(),
        };
        addDebugMessage(sysMsg);
        break;

      case 'agent_status':
        const status = data.payload.status as string;
        setDebugStatus(status === 'running' ? 'running' : status === 'completed' ? 'completed' : status === 'failed' ? 'error' : 'idle');
        if (status === 'completed' || status === 'failed' || status === 'cancelled') {
          clearDebugStreamContent();
        }
        break;

      case 'sandbox_ready':
        const sandboxUrl = data.payload.url as string;
        const sandboxId = data.payload.id as string;
        const sandboxPort = data.payload.port as number;
        const sandboxProjectPath = data.payload.projectPath as string;
        const sandboxMode = data.payload.mode as string;
        if (sandboxUrl) {
          setDebugSandboxServer({
            id: sandboxId || '',
            threadId: debugThreadId || '',
            projectPath: sandboxProjectPath || '',
            mode: sandboxMode || 'local',
            port: sandboxPort || 0,
            url: sandboxUrl,
            status: 'running',
          });
          message.success('沙箱已启动');
        }
        break;
    }
  };

  // 工作流模式 - 加载 thread 和 WebSocket
  useEffect(() => {
    if (!isDebugMode && threadId) {
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
  }, [threadId, isDebugMode]);

  // 调试模式 - 初始化（每次进入都重新开始）
  useEffect(() => {
    if (isDebugMode && agentId) {
      // 每次进入调试页面都清空之前的消息，开始新会话
      clearDebugAll();
      setDebugMode(true, agentId);
      // 重置 WebSocket 状态
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
      wsConnectedRef.current = false;
      setDebugWsConnected(false);
      // 加载 Agent 配置
      api.agents.get(agentId).then((config: AgentConfig) => {
        setDebugAgentConfig(config);
      }).catch(err => {
        message.error('加载 Agent 配置失败');
        console.error(err);
      });
      // 检查 Docker 可用性
      api.sandbox.checkDocker().then(res => {
        setDockerAvailable(res.available);
      }).catch(() => {
        setDockerAvailable(false);
      });
    } else {
      setDebugMode(false);
      setDebugAgentConfig(null);
    }

    return () => {
      // 组件卸载时清理 WebSocket
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
      wsConnectedRef.current = false;
    };
  }, [isDebugMode, agentId]);

  // 调试模式 - 当 debugThreadId 变化时连接 WebSocket
  useEffect(() => {
    if (isDebugMode && debugThreadId && !wsConnectedRef.current) {
      connectDebugWebSocket(debugThreadId);
      threadIdRef.current = debugThreadId;
    }
  }, [isDebugMode, debugThreadId]);

  // Load agent configs for @mention dropdown (仅工作流模式)
  useEffect(() => {
    if (!isDebugMode) {
      loadAgentConfigs();
    }
  }, [loadAgentConfigs, isDebugMode]);

  // Load workflow template when thread is loaded (仅工作流模式)
  useEffect(() => {
    if (isDebugMode) return;

    const loadWorkflowContext = async () => {
      // 加载工作流模板（用于获取可用 Agent 列表）
      if (currentThread?.workflowTemplateId) {
        await loadWorkflowTemplate(currentThread.workflowTemplateId);
      }
      // 加载项目上下文（获取 localPath）- 优先用路由参数中的 projectId
      const projectToLoad = projectId || currentThread?.projectId;
      if (projectToLoad) {
        await loadProjectContext(projectToLoad);
      }
    };

    loadWorkflowContext();

    return () => {
      clearProjectContext();
    };
  }, [currentThread?.workflowTemplateId, currentThread?.projectId, projectId, loadWorkflowTemplate, loadProjectContext, clearProjectContext, isDebugMode]);

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
      case 'sandbox_ready':
        // 沙箱就绪，更新沙箱 URL
        const sandboxUrl = data.payload.url as string;
        const sandboxId = data.payload.id as string;
        const sandboxPort = data.payload.port as number;
        const sandboxProjectPath = data.payload.projectPath as string;
        const sandboxMode = data.payload.mode as string;
        if (sandboxUrl) {
          setSandboxServer({
            id: sandboxId || '',
            threadId: threadIdRef.current || '',
            projectPath: sandboxProjectPath || '',
            mode: sandboxMode || 'local',
            port: sandboxPort || 0,
            url: sandboxUrl,
            status: 'running',
          });
          message.success('沙箱已启动');
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
   * 调试模式：直接发送给当前 Agent
   * 工作流模式：支持 @mention 触发特定 Agent
   */
  const handleSend = async () => {
    if (!inputValue.trim()) return;

    const content = inputValue.trim();
    setInputValue('');
    setMentionListVisible(false);

    // 调试模式
    if (isDebugMode) {
      await handleDebugSend(content);
      return;
    }

    // 工作流模式 - 检查是否是 @mention 命令
    const mentionMatch = content.match(/^@(\S+)\s*(.*)/);
    if (mentionMatch) {
      const agentName = mentionMatch[1].toLowerCase();
      const input = mentionMatch[2] || content;

      if (selectedAgentId) {
        await sendMessage(content, true);
        await spawnAgent('custom', input, selectedAgentId);
        setSelectedAgentId(null);
        return;
      }

      const agentByName = agentOptions.find(opt =>
        opt.name.toLowerCase() === agentName ||
        opt.label.toLowerCase() === agentName
      );
      if (agentByName) {
        await sendMessage(content, true);
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

  /**
   * 调试模式发送消息
   */
  const handleDebugSend = async (content: string) => {
    if (!debugAgentConfig) {
      message.error('Agent 配置未加载');
      return;
    }

    // 添加用户消息
    const userMsg: Message = {
      id: Date.now().toString(),
      threadId: debugThreadId || '',
      role: 'user',
      content,
      messageType: 'text',
      createdAt: new Date().toISOString(),
    };
    addDebugMessage(userMsg);
    setDebugStatus('running');

    try {
      if (debugThreadId && wsConnectedRef.current) {
        // 已有会话，继续发送
        await api.agents.continueDebug(debugThreadId, content);
      } else {
        // 新会话：创建 thread -> 连接 WebSocket -> 调用 debug
        const threadResult = await api.agents.createDebugThread(debugProjectPath || undefined);
        const newThreadId = threadResult.threadId;
        console.log('[Debug] Created thread:', newThreadId);

        setDebugThreadId(newThreadId);

        // 等待 WebSocket 连接
        await new Promise<void>((resolve, reject) => {
          const startTime = Date.now();
          const check = () => {
            if (wsConnectedRef.current) {
              resolve();
            } else if (Date.now() - startTime > 5000) {
              reject(new Error('WebSocket connection timeout'));
            } else {
              setTimeout(check, 100);
            }
          };
          check();
        });

        // 调用 debug API
        await api.agents.debug(debugAgentConfig.id, content, debugProjectPath || undefined, newThreadId);
      }
    } catch (error: any) {
      setDebugStatus('error');
      const errorMsg: Message = {
        id: Date.now().toString(),
        threadId: debugThreadId || '',
        role: 'system',
        content: `错误: ${error.message || '请求失败'}`,
        messageType: 'text',
        createdAt: new Date().toISOString(),
      };
      addDebugMessage(errorMsg);
      console.error('[Debug] Error:', error);
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
   * 获取工作目录
   * 调试模式：用户输入
   * 工作流模式：从项目上下文获取
   */
  const getProjectPath = () => {
    if (isDebugMode) {
      return debugProjectPath;
    } else {
      return currentProject?.localPath || '';
    }
  };

  /**
   * 运行沙箱
   */
  const handleRunSandbox = async (mode: 'local' | 'docker') => {
    const projectPath = getProjectPath();

    if (!projectPath.trim()) {
      message.warning('请先设置工作目录');
      return;
    }

    if (mode === 'docker' && !dockerAvailable) {
      message.warning('Docker不可用，请确保Docker已启动');
      return;
    }

    // 根据模式设置加载状态
    if (isDebugMode) {
      setDebugSandboxLoading(true);
    } else {
      setSandboxLoading(true);
    }

    try {
      const server = await api.sandbox.runProject(threadId || debugThreadId || undefined, projectPath, mode);
      // 根据模式设置沙箱服务器状态
      if (isDebugMode) {
        setDebugSandboxServer(server);
      } else {
        setSandboxServer(server);
      }
      message.success(`项目已在${mode === 'docker' ? '容器' : '本地'}沙箱中启动`);
    } catch (error: any) {
      message.error(`启动失败: ${error.message || '未知错误'}`);
    } finally {
      // 根据模式重置加载状态
      if (isDebugMode) {
        setDebugSandboxLoading(false);
      } else {
        setSandboxLoading(false);
      }
    }
  };

  /**
   * 停止沙箱
   */
  const handleStopSandbox = async () => {
    if (!currentSandboxServer) return;

    try {
      await api.sandbox.stopServer(currentSandboxServer.id);
      // 根据模式清除沙箱服务器状态
      if (isDebugMode) {
        setDebugSandboxServer(null);
      } else {
        setSandboxServer(null);
      }
      message.success('已停止');
    } catch (error: any) {
      message.error('停止失败');
    }
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

  const isRunning = isDebugMode ? debugStatus === 'running' : activeAgents.length > 0;
  const isPaused = !isDebugMode && currentThread?.status === 'paused';

  // Get agents available for @mention
  // 调试模式：只显示当前调试的 Agent
  // 工作流模式：从工作流模板获取
  const mentionableAgents = isDebugMode
    ? (debugAgentConfig ? [debugAgentConfig] : [])
    : getFilteredAgents();

  // Create a map of agent id -> display info for @mention
  const agentOptions = mentionableAgents.map(agent => ({
    id: agent.id,
    role: agent.role,
    name: agent.name,
    label: `${agent.name} (${AgentRoleLabels[agent.role as keyof typeof AgentRoleLabels] || agent.role})`,
  }));

  // 获取工作目录
  const displayProjectPath = isDebugMode ? debugProjectPath : (currentProject?.localPath || '');

  return (
    <div className="thread-view-wrapper">
      {/* 左侧文件树侧边栏 - 调试模式和工作流模式都显示 */}
      {fileSidebarVisible && (isDebugMode || projectId) && (
        <div className="file-sidebar">
          {/* 工作目录显示/输入 */}
          <div className="file-sidebar-path">
            <span className="path-label">目录：</span>
            {isDebugMode ? (
              <Input
                placeholder="输入工作目录"
                value={debugProjectPath}
                onChange={e => setDebugProjectPath(e.target.value)}
                size="small"
                style={{ flex: 1, minWidth: 0 }}
              />
            ) : (
              <span className="path-value" title={displayProjectPath}>
                {displayProjectPath || '未设置'}
              </span>
            )}
          </div>
          <div className="file-tree-wrapper">
            {displayProjectPath ? (
              <FileTree
                projectId={projectId || 'debug'}
                projectPath={displayProjectPath}
                onFileSelect={handleFileSelect}
              />
            ) : (
              <div style={{ padding: 20, color: '#999', textAlign: 'center' }}>
                {isDebugMode ? '请输入工作目录' : '项目目录未设置'}
              </div>
            )}
          </div>
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
                onClick={() => isDebugMode ? navigate('/agents') : navigate(`/projects/${projectId}`)}
                size="small"
              >
                {isDebugMode ? '返回 Agent 列表' : '返回项目'}
              </Button>
            <Tag color={wsConnected ? 'green' : 'red'}>
              {wsConnected ? '已连接' : '未连接'}
            </Tag>
            {isDebugMode && debugAgentConfig && (
              <Tag color="purple">调试: {debugAgentConfig.name}</Tag>
            )}
            {currentThread && !isDebugMode && (
              <Tag color={PhaseColors[currentThread.currentPhase]}>
                {PhaseLabels[currentThread.currentPhase]}
              </Tag>
            )}
            {isRunning && (
              <Badge status="processing" text={`${activeAgents.length} 个 Agent 运行中`} />
            )}
          </Space>
          <Space>
            <InterventionControls
              onPause={handlePause}
              onResume={handleResume}
              onSkip={handleSkip}
              onRetry={handleRetry}
              onStop={handleStop}
              isPaused={isPaused}
              isRunning={isRunning}
            />
            {/* 产物按钮 */}
            <Tooltip title={artifactsSidebarVisible ? '隐藏产物' : '查看产物列表'}>
              <Button
                icon={<UnorderedListOutlined />}
                onClick={() => {
                  const willShow = !artifactsSidebarVisible;
                  setArtifactsSidebarVisible(willShow);
                  // 打开产物时关闭沙箱
                  if (willShow) {
                    setSandboxSidebarVisible(false);
                  }
                }}
                size="small"
                type={artifactsSidebarVisible ? 'primary' : 'default'}
              >
                产物
              </Button>
            </Tooltip>
            {/* 沙箱按钮 */}
            <Tooltip title={sandboxSidebarVisible ? '隐藏沙箱' : '打开沙箱预览'}>
              <Button
                icon={<DesktopOutlined />}
                onClick={() => {
                  const willShow = !sandboxSidebarVisible;
                  setSandboxSidebarVisible(willShow);
                  // 打开沙箱时关闭产物
                  if (willShow) {
                    setArtifactsSidebarVisible(false);
                    // 收起左侧目录树，给对话框更大空间
                    setFileSidebarVisible(false);
                  }
                }}
                size="small"
                type={currentSandboxServer ? 'primary' : sandboxSidebarVisible ? 'default' : 'default'}
              >
                沙箱
              </Button>
            </Tooltip>
          </Space>
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

      {/* 产物侧边栏 */}
      {artifactsSidebarVisible && (
        <div className="artifacts-sidebar">
          <div className="artifacts-sidebar-header">
            <span>产物列表</span>
            <Button
              type="text"
              size="small"
              onClick={() => setArtifactsSidebarVisible(false)}
            >
              ✕
            </Button>
          </div>
          <div className="artifacts-sidebar-content">
            {artifacts.length > 0 ? (
              <List
                dataSource={artifacts}
                renderItem={renderArtifactItem}
                split
                style={{ padding: '12px 16px' }}
              />
            ) : (
              <Empty
                description="暂无产物"
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                style={{ padding: '40px 16px' }}
              />
            )}

            <Divider style={{ margin: '12px 16px' }} />

            {/* 审查报告 */}
            {reviewResult && (
              <div style={{ padding: '0 16px 16px' }}>
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
              </div>
            )}
          </div>
        </div>
      )}

      {/* 沙箱侧边栏 */}
      {sandboxSidebarVisible && (
        <SandboxPanel
          onClose={() => setSandboxSidebarVisible(false)}
          isDebugMode={isDebugMode}
          hasProjectPath={Boolean(getProjectPath())}
          sandboxServer={currentSandboxServer}
          sandboxLoading={currentSandboxLoading}
          dockerAvailable={dockerAvailable}
          onRunSandbox={handleRunSandbox}
          onStopSandbox={handleStopSandbox}
        />
      )}
    </div>
  );
};

export default ThreadView;