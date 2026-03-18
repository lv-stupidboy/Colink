/**
 * 主题配色配置
 * 支持6种主题：翡翠绿、深海蓝、优雅紫、樱花粉、深邃黑、科技蓝
 */

import type { ThemeConfig, ThemeColors, ThemeName } from './types';

// 翡翠绿主题 - 当前默认，清新活力
const emeraldColors: ThemeColors = {
  primary: '#10b981',
  primaryHover: '#059669',
  primaryActive: '#047857',
  primaryBg: '#d1fae5',
  primaryBgHover: '#a7f3d0',
  primaryBorder: '#34d399',
  primaryLight: '#ecfdf5',

  bgBase: '#f0fdf4',
  bgContainer: '#ffffff',
  bgElevated: '#ffffff',
  bgSidebar: '#fafafa',

  textPrimary: '#047857',
  textSecondary: '#6b7280',

  borderColor: '#e5e7eb',
  borderLight: '#f0f0f0',

  shadowSm: '0 1px 2px rgba(0, 0, 0, 0.05)',
  shadowMd: '0 4px 6px rgba(0, 0, 0, 0.07)',

  primaryOpacity5: 'rgba(16, 185, 129, 0.05)',
  primaryOpacity10: 'rgba(16, 185, 129, 0.1)',
  primaryOpacity15: 'rgba(16, 185, 129, 0.15)',
  primaryOpacity20: 'rgba(16, 185, 129, 0.2)',
  primaryOpacity35: 'rgba(16, 185, 129, 0.35)',
  primaryOpacity40: 'rgba(16, 185, 129, 0.4)',
  primaryOpacity50: 'rgba(16, 185, 129, 0.5)',
};

// 深海蓝主题 - 专业稳重，企业风格
const blueColors: ThemeColors = {
  primary: '#3b82f6',
  primaryHover: '#2563eb',
  primaryActive: '#1d4ed8',
  primaryBg: '#dbeafe',
  primaryBgHover: '#bfdbfe',
  primaryBorder: '#60a5fa',
  primaryLight: '#eff6ff',

  bgBase: '#f8fafc',
  bgContainer: '#ffffff',
  bgElevated: '#ffffff',
  bgSidebar: '#f1f5f9',

  textPrimary: '#1e40af',
  textSecondary: '#64748b',

  borderColor: '#e2e8f0',
  borderLight: '#f1f5f9',

  shadowSm: '0 1px 2px rgba(0, 0, 0, 0.05)',
  shadowMd: '0 4px 6px rgba(0, 0, 0, 0.07)',

  primaryOpacity5: 'rgba(59, 130, 246, 0.05)',
  primaryOpacity10: 'rgba(59, 130, 246, 0.1)',
  primaryOpacity15: 'rgba(59, 130, 246, 0.15)',
  primaryOpacity20: 'rgba(59, 130, 246, 0.2)',
  primaryOpacity35: 'rgba(59, 130, 246, 0.35)',
  primaryOpacity40: 'rgba(59, 130, 246, 0.4)',
  primaryOpacity50: 'rgba(59, 130, 246, 0.5)',
};

// 优雅紫主题 - 高端优雅，创意风格
const purpleColors: ThemeColors = {
  primary: '#8b5cf6',
  primaryHover: '#7c3aed',
  primaryActive: '#6d28d9',
  primaryBg: '#ede9fe',
  primaryBgHover: '#ddd6fe',
  primaryBorder: '#a78bfa',
  primaryLight: '#f5f3ff',

  bgBase: '#f5f3ff',
  bgContainer: '#ffffff',
  bgElevated: '#ffffff',
  bgSidebar: '#faf5ff',

  textPrimary: '#5b21b6',
  textSecondary: '#7c3aed',

  borderColor: '#e5e7eb',
  borderLight: '#f3e8ff',

  shadowSm: '0 1px 2px rgba(0, 0, 0, 0.05)',
  shadowMd: '0 4px 6px rgba(0, 0, 0, 0.07)',

  primaryOpacity5: 'rgba(139, 92, 246, 0.05)',
  primaryOpacity10: 'rgba(139, 92, 246, 0.1)',
  primaryOpacity15: 'rgba(139, 92, 246, 0.15)',
  primaryOpacity20: 'rgba(139, 92, 246, 0.2)',
  primaryOpacity35: 'rgba(139, 92, 246, 0.35)',
  primaryOpacity40: 'rgba(139, 92, 246, 0.4)',
  primaryOpacity50: 'rgba(139, 92, 246, 0.5)',
};

