<script lang="ts">
  import type { PokerVoteCategory } from '../../types/poker';
  import { voteCategories } from './categoryEstimation';

  interface Props {
    category: PokerVoteCategory | null;
    locked?: boolean;
    disabled?: boolean;
    onSelect: (category: PokerVoteCategory) => void;
  }
  let { category, locked = false, disabled = false, onSelect }: Props = $props();
  let changing = $state(false);
  let selected = $derived(voteCategories.find((item) => item.id === category));
</script>

<section
  class="mb-4 rounded-lg bg-white dark:bg-gray-800 p-4 md:p-6 shadow-md text-gray-800 dark:text-gray-100"
  aria-label="评点身份"
>
  {#if !selected || (changing && !locked)}
    <h3 class="text-xl font-semibold">先选择你的评点身份</h3>
    <p class="mt-1 mb-4 text-sm text-gray-600 dark:text-gray-300">选择后只显示对应的评点卡牌，结束后可查看全部结果。</p>
    <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
      {#each voteCategories as role}
        <button
          type="button"
          {disabled}
          aria-pressed={category === role.id}
          onclick={() => {
            onSelect(role.id);
            changing = false;
          }}
          class="rounded-lg border border-gray-300 dark:border-gray-500 px-4 py-4 text-lg font-semibold hover:border-blue-500 hover:bg-blue-50 dark:hover:bg-gray-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          >{role.label}</button
        >
      {/each}
    </div>
  {:else}
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p>我的身份：<strong>{selected.label}</strong></p>
      <button
        type="button"
        disabled={disabled || locked}
        onclick={() => (changing = true)}
        class="rounded px-2 py-1 text-sm text-blue-800 dark:text-sky-400 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:opacity-40 disabled:cursor-not-allowed"
        >更换身份</button
      >
    </div>
    {#if locked}<p class="mt-2 text-sm text-gray-600 dark:text-gray-300">本轮已评点，撤回评分后可更换身份。</p>{/if}
  {/if}
</section>
