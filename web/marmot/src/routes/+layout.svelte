<script lang="ts">
	import { onMount } from 'svelte';
	import '../app.css';
	import { page } from '$app/stores';
	import { auth, isAnonymousMode } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import UserIcon from '~icons/heroicons/user-16-solid';

	let showBanner: boolean | null = null;
	let isDropdownOpen = false;
	let isAdmin = false;
	let checkingAnonymousMode = true;
	let manualNavigation = false;

	const appName = 'Marmot';

	onMount(async () => {
		checkingAnonymousMode = true;
		await auth.checkAnonymousMode();
		checkingAnonymousMode = false;

		if (browser && $auth) {
			isAdmin = auth.hasRole('admin');
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
	}

	function closeDropdown() {
		isDropdownOpen = false;
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
		return word.charAt(0).toUpperCase() + word.slice(1);
	}

	$: pathSegments = $page.url.pathname.split('/').filter(Boolean);
	$: processedSegments = pathSegments.map((segment, index) =>
		index < 2 ? capitalizeWord(segment) : segment
	);
	$: reversedSegments = [...processedSegments].reverse();
	$: pageTitle = reversedSegments.join(' - ');
	$: dynamicTitle = $page.url.pathname === '/' ? 'Marmot' : `${pageTitle} - Marmot`;
</script>

<svelte:head>
	<title>{dynamicTitle}</title>
</svelte:head>

<svelte:window on:click={closeDropdown} />

<div class="h-screen flex flex-col">
	{#if !$page.url.pathname.startsWith('/login')}
		<nav class="bg-earthy-brown-50 dark:bg-gray-900 flex-none">
			<div class="max-w-14xl mx-auto px-4 sm:px-6 lg:px-8">
				<div class="flex items-center justify-between h-16">
					<div class="flex items-center">
						{#if $page.url.pathname !== '/'}
							<a href="/" class="flex-shrink-0">
								<img class="h-8 w-8" src="/images/marmot.svg" alt="Logo" />
							</a>
						{/if}
					</div>
					<div class="ml-4 flex items-center md:ml-6">
						<div class="relative">
							<div>
								<button
									class="max-w-xs flex items-center text-sm rounded-full focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
									id="user-menu"
									aria-haspopup="true"
									on:click|stopPropagation={toggleDropdown}
								>
									<div
										class="h-8 w-8 rounded-full bg-gray-200 dark:bg-gray-700 flex items-center justify-center"
									>
										<UserIcon class="h-5 w-5 text-gray-600 dark:text-gray-300" />
									</div>
								</button>
							</div>
							{#if isDropdownOpen}
								<div
									class="origin-top-right absolute right-0 mt-2 w-48 rounded-md shadow dark:shadow-white/10 bg-earthy-brown-50 dark:bg-gray-800 ring-1 ring-black ring-opacity-5"
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
										<a
											href="#"
											on:click|preventDefault={handleLogout}
											class="block px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
											role="menuitem"
										>
											Logout
										</a>
									{:else}
										<!-- User is in anonymous mode -->
										<a
											href="#"
											on:click|preventDefault={handleLogin}
											class="block px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
											role="menuitem"
										>
											Login
										</a>
									{/if}
								</div>
							{/if}
						</div>
					</div>
				</div>
			</div>
		</nav>
	{/if}

	<main class="{showBanner ? 'h-[calc(100vh-104px)]' : 'h-[calc(100vh-64px)]'} overflow-y-auto">
		<slot />
	</main>
</div>
