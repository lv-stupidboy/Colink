/**
 * 主题类型定义
 */

export type ThemeName = 'emerald' | 'blue' | 'purple' | 'pink' | 'dark' | 'cyan';

export interface ThemeColors {
  // 主色系
  primary: string;
  primaryHover: string;
  primaryActive: string;
  primaryBg: string;
  primaryBgHover: string;
  primaryBorder: string;
  primaryLight: string;

  // 背景色
  bgBase: string;
  bgContainer: string;
  bgElevated: string;
  bgSidebar: string;

  // 文本色
  textPrimary: string;
  textSecondary: string;

  // 边框色
  borderColor: string;
  borderLight: string;

  // 阴影
  shadowSm: string;
  shadowMd: string;

  // 透明度变量
  primaryOpacity5: string;
  primaryOpacity10: string;
  primaryOpacity15: string;
  primaryOpacity20: string;
  primaryOpacity35: string;
  primaryOpacity40: string;
  primaryOpacity50: string;
}

export interface ThemeConfig {
  name: ThemeName;
  label: string;
  colors: ThemeColors;
  isDark: boolean;
}

export interface ThemeState {
  currentTheme: ThemeName;
  themeConfig: ThemeConfig;
}

export interface ThemeActions {
  setTheme: (themeName: ThemeName) => void;
}