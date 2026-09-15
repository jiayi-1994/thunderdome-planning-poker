<script lang="ts">
  import { AppConfig, appRoutes } from '../config';
  import { user } from '../stores';
  import { Github, Users, Zap } from '@lucide/svelte';
  import LL from '../i18n/i18n-svelte';
  import Countries from '../components/user/Countries.svelte';
  import BrowserMock from '../components/global/BrowserMock.svelte';
  import { onMount } from 'svelte';
  import type { ApiClient } from '../types/apiclient';

  interface Props {
    xfetch: ApiClient;
  }

  let { xfetch }: Props = $props();

  const { ShowActiveCountries, PathPrefix, RepoURL } = AppConfig;

  const slogans = $derived([
    $LL.landingSlogan1(),
    $LL.landingSlogan2(),
    $LL.landingSlogan3(),
    $LL.landingSlogan4(),
    $LL.landingSlogan5(),
    $LL.landingSlogan6(),
    $LL.landingSlogan7(),
    $LL.landingSlogan8(),
  ]);

  const sloganIndex = Math.floor(Math.random() * 8);
  let randomSlogan = $derived(slogans[sloganIndex]);

  onMount(() => window.scrollTo(0, 0));
</script>

<svelte:head>
  <title>{$LL.appName()} - {$LL.appSubtitle()}</title>
</svelte:head>

