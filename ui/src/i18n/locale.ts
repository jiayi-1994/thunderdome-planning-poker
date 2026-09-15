import type { Locales } from './i18n-types';
import { isLocale } from './i18n-util';

/** Respect saved preferences, then the site default; never load an unknown locale. */
export function resolveLocale(preferred?: unknown, configuredDefault?: unknown): Locales {
  if (typeof preferred === 'string' && isLocale(preferred)) return preferred;
  if (typeof configuredDefault === 'string' && isLocale(configuredDefault)) return configuredDefault;
  return 'zh';
}
