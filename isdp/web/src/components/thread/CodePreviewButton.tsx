// isdp/web/src/components/thread/CodePreviewButton.tsx
import React, { memo, useMemo } from 'react';
import { CodeOutlined, RightOutlined, FileOutlined } from '@ant-design/icons';
import type { FileChange } from '@/types/content';
import './CodePreviewButton.css';

interface CodePreviewButtonProps {
  files: FileChange[];
  onClick: () => void;
}

/**
 * 代码预览入口按钮组件
 * 在气泡内显示代码摘要，点击打开右侧面板
 *
 * 设计理念：代码编辑器风格
 * - 深色背景模拟 IDE
 * - 文件名 + 变更统计的清晰展示
 * - 微妙的悬停效果引导点击
 */
export const CodePreviewButton: React.FC<CodePreviewButtonProps> = memo(({
  files,
  onClick,
}) => {
  // 计算总变更
  const stats = useMemo(() => {
    const additions = files.reduce((sum, f) => sum + f.additions, 0);
    const deletions = files.reduce((sum, f) => sum + f.deletions, 0);
    const newFiles = files.filter(f => f.isNew).length;
    return { additions, deletions, newFiles, total: files.length };
  }, [files]);

  // 显示第一个文件的预览
  const firstFile = files[0];
  const previewLines = useMemo(() => {
    if (!firstFile?.modifiedContent) return '';
    return firstFile.modifiedContent.split('\n').slice(0, 3).join('\n');
  }, [firstFile]);

  if (files.length === 0) return null;

  return (
    <div className="code-preview-button" onClick={onClick}>
      {/* 头部：文件信息 + 变更统计 */}
      <div className="code-preview-button__header">
        <div className="code-preview-button__file-info">
          <FileOutlined className="code-preview-button__file-icon" />
          <span className="code-preview-button__filename">
            {firstFile?.filename || '代码文件'}
          </span>
          {firstFile?.isNew && (
            <span className="code-preview-button__badge code-preview-button__badge--new">
              新增
            </span>
          )}
        </div>

        <div className="code-preview-button__stats">
          {stats.additions > 0 && (
            <span className="code-preview-button__stat code-preview-button__stat--add">
              +{stats.additions}
            </span>
          )}
          {stats.deletions > 0 && (
            <span className="code-preview-button__stat code-preview-button__stat--del">
              -{stats.deletions}
            </span>
          )}
        </div>
      </div>

      {/* 预览区域 */}
      <div className="code-preview-button__preview">
        <pre className="code-preview-button__code">{previewLines}...</pre>
      </div>

      {/* 底部：操作提示 */}
      <div className="code-preview-button__footer">
        <div className="code-preview-button__hint">
          <CodeOutlined />
          <span>
            {stats.total > 1
              ? `${stats.total} 个文件变更`
              : '查看代码详情'}
          </span>
        </div>
        <RightOutlined className="code-preview-button__arrow" />
      </div>
    </div>
  );
});

CodePreviewButton.displayName = 'CodePreviewButton';