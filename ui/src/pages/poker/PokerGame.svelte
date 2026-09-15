<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import Sockette from 'sockette';

  import PageLayout from '../../components/PageLayout.svelte';
  import CategoryVoting from '../../components/poker/CategoryVoting.svelte';
  import CategoryResults from '../../components/poker/CategoryResults.svelte';
  import { emptyCategoryVotes, voteCategories } from '../../components/poker/categoryEstimation';
  import PokerRoleSelector from '../../components/poker/PokerRoleSelector.svelte';
  import { readPokerRole, roleForRound, savePokerRole } from '../../components/poker/pokerRole';
  import { expirationMatchesRound } from '../../components/poker/votingDeadline';
  import { applyJiraSyncEvent, preserveJiraSyncs } from '../../components/poker/jiraWriteback';
  import JiraWritebackSettings from '../../components/poker/JiraWritebackSettings.svelte';
  import JiraSyncStatus from '../../components/poker/JiraSyncStatus.svelte';
  import PokerStories from '../../components/poker/PokerStories.svelte';
  import HollowButton from '../../components/global/HollowButton.svelte';
  import EditPokerGame from '../../components/poker/EditPokerGame.svelte';
  import DeleteConfirmation from '../../components/global/DeleteConfirmation.svelte';
  import { user } from '../../stores';
  import LL from '../../i18n/i18n-svelte';
  import { AppConfig, appRoutes } from '../../config';
  import UserCard from '../../components/poker/UserCard.svelte';
  import VotingControls from '../../components/poker/VotingControls.svelte';
  import InviteUser from '../../components/poker/InviteUser.svelte';
  import VoteTimer from '../../components/poker/VoteTimer.svelte';
  import { availablePokerPoints, pokerPointValues } from '../../components/poker/pointValues';
  import type { PokerGame, PokerStory, PokerVoteCategory, PokerVotingExpiration } from '../../types/poker';
  import { ExternalLink, Pencil, Settings, TimerOff, Trash } from '@lucide/svelte';
  import SubMenu from '../../components/global/SubMenu.svelte';
  import SubMenuItem from '../../components/global/SubMenuItem.svelte';
  import VotingMetrics from '../../components/poker/VotingMetrics.svelte';
  import FullpageLoader from '../../components/global/FullpageLoader.svelte';
  import JoinCodeForm from '../../components/global/JoinCodeForm.svelte';
  import { getWebsocketAddress } from '../../websocketUtil';

  import type { NotificationService } from '../../types/notifications';
  import type { ApiClient } from '../../types/apiclient';
  import Badge from '../../components/global/Badge.svelte';
  import EndStatusBadge from '../../components/global/EndStatusBadge.svelte';
  import EndGameModal from '../../components/poker/EndGameModal.svelte';

  interface Props {
    battleId: string;
    notifications: NotificationService;
    router: any;
    xfetch: ApiClient;
  }

  let { battleId, notifications, router, xfetch }: Props = $props();

  const { AllowGuests } = AppConfig;
  const loginOrRegister: string = AllowGuests ? appRoutes.register : appRoutes.login;

  const hostname: string = window.location.origin;

  const defaultStory: PokerStory = {
    id: '',
    active: false,
    points: '',
    priority: 0,
    skipped: false,
    voteEndTime: undefined,
    voteStartTime: undefined,
    votes: [],
    name: '',
    type: '',
    referenceId: '',
    link: '',
    description: '',
    acceptanceCriteria: '',
    position: 0,
  };

  let isLoading: boolean = $state(true);
  let JoinPassRequired: boolean = $state(false);
  let socketError: boolean = $state(false);
  let socketReconnecting: boolean = $state(false);
  let points: Array<string> = $state([...pokerPointValues]);
  let categoryVotes = $state(emptyCategoryVotes());
  let selectedCategory: PokerVoteCategory | null = $state(null);
  let pokerGame: PokerGame = $state({
    leaders: [],
    autoFinishVoting: false,
    createdDate: undefined,
    hideVoterIdentity: false,
    id: '',
    name: '',
    plans: [],
    pointAverageRounding: '',
    pointValuesAllowed: [],
    updatedDate: undefined,
    users: [],
    votingLocked: false,
    teamId: '',
  });
  let currentStory = $state({ ...defaultStory });
  let showEditGame: boolean = $state(false);
  let showJiraWriteback = $state(false);
  let showDeleteGame: boolean = $state(false);
  let isSpectator: boolean = $state(false);
  let voteStartTime: Date = $state(new Date());
  let voteDeadlineReached = $state(false);
  let showEndGameModal: boolean = $state(false);
  let gameOver: boolean = $derived(typeof pokerGame.endTime !== 'undefined' && pokerGame.endTime !== null);

  let ws: any;

  const onSocketMessage = function (evt: MessageEvent) {
    const previousPlans = pokerGame.plans;
    isLoading = false;
    const parsedEvent = JSON.parse(evt.data);

    switch (parsedEvent.type) {
      case 'join_code_required':
        JoinPassRequired = true;
        break;
      case 'join_code_incorrect':
        notifications.danger($LL.incorrectPassCode());
        break;
      case 'init': {
        JoinPassRequired = false;
        pokerGame = JSON.parse(parsedEvent.value);
        points = availablePokerPoints(pokerGame.pointValuesAllowed);
        const { spectator = false } = pokerGame.users.find((w) => w.id === $user.id) || {};
        isSpectator = spectator;

        currentStory = { ...defaultStory };
        categoryVotes = emptyCategoryVotes();
        voteDeadlineReached = false;
        if (pokerGame.activePlanId) {
          const activePlan = pokerGame.plans.find((p) => p.id === pokerGame.activePlanId);
          if (activePlan) {
            for (const ballot of activePlan.votes) {
              if (ballot.warriorId === $user.id && ballot.category) categoryVotes[ballot.category] = ballot.vote;
            }
            currentStory = activePlan;
            if (activePlan.active) {
              selectedCategory = roleForRound(activePlan.votes, $user.id, selectedCategory);
              if (selectedCategory) savePokerRole(battleId, $user.id, selectedCategory);
            }
            voteStartTime = new Date(activePlan.voteStartTime);
          }
        }

        break;
      }
      case 'user_joined': {
        pokerGame.users = JSON.parse(parsedEvent.value);
        const joinedWarrior = pokerGame.users.find((w) => w.id === parsedEvent.userId);
        if (joinedWarrior.id === $user.id) {
          isSpectator = joinedWarrior.spectator;
        }
        if ($user.notificationsEnabled) {
          notifications.success(
            `${$LL.warriorJoined({
              name: joinedWarrior.name,
            })}
    `,
          );
        }
        break;
      }
      case 'user_left':
        const leftWarrior = pokerGame.users.find((w) => w.id === parsedEvent.userId);
        pokerGame.users = JSON.parse(parsedEvent.value);

        if ($user.notificationsEnabled) {
          notifications.danger(
            `${$LL.warriorRetreated({
              name: leftWarrior.name,
            })}
    `,
          );
        }
        break;
      case 'users_updated':
        pokerGame.users = JSON.parse(parsedEvent.value);
        const updatedWarrior = pokerGame.users.find((w) => w.id === $user.id);
        isSpectator = updatedWarrior.spectator;
        break;
      case 'plan_added':
        pokerGame.plans = JSON.parse(parsedEvent.value);
        break;
      case 'story_arranged':
        pokerGame.plans = JSON.parse(parsedEvent.value);
        break;
      case 'plan_activated':
        const updatedPlans = JSON.parse(parsedEvent.value);
        const activePlan = updatedPlans.find((p) => p.active);
        currentStory = activePlan;
        voteStartTime = new Date(activePlan.voteStartTime);

        pokerGame.plans = updatedPlans;
        pokerGame.activePlanId = activePlan.id;
        pokerGame.votingLocked = false;
        voteDeadlineReached = false;
        categoryVotes = emptyCategoryVotes();
        break;
      case 'plan_skipped':
        const updatedPlans2 = JSON.parse(parsedEvent.value);
        currentStory = { ...defaultStory };
        pokerGame.plans = updatedPlans2;
        pokerGame.activePlanId = '';
        pokerGame.votingLocked = true;
        categoryVotes = emptyCategoryVotes();
        if ($user.notificationsEnabled) {
          notifications.warning($LL.planSkipped());
        }
        break;
      case 'vote_activity':
        const votedWarrior = pokerGame.users.find((w) => w.id === parsedEvent.userId);
        if ($user.notificationsEnabled) {
          notifications.success(
            `${$LL.warriorVoted({
              name: votedWarrior.name,
            })}
    `,
          );
        }

        pokerGame.plans = JSON.parse(parsedEvent.value);
        break;
      case 'vote_retracted':
        const devotedWarrior = pokerGame.users.find((w) => w.id === parsedEvent.userId);
        if ($user.notificationsEnabled) {
          notifications.warning(
            `${$LL.warriorRetractedVote({
              name: devotedWarrior.name,
            })}
    `,
          );
        }

        pokerGame.plans = JSON.parse(parsedEvent.value);
        // The server masks values in broadcasts, so keep selections until their ballot is removed.
        for (const category of voteCategories) {
          if (
            !pokerGame.plans
              .find((plan) => plan.id === pokerGame.activePlanId)
              ?.votes.some((vote) => vote.warriorId === $user.id && vote.category === category.id)
          )
            categoryVotes[category.id] = '';
        }
        break;
      case 'voting_ended':
        pokerGame.plans = JSON.parse(parsedEvent.value);
        pokerGame.votingLocked = true;
        break;
      case 'voting_expired': {
        const expiration: PokerVotingExpiration = JSON.parse(parsedEvent.value);
        const activePlan = pokerGame.plans.find((p) => p.id === pokerGame.activePlanId);
        if (expirationMatchesRound(activePlan, expiration)) {
          pokerGame.plans = expiration.plans;
          pokerGame.votingLocked = true;
          voteDeadlineReached = true;
        }
        break;
      }
      case 'jira_sync_updated':
        pokerGame.plans = applyJiraSyncEvent(pokerGame.plans, JSON.parse(parsedEvent.value));
        break;
      case 'plan_finalized':
        pokerGame.plans = JSON.parse(parsedEvent.value);
        pokerGame.activePlanId = '';
        currentStory = { ...defaultStory };
        categoryVotes = emptyCategoryVotes();
        break;
      case 'plan_revised':
        pokerGame.plans = JSON.parse(parsedEvent.value);
        if (pokerGame.activePlanId !== '') {
          const activePlan = pokerGame.plans.find((p) => p.id === pokerGame.activePlanId);
          currentStory = activePlan || { ...defaultStory };
        }
        break;
      case 'plan_burned':
        const postBurnPlans = JSON.parse(parsedEvent.value);

        if (
          pokerGame.activePlanId !== '' &&
          postBurnPlans.filter((p) => p.id === pokerGame.activePlanId).length === 0
        ) {
          pokerGame.activePlanId = '';
          currentStory = { ...defaultStory };
        }

        pokerGame.plans = postBurnPlans;

        break;
      case 'leaders_updated':
        pokerGame.leaders = parsedEvent.value;
        break;
      case 'battle_revised':
        const revisedBattle = JSON.parse(parsedEvent.value);
        pokerGame.name = revisedBattle.battleName;
        points = availablePokerPoints(revisedBattle.pointValuesAllowed);
        pokerGame.pointValuesAllowed = points;
        if (revisedBattle.votingDurationSeconds != null) {
          pokerGame.votingDurationSeconds = revisedBattle.votingDurationSeconds;
        }
        pokerGame.autoFinishVoting = revisedBattle.autoFinishVoting;
        pokerGame.pointAverageRounding = revisedBattle.pointAverageRounding;
        pokerGame.joinCode = revisedBattle.joinCode;
        pokerGame.hideVoterIdentity = revisedBattle.hideVoterIdentity;
        pokerGame.teamId = revisedBattle.teamId;
        break;
      case 'game_ended':
        const parsed = JSON.parse(parsedEvent.value);
        pokerGame.endTime = new Date(parsed.endTime);
        pokerGame.endReason = parsed.endReason;
        break;
      case 'battle_conceded':
        // poker over, goodbye.
        notifications.warning($LL.battleDeleted());
        router.route(appRoutes.games);
        break;
      case 'jab_warrior':
        const userToNudge = pokerGame.users.find((w) => w.id === parsedEvent.value);
        notifications.info(
          `${$LL.warriorNudgeMessage({
            name: userToNudge.name,
          })}
    `,
        );

        // msn wizz animation and sound
        if (userToNudge.id === $user.id) {
          document.querySelector('body').classList.toggle('shake');

          setTimeout(() => {
            document.querySelector('body').classList.toggle('shake');
          }, 700);
        }
        break;
      default:
        break;
    }
    pokerGame.plans = preserveJiraSyncs(previousPlans, pokerGame.plans);
  };

  onMount(() => {
    if (!$user.id) {
      router.route(`${loginOrRegister}/battle/${battleId}`);
      return;
    }
    selectedCategory = readPokerRole(battleId, $user.id);

    ws = new Sockette(`${getWebsocketAddress()}/api/arena/${battleId}`, {
      timeout: 2e3,
      maxAttempts: 15,
      onmessage: onSocketMessage,
      onerror: (err) => {
        socketError = true;
      },
      onclose: (e) => {
        if (e.code === 4004) {
          router.route(appRoutes.games);
        } else if (e.code === 4001) {
          user.delete();
          router.route(`${loginOrRegister}/battle/${battleId}`);
        } else if (e.code === 4003) {
          notifications.danger($LL.sessionDuplicate());
          router.route(`${appRoutes.games}`);
        } else if (e.code === 4002) {
          router.route(appRoutes.games);
        } else {
          socketReconnecting = true;
        }
      },
      onopen: () => {
        socketError = false;
        socketReconnecting = false;
        isLoading = false;
      },
      onmaximum: () => {
        socketReconnecting = false;
      },
    });
  });

  onDestroy(() => {
    if (ws) {
      ws.close();
    }
  });

  const sendSocketEvent = (type: string, value: any) => {
    ws.send(
      JSON.stringify({
        type,
        value,
      }),
    );
  };

  const handleVote = (category: PokerVoteCategory, point: string) => {
    if (category !== selectedCategory || votingDisabled) return;
    categoryVotes[category] = point;
    const voteValue = {
      planId: pokerGame.activePlanId,
      voteValue: point,
      category,
      autoFinishVoting: pokerGame.autoFinishVoting,
    };

    sendSocketEvent('vote', JSON.stringify(voteValue));
  };

  const handleUnvote = (category: PokerVoteCategory) => {
    if (votingDisabled) return;
    categoryVotes[category] = '';

    sendSocketEvent('retract_vote', JSON.stringify({ planId: pokerGame.activePlanId, category }));
  };

  // Determine if the warrior has voted on active Plan yet
  function didVote(warriorId: string) {
    if (pokerGame.activePlanId === '' || (pokerGame.votingLocked && pokerGame.hideVoterIdentity)) {
      return false;
    }
    const plan = pokerGame.plans.find((p) => p.id === pokerGame.activePlanId);
    const voted = plan?.votes.find((w) => w.warriorId === warriorId);

    return voted !== undefined;
  }

  // Determine if we are showing users vote
  function showVote(warriorId: string) {
    if (pokerGame.hideVoterIdentity || pokerGame.activePlanId === '' || pokerGame.votingLocked === false) {
      return '';
    }
    const story = pokerGame.plans.find((p) => p.id === pokerGame.activePlanId);
    const voted = story?.votes.find((w) => w.warriorId === warriorId && !w.category);

    return voted !== undefined ? voted.vote : '';
  }

  // get highest vote from active story
  function getHighestVote() {
    const voteCounts: Record<string, number> = {};
    points.forEach((p) => {
      voteCounts[p] = 0;
    });
    const highestVote = {
      vote: '',
      count: 0,
    };
    const activePlan = pokerGame.plans.find((p) => p.id === pokerGame.activePlanId);

    if (activePlan.votes.length > 0) {
      const reversedPoints = [...points].filter((v) => v !== '?' && v !== '☕️').reverse();
      reversedPoints.push('?');
      reversedPoints.push('☕️');

      // build a count of each vote
      activePlan.votes.forEach((v) => {
        const voteWarrior = pokerGame.users.find((w) => w.id === v.warriorId) || {};
        const { spectator = false } = voteWarrior;

        if (typeof voteCounts[v.vote] !== 'undefined' && !spectator) {
          ++voteCounts[v.vote];
        }
      });

      // find the highest vote giving priority to higher numbers
      reversedPoints.forEach((p) => {
        if (voteCounts[p] > highestVote.count) {
          highestVote.vote = p;
          highestVote.count = voteCounts[p];
        }
      });
    }

    return highestVote.vote;
  }

  let highestVoteCount = $derived(
    pokerGame.activePlanId !== '' && pokerGame.votingLocked === true ? getHighestVote() : '',
  );
  let showVotingResults = $derived(pokerGame.activePlanId !== '' && pokerGame.votingLocked === true);
  let activeStory = $derived(pokerGame.plans.find((p) => p.id === pokerGame.activePlanId));
  let votingDisabled = $derived(
    !activeStory?.active ||
      pokerGame.votingLocked ||
      voteDeadlineReached ||
      isSpectator ||
      socketError ||
      socketReconnecting ||
      gameOver,
  );
  let roleLocked = $derived(
    Boolean(
      activeStory?.active &&
      (activeStory.votes.some((vote) => vote.warriorId === $user.id) || Object.values(categoryVotes).some(Boolean)),
    ),
  );
  let otherCategoryVotes = $derived(
    activeStory?.active
      ? voteCategories.filter(
          (category) =>
            category.id !== selectedCategory &&
            activeStory.votes.some((vote) => vote.warriorId === $user.id && vote.category === category.id),
        )
      : [],
  );

  function selectRole(category: PokerVoteCategory) {
    if (roleLocked || isSpectator || socketError || socketReconnecting) return;
    selectedCategory = category;
    savePokerRole(battleId, $user.id, category);
  }
  let legacyResults = $derived(
    Boolean(
      showVotingResults &&
      activeStory &&
      activeStory.votes.length > 0 &&
      activeStory.votes.every((vote) => !vote.category),
    ),
  );

  let isFacilitator = $derived(pokerGame.leaders.includes($user.id));

  function concedeGame() {
    sendSocketEvent('concede_battle', '');
  }

  function abandonBattle() {
    sendSocketEvent('abandon_battle', '');
  }

  function toggleEditGame() {
    showEditGame = !showEditGame;
  }

  const toggleDeleteGame = () => {
    showDeleteGame = !showDeleteGame;
  };

  function handleGameEdit(revisedBattle: any) {
    sendSocketEvent('revise_battle', JSON.stringify(revisedBattle));
    toggleEditGame();
    pokerGame.leaderCode = revisedBattle.leaderCode;
  }

  function authBattle(joinPasscode: string) {
    sendSocketEvent('auth_game', joinPasscode);
  }

  function toggleEndGame() {
    showEndGameModal = !showEndGameModal;
  }

  function handleEndGame({ endGameReason }: any) {
    sendSocketEvent('end_game', JSON.stringify({ endReason: endGameReason }));
    toggleEndGame();
  }
