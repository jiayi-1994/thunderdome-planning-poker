import { describe, expect, it, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { tick } from 'svelte';
import JQLImport from './JQLImport.svelte';
import type { ApiClient } from '../../types/apiclient';

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
const instances = [
  { id: 'one', host: 'https://jira.example.com' },
  { id: 'two', host: 'https://other.example.com' },
];
const issueTypes = [
  { id: 'story', name: 'Story', subtask: false },
  { id: 'bug', name: 'Bug', subtask: false },
];
const sprint = { id: 577, name: 'ZStack Zaku Sprint41', state: 'active', boardName: 'Edge board' };
const upcomingSprint = { id: 578, name: 'ZStack Zaku Sprint42', state: 'future', boardName: 'Edge board' };
const response = (data: unknown) => new Response(JSON.stringify({ data }));
const instanceSelect = () => page.getByRole('combobox', { name: 'Select Jira Instance to import from' });
const typeSelect = () => page.getByRole('combobox', { name: 'Issue type', exact: true });
const sprintSelect = () => page.getByRole('combobox', { name: 'Sprint', exact: true });
const submit = () => page.getByRole('button', { name: 'Search', exact: true });

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(resolvePromise => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

function defaultResponse(url: string) {
  if (url.endsWith('jira-instances')) return response(instances);
  if (url.endsWith('/issue-types')) return response(issueTypes);
  if (url.includes('/sprints?')) return response([sprint, upcomingSprint]);
  return response({ issues: [] });
}

describe('Jira basic filters', () => {
  it('defaults to the real Story type and submits the selected sprint ID in JQL', async () => {
    const xfetch = vi.fn<ApiClient>(async url => defaultResponse(url));
    const handleImport = vi.fn();
    render(JQLImport, { notifications, xfetch, handleImport });
    await instanceSelect().selectOptions('0');
    await expect.element(typeSelect()).toHaveValue('story');
    await expect.element(sprintSelect()).toHaveTextContent('ZStack Zaku Sprint41 · Active · Edge board · #577');
    await sprintSelect().selectOptions('577');
    await expect.element(page.getByText('issuetype = "Story" AND Sprint = 577', { exact: true })).toBeVisible();
    await submit().click();
    expect(xfetch).toHaveBeenCalledWith('/api/users/user/jira-instances/one/jql-story-search', {
      body: { jql: 'issuetype = "Story" AND Sprint = 577', startAt: 0, maxResults: 100 },
    });
    expect(handleImport).not.toHaveBeenCalled();
  });

  it('escapes issue type names and leaves all types selected when Story is unavailable', async () => {
    const typeName = 'Product "request" \\ draft';
    const xfetch = vi.fn<ApiClient>(async url =>
      url.endsWith('/issue-types')
        ? response([{ id: 'custom', name: typeName, subtask: false }])
        : defaultResponse(url),
    );
    render(JQLImport, { notifications, xfetch });
    await instanceSelect().selectOptions('0');
    await expect.element(typeSelect()).toBeEnabled();
    await expect.element(typeSelect()).toHaveValue('');
    await typeSelect().selectOptions('custom');
    await submit().click();
    expect(xfetch).toHaveBeenCalledWith('/api/users/user/jira-instances/one/jql-story-search', {
      body: { jql: 'issuetype = "Product \\"request\\" \\\\ draft"', startAt: 0, maxResults: 100 },
    });
  });

  it('encodes sprint searches, preserves a selected sprint outside new results, and clears it completely', async () => {
    const xfetch = vi.fn<ApiClient>(async url =>
      url.includes('/sprints?query=Zaku') ? response([upcomingSprint]) : defaultResponse(url),
    );
    render(JQLImport, { notifications, xfetch });
    await instanceSelect().selectOptions('0');
    await sprintSelect().selectOptions('577');
    const query = 'Zaku & Edge / 41';
    await page.getByRole('textbox', { name: 'Find a sprint by name' }).fill(query);
    await page.getByRole('button', { name: 'Find sprints', exact: true }).click();
    await expect.element(sprintSelect()).toBeEnabled();
    expect(xfetch).toHaveBeenCalledWith(
      `/api/users/user/jira-instances/one/sprints?query=${encodeURIComponent(query)}`,
    );
    await expect.element(sprintSelect()).toHaveValue('577');
    await expect.element(sprintSelect()).toHaveTextContent('ZStack Zaku Sprint41');
    await sprintSelect().selectOptions('');
    await expect.element(page.getByText('issuetype = "Story"', { exact: true })).toBeVisible();
    await expect.element(sprintSelect()).not.toHaveTextContent('ZStack Zaku Sprint41');
    await submit().click();
    expect(xfetch).toHaveBeenLastCalledWith('/api/users/user/jira-instances/one/jql-story-search', {
      body: { jql: 'issuetype = "Story"', startAt: 0, maxResults: 100 },
    });
  });

  it('ignores old metadata after switching instances', async () => {
    const oldTypes = deferred<Response>();
    const oldSprints = deferred<Response>();
    const typeResponse = response([{ id: 'old', name: 'Old issue type', subtask: false }]);
    const sprintResponse = response([{ ...sprint, name: 'Old sprint' }]);
    render(JQLImport, {
      notifications,
      xfetch: async url => {
        if (url.includes('/one/issue-types')) return oldTypes.promise;
        if (url.includes('/one/sprints?')) return oldSprints.promise;
        return defaultResponse(url);
      },
    });
    await instanceSelect().selectOptions('0');
    await expect.element(typeSelect()).toBeDisabled();
    await instanceSelect().selectOptions('1');
    await expect.element(typeSelect()).toHaveValue('story');
    oldTypes.resolve(typeResponse);
    oldSprints.resolve(sprintResponse);
    await expect.poll(() => typeResponse.bodyUsed && sprintResponse.bodyUsed).toBe(true);
    await tick();
    await expect.element(typeSelect()).not.toHaveTextContent('Old issue type');
    await expect.element(sprintSelect()).not.toHaveTextContent('Old sprint');
    await expect.element(typeSelect()).toHaveValue('story');
  });

  it('keeps the latest sprint search when an older name search resolves later', async () => {
    const older = deferred<Response>();
    const newer = deferred<Response>();
    const oldResponse = response([{ ...sprint, name: 'Older result' }]);
    render(JQLImport, {
      notifications,
      xfetch: async url => {
        if (url.endsWith('query=older')) return older.promise;
        if (url.endsWith('query=newer')) return newer.promise;
        return defaultResponse(url);
      },
    });
    await instanceSelect().selectOptions('0');
    const nameInput = page.getByRole('textbox', { name: 'Find a sprint by name' });
    await expect.element(sprintSelect()).toBeEnabled();
    await nameInput.fill('older');
    await nameInput.click();
    await userEvent.keyboard('{Enter}');
    await nameInput.fill('newer');
    await nameInput.click();
    await userEvent.keyboard('{Enter}');
    newer.resolve(response([{ ...upcomingSprint, name: 'Newer result' }]));
    await expect.element(sprintSelect()).toHaveTextContent('Newer result');
    older.resolve(oldResponse);
    await expect.poll(() => oldResponse.bodyUsed).toBe(true);
    await tick();
    await expect.element(sprintSelect()).not.toHaveTextContent('Older result');
    await expect.element(sprintSelect()).toHaveTextContent('Newer result');
  });

  it('preserves manually written advanced JQL when metadata arrives and modes are toggled', async () => {
    const types = deferred<Response>();
    const sprints = deferred<Response>();
    const xfetch = vi.fn<ApiClient>(async url => {
      if (url.endsWith('/issue-types')) return types.promise;
      if (url.includes('/sprints?')) return sprints.promise;
      return defaultResponse(url);
    });
    render(JQLImport, { notifications, xfetch });
    await instanceSelect().selectOptions('0');
    await page.getByRole('button', { name: 'Advanced JQL', exact: true }).click();
    const manual = 'project = "DEMO" AND status = "In Progress"';
    await page.getByRole('searchbox').fill(manual);
    types.resolve(response(issueTypes));
    sprints.resolve(response([sprint]));
    await expect.poll(() => document.querySelector<HTMLSelectElement>('#jira-issue-type')?.value).toBe('story');
    await expect.element(page.getByRole('searchbox')).toHaveValue(manual);
    await page.getByRole('button', { name: 'Basic filters', exact: true }).click();
    await sprintSelect().selectOptions('577');
    await page.getByRole('button', { name: 'Advanced JQL', exact: true }).click();
    await expect.element(page.getByRole('searchbox')).toHaveValue(manual);
    await submit().click();
    expect(xfetch).toHaveBeenLastCalledWith('/api/users/user/jira-instances/one/jql-story-search', {
      body: { jql: manual, startAt: 0, maxResults: 100 },
    });
  });

  it('offers retries after metadata failure and still allows a manual search', async () => {
    const xfetch = vi.fn<ApiClient>(async url => {
      if (url.endsWith('/issue-types') || url.includes('/sprints?')) throw new Error('Jira unavailable');
      return defaultResponse(url);
    });
    render(JQLImport, { notifications, xfetch });
    await instanceSelect().selectOptions('0');
    await expect.element(page.getByRole('button', { name: 'Retry issue types' })).toBeVisible();
    await expect.element(page.getByRole('button', { name: 'Retry sprints' })).toBeVisible();
    await page.getByRole('button', { name: 'Advanced JQL', exact: true }).click();
    await page.getByRole('searchbox').fill('issuetype = Story AND Sprint = 577');
    await submit().click();
    expect(xfetch).toHaveBeenLastCalledWith('/api/users/user/jira-instances/one/jql-story-search', {
      body: { jql: 'issuetype = Story AND Sprint = 577', startAt: 0, maxResults: 100 },
    });
  });

  it('recovers empty and failed metadata through the retry controls', async () => {
    let retry = false;
    render(JQLImport, {
      notifications,
      xfetch: async url => {
        if (url.endsWith('/issue-types')) {
          if (!retry) throw new Error('Temporarily unavailable');
          return response(issueTypes);
        }
        if (url.includes('/sprints?')) return response([]);
        return defaultResponse(url);
      },
    });
    await instanceSelect().selectOptions('0');
    await expect.element(page.getByText('No matching sprints. Try another name or use Advanced JQL.')).toBeVisible();
    retry = true;
    await page.getByRole('button', { name: 'Retry issue types' }).click();
    await expect.element(typeSelect()).toHaveValue('story');
    await expect.element(page.getByRole('button', { name: 'Retry issue types' })).not.toBeInTheDocument();
  });
});
