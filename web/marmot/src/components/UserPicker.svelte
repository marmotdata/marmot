<script lang="ts">
	import { fetchApi } from '$lib/api';
	import Icon from '@iconify/svelte';
	import { onMount } from 'svelte';

	export let selectedUserIds: string[] = [];
	export let onChange: (userIds: string[], users: User[]) => void = () => {};
	export let placeholder = 'Search users...';
	export let className = '';
	export let minSearchLength = 2;
	export let disabled = false;

	interface User {
		id: string;
		username: string;
		name: string;
	}

	let searchResults: User[] = [];
	let selectedUsersCache: Map<string, User> = new Map();
	let selectedUsers: User[] = [];
	let searchQuery = '';
	let isOpen = false;
	let isLoading = false;
	let searchTimeout: NodeJS.Timeout;
	let dropdownRef: HTMLDivElement;
	let inputRef: HTMLInputElement;
	let focusedIndex = -1;

	onMount(() => {
		document.addEventListener('click', handleClickOutside);
		return () => {
			document.removeEventListener('click', handleClickOutside);
		};
	});

	async function loadSelectedUsers() {
		if (selectedUserIds.length === 0) {
			selectedUsers = [];
			return;
		}

		for (const userId of selectedUserIds) {
			if (!selectedUsersCache.has(userId)) {
				try {
					const response = await fetchApi(`/users/${userId}`);
					if (response.ok) {
						const user = await response.json();
						selectedUsersCache.set(userId, user);
					}
				} catch (err) {
					console.error(`Failed to fetch user ${userId}:`, err);
				}
			}
		}

		// Update selectedUsers array
		selectedUsers = selectedUserIds
			.map(id => selectedUsersCache.get(id))
			.filter((user): user is User => user !== undefined);
	}

	$: {
		// React to changes in selectedUserIds
		selectedUserIds;
		loadSelectedUsers();
	}

	function handleClickOutside(event: MouseEvent) {
		if (dropdownRef && !dropdownRef.contains(event.target as Node)) {
			isOpen = false;
		}
	}

	async function handleSearch(e: Event) {
		const target = e.target as HTMLInputElement;
		searchQuery = target.value;

		clearTimeout(searchTimeout);

		if (searchQuery.length < minSearchLength) {
			searchResults = [];
			focusedIndex = -1;
			isOpen = searchQuery.length > 0;
			return;
		}

		searchTimeout = setTimeout(async () => {
			await performSearch();
		}, 300);
	}

	function handleKeyDown(e: KeyboardEvent) {
		const availableResults = searchResults.filter(u => !selectedUserIds.includes(u.id));

		if (e.key === 'ArrowDown') {
			e.preventDefault();
			focusedIndex = Math.min(focusedIndex + 1, availableResults.length - 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			focusedIndex = Math.max(focusedIndex - 1, -1);
		} else if (e.key === 'Enter' && focusedIndex >= 0 && focusedIndex < availableResults.length) {
			e.preventDefault();
			addUser(availableResults[focusedIndex].id);
		} else if (e.key === 'Escape') {
			e.preventDefault();
			closeDropdown();
		}
	}

	async function performSearch() {
		isLoading = true;
		isOpen = true;

		try {
			const response = await fetchApi(`/users?query=${encodeURIComponent(searchQuery)}&limit=50`);
			if (response.ok) {
				const data = await response.json();
				searchResults = data.users || [];

				searchResults.forEach(user => {
					selectedUsersCache.set(user.id, user);
				});

				focusedIndex = -1; // Reset focus when results change
			}
		} catch (err) {
			console.error('Failed to search users:', err);
			searchResults = [];
			focusedIndex = -1;
		} finally {
			isLoading = false;
		}
	}

	function addUser(userId: string) {
		selectedUserIds = [...selectedUserIds, userId];
		// User should be in cache from search results
		const user = selectedUsersCache.get(userId);
		if (user) {
			selectedUsers = [...selectedUsers, user];
			onChange(selectedUserIds, selectedUsers);
		}
		searchQuery = '';
		searchResults = [];
		focusedIndex = -1;
		isOpen = false;
	}

	function removeUser(userId: string) {
		selectedUserIds = selectedUserIds.filter((id) => id !== userId);
		selectedUsers = selectedUsers.filter((u) => u.id !== userId);
		onChange(selectedUserIds, selectedUsers);
	}

	function openAddUser() {
		if (disabled) return;
		isOpen = true;
		setTimeout(() => inputRef?.focus(), 50);
	}

	function closeDropdown() {
		isOpen = false;
		searchQuery = '';
		searchResults = [];
		focusedIndex = -1;
	}
</script>

<div bind:this={dropdownRef} class="relative {className}">
	<div class="flex flex-wrap items-center gap-2">
		{#each selectedUsers as user (user.id)}
			<div
				class="group relative flex items-center gap-2 px-2 py-1.5 rounded-full bg-gray-200 dark:bg-gray-700 flex-shrink-0"
				title="@{user.username}"
			>
				<div
					class="w-6 h-6 rounded-full bg-gray-300 dark:bg-gray-600 flex items-center justify-center text-gray-700 dark:text-gray-300 text-xs font-medium flex-shrink-0"
				>
					{user.name.charAt(0).toUpperCase()}
				</div>
				<span class="text-sm text-gray-900 dark:text-gray-100 pr-1">{user.name}</span>
				{#if !disabled}
					<button
						type="button"
						on:click={() => removeUser(user.id)}
						class="absolute inset-0 rounded-full bg-red-500 hover:bg-red-600 text-white opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center"
						title="Remove {user.name}"
					>
						<Icon icon="material-symbols:close" class="h-5 w-5" />
					</button>
				{/if}
			</div>
		{/each}

		{#if !disabled}
			<button
				type="button"
				on:click={openAddUser}
				class="w-8 h-8 rounded-full border-2 border-dashed border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 transition-colors flex items-center justify-center flex-shrink-0"
				title="Add user"
			>
				<Icon icon="material-symbols:add" class="h-4 w-4" />
			</button>
		{/if}
	</div>

	{#if isOpen}
		<div
			class="absolute z-50 mt-2 w-full bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg"
		>
			<div class="p-3 border-b border-gray-200 dark:border-gray-700">
				<input
					bind:this={inputRef}
					type="text"
					bind:value={searchQuery}
					on:input={handleSearch}
					on:keydown={handleKeyDown}
					placeholder={placeholder}
					class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-1 focus:ring-gray-400 dark:bg-gray-700 dark:text-gray-100"
				/>
			</div>

			<div class="max-h-60 overflow-auto">
				{#if searchQuery.length < minSearchLength}
					<div class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
						Type at least {minSearchLength} characters to search
					</div>
				{:else if isLoading}
					<div class="px-4 py-8 flex items-center justify-center gap-2 text-sm text-gray-500 dark:text-gray-400">
						<div class="animate-spin rounded-full h-4 w-4 border-2 border-gray-300 border-t-gray-600"></div>
						Searching...
					</div>
				{:else if searchResults.length === 0}
					<div class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
						No users found
					</div>
				{:else}
					{@const availableUsers = searchResults.filter(u => !selectedUserIds.includes(u.id))}
					<div class="py-1">
						{#each availableUsers as user, index (user.id)}
							<button
								type="button"
								on:click={() => addUser(user.id)}
								class="w-full flex items-center gap-3 px-4 py-2.5 transition-colors {index === focusedIndex
									? 'bg-gray-100 dark:bg-gray-700'
									: 'hover:bg-gray-50 dark:hover:bg-gray-700/50'}"
							>
								<div
									class="w-8 h-8 rounded-full bg-gray-200 dark:bg-gray-700 flex items-center justify-center text-gray-700 dark:text-gray-300 text-sm font-medium flex-shrink-0"
								>
									{user.name.charAt(0).toUpperCase()}
								</div>
								<div class="flex-1 min-w-0 text-left">
									<div class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
										{user.name}
									</div>
									<div class="text-xs text-gray-500 dark:text-gray-400 truncate">
										@{user.username}
									</div>
								</div>
							</button>
						{/each}
					</div>
				{/if}
			</div>

			<div class="p-2 border-t border-gray-200 dark:border-gray-700">
				<button
					type="button"
					on:click={closeDropdown}
					class="w-full px-3 py-1.5 text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
				>
					Close
				</button>
			</div>
		</div>
	{/if}
</div>
