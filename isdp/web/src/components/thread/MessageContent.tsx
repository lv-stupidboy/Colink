// isdp/web/src/components/thread/MessageContent.tsx
import React, { memo } from 'react';
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
 * 消息内容渲染组件
 * 支持 Markdown、代码高亮
 */
export const MessageContent: React.FC<MessageContentProps> = memo(({
  content,
  className = '',
}) => {
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
        {content}
      </ReactMarkdown>
    </div>
  );
});

MessageContent.displayName = 'MessageContent';