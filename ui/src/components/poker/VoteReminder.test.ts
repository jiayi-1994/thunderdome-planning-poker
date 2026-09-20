import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { render, cleanup } from 'vitest-browser-svelte';
import { tick } from 'svelte';
import { get } from 'svelte/store';
import LL from '../../i18n/i18n-svelte';
import { AppConfig } from '../../config';
import VoteReminder from './VoteReminder.svelte';

class FakeNotification {
  static permission: NotificationPermission = 'default';
  static requestPermission = vi.fn(async (): Promise<NotificationPermission> => {
    FakeNotification.permission = 'granted';
    return 'granted';
  });
  static instances: FakeNotification[] = [];
  static failConstruction = false;
  title: string;
  options: NotificationOptions;
  onclick: (() => void) | null = null;
  close = vi.fn();

  constructor(title: string, options: NotificationOptions = {}) {
    if (FakeNotification.failConstruction) throw new Error('Notification unavailable');
    this.title = title;
    this.options = options;
    FakeNotification.instances.push(this);
  }
}

let nextRound = 0;
const props = () => ({
  roundKey: `vote-reminder-test-${++nextRound}`,
  remainingSeconds: 31,
  eligible: true,
});
const labels = () => get(LL).voteReminderUI;
const alert = () => page.getByRole('alert');
const permissionButton = () => page.getByRole('button', { name: labels().enableDesktop(), exact: true });
let originalTitle: string;
let originalPrefix: string;
const fixtureIcons: HTMLLinkElement[] = [];

async function setBackground(background: boolean) {
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(background);
  vi.spyOn(document, 'hasFocus').mockReturnValue(!background);
  document.dispatchEvent(new Event('visibilitychange'));
  window.dispatchEvent(new Event(background ? 'blur' : 'focus'));
  await tick();
}

function addOriginalIcon() {
  const icon = document.createElement('link');
  icon.rel = 'icon';
  icon.setAttribute('href', '/original.png');
  icon.type = 'image/png';
  icon.sizes.value = '32x32';
  document.head.appendChild(icon);
  fixtureIcons.push(icon);
  return icon;
}

beforeEach(() => {
  originalTitle = document.title;
  document.title = 'Planning room';
  originalPrefix = AppConfig.PathPrefix;
  AppConfig.PathPrefix = '';
  vi.stubGlobal('isSecureContext', true);
  vi.stubGlobal('Notification', FakeNotification);
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false);
  vi.spyOn(document, 'hasFocus').mockReturnValue(true);
  FakeNotification.permission = 'default';
  FakeNotification.instances = [];
  FakeNotification.failConstruction = false;
  FakeNotification.requestPermission.mockReset().mockImplementation(async () => {
    FakeNotification.permission = 'granted';
    return 'granted';
  });
  sessionStorage.removeItem('thunderdome.vote-reminder.last-round');
});

