import React, { useState, useEffect, useMemo } from 'react';
import { Button, Space, Spin, message, Tooltip, Empty } from 'antd';
import { CopyOutlined, CheckOutlined, FileOutlined, ExpandOutlined } from '@ant-design/icons';
import Editor from '@monaco-editor/react';
import { FocusShell } from './FocusShell';
import api from '@/api/client';
import './FilePreviewPanel.css';

interface FilePreviewPanelProps {
  basePath: string;
  filePath: string;
  onClose: () => void;
  width?: number;
}

// 根据文件扩展名获取 Monaco 语言 ID
const getLanguage = (filePath: string): string => {
  const ext = filePath.split('.').pop()?.toLowerCase() || '';
  const languageMap: Record<string, string> = {
    // JavaScript/TypeScript
    'js': 'javascript',
    'jsx': 'javascript',
    'ts': 'typescript',
    'tsx': 'typescript',
    'mjs': 'javascript',
    // Web
    'html': 'html',
    'htm': 'html',
    'css': 'css',
    'scss': 'scss',
    'sass': 'scss',
    'less': 'less',
    'vue': 'html',
    'svelte': 'html',
    // Backend
    'go': 'go',
    'py': 'python',
    'java': 'java',
    'kt': 'kotlin',
    'kts': 'kotlin',
    'rs': 'rust',
    'rb': 'ruby',
    'php': 'php',
    'swift': 'swift',
    'c': 'c',
    'cpp': 'cpp',
    'h': 'c',
    'hpp': 'cpp',
    // Shell
    'sh': 'shell',
    'bash': 'shell',
    'zsh': 'shell',
    'ps1': 'powershell',
    // Config
    'json': 'json',
    'yaml': 'yaml',
    'yml': 'yaml',
    'toml': 'ini',
    'xml': 'xml',
    'ini': 'ini',
    'conf': 'ini',
    'cfg': 'ini',
    'env': 'plaintext',
    // Data
    'sql': 'sql',
    'csv': 'plaintext',
    // Docs
    'md': 'markdown',
    'markdown': 'markdown',
    'txt': 'plaintext',
    'log': 'plaintext',
    // Special
    'dockerfile': 'dockerfile',
    'makefile': 'makefile',
  };
  return languageMap[ext] || 'plaintext';
};

// 判断是否为图片文件
const isImageFile = (filePath: string): boolean => {
  const ext = filePath.split('.').pop()?.toLowerCase() || '';
  return ['png', 'jpg', 'jpeg', 'gif', 'svg', 'webp', 'bmp'].includes(ext);
};

