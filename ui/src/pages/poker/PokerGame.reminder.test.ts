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

  it('counts the visible participants as users join, leave, and become inactive', async () => {
    const state = game();
    state.users.push({ ...participant('offline'), active: false });
    await openGame(state);
    const heading = page.getByRole('heading', { name: /^Players/ });
    await expect.element(heading).toHaveTextContent('Players (2)');

    state.users.push({ ...participant('observer'), spectator: true });
    await event('user_joined', state.users, 'observer');
    await expect.element(heading).toHaveTextContent('Players (3)');

    state.users = state.users.filter(person => person.id !== 'observer');
    await event('user_left', state.users, 'observer');
    await expect.element(heading).toHaveTextContent('Players (2)');

    state.users = state.users.map(person => (person.id === 'other' ? { ...person, active: false } : person));
    await event('users_updated', state.users);
    await expect.element(heading).toHaveTextContent('Players (1)');
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
    state.plans[0].active = false;
    await event('voting_ended', state.plans);
    await expect.element(reminder()).not.toBeInTheDocument();

    state.plans[0].active = true;
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

  it.each(['voting_ended', 'voting_expired'])(
    'keeps saved results visible after %s until another story is activated',
    async type => {
      const state = game();
      state.leaders = ['self'];
      state.plans.push({ ...state.plans[0], id: 'next', name: 'Next story', active: false, votes: [] });
      await openGame(state);
      state.plans[0].active = false;
      state.plans[0].points = '0';
      state.plans[0].voteEndTime = now;
      state.plans[0].votes = [{ warriorId: 'self', category: 'backend', vote: '0' }];
      state.plans[0].estimation = { total: '0', categories: [{ category: 'backend', average: '0', count: 1 }] };
      await event(
        type,
        type === 'voting_expired'
          ? { planId: 'story', voteStartTime: state.plans[0].voteStartTime, plans: state.plans }
          : state.plans,
      );
      await expect.element(page.getByTestId('voting-save')).not.toBeInTheDocument();
      await expect.element(page.getByTestId('voting-restart')).not.toBeInTheDocument();
      await expect.element(page.getByRole('timer')).not.toBeInTheDocument();
      await expect.element(reminder()).not.toBeInTheDocument();
      await expect.element(page.getByTestId('currentplan-name')).toHaveTextContent('A story awaiting estimates');
      await expect.element(page.getByTestId('category-results')).toBeVisible();
      await expect.element(page.getByTestId('category-total')).toHaveTextContent('0');
      await expect.element(page.getByTestId('unvoted-participants')).not.toBeInTheDocument();
      expect(socket.send.mock.calls.some(([raw]) => JSON.parse(raw).type === 'finalize_plan')).toBe(false);

      // A reconnect returns no activePlanId after the server saves the round.
      state.activePlanId = '';
      state.votingLocked = true;
      await event('init', state);
      await expect.element(page.getByTestId('category-total')).toHaveTextContent('0');
      await expect.element(page.getByTestId('voting-save')).not.toBeInTheDocument();

      await page.getByTestId('plan').filter({ hasText: 'Next story' }).getByTestId('plan-activate').click();
      expect(JSON.parse(socket.send.mock.calls.at(-1)![0])).toEqual({ type: 'activate_plan', value: 'next' });
      await expect.element(page.getByTestId('category-results')).toBeVisible();
      state.plans[1] = {
        ...state.plans[1],
        active: true,
        voteStartTime: now,
        voteDeadline: new Date(now.getTime() + 300000),
      };
      await event('plan_activated', state.plans);
      await expect.element(page.getByTestId('category-results')).not.toBeInTheDocument();
      await expect.element(page.getByTestId('currentplan-name')).toHaveTextContent('Next story');
      await expect.element(page.getByRole('timer')).toHaveTextContent('05:00');
      await expect.element(page.getByTestId('unvoted-participants')).toHaveTextContent('Not voted: 2');
    },
  );

  it('restores the most recent saved round on a fresh load regardless of story order', async () => {
    const state = game();
    state.activePlanId = '';
    state.votingLocked = true;
    state.plans[0] = {
      ...state.plans[0],
      active: false,
      points: '5',
      voteEndTime: now,
      votes: [{ warriorId: 'self', vote: '5' }],
    };
    state.plans.push({
      ...state.plans[0],
      id: 'older',
      name: 'Older story',
      voteStartTime: new Date(now.getTime() - 300000),
    });
    await openGame(state);
    await expect.element(page.getByTestId('currentplan-name')).toHaveTextContent('A story awaiting estimates');
    await expect.element(page.getByTestId('voteresult-graph')).toBeVisible();
    await expect.element(page.getByRole('timer')).not.toBeInTheDocument();
  });

  it('does not restore an older result when the latest round was skipped', async () => {
    const state = game();
    state.activePlanId = '';
    state.votingLocked = true;
    state.plans[0] = { ...state.plans[0], active: false, points: '5', voteEndTime: now };
    state.plans.push({ ...state.plans[0], id: 'skipped', points: '', skipped: true, voteStartTime: now });
    await openGame(state);
    await expect.element(page.getByTestId('category-results')).not.toBeInTheDocument();
    await expect.element(page.getByTestId('unvoted-participants')).not.toBeInTheDocument();
  });

  it('shows up to three unvoted names and the total, excluding spectators and inactive users', async () => {
    const state = game();
    state.users = ['self', 'Alice', 'Bob', 'Carol', 'Dave'].map(id => ({ ...participant(id), name: id }));
    state.users.push({ ...participant('spectator'), spectator: true }, { ...participant('offline'), active: false });
    state.plans[0].votes = [{ warriorId: 'self', category: 'backend', vote: '0' }];
    await openGame(state);
    const summary = page.getByTestId('unvoted-participants');
    const names = page.getByTestId('unvoted-names');
    await expect.element(summary).toHaveTextContent('Not voted: 4');
    await expect.element(names).toHaveTextContent(/Alice、\s*Bob、\s*Carol…/);
    await expect.element(names).not.toHaveTextContent('Dave');

    // Even a masked ballot counts as submitted; multiple categories still count as one person.
    state.plans[0].votes.push(
      { warriorId: 'Alice', category: 'backend', vote: '' },
      { warriorId: 'Alice', category: 'frontend', vote: '' },
    );
    await event('vote_activity', state.plans, 'Alice');
    await expect.element(summary).toHaveTextContent('Not voted: 3');
    await expect.element(names).toHaveTextContent(/Bob、\s*Carol、\s*Dave/);

    state.plans[0].votes = state.plans[0].votes.filter(vote => vote.warriorId !== 'Alice');
    await event('vote_retracted', state.plans, 'Alice');
    await expect.element(summary).toHaveTextContent('Not voted: 4');
    await expect.element(names).toHaveTextContent(/Alice、\s*Bob、\s*Carol…/);

    state.users.find(person => person.id === 'Alice')!.spectator = true;
    await event('users_updated', state.users);
    await expect.element(summary).toHaveTextContent('Not voted: 3');
    state.users.find(person => person.id === 'Bob')!.active = false;
    await event('user_left', state.users, 'Bob');
    await expect.element(summary).toHaveTextContent('Not voted: 2');
    await expect.element(names).toHaveTextContent(/Carol、\s*Dave/);
    state.users.find(person => person.id === 'Bob')!.active = true;
    await event('user_joined', state.users, 'Bob');
    await expect.element(summary).toHaveTextContent('Not voted: 3');

    state.plans[0].votes.push(...['Bob', 'Carol', 'Dave'].map(warriorId => ({ warriorId, vote: '' })));
    await event('vote_activity', state.plans);
    await expect.element(summary).toHaveTextContent('Not voted: 0');
    await expect.element(names).not.toBeInTheDocument();

    await event('game_ended', { endTime: now.toISOString(), endReason: 'Finished' });
    await expect.element(summary).not.toBeInTheDocument();
  });

  it('keeps manual Save and Restart when a timed-out round was not automatically saved', async () => {
    const state = game();
    state.leaders = ['self'];
    await openGame(state);
    state.plans[0].active = false;
    state.plans[0].votes = [{ warriorId: 'self', category: 'backend', vote: '5' }];
    state.plans[0].estimation = { total: '5', categories: [{ category: 'backend', average: '5', count: 1 }] };
    await event('voting_expired', { planId: 'story', voteStartTime: state.plans[0].voteStartTime, plans: state.plans });
    await expect.element(page.getByTestId('voting-save')).toBeEnabled();
    await expect.element(page.getByTestId('voting-restart')).toBeVisible();
    await expect
      .element(
        page.getByText('本轮已超时，评分人数不足一半或没有有效评分，未自动保存。可确认结果后手动保存，或重新评点。'),
      )
      .toBeVisible();
    await page.getByTestId('voting-save').click();
    expect(JSON.parse(socket.send.mock.calls.at(-1)![0])).toMatchObject({ type: 'finalize_plan' });
  });

  it.each(['voting_ended', 'voting_expired'])('ignores a saved %s result from an earlier round', async type => {
    const state = await openGame();
    const oldRound = { ...state.plans[0], active: false, points: '5' };
    state.plans[0] = { ...state.plans[0], voteStartTime: now, voteDeadline: new Date(now.getTime() + 300000) };
    await event('plan_activated', state.plans);
    await event(
      type,
      type === 'voting_expired'
        ? { planId: oldRound.id, voteStartTime: oldRound.voteStartTime, plans: [oldRound] }
        : [oldRound],
    );
    await expect.element(page.getByRole('timer')).toHaveTextContent('05:00');
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
