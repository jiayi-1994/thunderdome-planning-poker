<script lang="ts">
  import type { PokerStoryVote, PokerUser, PokerVoteCategory } from '../../types/poker';
  import { categoryPointValues, emptyCategoryVotes, voteCategories } from './categoryEstimation';

  interface Props {
    points: string[];
    category: PokerVoteCategory;
    selections?: Record<PokerVoteCategory, string>;
    votes?: PokerStoryVote[];
    users?: PokerUser[];
    isLocked?: boolean;
    onVote: (category: PokerVoteCategory, point: string) => void;
    onRetract: (category: PokerVoteCategory) => void;
  }
  let {
    points,
    category: selectedCategory,
    selections = emptyCategoryVotes(),
    votes = [],
    users = [],
    isLocked = true,
    onVote,
    onRetract,
  }: Props = $props();
  let values = $derived(categoryPointValues(points));
</script>

<section
  class="bg-white dark:bg-gray-800 rounded-lg shadow-md p-4 md:p-6 text-gray-800 dark:text-gray-100"
  aria-label="分项评点"
>
  <h3 class="text-xl font-semibold">我的评点</h3>
  <p class="text-sm text-gray-600 dark:text-gray-300 mt-1 mb-5">
    仅为当前身份评点。再次点击已选点数可撤回；未评分者不计入平均分的分母。
  </p>
  <div>
    {#each voteCategories.filter((item) => item.id === selectedCategory) as category}
      <fieldset disabled={isLocked} class="min-w-0" data-testid={`voting-${category.id}`}>
        <legend class="text-lg font-semibold mb-1">{category.label}</legend>
        <p class="text-sm text-gray-600 dark:text-gray-300 mb-3">
          {votes.filter(
            (vote) =>
              vote.category === category.id && users.some((user) => user.id === vote.warriorId && !user.spectator),
          ).length} 人已评点
        </p>
        <div class="grid grid-cols-3 sm:grid-cols-5 gap-3">
          {#each values as point}
            <button
              type="button"
              disabled={isLocked}
              aria-pressed={selections[category.id] === point}
              aria-label={`${category.label} ${point} 点`}
              data-testid="pointCard"
              data-point={point}
              data-active={selections[category.id] === point}
              data-locked={isLocked}
              onclick={() => (selections[category.id] === point ? onRetract(category.id) : onVote(category.id, point))}
              class="rounded-lg border py-3 text-2xl font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 disabled:opacity-40 disabled:cursor-not-allowed {selections[
                category.id
              ] === point
                ? 'border-green-600 bg-green-100 text-green-800 dark:border-lime-400 dark:bg-lime-900 dark:text-lime-200'
                : 'border-gray-300 bg-white text-gray-800 hover:bg-gray-100 dark:border-gray-500 dark:bg-gray-700 dark:text-gray-100 dark:hover:bg-gray-600'}"
            >
              {point}
            </button>
          {/each}
        </div>
        <p class="mt-3 text-sm min-h-5" aria-live="polite">
          {selections[category.id] ? `我的评分：${selections[category.id]}` : '尚未评点'}
        </p>
      </fieldset>
    {/each}
  </div>
  <p class="mt-4 border-t border-gray-200 dark:border-gray-600 pt-4 text-sm text-gray-600 dark:text-gray-300">
    揭晓后，最终点数 = 测试平均分 + 前端平均分 + 后端平均分。
  </p>
</section>
