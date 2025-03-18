<script lang="ts">
	import { onMount } from 'svelte';
	import '../app.css';
	import { page } from '$app/stores';
	import { auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import UserIcon from '~icons/heroicons/user-16-solid';

	let showBanner: boolean | null = null;
	let isDropdownOpen = false;
	let isAdmin = false;

	onMount(() => {
		const bannerState = localStorage.getItem('alphaBannerDismissed');
		showBanner = bannerState !== 'true';

		// Check if user has admin role
		if (browser && $auth) {
			isAdmin = auth.hasRole('admin');
		}
	});

	// Update isAdmin whenever auth changes
	$: if (browser && $auth) {
		isAdmin = auth.hasRole('admin');
	}

	$: if (browser && $auth !== undefined) {
		if ($auth && $page.url.pathname.startsWith('/login')) {
			goto('/');
		} else if (!$auth && !$page.url.pathname.startsWith('/login')) {
			goto('/login');
		}
	}

	function toggleDropdown() {
		isDropdownOpen = !isDropdownOpen;
	}

	function closeDropdown() {
		isDropdownOpen = false;
	}

	function dismissBanner() {
		showBanner = false;
		localStorage.setItem('alphaBannerDismissed', 'true');
	}

	function handleLogout() {
		auth.clearToken();
		goto('/login');
	}
</script>

<svelte:window on:click={closeDropdown} />

<div class="h-screen flex flex-col">
	{#if showBanner !== null && showBanner}
		<div class="bg-orange-500 text-white flex-none">
			<div class="max-w-14xl mx-auto py-3 px-4 sm:px-6 lg:px-8">
				<div class="flex items-center justify-between flex-wrap">
					<div class="w-0 flex-1 flex items-center">
						<span class="flex p-2">
							<svg
								class="h-6 w-6 text-white"
								xmlns="http://www.w3.org/2000/svg"
								fill="none"
								viewBox="0 0 24 24"
								stroke="currentColor"
							>
								<path
									stroke-linecap="round"
									stroke-linejoin="round"
									stroke-width="2"
									d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
								/>
							</svg>
						</span>
						<p class="ml-3 font-medium">
							This software is in alpha state. You may encounter bugs and data may not be accurate.
						</p>
					</div>
					<div class="shrink-0">
						<button
							type="button"
							class="flex rounded-md p-2 hover:bg-orange-600 transition-colors focus:outline-none focus:ring-2 focus:ring-white"
							on:click={dismissBanner}
						>
							<svg
								class="h-5 w-5 text-white"
								xmlns="http://www.w3.org/2000/svg"
								viewBox="0 0 20 20"
								fill="currentColor"
							>
								<path
									fill-rule="evenodd"
									d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
									clip-rule="evenodd"
								/>
							</svg>
						</button>
					</div>
				</div>
			</div>
		</div>
	{/if}
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