<main class="bg-gray-100 dark:bg-gray-900">
  <header class="bg-gradient-to-r from-blue-600 to-indigo-700 text-white">
    <div class="container mx-auto px-4 py-16">
      <div class="max-w-7xl mx-auto text-center">
        <h1 class="text-4xl sm:text-5xl lg:text-6xl font-bold mb-6 leading-tight">
          <span class="block bg-clip-text text-transparent bg-gradient-to-r from-yellow-thunder to-orange-500">
            Thunderdome
          </span>
          {randomSlogan}
        </h1>
        <p class="max-w-4xl mx-auto text-xl sm:text-2xl text-blue-100 mb-8">
          {$LL.landingIntro()}
        </p>
        <div class="flex flex-col sm:flex-row items-center justify-center space-y-4 sm:space-y-0 sm:space-x-6">
          {#if $user.id}
            <a
              href={appRoutes.games}
              class="bg-white text-indigo-700 hover:bg-gray-100 font-semibold py-3 px-8 rounded-full transition duration-300 shadow-lg"
            >
              {$LL.startPlanning()}
            </a>
          {:else}
            <a
              href={appRoutes.register}
              class="bg-white text-indigo-700 hover:bg-gray-100 font-semibold py-3 px-8 rounded-full transition duration-300 shadow-lg"
            >
              {$LL.getStartedFree()}
            </a>
          {/if}
          <a
            href="#features"
            class="bg-transparent text-white hover:bg-white/10 border border-white font-semibold py-3 px-8 rounded-full transition duration-300"
          >
            {$LL.exploreFeatures()}
          </a>
        </div>
      </div>
    </div>
  </header>

  <section id="features" class="bg-white dark:bg-gray-800 py-20">
    <div class="container mx-auto px-4">
      <div class="flex flex-col md:flex-row items-center justify-between">
        <div class="md:w-1/2 md:pe-8 mb-8 md:mb-0">
          <div class="title-line bg-yellow-thunder"></div>
          <h2 class="text-4xl font-semibold font-rajdhani uppercase dark:text-white mb-6">
            {$LL.landingPokerTitle()}
          </h2>
          <p class="text-lg text-gray-600 dark:text-gray-400 mb-4">
            {$LL.landingPokerDescription()}
          </p>
          <ul class="space-y-3 text-gray-700 dark:text-gray-300 mb-8">
            <li class="flex items-start">
              <span class="text-indigo-500 dark:text-indigo-400 me-2">✓</span>
              <span>{@html $LL.landingPokerBias()}</span
              >
            </li>
            <li class="flex items-start">
              <span class="text-indigo-500 dark:text-indigo-400 me-2">✓</span>
              <span
                >{@html $LL.landingPokerScales()}</span
              >
            </li>
            <li class="flex items-start">
              <span class="text-indigo-500 dark:text-indigo-400 me-2">✓</span>
              <span
                >{@html $LL.landingPokerRemote()}</span
              >
            </li>
          </ul>
          <a
            href={appRoutes.games}
            class="group relative inline-flex items-center justify-center p-0.5 mb-2 me-2 overflow-hidden font-medium text-gray-900 rounded-lg group bg-gradient-to-br from-purple-600 to-blue-500 group-hover:from-purple-600 group-hover:to-blue-500 hover:text-white dark:text-white focus:ring-4 focus:outline-none focus:ring-blue-300 dark:focus:ring-blue-800"
          >
            <span
              class="relative px-5 py-2.5 transition-all ease-in duration-75 bg-white dark:bg-gray-900 rounded-md group-hover:bg-opacity-0"
            >
              {$user.id ? $LL.battleCreate() : $LL.tryPlanningPoker()}
            </span>
          </a>
        </div>
        <div class="md:w-1/2">
          <BrowserMock>
            <img
              class="rounded-b-lg hidden dark:block"
              src="{PathPrefix}/img/previews/planning_poker_20250812_dark.png"
              alt={$LL.appPreviewAlt()}
            />
            <img
              class="rounded-b-lg dark:hidden"
              src="{PathPrefix}/img/previews/planning_poker_20250812_light.png"
              alt={$LL.appPreviewAlt()}
            />
          </BrowserMock>
        </div>
      </div>
    </div>
  </section>

  <section class="bg-gray-100 dark:bg-gray-900 py-20">
    <div class="container mx-auto px-4">
      <div class="flex flex-col md:flex-row-reverse items-center justify-between">
        <div class="md:w-1/2 md:ps-8 mb-8 md:mb-0">
          <div class="title-line bg-yellow-thunder"></div>
          <h2 class="text-4xl font-semibold font-rajdhani uppercase dark:text-white mb-6">
            {$LL.landingRetroTitle()}
          </h2>
          <p class="text-lg text-gray-600 dark:text-gray-400 mb-4">
            {$LL.landingRetroDescription()}
          </p>
          <ul class="space-y-3 text-gray-700 dark:text-gray-300 mb-8">
            <li class="flex items-start">
              <span class="text-indigo-500 dark:text-indigo-400 me-2">✓</span>
              <span
                >{@html $LL.landingRetroFormats()}</span
              >
            </li>
            <li class="flex items-start">
              <span class="text-indigo-500 dark:text-indigo-400 me-2">✓</span>
              <span
                >{@html $LL.landingRetroSafety()}</span
              >
            </li>
            <li class="flex items-start">
              <span class="text-indigo-500 dark:text-indigo-400 me-2">✓</span>
              <span
                >{@html $LL.landingRetroActions()}</span
              >
            </li>
          </ul>
          <a
            href={appRoutes.retros}
            class="group relative inline-flex items-center justify-center p-0.5 mb-2 me-2 overflow-hidden font-medium text-gray-900 rounded-lg group bg-gradient-to-br from-purple-600 to-blue-500 group-hover:from-purple-600 group-hover:to-blue-500 hover:text-white dark:text-white focus:ring-4 focus:outline-none focus:ring-blue-300 dark:focus:ring-blue-800"
          >
            <span
              class="relative px-5 py-2.5 transition-all ease-in duration-75 bg-white dark:bg-gray-900 rounded-md group-hover:bg-opacity-0"
            >
              {$user.id ? $LL.startRetrospective() : $LL.tryRetrospectives()}
            </span>
          </a>
        </div>
        <div class="md:w-1/2">
          <BrowserMock>
            <img
              class="rounded-b-lg hidden dark:block"
              src="{PathPrefix}/img/previews/retro_20250812_dark.png"
              alt={$LL.retroPreviewAlt()}
            />
            <img
              class="rounded-b-lg dark:hidden"
              src="{PathPrefix}/img/previews/retro_20250812_light.png"
              alt={$LL.retroPreviewAlt()}
            />
          </BrowserMock>
        </div>
      </div>
    </div>
  </section>

  <section class="bg-white dark:bg-gray-800 py-20">
    <div class="container mx-auto px-4">
      <div class="flex flex-col md:flex-row items-center justify-between">
        <div class="md:w-1/2 md:pe-8 mb-8 md:mb-0">
          <div class="title-line bg-yellow-thunder"></div>
          <h2 class="text-4xl font-semibold font-rajdhani uppercase dark:text-white mb-6">
            {$LL.landingStoryTitle()}
          </h2>
          <p class="text-lg text-gray-600 dark:text-gray-400 mb-4">
            {$LL.landingStoryDescription()}
          </p>
          <ul class="space-y-3 text-gray-700 dark:text-gray-300 mb-8">
            <li class="flex items-start">
              <span class="text-indigo-500 dark:text-indigo-400 me-2">✓</span>
              <span
                >{@html $LL.landingStoryDrag()}</span
              >
            </li>
            <li class="flex items-start">
              <span class="text-indigo-500 dark:text-indigo-400 me-2">✓</span>
              <span
                >{@html $LL.landingStoryOrganization()}</span
              >
            </li>
            <li class="flex items-start">
              <span class="text-indigo-500 dark:text-indigo-400 me-2">✓</span>
              <span
                >{@html $LL.landingStoryDetails()}</span
              >
            </li>
          </ul>
          <a
            href={appRoutes.storyboards}
            class="group relative inline-flex items-center justify-center p-0.5 mb-2 me-2 overflow-hidden font-medium text-gray-900 rounded-lg group bg-gradient-to-br from-purple-600 to-blue-500 group-hover:from-purple-600 group-hover:to-blue-500 hover:text-white dark:text-white focus:ring-4 focus:outline-none focus:ring-blue-300 dark:focus:ring-blue-800"
          >
            <span
              class="relative px-5 py-2.5 transition-all ease-in duration-75 bg-white dark:bg-gray-900 rounded-md group-hover:bg-opacity-0"
            >
              {$user.id ? $LL.createStoryMap() : $LL.tryStoryMapping()}
            </span>
          </a>
        </div>
        <div class="md:w-1/2">
          <BrowserMock>
            <img
              class="rounded-b-lg hidden dark:block"
              src="{PathPrefix}/img/previews/storyboard_20250812_dark.png"
              alt={$LL.storyboardPreviewAlt()}
            />
            <img
              class="rounded-b-lg dark:hidden"
              src="{PathPrefix}/img/previews/storyboard_20250812_light.png"
              alt={$LL.storyboardPreviewAlt()}
            />
          </BrowserMock>
        </div>
      </div>
    </div>
  </section>

  <section class="bg-gray-100 dark:bg-gray-900 py-20">
    <div class="container mx-auto px-4">
      <div class="flex flex-col md:flex-row-reverse items-center justify-between">
        <div class="md:w-1/2 md:ps-8 mb-8 md:mb-0">
          <div class="title-line bg-yellow-thunder"></div>
          <h2 class="text-4xl font-semibold font-rajdhani uppercase dark:text-white mb-6">{$LL.landingCheckinTitle()}</h2>
          <p class="text-lg text-gray-600 dark:text-gray-400 mb-8">
            {$LL.landingCheckinDescription()}
          </p>
          <ul class="space-y-3 text-gray-700 dark:text-gray-300 mb-8">
            <li class="flex items-start">
              <span class="text-indigo-500 dark:text-indigo-400 me-2">✓</span>
              <span
                >{@html $LL.landingCheckinAlignment()}</span
              >
            </li>
            <li class="flex items-start">
              <span class="text-indigo-500 dark:text-indigo-400 me-2">✓</span>
              <span
                >{@html $LL.landingCheckinBlockers()}</span
              >
            </li>
            <li class="flex items-start">
              <span class="text-indigo-500 dark:text-indigo-400 me-2">✓</span>
              <span
                >{@html $LL.landingCheckinComments()}</span
              >
            </li>
          </ul>
        </div>
        <div class="md:w-1/2">
          <img
            class="rounded-lg shadow-lg hidden dark:block max-w-lg mx-auto"
            src="{PathPrefix}/img/previews/team_checkins_dark_2025_09_24.png"
            alt={$LL.checkinsPreviewAlt()}
          />
          <img
            class="rounded-lg shadow-lg dark:hidden max-w-lg mx-auto"
            src="{PathPrefix}/img/previews/team_checkins_light_2025_09_24.png"
            alt={$LL.checkinsPreviewAlt()}
          />
        </div>
      </div>
    </div>
  </section>

  <section class="bg-indigo-600 text-white py-20">
    <div class="container mx-auto px-4 text-center">
      <h2 class="text-4xl font-bold mb-6 font-rajdhani uppercase">{$LL.landingWhyTitle()}</h2>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-8">
        <div class="bg-white dark:bg-gray-800 rounded-lg p-6 text-gray-800 dark:text-white">
          <div class="text-indigo-500 text-4xl mb-4">
            <Zap class="h-12 w-12 mx-auto" />
          </div>
          <h3 class="text-xl font-semibold mb-2">{$LL.landingRemoteTitle()}</h3>
          <p>
            {$LL.landingRemoteDescription()}
          </p>
        </div>
        <div class="bg-white dark:bg-gray-800 rounded-lg p-6 text-gray-800 dark:text-white">
          <div class="text-indigo-500 text-4xl mb-4">
            <Users class="h-12 w-12 mx-auto" />
          </div>
          <h3 class="text-xl font-semibold mb-2">{$LL.landingSafetyTitle()}</h3>
          <p>
            {$LL.landingSafetyDescription()}
          </p>
        </div>
        <div class="bg-white dark:bg-gray-800 rounded-lg p-6 text-gray-800 dark:text-white">
          <div class="text-indigo-500 text-4xl mb-4">
            <Github class="h-12 w-12 mx-auto" />
          </div>
          <h3 class="text-xl font-semibold mb-2">{$LL.openSource()}</h3>
          <p>
            <a
              href={appRoutes.subscriptionPricing}
              class="text-indigo-400 dark:text-indigo-300 hover:text-yellow-thunder dark:hover:text-yellow-thunder font-bold"
              >{$LL.premiumCloudHosted()}</a
            >
            {$LL.hostingChoiceOr()}
            <a
              href="{RepoURL}/blob/main/docs/INSTALLATION.md"
              target="_blank"
              class="text-indigo-400 dark:text-indigo-300 hover:text-yellow-thunder dark:hover:text-yellow-thunder font-bold"
              >{$LL.selfHosted()}</a
            > {$LL.hostingChoiceEnd()}
          </p>
        </div>
      </div>
    </div>
  </section>

  {#if ShowActiveCountries}
    <section class="bg-slate-100 dark:bg-gray-900 py-20">
      <div class="container mx-auto px-4">
        <Countries {xfetch} />
      </div>
    </section>
  {/if}
</main>
