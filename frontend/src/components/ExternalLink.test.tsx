// @vitest-environment jsdom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import i18n from '../i18n';
import { ExternalLink } from './ExternalLink';

let container: HTMLDivElement;
let root: Root;

beforeEach(async () => {
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
  sessionStorage.clear();
  await i18n.changeLanguage('zh-CN');
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: false });
});

describe('ExternalLink', () => {
  it('uses a dedicated target name in the confirmation modal', async () => {
    await act(async () => {
      root.render(
        <ExternalLink
          href="https://ai1.sakrylle.com"
          targetName="Sakrylle"
          requireConfirm
        >
          DeepSeek
        </ExternalLink>,
      );
    });

    const link = container.querySelector('a');
    expect(link?.textContent).toContain('DeepSeek');

    await act(async () => {
      link?.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
    });

    const dialog = document.querySelector('[role="dialog"]');
    expect(dialog?.querySelector('strong')?.textContent).toBe('Sakrylle');
    expect(dialog?.textContent).toContain('ai1.sakrylle.com');
  });
});
