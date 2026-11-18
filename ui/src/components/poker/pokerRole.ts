import type { PokerStoryVote, PokerVoteCategory } from '../../types/poker';
import { currentVoteCategory } from './categoryEstimation';

const roleKey = (gameId: string, userId: string) => `poker-role:${gameId}:${userId}`;

export function readPokerRole(gameId: string, userId: string): PokerVoteCategory | null {
  try {
    return currentVoteCategory(localStorage.getItem(roleKey(gameId, userId)));
  } catch {
    return null;
  }
}

export function savePokerRole(gameId: string, userId: string, category: PokerVoteCategory) {
  try {
    localStorage.setItem(roleKey(gameId, userId), category);
  } catch {
    // Choosing a role still works when browser storage is unavailable.
  }
}

// A ballot already cast in this round takes precedence over a stale browser preference.
export function roleForRound(votes: PokerStoryVote[], userId: string, preferred: PokerVoteCategory | null) {
  const categories = votes.filter(vote => vote.warriorId === userId).map(vote => currentVoteCategory(vote.category));
  return preferred && categories.includes(preferred) ? preferred : (categories.find(Boolean) ?? preferred);
}
