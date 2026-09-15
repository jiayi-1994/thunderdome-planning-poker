import { beforeEach, describe, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { get } from 'svelte/store';
import en from './en';
import zh from './zh';
import { locale, setLocale } from './i18n-svelte';
import { loadLocale } from './i18n-util.sync';
import { resolveLocale } from './locale';
import { DefaultLocale, locales } from '../config';
import GlobalHeader from '../components/global/GlobalHeader.svelte';
import VotingControls from '../components/poker/VotingControls.svelte';
import EditPokerGame from '../components/poker/EditPokerGame.svelte';
import CreatePokerGame from '../components/poker/CreatePokerGame.svelte';
import JiraSyncStatus from '../components/poker/JiraSyncStatus.svelte';
import Landing from '../pages/Landing.svelte';

vi.mock('../stores', async () => {
  const { writable } = await import('svelte/store');
  return { user: writable({}) };
});

const notifications = {
  success: vi.fn(),
  danger: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
  show: vi.fn(),
  removeToast: vi.fn(),
};
const response = (data: unknown) => new Response(JSON.stringify({ data }), { status: 200 });
const deck = ['0', '1/2', '1', '2', '3', '5', '8'];

function flatten(value: object, prefix = ''): Record<string, string> {
  return Object.fromEntries(
    Object.entries(value).flatMap(([key, text]) => {
      const path = prefix ? `${prefix}.${key}` : key;
      return typeof text === 'string' ? [[path, text]] : Object.entries(flatten(text, path));
    }),
  );
}

describe('Chinese language support', () => {
  beforeEach(() => {
    loadLocale('zh');
    setLocale('zh');
  });

  it('translates every base key and preserves interpolation parameters', () => {
    const original = flatten(en);
    const translated = flatten(zh);
    expect(Object.keys(translated).sort()).toEqual(Object.keys(original).sort());
    const parameters = (value: string) =>
      [...new Set([...value.matchAll(/\{([^{}:|]+)(?::[^{}]+)?\}/g)].map(match => match[1]))].sort();
    for (const [key, value] of Object.entries(original)) {
      expect(translated[key].trim(), key).not.toBe('');
      expect(parameters(translated[key]), key).toEqual(parameters(value));
    }
    expect(locales.zh).toBe('简体中文');
  });

  it('defaults to Chinese and safely honors personal and configured languages', () => {
    expect(DefaultLocale).toBe('zh');
    for (const invalid of [undefined, null, '', 'unknown', 'zh-CN', 123]) {
      expect(resolveLocale(invalid, DefaultLocale)).toBe('zh');
      expect(resolveLocale(invalid, 'en')).toBe('en');
      expect(resolveLocale(invalid, 'unknown')).toBe('zh');
    }
    expect(resolveLocale('en', 'zh')).toBe('en');
    expect(resolveLocale('zh', 'en')).toBe('zh');
    expect(resolveLocale('fa', 'zh')).toBe('fa');
  });

  it('switches the home page, result Save button and Jira prompt using the language menu', async () => {
    const sendSocketEvent = vi.fn();
    const xfetch = vi.fn(async () => response([]));
    render(GlobalHeader, { currentPage: 'landing', router: { route: vi.fn() }, notifications, xfetch });
    render(Landing, { xfetch });
    render(VotingControls, {
      planId: 'story',
      votingLocked: true,
      categoryEstimation: true,
      calculatedPoints: '2.9',
      sendSocketEvent,
    });
    render(JiraSyncStatus, {
      gameId: 'game',
      storyId: 'story',
      xfetch,
      notifications,
      sync: {
        status: 'awaiting_save',
        points: '2.9',
        issueKey: 'ZE-17470',
        attempts: 0,
        updatedAt: '2026-09-15T00:00:00Z',
      },
    });
    await expect.element(page.getByRole('heading', { name: '通过评点达成团队共识' })).toBeVisible();
    await expect.element(page.getByTestId('voting-save')).toHaveTextContent('保存');
    await expect.element(page.getByTestId('jira-sync-status')).toHaveTextContent('点击 保存 确认分数后回写 Jira。');
    expect(sendSocketEvent).not.toHaveBeenCalled();
    expect(xfetch).not.toHaveBeenCalled();

    await page.getByRole('button', { name: '语言', exact: true }).first().click();
    await page.getByTestId('locale-English').click();
    await expect.poll(() => get(locale)).toBe('en');
    await expect.element(page.getByTestId('voting-save')).toHaveTextContent('Save');
    await expect.element(page.getByRole('heading', { name: 'Planning Poker That Gets Consensus' })).toBeVisible();
    await page.getByRole('button', { name: 'Locale', exact: true }).first().click();
    await page.getByTestId('locale-简体中文').click();
    await expect.element(page.getByTestId('voting-save')).toHaveTextContent('保存');
    await page.getByTestId('voting-save').click();
    expect(sendSocketEvent).toHaveBeenCalledExactlyOnceWith(
      'finalize_plan',
      JSON.stringify({ planId: 'story', planPoints: '2.9' }),
    );
  });

  it('shows Chinese creation labels and retains exactly the seven allowed cards', async () => {
    render(CreatePokerGame, {
      notifications,
      router: { route: vi.fn() },
      xfetch: async (url: string) =>
        response(
          url.endsWith('/estimation-scales/public')
            ? [{ id: 'default', name: 'Thunderdome Default', defaultScale: true, values: [...deck, '13', '?', '☕️'] }]
            : [],
        ),
    });
    await expect.element(page.getByLabelText('会议名称')).toBeVisible();
    await expect.element(page.getByText('评点尺度', { exact: true })).toBeVisible();
    await expect.element(page.getByRole('button', { name: '创建评点会议', exact: true })).toBeVisible();
    await expect
      .poll(() =>
        Array.from(
          document.querySelectorAll<HTMLInputElement>('input[type="checkbox"][value]:not([value=""])'),
          input => input.value,
        ),
      )
      .toEqual(deck);
  });

  it('saves countdown settings through the Chinese Save button', async () => {
    const handleBattleEdit = vi.fn();
    render(EditPokerGame, {
      battleName: '迭代评点',
      points: [...deck],
      votingLocked: true,
      handleBattleEdit,
      notifications,
      xfetch: async () => response([]),
    });
    await page.getByRole('spinbutton', { name: '每轮倒计时（分钟）' }).fill('3');
    await page.getByRole('button', { name: '保存', exact: true }).click();
    expect(handleBattleEdit).toHaveBeenCalledWith(
      expect.objectContaining({ votingDurationSeconds: 180, pointValuesAllowed: deck }),
    );
  });
});
