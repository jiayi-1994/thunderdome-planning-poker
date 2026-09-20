<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { LoaderCircle } from '@lucide/svelte';
  import SelectInput from '../forms/SelectInput.svelte';
  import LL from '../../i18n/i18n-svelte';
  import type { ApiClient } from '../../types/apiclient';

  type IssueType = { id: string; name: string; subtask: boolean };
  type Sprint = { id: number; name: string; state: string; boardName: string };

  interface Props {
    endpoint: string;
    xfetch: ApiClient;
    query?: string;
    loading?: boolean;
    disabled?: boolean;
  }

  let { endpoint, xfetch, query = $bindable(''), loading = $bindable(true), disabled = false }: Props = $props();
  let issueTypes = $state<IssueType[]>([]);
  let selectedIssueType = $state('');
  let issueTypesLoading = $state(true);
  let issueTypesError = $state(false);
  let sprints = $state<Sprint[]>([]);
  let selectedSprintId = $state('');
  let selectedSprint = $state<Sprint | undefined>();
  let sprintQuery = $state('');
  let sprintsLoading = $state(true);
  let sprintsError = $state(false);
  let alive = true;
  let issueTypesVersion = 0;
  let sprintsVersion = 0;
  let pendingSprintQuery: string | null = null;

  const sprintOptions = $derived(
    selectedSprint && !sprints.some(sprint => sprint.id === selectedSprint?.id)
      ? [selectedSprint, ...sprints]
      : sprints,
  );

  $effect(() => {
    const issueType = issueTypes.find(type => type.id === selectedIssueType);
    const clauses: string[] = [];
    // Jira can localize display names; queries must use the stable instance-specific ID.
    if (issueType) clauses.push(`issuetype = "${issueType.id.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`);
    if (selectedSprintId) clauses.push(`Sprint = ${selectedSprintId}`);
    query = clauses.join(' AND ') || 'ORDER BY created DESC';
    loading = issueTypesLoading || sprintsLoading;
  });

  async function loadIssueTypes() {
    const version = ++issueTypesVersion;
    issueTypesLoading = true;
    issueTypesError = false;
    try {
      const result = await (await xfetch(`${endpoint}/issue-types`)).json();
      if (!alive || version !== issueTypesVersion) return;
      if (!Array.isArray(result.data)) throw new Error('Invalid issue types response');
      issueTypes = result.data;
      selectedIssueType = issueTypes.find(type => type.name.toLowerCase() === 'story')?.id ?? '';
    } catch {
      if (alive && version === issueTypesVersion) issueTypesError = true;
    } finally {
      if (alive && version === issueTypesVersion) issueTypesLoading = false;
    }
  }

  async function loadSprints() {
    const requestedQuery = sprintQuery.trim();
    if (sprintsLoading && pendingSprintQuery === requestedQuery) return;
    pendingSprintQuery = requestedQuery;
    const version = ++sprintsVersion;
    sprintsLoading = true;
    sprintsError = false;
    try {
      const result = await (await xfetch(`${endpoint}/sprints?query=${encodeURIComponent(requestedQuery)}`)).json();
      if (!alive || version !== sprintsVersion) return;
      if (!Array.isArray(result.data)) throw new Error('Invalid sprints response');
      sprints = result.data;
    } catch {
      if (alive && version === sprintsVersion) sprintsError = true;
    } finally {
      if (alive && version === sprintsVersion) {
        sprintsLoading = false;
        pendingSprintQuery = null;
      }
    }
  }

  function sprintState(state: string) {
    if (state === 'active') return $LL.jiraImportUI.sprintActive();
    if (state === 'future') return $LL.jiraImportUI.sprintFuture();
    if (state === 'closed') return $LL.jiraImportUI.sprintClosed();
    return $LL.jiraImportUI.sprintUnknown();
  }

  onMount(() => {
    void loadIssueTypes();
    void loadSprints();
  });
  onDestroy(() => { alive = false; });
</script>

