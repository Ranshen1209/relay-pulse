/**
 * 主题管理 Hook
 *
 * 跟随浏览器的 prefers-color-scheme，并更新 DOM 主题属性。
 */

import { useEffect, useState } from 'react';

export type ThemeId = 'default-dark' | 'light-cool';

export interface Theme {
  id: ThemeId;
  nameKey: string; // i18n key
  isDark: boolean;
}

export const THEMES: Theme[] = [
  { id: 'default-dark', nameKey: 'theme.defaultDark', isDark: true },
  { id: 'light-cool', nameKey: 'theme.lightCool', isDark: false },
];

const DEFAULT_THEME: ThemeId = 'default-dark';
const DARK_MODE_QUERY = '(prefers-color-scheme: dark)';

function getSystemTheme(): ThemeId {
  if (typeof window === 'undefined') return DEFAULT_THEME;
  return window.matchMedia(DARK_MODE_QUERY).matches ? 'default-dark' : 'light-cool';
}

/**
 * 应用主题到 DOM
 */
function applyTheme(themeId: ThemeId): void {
  const root = document.documentElement;
  const theme = THEMES.find((t) => t.id === themeId);

  // 设置 data-theme 属性
  root.setAttribute('data-theme', themeId);

  // 设置 color-scheme（影响浏览器原生控件）
  root.style.colorScheme = theme?.isDark ? 'dark' : 'light';
}

/**
 * 主题管理 Hook
 */
export function useTheme(forceTheme?: ThemeId) {
  const [systemTheme, setSystemTheme] = useState<ThemeId>(getSystemTheme);
  const theme = forceTheme ?? systemTheme;

  useEffect(() => {
    const mediaQuery = window.matchMedia(DARK_MODE_QUERY);
    const syncTheme = () => setSystemTheme(mediaQuery.matches ? 'default-dark' : 'light-cool');

    mediaQuery.addEventListener('change', syncTheme);
    return () => mediaQuery.removeEventListener('change', syncTheme);
  }, []);

  useEffect(() => {
    applyTheme(theme);
    window.dispatchEvent(new CustomEvent('theme-change', { detail: theme }));
  }, [theme]);

  const currentTheme = THEMES.find((t) => t.id === theme) || THEMES[0];

  return {
    theme,
    currentTheme,
    isDark: currentTheme.isDark,
  };
}

/**
 * 获取当前主题 ID（非 Hook，用于工具函数）
 */
export function getCurrentThemeId(): ThemeId {
  if (typeof window === 'undefined') return DEFAULT_THEME;
  return (document.documentElement.getAttribute('data-theme') as ThemeId) || DEFAULT_THEME;
}

/**
 * 检查当前是否为暗色主题
 */
export function isDarkTheme(): boolean {
  const themeId = getCurrentThemeId();
  const theme = THEMES.find((t) => t.id === themeId);
  return theme?.isDark ?? true;
}
