<script lang="ts">
  import type { PokerJiraSync } from '../../types/poker';
  import type { ApiClient } from '../../types/apiclient';
  import type { NotificationService } from '../../types/notifications';
  import { jiraErrorMessage } from './jiraWriteback';

  interface Props {
    sync?: PokerJiraSync;
    gameId: string;
    storyId: string;
    canRetry?: boolean;
    xfetch: ApiClient;
    notifications: NotificationService;
  }
  let { sync, gameId, storyId, canRetry = false, xfetch, notifications }: Props = $props();
  let retrying = $state(false);
  let queuedAt = $state('');
  let waiting = $derived(retrying || (queuedAt !== '' && queuedAt === sync?.updatedAt));
  async function retry() {
    if (waiting) return;
    retrying = true;
    try {
      const res = await xfetch(`/api/battles/${gameId}/plans/${storyId}/jira-retry`, { method: 'POST' });
      if (!res.ok) throw res;
      queuedAt = sync?.updatedAt || '';
    } catch (error) {
      notifications.danger(await jiraErrorMessage(error, '重试 Jira 回写失败。'));
    } finally {
      retrying = false;
    }
  }
</script>

{#if sync}
  <div class="text-sm font-normal leading-relaxed" data-testid="jira-sync-status" role="status">
    {#if sync.status === 'awaiting_save'}
      <p class="text-gray-600 dark:text-gray-300">点击 Save 确认分数后回写 Jira。</p>
    {:else if sync.status === 'succeeded'}
      <p class="text-green-700 dark:text-lime-400">已回写 Jira · {sync.issueKey} · {sync.points} 点</p>
    {:else if sync.status === 'pending' || waiting}
      <p class="text-blue-700 dark:text-sky-400">
        {sync.attempts > 0 && !waiting ? 'Jira 回写暂未成功，正在自动重试…' : '正在回写 Jira…'}
      </p>
    {:else if sync.status === 'failed'}
      <p class="text-red-700 dark:text-red-400">Jira 回写失败 · {sync.error}</p>
      {#if canRetry}<button
          type="button"
          onclick={retry}
          disabled={waiting}
          class="mt-1 underline font-semibold text-blue-700 dark:text-sky-400"
          data-testid="jira-sync-retry">重试回写</button
        >{/if}
    {:else if sync.status === 'skipped'}
      <p class="text-gray-600 dark:text-gray-300">{sync.error}</p>
    {:else}
      <p class="text-gray-600 dark:text-gray-300">本次 Jira 回写已取消。</p>
    {/if}
  </div>
{/if}
