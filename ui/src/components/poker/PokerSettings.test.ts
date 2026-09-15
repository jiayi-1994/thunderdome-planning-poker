import { describe, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import EditPokerGame from './EditPokerGame.svelte';
import CreatePokerGame from './CreatePokerGame.svelte';

vi.mock('../../stores', async () => {
  const { writable } = await import('svelte/store');
  return { user: writable({ id: 'user', type: 'REGISTERED' }) };
});

const deck = ['0', '1/2', '1', '2', '3', '5', '8'];
const notifications = {
  success: vi.fn(),
  danger: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
  show: vi.fn(),
  removeToast: vi.fn(),
};
const response = (data: unknown) => new Response(JSON.stringify({ data }), { status: 200 });
const visibleCards = () =>
  Array.from(
    document.querySelectorAll<HTMLInputElement>('input[type="checkbox"][value]:not([value=""])'),
    input => input.value,
  );

describe('Poker countdown and card settings', () => {
  it('loads a saved duration and saves the revised duration in seconds during an active round', async () => {
    const handleBattleEdit = vi.fn();
    render(EditPokerGame, {
      battleName: 'Release planning',
      points: [...deck],
      votingLocked: false,
      votingDurationSeconds: 300,
      handleBattleEdit,
      notifications,
      xfetch: async () => response([]),
    });
    const minutes = page.getByRole('spinbutton', { name: '每轮倒计时（分钟）' });
    await expect.element(minutes).toHaveValue(5);
    expect(visibleCards()).toEqual(deck);
    await minutes.fill('3');
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    expect(handleBattleEdit).toHaveBeenCalledWith(
      expect.objectContaining({ votingDurationSeconds: 180, pointValuesAllowed: deck }),
    );
  });

  it('defaults to two minutes and blocks blank, fractional, or out-of-range durations', async () => {
    const handleBattleEdit = vi.fn();
    render(EditPokerGame, {
      battleName: 'Release planning',
      points: [...deck],
      votingLocked: true,
      handleBattleEdit,
      notifications,
      xfetch: async () => response([]),
    });
    const minutes = page.getByRole('spinbutton', { name: '每轮倒计时（分钟）' });
    await expect.element(minutes).toHaveValue(2);
    for (const invalid of ['', '0', '61', '1.5']) {
      await minutes.fill(invalid);
      await page.getByRole('button', { name: 'Save', exact: true }).click();
      expect(handleBattleEdit).not.toHaveBeenCalled();
    }
    await minutes.fill('60');
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    expect(handleBattleEdit).toHaveBeenCalledWith(expect.objectContaining({ votingDurationSeconds: 3600 }));
  });

  it('only offers the seven retained cards when creating a game from an older scale', async () => {
    render(CreatePokerGame, {
      notifications,
      router: { route: vi.fn() },
      xfetch: async (url: string) =>
        response(
          url.endsWith('/estimation-scales/public')
            ? [
                { id: 'shirts', name: 'T-Shirt Sizes', defaultScale: true, values: ['XS', 'S', 'M', 'L', '?'] },
                {
                  id: 'default',
                  name: 'Thunderdome Default',
                  defaultScale: true,
                  values: [...deck, '13', '20', '21', '34', '40', '55', '100', '?', '☕️'],
                },
              ]
            : [],
        ),
    });
    await expect.poll(visibleCards).toEqual(deck);
    await page.getByRole('button', { name: 'Thunderdome Default', exact: true }).click();
    await expect.element(page.getByRole('button', { name: 'T-Shirt Sizes' })).not.toBeInTheDocument();
  });

  it.each([
    { scope: 'user' as const, apiPrefix: '/api', meta: { jiraWritebackEnabled: true } },
    { scope: 'project' as const, apiPrefix: '/api/projects/project', meta: { jiraWritebackEnabled: true } },
    {
      scope: 'user' as const,
      apiPrefix: '/api',
      meta: { jiraWritebackEnabled: false, jiraWritebackWarning: '会议已创建，但 Jira 连接失败，请检查配置。' },
    },
    { scope: 'user' as const, apiPrefix: '/api', meta: { jiraWritebackEnabled: false } },
  ])('opens the created $scope game and reports Jira initialization: $meta', async ({ scope, apiPrefix, meta }) => {
    vi.clearAllMocks();
    const route = vi.fn();
    const xfetch = vi.fn(async (url: string, config?: { body?: unknown }) => {
      if (config?.body) return new Response(JSON.stringify({ data: { id: 'new-game' }, meta }), { status: 200 });
      return response(
        url.endsWith('/estimation-scales/public')
          ? [{ id: 'default', name: 'Thunderdome Default', defaultScale: true, values: deck }]
          : [],
      );
    });
    render(CreatePokerGame, { notifications, router: { route }, xfetch, scope, apiPrefix });
    await expect.poll(visibleCards).toEqual(deck);
    await page.getByRole('textbox', { name: 'Game Name', exact: true }).fill('New planning');
    await page.getByRole('button', { name: 'Create Game', exact: true }).click();
    await expect.poll(() => route.mock.calls).toEqual([['/game/new-game']]);
    expect(xfetch.mock.calls.filter(([, config]) => config?.body)).toHaveLength(1);
    expect(xfetch.mock.calls.some(([url]) => url.includes('jira-writeback'))).toBe(false);
    expect(notifications.danger).not.toHaveBeenCalled();
    if (meta.jiraWritebackEnabled) {
      expect(notifications.success).toHaveBeenCalledWith(
        'Jira Story Points writeback is enabled. Points are written only after you click Save.',
      );
    } else if (meta.jiraWritebackWarning) {
      expect(notifications.warning).toHaveBeenCalledWith(meta.jiraWritebackWarning);
    } else {
      expect(notifications.success).not.toHaveBeenCalled();
      expect(notifications.warning).not.toHaveBeenCalled();
    }
  });
});
