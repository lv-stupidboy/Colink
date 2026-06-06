// isdp/web/src/components/thread/ContentBlock/RichBlocks.tsx
import React, { memo } from 'react';
import {
  Card,
  Tag,
  Button,
  Checkbox,
  Image,
  Space,
  Progress,
  Tooltip,
} from 'antd';
import {
  FileOutlined,
  DownloadOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  LinkOutlined,
  AudioOutlined,
  FileTextOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import type {
  RichBlock,
  CardRichBlock,
  DiffRichBlock,
  ChecklistRichBlock,
  MediaGalleryRichBlock,
  AudioRichBlock,
  InteractiveRichBlock,
  HtmlWidgetRichBlock,
  FileRichBlock,
} from '@/types';
import './RichBlocks.css';

interface RichBlocksProps {
  blocks: RichBlock[];
  onInteractiveAction?: (blockId: string, action: string, value?: string | string[]) => void;
}

/**
 * 富内容块渲染器
 * 根据 richType 渲染不同类型的富内容块
 */
export const RichBlocks: React.FC<RichBlocksProps> = memo(({
  blocks,
  onInteractiveAction,
}) => {
  // 按 groupId 分组交互块
  const groupedBlocks = groupInteractiveBlocks(blocks);

  return (
    <div className="rich-blocks-container">
      {groupedBlocks.map((blockOrGroup, index) => {
        if (Array.isArray(blockOrGroup)) {
          // 交互块组
          return (
            <InteractiveGroupBlock
              key={`interactive-group-${index}`}
              blocks={blockOrGroup as InteractiveRichBlock[]}
              onAction={onInteractiveAction}
            />
          );
        }
        // 单个富内容块
        return renderRichBlock(blockOrGroup as RichBlock, index, onInteractiveAction);
      })}
    </div>
  );
});

/**
 * 渲染单个富内容块
 */
function renderRichBlock(
  block: RichBlock,
  index: number,
  onAction?: (blockId: string, action: string, value?: string | string[]) => void
): React.ReactNode {
  switch (block.richType) {
    case 'card':
      return <CardBlock key={block.id || index} block={block as CardRichBlock} />;
    case 'diff':
      return <DiffBlock key={block.id || index} block={block as DiffRichBlock} />;
    case 'checklist':
      return <ChecklistBlock key={block.id || index} block={block as ChecklistRichBlock} />;
    case 'media_gallery':
      return <MediaGalleryBlock key={block.id || index} block={block as MediaGalleryRichBlock} />;
    case 'audio':
      return <AudioBlock key={block.id || index} block={block as AudioRichBlock} />;
    case 'interactive':
      return (
        <InteractiveBlock
          key={block.id || index}
          block={block as InteractiveRichBlock}
          onAction={onAction}
        />
      );
    case 'html_widget':
      return <HtmlWidgetBlock key={block.id || index} block={block as HtmlWidgetRichBlock} />;
    case 'file':
      return <FileBlock key={block.id || index} block={block as FileRichBlock} />;
    default:
      return null;
  }
}

/**
 * 按 groupId 分组交互块
 */
function groupInteractiveBlocks(blocks: RichBlock[]): Array<RichBlock | RichBlock[]> {
  const result: Array<RichBlock | RichBlock[]> = [];
  const interactiveGroups: Record<string, InteractiveRichBlock[]> = {};
  const ungroupedInteractive: InteractiveRichBlock[] = [];

  for (const block of blocks) {
    if (block.richType !== 'interactive') {
      // 非交互块直接添加
      result.push(block);
    } else {
      const interactiveBlock = block as InteractiveRichBlock;
      if (interactiveBlock.groupId) {
        // 有 groupId 的分组
        if (!interactiveGroups[interactiveBlock.groupId]) {
          interactiveGroups[interactiveBlock.groupId] = [];
        }
        interactiveGroups[interactiveBlock.groupId].push(interactiveBlock);
      } else {
        // 无 groupId 的收集
        ungroupedInteractive.push(interactiveBlock);
      }
    }
  }

  // 处理分组交互块
  for (const groupId of Object.keys(interactiveGroups)) {
    result.push(interactiveGroups[groupId]);
  }

  // 处理未分组的交互块（≥2个自动合并）
  if (ungroupedInteractive.length >= 2) {
    result.push(ungroupedInteractive);
  } else {
    // 单个未分组直接添加
    for (const block of ungroupedInteractive) {
      result.push(block);
    }
  }

  return result;
}

// ========== 各类型块组件 ==========

/** 信息卡片块 */
const CardBlock: React.FC<{ block: CardRichBlock }> = memo(({ block }) => (
  <Card
    className="rich-card-block"
    size="small"
    title={
      <Space>
        {block.icon && <span className="rich-card-icon">{block.icon}</span>}
        <span>{block.title}</span>
      </Space>
    }
    actions={block.actions?.map((action) => (
      <Button
        key={action.id}
        type="link"
        size="small"
        href={action.url}
        onClick={() => {}}
      >
        {action.label}
      </Button>
    ))}
  >
    <div className="rich-card-description">{block.description}</div>
    {block.metadata && (
      <div className="rich-card-metadata">
        {Object.entries(block.metadata).map(([key, value]) => (
          <Tag key={key}>{key}: {String(value)}</Tag>
        ))}
      </div>
    )}
  </Card>
));
CardBlock.displayName = 'CardBlock';

/** 代码差异块 */
const DiffBlock: React.FC<{ block: DiffRichBlock }> = memo(({ block }) => (
  <div className="rich-diff-block">
    <div className="rich-diff-header">
      <FileOutlined />
      <span className="rich-diff-filename">{block.filename}</span>
      <Tag color="green">{block.additions}+</Tag>
      <Tag color="red">{block.deletions}-</Tag>
    </div>
    <pre className="rich-diff-content">{block.diffContent}</pre>
  </div>
));
DiffBlock.displayName = 'DiffBlock';

/** 待办清单块 */
const ChecklistBlock: React.FC<{ block: ChecklistRichBlock }> = memo(({ block }) => (
  <div className="rich-checklist-block">
    {block.title && <div className="rich-checklist-title">{block.title}</div>}
    <div className="rich-checklist-items">
      {block.items.map((item) => (
        <div key={item.id} className="rich-checklist-item">
          <Checkbox checked={item.checked} disabled>
            {item.content}
          </Checkbox>
          {item.status === 'done' && <CheckCircleOutlined className="checklist-status-done" />}
          {item.status === 'failed' && <CloseCircleOutlined className="checklist-status-failed" />}
        </div>
      ))}
    </div>
  </div>
));
ChecklistBlock.displayName = 'ChecklistBlock';

/** 图片画廊块 */
const MediaGalleryBlock: React.FC<{ block: MediaGalleryRichBlock }> = memo(({ block }) => (
  <div className="rich-media-gallery-block">
    <Image.PreviewGroup>
      {block.images.map((image) => (
        <Image
          key={image.id}
          src={image.thumbnailUrl || image.url}
          alt={image.caption}
          width={80}
          height={80}
          style={{ objectFit: 'cover', borderRadius: 4 }}
        />
      ))}
    </Image.PreviewGroup>
    {block.caption && <div className="rich-media-caption">{block.caption}</div>}
  </div>
));
MediaGalleryBlock.displayName = 'MediaGalleryBlock';

/** 音频块（TTS） */
const AudioBlock: React.FC<{ block: AudioRichBlock }> = memo(({ block }) => (
  <div className="rich-audio-block">
    <div className="rich-audio-header">
      <AudioOutlined />
      <span>TTS音频</span>
      {block.status === 'generating' && (
        <Progress percent={50} size="small" status="active" style={{ width: 100 }} />
      )}
      {block.status === 'ready' && (
        <Tag color="success">就绪</Tag>
      )}
    </div>
    {block.status === 'ready' && (
      <audio controls src={block.audioUrl} className="rich-audio-player">
        您的浏览器不支持音频播放
      </audio>
    )}
    {block.transcript && (
      <div className="rich-audio-transcript">{block.transcript}</div>
    )}
  </div>
));
AudioBlock.displayName = 'AudioBlock';

/** 交互块（单个） */
const InteractiveBlock: React.FC<{
  block: InteractiveRichBlock;
  onAction?: (blockId: string, action: string, value?: string | string[]) => void;
}> = memo(({ block, onAction }) => {
  const handleSelect = (optionId: string) => {
    onAction?.(block.id, 'select', optionId);
  };

  const handleMultiSelect = (optionIds: string[]) => {
    onAction?.(block.id, 'multi_select', optionIds);
  };

  const handleConfirm = () => {
    onAction?.(block.id, 'confirm', undefined);
  };

  switch (block.interactiveType) {
    case 'choice':
      return (
        <div className="rich-interactive-block rich-interactive-choice">
          <div className="rich-interactive-prompt">{block.prompt}</div>
          <div className="rich-interactive-options">
            {block.options?.map((option) => (
              <Button
                key={option.id}
                type={block.selectedOptionId === option.id ? 'primary' : 'default'}
                size="small"
                onClick={() => handleSelect(option.id)}
                disabled={option.disabled || block.selectedOptionId !== undefined}
              >
                {option.icon && <span>{option.icon}</span>}
                {option.label}
              </Button>
            ))}
          </div>
        </div>
      );

    case 'multi_select':
      return (
        <div className="rich-interactive-block rich-interactive-multi-select">
          <div className="rich-interactive-prompt">{block.prompt}</div>
          <div className="rich-interactive-options">
            {block.options?.map((option) => (
              <Checkbox
                key={option.id}
                checked={block.selectedOptionIds?.includes(option.id)}
                disabled={option.disabled}
              >
                {option.icon && <span>{option.icon}</span>}
                {option.label}
              </Checkbox>
            ))}
          </div>
          <Button
            type="primary"
            size="small"
            onClick={() => handleMultiSelect(block.selectedOptionIds || [])}
          >
            确认选择
          </Button>
        </div>
      );

    case 'confirm':
      return (
        <div className="rich-interactive-block rich-interactive-confirm">
          <div className="rich-interactive-prompt">{block.prompt}</div>
          <Space>
            <Button
              type="primary"
              size="small"
              onClick={handleConfirm}
            >
              确认
            </Button>
            <Button
              size="small"
              onClick={() => onAction?.(block.id, 'cancel')}
            >
              取消
            </Button>
          </Space>
        </div>
      );

    case 'input':
      return (
        <div className="rich-interactive-block rich-interactive-input">
          <div className="rich-interactive-prompt">{block.prompt}</div>
          <input
            type="text"
            placeholder={block.placeholder}
            className="rich-interactive-input-field"
          />
          <Button
            type="primary"
            size="small"
            onClick={() => onAction?.(block.id, 'submit', block.inputValue)}
          >
            提交
          </Button>
        </div>
      );

    default:
      return null;
  }
});
InteractiveBlock.displayName = 'InteractiveBlock';

/** 交互块组 */
const InteractiveGroupBlock: React.FC<{
  blocks: InteractiveRichBlock[];
  onAction?: (blockId: string, action: string, value?: string | string[]) => void;
}> = memo(({ blocks, onAction }) => (
  <div className="rich-interactive-group">
    <div className="rich-interactive-group-header">
      <ThunderboltOutlined />
      <span>用户交互 ({blocks.length}项)</span>
    </div>
    <div className="rich-interactive-group-items">
      {blocks.map((block) => (
        <InteractiveBlock key={block.id} block={block} onAction={onAction} />
      ))}
    </div>
  </div>
));
InteractiveGroupBlock.displayName = 'InteractiveGroupBlock';

/** HTML Widget块 */
const HtmlWidgetBlock: React.FC<{ block: HtmlWidgetRichBlock }> = memo(({ block }) => (
  <div className="rich-html-widget-block">
    <div className="rich-html-widget-header">
      <LinkOutlined />
      <span>{block.title || '嵌入内容'}</span>
      <Tooltip title="在新窗口打开">
        <Button
          type="link"
          size="small"
          icon={<LinkOutlined />}
          href={block.iframeUrl}
          target="_blank"
        />
      </Tooltip>
    </div>
    <iframe
      src={block.iframeUrl}
      width={block.width || '100%'}
      height={block.height || 400}
      title={block.title || '嵌入内容'}
      className="rich-html-widget-iframe"
      sandbox="allow-scripts allow-same-origin"
    />
  </div>
));
HtmlWidgetBlock.displayName = 'HtmlWidgetBlock';

/** 文件附件块 */
const FileBlock: React.FC<{ block: FileRichBlock }> = memo(({ block }) => (
  <div className="rich-file-block">
    <FileTextOutlined className="rich-file-icon" />
    <div className="rich-file-info">
      <span className="rich-file-name">{block.filename}</span>
      {block.fileSize && (
        <span className="rich-file-size">{formatFileSize(block.fileSize)}</span>
      )}
    </div>
    {block.downloadUrl && (
      <Button
        type="link"
        size="small"
        icon={<DownloadOutlined />}
        href={block.downloadUrl}
      >
        下载
      </Button>
    )}
  </div>
));
FileBlock.displayName = 'FileBlock';

/** 格式化文件大小 */
function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`;
}

RichBlocks.displayName = 'RichBlocks';