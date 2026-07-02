import React, { useMemo } from 'react';
import type { TextBlock as TextBlockType, AgentConfig } from '@/types';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { highlightMentions } from '@/utils/mentionHighlight';
import './ContentBlock.css';

interface TextBlockProps {
  block: TextBlockType;
  agentConfigs?: AgentConfig[];
}

/**
 * 文本块组件
 *
 * 使用 GFM (GitHub Flavored Markdown) 渲染，支持表格、任务列表等
 * 同时高亮 @mention 内容
 *
 * 性能优化：
 * - useMemo 缓存 Markdown 解析结果，避免重复解析已完成内容
 * - 只在 block.content 变化时才重新解析
 */
const TextBlock: React.FC<TextBlockProps> = ({ block, agentConfigs = [] }) => {
  // 检查是否有 @mention
  function hasMentions(text: string): boolean {
    return /@[^\s@]+/.test(text);
  }

  // 自定义段落渲染，处理 @mention
  const customComponents = useMemo(() => ({
    p: ({ children }: { children?: React.ReactNode }) => {
      // 如果 children 是字符串，处理 @mention
      if (typeof children === 'string' && hasMentions(children)) {
        return <p>{highlightMentions(children, agentConfigs)}</p>;
      }
      // 处理 children 数组
      if (Array.isArray(children)) {
        const processed = children.map((child, index) => {
          if (typeof child === 'string' && hasMentions(child)) {
            return <React.Fragment key={index}>{highlightMentions(child, agentConfigs)}</React.Fragment>;
          }
          return child;
        });
        return <p>{processed}</p>;
      }
      return <p>{children}</p>;
    },
    // 处理列表项中的文本
    li: ({ children }: { children?: React.ReactNode }) => {
      if (typeof children === 'string' && hasMentions(children)) {
        return <li>{highlightMentions(children, agentConfigs)}</li>;
      }
      if (Array.isArray(children)) {
        const processed = children.map((child, index) => {
          if (typeof child === 'string' && hasMentions(child)) {
            return <React.Fragment key={index}>{highlightMentions(child, agentConfigs)}</React.Fragment>;
          }
          return child;
        });
        return <li>{processed}</li>;
      }
      return <li>{children}</li>;
    },
  }), [agentConfigs]);

  // ✅ 性能优化：缓存 Markdown 解析结果
  // 只在 block.content 变化时才重新解析，避免重复解析已完成内容
  const markdownContent = useMemo(() => (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      components={customComponents}
    >
      {block.content}
    </ReactMarkdown>
  ), [block.content, customComponents]);

  return (
    <div className="text-block">
      {markdownContent}
    </div>
  );
};

export default React.memo(TextBlock);