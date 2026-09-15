export type JiraStoryReference = { referenceId?: string; link?: string };

// Keep this normalization aligned with internal/db/poker/story_import.go.
export function jiraStoryIdentity(story: JiraStoryReference): string {
  try {
    const url = new URL(story.link?.trim() || '');
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return '';
    const match = url.pathname.replace(/\/+$/, '').match(/^(.*?)\/browse\/([A-Za-z][A-Za-z0-9_]*-\d+)$/);
    if (!match) return '';
    const key = match[2].toUpperCase();
    if (story.referenceId?.trim() && story.referenceId.trim().toUpperCase() !== key) return '';
    return `${url.origin}${match[1]}/browse/${key}`;
  } catch {
    return '';
  }
}
