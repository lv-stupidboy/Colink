import React, { useEffect, useRef, useState, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Input,
  Button,
  Space,
  Tag,
  Spin,
  message,
  Tooltip,
  Modal,
  Typography,
  Collapse,
  Alert,
  List,
  Divider,
  Badge,
  Empty,
  notification,
} from 'antd';
import {
  RobotOutlined,
  ArrowLeftOutlined,
  FileTextOutlined,
  CodeOutlined,
  FileOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  FileSearchOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  DesktopOutlined,
  UnorderedListOutlined,
  FullscreenOutlined,
  ThunderboltOutlined,
  ApartmentOutlined,
} from '@ant-design/icons';
import { useAppStore } from '@/store';
import { useDebugThreadStore } from '@/store/debugThread';
import type { Message, Artifact, ReviewIssue, MergeCheckResult, AgentConfig, ToolEvent, MessageContentBlock, QuestionItem } from '@/types';
import type { FileChange } from '@/types/content';
import { AgentRoleLabels, ArtifactTypeLabels } from '@/types';
import { ReviewReport } from '@/components/ReviewReport';
import { RightPanel, TaskList, ThreadInput } from '@/components/thread';
import { FilePreviewPanel } from '@/components/thread/FilePreviewPanel';
import { BlockingDetector } from '@/utils/blockingDetector';
import { sendAgentCompletionNotification, requestNotificationPermission, isNotificationGranted, clearPendingNotifications } from '@/utils/systemNotification';
import { ChatMessageList } from '@/components/thread/ChatMessageList';
import { StatusPanel } from '@/components/thread/StatusPanel';
import QuestionModal from '@/components/thread/QuestionModal';
import FileTree from '@/components/FileTree';
import api from '@/api/client';
import type { Thread } from '@/types';
import './ThreadView.css';

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
  const threadIdRef = useRef<string | null>(null);
  const wsConnectedRef = useRef(false);

  // 判断是否为调试模式
  const isDebugMode = Boolean(agentId);

  // 团队模式的 store - 使用 selector 订阅避免整体重渲染
  // 低频状态 - 只在切换 thread 时变化
  const currentThread = useAppStore((s) => s.currentThread);
  const activeAgents = useAppStore((s) => s.activeAgents);
  const loadingProjectContext = useAppStore((s) => s.loadingProjectContext);
  const currentProject = useAppStore((s) => s.currentProject);
  const debugAgentConfig = useAppStore((s) => s.debugAgentConfig);
  const debugProjectPath = useAppStore((s) => s.debugProjectPath);
  const sandboxServer = useAppStore((s) => s.sandboxServer);
  const sandboxLoading = useAppStore((s) => s.sandboxLoading);
  const dockerAvailable = useAppStore((s) => s.dockerAvailable);
  const soloMode = useAppStore((s) => s.soloMode);

  // 高频状态 - 只订阅 messages（流式消息由 StreamingMessage 组件独立处理）
  const workflowMessages = useAppStore((s) => s.messages);
  const workflowLoading = useAppStore((s) => s.loading);
  const workflowWsConnected = useAppStore((s) => s.wsConnected);

  // Actions - 使用 getState() 获取避免订阅
  const loadThread = useAppStore((s) => s.loadThread);
  const sendMessage = useAppStore((s) => s.sendMessage);
  const spawnAgent = useAppStore((s) => s.spawnAgent);
  const setWsConnected = useAppStore((s) => s.setWsConnected);
  const addMessage = useAppStore((s) => s.addMessage);
  const updateAgentStatus = useAppStore((s) => s.updateAgentStatus);
  const updateStreamingMessage = useAppStore((s) => s.updateStreamingMessage);
  const finalizeStreamingMessage = useAppStore((s) => s.finalizeStreamingMessage);
  const recoverInvocationState = useAppStore((s) => s.recoverInvocationState);
  const updateInvocationStatus = useAppStore((s) => s.updateInvocationStatus);
  const updateProgress = useAppStore((s) => s.updateProgress);
  const loadProjectContext = useAppStore((s) => s.loadProjectContext);
  const loadWorkflowTemplate = useAppStore((s) => s.loadWorkflowTemplate);
  const clearProjectContext = useAppStore((s) => s.clearProjectContext);
  const clearThreadMessages = useAppStore((s) => s.clearThreadMessages);
  const setCurrentThread = useAppStore((s) => s.setCurrentThread);
  const getFilteredAgents = useAppStore((s) => s.getFilteredAgents);
  const loadAgentConfigs = useAppStore((s) => s.loadAgentConfigs);
  const setDebugMode = useAppStore((s) => s.setDebugMode);
  const setDebugAgentConfig = useAppStore((s) => s.setDebugAgentConfig);
  const setDebugProjectPath = useAppStore((s) => s.setDebugProjectPath);
  const setSoloMode = useAppStore((s) => s.setSoloMode);
  const setSandboxServer = useAppStore((s) => s.setSandboxServer);
  const setSandboxLoading = useAppStore((s) => s.setSandboxLoading);
  const setDockerAvailable = useAppStore((s) => s.setDockerAvailable);
  const updateAgentUsage = useAppStore((s) => s.updateAgentUsage);
  const updateInvocationFullPrompt = useAppStore((s) => s.updateInvocationFullPrompt);
  const appendContentBlock = useAppStore((s) => s.appendContentBlock);
  const updateContentBlock = useAppStore((s) => s.updateContentBlock);

  // 阻塞提醒相关 - 使用 notification 自动显示
  const blockingItems = useAppStore((s) => s.blockingItems);
  const blockingReminderEnabled = useAppStore((s) => s.blockingReminderEnabled);
  const removeBlockingItem = useAppStore((s) => s.removeBlockingItem);

  // 调试模式的独立 store
  const {
    threadId: debugThreadId,
    messages: debugMessages,
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

  // 工具事件本地状态（用于旧版 CLI 输出块兼容显示）
  const [toolEvents, setToolEvents] = useState<Record<string, ToolEvent[]>>({});

  // invocation 完成后延迟清理工具事件（延长到 30 秒，给用户足够时间查看）
  const clearToolEvents = useCallback((invocationId: string, delay = 30000) => {
    setTimeout(() => {
      setToolEvents(prev => {
        const next = { ...prev };
        delete next[invocationId];
        return next;
      });
    }, delay);
  }, []);

  // 任务抽屉状态（默认收起）
  const [taskDrawerOpen, setTaskDrawerOpen] = useState(false);

  // 根据模式选择使用哪个状态
  const messages = isDebugMode ? debugMessages : workflowMessages;

  // 调试模式下，不使用全屏 loading，只在消息区显示加载状态
  // 因为 debugStatus === 'running' 只是表示 Agent 正在执行，不应该阻止用户交互
  const loading = isDebugMode ? false : workflowLoading;
  // 调试模式使用本地状态，团队模式使用全局状态
  const wsConnected = isDebugMode ? debugWsConnected : workflowWsConnected;
  // 沙箱状态根据模式选择
  const currentSandboxServer = isDebugMode ? debugSandboxServer : sandboxServer;
  const currentSandboxLoading = isDebugMode ? debugSandboxLoading : sandboxLoading;

  const [artifacts, setArtifacts] = useState<Artifact[]>([]);
  const [reviewResult, setReviewResult] = useState<MergeCheckResult | undefined>();
  const [reviewIssues, setReviewIssues] = useState<ReviewIssue[]>([]);
  const [checkpointModalVisible, setCheckpointModalVisible] = useState(false);
  const [currentCheckpoint, setCurrentCheckpoint] = useState<{
    type: 'requirement' | 'design' | 'review' | 'deploy';
    title: string;
    content: string;
  } | null>(null);
  const [fileSidebarVisible, setFileSidebarVisible] = useState(false);
  const [artifactsSidebarVisible, setArtifactsSidebarVisible] = useState(false);

  // 文件预览状态
  const [filePreviewVisible, setFilePreviewVisible] = useState(false);
  const [filePreviewPath, setFilePreviewPath] = useState<string | null>(null);

  // 右侧面板状态（代码/沙箱统一管理）
  const [rightPanelVisible, setRightPanelVisible] = useState(false);

  // AskUserQuestion 状态
  const [pendingQuestion, setPendingQuestion] = useState<{
    invocationId: string;
    toolId: string;
    questions: QuestionItem[];
  } | null>(null);

  // Agent 完成后预填入状态
  const [prefilledMention, setPrefilledMention] = useState<string | undefined>(undefined);
  const [rightPanelActiveTab, setRightPanelActiveTab] = useState<'code' | 'sandbox'>('code');
  const [rightPanelWidth, setRightPanelWidth] = useState(520);
  const [isResizing, setIsResizing] = useState(false);
  const resizeStartX = useRef(0);
  const resizeStartWidth = useRef(0);

  // 代码文件列表
  const [codeFiles, setCodeFiles] = useState<FileChange[]>([]);
  const [expandedFiles, setExpandedFiles] = useState<Set<string>>(new Set());

  // Solo 模式任务管理
  const [soloTasks, setSoloTasks] = useState<Thread[]>([]);
  const [soloActiveTask, setSoloActiveTask] = useState<Thread | null>(null);
  const [soloNewTaskPending, setSoloNewTaskPending] = useState(false); // 是否正在创建新任务

  // 打开右侧面板显示代码
  const openCodePanel = (files: FileChange[]) => {
    setCodeFiles(files);
    setRightPanelActiveTab('code');
    setRightPanelVisible(true);
  };

  // 关闭右侧面板
  const closeRightPanel = () => {
    setRightPanelVisible(false);
  };

  // 切换文件展开
  const toggleFileExpand = (fileId: string) => {
    const newExpanded = new Set(expandedFiles);
    if (newExpanded.has(fileId)) {
      newExpanded.delete(fileId);
    } else {
      newExpanded.add(fileId);
    }
    setExpandedFiles(newExpanded);
  };

  // 调试模式的 WebSocket 连接
  const connectDebugWebSocket = (id: string) => {
    // 先关闭已有连接，避免重复连接
    if (wsRef.current) {
      const oldWs = wsRef.current;
      oldWs.onopen = null;
      oldWs.onclose = null;
      oldWs.onmessage = null;
      oldWs.onerror = null;
      oldWs.close();
      wsRef.current = null;
    }

    const wsUrl = `ws://${window.location.host}/api/v1/ws?threadId=${id}`;
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      if (wsRef.current === ws) {
        wsConnectedRef.current = true;
        setDebugWsConnected(true);
      }
    };

    ws.onclose = () => {
      if (wsRef.current === ws) {
        wsConnectedRef.current = false;
        setDebugWsConnected(false);
      }
    };

    ws.onmessage = (event) => {
      // 确保这是当前的 WebSocket
      if (wsRef.current === ws) {
        const data = JSON.parse(event.data);
        handleDebugWsMessage(data);
      }
    };
  };

  // 调试模式的 WebSocket 消息处理
  const handleDebugWsMessage = (data: { type: string; payload: Record<string, unknown> }) => {
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
          agentName: data.payload.agentName as string,
          content: data.payload.content as string,
          contentBlocks: data.payload.contentBlocks as MessageContentBlock[] | undefined,
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

      case 'agent_state_restore':
        // WebSocket 重连时恢复运行中的 Agent 状态
        const runningAgents = data.payload.runningAgents as Array<{
          invocationId: string;
          agentId: string;
          agentName: string;
          accumulatedOutput: string;
          status: string;
        }>;
        if (runningAgents && runningAgents.length > 0) {
          console.log('[WebSocket] Restoring debug agent state:', runningAgents);
          runningAgents.forEach(agent => {
            // 恢复累积的输出内容
            if (agent.accumulatedOutput) {
              appendDebugStreamChunk(agent.accumulatedOutput);
            }
            setDebugStatus('running');
          });
          message.info('已恢复 Agent 执行状态');
        }
        break;
    }
  };

  // 团队模式 - 加载 thread 和 WebSocket
  useEffect(() => {
    if (!isDebugMode && threadId) {
      loadThread(threadId);
      connectWebSocket(threadId);
      loadArtifacts(threadId);
      loadReviewData(threadId);
    }

    return () => {
      // 清理：关闭 WebSocket 连接
      if (wsRef.current) {
        wsRef.current.onopen = null;
        wsRef.current.onclose = null;
        wsRef.current.onmessage = null;
        wsRef.current.onerror = null;
        wsRef.current.close();
        wsRef.current = null;
        wsConnectedRef.current = false;
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
      // 重置 Solo 模式任务状态
      setSoloActiveTask(null);
      setSoloNewTaskPending(true);
      // 加载 Agent 配置
      api.agents.get(agentId).then((config: AgentConfig) => {
        setDebugAgentConfig(config);
        // 全栈工程师角色自动进入 Solo 模式
        if (config.role === 'fullstack_engineer') {
          setSoloMode(true);
        }
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
      // 退出调试模式时关闭 Solo 模式
      setSoloMode(false);
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

  // Solo 模式 - 处理 URL 中的 threadId
  useEffect(() => {
    if (soloMode && threadId && soloTasks.length > 0) {
      const task = soloTasks.find(t => t.id === threadId);
      if (task) {
        setSoloActiveTask(task);
        setSoloNewTaskPending(false);
      }
    }
  }, [soloMode, threadId, soloTasks]);

  // 调试模式 - 当 debugThreadId 变化时连接 WebSocket
  useEffect(() => {
    if (isDebugMode && debugThreadId && !wsConnectedRef.current) {
      connectDebugWebSocket(debugThreadId);
      threadIdRef.current = debugThreadId;
    }
  }, [isDebugMode, debugThreadId]);

  // Load agent configs for @mention dropdown (仅团队模式)
  useEffect(() => {
    if (!isDebugMode) {
      loadAgentConfigs();
    }
  }, [loadAgentConfigs, isDebugMode]);

  // Load workflow template when thread is loaded (仅团队模式)
  useEffect(() => {
    if (isDebugMode) return;

    const loadWorkflowContext = async () => {
      // 加载Agent团队（用于获取可用 Agent 列表）
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

  // 沙箱面板拖拽调整大小
  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizing) return;
      const deltaX = resizeStartX.current - e.clientX;
      const newWidth = Math.max(300, Math.min(800, resizeStartWidth.current + deltaX));
      setRightPanelWidth(newWidth);
    };

    const handleMouseUp = () => {
      if (isResizing) {
        setIsResizing(false);
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
      }
    };

    if (isResizing) {
      document.addEventListener('mousemove', handleMouseMove);
      document.addEventListener('mouseup', handleMouseUp);
    }

    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };
  }, [isResizing]);

  const connectWebSocket = (id: string) => {
    // 先关闭已有连接，避免重复连接
    if (wsRef.current) {
      const oldWs = wsRef.current;
      oldWs.onopen = null;
      oldWs.onclose = null;
      oldWs.onmessage = null;
      oldWs.onerror = null;
      oldWs.close();
      wsRef.current = null;
    }

    const wsUrl = `ws://${window.location.host}/api/v1/ws?threadId=${id}`;
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      if (wsRef.current === ws) {
        wsConnectedRef.current = true;
        setWsConnected(true);
      }
    };

    ws.onclose = () => {
      if (wsRef.current === ws) {
        wsConnectedRef.current = false;
        setWsConnected(false);
      }
    };

    ws.onmessage = (event) => {
      // 确保这是当前的 WebSocket
      if (wsRef.current === ws) {
        const data = JSON.parse(event.data);
        handleWsMessage(data);
      }
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
      case 'agent_output_chunk': {
        const chunkType = data.payload.chunkType as string || 'text';
        const invocId = data.payload.invocationId as string;

        if (chunkType === 'thinking') {
          // 思考块 - Store 会智能累积
          const thinkingText = data.payload.chunk as string;
          const isDone = data.payload.done as boolean;

          if (isDone) {
            // thinking 完成，更新状态为 success
            updateContentBlock(invocId, `thinking-${invocId}`, { status: 'success' });
          } else if (thinkingText) {
            appendContentBlock(invocId, {
              id: `thinking-${invocId}`,
              type: 'thinking',
              content: thinkingText,
              timestamp: Date.now(),
              status: 'streaming',
            });
          }
          updateProgress(invocId, 'thinking');
        } else if (chunkType === 'tool_use') {
          // 工具调用块
          const toolName = data.payload.toolName as string;
          const toolInput = data.payload.toolInput as Record<string, unknown>;
          const toolId = data.payload.toolId as string;
          updateProgress(invocId, 'tool_use', toolName, toolInput);

          // 追加工具调用块
          appendContentBlock(invocId, {
            id: toolId || `tool-${invocId}-${toolName}`,
            type: 'tool_use',
            toolName: toolName || 'Unknown',
            toolId: toolId || '',
            input: toolInput,
            timestamp: Date.now(),
            status: 'streaming',
            startedAt: Date.now(),
          });
        } else if (chunkType === 'tool_result') {
          // 工具调用结果 - 更新对应的工具块状态
          const toolId = data.payload.toolId as string;
          const isError = data.payload.isError as boolean;
          const toolOutput = data.payload.toolOutput as string;

          if (toolId) {
            updateContentBlock(invocId, toolId, {
              status: isError ? 'failed' : 'success',
              output: toolOutput,
              isError,
              duration: Date.now() - (data.payload.timestamp as number || Date.now()),
              completedAt: Date.now(),
            });
          }
        } else if (chunkType === 'text') {
          // 文本块 - 使用 updateStreamingMessage，Store 会智能累积
          const textContent = data.payload.chunk as string;
          if (textContent) {
            updateStreamingMessage(
              invocId,
              textContent,
              data.payload.agentId as string || '',
              data.payload.agentName as string
            );
          }
          updateProgress(invocId, 'generating');
        } else if (chunkType === 'question') {
          // AskUserQuestion 工具调用 - 需要用户输入
          const toolId = data.payload.toolId as string;
          const toolName = data.payload.toolName as string;
          const questions = data.payload.questions as QuestionItem[];
          const toolInput = data.payload.toolInput as Record<string, unknown>;

          // 追加问题块
          appendContentBlock(invocId, {
            id: `question-${toolId}`,
            type: 'question',
            toolName: toolName || 'AskUserQuestion',
            toolId: toolId || '',
            questions: questions || [],
            input: toolInput,
            timestamp: Date.now(),
            status: 'waiting_user_input',
            startedAt: Date.now(),
          });

          // 显示问题弹窗
          setPendingQuestion({
            invocationId: invocId,
            toolId: toolId,
            questions: questions || [],
          });
        }
        break;
      }
      case 'agent_message': {
        // Agent 完成消息：后端已保存到数据库并广播真实ID
        // payload 包含: messageId（真实UUID）、agentId、content、contentBlocks、agentName、agentRole
        const realMessageId = data.payload.messageId as string;
        const agentId = data.payload.agentId as string;
        const content = data.payload.content as string;
        const contentBlocks = data.payload.contentBlocks as MessageContentBlock[] | undefined;
        const agentName = data.payload.agentName as string;
        const agentRole = data.payload.agentRole as string;

        if (!realMessageId) {
          break;
        }

        // 使用 getState() 避免闭包陷阱
        const state = useAppStore.getState();
        const invocationId = state.streamingInvocationId;

        // 检查是否有临时消息需要替换ID（流式场景）
        if (invocationId && state.isStreaming) {
          const tempId = `agent-${invocationId}`;
          // 先完成流式消息（添加临时消息）
          finalizeStreamingMessage(invocationId);
          // 然后用真实ID替换临时ID
          useAppStore.getState().replaceMessageId(tempId, realMessageId);
        } else {
          // 非流式场景：检查是否已有临时消息（可能由 agent_status/completed 创建）
          const tempId = `agent-${realMessageId}`;
          const existingTemp = state.messages.find(m => m.id === tempId);
          if (existingTemp) {
            // 替换临时ID为真实ID
            useAppStore.getState().replaceMessageId(tempId, realMessageId);
          } else {
            // 直接添加新消息（使用真实ID）
            addMessage({
              id: realMessageId,
              threadId: threadId!,
              role: 'agent',
              agentId: agentId,
              content: content,
              contentBlocks: contentBlocks,
              messageType: 'text',
              metadata: {
                agentName: agentName,
                agentRole: agentRole,
              },
              createdAt: new Date().toISOString(),
            });
          }
        }
        break;
      }
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
      case 'agent_status': {
        const status = data.payload.status as string;
        const invocId = data.payload.invocationId as string;
        const agentName = data.payload.agentName as string;
        const agentId = data.payload.agentId as string || '';
        const input = data.payload.input as string | undefined;
        console.log('[agent_status] Received:', { status, invocId, agentName, agentId });
        updateAgentStatus(invocId, status, agentName, input);
        // Agent 完成时，如果有流式消息缓存，转为正式消息
        if (status === 'completed' || status === 'failed') {
          // 使用 getState() 避免闭包陷阱
          const state = useAppStore.getState();
          if (state.isStreaming && state.streamingInvocationId === invocId) {
            finalizeStreamingMessage(invocId);
          }
          // 延迟清理工具事件
          clearToolEvents(invocId);

          // Agent 完成后预填入：设置上一个对话的 Agent 名称
          // 只在 completed 状态且非调试模式下预填入
          if (status === 'completed' && !isDebugMode && agentName) {
            setPrefilledMention(agentName);
          }

          // 检测调度结束阻塞：Agent 完成且没有调用下一个 agent
          // 延迟 2 秒检测，等待可能的新 Agent 启动
          setTimeout(() => {
            const currentState = useAppStore.getState();
            console.log('[blockingCheck] After 2s delay:', {
              activeAgentsCount: currentState.activeAgents.length,
              blockingReminderEnabled: currentState.blockingReminderEnabled,
              invocId,
              agentId,
              agentName
            });
            // 如果没有活跃 Agent 且阻塞提醒已开启，添加调度结束阻塞
            if (currentState.activeAgents.length === 0 && currentState.blockingReminderEnabled) {
              const scheduleEnd = BlockingDetector.detectScheduleEnd(invocId, agentId, agentName);
              console.log('[blockingCheck] Creating blocking item:', scheduleEnd);
              if (scheduleEnd) {
                // 使用 currentState 的 addBlockingItem 避免闭包问题
                currentState.addBlockingItem(scheduleEnd);
                console.log('[blockingCheck] Blocking item added successfully');
              }
            } else {
              console.log('[blockingCheck] Skipped: activeAgents > 0 or reminder disabled');
            }
          }, 2000);
        }
        break;
      }
      case 'usage_update': {
        // Token 使用更新
        const invocId = data.payload.invocationId as string;
        const usage = data.payload.usage as {
          inputTokens?: number;
          outputTokens?: number;
          cacheReadTokens?: number;
          cacheCreationTokens?: number;
          costUsd?: number;
          durationMs?: number;
          durationApiMs?: number;
          numTurns?: number;
        };
        if (invocId && usage) {
          updateAgentUsage(invocId, usage);
        }
        break;
      }
      case 'sandbox_ready': {
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
      case 'agent_state_restore': {
        // WebSocket 重连时恢复运行中的 Agent 状态
        const runningAgents = data.payload.runningAgents as Array<{
          invocationId: string;
          agentId: string;
          agentName: string;
          accumulatedOutput: string;
          status: string;
        }>;
        if (runningAgents && runningAgents.length > 0) {
          console.log('[WebSocket] Restoring agent state:', runningAgents);
          runningAgents.forEach(agent => {
            // 更新 Agent 状态为运行中
            updateAgentStatus(agent.invocationId, 'running', agent.agentName);
            // 恢复累积的输出内容
            if (agent.accumulatedOutput) {
              updateStreamingMessage(
                agent.invocationId,
                agent.accumulatedOutput,
                agent.agentId,
                agent.agentName
              );
            }
          });
          message.info(`已恢复 ${runningAgents.length} 个运行中的 Agent`);
        }
        break;
      }
      case 'invocation_recovery': {
        // 后台执行支持：恢复 invocation 的内容块或最近完成的状态
        console.log('[WebSocket] Received invocation_recovery message:', data.payload);
        const payload = data.payload as {
          invocationId?: string;
          contentBlocks?: MessageContentBlock[];
          status?: string;
          agentId?: string;
          agentName?: string;
          recentlyCompleted?: Array<{
            invocationId: string;
            agentId: string;
            agentName: string;
            status: string;
          }>;
        };

        // 处理运行中的 invocation 恢复
        if (payload.invocationId && payload.contentBlocks && payload.status === 'running') {
          console.log('[WebSocket] Recovering invocation state:', payload.invocationId, 'blocks:', payload.contentBlocks.length);
          recoverInvocationState(payload.invocationId, payload.contentBlocks, payload.status, payload.agentId, payload.agentName);
        }

        // 处理最近完成的 invocation 状态同步
        if (payload.recentlyCompleted && payload.recentlyCompleted.length > 0) {
          console.log('[WebSocket] Syncing recently completed invocations:', payload.recentlyCompleted.length, payload.recentlyCompleted);
          payload.recentlyCompleted.forEach((inv) => {
            console.log('[WebSocket] Updating invocation status:', inv.invocationId, 'to', inv.status);
            // 更新 invocation 状态为完成
            updateInvocationStatus(inv.invocationId, inv.status, inv.agentId, inv.agentName);
          });
        }
        break;
      }
      case 'invocation_full_prompt': {
        // 完整 prompt 更新（用于调用日志显示）
        const invocId = data.payload.invocationId as string;
        const fullPrompt = data.payload.fullPrompt as string;
        if (invocId && fullPrompt) {
          updateInvocationFullPrompt(invocId, fullPrompt);
        }
        break;
      }
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
   * 调试模式发送消息
   */
  const handleDebugSend = async (content: string) => {
    if (!debugAgentConfig) {
      message.error('Agent 配置未加载');
      return;
    }

    // 用户发送新消息时，清除之前的阻塞项（开始新的交互）
    const state = useAppStore.getState();
    if (state.blockingItems.length > 0) {
      state.blockingItems.forEach(b => removeBlockingItem(b.id));
    }

    // 判断是否需要创建新会话
    // soloNewTaskPending 为 true 表示用户明确要新建任务
    // 或者 debugThreadId 为空表示还没有会话
    const needNewSession = soloNewTaskPending || !debugThreadId;

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
      if (!needNewSession && debugThreadId && wsConnectedRef.current) {
        // 已有会话且 WebSocket 已连接，继续发送
        await api.agents.continueDebug(debugThreadId, content);
      } else {
        // 新会话：创建 thread -> 连接 WebSocket -> 调用 debug
        // 先关闭旧连接
        if (wsRef.current) {
          wsRef.current.close();
          wsRef.current = null;
        }
        wsConnectedRef.current = false;
        setDebugWsConnected(false);

        const threadResult = await api.agents.createDebugThread(debugProjectPath || undefined);
        const newThreadId = threadResult.threadId;

        setDebugThreadId(newThreadId);
        setSoloNewTaskPending(false);

        // 主动连接 WebSocket（不依赖 useEffect）
        connectDebugWebSocket(newThreadId);

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

  // ========== Solo 模式任务管理 ==========

  // 加载 Solo 模式任务列表
  const loadSoloTasks = useCallback(async () => {
    if (!projectId) return;
    try {
      const data = await api.threads.list(projectId);
      const sorted = (data || []).sort((a, b) =>
        new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
      );
      setSoloTasks(sorted);
    } catch (error) {
      console.error('Failed to load solo tasks:', error);
    }
  }, [projectId]);

  // Solo 模式 - 加载任务列表
  useEffect(() => {
    if (soloMode && projectId) {
      loadSoloTasks();
    }
  }, [soloMode, projectId, loadSoloTasks]);

  // 选择任务
  const handleSelectSoloTask = useCallback(async (task: Thread) => {
    // 1. 清空当前消息
    if (isDebugMode) {
      clearDebugAll();
    } else {
      // 团队模式：清除团队消息
      clearThreadMessages();
    }

    // 2. 设置活跃任务
    setSoloActiveTask(task);
    setSoloNewTaskPending(false);

    // 3. 调试模式：设置 threadId + 加载消息 + 连接 WebSocket
    if (isDebugMode) {
      setDebugThreadId(task.id);

      // 加载历史消息
      try {
        const messages = await api.messages.list(task.id);
        messages.forEach(msg => addDebugMessage(msg));
      } catch (error) {
        console.error('Failed to load messages:', error);
      }

      // 连接 WebSocket（函数内部会先关闭现有连接）
      connectDebugWebSocket(task.id);
    } else {
      // 团队模式：设置 currentThread + 加载历史消息
      setCurrentThread(task);
      try {
        const messages = await api.messages.list(task.id);
        messages.forEach(msg => addMessage(msg));
      } catch (error) {
        console.error('Failed to load messages:', error);
      }
    }

    // 4. 更新 URL（不触发重新渲染）
    if (isDebugMode && agentId) {
      navigate(`/agents/${agentId}?threadId=${task.id}`, { replace: true });
    } else if (projectId) {
      navigate(`/projects/${projectId}/threads/${task.id}`, { replace: true });
    }
  }, [isDebugMode, agentId, projectId, navigate, clearDebugAll, clearThreadMessages, setCurrentThread, setDebugThreadId, addDebugMessage, connectDebugWebSocket, addMessage]);

  // 新建任务
  const handleCreateSoloTask = useCallback(() => {
    // 1. 清空当前消息和状态
    if (isDebugMode) {
      clearDebugAll();
    } else {
      // 团队模式：清除团队消息
      clearThreadMessages();
    }

    // 2. 重置活跃任务状态，标记为新任务待创建
    setSoloActiveTask(null);
    setSoloNewTaskPending(true);

    // 3. 不再导航跳转，保持在当前页面
  }, [isDebugMode, clearDebugAll, clearThreadMessages]);

  // Solo 模式发送消息（处理新任务命名）
  const handleSoloSend = useCallback(async (content: string) => {
    // 如果是新任务，先创建 thread
    if (soloNewTaskPending) {
      try {
        // 用第一条消息的前 30 个字符作为任务名
        const taskName = content.slice(0, 30) + (content.length > 30 ? '...' : '');

        let newThread: Thread;

        if (isDebugMode) {
          // 调试模式：使用 createDebugThread API（不需要 projectId）
          const threadResult = await api.agents.createDebugThread(debugProjectPath || undefined);
          // 更新 thread 名称
          newThread = await api.threads.get(threadResult.threadId);
          // 如果需要更新名称，可以调用 updateStatus 或其他 API
        } else if (projectId) {
          // 团队模式：使用 threads.create API
          newThread = await api.threads.create(projectId, taskName);
        } else {
          message.error('无法创建任务：缺少项目信息');
          return;
        }

        setSoloActiveTask(newThread);
        setSoloNewTaskPending(false);
        // 更新任务列表
        setSoloTasks(prev => [newThread, ...prev]);
        // 设置 threadId
        if (isDebugMode) {
          setDebugThreadId(newThread.id);
          // 连接 WebSocket
          connectDebugWebSocket(newThread.id);

          // 添加用户消息
          const userMsg: Message = {
            id: Date.now().toString(),
            threadId: newThread.id,
            role: 'user',
            content,
            messageType: 'text',
            createdAt: new Date().toISOString(),
          };
          addDebugMessage(userMsg);
          setDebugStatus('running');

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

          // 直接调用 debug API，使用 newThread.id 而不是依赖 state
          if (debugAgentConfig) {
            await api.agents.debug(debugAgentConfig.id, content, debugProjectPath || undefined, newThread.id);
          }
        } else {
          // 团队模式：设置 currentThread 以便 sendMessage 能正常工作
          setCurrentThread(newThread);
        }
        // 更新 URL
        if (isDebugMode && agentId) {
          navigate(`/agents/${agentId}?threadId=${newThread.id}`, { replace: true });
        } else if (projectId) {
          navigate(`/projects/${projectId}/threads/${newThread.id}`, { replace: true });
        }
      } catch (error) {
        console.error('Failed to create new task:', error);
        message.error('创建任务失败');
        return;
      }
    } else {
      // 不是新任务，调用原有的发送逻辑
      if (isDebugMode) {
        await handleDebugSend(content);
      } else {
        await sendMessage(content);
      }
    }
  }, [soloNewTaskPending, projectId, isDebugMode, agentId, navigate, setDebugThreadId, handleDebugSend, sendMessage, debugProjectPath, connectDebugWebSocket, setCurrentThread, debugAgentConfig, addDebugMessage, setDebugStatus]);

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
   * 团队模式：从项目上下文获取
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
    // TODO: 可以添加文件预览或其他操作
    if (!isDir) {
      message.info(`选中文件: ${path}`);
    }
  };

  // 处理文件打开（触发预览）
  const handleFileOpen = (filePath: string) => {
    setFilePreviewPath(filePath);
    setFilePreviewVisible(true);
    setRightPanelVisible(false);  // 关闭 RightPanel，互斥
  };

  // Get agents available for @mention
  // 调试模式：只显示当前调试的 Agent
  // 团队模式：从Agent团队获取
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

  // 阻塞通知触发：仅在所有 Agent 完成后显示（使用系统通知）
  useEffect(() => {
    const noActiveAgents = activeAgents.length === 0;
    const shouldShow = blockingReminderEnabled &&
                       blockingItems.length > 0 &&
                       noActiveAgents;

    console.log('[blockingEffect] Check:', {
      blockingItemsCount: blockingItems.length,
      blockingReminderEnabled,
      activeAgentsCount: activeAgents.length,
      shouldShow,
      notificationGranted: isNotificationGranted()
    });

    if (shouldShow) {
      // 取第一个阻塞项（只有 schedule_end 类型）
      const item = blockingItems[0];
      console.log('[blockingEffect] Showing system notification for:', item);

      // 发送系统级通知（即使切换到其他应用也能收到）
      sendAgentCompletionNotification(item.sourceAgentName, item.summary);

      // 同时显示页面内的通知（作为备份，当用户在页面内时）
      notification.success({
        message: 'Agent 执行完成',
        description: `${item.sourceAgentName} 已完成执行`,
        duration: 3,
        placement: 'topRight',
      });

      // 自动清除阻塞项
      removeBlockingItem(item.id);
    }
  }, [blockingItems, blockingReminderEnabled, activeAgents.length, removeBlockingItem]);

  /**
   * 处理 AskUserQuestion 用户答案提交
   * 采用 --resume 方案：用户响应发送自然语言消息，触发新的 SpawnAgent 恢复会话
   */
  const handleSubmitQuestionAnswer = useCallback(async (answers: Record<number, string | string[]>) => {
    if (!pendingQuestion || !threadId) return;

    try {
      // 将答案转换为自然语言消息格式
      // 对于多选，将多个答案合并为一个字符串
      const answerStrings: string[] = [];
      Object.entries(answers).forEach(([_index, value]) => {
        if (Array.isArray(value)) {
          answerStrings.push(`${(value as string[]).join('、')}`);
        } else {
          answerStrings.push(value as string);
        }
      });

      // 生成自然语言消息（而非技术性 tool_result）
      // 这样 Agent 可以通过 --resume 恢复会话并继续执行
      const userResponseMessage = answerStrings.join('\n');

      // 更新内容块状态为 success（用户已响应，Agent 正在处理）
      updateContentBlock(pendingQuestion.invocationId, `question-${pendingQuestion.toolId}`, {
        status: 'success',
        output: userResponseMessage,
        completedAt: Date.now(),
      });

      // 关闭弹窗
      setPendingQuestion(null);

      // 发送用户消息（通过 WebSocket），这会触发 SpawnAgentForUserMessage
      // 后端会检查最近的 completed invocation，使用其 SessionID 进行 --resume
      await sendMessage(userResponseMessage);

      message.success('答案已提交，Agent 正在处理...');
    } catch (error) {
      console.error('提交答案失败:', error);
      message.error('提交答案失败，请重试');
    }
  }, [pendingQuestion, threadId, updateContentBlock, sendMessage]);

  /**
   * 处理发送消息
   * 调试模式：直接发送给当前 Agent
   * 团队模式：支持 @mention 触发特定 Agent
   */
  const handleSend = useCallback(async (content: string) => {
    if (!content.trim()) return;

    // 清除累积的通知计数（开始新一轮任务）
    clearPendingNotifications();

    // 在触发 Agent 时请求系统通知权限（首次）
    if (!isNotificationGranted()) {
      requestNotificationPermission();
    }

    // Solo 模式 - 使用特殊的发送逻辑（处理新任务创建）
    if (soloMode) {
      await handleSoloSend(content);
      return;
    }

    // 调试模式
    if (isDebugMode) {
      await handleDebugSend(content);
      return;
    }

    // 团队模式 - 检查是否是 @mention 命令
    const mentionMatch = content.match(/^@(\S+)\s*(.*)/);
    if (mentionMatch) {
      const agentName = mentionMatch[1].toLowerCase();
      const input = mentionMatch[2] || content;

      const agentByName = agentOptions.find(opt =>
        opt.name.toLowerCase() === agentName ||
        opt.label.toLowerCase() === agentName
      );
      if (agentByName) {
        await sendMessage(content, true);
        await spawnAgent('custom', input, agentByName.id);
        return;
      }

      message.warning(`未找到 Agent: ${agentName}，请从下拉列表中选择`);
      return;
    } else {
      await sendMessage(content);
    }
  }, [soloMode, isDebugMode, agentOptions, sendMessage, spawnAgent, handleSoloSend, handleDebugSend]);

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%' }}>
        <Spin size="large" />
      </div>
    );
  }

  const isRunning = isDebugMode ? debugStatus === 'running' : activeAgents.length > 0;

  // 获取工作目录
  const displayProjectPath = isDebugMode ? debugProjectPath : (currentProject?.localPath || '');

  // 切换 Solo 模式
  const toggleSoloMode = () => {
    setSoloMode(!soloMode);
    if (!soloMode) {
      // 进入 Solo 模式时，收起侧边栏
      setFileSidebarVisible(false);
      setArtifactsSidebarVisible(false);
      setRightPanelVisible(false);
    }
  };

  // 右侧面板拖拽调整大小
  const handleResizeStart = (e: React.MouseEvent) => {
    e.preventDefault();
    setIsResizing(true);
    resizeStartX.current = e.clientX;
    resizeStartWidth.current = rightPanelWidth;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  };

  return (
    <div className={`thread-view-wrapper ${soloMode ? 'solo-mode' : ''}`}>
      {/* Solo 模式下的顶部切换栏 */}
      {soloMode && (
        <div className="solo-mode-header">
          <div className="solo-mode-tabs">
            {/* 任务抽屉切换按钮 */}
            <Button
              type="text"
              className={`solo-mode-tab ${taskDrawerOpen ? 'active' : ''}`}
              icon={<UnorderedListOutlined />}
              onClick={() => setTaskDrawerOpen(!taskDrawerOpen)}
            >
              任务
            </Button>
            <Button
              type="text"
              className="solo-mode-tab"
              icon={<ApartmentOutlined />}
              onClick={() => setSoloMode(false)}
            >
              代码模式
            </Button>
            <Button
              type="text"
              className="solo-mode-tab active"
              icon={<ThunderboltOutlined />}
            >
              Solo 模式
            </Button>
          </div>
          <Button
            className={`solo-mode-action-btn ${rightPanelVisible ? 'primary' : ''}`}
            icon={<DesktopOutlined />}
            onClick={() => {
              setRightPanelVisible(!rightPanelVisible);
              setFilePreviewVisible(false);  // 关闭文件预览，互斥
            }}
          >
            面板
          </Button>
        </div>
      )}

      {/* 正常模式下的左侧文件树侧边栏 */}
      {!soloMode && fileSidebarVisible && (isDebugMode || projectId) && (
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
                onFileOpen={handleFileOpen}
              />
            ) : (
              <div style={{ padding: 20, color: '#999', textAlign: 'center' }}>
                {isDebugMode ? '请输入工作目录' : '项目目录未设置'}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Solo 模式下：任务列表 + 消息区 + 沙箱 并排显示 */}
      {soloMode ? (
        /* Solo 模式：任务列表 + 消息区 + 沙箱 并排 */
        <div className="solo-mode-body">
          {/* 任务抽屉 */}
          <div className={`solo-task-drawer ${!taskDrawerOpen ? 'collapsed' : ''}`}>
            <TaskList
              tasks={soloTasks}
              activeThreadId={soloActiveTask?.id || null}
              onSelectTask={handleSelectSoloTask}
              onCreateTask={handleCreateSoloTask}
              onDeleteTask={(taskId) => {
                // 如果删除的是当前任务，清空状态
                if (soloActiveTask?.id === taskId) {
                  setSoloActiveTask(null);
                  clearDebugAll();
                }
                // 刷新任务列表
                loadSoloTasks();
              }}
              isRunning={debugStatus === 'running'}
            />
          </div>

          {/* 消息区 */}
          <div className="solo-mode-content">
            <div className="thread-view">
              {/* 消息区域 */}
              <div className="thread-messages">
              {messages.length === 0 ? (
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
                  agentConfigs={mentionableAgents}
                  projectPath={displayProjectPath}
                  toolEvents={toolEvents}
                  onStopAgent={handleStopAgent}
                  onRetryAgent={handleRetryAgent}
                  onOpenCodePanel={openCodePanel}
                  autoScroll={true}
                />
              )}
              <div ref={messagesEndRef} />
            </div>

            {/* 底部输入区 - 独立组件 */}
            <ThreadInput
              placeholder="输入您的需求..."
              loadingContext={loadingProjectContext}
              agentOptions={agentOptions}
              onSend={handleSend}
              prefilledMention={prefilledMention}
              onPrefillConsumed={() => setPrefilledMention(undefined)}
            />
            </div>
          </div>

          {/* Solo 模式下的右侧面板 */}
          {rightPanelVisible && (
            <>
              <div className={`resize-handle ${isResizing ? 'resizing' : ''}`} onMouseDown={handleResizeStart} style={{ width: isResizing ? 3 : 6 }} />
              <div style={{ position: 'relative', display: 'flex' }}>
                {isResizing && <div className="resize-overlay" />}
                <RightPanel
                  visible={rightPanelVisible}
                  onClose={closeRightPanel}
                  activeTab={rightPanelActiveTab}
                  onTabChange={setRightPanelActiveTab}
                  codeFiles={codeFiles}
                  expandedFiles={expandedFiles}
                  onToggleFile={toggleFileExpand}
                  sandboxServer={currentSandboxServer}
                  sandboxLoading={currentSandboxLoading}
                  dockerAvailable={dockerAvailable}
                  hasProjectPath={Boolean(getProjectPath())}
                  isDebugMode={isDebugMode}
                  onRunSandbox={handleRunSandbox}
                  onStopSandbox={handleStopSandbox}
                  width={rightPanelWidth}
                />
              </div>
            </>
          )}

          {/* Solo 模式下的状态栏 */}
          <StatusPanel width={320} threadId={threadId || debugThreadId || undefined} />
        </div>
      ) : (
        /* 非Solo模式：原有结构 */
        <>
          <div className="thread-view">
            {/* 干预控制面板 */}
            <div className="intervention-bar">
              <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                <Space>
                  <Tooltip title={fileSidebarVisible ? '隐藏文件树' : '显示文件树'}>
                    <Button icon={fileSidebarVisible ? <MenuFoldOutlined /> : <MenuUnfoldOutlined />} onClick={() => setFileSidebarVisible(!fileSidebarVisible)} size="small" />
                  </Tooltip>
                  <Button icon={<ArrowLeftOutlined />} onClick={() => isDebugMode ? navigate('/agents') : navigate(`/projects/${projectId}`)} size="small">
                    {isDebugMode ? '返回 Agent 列表' : '返回项目'}
                  </Button>
                  <Tag color={wsConnected ? 'green' : 'red'}>{wsConnected ? '已连接' : '未连接'}</Tag>
                  {isDebugMode && debugAgentConfig && <Tag color="purple">调试: {debugAgentConfig.name}</Tag>}
                  {isRunning && <Badge status="processing" text={`${activeAgents.length} 个 Agent 运行中`} />}
                </Space>
                <Space>
                  <Tooltip title="进入 Solo 模式">
                    <Button icon={<FullscreenOutlined />} onClick={toggleSoloMode} size="small" type={soloMode ? 'primary' : 'default'}>Solo</Button>
                  </Tooltip>
                  <Tooltip title={artifactsSidebarVisible ? '隐藏产物' : '查看产物列表'}>
                    <Button
                      icon={<UnorderedListOutlined />}
                      onClick={() => { setArtifactsSidebarVisible(!artifactsSidebarVisible); setRightPanelVisible(false); }}
                      size="small"
                      type={artifactsSidebarVisible ? 'primary' : 'default'}
                    >产物</Button>
                  </Tooltip>
                  <Tooltip title={rightPanelVisible ? '隐藏面板' : '打开代码/沙箱面板'}>
                    <Button
                      icon={<DesktopOutlined />}
                      onClick={() => {
                        setRightPanelVisible(!rightPanelVisible);
                        setFilePreviewVisible(false);  // 关闭文件预览，互斥
                        setArtifactsSidebarVisible(false);
                        setFileSidebarVisible(false);
                      }}
                      size="small"
                      type={rightPanelVisible || currentSandboxServer ? 'primary' : 'default'}
                    >面板</Button>
                  </Tooltip>
                </Space>
              </Space>
            </div>

            {/* 消息区域 */}
            <div className="thread-messages">
              {messages.length === 0 ? (
                <div style={{ textAlign: 'center', padding: '60px 20px', color: '#999' }}>
                  <RobotOutlined style={{ fontSize: 48, marginBottom: 16 }} />
                  <Title level={4} type="secondary">开始您的开发任务</Title>
                  <Text type="secondary">在下方输入您的需求，或使用 @需求分析师、@架构师、@开发者 等 Agent 协助开发</Text>
                </div>
              ) : (
                <ChatMessageList
                  messages={messages}
                  agentConfigs={mentionableAgents}
                  projectPath={displayProjectPath}
                  toolEvents={toolEvents}
                  onStopAgent={handleStopAgent}
                  onRetryAgent={handleRetryAgent}
                  onOpenCodePanel={openCodePanel}
                  autoScroll={true}
                />
              )}
              <div ref={messagesEndRef} />
            </div>

            {/* 底部输入区 - 独立组件 */}
            <ThreadInput
              placeholder="输入消息或使用 @需求分析师 @架构师 @开发者 等触发 Agent..."
              loadingContext={loadingProjectContext}
              agentOptions={agentOptions}
              onSend={handleSend}
              prefilledMention={prefilledMention}
              onPrefillConsumed={() => setPrefilledMention(undefined)}
            />

            {/* 检查点确认弹窗 */}
            <Modal
              title={<Space><ExclamationCircleOutlined style={{ color: '#faad14' }} /><span>{currentCheckpoint?.title || '确认检查点'}</span></Space>}
              open={checkpointModalVisible}
              onOk={handleCheckpointConfirm}
              onCancel={handleCheckpointReject}
              okText="确认通过"
              cancelText="需要修改"
              width={600}
            >
              <Alert type="info" message="请确认以下内容是否符合预期" description={currentCheckpoint?.content} showIcon style={{ marginBottom: 16 }} />
              <Text type="secondary">确认后将进入下一阶段，如需修改请点击"需要修改"并在对话中描述您的修改要求。</Text>
            </Modal>
          </div>

          {/* 产物侧边栏 */}
          {artifactsSidebarVisible && (
            <div className="artifacts-sidebar">
              <div className="artifacts-sidebar-header">
                <span>产物列表</span>
                <Button type="text" size="small" onClick={() => setArtifactsSidebarVisible(false)}>✕</Button>
              </div>
              <div className="artifacts-sidebar-content">
                {artifacts.length > 0 ? (
                  <List dataSource={artifacts} renderItem={renderArtifactItem} split style={{ padding: '12px 16px' }} />
                ) : (
                  <Empty description="暂无产物" image={Empty.PRESENTED_IMAGE_SIMPLE} style={{ padding: '40px 16px' }} />
                )}
                <Divider style={{ margin: '12px 16px' }} />
                {reviewResult && (
                  <div style={{ padding: '0 16px 16px' }}>
                    <Collapse defaultActiveKey={['review']}>
                      <Panel
                        header={<Space><ExclamationCircleOutlined /><span>审查状态</span>{reviewResult.decision === 'allow' ? <Tag color="green">可以放行</Tag> : <Tag color="red">{reviewResult.p1Issues + reviewResult.p2Issues} 个问题</Tag>}</Space>}
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

          {/* 右侧面板（代码/沙箱） */}
          {/* StatusPanel - 状态栏 */}
          <StatusPanel width={320} threadId={threadId || debugThreadId || undefined} />
          {/* 文件预览面板 */}
          {filePreviewVisible && filePreviewPath && (
            <FilePreviewPanel
              basePath={displayProjectPath}
              filePath={filePreviewPath}
              onClose={() => {
                setFilePreviewVisible(false);
                setFilePreviewPath(null);
              }}
              width={rightPanelWidth}
            />
          )}
          {rightPanelVisible && !filePreviewVisible && (
            <>
              <div className={`resize-handle ${isResizing ? 'resizing' : ''}`} onMouseDown={handleResizeStart} style={{ width: isResizing ? 3 : 6 }} />
              <div style={{ position: 'relative', display: 'flex' }}>
                {isResizing && <div className="resize-overlay" />}
                <RightPanel
                  visible={rightPanelVisible}
                  onClose={closeRightPanel}
                  activeTab={rightPanelActiveTab}
                  onTabChange={setRightPanelActiveTab}
                  codeFiles={codeFiles}
                  expandedFiles={expandedFiles}
                  onToggleFile={toggleFileExpand}
                  sandboxServer={currentSandboxServer}
                  sandboxLoading={currentSandboxLoading}
                  dockerAvailable={dockerAvailable}
                  hasProjectPath={Boolean(getProjectPath())}
                  isDebugMode={isDebugMode}
                  onRunSandbox={handleRunSandbox}
                  onStopSandbox={handleStopSandbox}
                  width={rightPanelWidth}
                />
              </div>
            </>
          )}
        </>
      )}

      {/* AskUserQuestion 弹窗 */}
      <QuestionModal
        visible={pendingQuestion !== null}
        questions={pendingQuestion?.questions || []}
        onSubmit={handleSubmitQuestionAnswer}
        onCancel={() => setPendingQuestion(null)}
      />
    </div>
  );
};

export default ThreadView;