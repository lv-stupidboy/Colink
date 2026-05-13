import React, { useState, useEffect, useRef, useCallback } from 'react';
import { Select, Input, Button, message } from 'antd';
import {
  WechatOutlined,
  GlobalOutlined,
  BookOutlined,
  CloseOutlined,
  SendOutlined,
  MessageOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import api from '@/api/client';
import type { HelpConfig } from '@/types';
import './FloatingRobot.css';

// 反馈类型选项
const feedbackTypes = [
  { label: '功能问题 - 功能异常或无法使用', value: '功能问题' },
  { label: '体验问题 - 操作不便或界面问题', value: '体验问题' },
  { label: '性能问题 - 响应慢或卡顿', value: '性能问题' },
  { label: '建议反馈 - 功能建议或改进想法', value: '建议反馈' },
  { label: '其他 - 其他类型的问题', value: '其他' },
];

interface Position {
  side: 'left' | 'right';
  top: number;
}

interface FeedbackImage {
  id: string;
  dataUrl: string;
  name: string;
}

const FloatingRobot: React.FC = () => {
  const [position, setPosition] = useState<Position>(() => {
    // 从 localStorage 读取位置
    const saved = localStorage.getItem('floating-robot-position');
    if (saved) {
      try {
        return JSON.parse(saved);
      } catch {
        return { side: 'right', top: 100 };
      }
    }
    return { side: 'right', top: 100 };
  });

  const [isExpanded, setIsExpanded] = useState(false);
  const [isDragging, setIsDragging] = useState(false);
  const [dragStartY, setDragStartY] = useState(0);
  const [dragStartTop, setDragStartTop] = useState(0);
  const [helpConfig, setHelpConfig] = useState<HelpConfig | null>(null);
  const [loading, setLoading] = useState(false);
  const [feedbackType, setFeedbackType] = useState('功能问题');
  const [feedbackDesc, setFeedbackDesc] = useState('');
  const [feedbackImages, setFeedbackImages] = useState<FeedbackImage[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [showFeedbackPanel, setShowFeedbackPanel] = useState(false);
  const [panelUpward, setPanelUpward] = useState(false);

  const containerRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // 加载帮助配置
  useEffect(() => {
    const loadConfig = async () => {
      setLoading(true);
      try {
        const config = await api.help.getConfig();
        setHelpConfig(config);
      } catch (err) {
        console.warn('Failed to load help config:', err);
      } finally {
        setLoading(false);
      }
    };
    loadConfig();
  }, []);

  // 计算面板是否需要向上展开
  useEffect(() => {
    const windowHeight = window.innerHeight;
    const robotHeight = 48;
    const helpPanelHeight = 220; // 帮助面板预估高度
    const feedbackPanelHeight = 400; // 反馈面板预估高度（包含图片区域）

    // 检查帮助面板是否超出屏幕底部
    const helpPanelBottom = position.top + helpPanelHeight;
    const needHelpPanelUpward = helpPanelBottom > windowHeight - 20;

    // 检查反馈面板是否超出屏幕底部
    const feedbackPanelBottom = position.top + feedbackPanelHeight;
    const needFeedbackPanelUpward = feedbackPanelBottom > windowHeight - 20;

    setPanelUpward(needHelpPanelUpward || needFeedbackPanelUpward);
  }, [position.top, showFeedbackPanel, feedbackImages.length]);

  // 处理粘贴图片
  const handlePaste = useCallback((e: React.ClipboardEvent<HTMLTextAreaElement>) => {
    const items = e.clipboardData.items;

    for (let i = 0; i < items.length; i++) {
      const item = items[i];

      if (item.type.indexOf('image') !== -1) {
        const file = item.getAsFile();
        if (file) {
          // 检查图片数量限制
          if (feedbackImages.length >= 5) {
            message.warning('最多支持 5 张图片');
            return;
          }

          // 转换为 base64
          const reader = new FileReader();
          reader.onload = (event) => {
            const dataUrl = event.target?.result as string;
            const newImage: FeedbackImage = {
              id: `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
              dataUrl,
              name: file.name || `图片-${feedbackImages.length + 1}`,
            };
            setFeedbackImages(prev => [...prev, newImage]);
            message.success('图片已添加');
          };
          reader.readAsDataURL(file);
        }
      }
    }
  }, [feedbackImages.length]);

  // 删除图片
  const handleRemoveImage = useCallback((imageId: string) => {
    setFeedbackImages(prev => prev.filter(img => img.id !== imageId));
  }, []);

  // 拖拽处理
  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    if (isExpanded) return;
    setIsDragging(true);
    setDragStartY(e.clientY);
    setDragStartTop(position.top);
    e.preventDefault();
  }, [isExpanded, position.top]);

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (!isDragging) return;

    const deltaY = e.clientY - dragStartY;
    const newTop = Math.max(50, Math.min(window.innerHeight - 100, dragStartTop + deltaY));

    // 判断左右位置
    const centerX = window.innerWidth / 2;
    const newSide = e.clientX < centerX ? 'left' : 'right';

    setPosition({ side: newSide, top: newTop });
  }, [isDragging, dragStartY, dragStartTop]);

  const handleMouseUp = useCallback(() => {
    if (!isDragging) return;
    setIsDragging(false);
    // 保存位置
    localStorage.setItem('floating-robot-position', JSON.stringify(position));
  }, [isDragging, position]);

  // 全局拖拽事件
  useEffect(() => {
    if (isDragging) {
      window.addEventListener('mousemove', handleMouseMove);
      window.addEventListener('mouseup', handleMouseUp);
      return () => {
        window.removeEventListener('mousemove', handleMouseMove);
        window.removeEventListener('mouseup', handleMouseUp);
      };
    }
  }, [isDragging, handleMouseMove, handleMouseUp]);

  // 点击展开/收起
  const handleClick = useCallback(() => {
    if (isDragging) return;
    setIsExpanded(!isExpanded);
    setShowFeedbackPanel(false);
  }, [isDragging, isExpanded]);

  // 关闭面板
  const handleClose = useCallback(() => {
    setIsExpanded(false);
    setShowFeedbackPanel(false);
  }, []);

  // 复制群号
  const handleCopyGroup = useCallback(() => {
    if (helpConfig?.supportGroup) {
      navigator.clipboard.writeText(helpConfig.supportGroup);
      message.success('群号已复制');
    }
  }, [helpConfig]);

  // 打开官网
  const handleOpenWebsite = useCallback(() => {
    if (helpConfig?.officialWebsite) {
      window.open(helpConfig.officialWebsite, '_blank');
    }
  }, [helpConfig]);

  // 打开文档
  const handleOpenDoc = useCallback(() => {
    if (helpConfig?.docLink) {
      window.open(helpConfig.docLink, '_blank');
    }
  }, [helpConfig]);

  // 打开反馈面板
  const handleOpenFeedback = useCallback(() => {
    setShowFeedbackPanel(true);
  }, []);

  // 关闭反馈面板
  const handleCloseFeedback = useCallback(() => {
    setShowFeedbackPanel(false);
    setFeedbackDesc('');
    setFeedbackImages([]);
  }, []);

  // 提交反馈
  const handleSubmitFeedback = useCallback(async () => {
    if (!feedbackDesc.trim() && feedbackImages.length === 0) {
      message.warning('请填写问题描述或添加图片');
      return;
    }

    setSubmitting(true);
    try {
      await api.help.submitFeedback({
        type: feedbackType,
        description: feedbackDesc,
        images: feedbackImages.map(img => ({
          name: img.name,
          data: img.dataUrl,
        })),
      });
      message.success('反馈已提交，感谢您的反馈！');
      setFeedbackDesc('');
      setFeedbackImages([]);
      setShowFeedbackPanel(false);
      setIsExpanded(false);
    } catch (err: any) {
      const errorMsg = err.response?.data?.error || '提交失败';
      message.error(errorMsg);
    } finally {
      setSubmitting(false);
    }
  }, [feedbackType, feedbackDesc, feedbackImages]);

  if (loading && !helpConfig) {
    return null;
  }

  // 即使配置为空也显示图标，面板中提示暂未配置

  return (
    <div
      ref={containerRef}
      className={`floating-robot ${position.side} ${isExpanded ? 'expanded' : ''} ${isDragging ? 'dragging' : ''} ${panelUpward ? 'panel-upward' : ''}`}
      style={{ top: position.top }}
      onMouseDown={handleMouseDown}
      onClick={handleClick}
    >
      {/* 机器人按钮 */}
      <div className={`robot-btn ${isExpanded ? 'rippling' : ''}`}>
        <svg className="robot-icon" viewBox="0 0 48 48" fill="currentColor">
          {/* 机器人头部 */}
          <rect x="10" y="12" width="28" height="24" rx="6" />
          {/* 眼睛 */}
          <circle cx="18" cy="22" r="4" fill="white" />
          <circle cx="30" cy="22" r="4" fill="white" />
          <circle cx="18" cy="22" r="2" fill="#1e293b" />
          <circle cx="30" cy="22" r="2" fill="#1e293b" />
          {/* 天线 */}
          <line x1="24" y1="4" x2="24" y2="12" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
          <circle cx="24" cy="4" r="3" />
          {/* 嘴巴 */}
          <rect x="16" y="30" width="16" height="4" rx="2" fill="white" fillOpacity="0.6" />
          {/* 耳朵 */}
          <rect x="4" y="18" width="6" height="10" rx="2" />
          <rect x="38" y="18" width="6" height="10" rx="2" />
        </svg>
      </div>

      {/* 展开面板 */}
      <div className="robot-panel" onClick={(e) => e.stopPropagation()}>
        {/* 关闭按钮 */}
        <button className="panel-close" onClick={handleClose}>
          <CloseOutlined />
        </button>

        {/* 标题 */}
        <div className="panel-title">
          <span>帮助与反馈</span>
        </div>

        {/* 信息项 */}
        {helpConfig?.supportGroup && (
          <div className="panel-item clickable" onClick={handleCopyGroup}>
            <WechatOutlined className="panel-item-icon" />
            <div className="panel-item-content">
              <span className="panel-item-label">支撑群号</span>
              <span className="panel-item-value">{helpConfig.supportGroup}</span>
            </div>
          </div>
        )}

        {helpConfig?.officialWebsite && (
          <div className="panel-item clickable" onClick={handleOpenWebsite}>
            <GlobalOutlined className="panel-item-icon" />
            <div className="panel-item-content">
              <span className="panel-item-label">官方网站</span>
            </div>
          </div>
        )}

        {helpConfig?.docLink && (
          <div className="panel-item clickable" onClick={handleOpenDoc}>
            <BookOutlined className="panel-item-icon" />
            <div className="panel-item-content">
              <span className="panel-item-label">指导文档</span>
            </div>
          </div>
        )}

        {helpConfig?.feedbackEnabled && (
          <div className="panel-item clickable" onClick={handleOpenFeedback}>
            <MessageOutlined className="panel-item-icon" />
            <div className="panel-item-content">
              <span className="panel-item-label">问题反馈</span>
            </div>
          </div>
        )}

        {/* 空状态提示 */}
        {!helpConfig?.supportGroup && !helpConfig?.officialWebsite && !helpConfig?.docLink && !helpConfig?.feedbackEnabled && (
          <div className="panel-empty">
            <span className="panel-empty-text">暂未配置帮助信息</span>
          </div>
        )}
      </div>

      {/* 反馈表单面板 */}
      {showFeedbackPanel && (
        <div
          className={`feedback-panel ${position.side === 'left' ? 'feedback-panel-right' : 'feedback-panel-left'} ${panelUpward ? 'feedback-panel-upward' : ''}`}
          onClick={(e) => e.stopPropagation()}
        >
          {/* 关闭按钮 */}
          <button className="panel-close" onClick={handleCloseFeedback}>
            <CloseOutlined />
          </button>

          {/* 标题 */}
          <div className="panel-title">
            <span>提交反馈</span>
          </div>

          {/* 反馈类型 */}
          <div className="feedback-form-section">
            <div className="feedback-form-label">问题类型</div>
            <Select
              className="feedback-type-select"
              value={feedbackType}
              onChange={setFeedbackType}
              options={feedbackTypes}
              style={{ width: '100%' }}
              placement={panelUpward ? 'topLeft' : 'bottomLeft'}
              getPopupContainer={(triggerNode) => triggerNode.parentElement || document.body}
            />
          </div>

          {/* 问题描述 */}
          <div className="feedback-form-section">
            <div className="feedback-form-label">问题描述（支持粘贴图片）</div>
            <Input.TextArea
              ref={textareaRef}
              className="feedback-textarea"
              placeholder="请详细描述您遇到的问题或建议，可直接粘贴截图..."
              value={feedbackDesc}
              onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setFeedbackDesc(e.target.value)}
              onPaste={handlePaste}
              rows={4}
              maxLength={1000}
              showCount
            />
          </div>

          {/* 图片预览区域 */}
          {feedbackImages.length > 0 && (
            <div className="feedback-images-section">
              <div className="feedback-images-label">
                图片附件 ({feedbackImages.length}/5)
              </div>
              <div className="feedback-images-preview">
                {feedbackImages.map(img => (
                  <div key={img.id} className="feedback-image-item">
                    <img src={img.dataUrl} alt={img.name} className="feedback-image-thumb" />
                    <button
                      className="feedback-image-remove"
                      onClick={() => handleRemoveImage(img.id)}
                      title="删除图片"
                    >
                      <DeleteOutlined />
                    </button>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* 提交按钮 */}
          <Button
            className="feedback-submit-btn"
            type="primary"
            icon={<SendOutlined />}
            onClick={handleSubmitFeedback}
            loading={submitting}
          >
            提交反馈
          </Button>
        </div>
      )}
    </div>
  );
};

export default FloatingRobot;