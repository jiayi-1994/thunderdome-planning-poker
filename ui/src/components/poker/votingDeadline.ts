import type { PokerStory, PokerVotingExpiration } from '../../types/poker';

export const votingDurationMs = 2 * 60 * 1000;

export function remainingVotingSeconds(startTime: Date, deadline: Date | undefined, now: number): number {
  const end = deadline ? new Date(deadline).getTime() : new Date(startTime).getTime() + votingDurationMs;
  return Number.isFinite(end) ? Math.max(0, Math.ceil((end - now) / 1000)) : 0;
}

export function expirationMatchesRound(
  story: Pick<PokerStory, 'id' | 'voteStartTime'> | undefined,
  expiration: Pick<PokerVotingExpiration, 'planId' | 'voteStartTime'>,
): boolean {
  return Boolean(
    story &&
    story.id === expiration.planId &&
    new Date(story.voteStartTime).getTime() === new Date(expiration.voteStartTime).getTime(),
  );
}
