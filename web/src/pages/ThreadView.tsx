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
  TeamOutlined,
} from '@ant-design/icons';
import { useAppStore } from '@/store';
import { useDebugThreadStore } from '@/store/debugThread';
import type { Message, Artifact, ReviewIssue, MergeCheckResult, AgentConfig, ToolEvent, MessageContentBlock, ImageAttachment } from '@/types';
import type { FileChange } from '@/types/content';
import { ArtifactTypeLabels } from '@/types';
import { ReviewReport } from '@/components/ReviewReport';
import { RightPanel, ThreadInput } from '@/components/thread';
import { FilePreviewPanel } from '@/components/thread/FilePreviewPanel';
import { BlockingDetector } from '@/utils/blockingDetector';
import { sendAgentCompletionNotification, requestNotificationPermission, isNotificationGranted, clearPendingNotifications } from '@/utils/systemNotification';
import { ChatMessageList } from '@/components/thread/ChatMessageList';
import { StatusPanel } from '@/components/thread/StatusPanel';
import MessageScrollIndicator from '@/components/thread/MessageScrollIndicator';
import FileTree from '@/components/FileTree';
import api from '@/api/client';
import { snakeToCamel } from '@/api/transform';
import { AGENT_TYPE_COLORS } from '@/config/agentTypeColors';
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
  const chatMessageListRef = useRef<HTMLDivElement>(null);
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
  const currentWorkflowTemplate = useAppStore((s) => s.currentWorkflowTemplate);
  const debugAgentConfig = useAppStore((s) => s.debugAgentConfig);
  const debugProjectPath = useAppStore((s) => s.debugProjectPath);
  const sandboxServer = useAppStore((s) => s.sandboxServer);
  const sandboxLoading = useAppStore((s) => s.sandboxLoading);
  const dockerAvailable = useAppStore((s) => s.dockerAvailable);

  // 高频状态 - 只订阅 messages（流式消息由 StreamingMessage 组件独立处理）
  const workflowMessages = useAppStore((s) => s.messages);
  const workflowLoading = useAppStore((s) => s.loading);
  const workflowWsConnected = useAppStore((s) => s.wsConnected);
  // 消息分页状态
  const messagesHasMore = useAppStore((s) => s.messagesHasMore);
  const messagesLoadingMore = useAppStore((s) => s.messagesLoadingMore);
  const loadMoreMessages = useAppStore((s) => s.loadMoreMessages);

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
  const getFilteredAgents = useAppStore((s) => s.getFilteredAgents);
  const loadAgentConfigs = useAppStore((s) => s.loadAgentConfigs);
  const setDebugMode = useAppStore((s) => s.setDebugMode);
  const setDebugAgentConfig = useAppStore((s) => s.setDebugAgentConfig);
  const setDebugProjectPath = useAppStore((s) => s.setDebugProjectPath);
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
    setMessages: setDebugMessages,
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

  // Agent 类型列表（用于颜色动态分配）
  const [agentTypes, setAgentTypes] = useState<{ type: string }[]>([]);

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
  const [memoryRefreshKey, setMemoryRefreshKey] = useState(0);
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

  // Agent 完成后预填入状态
  const [prefilledMention, setPrefilledMention] = useState<string | undefined>(undefined);
  // 点击 Agent 头像/名称追加 @mention 状态
  const [appendMention, setAppendMention] = useState<string | undefined>(undefined);
  const [rightPanelActiveTab, setRightPanelActiveTab] = useState<'code' | 'sandbox'>('code');
  const [rightPanelWidth, setRightPanelWidth] = useState(520);
  const [isResizing, setIsResizing] = useState(false);
  const resizeStartX = useRef(0);
  const resizeStartWidth = useRef(0);

  // 代码文件列表
  const [codeFiles, setCodeFiles] = useState<FileChange[]>([]);
  const [expandedFiles, setExpandedFiles] = useState<Set<string>>(new Set());

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

  // 点击 Agent 头像/名称时追加 @mention
  const handleAgentClick = useCallback((agentName: string) => {
    setAppendMention(agentName);
  }, []);

  // 调试模式的 WebSocket 连接（带心跳检测支持）
  // 浏览器 WebSocket 会自动响应 ping 消息
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
    console.log('[WebSocket-Debug] Connecting to:', wsUrl);
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      if (wsRef.current === ws) {
        console.log('[WebSocket-Debug] Connected successfully');
        wsConnectedRef.current = true;
        setDebugWsConnected(true);
      }
    };

    ws.onclose = (event) => {
      if (wsRef.current === ws) {
        console.log('[WebSocket-Debug] Disconnected, code:', event.code, 'reason:', event.reason);
        wsConnectedRef.current = false;
        setDebugWsConnected(false);
      }
    };

    ws.onmessage = (event) => {
      // 确保这是当前的 WebSocket
      if (wsRef.current === ws) {
        const data = JSON.parse(event.data);
        // ping/pong 是协议层消息，不会触发 onmessage
        handleDebugWsMessage(data);
      }
    };

    ws.onerror = (error) => {
      console.error('[WebSocket-Debug] Error:', error);
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

      case 'memory_updated':
        setMemoryRefreshKey(value => value + 1);
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

    // 加载 agentTypes（两种模式都需要）
    api.baseAgents.getTypes().then(types => setAgentTypes(types)).catch(() => {});

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

  // 调试模式 - 初始化（检查 URL threadId 决定是否恢复会话）
  useEffect(() => {
    if (isDebugMode && agentId) {
      // 检查 URL 中是否有 threadId，决定是否恢复现有会话
      const hasExistingThread = Boolean(threadId);

      if (hasExistingThread) {
        // URL 中有 threadId，恢复现有会话
        setDebugMode(true, agentId);

        // 设置 threadId
        setDebugThreadId(threadId!);

        // 加载历史消息（直接替换，不追加）
        api.messages.list(threadId!).then(result => {
          // 从 metadata 中提取 agentName 设置到直接属性
          const processedMessages = (result.messages || []).map((msg: Message) => {
            const metadataAgentName = msg.metadata?.agentName as string | undefined;
            return {
              ...msg,
              agentName: msg.agentName || metadataAgentName || undefined,
            };
          });
          setDebugMessages(processedMessages);
        }).catch(err => {
          console.error('Failed to load messages:', err);
        });

        // 加载 Agent 配置
        api.agents.get(agentId).then((config: AgentConfig) => {
          setDebugAgentConfig(config);
        }).catch(err => {
          message.error('加载 Agent 配置失败');
          console.error(err);
        });
      } else {
        // URL 中没有 threadId，创建新会话
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
      }

      // 检查 Docker 可用性（无论新会话还是恢复）
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
  }, [isDebugMode, agentId, threadId]);

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
      // loadThread 已经加载了 thread 的 workflowTemplate
      // 这里只需要加载项目信息（用于 localPath）
      // loadProjectContext 不会覆盖已设置的 workflowTemplate
      const projectToLoad = projectId || currentThread?.projectId;
      if (projectToLoad && currentThread) {
        await loadProjectContext(projectToLoad);
      }
    };

    if (currentThread) {
      loadWorkflowContext();
    }
  }, [currentThread, projectId, loadProjectContext, isDebugMode]);

  
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
    console.log('[WebSocket-Team] Connecting to:', wsUrl);
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      if (wsRef.current === ws) {
        console.log('[WebSocket-Team] Connected successfully');
        wsConnectedRef.current = true;
        setWsConnected(true);
      }
    };

    ws.onclose = (event) => {
      if (wsRef.current === ws) {
        console.log('[WebSocket-Team] Disconnected, code:', event.code, 'reason:', event.reason);
        wsConnectedRef.current = false;
        setWsConnected(false);
      }
    };

    ws.onmessage = (event) => {
      // 确保这是当前的 WebSocket
      if (wsRef.current === ws) {
        const data = JSON.parse(event.data);
        // ping/pong 是 WebSocket 协议层消息，不会触发 onmessage
        handleWsMessage(data);
      }
    };

    ws.onerror = (error) => {
      console.error('[WebSocket-Team] Error:', error);
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
      case 'memory_updated':
        setMemoryRefreshKey(value => value + 1);
        break;

      case 'agent_output_chunk': {
        const chunkType = data.payload.chunkType as string || 'text';
        const invocId = data.payload.invocationId as string;

        if (chunkType === 'thinking') {
          // 思考块 - Store 会智能累积
          const thinkingText = data.payload.chunk as string;
          const isDone = data.payload.done as boolean;

          // 先追加内容（即使 done=true 也可能有最后一部分内容）
          if (thinkingText) {
            appendContentBlock(invocId, {
              id: `thinking-${invocId}`,
              type: 'thinking',
              content: thinkingText,
              timestamp: Date.now(),
              status: isDone ? 'success' : 'streaming',
            });
          }

          // 如果 done=true，确保状态更新为 success
          if (isDone) {
            updateContentBlock(invocId, `thinking-${invocId}`, { status: 'success' });
          }

          updateProgress(invocId, 'thinking');
        } else if (chunkType === 'tool_use') {
          // 工具调用块
          const toolName = data.payload.toolName as string;
          // 转换 input 字段为 camelCase（后端返回 snake_case）
          const rawInput = data.payload.toolInput as Record<string, unknown>;
          const toolInput = rawInput ? snakeToCamel(rawInput) : rawInput;
          const toolId = data.payload.toolId as string;
          const toolIndex = data.payload.toolIndex as number;
          updateProgress(invocId, 'tool_use', toolName, toolInput);

          // 追加工具调用块
          appendContentBlock(invocId, {
            id: toolId || `tool-${invocId}-${toolName}`,
            type: 'tool_use',
            toolName: toolName || 'Unknown',
            toolId: toolId || '',
            toolIndex: toolIndex,
            input: toolInput,
            timestamp: Date.now(),
            status: 'streaming',
            startedAt: Date.now(),
          });
        } else if (chunkType === 'input_json_delta') {
          // 工具参数增量更新 - 累积 partialJSON 并更新 input
          const toolIndex = data.payload.toolIndex as number;
          const partialJSON = data.payload.partialJSON as string;

          if (toolIndex !== undefined && partialJSON) {
            // 找到对应的 tool_use block（找最后一个匹配 toolIndex 的）
            const blocks = useAppStore.getState().streamingContentBlocks;
            // 反向查找，找到最新的匹配块
            let targetBlockIdx = -1;
            for (let i = blocks.length - 1; i >= 0; i--) {
              const b = blocks[i];
              if (b.type === 'tool_use' && (b as any).toolIndex === toolIndex) {
                targetBlockIdx = i;
                break;
              }
            }

            if (targetBlockIdx >= 0) {
              const targetBlock = blocks[targetBlockIdx] as any;
              // 累积 inputJSON
              const accumulatedJSON = (targetBlock.inputJSON || '') + partialJSON;

              // 尝试解析 JSON
              try {
                const parsedRaw = JSON.parse(accumulatedJSON);
                // 转换为 camelCase（后端返回 snake_case）
                const parsedInput = snakeToCamel(parsedRaw);
                // JSON 完整，更新 input
                updateContentBlock(invocId, targetBlock.id, {
                  input: parsedInput,
                  inputJSON: accumulatedJSON,
                });
                console.log('[WebSocket] input_json_delta parsed successfully', {
                  toolIndex,
                  blockId: targetBlock.id,
                  inputFields: Object.keys(parsedInput),
                });
              } catch (e) {
                // JSON 不完整，只更新 inputJSON，等待更多 delta
                updateContentBlock(invocId, targetBlock.id, {
                  inputJSON: accumulatedJSON,
                });
                console.log('[WebSocket] input_json_delta incomplete, waiting for more', {
                  toolIndex,
                  partialJSON: partialJSON.slice(0, 50),
                });
              }
            }
          }
        } else if (chunkType === 'tool_result') {
          // 工具调用结果 - 更新对应的工具块状态
          const toolId = data.payload.toolId as string;
          const isError = data.payload.isError as boolean;
          const toolOutput = data.payload.toolOutput as string;

          if (toolId) {
            // 从已保存的 contentBlocks 中获取正确的 startedAt 计算 duration
            const existingBlocks = useAppStore.getState().streamingContentBlocks;
            const existingBlock = existingBlocks.find(b => b.id === toolId);
            // 只有 tool_use 和 question 类型有 startedAt 属性
            const startedAt = (existingBlock && (existingBlock.type === 'tool_use' || existingBlock.type === 'question'))
              ? (existingBlock as any).startedAt
              : Date.now();

            updateContentBlock(invocId, toolId, {
              status: isError ? 'failed' : 'success',
              output: toolOutput,
              isError,
              duration: Date.now() - startedAt,
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
          // questions 可能直接在 payload.questions，也可能在 payload.input.questions
          const questions = (data.payload.questions as any[]) || ((data.payload.toolInput as any)?.questions as any[]) || [];
          const toolInput = data.payload.toolInput as Record<string, unknown>;
          const agentId = data.payload.agentId as string;
          const agentName = data.payload.agentName as string;

          // 调试日志：输出完整的 WebSocket payload
          console.log('[WebSocket] Received question chunk:', {
            payload: data.payload,
            agentId,
            agentName,
            toolId,
            toolName,
            questionsFromPayload: data.payload.questions,
            questionsFromToolInput: (data.payload.toolInput as any)?.questions,
            questionsCount: questions?.length,
          });

          // 构建要追加的 block 对象
          const blockToAppend = {
            id: `question-${toolId}`,
            type: 'question' as const,
            toolName: toolName || 'AskUserQuestion',
            toolId: toolId || '',
            invocationId: invocId,
            agentId: agentId || '',
            agentName: agentName || '',
            questions: questions || [],
            input: toolInput,
            timestamp: Date.now(),
            status: 'waiting_user_input' as const,  // 使用 as const 确保类型为 ContentBlockStatus
            startedAt: Date.now(),
          };

          console.log('[WebSocket] Appending question block:', blockToAppend);

          // 追加问题块（保存 agentName 用于 resume 调用）
          appendContentBlock(invocId, blockToAppend);

          // 不再使用弹窗，问题选项直接内联显示在对话中
        }
        break;
      }
      case 'agent_message': {
        // Agent 完成消息：后端已保存到数据库并广播真实ID
        // payload 包含: messageId（真实UUID）、invocationId、agentId、content、contentBlocks、agentName、agentRole、metadata
        const realMessageId = data.payload.messageId as string;
        const invocationIdFromPayload = data.payload.invocationId as string;
        const agentId = data.payload.agentId as string;
        const content = data.payload.content as string;
        // 转换 contentBlocks 为 camelCase（后端返回 snake_case）
        const rawContentBlocks = data.payload.contentBlocks as MessageContentBlock[] | undefined;
        const contentBlocks = rawContentBlocks ? snakeToCamel(rawContentBlocks) : rawContentBlocks;
        const agentName = data.payload.agentName as string;
        const agentRole = data.payload.agentRole as string;
        const metadataFromPayload = data.payload.metadata as Record<string, unknown> | undefined;

        if (!realMessageId) {
          break;
        }

        // 使用 getState() 避免闭包陷阱
        const state = useAppStore.getState();
        const invocationId = state.streamingInvocationId;

        // 检查是否有临时消息需要替换ID（流式场景）
        if (invocationId && state.isStreaming) {
          const tempId = `agent-${invocationId}`;
          // 先完成流式消息（添加临时消息）- finalizeStreamingMessage 会将 question block IDs 加入 submittedQuestionBlockIds
          finalizeStreamingMessage(invocationId);
          // 然后用真实ID和真实contentBlocks替换临时消息（避免重复渲染 question blocks）
          // 同时更新 agentName 和 agentRole 以及 metadata
          useAppStore.getState().replaceMessageId(tempId, realMessageId, contentBlocks, agentName, agentRole, metadataFromPayload);
        } else {
          // 非流式场景：检查是否已有临时消息
          // 优先使用 payload 中的 invocationId（取消/中断场景下前端 invocationId 已被重置）
          const tempId = invocationIdFromPayload ? `agent-${invocationIdFromPayload}` : `agent-${realMessageId}`;
          const existingTemp = state.messages.find(m => m.id === tempId);
          if (existingTemp) {
            // 替换临时ID为真实ID，同时替换contentBlocks和metadata
            useAppStore.getState().replaceMessageId(tempId, realMessageId, contentBlocks, agentName, agentRole, metadataFromPayload);
          } else {
            // 直接添加新消息（使用真实ID）
            addMessage({
              id: realMessageId,
              threadId: threadId!,
              role: 'agent',
              agentId: agentId,
              agentName: agentName,  // 设置直接属性
              content: content,
              contentBlocks: contentBlocks,
              messageType: 'text',
              metadata: metadataFromPayload || {
                agentName: agentName,
                agentRole: agentRole,
              },
              createdAt: new Date().toISOString(),
            });

            // 如果 contentBlocks 包含已提交的 question blocks，将其 IDs 加入 submittedQuestionBlockIds
            // waiting_user_input 状态的 question blocks 应保留在历史消息中渲染（等待用户响应）
            if (contentBlocks && contentBlocks.length > 0) {
              const submittedQuestionBlockIds = contentBlocks
                .filter(b => b.type === 'question' && (b.status === 'success' || b.status === 'failed'))
                .map(b => b.id);
              submittedQuestionBlockIds.forEach(id => useAppStore.getState().markQuestionSubmitted(id));
            }
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
        // failed 状态时获取详细错误信息
        const errorDetails = status === 'failed' ? (data.payload.errorDetails as string | undefined) : undefined;
        console.log('[agent_status] Received:', { status, invocId, agentName, agentId, hasErrorDetails: !!errorDetails });
        // failed 状态时传递 errorDetails 作为 input（用于保存错误信息）
        updateAgentStatus(invocId, status, agentName, errorDetails || input);
        // Agent 完成或中断时清理工具事件（包括 cancelled）
        if (status === 'completed' || status === 'failed' || status === 'interrupted' || status === 'cancelled') {
          clearToolEvents(invocId);

          // Agent 完成后预填入：设置上一个对话的 Agent 名称
          // 只在 completed 状态且非调试模式下预填入（interrupted 不预填入，用户需要回答问题）
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
          contextUsed?: number;   // ACP: session context 已使用量
          contextSize?: number;   // ACP: session context 总容量
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
    // debugThreadId 为空表示还没有会话
    const needNewSession = !debugThreadId;

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
      await spawnAgent('agent', '请重新处理上一次的任务', agentId);
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
    requiresHuman: agent.requiresHuman,
    isSystem: agent.isSystem,
    name: agent.name,
    label: agent.name,
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
   * 处理内联 AskUserQuestion 用户答案提交
   * 采用 --resume 方案：用户响应发送自然语言消息，触发新的 SpawnAgent 恢复会话
   *
   * 关键改进：在答案前加上 @AgentName 前缀，确保后端使用 resume 调用正确的 Agent
   */
  const handleInlineQuestionSubmit = useCallback(async (blockId: string, answers: Record<number, string | string[]>, invocationId: string) => {
    if (!threadId) {
      console.error('[handleInlineQuestionSubmit] No threadId available');
      return;
    }

    try {
      // 获取提出问题的 Agent 名称（从 contentBlocks 中获取）
      const state = useAppStore.getState();
      const streamingContentBlocks = state.streamingContentBlocks;

      // 先从 streamingContentBlocks 查找（最新数据）
      let questionBlock = streamingContentBlocks.find(b => b.id === blockId);

      // 如果 streamingContentBlocks 中找不到（可能已被 finalizeStreamingMessage 清空）
      // 则从 messages 的 contentBlocks 中查找（备选方案）
      if (!questionBlock) {
        console.log('[handleInlineQuestionSubmit] Not found in streamingContentBlocks, searching in messages');
        for (const msg of state.messages) {
          if (msg.contentBlocks) {
            const found = msg.contentBlocks.find(b => b.id === blockId);
            if (found && found.type === 'question') {
              questionBlock = found;
              console.log('[handleInlineQuestionSubmit] Found in message:', msg.id, 'block:', found);
              break;
            }
          }
        }
      }

      // 调试日志：输出完整的查找过程
      console.log('[handleInlineQuestionSubmit] Looking for question block:', {
        blockId,
        invocationId,
        streamingContentBlocksCount: streamingContentBlocks.length,
        streamingContentBlocksIds: streamingContentBlocks.map(b => b.id),
        foundInStreaming: streamingContentBlocks.find(b => b.id === blockId),
        foundBlock: questionBlock,
        agentName: (questionBlock as any)?.agentName,
      });

      const agentName = (questionBlock as any)?.agentName || '';
      const invocationIdFromBlock = (questionBlock as any)?.invocationId || invocationId;
      const toolId = (questionBlock as any)?.toolId || '';

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
      const userResponseContent = answerStrings.join('\n');

      // === 提交路径分流 ===
      // ACP elicitation 模式：agent 仍 running（isStreaming=true），CLI 在同一 prompt turn
      // 内异步等 elicitation/create 的 SendResponse(accept, content)。这条路径走
      // submitQuestionAnswer API，后端会调 SendToolResult → SDK 内置 AskUserQuestion 工具
      // 在 updatedInput 上拿到答案后继续推 chunk，**不启动新的 invocation**。
      //
      // native CLI 模式：AskUserQuestion 触发 stdin 关闭 → CLI 进程被 cancel →
      // invocation interrupted（isStreaming=false）。这条路径靠"前端发一条 @AgentName
      // 新消息 → SpawnAgentForUserMessage → resume"重启来续上对话。
      const isAgentStillRunning = useAppStore.getState().isStreaming;

      // 更新内容块状态为 success（用户已响应，UI 立刻反馈）
      // 显示用户原始选择内容（不带 @mention 前缀，因为 ACP 路径下我们不发 @mention）
      updateContentBlock(invocationId, blockId, {
        status: 'success',
        output: userResponseContent,
        completedAt: Date.now(),
      });

      if (isAgentStillRunning && toolId) {
        // === ACP elicitation 路径 ===
        console.log('[handleInlineQuestionSubmit] ACP elicitation path', { toolId, invocationIdFromBlock, answer: userResponseContent });
        try {
          await api.agents.submitQuestionAnswer(threadId, toolId, userResponseContent);
          message.success('答案已提交');
        } catch (e) {
          console.error('[handleInlineQuestionSubmit] submitQuestionAnswer failed', e);
          message.error('答案提交失败：' + (e as Error).message);
          return;
        }
      } else {
        // === native CLI 路径（保留原有 @mention 重启逻辑） ===
        // 关键：在答案前加上 @AgentName 前缀
        // 这样后端 SpawnAgentForUserMessage 会解析 @mention
        // shouldUseResumeStrategy 会判断用户 @ 的是同一个 Agent（与最后一个完成的相同）
        // 从而使用 resume 会话策略，调用正确的 Agent
        const userResponseMessage = agentName
          ? `@${agentName} ${userResponseContent}`
          : userResponseContent;

        console.log('[handleInlineQuestionSubmit] native @mention path', {
          agentName,
          finalMessage: userResponseMessage,
          blockId,
          invocationId
        });

        // native 路径下展示带 @mention 的完整消息
        updateContentBlock(invocationId, blockId, {
          status: 'success',
          output: userResponseMessage,
          completedAt: Date.now(),
        });

        await sendMessage(userResponseMessage);
        message.success('答案已提交，Agent 正在处理...');
      }

      // 调用 API 关闭待办任务（两条路径都要做）
      try {
        await api.humanTasks.completeByInvocation(invocationIdFromBlock);
        console.log('[handleInlineQuestionSubmit] 待办任务已关闭, invocationId:', invocationIdFromBlock);
      } catch (e) {
        console.warn('[handleInlineQuestionSubmit] 关闭待办任务失败:', e);
      }
    } catch (error) {
      console.error('提交答案失败:', error);
      message.error('提交答案失败，请重试');
    }
  }, [threadId, updateContentBlock, sendMessage]);

  /**
   * 处理 Rich Block 交互动作
   * 将用户的交互动作（如确认、选择）反馈给 agent
   */
  const handleInteractiveAction = useCallback(async (blockId: string, action: string, value?: string | string[]) => {
    if (!threadId) {
      console.error('[handleInteractiveAction] No threadId available');
      return;
    }

    try {
      // 获取提出问题的 Agent 名称（从 streamingContentBlocks 或 messages 中获取）
      const state = useAppStore.getState();

      // 先从 streamingContentBlocks 查找 rich block
      let richBlock = state.streamingContentBlocks.find(b => b.id === blockId);

      // 如果找不到，从 messages 的 contentBlocks 中查找
      if (!richBlock) {
        for (const msg of state.messages) {
          if (msg.contentBlocks) {
            const found = msg.contentBlocks.find(b => b.id === blockId);
            if (found && found.type === 'rich') {
              richBlock = found;
              break;
            }
          }
        }
      }

      // 获取 agentName（如果有的话）
      const agentName = (richBlock as any)?.agentName || state.streamingAgentName || '';

      // 构建交互消息
      let actionMessage = '';
      if (action === 'confirm') {
        actionMessage = '已确认方案';
      } else if (action === 'cancel') {
        actionMessage = '已取消';
      } else if (action === 'select' && value) {
        actionMessage = `已选择: ${value}`;
      } else if (action === 'multi_select' && Array.isArray(value)) {
        actionMessage = `已选择: ${value.join('、')}`;
      } else {
        actionMessage = `交互动作: ${action}`;
        if (value) {
          actionMessage += ` - ${Array.isArray(value) ? value.join('、') : value}`;
        }
      }

      // 加上 @AgentName 前缀（如果有）
      const finalMessage = agentName ? `@${agentName} ${actionMessage}` : actionMessage;

      console.log('[handleInteractiveAction] Sending interactive action:', {
        blockId,
        action,
        value,
        agentName,
        finalMessage,
      });

      // 发送消息，触发 agent 继续执行
      await sendMessage(finalMessage);

      message.success('交互已确认，Agent 正在处理...');
    } catch (error) {
      console.error('[handleInteractiveAction] Failed:', error);
      message.error('交互确认失败，请重试');
    }
  }, [threadId, sendMessage]);

  /**
   * 处理发送消息
   * 调试模式：直接发送给当前 Agent
   * 团队模式：支持 @mention 触发特定 Agent
   */
  const handleSend = useCallback(async (content: string, images?: ImageAttachment[]) => {
    if (!content.trim() && !images?.length) return;

    // 用户发送新消息时，清除之前的阻塞项（开始新的交互）
    const state = useAppStore.getState();
    if (state.blockingItems.length > 0) {
      state.blockingItems.forEach(b => removeBlockingItem(b.id));
    }

    // 清除累积的通知计数（开始新一轮任务）
    clearPendingNotifications();

    // 在触发 Agent 时请求系统通知权限（首次）
    if (!isNotificationGranted()) {
      requestNotificationPermission();
    }

    // 调试模式
    if (isDebugMode) {
      await handleDebugSend(content);
      return;
    }

    // 团队模式 - 检查是否是 @mention 命令
    // 修复：使用 slice 剥离 @mention 部分，支持多行文本（正则 .* 不匹配换行符）
    const mentionMatch = content.match(/^@(\S+)\s*/);
    if (mentionMatch) {
      const agentName = mentionMatch[1].toLowerCase();
      // 剥离 @mention 部分，剩余内容作为 input（支持多行）
      const input = content.slice(mentionMatch[0].length) || content;

      const agentByName = agentOptions.find(opt =>
        opt.name.toLowerCase() === agentName ||
        opt.label.toLowerCase() === agentName
      );
      if (agentByName) {
        await sendMessage(content, true, images);
        await spawnAgent('agent', input, agentByName.id, images);
        return;
      }

      message.warning(`未找到 Agent: ${agentName}，请从下拉列表中选择`);
      return;
    } else {
      await sendMessage(content, false, images);
    }
  }, [isDebugMode, agentOptions, sendMessage, spawnAgent, handleDebugSend]);

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
    <div className="thread-view-wrapper">
      {/* 左侧文件树侧边栏 */}
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

      {/* 消息区域 */}
        <div className="thread-view">
          {/* 干预控制面板 */}
          <div className="intervention-bar">
            <Space style={{ width: '100%', justifyContent: 'space-between' }}>
              <Space>
                <Tooltip title={fileSidebarVisible ? '隐藏文件树' : '显示文件树'}>
                  <Button icon={fileSidebarVisible ? <MenuFoldOutlined /> : <MenuUnfoldOutlined />} onClick={() => setFileSidebarVisible(!fileSidebarVisible)} size="small" />
                </Tooltip>
                <Button icon={<ArrowLeftOutlined />} onClick={() => isDebugMode ? navigate('/agents') : navigate(`/projects/${projectId || currentThread?.projectId}`)} size="small">
                  {isDebugMode ? '返回 Agent 列表' : '返回项目'}
                </Button>
                <Tag color={wsConnected ? 'green' : 'red'}>{wsConnected ? '已连接' : '未连接'}</Tag>
                {isDebugMode && debugAgentConfig && <Tag color="purple">调试: {debugAgentConfig.name}</Tag>}
                {isRunning && <Badge status="processing" text={`${activeAgents.length} 个 Agent 运行中`} />}
              </Space>
              {/* 中间：任务名和团队名 */}
              <Space style={{ flex: 1, justifyContent: 'center' }}>
                {currentThread?.name && (
                  <Text strong style={{ fontSize: 14 }}>{currentThread.name}</Text>
                )}
                {currentWorkflowTemplate?.name && (
                  <Tag icon={<TeamOutlined />} color="blue">{currentWorkflowTemplate.name}</Tag>
                )}
              </Space>
              <Space>
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

          {/* 消息列表 */}
          <div className="thread-messages">
            {messages.length === 0 ? (
              <div style={{ textAlign: 'center', padding: '60px 20px', color: '#999' }}>
                <RobotOutlined style={{ fontSize: 48, marginBottom: 16 }} />
                <Title level={4} type="secondary">开始您的开发任务</Title>
                <Text type="secondary">在下方输入您的需求，或使用 @需求分析师、@架构师、@开发者 等 Agent 协助开发</Text>
              </div>
            ) : (
              <>
                <ChatMessageList
                  ref={chatMessageListRef}
                  messages={messages}
                  agentConfigs={mentionableAgents}
                  projectPath={displayProjectPath}
                  toolEvents={toolEvents}
                  agentTypes={agentTypes}
                  onStopAgent={handleStopAgent}
                  onRetryAgent={handleRetryAgent}
                  onOpenCodePanel={openCodePanel}
                  autoScroll={true}
                  onQuestionSubmit={handleInlineQuestionSubmit}
                  onInteractiveAction={handleInteractiveAction}
                  hasMoreHistory={!isDebugMode && messagesHasMore}
                  loadingMore={!isDebugMode && messagesLoadingMore}
                  onLoadMore={!isDebugMode ? loadMoreMessages : undefined}
                  onAgentClick={handleAgentClick}
                />
                <MessageScrollIndicator
                  messages={messages}
                  agentConfigs={mentionableAgents}
                  containerRef={chatMessageListRef}
                />
              </>
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
            appendMention={appendMention}
            onAppendConsumed={() => setAppendMention(undefined)}
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
          <StatusPanel width={320} threadId={threadId || debugThreadId || undefined} projectPath={displayProjectPath} memoryRefreshKey={memoryRefreshKey} />
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
        </div>
  );
};

export default ThreadView;
