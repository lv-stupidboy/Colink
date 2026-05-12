import React, { useState, useEffect, useRef, useCallback } from 'react';
import { Select, Input, Button, message } from 'antd';
import {
  WechatOutlined,
  GlobalOutlined,
  BookOutlined,
  CloseOutlined,
  SendOutlined,
} from '@ant-design/icons';
import api from '@/api/client';
import type { HelpConfig } from '@/types';
import './FloatingRobot.css';

// 反馈类型选项
const feedbackTypes = [
  { label: '功能问题', value: '功能问题' },
  { label: '体验问题', value: '体验问题' },
  { label: '建议反馈', value: '建议反馈' },
  { label: '其他', value: '其他' },
];

interface Position {
  side: 'left' | 'right';
  top: number;
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
  const [submitting, setSubmitting] = useState(false);

  const containerRef = useRef<HTMLDivElement>(null);

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
  }, [isDragging, isExpanded]);

  // 关闭面板
  const handleClose = useCallback(() => {
    setIsExpanded(false);
  }, []);

  // 复制群号
  const handleCopyGroup = useCallback(() => {
    if (helpConfig?.support_group) {
      navigator.clipboard.writeText(helpConfig.support_group);
      message.success('群号已复制');
    }
  }, [helpConfig]);

  // 打开官网
  const handleOpenWebsite = useCallback(() => {
    if (helpConfig?.official_website) {
      window.open(helpConfig.official_website, '_blank');
    }
  }, [helpConfig]);

  // 打开文档
  const handleOpenDoc = useCallback(() => {
    if (helpConfig?.doc_link) {
      window.open(helpConfig.doc_link, '_blank');
    }
  }, [helpConfig]);

  // 提交反馈
  const handleSubmitFeedback = useCallback(async () => {
    if (!feedbackDesc.trim()) {
      message.warning('请填写问题描述');
      return;
    }

    setSubmitting(true);
    try {
      await api.help.submitFeedback({
        type: feedbackType,
        description: feedbackDesc,
      });
      message.success('反馈已提交，感谢您的反馈！');
      setFeedbackDesc('');
      setIsExpanded(false);
    } catch (err: any) {
      const errorMsg = err.response?.data?.error || '提交失败';
      message.error(errorMsg);
    } finally {
      setSubmitting(false);
    }
  }, [feedbackType, feedbackDesc]);

  if (loading && !helpConfig) {
    return null;
  }

  // 如果所有配置都为空，不显示
  if (!helpConfig?.support_group && !helpConfig?.official_website && !helpConfig?.doc_link && !helpConfig?.feedback_enabled) {
    return null;
  }

  return (
    <div
      ref={containerRef}
      className={`floating-robot ${position.side} ${isExpanded ? 'expanded' : ''} ${isDragging ? 'dragging' : ''}`}
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
        {helpConfig?.support_group && (
          <div className="panel-item clickable" onClick={handleCopyGroup}>
            <WechatOutlined className="panel-item-icon" />
            <div className="panel-item-content">
              <span className="panel-item-label">支撑群号</span>
              <span className="panel-item-value">{helpConfig.support_group}</span>
            </div>
          </div>
        )}

        {helpConfig?.official_website && (
          <div className="panel-item clickable" onClick={handleOpenWebsite}>
            <GlobalOutlined className="panel-item-icon" />
            <div className="panel-item-content">
              <span className="panel-item-label">官方网站</span>
            </div>
          </div>
        )}

        {helpConfig?.doc_link && (
          <div className="panel-item clickable" onClick={handleOpenDoc}>
            <BookOutlined className="panel-item-icon" />
            <div className="panel-item-content">
              <span className="panel-item-label">指导文档</span>
            </div>
          </div>
        )}

        {/* 反馈表单 */}
        {helpConfig?.feedback_enabled && (
          <>
            <div className="panel-divider" />
            <div className="feedback-section">
              <div className="feedback-title">问题反馈</div>
              <Select
                className="feedback-type-select"
                value={feedbackType}
                onChange={setFeedbackType}
                options={feedbackTypes}
                style={{ width: '100%' }}
              />
              <Input.TextArea
                className="feedback-textarea"
                placeholder="请描述您遇到的问题..."
                value={feedbackDesc}
                onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setFeedbackDesc(e.target.value)}
                rows={3}
                maxLength={500}
              />
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
          </>
        )}
      </div>
    </div>
  );
};

export default FloatingRobot;