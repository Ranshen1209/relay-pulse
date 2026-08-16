// @vitest-environment jsdom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { useUrlState } from './useUrlState';

let container: HTMLDivElement;
let root: Root;

function UrlStateProbe() {
  const [state] = useUrlState();
  return <div data-testid="url-state" data-view-mode={state.viewMode} />;
}

async function renderAt(url: string) {
  await act(async () => {
    root.render(
      <MemoryRouter initialEntries={[url]}>
        <UrlStateProbe />
      </MemoryRouter>,
    );
  });
}

beforeEach(() => {
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: false });
});

describe('useUrlState view mode', () => {
  it('defaults to the table list', async () => {
    await renderAt('/');

    expect(container.querySelector('[data-testid="url-state"]')?.getAttribute('data-view-mode')).toBe('table');
  });

  it('respects an explicit table view in the URL', async () => {
    await renderAt('/?view=table');

    expect(container.querySelector('[data-testid="url-state"]')?.getAttribute('data-view-mode')).toBe('table');
  });
});
