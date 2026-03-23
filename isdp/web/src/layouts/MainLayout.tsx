import React, { useState } from 'react';
import { Layout, Menu, Typography, Space, Tag } from 'antd';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import {
  DashboardOutlined,
  ProjectOutlined,
  ThunderboltOutlined,
  InboxOutlined,
  SettingOutlined,
  ApartmentOutlined,
  ExperimentOutlined,
  RobotOutlined,
  BookOutlined,
  CloudServerOutlined,
  DatabaseOutlined,
  CodeOutlined,
  OrderedListOutlined,
  ApiOutlined,
  SafetyCertificateOutlined,
  ControlOutlined,
} from '@ant-design/icons';
import type { MenuProps } from 'antd';
import ThemeSwitcher from '@/components/ThemeSwitcher';
import Logo from '@/components/Logo';
import { VERSION, BETA_LABEL } from '@/config/version';

const { Header, Sider, Content } = Layout;
const { Title } = Typography;

const MainLayout: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const [openKeys, setOpenKeys] = useState<string[]>([]);

  /**
   * 导航菜单配置
   */
  const menuItems: MenuProps['items'] = [
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
      key: 'agents',
      icon: <ThunderboltOutlined />,
      label: 'Agent 角色',
      children: [
        {
          key: '/agents/roles',
          icon: <RobotOutlined />,
          label: '角色管理',
        },
        {
          key: '/agents/commands',
          icon: <CodeOutlined />,
          label: '命令集',
        },
        {
          key: '/agents/subagents',
          icon: <ApiOutlined />,
          label: '子代理',
        },
        {
          key: '/agents/skills',
          icon: <BookOutlined />,
          label: '技能库',
        },
        {
          key: '/agents/rules',
          icon: <SafetyCertificateOutlined />,
          label: '规约',
        },
        {
          key: '/agents/plugins',
          icon: <ControlOutlined />,
          label: '插件',
        },
        {
          key: '/agents/hooks',
          icon: <OrderedListOutlined />,
          label: '钩子',
        },
        {
          key: '/agents/settings',
          icon: <SettingOutlined />,
          label: '设置',
        },
      ],
    },
    {
      key: '/knowledge',
      icon: <DatabaseOutlined />,
      label: '知识库',
    },
    {
      key: 'settings',
      icon: <SettingOutlined />,
      label: '设置',
      children: [
        {
          key: '/settings/general',
          icon: <SettingOutlined />,
          label: '通用设置',
        },
        {
          key: '/settings/base-agents',
          icon: <RobotOutlined />,
          label: '基础Agent设置',
        },
        {
          key: '/sandbox',
          icon: <InboxOutlined />,
          label: '沙箱环境',
        },
        {
          key: '/registries',
          icon: <CloudServerOutlined />,
          label: '联邦技能源',
        },
      ],
    },
  ];

  // 获取当前选中的菜单项
  const getSelectedKey = () => {
    const path = location.pathname;
    if (path.startsWith('/projects')) return '/projects';
    if (path.startsWith('/threads')) return '/projects';
    if (path.startsWith('/registries')) return '/registries';
    if (path.startsWith('/sandbox')) return '/sandbox';
    if (path.startsWith('/knowledge')) return '/knowledge';
    // Agent 角色子菜单路由
    if (path.startsWith('/agents/roles')) return '/agents/roles';
    if (path.startsWith('/agents/commands')) return '/agents/commands';
    if (path.startsWith('/agents/subagents')) return '/agents/subagents';
    if (path.startsWith('/agents/skills')) return '/agents/skills';
    if (path.startsWith('/agents/rules')) return '/agents/rules';
    if (path.startsWith('/agents/plugins')) return '/agents/plugins';
    if (path.startsWith('/agents/hooks')) return '/agents/hooks';
    if (path.startsWith('/agents/settings')) return '/agents/settings';
    if (path.startsWith('/agents')) return '/agents/roles'; // /agents 重定向到 /agents/roles
    if (path.startsWith('/settings')) return location.pathname;
    return path;
  };

  // 获取展开的子菜单
  const getOpenKeys = () => {
    const path = location.pathname;
    if (path.startsWith('/settings') || path.startsWith('/registries') || path.startsWith('/sandbox')) {
      return ['settings'];
    }
    // Agent 角色子菜单展开
    if (path.startsWith('/agents') || path.startsWith('/skills') || path.startsWith('/subagents')) {
      return ['agents'];
    }
    return [];
  };

  const handleMenuClick: MenuProps['onClick'] = ({ key }) => {
    if (key.startsWith('/')) {
      navigate(key);
    }
  };

  const handleOpenChange: MenuProps['onOpenChange'] = (keys) => {
    setOpenKeys(keys as string[]);
  };

  return (
    <Layout style={{ height: '100vh', overflow: 'hidden' }}>
      <Sider
        width={220}
        theme="light"
        style={{
          background: 'var(--bg-sidebar)',
          borderRight: '1px solid var(--border-color)',
          height: '100vh',
          overflow: 'auto',
        }}
      >
        <Logo />
        <Menu
          mode="inline"
          selectedKeys={[getSelectedKey()]}
          openKeys={openKeys.length > 0 ? openKeys : getOpenKeys()}
          items={menuItems}
          onClick={handleMenuClick}
          onOpenChange={handleOpenChange}
          style={{ borderRight: 0 }}
          // 覆盖子菜单父项样式，使其与其他菜单项对齐
          className="main-menu"
        />
      </Sider>
      <Layout style={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
        <Header
          style={{
            background: 'var(--bg-container)',
            padding: '0 24px',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            borderBottom: '1px solid var(--border-color)',
            flexShrink: 0,
          }}
        >
          <Title level={4} style={{ margin: 0, color: 'var(--text-primary)' }}>
            智能软件开发平台
          </Title>
          <Space size="small">
            <Tag color="orange" style={{ margin: 0 }}>
              <ExperimentOutlined /> {BETA_LABEL}
            </Tag>
            <Tag color="blue" style={{ margin: 0 }}>{VERSION}</Tag>
            <ThemeSwitcher />
          </Space>
        </Header>
        <Content
          style={{
            flex: 1,
            margin: 0,
            background: 'var(--bg-container)',
            padding: 16,
            boxShadow: 'var(--shadow-sm)',
            overflow: 'auto',
          }}
        >
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
};

export default MainLayout;
