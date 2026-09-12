<script lang="ts">
  import HollowButton from '../global/HollowButton.svelte';
  import { user } from '../../stores';
  import type { ApiClient } from '../../types/apiclient';

  interface Props {
    instanceId: string;
    xfetch: ApiClient;
  }
  let { instanceId, xfetch }: Props = $props();
  let checking = $state(false);
  let result = $state('');
  let failed = $state(false);

  async function testConnection() {
    if (checking) return;
    checking = true;
    result = '';
    failed = false;
    try {
      const response = await xfetch(`/api/users/${$user.id}/jira-instances/${instanceId}/test`, { method: 'POST' });
      const payload = await response.json();
      if (!payload.data?.connected) throw new Error('Connection was not verified');
      result = payload.data.display_name ? `Connected: ${payload.data.display_name}` : 'Connection verified';
    } catch (error) {
      failed = true;
      result = 'Connection test failed. Please try again.';
      if (Array.isArray(error) && error[1] instanceof Response) {
        try {
          const payload = await error[1].json();
          if (typeof payload.error === 'string' && payload.error) result = payload.error;
        } catch {
          // Keep the fallback when a proxy returns HTML instead of an API error.
        }
      }
    } finally {
      checking = false;
    }
  }
</script>

<div class="inline-flex flex-col items-end gap-2 max-w-xs align-top">
  <HollowButton disabled={checking} onClick={testConnection} testid="jira-test">
    {checking ? 'Testing...' : 'Test connection'}
  </HollowButton>
  <p
    aria-live="polite"
    class="whitespace-normal text-sm"
    class:text-red-600={failed}
    class:dark:text-red-400={failed}
    class:text-green-700={!failed}
    class:dark:text-green-400={!failed}
  >
    {result}
  </p>
</div>
