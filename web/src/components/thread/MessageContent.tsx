// isdp/web/src/components/thread/MessageContent.tsx
import React, { memo, useMemo } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';
import 'highlight.js/styles/atom-one-dark.css';
import './MessageContent.css';

interface MessageContentProps {
  content: string;
  className?: string;
}

/**
 * 过滤掉 a2a-handoff 交接块（已在调用日志面板中单独展示）
 * 避免对话框中重复显示
 */
const filterA2AHandoff = (content: string): string => {
  // 移除 <a2a-handoff>...</a2a-handoff> 块
  // 使用非贪婪匹配，避免误删其他内容
  return content.replace(/<a2a-handoff>[\s\S]*?<\/a2a-handoff>/g, '').trim();
};

/**
 * 消息内容渲染组件
 * 支持 Markdown、代码高亮
 */
export const MessageContent: React.FC<MessageContentProps> = memo(({
  content,
  className = '',
}) => {
  // 过滤掉 a2a-handoff 交接块
  const filteredContent = useMemo(() => filterA2AHandoff(content), [content]);

  return (
    <div className={`message-content-wrapper ${className}`}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeHighlight]}
        components={{
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
                {language && <div className="code-block-header">{language}</div>}
                <button
                  className="code-copy-btn"
                  onClick={(e) => {
                    navigator.clipboard.writeText(codeContent);
                    const btn = e.currentTarget;
                    btn.textContent = '已复制';
                    setTimeout(() => { btn.textContent = '复制'; }, 1500);
                  }}
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
          h1: ({ children }) => <h1 className="message-h1">{children}</h1>,
          h2: ({ children }) => <h2 className="message-h2">{children}</h2>,
          h3: ({ children }) => <h3 className="message-h3">{children}</h3>,

          // 列表
          ul: ({ children }) => <ul className="message-list">{children}</ul>,
          ol: ({ children }) => <ol className="message-list">{children}</ol>,

          // 段落
          p: ({ children }) => <p className="message-paragraph">{children}</p>,
        }}
      >
        {filteredContent}
      </ReactMarkdown>
    </div>
  );
});

MessageContent.displayName = 'MessageContent';