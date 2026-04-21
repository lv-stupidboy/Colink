/**
 * Agent 类型图标组件
 * 根据 requiresHuman 和 isSystem 显示不同图标组合
 */

import React from 'react';
import { Badge } from 'antd';
import { RobotOutlined, UserOutlined, CrownOutlined } from '@ant-design/icons';

interface AgentTypeIconProps {
  requiresHuman: boolean;  // 是否需要人工参与
  isSystem?: boolean;      // 是否为系统预置角色
  size?: number;           // 图标大小
  iconColor?: string;      // 图标颜色（默认 var(--color-primary），工作流中应使用 #fff）
  className?: string;
  style?: React.CSSProperties;
}

const AgentTypeIcon: React.FC<AgentTypeIconProps> = ({
  requiresHuman,
  isSystem = false,
  size = 24,
  iconColor,
  className,
  style,
}) => {
  // 默认图标颜色
  const defaultColor = iconColor || 'var(--color-primary)';

  // 系统预置角色：使用皇冠图标
  if (isSystem) {
    return (
      <CrownOutlined
        className={className}
        style={{ fontSize: size, color: iconColor || '#faad14', ...style }}
      />
    );
  }

  // 需要人工参与：机器人图标 + Badge叠加人形小图标
  if (requiresHuman) {
    // Badge 小图标尺寸：约为主图标的一半
    const badgeSize = size * 0.5;

    // 关键修复：当 iconColor="#fff"（在蓝色背景上）时，小人图标需要用蓝色才能在白色背景上可见
    const userIconColor = iconColor === '#fff' ? 'var(--color-primary)' : defaultColor;
    // Badge 位置偏移 - 右下角
    // Badge offset: [水平偏移, 垂直偏移]，正值向右向下
    // 默认在右上角，要移到右下角需要较大的垂直向下偏移
    const badgeOffset = [-size * 0.08, size * 0.75];

    return (
      <Badge
        className={className}
        style={style}
        count={
          <div
            style={{
              width: badgeSize + 2,
              height: badgeSize + 2,
              borderRadius: '50%',
              backgroundColor: '#fff',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 1px 3px rgba(0,0,0,0.15)',
            }}
          >
            <UserOutlined
              style={{
                fontSize: badgeSize - 2,
                color: userIconColor,
              }}
            />
          </div>
        }
        offset={badgeOffset}
        size="default"
      >
        <RobotOutlined style={{ fontSize: size, color: defaultColor }} />
      </Badge>
    );
  }

  // 纯 Agent：机器人图标
  return (
    <RobotOutlined
      className={className}
      style={{ fontSize: size, color: defaultColor, ...style }}
    />
  );
};

export default AgentTypeIcon;