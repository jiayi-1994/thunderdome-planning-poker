import { describe, expect, it } from 'vitest';
import { jiraStoryIdentity } from './storyIdentity';

describe('Jira story identity', () => {
  it.each([
    [
      ' test-1 ',
      ' https://JIRA.example.com:443/jira/browse/test-1/?a=1#details ',
      'https://jira.example.com/jira/browse/TEST-1',
    ],
    ['', 'http://jira.example.com:80/browse/TEST-1', 'http://jira.example.com/browse/TEST-1'],
    ['TEST-1', 'https://jira.example.com:8443/jira/browse/TEST-1', 'https://jira.example.com:8443/jira/browse/TEST-1'],
    ['TEST-1', 'https://jira.example.com/browse/TEST-2', ''],
    ['TEST-1', 'https://jira.example.com/other/TEST-1', ''],
    ['TEST-1', 'https://jira.example.com/browse/TEST-1/more', ''],
    ['TEST-1', 'https://user@jira.example.com/browse/TEST-1', ''],
    ['TEST-1', '', ''],
    ['', 'not a URL', ''],
  ])('normalizes %s at %s', (referenceId, link, expected) => {
    expect(jiraStoryIdentity({ referenceId, link })).toBe(expected);
  });
});
