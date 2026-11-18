import { afterEach, describe, expect, it, vi } from 'vitest';
import Cookies from 'js-cookie';
import { user } from './stores';
import type { SessionUser } from './types/user';

vi.mock('./config', () => ({
  AppConfig: { CookieName: 'session-profile-test', PathPrefix: '' },
  rtlLanguages: [],
}));

const account: SessionUser = {
  id: 'account',
  name: 'Session test',
  rank: 'REGISTERED',
  locale: 'zh',
  lastActive: '',
  updatedDate: '',
  subscribed: false,
};

afterEach(() => {
  vi.restoreAllMocks();
  user.delete();
});

describe('login cookie renewal', () => {
  it('refreshes the browser profile after the server confirms the same account', async () => {
    user.create(account);
    const write = vi.spyOn(Cookies, 'set');
    const request = vi.spyOn(window, 'fetch').mockResolvedValue(Response.json({ data: { id: account.id } }));

    await user.renewSession();

    expect(request).toHaveBeenCalledExactlyOnceWith('/api/auth/user', { cache: 'no-store' });
    expect(write).toHaveBeenCalledExactlyOnceWith('session-profile-test', JSON.stringify(account), {
      expires: 400,
      SameSite: 'strict',
      path: '/',
    });
  });

  it('does not refresh the browser profile for a rejected login', async () => {
    user.create(account);
    const write = vi.spyOn(Cookies, 'set');
    vi.spyOn(window, 'fetch').mockResolvedValue(new Response(null, { status: 401 }));

    await user.renewSession();

    expect(write).not.toHaveBeenCalled();
  });

  it('keeps guest cookies on their existing lifetime without making a renewal request', async () => {
    const request = vi.spyOn(window, 'fetch');
    const write = vi.spyOn(Cookies, 'set');
    user.create({ ...account, rank: 'GUEST' });

    await user.renewSession();

    expect(request).not.toHaveBeenCalled();
    expect(write).toHaveBeenCalledExactlyOnceWith('session-profile-test', expect.any(String), {
      expires: 365,
      SameSite: 'strict',
      path: '/',
    });
  });
});