export const FilePreviewPanel: React.FC<FilePreviewPanelProps> = ({
  basePath,
  filePath,
  onClose,
  width = 520,
}) => {
  const [loading, setLoading] = useState(true);
  const [content, setContent] = useState<string>('');
  const [size, setSize] = useState<number>(0);
  const [truncated, setTruncated] = useState(false);
  const [isBinary, setIsBinary] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [focusMode, setFocusMode] = useState(false);
  const [copied, setCopied] = useState<'content' | 'path' | null>(null);

  // 获取文件内容
  useEffect(() => {
    const fetchContent = async () => {
      setLoading(true);
      setError(null);
      try {
        const result = await api.files.getContent(basePath, filePath);
        setContent(result.content);
        setSize(result.size);
        setTruncated(result.truncated);
        setIsBinary(result.isBinary);
      } catch (err: any) {
        setError(err.message || '加载文件失败');
      } finally {
        setLoading(false);
      }
    };
    fetchContent();
  }, [basePath, filePath]);

  // 复制内容
  const handleCopyContent = async () => {
    try {
      await navigator.clipboard.writeText(content);
      setCopied('content');
      message.success('已复制内容');
      setTimeout(() => setCopied(null), 2000);
    } catch {
      message.error('复制失败');
    }
  };

  // 复制路径
  const handleCopyPath = async () => {
    try {
      const fullPath = `${basePath}/${filePath}`;
      await navigator.clipboard.writeText(fullPath);
      setCopied('path');
      message.success('已复制路径');
      setTimeout(() => setCopied(null), 2000);
    } catch {
      message.error('复制失败');
    }
  };

  // 文件名显示
  const fileName = filePath.split('/').pop() || filePath;

  // 语言
  const language = useMemo(() => getLanguage(filePath), [filePath]);

  // 是否图片
  const isImage = isImageFile(filePath);

  // 图片 URL
  const imageUrl = `/api/v1/files/image?basePath=${encodeURIComponent(basePath)}&path=${encodeURIComponent(filePath)}`;

  // 渲染内容
  const renderContent = () => {
    if (loading) {
      return (
        <div className="file-preview-loading">
          <Spin />
          <span>加载中...</span>
        </div>
      );
    }

    if (error) {
      return (
        <div className="file-preview-error">
          <Empty description={error} />
        </div>
      );
    }

    if (isBinary) {
      return (
        <div className="file-preview-binary">
          <Empty description="二进制文件无法预览" />
        </div>
      );
    }

    if (isImage) {
      return (
        <div className="file-preview-image">
          <img src={imageUrl} alt={fileName} style={{ maxWidth: '100%', maxHeight: '100%' }} />
        </div>
      );
    }

    return (
      <Editor
        height="100%"
        language={language}
        value={content}
        options={{
          readOnly: true,
          minimap: { enabled: false },
          lineNumbers: 'on',
          fontSize: 13,
          scrollBeyondLastLine: false,
          wordWrap: 'on',
          automaticLayout: true,
          theme: 'vs-dark',
        }}
      />
    );
  };

  // 专注模式内容
  if (focusMode) {
    return (
      <FocusShell onExit={() => setFocusMode(false)} title={fileName}>
        <Editor
          height="100%"
          language={language}
          value={content}
          options={{
            readOnly: true,
            minimap: { enabled: true },
            lineNumbers: 'on',
            fontSize: 14,
            scrollBeyondLastLine: false,
            wordWrap: 'on',
            automaticLayout: true,
            theme: 'vs-dark',
          }}
        />
      </FocusShell>
    );
  }

  return (
    <div className="file-preview-panel" style={{ width }}>
      {/* 工具栏 */}
      <div className="file-preview-toolbar">
        <span className="file-preview-filename" title={filePath}>
          <FileOutlined /> {fileName}
        </span>
        <Space size="small">
          <Tooltip title="复制内容">
            <Button
              size="small"
              icon={copied === 'content' ? <CheckOutlined /> : <CopyOutlined />}
              onClick={handleCopyContent}
              disabled={loading || isBinary || isImage}
            >
              Copy
            </Button>
          </Tooltip>
          <Tooltip title="复制路径">
            <Button
              size="small"
              icon={copied === 'path' ? <CheckOutlined /> : <CopyOutlined />}
              onClick={handleCopyPath}
            >
              Path
            </Button>
          </Tooltip>
          <Tooltip title="专注模式">
            <Button
              size="small"
              icon={<ExpandOutlined />}
              onClick={() => setFocusMode(true)}
              disabled={loading || isBinary || isImage}
            >
              专注
            </Button>
          </Tooltip>
          <Tooltip title="关闭">
            <Button size="small" onClick={onClose}>
              ✕
            </Button>
          </Tooltip>
        </Space>
      </div>

      {/* 文件大小提示 */}
      {size > 0 && (
        <div className="file-preview-meta">
          {size < 1024 ? `${size}B` : size < 1024 * 1024 ? `${Math.round(size / 1024)}KB` : `${Math.round(size / 1024 / 1024)}MB`}
          {truncated && <span className="file-preview-truncated">(已截断)</span>}
        </div>
      )}

      {/* 内容区域 */}
      <div className="file-preview-content">
        {renderContent()}
      </div>
    </div>
  );
};

export default FilePreviewPanel;