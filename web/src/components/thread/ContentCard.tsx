// isdp/web/src/components/thread/ContentCard.tsx
import React, { useState, useEffect, memo, useCallback } from 'react';
import { Spin, Alert, Button, Space, Modal, Tooltip } from 'antd';
import {
  DownloadOutlined,
  CopyOutlined,
  FullscreenOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import mermaid from 'mermaid';
import type { ContentType } from '@/types/content';
import './ContentCard.css';

interface ContentCardProps {
  type: ContentType;
  content: string;
  title?: string;
  language?: string;
}

// 初始化 Mermaid 配置
mermaid.initialize({
  startOnLoad: false,
  theme: 'default',
  securityLevel: 'loose',
  flowchart: {
    useMaxWidth: true,
    htmlLabels: true,
    curve: 'basis',
  },
  sequence: {
    useMaxWidth: true,
    diagramMarginX: 8,
    diagramMarginY: 8,
  },
  themeVariables: {
    primaryColor: '#e6f7ff',
    primaryTextColor: '#1890ff',
    primaryBorderColor: '#91d5ff',
    lineColor: '#91d5ff',
    secondaryColor: '#f6ffed',
    tertiaryColor: '#fff7e6',
  },
});

// 生成唯一 ID
let diagramId = 0;
const generateDiagramId = () => `diagram-${Date.now()}-${++diagramId}`;

/**
 * 视觉内容卡片组件
 * 用于在气泡内展示架构图、错误日志等视觉内容
 *
 * 设计理念：技术精致风格
 * - 清晰的视觉层次
 * - 微妙的阴影和边框
 * - 流畅的交互动效
 * - 专业的排版
 */
export const ContentCard: React.FC<ContentCardProps> = memo(({
  type,
  content,
  title,
  language,
}) => {
  const [svg, setSvg] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>('');
  const [fullscreen, setFullscreen] = useState(false);
  const [copied, setCopied] = useState(false);

  // 渲染架构图
  useEffect(() => {
    if (type === 'diagram' && (language === 'mermaid' || !language)) {
      setLoading(true);
      setError('');
      const id = generateDiagramId();

      mermaid.render(id, content)
        .then(({ svg }) => {
          setSvg(svg);
          setLoading(false);
        })
        .catch((err) => {
          setError(err.message || '架构图渲染失败，请检查语法');
          setLoading(false);
        });
    }
  }, [type, language, content]);

  // 下载 SVG
  const handleDownload = useCallback(() => {
    if (!svg) return;
    const blob = new Blob([svg], { type: 'image/svg+xml' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${title || 'diagram'}.svg`;
    a.click();
    URL.revokeObjectURL(url);
  }, [svg, title]);

  // 复制源码
  const handleCopySource = useCallback(() => {
    navigator.clipboard.writeText(content);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }, [content]);

  // 获取图标和标题
  const getMeta = () => {
    switch (type) {
      case 'diagram':
        return { icon: '🖼️', label: '架构图', color: '#1890ff' };
      case 'chart':
        return { icon: '📊', label: '数据图表', color: '#52c41a' };
      case 'image':
        return { icon: '📷', label: '图片', color: '#722ed1' };
      case 'error-log':
        return { icon: '❌', label: '错误日志', color: '#f5222d' };
      default:
        return { icon: '📄', label: '内容', color: '#666' };
    }
  };

  const meta = getMeta();

  // 渲染错误日志
  if (type === 'error-log') {
    return (
      <div className="content-card content-card--error">
        <div className="content-card__header">
          <div className="content-card__title">
            <span className="content-card__icon">{meta.icon}</span>
            <span className="content-card__label">{title || meta.label}</span>
          </div>
          <Tooltip title={copied ? '已复制' : '复制日志'}>
            <Button
              size="small"
              type="text"
              icon={copied ? <CheckCircleOutlined style={{ color: '#52c41a' }} /> : <CopyOutlined />}
              onClick={handleCopySource}
              className="content-card__action"
            />
          </Tooltip>
        </div>
        <div className="content-card__body content-card__body--terminal">
          <pre className="error-log-content">{content}</pre>
        </div>
      </div>
    );
  }

  // 渲染架构图
  if (type === 'diagram') {
    return (
      <div className="content-card content-card--diagram">
        <div className="content-card__header">
          <div className="content-card__title">
            <span className="content-card__icon">{meta.icon}</span>
            <span className="content-card__label">{title || meta.label}</span>
          </div>
          <Space size={4} className="content-card__actions">
            <Tooltip title="全屏预览">
              <Button
                size="small"
                type="text"
                icon={<FullscreenOutlined />}
                onClick={() => setFullscreen(true)}
                className="content-card__action"
              />
            </Tooltip>
            <Tooltip title="下载 SVG">
              <Button
                size="small"
                type="text"
                icon={<DownloadOutlined />}
                onClick={handleDownload}
                disabled={!svg}
                className="content-card__action"
              />
            </Tooltip>
          </Space>
        </div>

        <div className="content-card__body">
          {loading && (
            <div className="content-card__loading">
              <Spin tip="渲染中..." />
            </div>
          )}

          {error && (
            <div className="content-card__error">
              <Alert
                type="warning"
                message="架构图渲染失败"
                description={
                  <div>
                    <p style={{ marginBottom: 8, color: '#666' }}>{error}</p>
                    <details>
                      <summary style={{ cursor: 'pointer', color: '#1890ff' }}>查看源码</summary>
                      <pre className="error-source">{content}</pre>
                    </details>
                  </div>
                }
                icon={<ExclamationCircleOutlined />}
                showIcon
              />
            </div>
          )}

          {!loading && !error && svg && (
            <div
              className="diagram-svg-container"
              dangerouslySetInnerHTML={{ __html: svg }}
            />
          )}
        </div>

        {/* 全屏预览模态框 */}
        <Modal
          open={fullscreen}
          onCancel={() => setFullscreen(false)}
          footer={null}
          width="90vw"
          centered
          className="diagram-modal"
          title={
            <div className="diagram-modal__title">
              <span className="content-card__icon">{meta.icon}</span>
              <span>{title || meta.label}</span>
            </div>
          }
        >
          <div
            className="diagram-fullscreen"
            dangerouslySetInnerHTML={{ __html: svg }}
          />
        </Modal>
      </div>
    );
  }

  // 其他类型暂不处理
  return null;
});

ContentCard.displayName = 'ContentCard';