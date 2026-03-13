import React from 'react';
import { ConfigProvider, App as AntApp, theme } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import MainLayout from '@/layouts/MainLayout';
import ProjectList from '@/pages/ProjectList';
import ProjectDetail from '@/pages/ProjectDetail';
import ThreadView from '@/pages/ThreadView';
import AgentConfig from '@/pages/AgentConfig';
import Dashboard from '@/pages/Dashboard';
import SandboxPage from '@/pages/Sandbox';
import SettingsPage from '@/pages/Settings';
import WorkflowPage from '@/pages/Workflow';

const App: React.FC = () => {
  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: theme.defaultAlgorithm,
        token: {
          // C 清新活力风配色 - 翡翠绿主题
          colorPrimary: '#10b981',
          colorSuccess: '#10b981',
          colorWarning: '#f59e0b',
          colorError: '#ef4444',
          colorInfo: '#14b8a6',
          colorLink: '#10b981',
          colorLinkHover: '#059669',
          colorLinkActive: '#047857',
          borderRadius: 12,
          fontSize: 14,
          fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial',
          // 科技感细节 - 轻微发光效果
          controlOutline: 'rgba(16, 185, 129, 0.28)',
          colorPrimaryBorder: '#059669',
          colorPrimaryBg: '#d1fae5',
          colorPrimaryBgHover: '#a7f3d0',
        },
        components: {
          Button: {
            borderRadius: 12,
            fontWeight: 600,
            colorPrimary: '#fff',
            colorPrimaryBg: '#10b981',
            colorPrimaryBgHover: '#059669',
            primaryShadow: '0 4px 14px rgba(16, 185, 129, 0.35)',
          },
          Card: {
            borderRadius: 16,
            boxShadowTertiary: '0 8px 24px rgba(16, 185, 129, 0.12)',
          },
          Table: {
            borderRadius: 16,
            headerBg: 'linear-gradient(180deg, #f0fdf4 0%, #ecfdf5 100%)',
            rowHoverBg: 'rgba(16, 185, 129, 0.04)',
          },
          Input: {
            borderRadius: 12,
            activeBorderColor: '#10b981',
            activeShadow: '0 0 0 3px rgba(16, 185, 129, 0.1)',
          },
          Menu: {
            itemBorderRadius: 12,
            itemSelectedBg: 'rgba(16, 185, 129, 0.08)',
            itemSelectedColor: '#047857',
          },
          Modal: {
            borderRadiusLG: 20,
            headerBg: 'linear-gradient(180deg, #f0fdf4 0%, #ffffff 100%)',
          },
          Tag: {
            borderRadius: 8,
            defaultColor: '#047857',
            defaultBg: '#ecfdf5',
          },
          Progress: {
            borderRadius: 6,
            defaultColor: 'linear-gradient(90deg, #10b981 0%, #14b8a6 100%)',
          },
          Statistic: {
            contentFontSize: 28,
          },
        },
      }}
    >
      <AntApp>
        <BrowserRouter>
          <Routes>
            <Route path="/" element={<MainLayout />}>
              <Route index element={<Dashboard />} />
              <Route path="dashboard" element={<Dashboard />} />

              {/* 项目空间 - 按 PRD 要求的层级结构 */}
              <Route path="projects" element={<ProjectList />} />
              <Route path="projects/:projectId" element={<ProjectDetail />} />
              <Route path="projects/:projectId/threads/:threadId" element={<ThreadView />} />

              {/* 兼容旧路由，重定向到新路由 */}
              <Route path="threads/:threadId" element={<ThreadView />} />

              <Route path="agents" element={<AgentConfig />} />
              <Route path="workflow" element={<WorkflowPage />} />
              <Route path="sandbox" element={<SandboxPage />} />
              <Route path="settings" element={<SettingsPage />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </AntApp>
    </ConfigProvider>
  );
};

export default App;