<div class="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-4">
  <div class="min-w-0">
    <label for="jira-issue-type" class="field-label">{$LL.jiraImportUI.issueType()}</label>
    <SelectInput id="jira-issue-type" bind:value={selectedIssueType} disabled={disabled || issueTypesLoading || issueTypesError}>
      <option value="">{$LL.jiraImportUI.allIssueTypes()}</option>
      {#each issueTypes as issueType (issueType.id)}
        <option value={issueType.id}>{issueType.name}</option>
      {/each}
    </SelectInput>
    {#if issueTypesLoading}
      <p class="field-hint" role="status">{$LL.jiraImportUI.loadingIssueTypes()}</p>
    {:else if issueTypesError}
      <p class="field-error" role="alert">{$LL.jiraImportUI.issueTypesError()}</p>
      <button type="button" class="retry-button" onclick={loadIssueTypes} disabled={disabled}>{$LL.jiraImportUI.retryIssueTypes()}</button>
    {:else if issueTypes.length === 0}
      <p class="field-hint">{$LL.jiraImportUI.noIssueTypes()}</p>
    {/if}
  </div>

  <div class="min-w-0">
    <label for="jira-sprint-query" class="field-label">{$LL.jiraImportUI.sprintSearchLabel()}</label>
    <div class="flex flex-col sm:flex-row gap-2 mb-3">
      <input
        id="jira-sprint-query"
        type="text"
        maxlength={200}
        class="filter-input min-w-0 flex-1"
        bind:value={sprintQuery}
        placeholder={$LL.jiraImportUI.sprintQueryPlaceholder()}
        disabled={disabled}
        onkeydown={event => {
          if (event.key === 'Enter') {
            event.preventDefault();
            void loadSprints();
          }
        }}
      />
      <button type="button" class="lookup-button" onclick={loadSprints} disabled={disabled || sprintsLoading} aria-busy={sprintsLoading}>
        {#if sprintsLoading}<LoaderCircle class="w-4 h-4 motion-safe:animate-spin shrink-0" aria-hidden="true" />{/if}
        {sprintsLoading ? $LL.jiraImportUI.loadingSprints() : $LL.jiraImportUI.findSprints()}
      </button>
    </div>
    <label for="jira-sprint" class="field-label">{$LL.jiraImportUI.sprint()}</label>
    <SelectInput
      id="jira-sprint"
      bind:value={selectedSprintId}
      disabled={disabled || sprintsLoading}
      onchange={event => {
        selectedSprintId = (event.currentTarget as HTMLSelectElement).value;
        selectedSprint = sprintOptions.find(sprint => String(sprint.id) === selectedSprintId);
      }}
    >
      <option value="">{$LL.jiraImportUI.allSprints()}</option>
      {#each sprintOptions as sprint (sprint.id)}
        <option value={String(sprint.id)}>{sprint.name} · {sprintState(sprint.state)}{sprint.boardName ? ` · ${sprint.boardName}` : ''} · #{sprint.id}</option>
      {/each}
    </SelectInput>
    {#if sprintsError}
      <p class="field-error" role="alert">{$LL.jiraImportUI.sprintsError()}</p>
      <button type="button" class="retry-button" onclick={loadSprints} disabled={disabled}>{$LL.jiraImportUI.retrySprints()}</button>
    {:else if !sprintsLoading && sprints.length === 0}
      <p class="field-hint">{$LL.jiraImportUI.noSprints()}</p>
    {:else}
      <p class="field-hint">{$LL.jiraImportUI.sprintsHint()}</p>
    {/if}
  </div>
</div>

<style lang="postcss">
  .field-label { @apply block mb-2 text-sm font-medium text-gray-900 dark:text-white; }
  .field-hint { @apply mt-2 text-sm text-gray-600 dark:text-gray-400; }
  .field-error { @apply mt-2 text-sm text-red-700 dark:text-red-300; }
  .filter-input { @apply block w-full px-3 py-2 text-sm text-gray-900 border border-gray-300 rounded bg-white dark:bg-gray-900 dark:border-gray-700 dark:text-white focus:outline-none focus:border-indigo-500; }
  .lookup-button { @apply inline-flex items-center justify-center gap-2 shrink-0 px-3 py-2 rounded border border-gray-300 dark:border-gray-600 text-sm text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 disabled:opacity-60 focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500; }
  .retry-button { @apply mt-1 text-sm underline text-blue-700 dark:text-blue-300 focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500; }
</style>
