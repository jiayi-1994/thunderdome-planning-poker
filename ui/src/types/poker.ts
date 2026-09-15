export type PokerGame = {
  activePlanId?: string;
  autoFinishVoting: boolean;
  votingDurationSeconds?: number;
  createdDate: Date;
  hideVoterIdentity: boolean;
  id: string;
  joinCode?: string;
  leaderCode?: string;
  leaders: Array<String>;
  name: string;
  plans: Array<PokerStory>;
  pointAverageRounding: string;
  pointValuesAllowed: Array<string>;
  updatedDate: Date;
  users: Array<PokerUser>;
  votingLocked: boolean;
  teamId?: string;
  endTime?: Date;
  endReason?: string;
};

export type PokerStory = {
  id: string;
  name: string;
  type: string;
  referenceId?: string;
  link?: string;
  description?: string;
  acceptanceCriteria?: string;
  active: boolean;
  points: string;
  priority: number;
  skipped: boolean;
  voteEndTime: Date;
  voteStartTime: Date;
  voteDeadline?: Date;
  votes: Array<PokerStoryVote>;
  estimation?: PokerEstimation;
  jiraSync?: PokerJiraSync;
  position: number;
};

export type PokerStoryVote = {
  vote: string;
  warriorId: string;
  category?: PokerVoteCategory;
};

export type PokerVoteCategory = 'testing' | 'frontend' | 'backend';

export type PokerJiraSettings = {
  enabled: boolean;
  instanceId: string;
  fieldId: string;
  fieldName: string;
  host: string;
};

export type PokerJiraSync = {
  status: 'pending' | 'succeeded' | 'failed' | 'skipped' | 'cancelled';
  issueKey: string;
  points: string;
  error?: string;
  attempts: number;
  updatedAt: string;
};

export type PokerJiraSyncEvent = {
  planId: string;
  voteStartTime: Date;
  sync: PokerJiraSync;
};

export type PokerVotingExpiration = {
  planId: string;
  voteStartTime: Date;
  plans: PokerStory[];
};

export type PokerEstimation = {
  categories: Array<{
    category: PokerVoteCategory;
    average: string;
    count: number;
    distinctValues?: string[];
    needsDiscussion?: boolean;
  }>;
  total: string;
};

export type PokerUser = {
  abandoned: boolean;
  active: boolean;
  avatar: string;
  gravatarHash: string;
  id: string;
  name: string;
  rank: string;
  spectator: boolean;
};
