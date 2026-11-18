import type { PokerResultCategory, PokerVoteCategory } from '../../types/poker';
import { availablePokerPoints, pokerPointValues } from './pointValues';

export const voteCategories: Array<{ id: PokerVoteCategory; label: string }> = [
  { id: 'testing', label: '测试' },
  { id: 'development', label: '开发' },
];

export const voteCategoryLabels: Record<PokerResultCategory, string> = {
  testing: '测试',
  development: '开发',
  frontend: '前端开发',
  backend: '后端开发',
};

export function currentVoteCategory(category: string | null | undefined): PokerVoteCategory | null {
  if (category === 'development' || category === 'frontend' || category === 'backend') return 'development';
  return category === 'testing' ? 'testing' : null;
}

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

export const emptyCategoryVotes = (): Record<PokerVoteCategory, string> => ({ testing: '', development: '' });
