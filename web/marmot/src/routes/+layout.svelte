<script lang="ts">
	import { onMount } from 'svelte';
	import '../app.css';
	import { page } from '$app/stores';
	import { auth, isAnonymousMode } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { fetchApi } from '$lib/api';
	import UserIcon from '~icons/heroicons/user-16-solid';
	import Icon from '@iconify/svelte';
	import Banner from '$lib/components/Banner.svelte';
	import Footer from '$lib/components/Footer.svelte';
	import GlobalSearch from '$components/query/GlobalSearch.svelte';
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

	interface UserProfile {
		id: string;
		username: string;
		name: string;
		profile_picture?: string;
	}

	let bannerConfig: BannerConfig | null = null;
	let isDropdownOpen = false;
	let isGovernanceOpen = false;
	let isAdmin = false;
	let checkingAnonymousMode = true;
	let manualNavigation = false;
	let showSearchModal = false;
	let searchInput: GlobalSearch;
	let userProfile: UserProfile | null = null;

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

	function openSearchModal() {
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
		checkingAnonymousMode = true;
		await auth.checkAnonymousMode();
		checkingAnonymousMode = false;

		if (browser && $auth) {
			isAdmin = auth.hasRole('admin');

			try {
				const profileRes = await fetchApi('/users/me');
				if (profileRes.ok) {
					userProfile = await profileRes.json();
				}
			} catch (err) {
				console.error('Failed to fetch user profile:', err);
			}
		}

		if (browser) {
			try {
				const response = await fetch('/api/v1/ui/config');
				if (response.ok) {
					const data = await response.json();
					bannerConfig = data.banner;
				}
			} catch (error) {
				console.error('Failed to fetch UI config:', error);
			}
		}
	});

	$: if (browser && $auth) {
		isAdmin = auth.hasRole('admin');
	}

	function withManualNav(fn: () => void) {
		manualNavigation = true;
		fn();
		setTimeout(() => {
			manualNavigation = false;
		}, 100);
	}

	$: if (browser && !checkingAnonymousMode && !manualNavigation) {
		if ($auth && $page.url.pathname.startsWith('/login')) {
			goto('/');
		} else if (!$auth && !$page.url.pathname.startsWith('/login') && !$isAnonymousMode) {
			goto('/login');
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
			window.location.href = '/';
		});
	}

	function handleLogin() {
		withManualNav(() => {
			goto('/login');
		});
	}

	function capitalizeWord(word: string): string {
		return word
			.split('-')
			.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
			.join(' ');
	}

	$: pathSegments = $page.url.pathname.split('/').filter(Boolean);
	$: decodedSegments = pathSegments.map((segment) => decodeURIComponent(segment));
	$: processedSegments = decodedSegments.map((segment, index) =>
		index < 2 ? capitalizeWord(segment) : segment
	);
	$: reversedSegments = [...processedSegments].reverse();
	$: pageTitle = reversedSegments.join(' - ');
	$: dynamicTitle = $page.url.pathname === '/' ? 'Marmot' : `${pageTitle} - Marmot`;
</script>

<svelte:head>
	<title>{dynamicTitle}</title>
</svelte:head>