afterEach(async () => {
  await cleanup();
  for (const icon of fixtureIcons.splice(0)) icon.remove();
  document.title = originalTitle;
  AppConfig.PathPrefix = originalPrefix;
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('personal voting reminder', () => {
  it('starts at 30 seconds and restores the tab when voting finishes', async () => {
    const icon = addOriginalIcon();
    const { rerender } = render(VoteReminder, props());
    await expect.element(alert()).not.toBeInTheDocument();
    expect(document.title).toBe('Planning room');
    await rerender({ remainingSeconds: 30 });
    await expect.element(alert()).toHaveTextContent(labels().warning());
    expect(document.title).toBe(`🔴 ${labels().title()} · Planning room`);
    expect(icon.getAttribute('href')).toBe('/img/vote-reminder.svg');
    expect(icon.type).toBe('image/svg+xml');
    const alertElement = document.querySelector('[role="alert"]');
    await rerender({ remainingSeconds: 29 });
    expect(document.querySelector('[role="alert"]')).toBe(alertElement);
    expect(alertElement?.textContent).toContain(labels().warning());
    await rerender({ eligible: false });
    await expect.element(alert()).not.toBeInTheDocument();
    expect(document.title).toBe('Planning room');
    expect(icon.getAttribute('href')).toBe('/original.png');
    expect(icon.type).toBe('image/png');
    expect(icon.sizes.value).toBe('32x32');
  });

  it('handles skipped countdown ticks and clears immediately at expiry', async () => {
    const { rerender } = render(VoteReminder, props());
    await rerender({ remainingSeconds: 25 });
    await expect.element(alert()).toBeInTheDocument();
    await rerender({ remainingSeconds: 0 });
    await expect.element(alert()).not.toBeInTheDocument();
    expect(document.title).toBe('Planning room');
  });

  it('never reminds already-voted, disabled, or inactive users', async () => {
    const { rerender } = render(VoteReminder, {
      ...props(),
      remainingSeconds: 20,
      eligible: false,
    });
    await expect.element(alert()).not.toBeInTheDocument();
    await rerender({ eligible: true, enabled: false });
    await expect.element(alert()).not.toBeInTheDocument();
    await expect.element(permissionButton()).not.toBeInTheDocument();
    await rerender({ enabled: true, roundKey: '' });
    await expect.element(alert()).not.toBeInTheDocument();
    expect(document.title).toBe('Planning room');
  });

  it('uses the configured path prefix and removes an icon it created on unmount', async () => {
    AppConfig.PathPrefix = '/poker/';
    const before = Array.from(document.head.querySelectorAll('link[rel~="icon"]'));
    const { unmount } = render(VoteReminder, {
      ...props(),
      remainingSeconds: 20,
    });
    await expect.element(alert()).toBeInTheDocument();
    expect(document.head.querySelector('link[rel~="icon"]')?.getAttribute('href')).toBe('/poker/img/vote-reminder.svg');
    await unmount();
    expect(document.title).toBe('Planning room');
    expect(Array.from(document.head.querySelectorAll('link[rel~="icon"]'))).toEqual(before);
  });

  it('does not overwrite a newer navigation title or favicon while cleaning up', async () => {
    const icon = addOriginalIcon();
    const { unmount } = render(VoteReminder, {
      ...props(),
      remainingSeconds: 20,
    });
    await expect.element(alert()).toBeInTheDocument();
    document.title = 'Dashboard';
    icon.setAttribute('href', '/dashboard.ico');
    icon.type = 'image/x-icon';
    await unmount();
    expect(document.title).toBe('Dashboard');
    expect(icon.getAttribute('href')).toBe('/dashboard.ico');
    expect(icon.type).toBe('image/x-icon');
  });

  it('keeps the tab reminder on HTTP and never requests desktop permission automatically', async () => {
    vi.stubGlobal('isSecureContext', false);
    FakeNotification.permission = 'granted';
    await setBackground(true);
    render(VoteReminder, { ...props(), remainingSeconds: 20 });
    await expect.element(alert()).toBeInTheDocument();
    await expect.element(page.getByText(labels().httpsRequired())).toBeInTheDocument();
    await expect.element(permissionButton()).not.toBeInTheDocument();
    expect(FakeNotification.instances).toHaveLength(0);
    expect(FakeNotification.requestPermission).not.toHaveBeenCalled();
  });

  it('reports missing desktop notification support without affecting the tab reminder', async () => {
    vi.stubGlobal('Notification', undefined);
    render(VoteReminder, { ...props(), remainingSeconds: 20 });
    await expect.element(alert()).toBeInTheDocument();
    await expect.element(page.getByText(labels().unsupported())).toBeInTheDocument();
    await expect.element(permissionButton()).not.toBeInTheDocument();
  });

  it('asks for desktop permission only after the user clicks the enable button', async () => {
    render(VoteReminder, { ...props(), remainingSeconds: 20 });
    await expect.element(permissionButton()).toBeInTheDocument();
    expect(FakeNotification.requestPermission).not.toHaveBeenCalled();
    await permissionButton().click();
    await expect.element(page.getByText(labels().desktopEnabled())).toBeInTheDocument();
    expect(FakeNotification.requestPermission).toHaveBeenCalledTimes(1);
    expect(FakeNotification.instances).toHaveLength(0);
  });

  it('does not prompt or notify in the background before permission is granted', async () => {
    await setBackground(true);
    render(VoteReminder, { ...props(), remainingSeconds: 20 });
    await expect.element(alert()).toBeInTheDocument();
    expect(FakeNotification.requestPermission).not.toHaveBeenCalled();
    expect(FakeNotification.instances).toHaveLength(0);
  });

  it('ignores permission results after leaving the meeting', async () => {
    let finishPermission!: (permission: NotificationPermission) => void;
    FakeNotification.requestPermission.mockImplementationOnce(
      () =>
        new Promise(resolve => {
          finishPermission = resolve;
        }),
    );
    await setBackground(true);
    const { unmount } = render(VoteReminder, { ...props(), remainingSeconds: 20 });
    await permissionButton().click();
    await expect.element(page.getByRole('button', { name: labels().requestingPermission() })).toBeDisabled();
    await unmount();
    finishPermission('granted');
    await tick();
    expect(FakeNotification.instances).toHaveLength(0);
    expect(document.title).toBe('Planning room');
  });

  it('leaves denied permissions alone and shows browser settings guidance', async () => {
    FakeNotification.permission = 'denied';
    render(VoteReminder, { ...props(), remainingSeconds: 20 });
    await expect.element(page.getByText(labels().permissionDenied())).toBeInTheDocument();
    await expect.element(permissionButton()).not.toBeInTheDocument();
    expect(FakeNotification.requestPermission).not.toHaveBeenCalled();
    await expect.element(alert()).toBeInTheDocument();
  });

  it('handles a rejected permission request and allows retrying', async () => {
    FakeNotification.requestPermission.mockRejectedValueOnce(new Error('Permission request failed'));
    render(VoteReminder, { ...props(), remainingSeconds: 20 });
    await permissionButton().click();
    await expect.element(page.getByRole('status')).toHaveTextContent(labels().permissionFailed());
    await expect.element(alert()).toBeInTheDocument();
    await permissionButton().click();
    await expect.element(page.getByText(labels().desktopEnabled())).toBeInTheDocument();
    await expect.element(page.getByRole('status')).not.toBeInTheDocument();
  });

  it('notifies only once when an unvoted user switches to another window in the final 30 seconds', async () => {
    FakeNotification.permission = 'granted';
    const { rerender } = render(VoteReminder, props());
    await rerender({ remainingSeconds: 30 });
    expect(FakeNotification.instances).toHaveLength(0);
    await setBackground(true);
    expect(FakeNotification.instances).toHaveLength(1);
    expect(FakeNotification.instances[0].title).toBe(labels().notificationTitle());
    expect(FakeNotification.instances[0].options.body).toBe(labels().notificationBody());
    await rerender({ remainingSeconds: 15 });
    await setBackground(false);
    await setBackground(true);
    expect(FakeNotification.instances).toHaveLength(1);
    await rerender({ eligible: false });
    expect(FakeNotification.instances[0].close).toHaveBeenCalledTimes(1);
  });

  it('notifies when the tab remains visible but another application has focus', async () => {
    FakeNotification.permission = 'granted';
    render(VoteReminder, { ...props(), remainingSeconds: 25 });
    await expect.element(alert()).toBeInTheDocument();
    vi.spyOn(document, 'hasFocus').mockReturnValue(false);
    window.dispatchEvent(new Event('blur'));
    await tick();
    expect(document.hidden).toBe(false);
    expect(FakeNotification.instances).toHaveLength(1);
  });

  it('closes old notifications at a new round and allows the next round to notify', async () => {
    FakeNotification.permission = 'granted';
    await setBackground(true);
    const { rerender, unmount } = render(VoteReminder, {
      ...props(),
      remainingSeconds: 25,
    });
    await expect.element(alert()).toBeInTheDocument();
    expect(FakeNotification.instances).toHaveLength(1);
    await rerender({ roundKey: props().roundKey, remainingSeconds: 120 });
    expect(FakeNotification.instances[0].close).toHaveBeenCalledTimes(1);
    await rerender({ remainingSeconds: 20 });
    expect(FakeNotification.instances).toHaveLength(2);
    await unmount();
    expect(FakeNotification.instances[1].close).toHaveBeenCalledTimes(1);
  });

  it('closes notifications on expiry and on disabling reminders', async () => {
    FakeNotification.permission = 'granted';
    await setBackground(true);
    const { rerender } = render(VoteReminder, {
      ...props(),
      remainingSeconds: 25,
    });
    await expect.element(alert()).toBeInTheDocument();
    await rerender({ remainingSeconds: 0 });
    expect(FakeNotification.instances[0].close).toHaveBeenCalledTimes(1);
    await rerender({ roundKey: props().roundKey, remainingSeconds: 20 });
    expect(FakeNotification.instances).toHaveLength(2);
    await rerender({ enabled: false });
    expect(FakeNotification.instances[1].close).toHaveBeenCalledTimes(1);
    expect(document.title).toBe('Planning room');
  });

  it('does not notify again when the same round reconnects or remounts', async () => {
    FakeNotification.permission = 'granted';
    await setBackground(true);
    const round = props();
    const { rerender, unmount } = render(VoteReminder, {
      ...round,
      remainingSeconds: 25,
    });
    await expect.element(alert()).toBeInTheDocument();
    await rerender({ eligible: false });
    await rerender({ eligible: true });
    await unmount();
    render(VoteReminder, { ...round, remainingSeconds: 20 });
    await expect.element(alert()).toBeInTheDocument();
    expect(FakeNotification.instances).toHaveLength(1);
  });

  it('honors a session-stored notification after a page reload', async () => {
    FakeNotification.permission = 'granted';
    await setBackground(true);
    const round = props();
    sessionStorage.setItem('thunderdome.vote-reminder.last-round', round.roundKey);
    render(VoteReminder, { ...round, remainingSeconds: 25 });
    await expect.element(alert()).toBeInTheDocument();
    expect(FakeNotification.instances).toHaveLength(0);
  });

  it('falls back to in-memory deduplication when session storage is blocked', async () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('Storage denied');
    });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('Storage denied');
    });
    FakeNotification.permission = 'granted';
    await setBackground(true);
    const { rerender } = render(VoteReminder, {
      ...props(),
      remainingSeconds: 25,
    });
    await expect.element(alert()).toBeInTheDocument();
    await rerender({ eligible: false });
    await rerender({ eligible: true });
    expect(FakeNotification.instances).toHaveLength(1);
  });

  it('focuses the meeting and closes the notification when it is clicked', async () => {
    FakeNotification.permission = 'granted';
    const focus = vi.spyOn(window, 'focus').mockImplementation(() => {});
    await setBackground(true);
    render(VoteReminder, { ...props(), remainingSeconds: 25 });
    await expect.element(alert()).toBeInTheDocument();
    FakeNotification.instances[0].onclick?.();
    expect(focus).toHaveBeenCalledTimes(1);
    expect(FakeNotification.instances[0].close).toHaveBeenCalledTimes(1);
  });

  it('keeps the tab reminder if desktop notification construction fails', async () => {
    FakeNotification.permission = 'granted';
    FakeNotification.failConstruction = true;
    await setBackground(true);
    const { rerender } = render(VoteReminder, {
      ...props(),
      remainingSeconds: 25,
    });
    await expect.element(alert()).toBeInTheDocument();
    expect(document.title).toContain('🔴');
    FakeNotification.failConstruction = false;
    await rerender({ remainingSeconds: 20 });
    await setBackground(false);
    await setBackground(true);
    expect(FakeNotification.instances).toHaveLength(0);
    await rerender({ eligible: false });
    expect(document.title).toBe('Planning room');
  });
});
