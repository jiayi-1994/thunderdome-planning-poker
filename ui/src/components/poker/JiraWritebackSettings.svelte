<script lang="ts">
  import { onMount } from 'svelte';
  import Modal from '../global/Modal.svelte';
  import SolidButton from '../global/SolidButton.svelte';
  import HollowButton from '../global/HollowButton.svelte';
  import { appRoutes } from '../../config';
  import { jiraErrorMessage } from './jiraWriteback';
  import type { ApiClient } from '../../types/apiclient';
  import type { NotificationService } from '../../types/notifications';
  import type { PokerJiraSettings } from '../../types/poker';

  interface Props {
    gameId: string;
    xfetch: ApiClient;
    notifications: NotificationService;
    close: () => void;
  }
  let { gameId, xfetch, notifications, close }: Props = $props();
  let settings = $state<PokerJiraSettings>({ enabled: false, instanceId: '', fieldId: '', fieldName: '', host: '' });
  let instances = $state<Array<{ id: string; host: string }>>([]);
  let fields = $state<Array<{ id: string; name: string }>>([]);
  let loading = $state(true);
  let loaded = $state(false);
  let loadingFields = $state(false);
  let saving = $state(false);
  let errorMessage = $state('');
  let fieldsError = $state('');
  let fieldRequest = 0;
  let closed = false;
  let endpoint = $derived(`/api/battles/${gameId}/jira-writeback`);
  let ownsInstance = $derived(instances.some((instance) => instance.id === settings.instanceId));
  let canSave = $derived(
    loaded &&
      !loading &&
      !saving &&
      (!settings.enabled || (ownsInstance && fields.some((field) => field.id === settings.fieldId) && !loadingFields)),
  );

  async function loadFields(reset = false) {
    const request = ++fieldRequest;
    fields = [];
    fieldsError = '';
    if (reset) settings.fieldId = '';
    if (!ownsInstance) {
      loadingFields = false;
      return;
    }
    loadingFields = true;
    try {
      const res = await xfetch(`${endpoint}/fields?instanceId=${encodeURIComponent(settings.instanceId)}`);
      if (!res.ok) throw res;
      const result = await res.json();
      if (request !== fieldRequest || closed) return;
      fields = result.data;
      if (!fields.some((field) => field.id === settings.fieldId)) {
        const storyPoints = fields.filter((field) => field.name.trim().toLowerCase() === 'story points');
        settings.fieldId = storyPoints.length === 1 ? storyPoints[0].id : '';
      }
      if (fields.length === 0) fieldsError = '此 Jira 实例没有可用的数字字段，请先配置 Story Points 字段。';
    } catch (error) {
      const message = await jiraErrorMessage(error, '读取 Jira 字段失败，请重试。');
      if (request === fieldRequest && !closed) fieldsError = message;
    } finally {
      if (request === fieldRequest && !closed) loadingFields = false;
    }
  }

  async function loadSettings() {
    loading = true;
    errorMessage = '';
    try {
      const res = await xfetch(endpoint);
      if (!res.ok) throw res;
      const result = await res.json();
      if (closed) return;
      settings = result.data.settings;
      instances = result.data.instances;
      loaded = true;
      if (settings.instanceId) await loadFields();
    } catch (error) {
      errorMessage = await jiraErrorMessage(error, '读取 Jira 回写设置失败，请重试。');
    } finally {
      loading = false;
    }
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    if (!canSave) return;
    saving = true;
    errorMessage = '';
    try {
      const res = await xfetch(endpoint, {
        method: 'PUT',
        body: { enabled: settings.enabled, instanceId: settings.instanceId, fieldId: settings.fieldId },
      });
      if (!res.ok) throw res;
      notifications.success(settings.enabled ? '已开启保存评点后回写 Jira。' : '已关闭 Jira 回写。');
      close();
    } catch (error) {
      errorMessage = await jiraErrorMessage(error, '保存 Jira 回写设置失败，请重试。');
    } finally {
      saving = false;
    }
  }

  onMount(() => {
    void loadSettings();
    return () => {
      closed = true;
      fieldRequest++;
    };
  });
</script>

