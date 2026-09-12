import { describe, it, expect, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import CreateJiraInstance from './CreateJiraInstance.svelte';

const setup = (xfetch = vi.fn().mockResolvedValue(new Response('{}'))) => {
  const notifications = {
    show: vi.fn(),
    success: vi.fn(),
    danger: vi.fn(),
    warning: vi.fn(),
    info: vi.fn(),
    removeToast: vi.fn(),
  };
  const handleCreate = vi.fn();
  const toggleClose = vi.fn();
  render(CreateJiraInstance, { xfetch, notifications, handleCreate, toggleClose });
  return { xfetch, notifications, handleCreate, toggleClose };
};

describe('Jira connection authentication', () => {
  it('supports a non-email Jira Server username and hides the password', async () => {
    const { xfetch, handleCreate } = setup();
    await page.getByLabelText('Host', { exact: true }).fill('http://jira.example.com');
    await page.getByRole('checkbox').click();
    await page.getByLabelText('Authentication', { exact: true }).selectOptions('basic');
    await page.getByLabelText('Jira Username', { exact: true }).fill('jira.user');
    await page.getByLabelText('Jira Password').fill('test-password');
    await expect.element(page.getByLabelText('Jira Password')).toHaveAttribute('type', 'password');
    await page.getByRole('button', { name: 'Create', exact: true }).click();
    await expect.poll(() => handleCreate.mock.calls.length).toBe(1);
    expect(xfetch.mock.calls[0][1].body).toEqual({
      host: 'http://jira.example.com',
      client_mail: 'jira.user',
      access_token: 'test-password',
      jira_data_center: true,
      auth_method: 'basic',
    });
  });

  it('accepts a PAT without an unused username', async () => {
    const { xfetch, handleCreate } = setup();
    await page.getByLabelText('Host', { exact: true }).fill('https://jira.example.com');
    await page.getByRole('checkbox').click();
    await page.getByLabelText('Personal Access Token', { exact: true }).fill('test-token');
    await page.getByRole('button', { name: 'Create', exact: true }).click();
    await expect.poll(() => handleCreate.mock.calls.length).toBe(1);
    expect(xfetch.mock.calls[0][1].body.auth_method).toBe('pat');
    expect(xfetch.mock.calls[0][1].body.client_mail).toBe('');
  });

  it('still requires an email for Jira Cloud', async () => {
    const { xfetch, notifications } = setup();
    await page.getByLabelText('Host', { exact: true }).fill('https://example.atlassian.net');
    await page.getByLabelText('Jira User Email').fill('jira.user');
    await page.getByLabelText('API Access Token', { exact: true }).fill('test-token');
    await page.getByRole('button', { name: 'Create', exact: true }).click();
    expect(xfetch).not.toHaveBeenCalled();
    expect(notifications.danger).toHaveBeenCalledWith('Enter a valid Jira Cloud user email');
  });

  it('shows the API validation error and keeps the form open', async () => {
    const { notifications, toggleClose } = setup(
      vi.fn().mockRejectedValue([400, new Response(JSON.stringify({ error: 'Enter a valid Jira Cloud user email' }))]),
    );
    await page.getByLabelText('Host', { exact: true }).fill('https://example.atlassian.net');
    await page.getByLabelText('Jira User Email').fill('jira@example.com');
    await page.getByLabelText('API Access Token', { exact: true }).fill('test-token');
    await page.getByRole('button', { name: 'Create', exact: true }).click();
    await expect.poll(() => notifications.danger.mock.calls.length).toBe(1);
    expect(notifications.danger).toHaveBeenCalledWith('Enter a valid Jira Cloud user email');
    expect(toggleClose).not.toHaveBeenCalled();
    await expect.element(page.getByRole('button', { name: 'Create', exact: true })).toBeEnabled();
  });
});
