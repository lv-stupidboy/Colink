import React, { useState, useEffect } from 'react';
import { Modal, Typography, Space, Tag, Button, Spin, Empty, message } from 'antd';
import {
  FileTextOutlined,
  CodeOutlined,
  FileSearchOutlined,
  DownloadOutlined,
  CopyOutlined,
  FullscreenOutlined,
} from '@ant-design/icons';
import type { Artifact } from '@/types';
import { ArtifactTypeLabels } from '@/types';

const { Text } = Typography;

interface ArtifactPreviewProps {
  artifact: Artifact | null;
  visible: boolean;
  onClose: () => void;
}

/**
 * 产物预览组件
 * PRD Section 2.1.4-2.1.6 - 产物展示设计
 *
 * 支持：
 * - 架构图预览 (Mermaid)
 * - API 文档预览 (Markdown)
 * - 代码文件预览 (代码高亮)
 * - 审查报告展示
 */
export const ArtifactPreview: React.FC<ArtifactPreviewProps> = ({
  artifact,
  visible,
  onClose,
}) => {
  const [loading, setLoading] = useState(false);
  const [content, setContent] = useState<string>('');
  const [fullscreen, setFullscreen] = useState(false);

  useEffect(() => {
    if (visible && artifact) {
      loadContent();
    }
  }, [visible, artifact]);

  const loadContent = async () => {
    if (!artifact) return;

    setLoading(true);
    try {
      // 如果产物已经有内容，直接使用
      if (artifact.content) {
        setContent(artifact.content);
      } else {
        // 否则从 API 获取
        const data = await fetch(`/api/v1/artifacts/${artifact.id}`);
        const result = await data.json();
        setContent(result.content || '');
      }
    } catch (error) {
      message.error('加载产物内容失败');
      setContent('');
    } finally {
      setLoading(false);
    }
  };

  const handleCopy = () => {
    if (content) {
      navigator.clipboard.writeText(content);
      message.success('已复制到剪贴板');
    }
  };

  const handleDownload = () => {
    if (!artifact || !content) return;

    const blob = new Blob([content], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = artifact.name || 'artifact';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    message.success('下载成功');
  };

  /**
   * 渲染代码预览
   */
  const renderCode = () => (
    <div className="artifact-preview-code">
      {loading ? (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <Spin />
        </div>
      ) : content ? (
        <pre
          style={{
            background: '#1e1e1e',
            color: '#d4d4d4',
            padding: 16,
            borderRadius: 8,
            overflow: 'auto',
            maxHeight: fullscreen ? '80vh' : 400,
            fontSize: 13,
            fontFamily: 'Consolas, Monaco, "Courier New", monospace',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
          }}
        >
          {content}
        </pre>
      ) : (
        <Empty description="暂无内容" />
      )}
    </div>
  );

  /**
   * 渲染文档预览 (Markdown)
   * TODO: 需要集成 markdown 渲染库
   */
  const renderDocument = () => (
    <div className="artifact-preview-document">
      {loading ? (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <Spin />
        </div>
      ) : content ? (
        <div
          style={{
            background: '#fff',
            padding: 24,
            borderRadius: 8,
            border: '1px solid #f0f0f0',
            maxHeight: fullscreen ? '80vh' : 400,
            overflow: 'auto',
          }}
          className="markdown-body"
        >
          <pre style={{ whiteSpace: 'pre-wrap' }}>{content}</pre>
        </div>
      ) : (
        <Empty description="暂无内容" />
      )}
    </div>
  );

  /**
   * 渲染配置文件预览
   */
  const renderConfig = () => (
    <div className="artifact-preview-config">
      {loading ? (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <Spin />
        </div>
      ) : content ? (
        <pre
          style={{
            background: '#f5f5f5',
            padding: 16,
            borderRadius: 8,
            maxHeight: fullscreen ? '80vh' : 400,
            overflow: 'auto',
            fontSize: 13,
          }}
        >
          {content}
        </pre>
      ) : (
        <Empty description="暂无内容" />
      )}
    </div>
  );

  /**
   * 根据产物类型渲染内容
   */
  const renderContent = () => {
    if (!artifact) return null;

    switch (artifact.type) {
      case 'code':
        return renderCode();
      case 'document':
        return renderDocument();
      case 'review':
        return renderDocument(); // 审查报告按文档处理
      case 'test':
        return renderCode(); // 测试文件按代码处理
      case 'config':
        return renderConfig();
      default:
        return renderDocument();
    }
  };

  if (!artifact) return null;

  const typeIconMap: Record<string, React.ReactNode> = {
    code: <CodeOutlined />,
    document: <FileTextOutlined />,
    review: <FileSearchOutlined />,
    test: <CodeOutlined />,
    config: <FileTextOutlined />,
  };

  return (
    <Modal
      title={
        <Space>
          {typeIconMap[artifact.type] || <FileTextOutlined />}
          <span>{artifact.name}</span>
          <Tag>{ArtifactTypeLabels[artifact.type] || artifact.type}</Tag>
        </Space>
      }
      open={visible}
      onCancel={onClose}
      width={fullscreen ? '100%' : 800}
      style={fullscreen ? { top: 0, padding: 0, maxWidth: '100%' } : undefined}
      bodyStyle={fullscreen ? { height: 'calc(100vh - 110px)', overflow: 'auto' } : undefined}
      footer={
        <Space>
          <Button icon={<CopyOutlined />} onClick={handleCopy}>
            复制
          </Button>
          <Button icon={<DownloadOutlined />} onClick={handleDownload}>
            下载
          </Button>
          <Button
            icon={<FullscreenOutlined />}
            onClick={() => setFullscreen(!fullscreen)}
          >
            {fullscreen ? '退出全屏' : '全屏'}
          </Button>
          <Button onClick={onClose}>关闭</Button>
        </Space>
      }
    >
      {/* 产物元信息 */}
      <div style={{ marginBottom: 16 }}>
        <Space split={<span style={{ color: '#d9d9d9' }}>|</span>}>
          <Text type="secondary">
            创建时间: {new Date(artifact.createdAt).toLocaleString()}
          </Text>
          {artifact.path && (
            <Text type="secondary">
              路径: {artifact.path}
            </Text>
          )}
        </Space>
      </div>

      {/* 产物内容 */}
      {renderContent()}

      {/* 产物元数据 */}
      {artifact.metadata && Object.keys(artifact.metadata).length > 0 && (
        <div style={{ marginTop: 16 }}>
          <Text type="secondary">元数据:</Text>
          <pre style={{ fontSize: 12, background: '#fafafa', padding: 8, marginTop: 8 }}>
            {JSON.stringify(artifact.metadata, null, 2)}
          </pre>
        </div>
      )}
    </Modal>
  );
};

export default ArtifactPreview;