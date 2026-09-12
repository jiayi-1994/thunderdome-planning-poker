<script lang="ts">
  import SolidButton from '../global/SolidButton.svelte';
  import Modal from '../global/Modal.svelte';
  import LL from '../../i18n/i18n-svelte';
  import { user } from '../../stores';
  import TextInput from '../forms/TextInput.svelte';
  import Checkbox from '../forms/Checkbox.svelte';
  import SelectInput from '../forms/SelectInput.svelte';

  import type { NotificationService } from '../../types/notifications';
  import { onMount } from 'svelte';
  import type { SessionUser } from '../../types/user';

  interface Props {
    handleCreate?: any;
    toggleClose?: any;
    xfetch?: any;
    notifications: NotificationService;
  }

  let { handleCreate = () => {}, toggleClose = () => {}, xfetch = () => {}, notifications }: Props = $props();

  let host = $state('');
  let client_mail = $state('');
  let access_token = $state('');

  let jira_data_center = $state(false);
  let auth_method = $state('pat');
  let submitting = $state(false);
  const passwordAuth = $derived(jira_data_center && auth_method === 'basic');

  function handleSubmit(event: Event) {
    event.preventDefault();
    if (submitting) return;
    host = host.trim();
    client_mail = client_mail.trim();

    if (host === '') {
      notifications.danger('Host field required');
      return false;
    }

    if (!/(http(s?)):\/\//i.test(host)) {
      notifications.danger('Host must contain protocol e.g. https:// or http://');
      return false;
    }

    if ((!jira_data_center || passwordAuth) && client_mail === '') {
      notifications.danger(passwordAuth ? 'Jira username is required' : 'Jira Cloud user email is required');
      return false;
    }

    if (!jira_data_center && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(client_mail)) {
      notifications.danger('Enter a valid Jira Cloud user email');
      return false;
    }

    if (access_token === '') {
      notifications.danger(passwordAuth ? 'Jira password is required' : 'Jira API token is required');
      return false;
    }

    const body = {
      host,
      client_mail,
      access_token,
      jira_data_center,
      auth_method: jira_data_center ? auth_method : 'basic',
    };

    submitting = true;
    xfetch(`/api/users/${$user.id}/jira-instances`, { body })
      .then((res: Response) => res.json())
      .then(function () {
        handleCreate();
        toggleClose();
      })
      .catch(function (error: any) {
        if (Array.isArray(error)) {
          return error[1].json().then(function (result: any) {
            if (result.error === 'REQUIRES_SUBSCRIBED_USER') {
              user.update({
                id: $user.id,
                name: $user.name,
                email: $user.email,
                rank: $user.rank,
                avatar: $user.avatar,
                verified: $user.verified,
                notificationsEnabled: $user.notificationsEnabled,
                locale: $user.locale,
                theme: $user.theme,
                subscribed: false,
              } as SessionUser);
              notifications.danger('subscription(s) expired');
            } else {
              notifications.danger(result.error || 'Failed to create Jira instance');
            }
          });
        } else {
          notifications.danger('failed to create jira instance');
        }
      })
      .catch(() => notifications.danger('Failed to create Jira instance. Please try again.'))
      .finally(() => {
        submitting = false;
      });
  }

  let focusInput: any;
  onMount(() => {
    focusInput?.focus();
  });
</script>

<Modal closeModal={toggleClose} ariaLabel={$LL.modalCreateJiraInstance()}>
  <form onsubmit={handleSubmit} name="createjirainstance">
    <div class="mb-4">
      <label class="block dark:text-gray-400 font-bold mb-2" for="host"> Host </label>
      <TextInput
        id="host"
        name="host"
        bind:value={host}
        bind:this={focusInput}
        placeholder="Enter the Jira Hostname..."
        required
      />
      <span class="text-sm dark:text-gray-400">
        {jira_data_center ? 'Example: https://jira.example.com (include /jira if used)' : 'Example: https://yourjira.atlassian.net'}
      </span>
    </div>
    <div class="mb-4">
      <Checkbox
        bind:checked={jira_data_center}
        id="jira_data_center"
        name="jira_data_center"
        label="Jira Server / Data Center (self-hosted)"
      />
    </div>
    {#if jira_data_center}
      <div class="mb-4">
        <label class="block dark:text-gray-400 font-bold mb-2" for="auth_method">Authentication</label>
        <SelectInput id="auth_method" name="auth_method" bind:value={auth_method}>
          <option value="pat">Personal access token (PAT)</option>
          <option value="basic">Username and password</option>
        </SelectInput>
        <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
          {passwordAuth
            ? 'Use your Jira login username and password. Supports older Jira Server versions such as 8.3.'
            : 'Native PAT support requires Jira 8.14 or later. For older versions, select username and password.'}
        </p>
      </div>
    {/if}
    <div class="mb-4">
      <label class="block dark:text-gray-400 font-bold mb-2" for="client_mail">
        {jira_data_center ? (passwordAuth ? 'Jira Username' : 'Jira Username (optional)') : 'Jira User Email'}
      </label>
      <TextInput
        id="client_mail"
        name="client_mail"
        bind:value={client_mail}
        placeholder={jira_data_center ? 'Enter your Jira username...' : 'Enter your Jira user email...'}
        required={!jira_data_center || passwordAuth}
        autocomplete="username"
      />
    </div>
    <div class="mb-4">
      <label class="block dark:text-gray-400 font-bold mb-2" for="access_token">
        {passwordAuth ? 'Jira Password' : jira_data_center ? 'Personal Access Token' : 'API Access Token'}
      </label>
      <TextInput
        id="access_token"
        name="access_token"
        type="password"
        autocomplete="off"
        bind:value={access_token}
        placeholder={passwordAuth ? 'Enter your Jira password...' : 'Enter your Jira API token...'}
        required
      />
    </div>
    <div class="text-right">
      <div>
        <SolidButton type="submit" disabled={submitting}>
          {submitting ? 'Creating...' : $LL.create()}
        </SolidButton>
      </div>
    </div>
  </form>
</Modal>
