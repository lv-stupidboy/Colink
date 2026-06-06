// isdp/web/src/components/thread/ThreadInput.tsx
import React, { memo, useState, useCallback, useRef, useEffect } from 'react';
import { Input, Button, Space, Card, List, Spin, Upload, Image } from 'antd';
import { SendOutlined, PictureOutlined, CloseOutlined } from '@ant-design/icons';
import AgentTypeIcon from '@/components/AgentTypeIcon';
import type { AgentRole, ImageAttachment } from '@/types';

const { TextArea } = Input;

interface AgentOption {
  id: string;
  role: AgentRole;
  requiresHuman: boolean;
  isSystem?: boolean;
  name: string;
  label: string;
}

interface ThreadInputProps {
  placeholder: string;
  loadingContext: boolean;
  agentOptions: AgentOption[];
  onSend: (content: string, images?: ImageAttachment[]) => void; // 支持图片参数
  disabled?: boolean;
  prefilledMention?: string;
  onPrefillConsumed?: () => void;
  appendMention?: string;
  onAppendConsumed?: () => void;
}

/**
 * 独立的输入组件
 * 支持文本输入和图片上传（多模态）
 */
export const ThreadInput: React.FC<ThreadInputProps> = memo(({
  placeholder,
  loadingContext,
  agentOptions,
  onSend,
  disabled = false,
  prefilledMention,
  onPrefillConsumed,
  appendMention,
  onAppendConsumed,
}) => {
  const inputRef = useRef<any>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const [inputValue, setInputValue] = useState('');
  const [images, setImages] = useState<ImageAttachment[]>([]);
  const [mentionListVisible, setMentionListVisible] = useState(false);
  const [mentionFilter, setMentionFilter] = useState('');
  const [highlightedIndex, setHighlightedIndex] = useState(0);
  const [showPrefillHint, setShowPrefillHint] = useState(false);

  // 将文件转换为 ImageAttachment
  const fileToImageAttachment = useCallback(async (file: File): Promise<ImageAttachment> => {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = (e) => {
        const dataUrl = e.target?.result as string;
        // 提取 base64 数据（去掉 data:image/xxx;base64, 前缀）
        const base64Match = dataUrl.match(/^data:image\/[^;]+;base64,(.+)$/);
        if (!base64Match) {
          reject(new Error('Invalid image format'));
          return;
        }

        // 获取图片尺寸
        const img = new window.Image();
        img.onload = () => {
          resolve({
            id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
            base64: base64Match[1],
            mimeType: file.type,
            filename: file.name,
            width: img.width,
            height: img.height,
          });
        };
        img.onerror = () => reject(new Error('Failed to load image'));
        img.src = dataUrl;
      };
      reader.onerror = () => reject(new Error('Failed to read file'));
      reader.readAsDataURL(file);
    });
  }, []);

  // 处理图片上传
  const handleImageUpload = useCallback(async (file: File) => {
    // 检查文件类型
    if (!file.type.startsWith('image/')) {
      return false;
    }

    // 检查文件大小（限制 10MB）
    if (file.size > 10 * 1024 * 1024) {
      return false;
    }

    try {
      const attachment = await fileToImageAttachment(file);
      setImages(prev => [...prev, attachment]);
    } catch (err) {
      console.error('Failed to process image:', err);
    }

    return false; // 阻止 antd Upload 的默认上传行为
  }, [fileToImageAttachment]);

  // 删除图片
  const handleRemoveImage = useCallback((imageId: string) => {
    setImages(prev => prev.filter(img => img.id !== imageId));
  }, []);

  // 处理粘贴图片
  const handlePaste = useCallback(async (e: React.ClipboardEvent) => {
    const items = e.clipboardData.items;
    for (const item of items) {
      if (item.type.startsWith('image/')) {
        const file = item.getAsFile();
        if (file) {
          // 检查文件大小（限制 10MB）
          if (file.size > 10 * 1024 * 1024) {
            continue;
          }
          try {
            const attachment = await fileToImageAttachment(file);
            setImages(prev => [...prev, attachment]);
          } catch (err) {
            console.error('Failed to process pasted image:', err);
          }
        }
      }
    }
  }, [fileToImageAttachment]);

  // 发送消息
  const handleSend = useCallback(() => {
    const hasContent = inputValue.trim() || images.length > 0;
    if (!hasContent || disabled) return;

    const content = inputValue.trim();
    const imagesToSend = images.length > 0 ? images : undefined;

    setInputValue('');
    setImages([]);
    setMentionListVisible(false);
    onSend(content, imagesToSend);
  }, [inputValue, images, disabled, onSend]);

  // 输入变化
  const handleInputChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const value = e.target.value;
    setInputValue(value);

    // 检测 @ 符号
    const lastAtIndex = value.lastIndexOf('@');
    if (lastAtIndex >= 0 && lastAtIndex === value.length - 1) {
      setMentionListVisible(true);
      setMentionFilter('');
      setHighlightedIndex(0);
    } else if (lastAtIndex >= 0 && value.indexOf(' ', lastAtIndex) === -1) {
      setMentionListVisible(true);
      setMentionFilter(value.substring(lastAtIndex + 1).toLowerCase());
      setHighlightedIndex(0);
    } else {
      setMentionListVisible(false);
    }
  }, []);

  // 选择 mention
  const selectMention = useCallback((name: string) => {
    const lastAtIndex = inputValue.lastIndexOf('@');
    if (lastAtIndex >= 0) {
      setInputValue(inputValue.substring(0, lastAtIndex) + '@' + name + ' ');
    }
    setMentionListVisible(false);
    inputRef.current?.focus();
  }, [inputValue]);

  // 过滤 Agent 列表
  const filteredAgents = agentOptions.filter(opt =>
    !mentionFilter ||
    opt.label.toLowerCase().includes(mentionFilter.toLowerCase()) ||
    opt.role.toLowerCase().includes(mentionFilter.toLowerCase())
  );

  // 当过滤列表变化时，重置高亮索引
  useEffect(() => {
    setHighlightedIndex(0);
  }, [mentionFilter]);

  // 滚动到高亮项
  useEffect(() => {
    if (mentionListVisible && listRef.current) {
      const items = listRef.current.querySelectorAll('.mention-list-item');
      if (items[highlightedIndex]) {
        items[highlightedIndex].scrollIntoView({ block: 'nearest' });
      }
    }
  }, [highlightedIndex, mentionListVisible]);

  // 自动填入 @mention（阻塞确认后触发）
  useEffect(() => {
    if (prefilledMention && inputRef.current) {
      const agentExists = agentOptions.some(
        opt => opt.name === prefilledMention || opt.label.includes(prefilledMention)
      );

      if (!agentExists) return;

      const currentText = inputValue;

      if (currentText.startsWith('@')) {
        onPrefillConsumed?.();
        return;
      }

      const cursorPos = inputRef.current.selectionStart || 0;
      const mention = `@${prefilledMention} `;
      const newText = mention + currentText;

      setInputValue(newText);
      inputRef.current.focus();

      const newCursorPos = cursorPos + mention.length;
      setTimeout(() => {
        inputRef.current?.setSelectionRange(newCursorPos, newCursorPos);
      }, 0);

      setShowPrefillHint(true);
      setTimeout(() => setShowPrefillHint(false), 3000);

      onPrefillConsumed?.();
    }
  }, [prefilledMention, agentOptions, inputValue, onPrefillConsumed]);

  // 追加 @mention（点击 Agent 头像/名称触发）
  useEffect(() => {
    if (appendMention && inputRef.current) {
      const agentExists = agentOptions.some(
        opt => opt.name === appendMention || opt.label.includes(appendMention)
      );

      if (agentExists) {
        const currentText = inputValue.trim();
        const newMention = `@${appendMention} `;
        const newText = currentText ? `${currentText} ${newMention}` : newMention;
        setInputValue(newText);
        inputRef.current.focus();

        if (onAppendConsumed) {
          onAppendConsumed();
        }
      }
    }
  }, [appendMention, agentOptions, inputValue, onAppendConsumed]);

  // 键盘导航
  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (mentionListVisible && filteredAgents.length > 0) {
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setHighlightedIndex(prev => prev > 0 ? prev - 1 : filteredAgents.length - 1);
        return;
      }
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setHighlightedIndex(prev => prev < filteredAgents.length - 1 ? prev + 1 : 0);
        return;
      }
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        const agent = filteredAgents[highlightedIndex];
        if (agent) {
          selectMention(agent.name);
        }
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setMentionListVisible(false);
        return;
      }
    }

    // 正常发送
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }, [mentionListVisible, filteredAgents, highlightedIndex, selectMention, handleSend]);

  // 拖拽上传处理
  const handleDrop = useCallback(async (e: React.DragEvent) => {
    e.preventDefault();
    const files = e.dataTransfer.files;
    for (const file of files) {
      if (file.type.startsWith('image/')) {
        await handleImageUpload(file);
      }
    }
  }, [handleImageUpload]);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
  }, []);

  // 是否可以发送
  const canSend = !disabled && (inputValue.trim() || images.length > 0);

  return (
    <div
      className="thread-input"
      style={{ display: 'flex', gap: '12px', padding: '12px 16px' }}
      onDrop={handleDrop}
      onDragOver={handleDragOver}
    >
      <div style={{ position: 'relative', flex: 1 }}>
        {/* 预填入提示 */}
        {showPrefillHint && (
          <div
            className="prefill-hint"
            style={{
              position: 'absolute',
              top: -28,
              left: 0,
              padding: '4px 8px',
              background: 'var(--color-primary-opacity-10, rgba(24, 144, 255, 0.1))',
              borderRadius: 4,
              fontSize: 12,
              color: 'var(--color-primary, #1890ff)',
              whiteSpace: 'nowrap',
            }}
          >
            已自动填入 @{prefilledMention}，可切换其他 Agent
          </div>
        )}

        {/* 图片预览区域 */}
        {images.length > 0 && (
          <div
            className="image-preview-container"
            style={{
              display: 'flex',
              gap: 8,
              marginBottom: 8,
              padding: 8,
              background: 'var(--bg-container, #fafafa)',
              borderRadius: 8,
            }}
          >
            {images.map(img => (
              <div
                key={img.id}
                className="image-preview-item"
                style={{
                  position: 'relative',
                  width: 80,
                  height: 80,
                  borderRadius: 8,
                  overflow: 'hidden',
                }}
              >
                <Image
                  src={`data:${img.mimeType};base64,${img.base64}`}
                  alt={img.filename || 'image'}
                  style={{ width: 80, height: 80, objectFit: 'cover' }}
                  preview={true}
                />
                <Button
                  type="text"
                  size="small"
                  icon={<CloseOutlined />}
                  onClick={() => handleRemoveImage(img.id)}
                  style={{
                    position: 'absolute',
                    top: 2,
                    right: 2,
                    background: 'rgba(0,0,0,0.5)',
                    color: '#fff',
                    borderRadius: '50%',
                    width: 20,
                    height: 20,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                  }}
                />
              </div>
            ))}
          </div>
        )}

        <TextArea
          ref={inputRef}
          value={inputValue}
          onChange={handleInputChange}
          onKeyDown={handleKeyDown}
          onPaste={handlePaste}
          placeholder={placeholder}
          autoSize={{ minRows: 2, maxRows: 6 }}
          disabled={disabled}
        />

        {/* 图片上传按钮 */}
        <Upload
          accept="image/*"
          showUploadList={false}
          beforeUpload={handleImageUpload}
          multiple
        >
          <Button
            type="text"
            icon={<PictureOutlined />}
            disabled={disabled}
            style={{
              position: 'absolute',
              right: 8,
              bottom: 8,
            }}
            title="上传图片"
          />
        </Upload>

        {mentionListVisible && (
          <Card
            size="small"
            className="mention-dropdown"
            style={{
              position: 'absolute',
              bottom: '100%',
              left: 0,
              marginBottom: 4,
              minWidth: 200,
              zIndex: 1000,
            }}
          >
            <div ref={listRef}>
              {loadingContext ? (
                <div style={{ padding: 16, textAlign: 'center' }}>
                  <Spin size="small" />
                  <span style={{ marginLeft: 8 }}>加载中...</span>
                </div>
              ) : agentOptions.length === 0 ? (
                <div style={{ padding: 16, textAlign: 'center', color: '#999' }}>
                  当前团队没有可用的 Agent
                </div>
              ) : filteredAgents.length === 0 ? (
                <div style={{ padding: 16, textAlign: 'center', color: '#999' }}>
                  没有匹配的 Agent
                </div>
              ) : (
                <List
                  size="small"
                  dataSource={filteredAgents}
                  renderItem={(opt, index) => {
                    const isHighlighted = index === highlightedIndex;

                    return (
                      <List.Item
                        className="mention-list-item"
                        style={{
                          cursor: 'pointer',
                          padding: '8px 12px',
                          backgroundColor: isHighlighted ? 'var(--color-primary-opacity-10, rgba(24, 144, 255, 0.1))' : 'transparent',
                          borderRadius: 4,
                          transition: 'background-color 0.15s ease',
                        }}
                        onClick={() => selectMention(opt.name)}
                        onMouseEnter={() => setHighlightedIndex(index)}
                      >
                        <Space>
                          <AgentTypeIcon
                            requiresHuman={opt.requiresHuman}
                            isSystem={opt.isSystem || false}
                            size={16}
                          />
                          <span>{opt.label}</span>
                        </Space>
                      </List.Item>
                    );
                  }}
                />
              )}
            </div>
          </Card>
        )}
      </div>
      <Space direction="vertical">
        <Button type="primary" icon={<SendOutlined />} onClick={handleSend} disabled={!canSend}>
          发送
        </Button>
      </Space>
    </div>
  );
});

ThreadInput.displayName = 'ThreadInput';