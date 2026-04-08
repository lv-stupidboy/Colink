// isdp/web/src/components/thread/CodePreviewButton.tsx
import React, { memo, useMemo } from 'react';
import { CodeOutlined, RightOutlined } from '@ant-design/icons';
import type { FileChange } from '@/types/content';
import './CodePreviewButton.css';

interface CodePreviewButtonProps {
  files: FileChange[];
  onClick: () => void;
}

/**
 * 代码预览入口按钮组件
 * 简洁轻量设计，点击打开右侧面板
 */
export const CodePreviewButton: React.FC<CodePreviewButtonProps> = memo(({
  files,
  onClick,
}) => {
  // 计算总变更
  const stats = useMemo(() => {
    const additions = files.reduce((sum, f) => sum + f.additions, 0);
    const deletions = files.reduce((sum, f) => sum + f.deletions, 0);
    return { additions, deletions, total: files.length };
  }, [files]);

  if (files.length === 0) return null;

  return (
    <div className="code-preview-button" onClick={onClick}>
      <div className="code-preview-button__info">
        <CodeOutlined className="code-preview-button__icon" />
        <span className="code-preview-button__text">
          {stats.total > 1 ? `${stats.total} 个文件变更` : '查看代码详情'}
        </span>
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
        <RightOutlined className="code-preview-button__arrow" />
      </div>
    </div>
  );
});

CodePreviewButton.displayName = 'CodePreviewButton';