// isdp/web/src/components/thread/CodePanel/FileList.tsx
import React, { memo } from 'react';
import type { FileChange } from '@/types/content';
import { FileItem } from './FileItem';

interface FileListProps {
  files: FileChange[];
  expandedFiles: Set<string>;
  onToggleFile: (fileId: string) => void;
}

/**
 * 文件列表组件
 * 纵向排列，类似 Git Changes
 */
export const FileList: React.FC<FileListProps> = memo(({
  files,
  expandedFiles,
  onToggleFile,
}) => {
  return (
    <div className="file-list">
      {files.map((file, index) => (
        <FileItem
          key={file.id}
          file={file}
          isExpanded={expandedFiles.has(file.id)}
          onToggle={() => onToggleFile(file.id)}
          index={index}
        />
      ))}
    </div>
  );
});

FileList.displayName = 'FileList';