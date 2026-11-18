import { derived, get, writable } from 'svelte/store';
import Cookies from 'js-cookie';
import { AppConfig, rtlLanguages } from './config';
import { locale } from './i18n/i18n-svelte';
import type { GlobalAlert } from './types/global-alerts';
import type { SessionUser } from './types/user';

const { PathPrefix, CookieName } = AppConfig;
const cookiePath = `${PathPrefix}/`;

declare global {
  let ActiveAlerts: any;
}

function initWarrior() {
  const store = writable(JSON.parse(Cookies.get(CookieName) || '{}'));

  function writeWarriorCookie(warrior: SessionUser) {
    Cookies.set(CookieName, JSON.stringify(warrior), {
      expires: warrior.rank === 'GUEST' ? 365 : 400,
      SameSite: 'strict',
      path: cookiePath,
    });
  }

  function saveWarrior(warrior: SessionUser) {
    writeWarriorCookie(warrior);
    store.set(warrior);
  }

  return {
    subscribe: store.subscribe,
    create: saveWarrior,
    update: saveWarrior,
    renewSession: async () => {
      const { id, rank } = get(store);
      if (!id || rank === 'GUEST') return;

      try {
        const response = await fetch(`${PathPrefix}/api/auth/user`, { cache: 'no-store' });
        if (!response.ok) return;

        const { data } = await response.json();
        const current = get(store);
        if (data?.id === id && current.id === id) writeWarriorCookie(current);
      } catch {
        // A failed background renewal leaves the current login state available.
      }
    },
    delete: () => {
      Cookies.remove(CookieName, { path: cookiePath });
      store.set({});
    },
  };
}

export const user = initWarrior();

function initActiveAlerts() {
  const activeAlerts = typeof ActiveAlerts != 'undefined' ? ActiveAlerts : [];
  const { subscribe, update } = writable(activeAlerts);

  return {
    subscribe,
    update: (alerts: GlobalAlert[]) => {
      update(a => (a = alerts));
    },
  };
}

export const activeAlerts = initActiveAlerts();

function initDismissedAlerts() {
  const dismissKey = 'dismissed_alerts';
  const dismissedAlerts = JSON.parse(localStorage.getItem(dismissKey) || '[]') as string[];
  const { subscribe, update } = writable(dismissedAlerts);

  return {
    subscribe,
    dismiss: (actives: string[], dismisses: string[]) => {
      // Only store valid alert IDs
      const validAlerts = actives;
      let alertsToDismiss = [...dismisses.filter(id => validAlerts.includes(id))];
      localStorage.setItem(dismissKey, JSON.stringify(alertsToDismiss));
      update((a: any) => (a = alertsToDismiss));
    },
  };
}

export const dir = derived(locale, $locale => (rtlLanguages.includes($locale) ? 'rtl' : 'ltr'));

export const dismissedAlerts = initDismissedAlerts();
