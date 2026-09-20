import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { tick } from 'svelte';
import PokerGame from './PokerGame.svelte';
import { user } from '../../stores';
import type { PokerGame as Game, PokerStoryVote, PokerUser } from '../../types/poker';
import type { SessionUser } from '../../types/user';

type SocketHandlers = {
  onmessage: (event: MessageEvent) => void;
  onopen: () => void;
  onclose: (event: { code: number }) => void;
};

const socket = vi.hoisted(() => ({
  handlers: undefined as SocketHandlers | undefined,
  send: vi.fn(),
  close: vi.fn(),
}));

vi.mock('sockette', () => ({
  default: class {
    constructor(_url: string, handlers: SocketHandlers) {
      socket.handlers = handlers;
    }
    send = socket.send;
    close = socket.close;
  },
}));

vi.mock('../../stores', async () => {
  const { writable } = await import('svelte/store');
  const session = writable({});
  return { user: { subscribe: session.subscribe, update: session.set, delete: vi.fn() } };
});

const now = new Date('2026-09-20T10:00:00Z');
const warning = 'Less than 30 seconds left. Please submit your estimate.';
const sessionUser: SessionUser = {
  id: 'self',
  name: 'Current voter',
  notificationsEnabled: true,
  createdDate: now.toISOString(),
  lastActive: now.toISOString(),
  updatedDate: now.toISOString(),
  locale: 'en',
  rank: 'REGISTERED',
  subscribed: false,
};
const participant = (id: string): PokerUser => ({
  id,
  name: id === 'self' ? 'Current voter' : 'Other voter',
  active: true,
  abandoned: false,
  spectator: false,
  avatar: '',
  gravatarHash: '',
  rank: 'REGISTERED',
});
const game = (): Game => ({
  id: 'reminder-game',
  name: 'Reminder integration',
  activePlanId: 'story',
  autoFinishVoting: false,
  votingDurationSeconds: 120,
  createdDate: now,
  updatedDate: now,
  hideVoterIdentity: true,
  leaders: ['other'],
  users: [participant('self'), participant('other')],
  plans: [
    {
      id: 'story',
      name: 'A story awaiting estimates',
      type: 'Story',
      link: '',
      active: true,
      points: '',
      priority: 0,
      position: 0,
      skipped: false,
      voteStartTime: new Date(now.getTime() - 95000),
      voteDeadline: new Date(now.getTime() + 25000),
      voteEndTime: new Date('0001-01-01T00:00:00Z'),
      votes: [],
    },
  ],
  pointAverageRounding: 'none',
  pointValuesAllowed: ['0', '1/2', '1', '2', '3', '5', '8'],
  votingLocked: false,
});

async function event(type: string, value: unknown, userId = 'self') {
  socket.handlers!.onmessage(
    new MessageEvent('message', {
      data: JSON.stringify({ type, value: JSON.stringify(value), userId }),
    }),
  );
  await tick();
}

async function openGame(state = game()) {
  render(PokerGame, {
    battleId: state.id,
    notifications: {
      success: vi.fn(),
      danger: vi.fn(),
      warning: vi.fn(),
      info: vi.fn(),
      show: vi.fn(),
      removeToast: vi.fn(),
    },
    router: { route: vi.fn() },
    xfetch: vi.fn(async () => new Response(JSON.stringify({ data: [] }))),
  });
  await tick();
  socket.handlers!.onopen();
  await event('init', state);
  return state;
}

const reminder = () => page.getByRole('alert').filter({ hasText: warning });

