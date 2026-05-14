import React, { useMemo, useEffect } from 'react';
import { ConfigProvider, App as AntApp, theme } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import MainLayout from '@/layouts/MainLayout';
import FloatingRobot from '@/components/FloatingRobot';
import ProjectList from '@/pages/ProjectList';
import ProjectDetail from '@/pages/ProjectDetail';
import ThreadView from '@/pages/ThreadView';
import AgentRoleList from '@/pages/AgentRoleList';
import Dashboard from '@/pages/Dashboard';
import SandboxPage from '@/pages/Sandbox';
import SettingsLayout from '@/pages/Settings/Layout';
import GeneralSettings from '@/pages/Settings/GeneralSettings';
import BaseAgentSettings from '@/pages/Settings/BaseAgentSettings';
import MarketLayout from '@/pages/Market/Layout';
import MarketManagement from '@/pages/Market/MarketManagement';
import TeamPackages from '@/pages/Market/TeamPackages';
import WorkflowPage from '@/pages/Workflow';
import TeamGraphEditor from '@/pages/Workflow/TeamGraphEditor';
import SkillLibrary from '@/pages/SkillLibrary';
import RegistryManagement from '@/pages/RegistryManagement';
import KnowledgeManagement from '@/pages/KnowledgeManagement';
import SubagentList from '@/pages/SubagentList';
import CommandList from '@/pages/CommandList';
import RuleList from '@/pages/RuleList';
import SettingsManagement from '@/pages/SettingsManagement';
import PlaceholderPage from '@/pages/PlaceholderPage';
import Tasks from '@/pages/Tasks';
import { useThemeStore } from '@/store/themeStore';
import '@/themes/themeVariables.css';

