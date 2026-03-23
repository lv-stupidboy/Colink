// isdp/web/src/components/thread/MessageContent.tsx
import React, { memo } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';
import 'highlight.js/styles/github-dark.css';
import './MessageContent.css';

interface MessageContentProps {
  content: string;
  className?: string;
}

/**
 * 消息内容渲染组件
 * 支持 Markdown、代码高亮、图片等
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
          // 代码块渲染
          code({ node, inline, className: codeClassName, children, ...props }: any) {
            const match = /language-(\w+)/.exec(codeClassName || '');
            const language = match ? match[1] : '';

            if (!inline && language) {
              // 块级代码
              return (
                <div className="code-block-wrapper">
                  <div className="code-block-header">
                    <span className="code-language">{language}</span>
                    <button
                      className="code-copy-btn"
                      onClick={(e) => {
                        const code = String(children).replace(/\n$/, '');
                        navigator.clipboard.writeText(code);
                        const btn = e.currentTarget;
                        btn.textContent = '已复制';
                        setTimeout(() => {
                          btn.textContent = '复制';
                        }, 2000);
                      }}
                    >
                      复制
                    </button>
                  </div>
                  <pre className={`code-block ${codeClassName}`}>
                    <code className={codeClassName} {...props}>
                      {children}
                    </code>
                  </pre>
                </div>
              );
            }

            // 行内代码
            return (
              <code className="inline-code" {...props}>
                {children}
              </code>
            );
          },

          // 图片渲染
          img({ node, src, alt, ...props }: any) {
            return (
              <div className="message-image-wrapper">
                <img
                  src={src}
                  alt={alt || ''}
                  className="message-image"
                  loading="lazy"
                  {...props}
                />
                {alt && <div className="message-image-caption">{alt}</div>}
              </div>
            );
          },

          // 链接渲染 - 新窗口打开
          a({ node, href, children, ...props }: any) {
            return (
              <a
                href={href}
                target="_blank"
                rel="noopener noreferrer"
                className="message-link"
                {...props}
              >
                {children}
              </a>
            );
          },

          // 表格渲染
          table({ node, children, ...props }: any) {
            return (
              <div className="message-table-wrapper">
                <table className="message-table" {...props}>
                  {children}
                </table>
              </div>
            );
          },

          // 引用块
          blockquote({ node, children, ...props }: any) {
            return (
              <blockquote className="message-blockquote" {...props}>
                {children}
              </blockquote>
            );
          },

          // 标题
          h1({ node, children, ...props }: any) {
            return <h1 className="message-h1" {...props}>{children}</h1>;
          },
          h2({ node, children, ...props }: any) {
            return <h2 className="message-h2" {...props}>{children}</h2>;
          },
          h3({ node, children, ...props }: any) {
            return <h3 className="message-h3" {...props}>{children}</h3>;
          },

          // 列表
          ul({ node, children, ...props }: any) {
            return <ul className="message-list" {...props}>{children}</ul>;
          },
          ol({ node, children, ...props }: any) {
            return <ol className="message-list message-list-ordered" {...props}>{children}</ol>;
          },

          // 段落
          p({ node, children, ...props }: any) {
            return <p className="message-paragraph" {...props}>{children}</p>;
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
});

MessageContent.displayName = 'MessageContent';