import React from 'react';
import { Layout, Menu, Typography } from 'antd';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import {
  DashboardOutlined,
  ProjectOutlined,
  ThunderboltOutlined,
  InboxOutlined,
  SettingOutlined,
  ApartmentOutlined,
} from '@ant-design/icons';

const { Header, Sider, Content } = Layout;
const { Title } = Typography;

const MainLayout: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();

  /**
   * 导航菜单配置
   * PRD Section 1.5 - Web页面布局
   *
   * 页面结构：
   * ├── 首页/仪表盘
   * ├── 项目空间
   * ├── 工作流编排
   * ├── 沙箱环境
   * └── 系统设置
   */
  const menuItems = [
    {
      key: '/dashboard',
      icon: <DashboardOutlined />,
      label: '首页',
    },
    {
      key: '/projects',
      icon: <ProjectOutlined />,
      label: '项目空间',
    },
    {
      key: '/workflow',
      icon: <ApartmentOutlined />,
      label: '工作流编排',
    },
    {
      key: '/agents',
      icon: <ThunderboltOutlined />,
      label: 'Agent 角色',
    },
    {
      key: '/sandbox',
      icon: <InboxOutlined />,
      label: '沙箱环境',
    },
    {
      key: '/settings',
      icon: <SettingOutlined />,
      label: '设置',
    },
  ];

  // 获取当前选中的菜单项
  const getSelectedKey = () => {
    const path = location.pathname;
    if (path.startsWith('/projects')) return '/projects';
    if (path.startsWith('/threads')) return '/projects';
    if (path.startsWith('/settings')) return '/settings';
    return path;
  };

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider width={220} theme="light">
        <div style={{ padding: '16px', borderBottom: '1px solid #f0f0f0' }}>
          <Title level={4} style={{ margin: 0 }}>
            ISDP
          </Title>
          <div style={{ fontSize: 12, color: '#999' }}>智能软件开发平台</div>
        </div>
        <Menu
          mode="inline"
          selectedKeys={[getSelectedKey()]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <Layout>
        <Header style={{ background: '#fff', padding: '0 24px' }}>
          <Title level={4} style={{ margin: '16px 0' }}>
            智能软件开发平台
          </Title>
        </Header>
        <Content style={{ margin: 16, background: '#fff', borderRadius: 8, padding: 24 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
};

export default MainLayout;
