import { describe, it, expect, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import CategoryVoting from './CategoryVoting.svelte';
import CategoryResults from './CategoryResults.svelte';
import VotingControls from './VotingControls.svelte';
import { categoryPointValues } from './categoryEstimation';
import type { PokerEstimation, PokerUser } from '../../types/poker';

const estimation: PokerEstimation = {
  categories: [
    { category: 'testing', average: '2.5', count: 2 },
    { category: 'frontend', average: '5', count: 1 },
    { category: 'backend', average: '5.33', count: 3 },
  ],
  total: '12.83',
};

describe('category estimation', () => {
  it('offers numeric cards including explicit zero for nonnumeric scales', () => {
    expect(categoryPointValues(['XS', 'S', 'M', '?'])).toEqual(['0', '1', '2', '3', '5', '8', '13', '?']);
    expect(categoryPointValues(['0', '1/2', '1', '2.5', '☕️'])).toEqual(['0', '1/2', '1', '2.5', '?']);
  });

  it('shows three independent ballots and retracts only the selected category', async () => {
    const onVote = vi.fn();
    const onRetract = vi.fn();
    render(CategoryVoting, {
      points: ['1', '2', '3'],
      selections: { testing: '2', frontend: '', backend: '' },
      isLocked: false,
      onVote,
      onRetract,
    });
    await expect
      .element(page.getByRole('button', { name: '测试 2 点', exact: true }))
      .toHaveAttribute('aria-pressed', 'true');
    await page.getByRole('button', { name: '前端开发 3 点', exact: true }).click();
    expect(onVote).toHaveBeenCalledWith('frontend', '3');
    await page.getByRole('button', { name: '测试 2 点', exact: true }).click();
    expect(onRetract).toHaveBeenCalledWith('testing');
    await expect
      .element(page.getByRole('button', { name: '后端开发 0 点', exact: true }))
      .toHaveAttribute('aria-pressed', 'false');
  });

  it('locks all categories for spectators, completed votes and disconnected clients', async () => {
    render(CategoryVoting, { points: ['1'], isLocked: true, onVote: vi.fn(), onRetract: vi.fn() });
    for (const name of ['测试 1 点', '前端开发 1 点', '后端开发 1 点']) {
      await expect.element(page.getByRole('button', { name, exact: true })).toBeDisabled();
    }
  });

  it('displays the authoritative decimal sum and hides voter identities', async () => {
    render(CategoryResults, {
      estimation,
      votes: [{ warriorId: 'one', category: 'testing', vote: '2' }],
      users: [{ id: 'one', name: 'Private name', spectator: false } as PokerUser],
      hideVoterIdentity: true,
    });
    await expect.element(page.getByTestId('category-total')).toHaveTextContent('12.83');
    await expect.element(page.getByText('匿名', { exact: true })).toBeVisible();
    await expect.element(page.getByText('Private name', { exact: true })).not.toBeInTheDocument();
  });

  it('saves the calculated total without snapping to the card scale', async () => {
    const sendSocketEvent = vi.fn();
    render(VotingControls, {
      planId: 'story',
      votingLocked: true,
      categoryEstimation: true,
      calculatedPoints: '12.83',
      points: ['1', '3', '5'],
      sendSocketEvent,
    });
    await page.getByTestId('voting-save').click();
    expect(sendSocketEvent).toHaveBeenCalledWith(
      'finalize_plan',
      JSON.stringify({ planId: 'story', planPoints: '12.83' }),
    );
  });

  it('excludes categories without numeric ballots and still allows saving participating categories', async () => {
    render(CategoryResults, {
      estimation: {
        categories: [
          { category: 'testing', average: '2.5', count: 2 },
          { category: 'frontend', average: '', count: 0 },
          { category: 'backend', average: '', count: 0 },
        ],
        total: '2.5',
      },
    });
    await expect.element(page.getByTestId('category-total')).toHaveTextContent('2.5');
    await expect.element(page.getByTestId('result-frontend')).toHaveTextContent('无有效评分，不参与合计');
    await expect.element(page.getByTestId('result-testing')).toHaveTextContent('2 个有效评分');
  });

  it('blocks saving an incomplete estimate and allows a zero total', async () => {
    const { rerender } = render(VotingControls, {
      planId: 'story',
      votingLocked: true,
      categoryEstimation: true,
      calculatedPoints: '',
    });
    await expect.element(page.getByTestId('voting-save')).toBeDisabled();
    await rerender({ calculatedPoints: '0' });
    await expect.element(page.getByTestId('voting-save')).toBeEnabled();
    await expect.element(page.getByTestId('final-calculated-points')).toHaveTextContent('0');
  });
});
