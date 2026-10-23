<script lang="ts">
	import '$lib/icon-bundle';
	import { onMount } from 'svelte';
	import '../app.css';
	import { page } from '$app/stores';
	import { auth, isAnonymousMode } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { browser } from '$app/environment';
	import { m } from '$lib/paraglide/messages';
	import { settingsAttention, refreshSettingsAttention } from '$lib/settings/attention';
	import { myAccess, loadMyAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { visibleItems } from '$lib/settings/sections';
	import { locale } from '$lib/i18n';
	import UserIcon from '~icons/heroicons/user-16-solid';
	import Icon from '@iconify/svelte';
	import { userProfile } from '$lib/stores/user';
	import { encryptionConfigured, allowUnencrypted } from '$lib/stores/encryption';
	import { tablePreviewEnabled } from '$lib/stores/features';
	import Banner from '$lib/components/Banner.svelte';
	import Footer from '$lib/components/Footer.svelte';
	import LanguageSelector from '$components/ui/LanguageSelector.svelte';
	import type GlobalSearch from '$components/query/GlobalSearch.svelte';
	import Avatar from '$components/user/Avatar.svelte';
	import ToastContainer from '$components/ui/ToastContainer.svelte';
	import NotificationBell from '$components/notifications/NotificationBell.svelte';

	interface BannerConfig {
		enabled: boolean;
		dismissible: boolean;
		variant: 'info' | 'warning' | 'error' | 'success';
		message: string;
		id: string;
	}

	let bannerConfig: BannerConfig | null = null;
	let isDropdownOpen = false;
	let isGovernanceOpen = false;
	let isMobileMenuOpen = false;
	let canQueryData = false;
	let checkingAnonymousMode = true;
	let manualNavigation = false;
	let showSearchModal = false;
	let searchInput: GlobalSearch;
	// Search is fetched on first use rather than with every page: it shows asset icons, and importing them eagerly put over a hundred logo modules in front of every page load.
	let SearchComponent: typeof GlobalSearch | null = null;
	function loadSearch(): Promise<typeof GlobalSearch> {
		return import('$components/query/GlobalSearch.svelte').then(
			(mod) => (SearchComponent = mod.default)
		);
	}

	const isMac = browser && navigator.platform.toUpperCase().indexOf('MAC') >= 0;

	$: currentSearchQuery = $page.url.searchParams.get('q') || '';

	function getHighlightedText(text: string): { text: string; class: string }[] {
		if (!text) return [];

		const regex =
			/@metadata\.[a-zA-Z0-9_.]+|"[^"]*"|'[^']*'|[:=<>!]+|\b(AND|OR|NOT|contains|range)\b/g;
		const parts: { text: string; class: string }[] = [];
		let lastIndex = 0;
		let match;

		while ((match = regex.exec(text)) !== null) {
			if (match.index > lastIndex) {
				parts.push({
					text: text.slice(lastIndex, match.index),
					class: 'text-gray-900 dark:text-gray-100'
				});
			}

			const matchText = match[0];
			let colorClass = 'text-gray-900 dark:text-gray-100';

			if (matchText.startsWith('@metadata.')) {
				colorClass = 'text-blue-500 dark:text-blue-400';
			} else if (matchText.match(/^["'][^"']*["']$/)) {
				colorClass = 'text-green-600 dark:text-green-400';
			} else if (matchText.match(/[:=<>!]+|contains|range/)) {
				colorClass = 'text-purple-500 dark:text-purple-400';
			} else if (matchText.match(/\b(AND|OR|NOT)\b/)) {
				colorClass = 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700';
			}

			parts.push({
				text: matchText,
				class: colorClass
			});

			lastIndex = match.index + matchText.length;
		}

		if (lastIndex < text.length) {
			parts.push({
				text: text.slice(lastIndex),
				class: 'text-gray-900 dark:text-gray-100'
			});
		}

		return parts;
	}

	async function openSearchModal() {
		if (!SearchComponent) await loadSearch();
		showSearchModal = true;
		setTimeout(() => {
			searchInput?.focus();
		}, 100);
	}

	function handleGlobalKeydown(event: KeyboardEvent) {
		const isCmdOrCtrl = isMac ? event.metaKey : event.ctrlKey;

		if (isCmdOrCtrl && event.key === 'k') {
			event.preventDefault();

			if (showSearchModal) {
				showSearchModal = false;
			} else {
				openSearchModal();
			}
		}

		if (event.key === 'Escape') {
			if (showSearchModal) {
				event.preventDefault();
				event.stopPropagation();
				showSearchModal = false;
			}
		}
	}

	onMount(async () => {
		// Fetched once the page has settled, so the first Cmd-K opens at once without slowing the page that is loading
		if (browser) {
			const idle = window.requestIdleCallback ?? ((fn: () => void) => setTimeout(fn, 1500));
			idle(() => loadSearch());
		}
		checkingAnonymousMode = true;
		await auth.checkAnonymousMode();
		checkingAnonymousMode = false;

		if (browser && $auth) {
			canQueryData = auth.hasPermission('assets', 'query');
			userProfile.load();
		}

		if (browser) {
			try {
				const response = await fetch('/api/v1/ui/config');
				if (response.ok) {
					const data = await response.json();
					bannerConfig = data.banner;
					encryptionConfigured.set(data.encryption_configured ?? true);
					allowUnencrypted.set(data.allow_unencrypted ?? false);
					tablePreviewEnabled.set(data.table_preview_enabled ?? false);
				}
			} catch (error) {
				console.error('Failed to fetch UI config:', error);
			}
		}
	});

	$: if (browser && $auth) {
		canQueryData = auth.hasPermission('assets', 'query');
		loadMyAccess();
	}
	// The gear shows for anyone who can manage any part of Settings, not only the admin role.
	$: canViewQueryLog = browser && $auth ? holdsAtRoot($myAccess, 'access:view') : false;
	$: canSeeSettings = browser && $auth ? visibleItems($myAccess).length > 0 : false;
	// Re-checked on navigation, at most every few minutes, so the dot clears without a reload once things are fixed
	$: if (canSeeSettings && $myAccess.checked && $page.url.pathname)
		refreshSettingsAttention($myAccess);
	$: settingsLabel =
		canSeeSettings && $settingsAttention > 0
			? m.settings_attention_dot({ count: $settingsAttention })
			: m.nav_settings();
	$: if ($page.url.pathname) isMobileMenuOpen = false;

	function withManualNav(fn: () => void) {
		manualNavigation = true;
		fn();
		setTimeout(() => {
			manualNavigation = false;
		}, 100);
	}

	$: if (browser && !checkingAnonymousMode && !manualNavigation) {
		const isOAuthPending = $page.url.searchParams.has('oauth_pending');
		if ($auth && $page.url.pathname.startsWith('/login') && !isOAuthPending) {
			goto(resolve('/'));
		} else if (!$auth && !$page.url.pathname.startsWith('/login') && !$isAnonymousMode) {
			goto(resolve('/login'));
		}
	}

	function toggleDropdown() {
		isDropdownOpen = !isDropdownOpen;
		isGovernanceOpen = false;
	}

	function toggleGovernance() {
		isGovernanceOpen = !isGovernanceOpen;
		isDropdownOpen = false;
	}

	function closeDropdown() {
		isDropdownOpen = false;
		isGovernanceOpen = false;
	}

	function handleLogout() {
		withManualNav(() => {
			auth.clearToken();
			userProfile.clear();
			window.location.href = '/';
		});
	}

	function handleLogin() {
		withManualNav(() => {
			goto(resolve('/login'));
		});
	}

	function capitalizeWord(word: string): string {
		return word
			.split('-')
			.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
			.join(' ');
	}

	// Known routes map to translated titles, unknown slugs keep the capitalise-the-slug fallback
	const pageTitles: Record<string, () => string> = {
		discover: m.pagetitle_discover,
		runs: m.pagetitle_runs,
		query: m.pagetitle_query,
		insights: m.pagetitle_insights,
		glossary: m.pagetitle_glossary,
		products: m.pagetitle_products,
		teams: m.pagetitle_teams,
		pipelines: m.pagetitle_pipelines,
		admin: m.pagetitle_admin,
		profile: m.pagetitle_profile,
		settings: m.pagetitle_settings,
		account: m.pagetitle_account,
		notifications: m.pagetitle_notifications,
		'asset-rules': m.pagetitle_asset_rules,
		'service-accounts': m.pagetitle_service_accounts,
		users: m.pagetitle_users,
		roles: m.pagetitle_roles,
		assets: m.pagetitle_assets,
		search: m.pagetitle_search,
		login: m.pagetitle_login
	};

	$: pathSegments = $page.url.pathname.split('/').filter(Boolean);
	$: decodedSegments = pathSegments.map((segment) => decodeURIComponent(segment));
	$: processedSegments = decodedSegments.map((segment, index) =>
		index < 2 ? capitalizeWord(segment) : segment
	);
	$: knownPageTitle = pageTitles[decodedSegments[0]];
	$: reversedSegments = [...processedSegments].reverse();
	$: pageTitle = reversedSegments.join(' - ');
	// $locale is read so the title recomputes when the language changes
	$: dynamicTitle =
		$locale && $page.url.pathname === '/'
			? 'Marmot'
			: knownPageTitle
				? [...processedSegments.slice(1)].reverse().concat(knownPageTitle()).join(' - ')
				: `${pageTitle} - Marmot`;
