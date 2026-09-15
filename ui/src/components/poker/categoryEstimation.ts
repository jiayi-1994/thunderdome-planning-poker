import type { PokerVoteCategory } from '../../types/poker';
import { availablePokerPoints, pokerPointValues } from './pointValues';

export const voteCategories: Array<{ id: PokerVoteCategory; label: string }> = [
  { id: 'testing', label: '测试' },
  { id: 'frontend', label: '前端开发' },
  { id: 'backend', label: '后端开发' },
];

export function numericPoint(value: string): number | null {
  if (value === '1/2') return 0.5;
  if (!/^\d+(\.\d+)?$/.test(value)) return null;
  const number = Number(value);
  return Number.isFinite(number) ? number : null;
}

export function categoryPointValues(points: string[]): string[] {
  const allowed = availablePokerPoints(points);
  return [...new Set(allowed.length ? allowed : pokerPointValues)];
}

export const emptyCategoryVotes = (): Record<PokerVoteCategory, string> => ({ testing: '', frontend: '', backend: '' });
