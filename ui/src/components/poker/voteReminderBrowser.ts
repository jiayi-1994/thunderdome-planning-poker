const notifiedRounds = new Set<string>();
const lastRoundStorageKey = 'thunderdome.vote-reminder.last-round';

// Session storage prevents another notification when a user reloads the current round.
// The in-memory guard still works when browser storage is unavailable.
export function claimRoundNotification(roundKey: string): boolean {
  if (notifiedRounds.has(roundKey)) return false;
  try {
    if (sessionStorage.getItem(lastRoundStorageKey) === roundKey) {
      notifiedRounds.add(roundKey);
      return false;
    }
  } catch {
    // Private browsing and storage policies must not interrupt voting.
  }
  notifiedRounds.add(roundKey);
  if (notifiedRounds.size > 100) notifiedRounds.delete(notifiedRounds.values().next().value!);
  try {
    sessionStorage.setItem(lastRoundStorageKey, roundKey);
  } catch {
    // The in-memory guard above remains available.
  }
  return true;
}

export function showTabVoteReminder(title: string, iconPath: string): () => void {
  const originalTitle = document.title;
  const reminderTitle = `🔴 ${title} · ${originalTitle}`;
  document.title = reminderTitle;

  const existingIcons = Array.from(document.head.querySelectorAll<HTMLLinkElement>('link[rel~="icon"]'));
  const createdIcon = existingIcons.length === 0 ? document.createElement('link') : undefined;
  if (createdIcon) {
    createdIcon.rel = 'icon';
    document.head.appendChild(createdIcon);
  }
  const icons = createdIcon ? [createdIcon] : existingIcons;
  const originals = icons.map(icon => ({
    icon,
    href: icon.getAttribute('href'),
    type: icon.getAttribute('type'),
    sizes: icon.getAttribute('sizes'),
  }));
  for (const icon of icons) {
    icon.setAttribute('href', iconPath);
    icon.setAttribute('type', 'image/svg+xml');
    icon.setAttribute('sizes', 'any');
  }

  return () => {
    // Navigation may have already installed a different title or favicon.
    if (document.title === reminderTitle) document.title = originalTitle;
    for (const { icon, href, type, sizes } of originals) {
      if (!icon.isConnected || icon.getAttribute('href') !== iconPath) continue;
      if (icon === createdIcon) {
        icon.remove();
        continue;
      }
      restoreAttribute(icon, 'href', href);
      if (icon.getAttribute('type') === 'image/svg+xml') restoreAttribute(icon, 'type', type);
      if (icon.getAttribute('sizes') === 'any') restoreAttribute(icon, 'sizes', sizes);
    }
  };
}

function restoreAttribute(element: Element, name: string, value: string | null): void {
  if (value === null) element.removeAttribute(name);
  else element.setAttribute(name, value);
}