</script>

<svelte:head>
	<title>{dynamicTitle}</title>
</svelte:head>

<svelte:window onclick={closeDropdown} onkeydown={handleGlobalKeydown} />

<!-- Keyed on the locale so a language change re-renders the shell in place instead of reloading the document -->
{#key $locale}
	<div class="h-screen flex flex-col">
		{#if !$page.url.pathname.startsWith('/login') && bannerConfig}
			<Banner
				enabled={bannerConfig.enabled}
				dismissible={bannerConfig.dismissible}
				variant={bannerConfig.variant}
				message={bannerConfig.message}
				id={bannerConfig.id}
			/>
		{/if}
		{#if !$page.url.pathname.startsWith('/login')}
			<nav class="glass-navbar flex-none sticky top-0 z-40">
				<div class="mx-auto px-4 sm:px-6 lg:px-8">
					<div class="flex items-center justify-between h-16 gap-6">
						<!-- Logo -->
						<div class="flex items-center flex-shrink-0">
							<a href={resolve('/')} class="flex-shrink-0 hover:opacity-80 transition-opacity">
								<img src="/images/marmot-text.svg" alt="Marmot" class="h-6 dark:invert" />
							</a>
						</div>

						<!-- Centered Search (desktop only) -->
						<div class="hidden sm:flex flex-1 justify-center max-w-3xl mx-auto">
							<button
								onclick={openSearchModal}
								class="flex items-center gap-2 px-4 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-600 rounded-lg hover:border-gray-300 dark:hover:border-gray-500 transition-colors w-full shadow-sm"
							>
								<Icon
									icon="material-symbols:search"
									class="w-4 h-4 text-gray-500 dark:text-white"
								/>
								<span class="flex-1 text-left truncate font-mono">
									{#if currentSearchQuery}
										{#each getHighlightedText(currentSearchQuery) as part, i (i)}
											<span class={part.class}>{part.text}</span>
										{/each}
									{:else}
										<span class="text-gray-600 dark:text-gray-400">{m.nav_search_everything()}</span
										>
									{/if}
								</span>
								<kbd
									class="px-2 py-0.5 text-xs font-semibold text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded flex-shrink-0"
								>
									{isMac ? '⌘' : 'Ctrl'}K
								</kbd>
							</button>
						</div>

						<!-- Right side: Menu Items -->
						<div class="flex items-center space-x-4 flex-shrink-0">
							<!-- Mobile Search Button -->
							<button
								onclick={openSearchModal}
								class="sm:hidden p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 transition-colors"
								aria-label={m.common_search()}
							>
								<Icon icon="material-symbols:search" class="w-5 h-5" />
							</button>

							<a
								href={resolve('/discover')}
								class="hidden lg:inline-flex items-center text-sm font-medium whitespace-nowrap focus:outline-none transition-colors px-3 py-2 rounded-md {$page.url.pathname.startsWith(
									'/discover'
								)
									? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
									: 'text-gray-600 dark:text-gray-300 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-700'}"
							>
								<span class="w-4 h-4 mr-1.5">
									<Icon icon="material-symbols:database" />
								</span>
								<span>{m.nav_discover()}</span>
							</a>

							{#if canQueryData}
								<a
									href={resolve('/query')}
									class="hidden lg:inline-flex items-center text-sm font-medium whitespace-nowrap focus:outline-none transition-colors px-3 py-2 rounded-md {$page.url.pathname.startsWith(
										'/query'
									)
										? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
										: 'text-gray-600 dark:text-gray-300 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-700'}"
								>
									<span class="w-4 h-4 mr-1.5">
										<Icon icon="material-symbols:terminal" />
									</span>
									<span>{m.nav_query()}</span>
								</a>
							{/if}

							<a
								href={resolve('/insights')}
								class="hidden lg:inline-flex items-center text-sm font-medium whitespace-nowrap focus:outline-none transition-colors px-3 py-2 rounded-md {$page.url.pathname.startsWith(
									'/insights'
								)
									? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
									: 'text-gray-600 dark:text-gray-300 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-700'}"
							>
								<span class="w-4 h-4 mr-1.5">
									<Icon icon="material-symbols:insights" />
								</span>
								<span>{m.nav_insights()}</span>
							</a>

							<a
								href={resolve('/runs')}
								class="hidden lg:inline-flex items-center text-sm font-medium whitespace-nowrap focus:outline-none transition-colors px-3 py-2 rounded-md {$page.url.pathname.startsWith(
									'/runs'
								)
									? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
									: 'text-gray-600 dark:text-gray-300 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-700'}"
							>
								<span class="w-4 h-4 mr-1.5">
									<Icon icon="material-symbols:play-circle-outline" />
								</span>
								<span>{m.nav_runs()}</span>
							</a>

							<div class="relative hidden lg:block">
								<button
									type="button"
									onclick={(e) => {
										e.stopPropagation();
										toggleGovernance();
									}}
									class="hidden lg:inline-flex items-center text-sm font-medium whitespace-nowrap focus:outline-none transition-colors px-3 py-2 rounded-md {$page.url.pathname.startsWith(
										'/glossary'
									) ||
									$page.url.pathname.startsWith('/products') ||
									$page.url.pathname.startsWith('/asset-rules') ||
									$page.url.pathname.startsWith('/query-audit')
										? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
										: 'text-gray-600 dark:text-gray-300 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-700'}"
								>
									<span class="w-4 h-4 mr-1.5">
										<Icon icon="material-symbols:shield-outline" />
									</span>
									<span>{m.nav_governance()}</span>
									<Icon
										icon="material-symbols:keyboard-arrow-down"
										class="w-4 h-4 ml-1 transition-transform {isGovernanceOpen ? 'rotate-180' : ''}"
									/>
								</button>
								{#if isGovernanceOpen}
									<div
										class="origin-top-left absolute left-0 mt-2 w-48 rounded-md glass-dropdown shadow-lg ring-1 ring-black ring-opacity-5 z-50"
										role="menu"
									>
										<a
											href={resolve('/glossary')}
											class="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 {$page.url.pathname.startsWith(
												'/glossary'
											)
												? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
												: ''}"
											role="menuitem"
										>
											<Icon icon="material-symbols:book" class="w-4 h-4" />
											{m.nav_glossary()}
										</a>
										<a
											href={resolve('/products')}
											class="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 {$page.url.pathname.startsWith(
												'/products'
											)
												? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
												: ''}"
											role="menuitem"
										>
											<Icon icon="material-symbols:inventory-2" class="w-4 h-4" />
											{m.nav_data_products()}
										</a>
										<a
											href={resolve('/asset-rules')}
											class="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 {$page.url.pathname.startsWith(
												'/asset-rules'
											)
												? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
												: ''}"
											role="menuitem"
										>
											<Icon icon="material-symbols:rule-settings" class="w-4 h-4" />
											{m.nav_asset_rules()}
										</a>
										{#if canViewQueryLog}
											<a
												href={resolve('/query-audit')}
												class="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 {$page.url.pathname.startsWith(
													'/query-audit'
												)
													? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
													: ''}"
												role="menuitem"
											>
												<Icon icon="material-symbols:history" class="w-4 h-4" />
												{m.nav_query_audit()}
											</a>
										{/if}
									</div>
								{/if}
							</div>

							{#if $auth}
								<NotificationBell />
							{/if}

							{#if canSeeSettings}
								<a
									href={resolve('/settings')}
									class="relative hidden rounded-lg p-2 transition-colors lg:inline-flex {$page.url.pathname.startsWith(
										'/settings'
									)
										? 'bg-gray-100 text-gray-900 dark:bg-gray-800 dark:text-gray-100'
										: 'text-gray-500 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-100'}"
									aria-label={settingsLabel}
									title={settingsLabel}
								>
									<Icon icon="material-symbols:settings-outline" class="h-5 w-5" />
									{#if $settingsAttention > 0}
										<span
											class="absolute right-1.5 top-1.5 h-2 w-2 rounded-full bg-red-500 ring-2 ring-white dark:ring-gray-900"
											aria-hidden="true"
										></span>
									{/if}
								</a>
							{/if}

							<button
								type="button"
								class="rounded-lg p-2 text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-800 lg:hidden"
								aria-label={m.nav_menu()}
								aria-expanded={isMobileMenuOpen}
								onclick={(e) => {
									e.stopPropagation();
									isMobileMenuOpen = !isMobileMenuOpen;
								}}
							>
								<Icon
									icon={isMobileMenuOpen ? 'material-symbols:close' : 'material-symbols:menu'}
									class="h-5 w-5"
								/>
							</button>

							<div class="relative">
								<div>
									<button
										class="max-w-xs flex items-center text-sm rounded-full focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-earthy-terracotta-500"
										id="user-menu"
										aria-haspopup="true"
										onclick={(e) => {
											e.stopPropagation();
											toggleDropdown();
										}}
									>
										{#if $userProfile}
											<Avatar
												name={$userProfile.name || $userProfile.username}
												profilePicture={$userProfile.profile_picture}
												size="sm"
											/>
										{:else}
											<div
												class="h-8 w-8 rounded-full bg-gray-200 dark:bg-gray-700 flex items-center justify-center"
											>
												<UserIcon class="h-5 w-5 text-gray-600 dark:text-gray-300" />
											</div>
										{/if}
									</button>
								</div>
								{#if isDropdownOpen}
									<div
										class="origin-top-right absolute right-0 mt-2 w-56 overflow-visible rounded-md glass-dropdown py-1 shadow-lg ring-1 ring-black ring-opacity-5 z-50"
										role="menu"
										aria-orientation="vertical"
										aria-labelledby="user-menu"
										onclick={(event) => event.stopPropagation()}
									>
										{#if $auth}
											<!-- User is authenticated -->
											<a
												href={resolve('/account')}
												class="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
												role="menuitem"
											>
												<Icon icon="material-symbols:person-outline" class="h-4 w-4" />
												{m.nav_account()}
											</a>
											<button
												type="button"
												onclick={handleLogout}
												class="w-full text-left block px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
												role="menuitem"
											>
												{m.nav_logout()}
											</button>
										{:else}
											<!-- User is in anonymous mode -->
											<button
												type="button"
												onclick={handleLogin}
												class="w-full text-left block px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
												role="menuitem"
											>
												{m.nav_login()}
											</button>
										{/if}
										<LanguageSelector variant="menu" />
									</div>
								{/if}
							</div>
						</div>
					</div>
				</div>
			</nav>
			{#if isMobileMenuOpen}
				<!-- Below lg the top bar keeps only search, notifications and the avatar; everything else lives here -->
				<div
					class="glass-navbar fixed inset-x-0 top-16 z-40 max-h-[calc(100dvh-4rem)] overflow-y-auto border-b border-gray-200 px-4 py-3 shadow-lg dark:border-gray-800 lg:hidden"
				>
					<nav class="grid gap-1 text-sm" aria-label={m.nav_menu()}>
						{#each [{ href: '/discover', label: m.nav_discover(), icon: 'material-symbols:database', show: true }, { href: '/query', label: m.nav_query(), icon: 'material-symbols:terminal', show: canQueryData }, { href: '/insights', label: m.nav_insights(), icon: 'material-symbols:insights', show: true }, { href: '/runs', label: m.nav_runs(), icon: 'material-symbols:play-circle-outline', show: true }, { href: '/glossary', label: m.nav_glossary(), icon: 'material-symbols:book', show: true }, { href: '/products', label: m.nav_data_products(), icon: 'material-symbols:inventory-2', show: true }, { href: '/asset-rules', label: m.nav_asset_rules(), icon: 'material-symbols:rule-settings', show: true }, { href: '/query-audit', label: m.nav_query_audit(), icon: 'material-symbols:history', show: canViewQueryLog }, { href: '/settings', label: m.nav_settings(), icon: 'material-symbols:settings-outline', show: canSeeSettings }] as item (item.href)}
							{#if item.show}
								<a
									href={resolve(item.href as '/')}
									class="flex items-center gap-3 rounded-lg px-3 py-2.5 {$page.url.pathname.startsWith(
										item.href
									)
										? 'bg-gray-100 font-medium text-gray-900 dark:bg-gray-800 dark:text-gray-100'
										: 'text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800'}"
								>
									<Icon icon={item.icon} class="h-5 w-5 text-gray-400" />
									{item.label}
								</a>
							{/if}
						{/each}
					</nav>
				</div>
			{/if}
		{/if}

		<main class="flex-1 overflow-y-auto flex flex-col">
			<div class="flex-1">
				<slot />
			</div>
			<!-- The query and glossary workspaces fill the viewport, so they have no footer -->
			{#if !$page.url.pathname.startsWith('/login') && !$page.url.pathname.startsWith('/query') && !$page.url.pathname.startsWith('/glossary')}
				<Footer />
			{/if}
		</main>
	</div>
{/key}

<!-- Search Modal -->
{#if showSearchModal}
	<div
		class="fixed inset-0 bg-black/50 dark:bg-black/70 backdrop-blur-sm z-50 flex items-start justify-center pt-[15vh] px-4"
		onclick={() => (showSearchModal = false)}
		onkeydown={(e) => e.key === 'Escape' && (showSearchModal = false)}
		role="button"
		tabindex="-1"
	>
		<div
			class="w-full max-w-2xl glass-modal rounded-xl shadow-2xl overflow-visible"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
			role="dialog"
			tabindex="-1"
		>
			<div class="p-6 search-modal-input">
				{#if SearchComponent}
					<svelte:component
						this={SearchComponent}
						bind:this={searchInput}
						autofocus={true}
						initialQuery={currentSearchQuery}
						onNavigate={() => (showSearchModal = false)}
					/>
				{/if}
			</div>
		</div>
	</div>
{/if}

<ToastContainer />
