import React from 'react';
import { RocketOutlined, MenuFoldOutlined, MenuUnfoldOutlined } from '@ant-design/icons';
import './Logo.css';

interface LogoProps {
  /** 是否显示副标题 */
  showSubtitle?: boolean;
  /** 尺寸模式 */
  size?: 'default' | 'small' | 'large';
  /** 是否收缩模式 */
  collapsed?: boolean;
  /** 收缩/展开回调 */
  onCollapse?: () => void;
}

/**
 * Logo 组件 - 方案 A：火箭图标
 * 代表创新与进取，适合智能软件开发平台定位
 */
const Logo: React.FC<LogoProps> = ({ showSubtitle = true, size = 'default', collapsed = false, onCollapse }) => {
  return (
    <div className={`logo-container logo-${size} ${collapsed ? 'logo-collapsed' : ''}`}>
      <div className="logo-icon-wrapper">
        <RocketOutlined className="logo-icon" />
        {/* 火焰效果 */}
        <div className="logo-flame">
          <span className="flame-inner" />
          <span className="flame-outer" />
        </div>
        {/* 烟雾粒子 */}
        <div className="logo-smoke">
          <span className="smoke-particle smoke-1" />
          <span className="smoke-particle smoke-2" />
          <span className="smoke-particle smoke-3" />
        </div>
      </div>
      {!collapsed && (
        <div className="logo-text">
          <span className="logo-title">ISDP</span>
          {showSubtitle && <span className="logo-subtitle">智能软件开发平台</span>}
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