describe('PokerGame voting reminder eligibility', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    socket.handlers = undefined;
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] });
    vi.setSystemTime(now);
    localStorage.removeItem('poker-role:reminder-game:self');
    user.update({ ...sessionUser });
  });

  afterEach(() => {
    vi.useRealTimers();
    localStorage.removeItem('poker-role:reminder-game:self');
  });

  it.each([
    { name: 'category', ballot: { warriorId: 'self', category: 'backend', vote: '' } satisfies PokerStoryVote },
    { name: 'legacy', ballot: { warriorId: 'self', vote: '' } satisfies PokerStoryVote },
  ])('reminds an unassigned voter until their own masked $name ballot arrives', async ({ ballot }) => {
    const state = await openGame();
    await expect.element(page.getByRole('button', { name: '后端开发', exact: true })).toBeVisible();
    await expect.element(reminder()).toBeVisible();
    await expect.poll(() => document.title).toContain('🔴');

    state.plans[0].votes = [{ warriorId: 'other', category: 'backend', vote: '' }];
    await event('vote_activity', state.plans, 'other');
    await expect.element(reminder()).toBeVisible();

    state.plans[0].votes.push(ballot);
    await event('vote_activity', state.plans);
    await expect.element(reminder()).not.toBeInTheDocument();
    await expect.poll(() => document.title).not.toContain('🔴');
  });

  it('stops immediately for a local zero-point selection and resumes after retracting it', async () => {
    await openGame();
    await expect.element(reminder()).toBeVisible();
    await page.getByRole('button', { name: '后端开发', exact: true }).click();
    await page.getByRole('button', { name: '后端开发 0 点', exact: true }).click();
    await expect.element(reminder()).not.toBeInTheDocument();
    expect(JSON.parse(socket.send.mock.calls.at(-1)![0])).toMatchObject({ type: 'vote' });
    expect(JSON.parse(JSON.parse(socket.send.mock.calls.at(-1)![0]).value)).toMatchObject({
      category: 'backend',
      voteValue: '0',
    });

    await page.getByRole('button', { name: '后端开发 0 点', exact: true }).click();
    await expect.element(reminder()).toBeVisible();
    expect(JSON.parse(socket.send.mock.calls.at(-1)![0])).toMatchObject({ type: 'retract_vote' });
  });

  it('suppresses reminders for spectators, locked voting and ended games', async () => {
    const state = await openGame();
    await expect.element(reminder()).toBeVisible();
    state.users[0].spectator = true;
    await event('users_updated', state.users);
    await expect.element(reminder()).not.toBeInTheDocument();

    state.users[0].spectator = false;
    await event('users_updated', state.users);
    await expect.element(reminder()).toBeVisible();
    await event('voting_ended', state.plans);
    await expect.element(reminder()).not.toBeInTheDocument();

    await event('init', state);
    await expect.element(reminder()).toBeVisible();
    await event('game_ended', { endTime: now.toISOString(), endReason: 'Finished' });
    await expect.element(reminder()).not.toBeInTheDocument();
  });

  it('honors the account notification preference', async () => {
    user.update({ ...sessionUser, notificationsEnabled: false });
    await openGame();
    await expect.element(reminder()).not.toBeInTheDocument();
    expect(document.title).not.toContain('🔴');
    user.update({ ...sessionUser, notificationsEnabled: true });
    await expect.element(reminder()).toBeVisible();
    user.update({ ...sessionUser, notificationsEnabled: false });
    await expect.element(reminder()).not.toBeInTheDocument();
  });

  it('waits for fresh server state after reconnecting', async () => {
    const state = await openGame();
    await expect.element(reminder()).toBeVisible();
    socket.handlers!.onclose({ code: 1006 });
    await tick();
    await expect.element(reminder()).not.toBeInTheDocument();
    socket.handlers!.onopen();
    await tick();
    await expect.element(reminder()).not.toBeInTheDocument();
    state.plans[0].votes = [{ warriorId: 'self', category: 'backend', vote: '' }];
    await event('init', state);
    await expect.element(reminder()).not.toBeInTheDocument();
    state.plans[0].votes = [];
    await event('vote_retracted', state.plans);
    await expect.element(reminder()).toBeVisible();
  });

  it('clears the reminder when the server deadline reaches zero', async () => {
    await openGame();
    await expect.element(reminder()).toBeVisible();
    await vi.advanceTimersByTimeAsync(25000);
    await tick();
    await expect.element(page.getByRole('timer')).toHaveTextContent('00:00');
    await expect.element(reminder()).not.toBeInTheDocument();
    await expect.poll(() => document.title).not.toContain('🔴');
  });
});
