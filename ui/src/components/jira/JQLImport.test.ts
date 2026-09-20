import { describe, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { tick } from 'svelte';
import { setLocale } from '../../i18n/i18n-svelte';
import { loadLocale } from '../../i18n/i18n-util.sync';
import JQLImport from './JQLImport.svelte';
import CreatePokerGame from '../poker/CreatePokerGame.svelte';
import PokerStories from '../poker/PokerStories.svelte';

vi.mock('../../stores', async () => {
  const { writable } = await import('svelte/store');
  return { user: writable({ id: 'user', subscribed: true }) };
});

const notifications = {
  success: vi.fn(),
  danger: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
  show: vi.fn(),
  removeToast: vi.fn(),
};
const issue = (key: string) => ({
  key,
  fields: { summary: `Story ${key}`, issuetype: { name: 'Story' }, priority: { name: 'Medium' } },
});
const response = (data: unknown) => new Response(JSON.stringify({ data }));
const instances = [
  { id: 'one', host: 'https://jira.example.com/jira/' },
  { id: 'two', host: 'https://other.example.com' },
];

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

function setup(issues = [issue('TEST-1')], existingStories: Array<{ referenceId: string; link: string }> = []) {
  const handleImport = vi.fn();
  const xfetch = vi.fn(async (url: string) => response(url.endsWith('jira-instances') ? instances : { issues }));
  const view = render(JQLImport, { notifications, xfetch, handleImport, existingStories });
  return { handleImport, xfetch, view };
}

async function search() {
  await page.getByRole('searchbox').fill('order by created DESC');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
}

describe('Jira search feedback', () => {
  it('shows a busy button and prevents duplicate submissions until the results arrive', async () => {
    const pending = deferred<Response>();
    const xfetch = vi.fn(async (url: string) =>
      url.endsWith('jira-instances') ? response(instances) : pending.promise,
    );
    render(JQLImport, { notifications, xfetch });
    await page.getByRole('combobox').selectOptions('0');
    await search();
    const busy = page.getByRole('button', { name: 'Searching...', exact: true });
    await expect.element(busy).toBeDisabled();
    await expect.element(busy).toHaveAttribute('aria-busy', 'true');
    const form = document.querySelector<HTMLFormElement>('form.search-form');
    expect(form).not.toBeNull();
    form!.requestSubmit();
    form!.requestSubmit();
    expect(xfetch.mock.calls.filter(([url]) => url.endsWith('jql-story-search'))).toHaveLength(1);

    pending.resolve(response({ issues: [issue('TEST-1')] }));
    await expect.element(page.getByText('[TEST-1] Story TEST-1')).toBeVisible();
    await expect.element(page.getByRole('button', { name: 'Search', exact: true })).toBeEnabled();
  });

  it.each([
    { kind: 'network failure', error: () => new Error('Network unavailable') },
    { kind: 'Jira validation error', error: () => [400, new Response(JSON.stringify({ error: 'Invalid JQL' }))] },
    { kind: 'non-JSON error response', error: () => [502, new Response('<html>Bad Gateway</html>')] },
  ])('restores the search button after a $kind and allows a retry', async ({ error }) => {
    const pending = deferred<Response>();
    const xfetch = vi.fn(async (url: string) => {
      if (url.endsWith('jira-instances')) return response(instances);
      return pending.promise;
    });
    render(JQLImport, { notifications, xfetch });
    await page.getByRole('combobox').selectOptions('0');
    await search();
    await expect.element(page.getByRole('button', { name: 'Searching...', exact: true })).toBeDisabled();
    pending.reject(error());
    await expect.element(page.getByRole('button', { name: 'Search', exact: true })).toBeEnabled();

    xfetch.mockResolvedValueOnce(response({ issues: [issue('RETRY-1')] }));
    await search();
    await expect.element(page.getByText('[RETRY-1] Story RETRY-1')).toBeVisible();
    expect(xfetch.mock.calls.filter(([url]) => url.endsWith('jql-story-search'))).toHaveLength(2);
  });

  it('clears the previous query error as soon as a retry starts', async () => {
    const retry = deferred<Response>();
    const xfetch = vi
      .fn()
      .mockResolvedValueOnce(response(instances))
      .mockRejectedValueOnce([400, new Response(JSON.stringify({ error: 'Invalid JQL' }))])
      .mockReturnValueOnce(retry.promise);
    render(JQLImport, { notifications, xfetch });
    await page.getByRole('combobox').selectOptions('0');
    await search();
    await expect.element(page.getByText('Jira JQL Search Error: Invalid JQL')).toBeVisible();
    await search();
    await expect.element(page.getByRole('button', { name: 'Searching...', exact: true })).toBeDisabled();
    await expect.element(page.getByText('Jira JQL Search Error: Invalid JQL')).not.toBeInTheDocument();
    retry.resolve(response({ issues: [] }));
    await expect.element(page.getByRole('button', { name: 'Search', exact: true })).toBeEnabled();
  });

  it('keeps the new instance search busy when the previous instance returns results', async () => {
    const oldSearch = deferred<Response>();
    const newSearch = deferred<Response>();
    const oldResponse = response({ issues: [issue('OLD-1')] });
    const readOldResponse = vi.spyOn(oldResponse, 'json').mockResolvedValue({ data: { issues: [issue('OLD-1')] } });
    render(JQLImport, {
      notifications,
      xfetch: async (url: string) => {
        if (url.endsWith('jira-instances')) return response(instances);
        return url.includes('/one/') ? oldSearch.promise : newSearch.promise;
      },
    });
    await page.getByRole('combobox').selectOptions('0');
    await search();
    await page.getByRole('combobox').selectOptions('1');
    await expect.element(page.getByRole('button', { name: 'Search', exact: true })).toBeEnabled();
    await search();
    oldSearch.resolve(oldResponse);
    await expect.poll(() => readOldResponse.mock.calls.length).toBe(1);
    await tick();
    await expect.element(page.getByRole('button', { name: 'Searching...', exact: true })).toBeDisabled();
    await expect.element(page.getByText('[OLD-1] Story OLD-1')).not.toBeInTheDocument();

    newSearch.resolve(response({ issues: [issue('NEW-1')] }));
    await expect.element(page.getByText('[NEW-1] Story NEW-1')).toBeVisible();
    await expect.element(page.getByRole('button', { name: 'Search', exact: true })).toBeEnabled();
  });

  it('ignores an old error body that finishes parsing during a new instance search', async () => {
    const oldSearch = deferred<Response>();
    const oldErrorBody = deferred<{ error: string }>();
    const newSearch = deferred<Response>();
    const oldErrorResponse = new Response();
    const readOldError = vi.spyOn(oldErrorResponse, 'json').mockReturnValue(oldErrorBody.promise);
    render(JQLImport, {
      notifications,
      xfetch: async (url: string) => {
        if (url.endsWith('jira-instances')) return response(instances);
        return url.includes('/one/') ? oldSearch.promise : newSearch.promise;
      },
    });
    await page.getByRole('combobox').selectOptions('0');
    await search();
    oldSearch.reject([400, oldErrorResponse]);
    await expect.poll(() => readOldError.mock.calls.length).toBe(1);
    await page.getByRole('combobox').selectOptions('1');
    await search();
    oldErrorBody.resolve({ error: 'Previous instance failed' });
    await oldErrorBody.promise;
    await tick();
    await expect.element(page.getByRole('button', { name: 'Searching...', exact: true })).toBeDisabled();
    await expect.element(page.getByText('Jira JQL Search Error: Previous instance failed')).not.toBeInTheDocument();

    newSearch.resolve(response({ issues: [issue('NEW-1')] }));
    await expect.element(page.getByText('[NEW-1] Story NEW-1')).toBeVisible();
    await expect.element(page.getByRole('button', { name: 'Search', exact: true })).toBeEnabled();
  });

  it('shows Chinese search feedback with visible JQL examples', async () => {
    loadLocale('zh');
    setLocale('zh');
    const pending = deferred<Response>();
    try {
      render(JQLImport, {
        notifications,
        xfetch: async (url: string) => (url.endsWith('jira-instances') ? response(instances) : pending.promise),
      });
      await page.getByRole('combobox').selectOptions('0');
      await expect.element(page.getByText('sprint = "Sprint 42"', { exact: true })).toBeVisible();
      await page.getByRole('searchbox').fill('sprint in openSprints()');
      await page.getByRole('button', { name: '搜索', exact: true }).click();
      await expect.element(page.getByRole('button', { name: '搜索中…', exact: true })).toBeDisabled();
      pending.resolve(response({ issues: [issue('TEST-1')] }));
      await expect.element(page.getByRole('button', { name: '导入', exact: true })).toBeVisible();
      await expect.element(page.getByRole('button', { name: '搜索', exact: true })).toBeEnabled();
    } finally {
      setLocale('en');
    }
  });
});

describe('Jira import deduplication', () => {
  it('discards an old search response after switching Jira instances', async () => {
    let finish: (response: Response) => void = () => {};
    const pending = new Promise<Response>(resolve => {
      finish = resolve;
    });
    const handleImport = vi.fn();
    render(JQLImport, {
      notifications,
      handleImport,
      xfetch: async (url: string) => (url.endsWith('jira-instances') ? response(instances) : pending),
    });
    await page.getByRole('combobox').selectOptions('0');
    await search();
    await page.getByRole('combobox').selectOptions('1');
    finish(response({ issues: [issue('OLD-1')] }));
    await expect.element(page.getByRole('button', { name: 'Import', exact: true })).not.toBeInTheDocument();
    expect(handleImport).not.toHaveBeenCalled();
  });

  it('updates filtering when another participant imports a story', async () => {
    const { view, handleImport } = setup();
    await page.getByRole('combobox').selectOptions('0');
    await search();
    await expect.element(page.getByRole('button', { name: 'Import', exact: true })).toBeVisible();
    await view.rerender({
      existingStories: [{ referenceId: 'TEST-1', link: 'https://jira.example.com/jira/browse/TEST-1' }],
    });
    await expect.element(page.getByText('All stories have been imported!')).toBeVisible();
    expect(handleImport).not.toHaveBeenCalled();
  });

  it('keeps creation draft stories excluded after closing and reopening import', async () => {
    const xfetch = vi.fn(async (url: string) => {
      if (url.endsWith('jira-instances')) return response(instances);
      if (url.endsWith('jql-story-search')) return response({ issues: [issue('TEST-1')] });
      if (url.endsWith('/estimation-scales/public'))
        return response([{ id: 'default', name: 'Default', defaultScale: true, values: ['1', '2'] }]);
      return response([]);
    });
    render(CreatePokerGame, { notifications, router: { route: vi.fn() }, xfetch });
    await page.getByRole('button', { name: 'Import Stories', exact: true }).click();
    await page.getByRole('dialog').getByRole('combobox').selectOptions('0');
    await search();
    await page.getByRole('button', { name: 'Import', exact: true }).click();
    await page.getByRole('button', { name: 'Close modal' }).click();
    await expect.element(page.getByPlaceholder('Enter a story name')).toHaveValue('Story TEST-1');
    await page.getByRole('button', { name: 'Import Stories', exact: true }).click();
    await page.getByRole('dialog').getByRole('combobox').selectOptions('0');
    await search();
    await expect.element(page.getByText('All stories have been imported!')).toBeVisible();
  });

  it('passes completed meeting stories to the importer even while the list shows unpointed stories', async () => {
    const sendSocketEvent = vi.fn();
    render(PokerStories, {
      notifications,
      sendSocketEvent,
      isFacilitator: true,
      gameId: 'game',
      plans: [
        {
          id: 'saved',
          name: 'Saved result',
          type: 'Story',
          referenceId: 'TEST-1',
          link: 'https://jira.example.com/jira/browse/TEST-1',
          points: '5',
          priority: 4,
          active: false,
          votes: [],
        },
      ],
      xfetch: async (url: string) =>
        response(url.endsWith('jira-instances') ? instances : { issues: [issue('TEST-1')] }),
    });
    await page.getByRole('button', { name: 'Import Stories', exact: true }).click();
    await page.getByRole('dialog').getByRole('combobox').selectOptions('0');
    await search();
    await expect.element(page.getByText('All stories have been imported!')).toBeVisible();
    expect(sendSocketEvent).not.toHaveBeenCalled();
  });

  it('excludes existing stories including completed stories after opening the importer', async () => {
    const { handleImport } = setup(
      [issue('TEST-1'), issue('TEST-2')],
      [{ referenceId: ' test-1 ', link: 'https://JIRA.example.com/jira/browse/TEST-1/?source=board#details' }],
    );
    await page.getByRole('combobox').selectOptions('0');
    await search();
    await expect.element(page.getByText('[TEST-2] Story TEST-2')).toBeVisible();
    await page.getByRole('button', { name: 'Import All', exact: true }).click();
    expect(handleImport.mock.calls.map(([story]) => story.referenceId)).toEqual(['TEST-2']);
  });

  it('does not reimport after searching again in the same modal', async () => {
    const { handleImport } = setup();
    await page.getByRole('combobox').selectOptions('0');
    await search();
    await page.getByRole('button', { name: 'Import', exact: true }).click();
    await search();
    await expect.element(page.getByText('All stories have been imported!')).toBeVisible();
    expect(handleImport).toHaveBeenCalledTimes(1);
  });

  it('imports a duplicated search result only once when importing all', async () => {
    const { handleImport } = setup([issue('TEST-1'), issue('TEST-1'), issue('TEST-2')]);
    await page.getByRole('combobox').selectOptions('0');
    await search();
    await page.getByRole('button', { name: 'Import All', exact: true }).click();
    expect(handleImport.mock.calls.map(([story]) => story.referenceId)).toEqual(['TEST-1', 'TEST-2']);
  });

  it('keeps the same issue key from different Jira instances distinct', async () => {
    const { handleImport } = setup();
    await page.getByRole('combobox').selectOptions('0');
    await search();
    await page.getByRole('button', { name: 'Import', exact: true }).click();
    await page.getByRole('combobox').selectOptions('1');
    await search();
    await page.getByRole('button', { name: 'Import', exact: true }).click();
    expect(handleImport.mock.calls.map(([story]) => story.link)).toEqual([
      'https://jira.example.com/jira/browse/TEST-1',
      'https://other.example.com/browse/TEST-1',
    ]);
  });
});
