/**
 * 主题切换组件
 * 使用 Ant Design Dropdown，显示色块预览和主题名称
 */

import React from 'react';
import { Dropdown, Button } from 'antd';
import { BgColorsOutlined, CheckOutlined } from '@ant-design/icons';
import { useThemeStore } from '@/store/themeStore';
import { themeList } from '@/themes/themeConfig';
import type { ThemeName } from '@/themes/types';
import './ThemeSwitcher.css';

const ThemeSwitcher: React.FC = () => {
  const { currentTheme, setTheme } = useThemeStore();

  const items = themeList.map((theme) => ({
    key: theme.name,
    label: (
      <div className="theme-menu-item">
        <div
          className="theme-color-preview"
          style={{ backgroundColor: theme.color }}
        />
        <span className="theme-label">{theme.label}</span>
        {currentTheme === theme.name && (
          <CheckOutlined className="theme-check-icon" />
        )}
      </div>
    ),
    onClick: () => setTheme(theme.name as ThemeName),
  }));

  return (
    <Dropdown
      menu={{ items, selectedKeys: [currentTheme] }}
      trigger={['click']}
      placement="bottomRight"
    >
      <Button
        type="text"
        className="theme-switcher-btn"
        icon={<BgColorsOutlined />}
      >
        主题
      </Button>
    </Dropdown>
  );
};

export default ThemeSwitcher;