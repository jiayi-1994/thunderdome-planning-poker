import { describe, it, expect, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import TestJiraConnection from './TestJiraConnection.svelte';

describe('Saved Jira connection test', () => {
  it('shows the authenticated account without resending credentials', async () => {
    const xfetch = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ data: { connected: true, display_name: 'Test User' } })));
    render(TestJiraConnection, { instanceId: 'instance-1', xfetch });
    await page.getByRole('button', { name: 'Test connection' }).click();
    await expect.element(page.getByText('Connected: Test User')).toBeVisible();
    expect(xfetch.mock.calls[0][0]).toContain('/jira-instances/instance-1/test');
    expect(xfetch.mock.calls[0][1]).toEqual({ method: 'POST' });
  });

  it('keeps the specific failure visible and permits retry', async () => {
    const xfetch = vi
      .fn()
      .mockRejectedValue([422, new Response(JSON.stringify({ error: 'Jira authentication failed (401)' }))]);
    render(TestJiraConnection, { instanceId: 'instance-1', xfetch });
    await page.getByRole('button', { name: 'Test connection' }).click();
    await expect.element(page.getByText('Jira authentication failed (401)')).toBeVisible();
    await expect.element(page.getByRole('button', { name: 'Test connection' })).toBeEnabled();
  });

  it('disables repeated tests while a request is pending', async () => {
    let finish: (response: Response) => void = () => {};
    const xfetch = vi.fn(
      () =>
        new Promise<Response>(resolve => {
          finish = resolve;
        }),
    );
    render(TestJiraConnection, { instanceId: 'instance-1', xfetch });
    await page.getByRole('button', { name: 'Test connection' }).click();
    await expect.element(page.getByRole('button', { name: 'Testing...' })).toBeDisabled();
    expect(xfetch).toHaveBeenCalledTimes(1);
    finish(new Response(JSON.stringify({ data: { connected: true } })));
    await expect.element(page.getByText('Connection verified')).toBeVisible();
  });
});
