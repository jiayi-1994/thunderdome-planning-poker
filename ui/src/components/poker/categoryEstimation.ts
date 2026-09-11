import type { PokerVoteCategory } from '../../types/poker';

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
  const numeric = points.filter(point => numericPoint(point) !== null);
  return [...new Set(['0', ...(numeric.length ? numeric : ['1', '2', '3', '5', '8', '13']), '?'])];
}

export const emptyCategoryVotes = (): Record<PokerVoteCategory, string> => ({ testing: '', frontend: '', backend: '' });
