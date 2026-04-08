import React from 'react';
import { MenuFoldOutlined, MenuUnfoldOutlined } from '@ant-design/icons';
import './Logo.css';

interface LogoProps {
  /** 尺寸模式 */
  size?: 'default' | 'small' | 'large';
  /** 是否收缩模式 */
  collapsed?: boolean;
  /** 收缩/展开回调 */
  onCollapse?: () => void;
}

/**
 * Logo 组件 - Colink 多智能体协作平台
 * 六边形网络设计：外环6个Agent节点，中心1个核心节点
 * 动效：亮点流动代表任务经过多个智能体协作
 */
const Logo: React.FC<LogoProps> = ({ size = 'default', collapsed = false, onCollapse }) => {
  return (
    <div className={`logo-container logo-${size} ${collapsed ? 'logo-collapsed' : ''}`}>
      <div className="logo-icon-wrapper">
        {/* 六边形网络 SVG */}
        <svg className="logo-svg" viewBox="0 0 32 32" fill="none">
          <defs>
            <linearGradient id="nodeGradient" x1="0%" y1="0%" x2="100%" y2="100%">
              <stop offset="0%" style={{ stopColor: '#10b981' }} />
              <stop offset="100%" style={{ stopColor: '#3b82f6' }} />
            </linearGradient>
          </defs>

          {/* 背景 */}
          <rect x="2" y="2" width="28" height="28" rx="6" fill="#0f172a" />

          {/* 六边形轮廓线 */}
          <polygon
            points="16,3 27,9 27,23 16,29 5,23 5,9"
            fill="none"
            stroke="#10b981"
            strokeWidth="1.5"
            strokeOpacity="0.35"
            strokeLinejoin="round"
          />

          {/* 从外环到中心的连接线 */}
          <g stroke="#10b981" strokeWidth="1" strokeOpacity="0.35">
            <line x1="16" y1="3" x2="16" y2="16" />
            <line x1="27" y1="9" x2="16" y2="16" />
            <line x1="27" y1="23" x2="16" y2="16" />
            <line x1="16" y1="29" x2="16" y2="16" />
            <line x1="5" y1="23" x2="16" y2="16" />
            <line x1="5" y1="9" x2="16" y2="16" />
          </g>

          {/* 外环节点 (6个) */}
          <circle cx="16" cy="3" r="2.5" fill="url(#nodeGradient)" className="logo-node" />
          <circle cx="27" cy="9" r="2.5" fill="url(#nodeGradient)" className="logo-node" />
          <circle cx="27" cy="23" r="2.5" fill="url(#nodeGradient)" className="logo-node" />
          <circle cx="16" cy="29" r="2.5" fill="url(#nodeGradient)" className="logo-node" />
          <circle cx="5" cy="23" r="2.5" fill="url(#nodeGradient)" className="logo-node" />
          <circle cx="5" cy="9" r="2.5" fill="url(#nodeGradient)" className="logo-node" />

          {/* 中心节点 */}
          <circle cx="16" cy="16" r="4" fill="url(#nodeGradient)" className="logo-node" />

          {/* 节点高光 */}
          <circle cx="16" cy="3" r="1" fill="white" opacity="0.3" />
          <circle cx="27" cy="9" r="1" fill="white" opacity="0.3" />
          <circle cx="27" cy="23" r="1" fill="white" opacity="0.3" />
          <circle cx="16" cy="29" r="1" fill="white" opacity="0.3" />
          <circle cx="5" cy="23" r="1" fill="white" opacity="0.3" />
          <circle cx="5" cy="9" r="1" fill="white" opacity="0.3" />
          <circle cx="16" cy="16" r="1.5" fill="white" opacity="0.4" />

          {/* 流动的亮点 - 代表任务流转 */}
          <circle className="flow-dot" r="1.5" />
        </svg>
      </div>
      {!collapsed && (
        <div className="logo-text">
          <span className="logo-title">Colink</span>
          <span className="logo-subtitle">多智能体协作平台</span>
        </div>
      )}
      {/* 收缩/展开按钮 */}
      {onCollapse && (
        <button
          className="logo-collapse-btn"
          onClick={onCollapse}
          title={collapsed ? '展开菜单' : '收起菜单'}
        >
          {collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
        </button>
      )}
    </div>
  );
};

export default Logo;