// 樱花粉主题 - 柔美浪漫
const pinkColors: ThemeColors = {
  primary: '#ec4899',
  primaryHover: '#db2777',
  primaryActive: '#be185d',
  primaryBg: '#fce7f3',
  primaryBgHover: '#fbcfe8',
  primaryBorder: '#f472b6',
  primaryLight: '#fdf2f8',

  bgBase: '#fdf2f8',
  bgContainer: '#ffffff',
  bgElevated: '#ffffff',
  bgSidebar: '#fdf2f8',

  textPrimary: '#9d174d',
  textSecondary: '#be185d',

  borderColor: '#fce7f3',
  borderLight: '#fdf2f8',

  shadowSm: '0 1px 2px rgba(0, 0, 0, 0.05)',
  shadowMd: '0 4px 6px rgba(0, 0, 0, 0.07)',

  primaryOpacity5: 'rgba(236, 72, 153, 0.05)',
  primaryOpacity10: 'rgba(236, 72, 153, 0.1)',
  primaryOpacity15: 'rgba(236, 72, 153, 0.15)',
  primaryOpacity20: 'rgba(236, 72, 153, 0.2)',
  primaryOpacity35: 'rgba(236, 72, 153, 0.35)',
  primaryOpacity40: 'rgba(236, 72, 153, 0.4)',
  primaryOpacity50: 'rgba(236, 72, 153, 0.5)',
};

// 深邃黑主题 - 深色模式，护眼
const darkColors: ThemeColors = {
  primary: '#10b981',
  primaryHover: '#34d399',
  primaryActive: '#059669',
  primaryBg: '#064e3b',
  primaryBgHover: '#065f46',
  primaryBorder: '#10b981',
  primaryLight: '#022c22',

  bgBase: '#0f172a',
  bgContainer: '#1e293b',
  bgElevated: '#334155',
  bgSidebar: '#1e293b',

  textPrimary: '#e2e8f0',
  textSecondary: '#94a3b8',

  borderColor: '#334155',
  borderLight: '#1e293b',

  shadowSm: '0 1px 2px rgba(0, 0, 0, 0.3)',
  shadowMd: '0 4px 6px rgba(0, 0, 0, 0.4)',

  primaryOpacity5: 'rgba(16, 185, 129, 0.1)',
  primaryOpacity10: 'rgba(16, 185, 129, 0.2)',
  primaryOpacity15: 'rgba(16, 185, 129, 0.3)',
  primaryOpacity20: 'rgba(16, 185, 129, 0.4)',
  primaryOpacity35: 'rgba(16, 185, 129, 0.5)',
  primaryOpacity40: 'rgba(16, 185, 129, 0.6)',
  primaryOpacity50: 'rgba(16, 185, 129, 0.7)',
};

// 科技蓝主题 - 科技感，现代风格
const cyanColors: ThemeColors = {
  primary: '#0ea5e9',
  primaryHover: '#0284c7',
  primaryActive: '#0369a1',
  primaryBg: '#e0f2fe',
  primaryBgHover: '#bae6fd',
  primaryBorder: '#38bdf8',
  primaryLight: '#f0f9ff',

  bgBase: '#f0f9ff',
  bgContainer: '#ffffff',
  bgElevated: '#ffffff',
  bgSidebar: '#f0f9ff',

  textPrimary: '#075985',
  textSecondary: '#64748b',

  borderColor: '#e0f2fe',
  borderLight: '#f0f9ff',

  shadowSm: '0 1px 2px rgba(0, 0, 0, 0.05)',
  shadowMd: '0 4px 6px rgba(0, 0, 0, 0.07)',

  primaryOpacity5: 'rgba(14, 165, 233, 0.05)',
  primaryOpacity10: 'rgba(14, 165, 233, 0.1)',
  primaryOpacity15: 'rgba(14, 165, 233, 0.15)',
  primaryOpacity20: 'rgba(14, 165, 233, 0.2)',
  primaryOpacity35: 'rgba(14, 165, 233, 0.35)',
  primaryOpacity40: 'rgba(14, 165, 233, 0.4)',
  primaryOpacity50: 'rgba(14, 165, 233, 0.5)',
};

// 主题配置映射
export const themeConfigs: Record<ThemeName, ThemeConfig> = {
  emerald: {
    name: 'emerald',
    label: '翡翠绿',
    colors: emeraldColors,
    isDark: false,
  },
  blue: {
    name: 'blue',
    label: '深海蓝',
    colors: blueColors,
    isDark: false,
  },
  purple: {
    name: 'purple',
    label: '优雅紫',
    colors: purpleColors,
    isDark: false,
  },
  pink: {
    name: 'pink',
    label: '樱花粉',
    colors: pinkColors,
    isDark: false,
  },
  dark: {
    name: 'dark',
    label: '深邃黑',
    colors: darkColors,
    isDark: true,
  },
  cyan: {
    name: 'cyan',
    label: '科技蓝',
    colors: cyanColors,
    isDark: false,
  },
};

// 默认主题
export const defaultTheme = themeConfigs.emerald;

// 主题列表（用于渲染选择器）
export const themeList: Array<{ name: ThemeName; label: string; color: string }> = [
  { name: 'emerald', label: '翡翠绿', color: '#10b981' },
  { name: 'blue', label: '深海蓝', color: '#3b82f6' },
  { name: 'purple', label: '优雅紫', color: '#8b5cf6' },
  { name: 'pink', label: '樱花粉', color: '#ec4899' },
  { name: 'dark', label: '深邃黑', color: '#18181b' },
  { name: 'cyan', label: '科技蓝', color: '#0ea5e9' },
];