<script lang="ts">
	import { fetchApi } from '$lib/api';
	import Icon from '@iconify/svelte';
	import { onMount } from 'svelte';
	import { Users, User } from 'lucide-svelte';
	import Avatar from '$components/user/Avatar.svelte';
	import { createKeyboardNavigationState } from '$lib/keyboard';

	export let selectedOwners: Owner[] | null = [];
	export let onChange: (owners: Owner[]) => void = () => {};
	export let placeholder = 'Search users or teams...';
	export let className = '';
	export let minSearchLength = 2;
	export let disabled = false;
	export let userOnly = false; // If true, only show users in search
	export let hideAddButton = false; // If true, hide the add button (for programmatic opening)
	export let hideSelectedOwners = false; // If true, hide the selected owners badges

	$: safeSelectedOwners = selectedOwners || [];
	$: searchPlaceholder = userOnly ? 'Search users...' : placeholder;
	$: availableResults = searchResults.filter(
		(o) => !safeSelectedOwners.some((so) => so.id === o.id && so.type === o.type)
	);

	export interface Owner {
		id: string;
		name: string;
		type: 'user' | 'team';
		username?: string; // For users
		email?: string; // For users
		profile_picture?: string; // For users
	}

	let searchResults: Owner[] = [];
	let searchQuery = '';
	let isOpen = false;
	let isLoading = false;
	let searchTimeout: ReturnType<typeof setTimeout>;
	let dropdownRef: HTMLDivElement;
	let inputRef: HTMLInputElement;
	let focusedIndex = -1;

	onMount(() => {
		document.addEventListener('click', handleClickOutside);
		return () => {
			document.removeEventListener('click', handleClickOutside);
		};
	});

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

	const { handleKeydown: handleKeyDown } = createKeyboardNavigationState(
		() => availableResults,
		() => focusedIndex,
		(i) => (focusedIndex = i),
		{
			onSelect: addOwner,
			onEscape: closeDropdown
		}
	);

	async function performSearch() {
		isLoading = true;
		isOpen = true;

		try {
			const response = await fetchApi(
				`/owners/search?q=${encodeURIComponent(searchQuery)}&limit=50`
			);
			if (response.ok) {
				const data = await response.json();
				let results = data.owners || [];

				// Filter to only users if userOnly mode is enabled
				if (userOnly) {
					results = results.filter((o: Owner) => o.type === 'user');
				}

				searchResults = results;
				focusedIndex = -1; // Reset focus when results change
			}
		} catch (err) {
			console.error('Failed to search owners:', err);
			searchResults = [];
			focusedIndex = -1;
		} finally {
			isLoading = false;
		}
	}

	function addOwner(owner: Owner) {
		const newOwners = [...safeSelectedOwners, owner];
		selectedOwners = newOwners;
		onChange(newOwners);
		searchQuery = '';
		searchResults = [];
		focusedIndex = -1;
		isOpen = false;
	}

	function removeOwner(owner: Owner) {
		const newOwners = safeSelectedOwners.filter(
			(o) => !(o.id === owner.id && o.type === owner.type)
		);
		selectedOwners = newOwners;
		onChange(newOwners);
	}

	function openAddOwner() {
		if (disabled) return;
		isOpen = true;
		setTimeout(() => inputRef?.focus(), 50);
	}

	// Expose method to parent components
	export function open() {
		openAddOwner();
	}

	function closeDropdown() {
		isOpen = false;
		searchQuery = '';
		searchResults = [];
		focusedIndex = -1;
	}

	function getOwnerInitial(owner: Owner): string {
		return owner.name.charAt(0).toUpperCase();
	}

	function handleOwnerClick(owner: Owner) {
		if (!disabled && owner.type === 'team') {
			window.location.href = `/teams/${owner.id}`;
		}
	}
</script>

