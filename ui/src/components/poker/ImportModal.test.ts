import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import { setLocale } from '../../i18n/i18n-svelte';
import { loadLocale } from '../../i18n/i18n-util.sync';
import ImportModal from './ImportModal.svelte';

vi.mock('../../stores', async () => {
  const { writable } = await import('svelte/store');
  return { user: writable({ id: 'user', subscribed: true }) };
});

const notifications = {
  success: vi.fn(),
  danger: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
  show: vi.fn(),
  removeToast: vi.fn(),
};

describe('Story import localization', () => {
  beforeEach(() => {
    loadLocale('zh');
    setLocale('zh');
  });

  afterEach(() => {
    loadLocale('en');
    setLocale('en');
  });

  it('shows Chinese import instructions and preserves the CSV field names when switching languages', async () => {
    render(ImportModal, {
      notifications,
      xfetch: async () => new Response(JSON.stringify({ data: [] })),
    });

    await expect.element(page.getByRole('dialog', { name: '导入评点需求' })).toBeVisible();
    await expect.element(page.getByRole('heading', { name: '导入需求', exact: true })).toBeVisible();
    await expect.element(page.getByRole('heading', { name: '从应用内导入' })).toBeVisible();
    await expect.element(page.getByRole('button', { name: '从评点会议导入' })).toBeVisible();
    await expect.element(page.getByRole('button', { name: '从故事地图导入' })).toBeVisible();
    await expect.element(page.getByRole('heading', { name: '从文件导入' })).toBeVisible();
    await expect.element(page.getByText('从 Jira 导出的 XML 文件中导入需求')).toBeVisible();
    await expect.element(page.getByText('CSV 文件须按以下顺序包含字段（表头可选）：')).toBeVisible();
    await expect
      .element(
        page.getByText('字段依次为：类型、标题、关联编号、链接、描述、验收标准。如包含表头，请使用上方的英文字段名。'),
      )
      .toBeVisible();
    const csvFields = page.getByText('Type,Title,ReferenceId,Link,Description,AcceptanceCriteria', { exact: true });
    await expect.element(csvFields).toBeVisible();

    loadLocale('en');
    setLocale('en');

    await expect.element(page.getByRole('dialog', { name: 'Import Poker Stories' })).toBeVisible();
    await expect.element(page.getByRole('heading', { name: 'Import Stories', exact: true })).toBeVisible();
    await expect.element(page.getByRole('heading', { name: 'Internal Import' })).toBeVisible();
    await expect.element(page.getByRole('button', { name: 'Import from Game' })).toBeVisible();
    await expect.element(page.getByRole('button', { name: 'Import from Storyboard' })).toBeVisible();
    await expect.element(page.getByRole('heading', { name: 'File Import' })).toBeVisible();
    await expect
      .element(page.getByText('CSV files must include these fields in order (header row optional):'))
      .toBeVisible();
    await expect.element(csvFields).toBeVisible();
  });
});
