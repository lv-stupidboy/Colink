// isdp/web/src/components/thread/CodePanel/SplitDiff.tsx
import React, { useMemo, memo, useRef, useCallback } from 'react';
import DiffMatchPatch from 'diff-match-patch';

interface SplitDiffProps {
  originalContent: string | null;
  modifiedContent: string;
  isNew?: boolean;
}

interface DiffLine {
  leftNumber: number | null;
  leftContent: string;
  leftType: 'normal' | 'deleted' | 'empty';
  rightNumber: number | null;
  rightContent: string;
  rightType: 'normal' | 'added' | 'empty';
}

/**
 * Split Diff 视图组件
 * 左边显示原始代码，右边显示变更后代码
 *
 * 设计理念：Git 风格 Split Diff
 * - 左右并排对比
 * - 同步滚动
 * - 行号高亮
 */
export const SplitDiff: React.FC<SplitDiffProps> = memo(({
  originalContent,
  modifiedContent,
  isNew = false,
}) => {
  const leftRef = useRef<HTMLDivElement>(null);
  const rightRef = useRef<HTMLDivElement>(null);

  // 计算 Diff
  const diffLines = useMemo(() => {
    const lines: DiffLine[] = [];

    // 新文件：左侧为空
    if (isNew || originalContent === null) {
      const modifiedLines = modifiedContent.split('\n');
      modifiedLines.forEach((line, index) => {
        lines.push({
          leftNumber: null,
          leftContent: '',
          leftType: 'empty',
          rightNumber: index + 1,
          rightContent: line,
          rightType: 'added',
        });
      });
      return lines;
    }

    // 使用 diff-match-patch 计算
    const dmp = new DiffMatchPatch();
    const diffs = dmp.diff_main(originalContent, modifiedContent);
    dmp.diff_cleanupSemantic(diffs);

    const originalLines = originalContent.split('\n');
    const modifiedLines = modifiedContent.split('\n');

    let leftLineNum = 0;
    let rightLineNum = 0;

    // 简单实现：按行对比
    const maxLines = Math.max(originalLines.length, modifiedLines.length);

    for (let i = 0; i < maxLines; i++) {
      const origLine = originalLines[i];
      const modLine = modifiedLines[i];

      if (origLine === undefined) {
        // 新增行
        lines.push({
          leftNumber: null,
          leftContent: '',
          leftType: 'empty',
          rightNumber: ++rightLineNum,
          rightContent: modLine,
          rightType: 'added',
        });
      } else if (modLine === undefined) {
        // 删除行
        lines.push({
          leftNumber: ++leftLineNum,
          leftContent: origLine,
          leftType: 'deleted',
          rightNumber: null,
          rightContent: '',
          rightType: 'empty',
        });
      } else if (origLine === modLine) {
        // 未修改行
        lines.push({
          leftNumber: ++leftLineNum,
          leftContent: origLine,
          leftType: 'normal',
          rightNumber: ++rightLineNum,
          rightContent: modLine,
          rightType: 'normal',
        });
      } else {
        // 修改行：显示为删除+新增
        lines.push({
          leftNumber: ++leftLineNum,
          leftContent: origLine,
          leftType: 'deleted',
          rightNumber: ++rightLineNum,
          rightContent: modLine,
          rightType: 'added',
        });
      }
    }

    return lines;
  }, [originalContent, modifiedContent, isNew]);

  // 同步滚动
  const handleScroll = useCallback((source: 'left' | 'right') => {
    if (!leftRef.current || !rightRef.current) return;

    if (source === 'left') {
      rightRef.current.scrollTop = leftRef.current.scrollTop;
    } else {
      leftRef.current.scrollTop = rightRef.current.scrollTop;
    }
  }, []);

  return (
    <div className="split-diff">
      {/* 左侧：原始代码 */}
      <div
        className="split-diff__side split-diff__side--left"
        ref={leftRef}
        onScroll={() => handleScroll('left')}
      >
        <div className="split-diff__header split-diff__header--original">
          原始代码
        </div>
        <div className="split-diff__content">
          {isNew && (
            <div className="split-diff__empty-notice">
              暂无内容（新文件）
            </div>
          )}
          {diffLines.map((line, index) => (
            <div key={index} className={`split-diff__line split-diff__line--${line.leftType}`}>
              <span className="split-diff__num">{line.leftNumber || ''}</span>
              <span className="split-diff__text">{line.leftContent}</span>
            </div>
          ))}
        </div>
      </div>

      {/* 右侧：变更后代码 */}
      <div
        className="split-diff__side split-diff__side--right"
        ref={rightRef}
        onScroll={() => handleScroll('right')}
      >
        <div className="split-diff__header split-diff__header--modified">
          变更后
        </div>
        <div className="split-diff__content">
          {diffLines.map((line, index) => (
            <div key={index} className={`split-diff__line split-diff__line--${line.rightType}`}>
              <span className="split-diff__num">{line.rightNumber || ''}</span>
              <span className="split-diff__text">{line.rightContent}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
});

SplitDiff.displayName = 'SplitDiff';