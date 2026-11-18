import { describe, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render } from 'vitest-browser-svelte';
import PokerRoleSelector from './PokerRoleSelector.svelte';
import { readPokerRole, roleForRound, savePokerRole } from './pokerRole';

describe('poker role selection', () => {
  it('requires an explicit initial choice and permits changing only before voting', async () => {
    const onSelect = vi.fn();
    const { rerender } = render(PokerRoleSelector, { category: null, onSelect });
    for (const name of ['测试', '开发']) {
      await expect.element(page.getByRole('button', { name, exact: true })).toHaveAttribute('aria-pressed', 'false');
    }
    await expect.element(page.getByRole('button', { name: '前端开发', exact: true })).not.toBeInTheDocument();
    await expect.element(page.getByRole('button', { name: '后端开发', exact: true })).not.toBeInTheDocument();
    await page.getByRole('button', { name: '开发', exact: true }).click();
    expect(onSelect).toHaveBeenCalledWith('development');
    await rerender({ category: 'development', locked: true });
    await expect.element(page.getByRole('button', { name: '更换身份' })).toBeDisabled();
    await rerender({ locked: false });
    await page.getByRole('button', { name: '更换身份' }).click();
    await page.getByRole('button', { name: '测试', exact: true }).click();
    expect(onSelect).toHaveBeenLastCalledWith('testing');
  });

  it('does not allow an open role chooser to bypass a newly received ballot', async () => {
    const { rerender } = render(PokerRoleSelector, { category: 'development', onSelect: vi.fn() });
    await page.getByRole('button', { name: '更换身份' }).click();
    await rerender({ locked: true });
    await expect.element(page.getByRole('button', { name: '测试', exact: true })).not.toBeInTheDocument();
    await expect.element(page.getByRole('button', { name: '更换身份' })).toBeDisabled();
  });

  it('isolates saved choices by game and user and restores the role of an existing ballot', () => {
    const game = `role-test-${crypto.randomUUID()}`;
    expect(readPokerRole(game, 'one')).toBeNull();
    savePokerRole(game, 'one', 'development');
    expect(readPokerRole(game, 'one')).toBe('development');
    expect(readPokerRole(game, 'two')).toBeNull();
    expect(readPokerRole(`${game}-other`, 'one')).toBeNull();
    expect(roleForRound([{ warriorId: 'one', category: 'testing', vote: '0' }], 'one', 'development')).toBe('testing');
    expect(roleForRound([], 'one', 'development')).toBe('development');
    for (const category of ['frontend', 'backend'] as const) {
      localStorage.setItem(`poker-role:${game}:one`, category);
      expect(readPokerRole(game, 'one')).toBe('development');
      expect(roleForRound([{ warriorId: 'one', category, vote: '0' }], 'one', 'testing')).toBe('development');
    }
    localStorage.removeItem(`poker-role:${game}:one`);
  });
});
