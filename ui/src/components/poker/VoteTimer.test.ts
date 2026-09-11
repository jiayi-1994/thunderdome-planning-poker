import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { tick } from 'svelte';
import VoteTimer from './VoteTimer.svelte';
import { expirationMatchesRound, remainingVotingSeconds } from './votingDeadline';

describe('two-minute voting countdown', () => {
  const startTime = new Date('2026-09-11T10:00:00Z');

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] });
    vi.setSystemTime(startTime);
  });
  afterEach(() => vi.useRealTimers());

  it('visibly counts down from 02:00 and expires at 00:00', async () => {
    const onExpire = vi.fn();
    render(VoteTimer, { currentStoryId: 'story', voteStartTime: startTime, votingLocked: false, onExpire });
    await expect.element(page.getByRole('timer')).toHaveTextContent('02:00');
    await vi.advanceTimersByTimeAsync(1000);
    await expect.element(page.getByRole('timer')).toHaveTextContent('01:59');
    await vi.advanceTimersByTimeAsync(89000);
    await expect.element(page.getByRole('timer')).toHaveTextContent('00:30');
    await expect.element(page.getByRole('timer')).toHaveClass('text-red-700');
    await vi.advanceTimersByTimeAsync(30000);
    await tick();
    await expect.element(page.getByRole('timer')).toHaveTextContent('00:00');
    expect(onExpire).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(2000);
    await expect.element(page.getByRole('timer')).toHaveTextContent('00:00');
  });

  it('restores the remaining time on reload instead of starting another two minutes', async () => {
    vi.setSystemTime(new Date(startTime.getTime() + 75000));
    render(VoteTimer, { currentStoryId: 'story', voteStartTime: startTime, votingLocked: false });
    await expect.element(page.getByRole('timer')).toHaveTextContent('00:45');
  });

  it('uses the server deadline and restarts only for a new round', async () => {
    const { rerender } = render(VoteTimer, {
      currentStoryId: 'story',
      voteStartTime: startTime,
      voteDeadline: new Date(startTime.getTime() + 10000),
      votingLocked: false,
    });
    await expect.element(page.getByRole('timer')).toHaveTextContent('00:10');
    await rerender({ voteStartTime: new Date(), voteDeadline: new Date(Date.now() + 120000) });
    await expect.element(page.getByRole('timer')).toHaveTextContent('02:00');
    await rerender({ votingLocked: true });
    await expect.element(page.getByRole('timer')).not.toBeInTheDocument();
  });

  it('never shows negative time and ignores expirations from previous rounds', () => {
    expect(remainingVotingSeconds(startTime, undefined, startTime.getTime() + 180000)).toBe(0);
    expect(
      expirationMatchesRound({ id: 'one', voteStartTime: startTime }, { planId: 'one', voteStartTime: startTime }),
    ).toBe(true);
    expect(
      expirationMatchesRound({ id: 'two', voteStartTime: startTime }, { planId: 'one', voteStartTime: startTime }),
    ).toBe(false);
    expect(
      expirationMatchesRound(
        { id: 'one', voteStartTime: new Date(startTime.getTime() + 1) },
        { planId: 'one', voteStartTime: startTime },
      ),
    ).toBe(false);
    expect(expirationMatchesRound(undefined, { planId: 'one', voteStartTime: startTime })).toBe(false);
  });
});