<svelte:window onclick={closeDropdown} onkeydown={handleGlobalKeydown} />

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
			<div class="max-w-14xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="flex items-center justify-between h-16 gap-6">
					<!-- Logo -->
					<div class="flex items-center flex-shrink-0">
						<a href="/" class="flex-shrink-0 hover:opacity-80 transition-opacity">
							<img src="/images/marmot.svg" alt="Marmot" class="h-8 w-8" />
						</a>
					</div>

					<!-- Centered Search (desktop only) -->
					<div class="hidden sm:flex flex-1 justify-center max-w-3xl mx-auto">
						<button
							onclick={openSearchModal}
							class="flex items-center gap-2 px-4 py-2 text-sm bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-600 rounded-lg hover:border-gray-300 dark:hover:border-gray-500 transition-colors w-full shadow-sm"
						>
							<Icon icon="material-symbols:search" class="w-4 h-4 text-gray-500 dark:text-white" />
							<span class="flex-1 text-left truncate font-mono">
								{#if currentSearchQuery}
									{#each getHighlightedText(currentSearchQuery) as part}
										<span class={part.class}>{part.text}</span>
									{/each}
								{:else}
									<span class="text-gray-600 dark:text-gray-400">Search everything...</span>
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
							aria-label="Search"
						>
							<Icon icon="material-symbols:search" class="w-5 h-5" />
						</button>

						<a
							href="/discover"
							class="inline-flex items-center text-sm font-medium whitespace-nowrap focus:outline-none transition-colors px-4 py-2 rounded-md {$page.url.pathname.startsWith(
								'/discover'
							)
								? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
								: 'text-gray-600 dark:text-gray-300 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-700'}"
						>
							<span class="w-4 h-4 mr-1.5">
								<Icon icon="material-symbols:database" />
							</span>
							<span>Discover</span>
						</a>

						<a
							href="/runs"
							class="inline-flex items-center text-sm font-medium whitespace-nowrap focus:outline-none transition-colors px-4 py-2 rounded-md {$page.url.pathname.startsWith(
								'/runs'
							)
								? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
								: 'text-gray-600 dark:text-gray-300 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-700'}"
						>
							<span class="w-4 h-4 mr-1.5">
								<Icon icon="material-symbols:play-circle-outline" />
							</span>
							<span>Runs</span>
						</a>

						<a
							href="/metrics"
							class="inline-flex items-center text-sm font-medium whitespace-nowrap focus:outline-none transition-colors px-4 py-2 rounded-md {$page.url.pathname.startsWith(
								'/metrics'
							)
								? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
								: 'text-gray-600 dark:text-gray-300 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-700'}"
						>
							<span class="w-4 h-4 mr-1.5">
								<Icon icon="material-symbols:area-chart-rounded" />
							</span>
							<span>Metrics</span>
						</a>

						<div class="relative">
							<button
								type="button"
								onclick={(e) => {
									e.stopPropagation();
									toggleGovernance();
								}}
								class="inline-flex items-center text-sm font-medium whitespace-nowrap focus:outline-none transition-colors px-4 py-2 rounded-md {$page.url.pathname.startsWith(
									'/glossary'
								) ||
								$page.url.pathname.startsWith('/products') ||
								$page.url.pathname.startsWith('/asset-rules')
									? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
									: 'text-gray-600 dark:text-gray-300 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-700'}"
							>
								<span class="w-4 h-4 mr-1.5">
									<Icon icon="material-symbols:shield-outline" />
								</span>
								<span>Governance</span>
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
										href="/glossary"
										class="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 {$page.url.pathname.startsWith(
											'/glossary'
										)
											? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
											: ''}"
										role="menuitem"
									>
										<Icon icon="material-symbols:book" class="w-4 h-4" />
										Glossary
									</a>
									<a
										href="/products"
										class="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 {$page.url.pathname.startsWith(
											'/products'
										)
											? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
											: ''}"
										role="menuitem"
									>
										<Icon icon="material-symbols:inventory-2" class="w-4 h-4" />
										Data Products
									</a>
									<a
										href="/asset-rules"
										class="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 {$page.url.pathname.startsWith(
											'/asset-rules'
										)
											? 'text-earthy-terracotta-700 dark:text-earthy-terracotta-700'
											: ''}"
										role="menuitem"
									>
										<Icon icon="material-symbols:rule-settings" class="w-4 h-4" />
										Asset Rules
									</a>
								</div>
							{/if}
						</div>

						{#if $auth}
							<NotificationBell />
						{/if}

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
									{#if userProfile}
										<Avatar
											name={userProfile.name || userProfile.username}
											profilePicture={userProfile.profile_picture}
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
									class="origin-top-right absolute right-0 mt-2 w-48 rounded-md glass-dropdown shadow-lg ring-1 ring-black ring-opacity-5 z-50"
									role="menu"
									aria-orientation="vertical"
									aria-labelledby="user-menu"
								>
									{#if $auth}
										<!-- User is authenticated -->
										<a
											href="/profile"
											class="block px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
											role="menuitem">Profile</a
										>
										{#if isAdmin}
											<a
												href="/admin"
												class="block px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
												role="menuitem">Admin</a
											>
										{/if}
										<button
											type="button"
											onclick={handleLogout}
											class="w-full text-left block px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
											role="menuitem"
										>
											Logout
										</button>
									{:else}
										<!-- User is in anonymous mode -->
										<button
											type="button"
											onclick={handleLogin}
											class="w-full text-left block px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
											role="menuitem"
										>
											Login
										</button>
									{/if}
								</div>
							{/if}
						</div>
					</div>
				</div>
			</div>
		</nav>
	{/if}

	<main class="flex-1 overflow-y-auto flex flex-col">
		<div class="flex-1">
			<slot />
		</div>
		{#if !$page.url.pathname.startsWith('/login')}
			<Footer />
		{/if}
	</main>
</div>

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
				<GlobalSearch
					bind:this={searchInput}
					autofocus={true}
					initialQuery={currentSearchQuery}
					onNavigate={() => (showSearchModal = false)}
				/>
			</div>
		</div>
	</div>
{/if}

<ToastContainer />
