// isdp/web/src/components/thread/index.ts
export { MessageCard } from './MessageCard';
export { MessageInput } from './MessageInput';
export { SandboxPanel } from './SandboxPanel';
export { MessageContent } from './MessageContent';
export { ContentCard } from './ContentCard';
export { CodePreviewButton } from './CodePreviewButton';
export { CodePanel } from './CodePanel';
export { RightPanel } from './RightPanel';
export { TaskList } from './TaskList';

// 新增消息渲染组件
export { ChatMessage } from './ChatMessage';
export type { ProgressInfo, ProgressStatus } from './ChatMessage';
export { ChatMessageList } from './ChatMessageList';
export { MessageContentEnhanced } from './MessageContentEnhanced';
export { ThinkingBlock, BrainIcon, hasThinkingContent, getThinkingContent } from './ThinkingBlock';
export { FilePathLink, hasFilePath } from './FilePathLink';

// CLI 输出块和工具行组件
export { CliOutputBlock } from './CliOutputBlock';
export { ToolRow } from './ToolRow';
export { MessageActions } from './MessageActions';

// 流式消息组件（隔离高频更新）
export { StreamingMessage } from './StreamingMessage';

// 独立输入组件（避免输入触发父组件重渲染）
export { ThreadInput } from './ThreadInput';