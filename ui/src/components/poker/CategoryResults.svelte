<script lang="ts">
  import type { PokerEstimation, PokerStoryVote, PokerUser } from '../../types/poker';
  import { voteCategories } from './categoryEstimation';

  interface Props {
    estimation?: PokerEstimation;
    votes?: PokerStoryVote[];
    users?: PokerUser[];
    hideVoterIdentity?: boolean;
  }
  let { estimation, votes = [], users = [], hideVoterIdentity = false }: Props = $props();
</script>

<section
  class="rounded-lg bg-white dark:bg-gray-800 p-4 md:p-6 shadow-md text-gray-800 dark:text-gray-100"
  aria-label="分项评点结果"
  data-testid="category-results"
>
  <h3 class="text-xl font-semibold mb-4">分项评点结果</h3>
  <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
    {#each voteCategories as category}
      {@const group = estimation?.categories.find((group) => group.category === category.id)}
      <div class="bg-gray-100 dark:bg-gray-700 rounded-lg p-4" data-testid={`result-${category.id}`}>
        <h4 class="font-semibold">{category.label}</h4>
        <div class="mt-2 text-3xl font-bold" data-testid="category-average">{group?.average || '—'}</div>
        <p class="mt-1 text-sm text-gray-600 dark:text-gray-300">
          {group?.count ? `平均点数 · ${group.count} 个有效评分` : '无有效评分，不参与合计'}
        </p>
        {#if votes.length}
          <ul class="mt-3 space-y-1 text-sm">
            {#each votes.filter((vote) => vote.category === category.id && users.some((user) => user.id === vote.warriorId && !user.spectator)) as vote}
              <li class="flex justify-between gap-2">
                <span class="truncate"
                  >{hideVoterIdentity ? '匿名' : users.find((user) => user.id === vote.warriorId)?.name}</span
                >
                <span class="font-semibold">{vote.vote}</span>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    {/each}
  </div>
  <div
    class="mt-5 border-t border-gray-200 dark:border-gray-600 pt-4 flex flex-wrap items-center justify-between gap-3"
  >
    <div>
      <h4 class="font-semibold">最终点数</h4>
      <p class="text-sm text-gray-600 dark:text-gray-300">测试平均分 + 前端平均分 + 后端平均分</p>
    </div>
    <output class="text-3xl font-bold text-green-700 dark:text-lime-400" data-testid="category-total"
      >{estimation?.total || '无有效评分'}</output
    >
  </div>
  <p class="mt-3 text-sm text-gray-600 dark:text-gray-300">
    {#if !estimation?.total}本轮没有数字评分，可重新开启评点。{/if}
    每类平均分 = 有效评分之和 ÷ 实际评分人数。未投票和“？”不计入分母；无有效评分的类别不参与合计。结果保留两位小数。
  </p>
</section>