const App: React.FC = () => {
  const { themeConfig, setTheme } = useThemeStore();
  const { colors, isDark } = themeConfig;

  // 初始化主题
  useEffect(() => {
    const stored = localStorage.getItem('isdp-theme-storage');
    if (stored) {
      try {
        const parsed = JSON.parse(stored);
        if (parsed.state?.currentTheme) {
          setTheme(parsed.state.currentTheme);
        }
      } catch {
        // 忽略解析错误
      }
    }
  }, [setTheme]);

  // 动态生成 Ant Design token 配置
  const antdTheme = useMemo(() => {
    const baseToken = {
      colorPrimary: colors.primary,
      colorSuccess: colors.primary,
      colorWarning: '#f59e0b',
      colorError: '#ef4444',
      colorInfo: '#14b8a6',
      colorLink: colors.primary,
      colorLinkHover: colors.primaryHover,
      colorLinkActive: colors.primaryActive,
      borderRadius: 12,
      fontSize: 14,
      fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial',
      controlOutline: `var(--color-primary-opacity-28)`,
      colorPrimaryBorder: colors.primaryHover,
      colorPrimaryBg: colors.primaryBg,
      colorPrimaryBgHover: colors.primaryBgHover,
    };

    return {
      algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
      token: baseToken,
      components: {
        Button: {
          borderRadius: 12,
          fontWeight: 600,
          colorPrimary: '#fff',
          colorPrimaryBg: colors.primary,
          colorPrimaryBgHover: colors.primaryHover,
          primaryShadow: `0 4px 14px var(--color-primary-opacity-35)`,
        },
        Card: {
          borderRadius: 16,
          boxShadowTertiary: `0 8px 24px var(--color-primary-opacity-12)`,
        },
        Table: {
          borderRadius: 16,
          headerBg: isDark ? 'transparent' : `linear-gradient(180deg, var(--bg-base) 0%, var(--color-primary-light) 100%)`,
          rowHoverBg: `var(--color-primary-opacity-10)`,
        },
        Input: {
          borderRadius: 12,
          activeBorderColor: colors.primary,
          activeShadow: `0 0 0 3px var(--color-primary-opacity-10)`,
        },
        Menu: {
          itemBorderRadius: 12,
          itemSelectedBg: `var(--color-primary-opacity-10)`,
          itemSelectedColor: colors.textPrimary,
        },
        Modal: {
          borderRadiusLG: 20,
          headerBg: isDark ? 'transparent' : `linear-gradient(180deg, var(--bg-base) 0%, var(--bg-container) 100%)`,
        },
        Tag: {
          borderRadius: 8,
          defaultColor: colors.textPrimary,
          defaultBg: colors.primaryLight,
        },
        Progress: {
          borderRadius: 6,
          defaultColor: `linear-gradient(90deg, ${colors.primary} 0%, #14b8a6 100%)`,
        },
        Statistic: {
          contentFontSize: 28,
        },
      },
    };
  }, [colors, isDark]);

  return (
    <ConfigProvider locale={zhCN} theme={antdTheme}>
      <AntApp>
        {/* 悬浮机器人快捷入口 */}
        <FloatingRobot />
        <BrowserRouter>
          <Routes>
            <Route path="/" element={<MainLayout />}>
              <Route index element={<Dashboard />} />
              <Route path="dashboard" element={<Dashboard />} />
              <Route path="tasks" element={<Tasks />} />

              {/* 项目空间 - 按 PRD 要求的层级结构 */}
              <Route path="projects" element={<ProjectList />} />
              <Route path="projects/:projectId" element={<ProjectDetail />} />
              <Route path="projects/:projectId/threads/:threadId" element={<ThreadView />} />

              {/* 兼容旧路由，重定向到新路由 */}
              <Route path="threads/:threadId" element={<ThreadView />} />

              {/* Agent资产管理 - 子菜单结构 */}
              <Route path="agents" element={<Navigate to="/agents/roles" replace />} />
              <Route path="agents/roles" element={<AgentRoleList />} />
              <Route path="agents/commands" element={<CommandList />} />
              <Route path="agents/subagents" element={<SubagentList />} />
              <Route path="agents/skills" element={<SkillLibrary />} />
              <Route path="agents/rules" element={<RuleList />} />
              <Route path="agents/settings" element={<SettingsManagement />} />
              <Route path="agents/plugins" element={<PlaceholderPage title="插件管理" description="该功能正在开发中，敬请期待" />} />
              <Route path="agents/knowledge" element={<KnowledgeManagement />} />

              {/* Agent 调试路由 */}
              <Route path="agents/:agentId/debug" element={<ThreadView />} />
              {/* 调试模式路由 - 直接使用 ThreadView */}
              <Route path="debug/:agentId" element={<ThreadView />} />

              {/* 兼容旧路由，重定向到新路由 */}
              <Route path="skills" element={<Navigate to="/agents/skills" replace />} />
              <Route path="subagents" element={<Navigate to="/agents/subagents" replace />} />

              <Route path="workflow" element={<WorkflowPage />} />
              <Route path="workflow/team/:teamId/graph" element={<TeamGraphEditor />} />
              <Route path="sandbox" element={<SandboxPage />} />
              <Route path="registries" element={<RegistryManagement />} />
              <Route path="knowledge" element={<Navigate to="/agents/knowledge" replace />} />

              {/* 市场 - 二级菜单 */}
              <Route path="market" element={<MarketLayout />}>
                <Route index element={<Navigate to="management" replace />} />
                <Route path="management" element={<MarketManagement />} />
                <Route path="team-packages" element={<TeamPackages />} />
              </Route>

              {/* 设置页面 - 二级菜单 */}
              <Route path="settings" element={<SettingsLayout />}>
                <Route index element={<Navigate to="general" replace />} />
                <Route path="general" element={<GeneralSettings />} />
                <Route path="base-agents" element={<BaseAgentSettings />} />
              </Route>
            </Route>
          </Routes>
        </BrowserRouter>
      </AntApp>
    </ConfigProvider>
  );
};

export default App;