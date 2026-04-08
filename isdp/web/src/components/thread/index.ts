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

// 消息渲染组件
export { ChatMessage } from './ChatMessage';
export type { ProgressInfo, ProgressStatus } from './ChatMessage';
export { ChatMessageList } from './ChatMessageList';
export { FilePathLink, hasFilePath } from './FilePathLink';
export { MessageActions } from './MessageActions';

// 流式消息组件（隔离高频更新）
export { StreamingMessage } from './StreamingMessage';

// 独立输入组件（避免输入触发父组件重渲染）
export { ThreadInput } from './ThreadInput';

// 内容块组件
export { ReplyPill } from './ContentBlock/ReplyPill';
export { WhisperBadge } from './ContentBlock/WhisperBadge';
export { RichBlocks } from './ContentBlock/RichBlocks';