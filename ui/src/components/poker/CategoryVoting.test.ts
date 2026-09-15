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
  it('uses only the allowed seven cards without reintroducing removed choices', () => {
    expect(categoryPointValues(['XS', 'S', 'M', '?'])).toEqual(['0', '1/2', '1', '2', '3', '5', '8']);
    expect(categoryPointValues(['0', '1/2', '1', '2.5', '13', '?', '☕️'])).toEqual(['0', '1/2', '1']);
  });

  it('shows only the chosen role and retracts its selected score', async () => {
    const onVote = vi.fn();
    const onRetract = vi.fn();
    render(CategoryVoting, {
      points: ['1', '2', '3'],
      category: 'testing',
      selections: { testing: '2', frontend: '', backend: '' },
      isLocked: false,
      onVote,
      onRetract,
    });
    await expect
      .element(page.getByRole('button', { name: '测试 2 点', exact: true }))
      .toHaveAttribute('aria-pressed', 'true');
    await page.getByRole('button', { name: '测试 3 点', exact: true }).click();
    expect(onVote).toHaveBeenCalledWith('testing', '3');
    await page.getByRole('button', { name: '测试 2 点', exact: true }).click();
    expect(onRetract).toHaveBeenCalledWith('testing');
    await expect.element(page.getByTestId('voting-backend')).not.toBeInTheDocument();
    await expect.element(page.getByTestId('voting-frontend')).not.toBeInTheDocument();
  });

  it('locks all categories for spectators, completed votes and disconnected clients', async () => {
    render(CategoryVoting, { category: 'backend', points: ['1'], isLocked: true, onVote: vi.fn(), onRetract: vi.fn() });
    await expect.element(page.getByRole('button', { name: '后端开发 1 点', exact: true })).toBeDisabled();
    await expect.element(page.getByTestId('voting-testing')).not.toBeInTheDocument();
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

  it('finishes voting without finalizing until Save is clicked', async () => {
    const sendSocketEvent = vi.fn();
    const { rerender } = render(VotingControls, {
      planId: 'story',
      votingLocked: false,
      categoryEstimation: true,
      calculatedPoints: '12.83',
      points: ['1', '3', '5'],
      sendSocketEvent,
    });
    await page.getByTestId('voting-finish').click();
    expect(sendSocketEvent).toHaveBeenCalledExactlyOnceWith('end_voting', 'story');
    await rerender({ votingLocked: true });
    await expect.element(page.getByTestId('final-calculated-points')).toHaveTextContent('12.83');
    expect(sendSocketEvent).toHaveBeenCalledTimes(1);
    await page.getByTestId('voting-save').click();
    expect(sendSocketEvent).toHaveBeenCalledWith(
      'finalize_plan',
      JSON.stringify({ planId: 'story', planPoints: '12.83' }),
    );
  });

  it('highlights the divergent role from the saved result, including anonymous and historical views', async () => {
    const { rerender } = render(CategoryResults, {
      estimation: {
        categories: [
          { category: 'testing', average: '2', count: 4, distinctValues: ['2'], needsDiscussion: false },
          { category: 'frontend', average: '', count: 0 },
          {
            category: 'backend',
            average: '2.38',
            count: 4,
            distinctValues: ['0.5', '1', '3', '5'],
            needsDiscussion: true,
          },
        ],
        total: '4.38',
      },
      hideVoterIdentity: true,
    });
    await expect.element(page.getByRole('alert')).toHaveTextContent('评分分歧较大，请先讨论');
    await expect.element(page.getByTestId('discussion-backend')).toHaveTextContent('4 种分值');
    await expect.element(page.getByTestId('result-backend')).toHaveTextContent('1/2、1、3、5');
    await expect.element(page.getByTestId('discussion-testing')).not.toBeInTheDocument();
    await expect.element(page.getByTestId('result-frontend')).toBeVisible();
    await expect.element(page.getByTestId('category-total')).toHaveTextContent('4.38');
    await rerender({ estimation });
    await expect.element(page.getByRole('alert')).not.toBeInTheDocument();
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
