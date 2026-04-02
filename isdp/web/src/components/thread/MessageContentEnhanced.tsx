// isdp/web/src/components/thread/MessageContentEnhanced.tsx
import React, { memo, useMemo, useCallback } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';
import 'highlight.js/styles/atom-one-dark.css';
import type { AgentConfig } from '@/types';
import { highlightMentions } from '@/utils/mentionHighlight';
import { FilePathLink } from './FilePathLink';
import './MessageContent.css';

/**
 * 增强的消息内容渲染组件
 * 支持 Markdown + @提及高亮 + 文件路径链接
 *
 * 性能优化：
 * - 使用 memo 避免不必要重渲染
 * - 使用 useMemo 缓存 markdown 组件
 * - 使用 useCallback 缓存回调函数
 */

interface MessageContentEnhancedProps {
  content: string;
  agentConfigs?: AgentConfig[];
  projectPath?: string;
  className?: string;
}

/**
 * 预处理文本：处理 @提及和文件路径
 * 将文本分割为需要特殊处理的片段
 */
function preprocessText(
  text: string,
  agentConfigs: AgentConfig[],
  projectPath?: string
): React.ReactNode[] {
  // 先处理 @提及高亮
  const mentionHighlighted = highlightMentions(text, agentConfigs);

  // 然后处理文件路径链接
  return mentionHighlighted.map((part) => {
    if (typeof part === 'string') {
      // 对字符串部分处理文件路径
      return <FilePathLink text={part} projectPath={projectPath} />;
    }
    // 已经是 React 元素（@提及），保持不变
    return part;
  });
}

/**
 * 增强的消息内容组件
 */
export const MessageContentEnhanced: React.FC<MessageContentEnhancedProps> = memo(({
  content,
  agentConfigs = [],
  projectPath,
  className = '',
}) => {
  // 缓存复制按钮点击处理
  const handleCopy = useCallback((e: React.MouseEvent<HTMLButtonElement>, codeContent: string) => {
    navigator.clipboard.writeText(codeContent);
    const btn = e.currentTarget;
    btn.textContent = '已复制';
    setTimeout(() => { btn.textContent = '复制'; }, 1500);
  }, []);

  // 缓存 markdown 组件定义，避免每次渲染重新创建
  const markdownComponents = useMemo(() => ({
    // 文本节点：处理 @提及和文件路径
    text({ children }: { children?: React.ReactNode }) {
      const textContent = String(children ?? '');
      if (!textContent) return null;

      // 检查是否包含需要处理的内容
      const hasSpecialContent =
        textContent.includes('@') ||
        /[A-Za-z]:\\|\/[\w.-]+\.[\w]+/.test(textContent);

      if (hasSpecialContent && agentConfigs.length > 0) {
        return <>{preprocessText(textContent, agentConfigs, projectPath)}</>;
      }

      return <>{textContent}</>;
    },

    // 代码块
    code({ inline, className: codeClassName, children, ...props }: any) {
      if (inline) {
        return <code className="inline-code" {...props}>{children}</code>;
      }
      // 块级代码
      const match = /language-(\w+)/.exec(codeClassName || '');
      const language = match ? match[1] : '';
      const codeContent = String(children).replace(/\n$/, '');

      return (
        <div className="code-block-wrapper">
          {language && (
            <div className="code-block-header">
              <span className="code-block-lang">{language}</span>
            </div>
          )}
          <button
            className="code-copy-btn"
            onClick={(e) => handleCopy(e, codeContent)}
          >
            复制
          </button>
          <pre className="code-block">
            <code className={codeClassName} {...props}>{children}</code>
          </pre>
        </div>
      );
    },

    // 链接
    a({ href, children }: any) {
      // 如果是 vscode:// 协议，保持原样
      if (href?.startsWith('vscode://')) {
        return (
          <a href={href} className="filepath-link" onClick={(e) => {
            e.preventDefault();
            window.open(href, '_blank');
          }}>
            {children}
          </a>
        );
      }
      return (
        <a href={href} target="_blank" rel="noopener noreferrer" className="message-link">
          {children}
        </a>
      );
    },

    // 表格
    table({ children }: any) {
      return (
        <div className="message-table-wrapper">
          <table className="message-table">{children}</table>
        </div>
      );
    },

    // 引用块
    blockquote({ children }: any) {
      return <blockquote className="message-blockquote">{children}</blockquote>;
    },

    // 标题
    h1: ({ children }: any) => <h1 className="message-h1">{children}</h1>,
    h2: ({ children }: any) => <h2 className="message-h2">{children}</h2>,
    h3: ({ children }: any) => <h3 className="message-h3">{children}</h3>,

    // 列表
    ul: ({ children }: any) => <ul className="message-list">{children}</ul>,
    ol: ({ children }: any) => <ol className="message-list">{children}</ol>,

    // 段落
    p: ({ children }: any) => <p className="message-paragraph">{children}</p>,
  }), [agentConfigs, projectPath, handleCopy]);

  return (
    <div className={`message-content-wrapper ${className}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeHighlight]}
        components={markdownComponents}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
});

MessageContentEnhanced.displayName = 'MessageContentEnhanced';