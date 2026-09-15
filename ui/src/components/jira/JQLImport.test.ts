import { describe, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
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
