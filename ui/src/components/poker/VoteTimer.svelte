<script lang="ts">
  import { onMount } from 'svelte';
  import { remainingVotingSeconds } from './votingDeadline';

  interface Props {
    currentStoryId?: string;
    votingLocked?: boolean;
    voteStartTime?: Date;
    voteDeadline?: Date;
    onExpire?: () => void;
  }

  let {
    currentStoryId = '',
    votingLocked = true,
    voteStartTime = new Date(),
    voteDeadline,
    onExpire = () => {},
  }: Props = $props();

  let currentTime = $state(Date.now());

  let active = $derived(currentStoryId !== '' && !votingLocked);
  let remaining = $derived(remainingVotingSeconds(voteStartTime, voteDeadline, currentTime));

  $effect(() => {
    if (active && remaining === 0) onExpire();
  });

  onMount(() => {
    const voteCounter = setInterval(() => {
      currentTime = Date.now();
    }, 250);

    return () => {
      clearInterval(voteCounter);
    };
  });
</script>

{#if active}
  <div class="text-sm text-gray-600 dark:text-gray-300 mb-1">评点倒计时</div>
  <div
    class="font-semibold text-3xl md:text-4xl tabular-nums {remaining <= 30
      ? 'text-red-700 dark:text-red-400'
      : 'text-gray-700 dark:text-gray-300'}"
    role="timer"
    aria-label="评点剩余时间"
    data-testid="vote-timer"
  >
    {String(Math.floor(remaining / 60)).padStart(2, '0')}:{String(remaining % 60).padStart(2, '0')}
  </div>
  <p class="text-sm text-gray-600 dark:text-gray-300 mt-1">
    {remaining === 0 ? '时间到，正在汇总评分…' : '默认 2 分钟，到时自动揭晓'}
  </p>
{/if}