<div bind:this={dropdownRef} class="relative {className}">
	{#if !hideSelectedOwners}
		<div class="flex flex-wrap items-center gap-2">
			{#each safeSelectedOwners as owner (owner.id + '-' + owner.type)}
				<button
					type="button"
					on:click={() => handleOwnerClick(owner)}
					class="group relative flex items-center gap-2 px-2 py-1.5 rounded-full {owner.type ===
					'team'
						? 'bg-blue-100 dark:bg-blue-900 hover:bg-blue-200 dark:hover:bg-blue-800'
						: 'bg-gray-200 dark:bg-gray-700'} flex-shrink-0 {owner.type === 'team' && !disabled
						? 'cursor-pointer'
						: 'cursor-default'} transition-colors"
					title={owner.type === 'user' ? `@${owner.username}` : `Team: ${owner.name}`}
				>
					{#if owner.type === 'team'}
						<div
							class="w-6 h-6 rounded-full bg-blue-200 dark:bg-blue-800 flex items-center justify-center text-gray-700 dark:text-gray-300 text-xs font-medium flex-shrink-0"
						>
							<Users class="h-3 w-3" />
						</div>
					{:else}
						<div class="flex-shrink-0">
							<Avatar name={owner.name} profilePicture={owner.profile_picture} size="xs" />
						</div>
					{/if}
					<span
						class="text-sm {owner.type === 'team'
							? 'text-blue-900 dark:text-blue-100'
							: 'text-gray-900 dark:text-gray-100'} pr-1">{owner.name}</span
					>
					{#if !disabled}
						<button
							type="button"
							on:click|stopPropagation={() => removeOwner(owner)}
							class="absolute inset-0 rounded-full bg-red-500 hover:bg-red-600 text-white opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center z-10"
							title="Remove {owner.name}"
						>
							<Icon icon="material-symbols:close" class="h-5 w-5" />
						</button>
					{/if}
				</button>
			{/each}

			{#if !disabled && !hideAddButton}
				<button
					type="button"
					on:click={openAddOwner}
					class="w-8 h-8 rounded-full border-2 border-dashed border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 transition-colors flex items-center justify-center flex-shrink-0"
					title="Add owner"
				>
					<Icon icon="material-symbols:add" class="h-4 w-4" />
				</button>
			{/if}
		</div>
	{/if}

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
					placeholder={searchPlaceholder}
					class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-1 focus:ring-gray-400 dark:bg-gray-700 dark:text-gray-100"
				/>
			</div>

			<div class="max-h-60 overflow-auto">
				{#if searchQuery.length < minSearchLength}
					<div class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
						Type at least {minSearchLength} characters to search
					</div>
				{:else if isLoading}
					<div
						class="px-4 py-8 flex items-center justify-center gap-2 text-sm text-gray-500 dark:text-gray-400"
					>
						<div
							class="animate-spin rounded-full h-4 w-4 border-2 border-gray-300 border-t-gray-600"
						></div>
						Searching...
					</div>
				{:else if searchResults.length === 0}
					<div class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">
						No users or teams found
					</div>
				{:else}
					{@const availableOwners = searchResults.filter(
						(o) => !safeSelectedOwners.some((so) => so.id === o.id && so.type === o.type)
					)}
					<div class="py-1">
						{#each availableOwners as owner, index (owner.id + '-' + owner.type)}
							<button
								type="button"
								on:click={() => addOwner(owner)}
								class="w-full flex items-center gap-3 px-4 py-2.5 transition-colors {index ===
								focusedIndex
									? 'bg-gray-100 dark:bg-gray-700'
									: 'hover:bg-gray-50 dark:hover:bg-gray-700/50'}"
							>
								{#if owner.type === 'team'}
									<div
										class="w-8 h-8 rounded-full bg-blue-100 dark:bg-blue-900 flex items-center justify-center text-gray-700 dark:text-gray-300 text-sm font-medium flex-shrink-0"
									>
										<Users class="h-4 w-4" />
									</div>
								{:else}
									<div class="flex-shrink-0">
										<Avatar name={owner.name} profilePicture={owner.profile_picture} size="sm" />
									</div>
								{/if}
								<div class="flex-1 min-w-0 text-left">
									<div
										class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate flex items-center gap-2"
									>
										{owner.name}
										{#if owner.type === 'team'}
											<span
												class="text-xs px-1.5 py-0.5 rounded bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200"
											>
												Team
											</span>
										{/if}
									</div>
									{#if owner.username}
										<div class="text-xs text-gray-500 dark:text-gray-400 truncate">
											@{owner.username}
										</div>
									{/if}
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