</script>

<svelte:head>
  <title
    >{$LL.battle()}
    {pokerGame.name} | {$LL.appName()}</title
  >
</svelte:head>

<PageLayout>
  <div class="mb-6 flex flex-wrap">
    <div class="w-full text-center md:w-2/3 md:text-left">
      {#if !gameOver}
        <h1
          class="text-4xl font-semibold font-rajdhani leading-tight dark:text-white flex items-center flex-wrap gap-2"
        >
          {#if currentStory.link}
            <a
              href={currentStory.link}
              target="_blank"
              class="text-blue-800 dark:text-sky-400 inline-block"
              data-testid="currentplan-link"
            >
              <ExternalLink class="w-8 h-8" />
            </a>
          {/if}
          {#if currentStory.type}
            <Badge label={currentStory.type} testId="currentplan-type" class="text-lg" />
          {/if}
          {#if currentStory.referenceId}
            <span data-testid="currentplan-refid">[{currentStory.referenceId}]</span>
          {/if}
          <span data-testid="currentplan-name">
            {#if currentStory.name === ''}
              [{$LL.votingNotStarted()}]
            {:else}
              {currentStory.name}
            {/if}
          </span>
        </h1>
      {/if}
      <h2
        class="inline-block text-gray-700 dark:text-gray-300 text-3xl font-semibold font-rajdhani leading-tight"
        data-testid="battle-name"
      >
        {pokerGame.name}
      </h2>
      {#if pokerGame.endTime}
        <EndStatusBadge
          endTime={pokerGame.endTime}
          endReason={pokerGame.endReason || 'Ended'}
          class="inline-block ms-2"
        />
      {/if}
    </div>

    <div class="w-full md:w-1/3 text-center md:text-right">
      <VoteTimer
        currentStoryId={currentStory.id}
        votingLocked={pokerGame.votingLocked || gameOver}
        {voteStartTime}
        voteDeadline={currentStory.voteDeadline}
        onExpire={() => {
          voteDeadlineReached = true;
        }}
      />
    </div>
  </div>

  <div class="flex flex-wrap mb-4 -mx-4">
    <div class="w-full lg:w-3/4 px-4">
      {#if !gameOver}
        {#if legacyResults}
          <div class=" mb-2 md:mb-4">
            <VotingMetrics
              pointValues={points}
              votes={pokerGame.plans.find((p) => p.id === pokerGame.activePlanId).votes}
              users={pokerGame.users}
              averageRounding={pokerGame.pointAverageRounding}
            />
          </div>
        {:else if showVotingResults}
          <div class="mb-4">
            <CategoryResults
              estimation={activeStory?.estimation}
              votes={activeStory?.votes}
              users={pokerGame.users}
              hideVoterIdentity={pokerGame.hideVoterIdentity}
            />
          </div>
        {:else}
          <div class="mb-4 lg:mb-6">
            {#if isSpectator}
              <p class="rounded-lg bg-white dark:bg-gray-800 p-6 text-gray-700 dark:text-gray-200">
                你正在旁观，评点结束后可查看三类结果和总分。
              </p>
            {:else}
              <PokerRoleSelector
                category={selectedCategory}
                locked={roleLocked}
                disabled={isLoading || socketError || socketReconnecting}
                onSelect={selectRole}
              />
            {/if}
            {#if otherCategoryVotes.length && !isSpectator}
              <div
                class="mb-4 rounded-lg border border-amber-400 bg-amber-50 p-4 text-amber-900 dark:bg-amber-950 dark:text-amber-100"
              >
                <p>本轮还有其他类别的已有评分。可先撤回，避免计入错误的类别。</p>
                {#each otherCategoryVotes as category}
                  <button
                    type="button"
                    disabled={votingDisabled}
                    onclick={() => handleUnvote(category.id)}
                    class="mt-2 mr-3 rounded border border-amber-600 px-3 py-2 focus-visible:ring-2 focus-visible:ring-blue-500 disabled:opacity-40"
                    >撤回{category.label}评分</button
                  >
                {/each}
              </div>
            {/if}
            {#if selectedCategory && !isSpectator}
              <CategoryVoting
                {points}
                category={selectedCategory}
                selections={categoryVotes}
                votes={activeStory?.votes}
                users={pokerGame.users}
                isLocked={votingDisabled}
                onVote={handleVote}
                onRetract={handleUnvote}
              />
            {/if}
          </div>
        {/if}
      {/if}

      {#if activeStory?.jiraSync && !activeStory.active}
        <div class="mb-4 rounded-lg bg-white dark:bg-gray-800 shadow p-4">
          <JiraSyncStatus
            sync={activeStory.jiraSync}
            gameId={pokerGame.id}
            storyId={activeStory.id}
            canRetry={isFacilitator}
            {xfetch}
            {notifications}
          />
        </div>
      {/if}

      <PokerStories
        plans={pokerGame.plans}
        {isFacilitator}
        {sendSocketEvent}
        {notifications}
        {xfetch}
        gameId={pokerGame.id}
        activePlanId={pokerGame.activePlanId}
        {gameOver}
      />
    </div>

    <div class="w-full lg:w-1/4 px-4">
      <div class="bg-white dark:bg-gray-800 shadow-lg mb-4 rounded-lg">
        <div class="bg-blue-500 dark:bg-gray-700 p-4 rounded-t-lg">
          <h3 class="text-3xl text-white leading-tight font-semibold font-rajdhani uppercase">
            {$LL.warriors()}
          </h3>
        </div>

        {#each pokerGame.users as war (war.id)}
          {#if war.active}
            <UserCard
              warrior={war}
              leaders={pokerGame.leaders}
              {isFacilitator}
              voted={didVote(war.id)}
              points={showVote(war.id)}
              autoFinishVoting={pokerGame.autoFinishVoting}
              {sendSocketEvent}
              {notifications}
              {gameOver}
            />
          {/if}
        {/each}

        {#if isFacilitator && !gameOver}
          <VotingControls
            {points}
            planId={pokerGame.activePlanId}
            {sendSocketEvent}
            votingLocked={pokerGame.votingLocked}
            highestVote={highestVoteCount}
            categoryEstimation={!legacyResults}
            calculatedPoints={activeStory?.estimation?.total || ''}
          />
        {/if}
      </div>

      <div class="bg-white dark:bg-gray-800 shadow-lg p-4 mb-4 rounded-lg">
        <InviteUser {hostname} battleId={pokerGame.id} joinCode={pokerGame.joinCode} {notifications} />
        {#if !isFacilitator}
          <div class="mt-4 text-right">
            <HollowButton color="red" onClick={abandonBattle} testid="battle-abandon">
              {$LL.battleAbandon()}
            </HollowButton>
          </div>
        {/if}
      </div>

      {#if isFacilitator}
        <div class="flex justify-end">
          <SubMenu label={$LL.gameSettings()} icon={Settings} testId="poker-settings">
            {#snippet children({ toggleSubmenu })}
              <SubMenuItem
                onClickHandler={() => {
                  toggleEditGame();
                  toggleSubmenu();
                }}
                testId="battle-edit"
                icon={Pencil}
                label={$LL.battleEdit()}
              />
              <SubMenuItem
                onClickHandler={() => {
                  showJiraWriteback = true;
                  toggleSubmenu();
                }}
                testId="jira-writeback-open"
                icon={ExternalLink}
                label="Jira 点数回写"
              />
              {#if !gameOver && isFacilitator}
                <SubMenuItem
                  onClickHandler={() => {
                    toggleEndGame();
                    toggleSubmenu();
                  }}
                  testId="end-game"
                  icon={TimerOff}
                  label={$LL.endGame()}
                />
              {/if}
              <SubMenuItem
                onClickHandler={() => {
                  toggleDeleteGame();
                  toggleSubmenu();
                }}
                testId="battle-delete"
                icon={Trash}
                label={$LL.battleDelete()}
              />
            {/snippet}
          </SubMenu>
        </div>
      {/if}
    </div>
  </div>

  {#if showEditGame}
    <EditPokerGame
      battleName={pokerGame.name}
      {points}
      votingLocked={pokerGame.votingLocked}
      autoFinishVoting={pokerGame.autoFinishVoting}
      votingDurationSeconds={pokerGame.votingDurationSeconds ?? 120}
      pointAverageRounding={pokerGame.pointAverageRounding}
      hideVoterIdentity={pokerGame.hideVoterIdentity}
      handleBattleEdit={handleGameEdit}
      toggleEditBattle={toggleEditGame}
      joinCode={pokerGame.joinCode}
      leaderCode={pokerGame.leaderCode}
      teamId={pokerGame.teamId}
      {notifications}
      {xfetch}
    />
  {/if}

  {#if showEndGameModal}
    <EndGameModal toggleModal={toggleEndGame} handleSubmit={handleEndGame} {notifications} {xfetch} />
  {/if}

  {#if showJiraWriteback}
    <JiraWritebackSettings
      gameId={pokerGame.id}
      {xfetch}
      {notifications}
      close={() => {
        showJiraWriteback = false;
      }}
    />
  {/if}

  {#if showDeleteGame}
    <DeleteConfirmation
      toggleDelete={toggleDeleteGame}
      handleDelete={concedeGame}
      confirmText={$LL.deleteBattleConfirmText()}
      confirmBtnText={$LL.deleteBattle()}
    />
  {/if}

  {#if socketReconnecting}
    <FullpageLoader>
      {$LL.battleSocketReconnecting()}
    </FullpageLoader>
  {:else if socketError}
    <FullpageLoader>
      {$LL.battleSocketError()}
    </FullpageLoader>
  {:else if isLoading}
    <FullpageLoader>
      {$LL.battleLoading()}
    </FullpageLoader>
  {:else if JoinPassRequired}
    <JoinCodeForm handleSubmit={authBattle} submitText={$LL.battleJoin()} />
  {/if}
</PageLayout>
