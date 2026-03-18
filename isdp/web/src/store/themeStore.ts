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
 * 生成带主题色的 favicon SVG
 */
const generateFavicon = (primaryColor: string, primaryHover: string): string => {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" fill="none">
  <defs>
    <linearGradient id="bgGrad" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" style="stop-color:${primaryColor}"/>
      <stop offset="100%" style="stop-color:${primaryHover}"/>
    </linearGradient>
    <linearGradient id="flameGrad" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" style="stop-color:#fbbf24"/>
      <stop offset="50%" style="stop-color:#f59e0b"/>
      <stop offset="100%" style="stop-color:#ef4444"/>
    </linearGradient>
  </defs>
  <rect x="2" y="2" width="28" height="28" rx="6" fill="url(#bgGrad)"/>
  <path d="M16 5C16 5 11 9 11 15C11 17 11.5 19 12 20L13 22H19L20 20C20.5 19 21 17 21 15C21 9 16 5 16 5Z" fill="white"/>
  <circle cx="16" cy="12" r="2" fill="${primaryColor}"/>
  <path d="M11 18L9 22L11 23L12.5 20.5Z" fill="#e5e7eb"/>
  <path d="M21 18L23 22L21 23L19.5 20.5Z" fill="#e5e7eb"/>
  <path d="M13.5 22L14.5 27L16 25L17.5 27L18.5 22Z" fill="url(#flameGrad)"/>
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
      name: 'isdp-theme-storage', // localStorage key
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