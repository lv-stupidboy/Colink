import React from 'react';
import { BulbOutlined, MenuFoldOutlined, MenuUnfoldOutlined } from '@ant-design/icons';
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
 * Logo 组件 - 熄灯工厂
 * 灯泡图标象征全自动化、无人值守的智能生产
 * 默认熄灭状态，hover时微光闪烁，寓意暗夜中自动运行
 */
const Logo: React.FC<LogoProps> = ({ size = 'default', collapsed = false, onCollapse }) => {
  return (
    <div className={`logo-container logo-${size} ${collapsed ? 'logo-collapsed' : ''}`}>
      <div className="logo-icon-wrapper">
        <BulbOutlined className="logo-icon" />
        {/* 熄灯效果 - 微弱的光晕 */}
        <div className="logo-glow">
          <span className="glow-ring glow-ring-1" />
          <span className="glow-ring glow-ring-2" />
          <span className="glow-ring glow-ring-3" />
        </div>
        {/* 星星粒子 - 象征暗夜中的自动化 */}
        <div className="logo-stars">
          <span className="star-particle star-1" />
          <span className="star-particle star-2" />
          <span className="star-particle star-3" />
        </div>
      </div>
      {!collapsed && (
        <div className="logo-text">
          <span className="logo-title">Lights-Out</span>
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