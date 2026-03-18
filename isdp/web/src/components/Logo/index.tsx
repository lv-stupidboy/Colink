import React from 'react';
import { RocketOutlined } from '@ant-design/icons';
import './Logo.css';

interface LogoProps {
  /** 是否显示副标题 */
  showSubtitle?: boolean;
  /** 尺寸模式 */
  size?: 'default' | 'small' | 'large';
}

/**
 * Logo 组件 - 方案 A：火箭图标
 * 代表创新与进取，适合智能软件开发平台定位
 */
const Logo: React.FC<LogoProps> = ({ showSubtitle = true, size = 'default' }) => {
  return (
    <div className={`logo-container logo-${size}`}>
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
      <div className="logo-text">
        <span className="logo-title">ISDP</span>
        {showSubtitle && <span className="logo-subtitle">智能软件开发平台</span>}
      </div>
    </div>
  );
};

export default Logo;