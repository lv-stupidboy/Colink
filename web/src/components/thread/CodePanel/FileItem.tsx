// isdp/web/src/components/thread/CodePanel/FileItem.tsx
import React, { memo } from 'react';
import { DownOutlined, RightOutlined, FileOutlined, FileAddOutlined } from '@ant-design/icons';
import type { FileChange } from '@/types/content';
import { SplitDiff } from './SplitDiff';

interface FileItemProps {
  file: FileChange;
  isExpanded: boolean;
  onToggle: () => void;
  index?: number;
}

/**
 * 单个文件项组件
 * 显示文件名、变更统计、支持展开/收起
 */
export const FileItem: React.FC<FileItemProps> = memo(({
  file,
  isExpanded,
  onToggle,
  index = 0,
}) => {
  return (
    <div className="file-item">
      {/* 文件头部 */}
      <div
        className={`file-item__header ${isExpanded ? 'file-item__header--expanded' : ''}`}
        onClick={onToggle}
        style={{ animationDelay: `${index * 30}ms` }}
      >
        <div className="file-item__info">
          <span className="file-item__expand-icon">
            {isExpanded ? <DownOutlined /> : <RightOutlined />}
          </span>
          <span className="file-item__icon">
            {file.isNew ? <FileAddOutlined /> : <FileOutlined />}
          </span>
          <span className={`file-item__name ${file.isNew ? 'file-item__name--new' : ''}`}>
            {file.filename}
          </span>
          {file.isNew && (
            <span className="file-item__badge">新增</span>
          )}
        </div>
        <div className="file-item__stats">
          {file.additions > 0 && (
            <span className="file-item__stat file-item__stat--add">+{file.additions}</span>
          )}
          {file.deletions > 0 && (
            <span className="file-item__stat file-item__stat--del">-{file.deletions}</span>
          )}
        </div>
      </div>

      {/* 展开的 Diff 视图 */}
      {isExpanded && (
        <div className="file-item__content">
          <SplitDiff
            originalContent={file.originalContent}
            modifiedContent={file.modifiedContent}
            isNew={file.isNew}
          />
        </div>
      )}
    </div>
  );
});

FileItem.displayName = 'FileItem';