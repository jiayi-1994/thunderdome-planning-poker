import { describe, expect, it, vi } from 'vitest';
import { page, userEvent } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import JiraWritebackSettings from './JiraWritebackSettings.svelte';
import JiraSyncStatus from './JiraSyncStatus.svelte';
import { applyJiraSyncEvent, preserveJiraSyncs } from './jiraWriteback';
import type { PokerJiraSync, PokerStory } from '../../types/poker';
import type { ApiClientConfig } from '../../types/apiclient';

const notifications = {
  success: vi.fn(),
  danger: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
  show: vi.fn(),
  removeToast: vi.fn(),
};
const settings = { enabled: false, instanceId: '', fieldId: '', fieldName: '', host: '' };
const response = (data: unknown) => new Response(JSON.stringify({ data }), { status: 200 });

describe('Jira writeback settings and status', () => {
  it('defaults to Story Points and saves without requiring a field selection', async () => {
    const close = vi.fn();
    const xfetch = vi.fn(async (url: string, config?: ApiClientConfig) => {
      if (config?.method === 'PUT') return response(settings);
      if (url.includes('/fields'))
        return response([
          { id: 'customfield_13565', name: 'CloseHours' },
          { id: 'customfield_13704', name: 'issue创建Story Point' },
          { id: 'customfield_10006', name: 'Story Points' },
        ]);
      return response({ settings, instances: [{ id: 'jira-one', host: 'https://team.atlassian.net' }] });
    });
    render(JiraWritebackSettings, { gameId: 'game', xfetch, notifications, close });
    await page.getByRole('checkbox').click();
    await expect.element(page.getByTestId('jira-writeback-save')).toBeDisabled();
    await userEvent.selectOptions(page.getByRole('combobox', { name: 'Jira 实例' }), 'jira-one');
    await expect.element(page.getByRole('combobox', { name: '点数字段' })).not.toBeDisabled();
    await expect.element(page.getByRole('combobox', { name: '点数字段' })).toHaveValue('customfield_10006');
    await page.getByTestId('jira-writeback-save').click();
    expect(xfetch).toHaveBeenCalledWith('/api/battles/game/jira-writeback', {
      method: 'PUT',
      body: { enabled: true, instanceId: 'jira-one', fieldId: 'customfield_10006' },
    });
    expect(close).toHaveBeenCalled();
  });

  it.each([
    { savedField: '', expected: 'customfield_10006' },
    { savedField: 'customfield_99999', expected: 'customfield_10006' },
    { savedField: 'customfield_13565', expected: 'customfield_13565' },
  ])(
    'resolves the default on reopening while preserving valid saved fields ($savedField)',
    async ({ savedField, expected }) => {
      render(JiraWritebackSettings, {
        gameId: 'game',
        xfetch: async (url: string) =>
          url.includes('/fields')
            ? response([
                { id: 'customfield_13565', name: 'CloseHours' },
                { id: 'customfield_10006', name: 'Story Points' },
              ])
            : response({
                settings: { ...settings, enabled: true, instanceId: 'jira-one', fieldId: savedField },
                instances: [{ id: 'jira-one', host: 'https://team.atlassian.net' }],
              }),
        notifications,
        close: vi.fn(),
      });
      await expect.element(page.getByRole('combobox', { name: '点数字段' })).toHaveValue(expected);
      await expect.element(page.getByTestId('jira-writeback-save')).not.toBeDisabled();
    },
  );

  it.each([
    [{ id: 'customfield_13565', name: 'CloseHours' }],
    [
      { id: 'customfield_1', name: 'Story Points' },
      { id: 'customfield_2', name: 'Story Points' },
    ],
  ])('requires an explicit choice when Story Points is missing or ambiguous (%j)', async (...fields) => {
    render(JiraWritebackSettings, {
      gameId: 'game',
      xfetch: async (url: string) =>
        url.includes('/fields')
          ? response(fields)
          : response({
              settings: { ...settings, enabled: true, instanceId: 'jira-one' },
              instances: [{ id: 'jira-one', host: 'https://team.atlassian.net' }],
            }),
      notifications,
      close: vi.fn(),
    });
    await expect.element(page.getByRole('combobox', { name: '点数字段' })).toHaveValue('');
    await expect.element(page.getByTestId('jira-writeback-save')).toBeDisabled();
  });

  it('does not enable saving when settings could not be loaded', async () => {
    render(JiraWritebackSettings, {
      gameId: 'game',
      xfetch: async () => new Response(JSON.stringify({ error: '读取设置失败' }), { status: 503 }),
      notifications,
      close: vi.fn(),
    });
    await expect.element(page.getByRole('alert')).toHaveTextContent('读取设置失败');
    await expect.element(page.getByTestId('jira-writeback-save')).toBeDisabled();
  });

  it('ignores late field responses from a previously selected Jira instance', async () => {
    let finishFirst: ((response: Response) => void) | undefined;
    const first = new Promise<Response>(resolve => {
      finishFirst = resolve;
    });
    const xfetch = vi.fn(async (url: string) => {
      if (url.includes('instanceId=one')) return first;
      if (url.includes('instanceId=two')) return response([{ id: 'customfield_2', name: 'Story Points' }]);
      return response({
        settings,
        instances: [
          { id: 'one', host: 'First Jira' },
          { id: 'two', host: 'Second Jira' },
        ],
      });
    });
    render(JiraWritebackSettings, { gameId: 'game', xfetch, notifications, close: vi.fn() });
    await page.getByRole('checkbox').click();
    await userEvent.selectOptions(page.getByRole('combobox', { name: 'Jira 实例' }), 'one');
    await userEvent.selectOptions(page.getByRole('combobox', { name: 'Jira 实例' }), 'two');
    await expect.element(page.getByRole('combobox', { name: '点数字段' })).toHaveValue('customfield_2');
    finishFirst?.(response([{ id: 'customfield_1', name: 'Story Points' }]));
    await first;
    await new Promise(requestAnimationFrame);
    await expect.element(page.getByRole('option', { name: 'Story Points（customfield_1）' })).not.toBeInTheDocument();
    await expect.element(page.getByRole('combobox', { name: 'Jira 实例' })).toHaveValue('two');
    await expect.element(page.getByRole('combobox', { name: '点数字段' })).toHaveValue('customfield_2');
  });

  it('shows failure without losing points and only offers retry to facilitators', async () => {
    const sync: PokerJiraSync = {
      status: 'failed',
      issueKey: 'TEST-1',
      points: '12.83',
      attempts: 3,
      error: 'Jira 无编辑权限（403）',
      updatedAt: '2026-09-11T10:00:00Z',
    };
    const xfetch = vi.fn(async () => new Response(null, { status: 202 }));
    const { rerender } = render(JiraSyncStatus, {
      gameId: 'game',
      storyId: 'story',
      sync,
      xfetch,
      notifications,
      canRetry: false,
    });
    await expect.element(page.getByRole('status')).toHaveTextContent('Jira 无编辑权限（403）');
    await expect.element(page.getByRole('button', { name: '重试回写' })).not.toBeInTheDocument();
    await rerender({ canRetry: true });
    await page.getByRole('button', { name: '重试回写' }).click();
    expect(xfetch).toHaveBeenCalledWith('/api/battles/game/plans/story/jira-retry', { method: 'POST' });
    await expect.element(page.getByRole('status')).toHaveTextContent('正在回写 Jira');
    await rerender({ sync: { ...sync, status: 'succeeded', error: '', updatedAt: '2026-09-11T10:00:10Z' } });
    await expect.element(page.getByRole('status')).toHaveTextContent('已回写 Jira · TEST-1 · 12.83 点');
  });
});

