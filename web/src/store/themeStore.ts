/**
 * 主题状态管理
 * 使用 Zustand + persist 中间件持久化用户选择
 */

import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { ThemeName, ThemeState, ThemeActions } from '@/themes/types';
import { themeConfigs, defaultTheme } from '@/themes/themeConfig';

type ThemeStore = ThemeState & ThemeActions;

/**
 * 生成带主题色的 favicon SVG - Colink 网络圆环设计
 */
const generateFavicon = (primaryColor: string, secondaryColor: string): string => {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" fill="none">
  <defs>
    <linearGradient id="nodeGradient" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" style="stop-color:${primaryColor}"/>
      <stop offset="100%" style="stop-color:${secondaryColor}"/>
    </linearGradient>
  </defs>
  <rect x="2" y="2" width="28" height="28" rx="6" fill="#0f172a"/>
  <polygon points="16,3 27,9 27,23 16,29 5,23 5,9" fill="none" stroke="${primaryColor}" stroke-width="1.5" stroke-opacity="0.35" stroke-linejoin="round"/>
  <g stroke="${primaryColor}" stroke-width="1" stroke-opacity="0.35">
    <line x1="16" y1="3" x2="16" y2="16"/>
    <line x1="27" y1="9" x2="16" y2="16"/>
    <line x1="27" y1="23" x2="16" y2="16"/>
    <line x1="16" y1="29" x2="16" y2="16"/>
    <line x1="5" y1="23" x2="16" y2="16"/>
    <line x1="5" y1="9" x2="16" y2="16"/>
  </g>
  <circle cx="16" cy="3" r="2.5" fill="url(#nodeGradient)"/>
  <circle cx="27" cy="9" r="2.5" fill="url(#nodeGradient)"/>
  <circle cx="27" cy="23" r="2.5" fill="url(#nodeGradient)"/>
  <circle cx="16" cy="29" r="2.5" fill="url(#nodeGradient)"/>
  <circle cx="5" cy="23" r="2.5" fill="url(#nodeGradient)"/>
  <circle cx="5" cy="9" r="2.5" fill="url(#nodeGradient)"/>
  <circle cx="16" cy="16" r="4" fill="url(#nodeGradient)"/>
  <circle cx="16" cy="3" r="1" fill="white" opacity="0.3"/>
  <circle cx="27" cy="9" r="1" fill="white" opacity="0.3"/>
  <circle cx="27" cy="23" r="1" fill="white" opacity="0.3"/>
  <circle cx="16" cy="29" r="1" fill="white" opacity="0.3"/>
  <circle cx="5" cy="23" r="1" fill="white" opacity="0.3"/>
  <circle cx="5" cy="9" r="1" fill="white" opacity="0.3"/>
  <circle cx="16" cy="16" r="1.5" fill="white" opacity="0.4"/>
</svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
};

/**
 * 更新 favicon
 */
const updateFavicon = (primaryColor: string, primaryHover: string) => {
  const link = document.querySelector("link[rel*='icon']") as HTMLLinkElement;
  if (link) {
    link.href = generateFavicon(primaryColor, primaryHover);
  }
};

export const useThemeStore = create<ThemeStore>()(
  persist(
    (set) => ({
      currentTheme: 'emerald',
      themeConfig: defaultTheme,

      setTheme: (themeName: ThemeName) => {
        const config = themeConfigs[themeName];
        if (!config) return;

        // 更新 DOM 属性，触发 CSS 变量切换
        document.documentElement.setAttribute('data-theme', themeName);

        // 如果是深色主题，添加 class
        if (config.isDark) {
          document.documentElement.classList.add('dark-theme');
        } else {
          document.documentElement.classList.remove('dark-theme');
        }

        // 更新 favicon
        updateFavicon(config.colors.primary, config.colors.primaryHover);

        // 更新状态
        set({ currentTheme: themeName, themeConfig: config });
      },
    }),
    {
      name: 'colink-theme-storage', // localStorage key
      partialize: (state) => ({ currentTheme: state.currentTheme }), // 只持久化主题名称
      onRehydrateStorage: () => (state) => {
        // 页面加载时从 localStorage 恢复主题
        if (state?.currentTheme) {
          const config = themeConfigs[state.currentTheme];
          if (config) {
            document.documentElement.setAttribute('data-theme', state.currentTheme);
            if (config.isDark) {
              document.documentElement.classList.add('dark-theme');
            }
            // 更新 favicon
            updateFavicon(config.colors.primary, config.colors.primaryHover);
            // 更新状态中的 themeConfig
            state.themeConfig = config;
          }
        }
      },
    }
  )
);

export default useThemeStore;