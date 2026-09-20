<script lang="ts">
  import { onMount } from 'svelte';
  import { AppConfig } from '../../config';
  import LL from '../../i18n/i18n-svelte';
  import { claimRoundNotification, showTabVoteReminder } from './voteReminderBrowser';

  interface Props {
    roundKey: string;
    remainingSeconds: number;
    eligible: boolean;
    enabled?: boolean;
  }

  let { roundKey, remainingSeconds, eligible, enabled = true }: Props = $props();
  let mounted = $state(false);
  let secureContext = $state(false);
  let supported = $state(false);
  let background = $state(false);
  let permission = $state<NotificationPermission>('default');
  let requestingPermission = $state(false);
  let permissionFailed = $state(false);
  let desktopNotification: Notification | undefined;

  const iconPath = `${String(AppConfig.PathPrefix || '').replace(/\/$/, '')}/img/vote-reminder.svg`;
  let active = $derived(enabled && eligible && roundKey !== '' && remainingSeconds > 0 && remainingSeconds <= 30);

  function closeDesktopNotification(notification = desktopNotification) {
    try {
      notification?.close();
    } catch {
      // A browser notification failure must not interrupt the meeting.
    }
    if (desktopNotification === notification) desktopNotification = undefined;
  }

  function syncBrowserState() {
    background = document.hidden || !document.hasFocus();
    if (supported) permission = Notification.permission;
  }

  async function enableDesktopNotifications() {
    if (!mounted || !secureContext || !supported || requestingPermission) return;
    requestingPermission = true;
    permissionFailed = false;
    try {
      const result = await Notification.requestPermission();
      if (mounted) permission = result;
    } catch {
      if (mounted) permissionFailed = true;
    } finally {
      if (mounted) requestingPermission = false;
    }
  }

  onMount(() => {
    mounted = true;
    secureContext = window.isSecureContext;
    supported = typeof Notification !== 'undefined';
    syncBrowserState();
    document.addEventListener('visibilitychange', syncBrowserState);
    window.addEventListener('focus', syncBrowserState);
    window.addEventListener('blur', syncBrowserState);
    return () => {
      mounted = false;
      document.removeEventListener('visibilitychange', syncBrowserState);
      window.removeEventListener('focus', syncBrowserState);
      window.removeEventListener('blur', syncBrowserState);
      closeDesktopNotification();
    };
  });

  $effect(() => {
    if (!mounted || !active) return;
    return showTabVoteReminder($LL.voteReminderUI.title(), iconPath);
  });

  $effect(() => {
    if (!mounted || !active || !roundKey) return;
    return closeDesktopNotification;
  });

  $effect(() => {
    if (!mounted || !active || !secureContext || !supported || permission !== 'granted' || !background) return;
    if (!claimRoundNotification(roundKey)) return;
    try {
      const notification = new Notification($LL.voteReminderUI.notificationTitle(), {
        body: $LL.voteReminderUI.notificationBody(),
        icon: iconPath,
        tag: 'thunderdome-vote-reminder',
      });
      desktopNotification = notification;
      notification.onclick = () => {
        try {
          window.focus();
        } catch {
          // Browsers may decline a focus request.
        } finally {
          closeDesktopNotification(notification);
        }
      };
    } catch {
      // Some mobile browsers require push notifications; the tab reminder remains available.
    }
  });
</script>

{#if enabled}
  <div class="mt-2 space-y-2 text-sm">
    {#if active}
      <p
        role="alert"
        class="rounded-md border border-red-300 bg-red-50 px-3 py-2 font-semibold text-red-800 dark:border-red-700 dark:bg-red-950 dark:text-red-200"
      >
        <span aria-hidden="true">●</span>
        {$LL.voteReminderUI.warning()}
      </p>
    {/if}
    {#if mounted}
      {#if !secureContext}
        <p class="text-xs text-gray-600 dark:text-gray-300">{$LL.voteReminderUI.httpsRequired()}</p>
      {:else if !supported}
        <p class="text-xs text-gray-600 dark:text-gray-300">{$LL.voteReminderUI.unsupported()}</p>
      {:else if permission === 'granted'}
        <p class="text-xs text-gray-600 dark:text-gray-300">{$LL.voteReminderUI.desktopEnabled()}</p>
      {:else if permission === 'denied'}
        <p class="text-xs text-gray-600 dark:text-gray-300">{$LL.voteReminderUI.permissionDenied()}</p>
      {:else}
        <button
          type="button"
          onclick={enableDesktopNotifications}
          disabled={requestingPermission}
          class="rounded border border-gray-300 px-2 py-1 text-xs text-gray-700 hover:bg-gray-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-600 disabled:opacity-60 dark:border-gray-600 dark:text-gray-200 dark:hover:bg-gray-800"
        >
          {requestingPermission ? $LL.voteReminderUI.requestingPermission() : $LL.voteReminderUI.enableDesktop()}
        </button>
      {/if}
      {#if permissionFailed}
        <p role="status" class="text-xs text-red-700 dark:text-red-300">{$LL.voteReminderUI.permissionFailed()}</p>
      {/if}
    {/if}
  </div>
{/if}