describe('Jira websocket ordering', () => {
  const start = new Date('2026-09-11T10:00:00Z');
  const story: PokerStory = {
    id: 'story',
    name: '',
    type: '',
    active: true,
    points: '',
    priority: 99,
    skipped: false,
    voteStartTime: start,
    voteEndTime: start,
    votes: [],
    position: 1,
  };
  const success: PokerJiraSync = {
    status: 'succeeded',
    points: '5',
    issueKey: 'TEST-1',
    attempts: 1,
    updatedAt: '2026-09-11T10:02:01Z',
  };
  const pending: PokerJiraSync = { ...success, status: 'pending', updatedAt: '2026-09-11T10:02:00Z' };

  it('retains success when it arrives before a delayed voting-ended event', () => {
    const early = applyJiraSyncEvent([story], { planId: story.id, voteStartTime: start, sync: success });
    const revealed = preserveJiraSyncs(early, [{ ...story, active: false, jiraSync: pending }]);
    expect(revealed[0].jiraSync?.status).toBe('succeeded');
    expect(revealed[0].active).toBe(false);
  });

  it('rejects old-round events and older status updates', () => {
    const current = [{ ...story, active: false, jiraSync: success }];
    expect(applyJiraSyncEvent(current, { planId: story.id, voteStartTime: start, sync: pending })[0].jiraSync).toEqual(
      success,
    );
    const restarted = [{ ...story, voteStartTime: new Date(start.getTime() + 120000) }];
    expect(
      applyJiraSyncEvent(restarted, { planId: story.id, voteStartTime: start, sync: success })[0].jiraSync,
    ).toBeUndefined();
    expect(preserveJiraSyncs(current, restarted)[0].jiraSync).toBeUndefined();
  });

  it('preserves ordering for database timestamps within the same millisecond', () => {
    const earlier = { ...pending, updatedAt: '2026-09-11T10:02:01.123100Z' };
    const later = { ...success, updatedAt: '2026-09-11T10:02:01.123900Z' };
    const current = [{ ...story, active: false, jiraSync: later }];
    expect(applyJiraSyncEvent(current, { planId: story.id, voteStartTime: start, sync: earlier })[0].jiraSync).toEqual(
      later,
    );
    expect(preserveJiraSyncs(current, [{ ...story, active: false, jiraSync: earlier }])[0].jiraSync).toEqual(later);
  });
});
