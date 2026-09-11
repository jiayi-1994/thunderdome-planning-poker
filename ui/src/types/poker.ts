export type PokerGame = {
  activePlanId?: string;
  autoFinishVoting: boolean;
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
  position: number;
};

export type PokerStoryVote = {
  vote: string;
  warriorId: string;
  category?: PokerVoteCategory;
};

export type PokerVoteCategory = 'testing' | 'frontend' | 'backend';

export type PokerVotingExpiration = {
  planId: string;
  voteStartTime: Date;
  plans: PokerStory[];
};

export type PokerEstimation = {
  categories: Array<{ category: PokerVoteCategory; average: string; count: number }>;
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
