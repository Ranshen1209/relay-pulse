// @vitest-environment jsdom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useTheme, type ThemeId } from './useTheme';

type ThemeChangeListener = (event: MediaQueryListEvent) => void;

let container: HTMLDivElement;
let root: Root;
let systemPrefersDark = false;
let listeners: Set<ThemeChangeListener>;

function ThemeProbe({ forceTheme }: { forceTheme?: ThemeId }) {
  const { theme } = useTheme(forceTheme);
  return <div data-testid="theme" data-theme={theme} />;
}

function setSystemTheme(isDark: boolean) {
  systemPrefersDark = isDark;
  const event = { matches: isDark } as MediaQueryListEvent;
  listeners.forEach((listener) => listener(event));
}

beforeEach(() => {
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
  systemPrefersDark = false;
  listeners = new Set();
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);

  vi.stubGlobal('matchMedia', vi.fn((query: string) => ({
    get matches() {
      return systemPrefersDark;
    },
    media: query,
    onchange: null,
    addEventListener: (type: string, listener: ThemeChangeListener) => {
      if (type === 'change') listeners.add(listener);
    },
    removeEventListener: (type: string, listener: ThemeChangeListener) => {
      if (type === 'change') listeners.delete(listener);
    },
    dispatchEvent: () => true,
  })));
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  document.documentElement.removeAttribute('data-theme');
  document.documentElement.style.removeProperty('color-scheme');
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: false });
  vi.unstubAllGlobals();
});

describe('useTheme', () => {
  it('applies the browser theme and reacts to preference changes', async () => {
    await act(async () => root.render(<ThemeProbe />));

    expect(document.documentElement.dataset.theme).toBe('light-cool');
    expect(document.documentElement.style.colorScheme).toBe('light');

    await act(async () => setSystemTheme(true));

    expect(document.documentElement.dataset.theme).toBe('default-dark');
    expect(document.documentElement.style.colorScheme).toBe('dark');
  });

  it('allows screenshot mode to force the dark theme', async () => {
    await act(async () => root.render(<ThemeProbe forceTheme="default-dark" />));

    expect(document.documentElement.dataset.theme).toBe('default-dark');
    expect(document.documentElement.style.colorScheme).toBe('dark');
  });
});