<Modal closeModal={close} ariaLabel="Jira 点数回写" widthClasses="md:w-2/3 lg:w-1/2 xl:w-2/5">
  <form onsubmit={save} class="space-y-5" data-testid="jira-writeback-settings">
    <div>
      <h2 class="text-2xl font-semibold text-gray-900 dark:text-white">Jira 点数回写</h2>
      <p class="mt-2 text-sm text-gray-600 dark:text-gray-300">
        评点结束后，点击评点结果下方的 Save，才将测试、前端、后端的平均分之和写入 Jira。
      </p>
    </div>
    {#if loading}
      <p role="status" class="text-gray-600 dark:text-gray-300">正在读取 Jira 设置…</p>
    {:else}
      <label class="flex items-center gap-3 font-semibold text-gray-800 dark:text-gray-100">
        <input type="checkbox" bind:checked={settings.enabled} disabled={saving} class="w-4 h-4 accent-blue-600" />
        点击 Save 保存评点后回写
      </label>
      {#if instances.length === 0}
        <p class="text-sm text-gray-600 dark:text-gray-300">
          尚未配置 Jira 账号。请先在<a href={appRoutes.profile} class="underline text-blue-700 dark:text-sky-400"
            >个人资料的 Jira 集成</a
          >中添加账号，再打开此设置。
        </p>
      {/if}
      {#if settings.enabled}
        {#if settings.instanceId && !ownsInstance}
          <p class="text-sm text-gray-600 dark:text-gray-300">
            当前回写使用另一位主持人的账号。可以关闭回写，或选择自己的 Jira 账号重新配置。
          </p>
        {/if}
        <div>
          <label for="jira-writeback-instance" class="block mb-2 font-semibold text-gray-800 dark:text-gray-200"
            >Jira 实例</label
          >
          <select
            id="jira-writeback-instance"
            bind:value={settings.instanceId}
            onchange={() => loadFields(true)}
            disabled={saving || instances.length === 0}
            class="block w-full rounded border border-gray-300 p-2 bg-white dark:bg-gray-900 dark:border-gray-600 dark:text-white"
          >
            <option value="" disabled>选择已配置的 Jira 实例</option>
            {#if settings.instanceId && !ownsInstance}<option value={settings.instanceId} disabled
                >{settings.host}（当前配置）</option
              >{/if}
            {#each instances as instance}<option value={instance.id}>{instance.host}</option>{/each}
          </select>
        </div>
        <div>
          <label for="jira-writeback-field" class="block mb-2 font-semibold text-gray-800 dark:text-gray-200"
            >点数字段</label
          >
          <select
            id="jira-writeback-field"
            bind:value={settings.fieldId}
            disabled={saving || loadingFields || fields.length === 0}
            class="block w-full rounded border border-gray-300 p-2 bg-white dark:bg-gray-900 dark:border-gray-600 dark:text-white"
          >
            <option value="" disabled>{loadingFields ? '正在读取字段…' : '选择 Story Points 字段'}</option>
            {#each fields as field}<option value={field.id}>{field.name}（{field.id}）</option>{/each}
          </select>
          <p class="mt-2 text-sm text-gray-600 dark:text-gray-300">
            默认选择 Story Points，回写 Jira 中的“预估”分数。
          </p>
          {#if fieldsError}
            <p role="alert" class="mt-2 text-sm text-red-700 dark:text-red-400">{fieldsError}</p>
            <button
              type="button"
              onclick={() => loadFields()}
              class="mt-1 text-sm underline text-blue-700 dark:text-sky-400">重新读取字段</button
            >
          {/if}
        </div>
        <p class="text-sm text-gray-600 dark:text-gray-300">
          需求须填写与此实例匹配的 Jira
          编号和链接。倒计时结束、手动结束和自动结束均不会直接回写；设置适用于之后点击 Save 保存的评点。
          没有有效评分时不回写，失败不会影响本地评点结果。
        </p>
      {/if}
    {/if}
    {#if errorMessage}<p role="alert" class="text-sm text-red-700 dark:text-red-400">{errorMessage}</p>{/if}
    <div class="flex justify-end gap-2">
      <HollowButton onClick={close} color="blue">取消</HollowButton>
      <SolidButton type="submit" disabled={!canSave} testid="jira-writeback-save"
        >{saving ? '正在保存…' : '保存设置'}</SolidButton
      >
    </div>
  </form>
</Modal>
