import React from 'react';
import { Outlet } from 'react-router-dom';
import { Menu } from 'antd';
import { SettingOutlined, RobotOutlined } from '@ant-design/icons';
import { useNavigate, useLocation } from 'react-router-dom';

const SettingsLayout: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();

  const menuItems = [
    {
      key: 'general',
      icon: <SettingOutlined />,
      label: '通用设置',
    },
    {
      key: 'base-agents',
      icon: <RobotOutlined />,
      label: '基础Agent设置',
    },
  ];

  const getSelectedKey = () => {
    const path = location.pathname;
    if (path.includes('base-agents')) {
      return 'base-agents';
    }
    return 'general';
  };

  const handleMenuClick = (key: string) => {
    navigate(`/settings/${key}`);
  };

  return (
    <div style={{ display: 'flex', gap: 24 }}>
      <div style={{ width: 240 }}>
        <Menu
          mode="inline"
          selectedKeys={[getSelectedKey()]}
          items={menuItems}
          onClick={({ key }) => handleMenuClick(key)}
          style={{ height: '100%', borderRight: 0 }}
        />
      </div>
      <div style={{ flex: 1 }}>
        <Outlet />
      </div>
    </div>
  );
};

export default SettingsLayout;