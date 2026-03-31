import React, { useState, useEffect } from 'react';
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
  ApiOutlined,
  SafetyCertificateOutlined,
  ControlOutlined,
  CompassOutlined,
  ContainerOutlined,
} from '@ant-design/icons';
import type { MenuProps } from 'antd';
import ThemeSwitcher from '@/components/ThemeSwitcher';
import Logo from '@/components/Logo';
import { VERSION, BETA_LABEL } from '@/config/version';
import { useAppStore } from '@/store';

const { Header, Sider, Content } = Layout;
const { Title } = Typography;

const MainLayout: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const [openKeys, setOpenKeys] = useState<string[]>([]);
  const [collapsed, setCollapsed] = useState(false);

  // 从 store 读取 soloMode 状态
  const soloMode = useAppStore((state) => state.soloMode);

  // 根据路径初始化展开的子菜单
  useEffect(() => {
    const path = location.pathname;
    if (path.startsWith('/settings') || path.startsWith('/registries')) {
      setOpenKeys(['settings']);
    } else if (path.startsWith('/agents')) {
      // Agent团队下的路径，需要展开到三级菜单
      if (path.startsWith('/asset-packages') ||
          path.startsWith('/agents/commands') || path.startsWith('/agents/subagents') ||
          path.startsWith('/agents/skills') || path.startsWith('/agents/rules') ||
          path.startsWith('/agents/settings') ||
          path.startsWith('/agents/plugins')) {
        setOpenKeys(['agents', 'assets']);
      } else {
        setOpenKeys(['agents']);
      }
    } else if (path.startsWith('/planning') || path.startsWith('/knowledge') || path.startsWith('/sandbox')) {
      setOpenKeys(['planning']);
    } else if (path === '/workflow') {
      setOpenKeys(['agents']);
    }
  }, []); // 只在初始化时执行一次

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
      key: 'agents',
      icon: <ApartmentOutlined />,
      label: 'Agent团队',
      children: [
        {
          key: '/workflow',
          icon: <ApartmentOutlined />,
          label: '团队管理',
        },
        {
          key: '/agents/roles',
          icon: <RobotOutlined />,
          label: '角色管理',
        },
        {
          key: 'assets',
          icon: <ThunderboltOutlined />,
          label: '角色资产',
          children: [
            {
              key: '/asset-packages',
              icon: <ContainerOutlined />,
              label: '资产包',
            },
            {
              key: '/agents/commands',
              icon: <CodeOutlined />,
              label: 'Commands',
            },
            {
              key: '/agents/subagents',
              icon: <ApiOutlined />,
              label: 'Subagents',
            },
            {
              key: '/agents/skills',
              icon: <BookOutlined />,
              label: 'Skills',
            },
            {
              key: '/agents/rules',
              icon: <SafetyCertificateOutlined />,
              label: 'Rules',
            },
            {
              key: '/agents/settings',
              icon: <SettingOutlined />,
              label: 'Settings',
            },
            {
              key: '/agents/plugins',
              icon: <ControlOutlined />,
              label: 'Plugins',
            },
          ],
        },
      ],
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
          key: '/registries',
          icon: <CloudServerOutlined />,
          label: '联邦技能源',
        },
      ],
    },
    {
      key: 'planning',
      icon: <CompassOutlined />,
      label: '规划板块',
      children: [
        {
          key: '/agents/knowledge',
          icon: <DatabaseOutlined />,
          label: '知识库',
        },
        {
          key: '/sandbox',
          icon: <InboxOutlined />,
          label: '沙箱环境',
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
    if (path === '/workflow') return '/workflow';
    // Agent 团队子菜单路由
    if (path.startsWith('/asset-packages')) return '/asset-packages';
    if (path.startsWith('/agents/roles')) return '/agents/roles';
    if (path.startsWith('/agents/commands')) return '/agents/commands';
    if (path.startsWith('/agents/subagents')) return '/agents/subagents';
    if (path.startsWith('/agents/skills')) return '/agents/skills';
    if (path.startsWith('/agents/rules')) return '/agents/rules';
    if (path.startsWith('/agents/settings')) return '/agents/settings';
    if (path.startsWith('/agents/plugins')) return '/agents/plugins';
    if (path.startsWith('/agents/knowledge')) return '/agents/knowledge';
    if (path.startsWith('/agents')) return '/agents/roles'; // /agents 重定向到 /agents/roles
    if (path.startsWith('/settings')) return location.pathname;
    return path;
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
        trigger={null}
        collapsible
        collapsed={collapsed}
        width={220}
        collapsedWidth={64}
        theme="light"
        style={{
          background: 'var(--bg-sidebar)',
          borderRight: '1px solid var(--border-color)',
          height: '100vh',
          overflow: 'auto',
        }}
      >
        <Logo
          collapsed={collapsed}
          onCollapse={() => setCollapsed(!collapsed)}
        />
        <Menu
          mode="inline"
          selectedKeys={[getSelectedKey()]}
          openKeys={openKeys}
          items={menuItems}
          onClick={handleMenuClick}
          onOpenChange={handleOpenChange}
          style={{ borderRight: 0 }}
          // 覆盖子菜单父项样式，使其与其他菜单项对齐
          className="main-menu"
        />
      </Sider>
      <Layout style={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
        {/* Solo 模式下隐藏 Header，因为 ThreadView 有自己的 header */}
        {!soloMode && (
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
            Lights-Out Factory - 熄灯工厂
          </Title>
          <Space size="small">
            <Tag color="orange" style={{ margin: 0 }}>
              <ExperimentOutlined /> {BETA_LABEL}
            </Tag>
            <Tag color="blue" style={{ margin: 0 }}>{VERSION}</Tag>
            <ThemeSwitcher />
          </Space>
        </Header>
        )}
        <Content
          style={{
            flex: 1,
            margin: 0,
            background: 'var(--bg-container)',
            padding: soloMode ? 0 : 16,
            boxShadow: 'var(--shadow-sm)',
            overflow: 'auto',
            position: 'relative',
          }}
        >
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
};

export default MainLayout;
