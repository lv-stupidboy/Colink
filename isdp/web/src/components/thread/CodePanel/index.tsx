// isdp/web/src/components/thread/CodePanel/index.tsx
import React, { memo, useMemo, useCallback } from 'react';
import { Button, Empty, Space, Tooltip, message } from 'antd';
import {
  CloseOutlined,
  MinusOutlined,
  RightOutlined,
  CheckOutlined,
  CopyOutlined,
  FileOutlined,
} from '@ant-design/icons';
import type { FileChange } from '@/types/content';
import { FileList } from './FileList';
import './CodePanel.css';

interface CodePanelProps {
  isOpen: boolean;
  isCollapsed: boolean;
  files: FileChange[];
  expandedFiles: Set<string>;
  onToggleCollapse: () => void;
  onClose: () => void;
  onToggleFile: (fileId: string) => void;
  onApplyAll?: () => void;
  onCopyAll?: () => void;
}

/**
 * 代码面板容器组件
 * 右侧面板，展示文件列表和 Diff 视图
 *
 * 设计理念：Git 风格变更面板
 * - 清晰的文件列表布局
 * - 可折叠的侧边栏
 * - 流畅的动画过渡
 */
export const CodePanel: React.FC<CodePanelProps> = memo(({
  isOpen: _isOpen,
  isCollapsed,
  files,
  expandedFiles,
  onToggleCollapse,
  onClose,
  onToggleFile,
  onApplyAll,
  onCopyAll,
}) => {
  // 计算总变更
  const stats = useMemo(() => {
    const additions = files.reduce((sum, f) => sum + f.additions, 0);
    const deletions = files.reduce((sum, f) => sum + f.deletions, 0);
    const newFiles = files.filter(f => f.isNew).length;
    return { additions, deletions, newFiles, total: files.length };
  }, [files]);

  // 复制全部代码
  const handleCopyAll = useCallback(() => {
    if (onCopyAll) {
      onCopyAll();
      return;
    }
    const allCode = files.map(f => `// ${f.filename}\n${f.modifiedContent}`).join('\n\n');
    navigator.clipboard.writeText(allCode);
    message.success('代码已复制到剪贴板');
  }, [files, onCopyAll]);

  // 应用全部变更
  const handleApplyAll = useCallback(() => {
    if (onApplyAll) {
      onApplyAll();
      return;
    }
    message.success('代码变更已应用');
  }, [onApplyAll]);

  // 收起状态
  if (isCollapsed) {
    return (
      <div className="code-panel-collapsed">
        <div className="code-panel-collapsed__content">
          <FileOutlined className="code-panel-collapsed__icon" />
          <div className="code-panel-collapsed__text">代码变更</div>
          <div className="code-panel-collapsed__count">{files.length}</div>
          <div className="code-panel-collapsed__stats">
            {stats.additions > 0 && (
              <span className="stat-add">+{stats.additions}</span>
            )}
            {stats.deletions > 0 && (
              <span className="stat-del">-{stats.deletions}</span>
            )}
          </div>
        </div>
        <Tooltip title="展开面板" placement="left">
          <Button
            type="primary"
            size="small"
            icon={<RightOutlined />}
            onClick={onToggleCollapse}
            className="code-panel-collapsed__btn"
          />
        </Tooltip>
      </div>
    );
  }

  // 展开状态
  return (
    <div className="code-panel">
      {/* 头部 */}
      <div className="code-panel__header">
        <div className="code-panel__title-row">
          <FileOutlined className="code-panel__title-icon" />
          <span className="code-panel__title">代码变更</span>
          {stats.additions > 0 && (
            <span className="code-panel__badge code-panel__badge--add">
              +{stats.additions}
            </span>
          )}
          {stats.deletions > 0 && (
            <span className="code-panel__badge code-panel__badge--del">
              -{stats.deletions}
            </span>
          )}
        </div>
        <Space size={4}>
          <Tooltip title="收起面板">
            <Button
              size="small"
              type="text"
              icon={<MinusOutlined />}
              onClick={onToggleCollapse}
              className="code-panel__action"
            />
          </Tooltip>
          <Tooltip title="关闭面板">
            <Button
              size="small"
              type="text"
              icon={<CloseOutlined />}
              onClick={onClose}
              className="code-panel__action"
            />
          </Tooltip>
        </Space>
      </div>

      {/* 文件列表 */}
      <div className="code-panel__body">
        {files.length === 0 ? (
          <div className="code-panel__empty">
            <Empty
              description="暂无代码变更"
              image={Empty.PRESENTED_IMAGE_SIMPLE}
            />
          </div>
        ) : (
          <FileList
            files={files}
            expandedFiles={expandedFiles}
            onToggleFile={onToggleFile}
          />
        )}
      </div>

      {/* 底部操作 */}
      {files.length > 0 && (
        <div className="code-panel__footer">
          <Button
            type="primary"
            icon={<CheckOutlined />}
            onClick={handleApplyAll}
            className="code-panel__btn code-panel__btn--primary"
          >
            全部应用
          </Button>
          <Button
            icon={<CopyOutlined />}
            onClick={handleCopyAll}
            className="code-panel__btn"
          >
            复制全部
          </Button>
        </div>
      )}
    </div>
  );
});

CodePanel.displayName = 'CodePanel';