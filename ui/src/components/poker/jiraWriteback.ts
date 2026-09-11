import type { PokerJiraSyncEvent, PokerStory } from '../../types/poker';
import { expirationMatchesRound } from './votingDeadline';

function compareSyncTimes(left: string, right: string): number {
  const milliseconds = new Date(left).getTime() - new Date(right).getTime();
  if (milliseconds !== 0) return milliseconds;
  // PostgreSQL timestamps can differ within one JS millisecond.
  const remainder = (value: string) => (value.match(/\.(\d+)/)?.[1] || '').padEnd(9, '0').slice(3, 9);
  return remainder(left).localeCompare(remainder(right));
}

export function applyJiraSyncEvent(stories: PokerStory[], event: PokerJiraSyncEvent): PokerStory[] {
  return stories.map(story => {
    // A fast Jira response can arrive before voting_ended. Keep it until that event reveals results.
    if (!expirationMatchesRound(story, event)) return story;
    if (story.jiraSync && compareSyncTimes(story.jiraSync.updatedAt, event.sync.updatedAt) > 0) return story;
    return { ...story, jiraSync: event.sync };
  });
}

export function preserveJiraSyncs(previous: PokerStory[], incoming: PokerStory[]): PokerStory[] {
  if (previous === incoming) return incoming;
  const previousStories = new Map(previous.map(story => [story.id, story]));
  return incoming.map(story => {
    const old = previousStories.get(story.id);
    if (!old?.jiraSync || !expirationMatchesRound(story, { planId: old.id, voteStartTime: old.voteStartTime }))
      return story;
    if (story.jiraSync && compareSyncTimes(story.jiraSync.updatedAt, old.jiraSync.updatedAt) >= 0) return story;
    return { ...story, jiraSync: old.jiraSync };
  });
}

export async function jiraErrorMessage(error: unknown, fallback: string): Promise<string> {
  const response =
    error instanceof Response ? error : Array.isArray(error) && error[1] instanceof Response ? error[1] : undefined;
  if (response) {
    try {
      const result = await response.json();
      if (result.error === 'REQUIRES_SUBSCRIBED_USER') return '当前账号需要订阅才能使用 Jira 集成。';
      if (typeof result.error === 'string') return result.error;
    } catch {
      /* Keep a useful fallback when the server did not return JSON. */
    }
  }
  return error instanceof Error ? error.message : fallback;
}
