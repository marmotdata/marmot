<script lang="ts">
	import { fetchApi } from '$lib/api';
	import IconifyIcon from '@iconify/svelte';
	import Icon from '$components/ui/Icon.svelte';
	import type { Asset } from '$lib/assets/types';

	let {
		show = $bindable(false),
		sourceMrn: _sourceMrn,
		targetMrn: _targetMrn,
		direction,
		position,
		onAdd
	}: {
		show: boolean;
		sourceMrn: string;
		targetMrn: string;
		direction: 'upstream' | 'downstream';
		position: { x: number; y: number };
		onAdd: (assetMrn: string) => Promise<void>;
	} = $props();

	let searchQuery = $state('');
	let searchResults = $state<Asset[]>([]);
	let isSearching = $state(false);
	let isAdding = $state(false);
	let error = $state('');
	let searchTimeout: ReturnType<typeof setTimeout>;
	let searchInputElement: HTMLInputElement;
	let selectedIndex = $state(-1);
	let resultButtons: HTMLButtonElement[] = [];

	$effect(() => {
		if (show) {
			// Reset state when modal opens
			searchQuery = '';
			searchResults = [];
			error = '';
			selectedIndex = -1;
			// Focus the input after a brief delay
			setTimeout(() => {
				searchInputElement?.focus();
			}, 50);
		}
	});

	// Reset selected index when search results change
	$effect(() => {
		// Track searchResults to trigger effect
		void searchResults;
		selectedIndex = -1;
		resultButtons = [];
	});

	// Scroll selected item into view
	$effect(() => {
		if (selectedIndex >= 0 && resultButtons[selectedIndex]) {
			resultButtons[selectedIndex].scrollIntoView({
				block: 'nearest',
				behavior: 'smooth'
			});
		}
	});

	async function searchAssets(query: string) {
		if (!query.trim()) {
			searchResults = [];
			return;
		}

		isSearching = true;
		error = '';

		try {
			const response = await fetchApi(`/assets/search?q=${encodeURIComponent(query)}&limit=20`);

			if (!response.ok) {
				throw new Error('Failed to search assets');
			}

			const data = await response.json();
			searchResults = data.assets || [];
		} catch (err) {
			console.error('Error searching assets:', err);
			error = 'Failed to search assets';
			searchResults = [];
		} finally {
			isSearching = false;
		}
	}

	function handleSearchInput() {
		if (searchTimeout) clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => searchAssets(searchQuery), 300);
	}

	async function handleSelectAsset(asset: Asset) {
		isAdding = true;
		error = '';

		try {
			await onAdd(asset.mrn);
			show = false;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to add lineage';
		} finally {
			isAdding = false;
		}
	}

	function handleClose() {
		if (!isAdding) {
			show = false;
		}
	}

	function handleKeyDown(event: KeyboardEvent) {
		if (isAdding) return;

		switch (event.key) {
			case 'ArrowDown':
				event.preventDefault();
				if (searchResults.length > 0) {
					selectedIndex = Math.min(selectedIndex + 1, searchResults.length - 1);
				}
				break;
			case 'ArrowUp':
				event.preventDefault();
				selectedIndex = Math.max(selectedIndex - 1, -1);
				break;
			case 'Enter':
				event.preventDefault();
				if (selectedIndex >= 0 && selectedIndex < searchResults.length) {
					handleSelectAsset(searchResults[selectedIndex]);
				}
				break;
			case 'Escape':
				event.preventDefault();
				handleClose();
				break;
		}
	}
</script>

{#if show}
	<!-- Invisible backdrop to close on click outside -->
	<div class="fixed inset-0 z-[100]" onclick={handleClose} role="button" tabindex="-1"></div>

	<!-- Inline search box -->
	<div
		class="absolute z-[101] w-96 bg-white dark:bg-gray-800 rounded-lg shadow-2xl border border-gray-200 dark:border-gray-700"
		style="left: {position.x}px; top: {position.y}px;"
		onclick={(e) => e.stopPropagation()}
		role="dialog"
		tabindex="-1"
	>
		<div class="p-4">
			<div class="flex items-center justify-between mb-3">
				<div class="flex items-center gap-2">
					<IconifyIcon
						icon={direction === 'upstream'
							? 'material-symbols:arrow-back-rounded'
							: 'material-symbols:arrow-forward-rounded'}
						class="w-5 h-5 text-earthy-terracotta-700 dark:text-earthy-terracotta-700"
					/>
					<h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
						Add {direction === 'upstream' ? 'Upstream' : 'Downstream'}
					</h3>
				</div>
				<button
					onclick={handleClose}
					disabled={isAdding}
					class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 disabled:opacity-50"
				>
					<IconifyIcon icon="material-symbols:close" class="w-5 h-5" />
				</button>
			</div>

			<div class="relative mb-3">
				<input
					bind:this={searchInputElement}
					type="text"
					bind:value={searchQuery}
					oninput={handleSearchInput}
					onkeydown={handleKeyDown}
					placeholder="Search for an asset..."
					disabled={isAdding}
					class="w-full px-3 py-2 pl-9 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-transparent disabled:opacity-50"
				/>
				<IconifyIcon
					icon="material-symbols:search"
					class="w-4 h-4 text-gray-400 absolute left-2.5 top-1/2 -translate-y-1/2"
				/>
			</div>

			<div class="max-h-80 overflow-y-auto">
				{#if isSearching}
					<div class="flex items-center justify-center py-6">
						<div
							class="animate-spin rounded-full h-6 w-6 border-b-2 border-earthy-terracotta-700"
						></div>
					</div>
				{:else if error}
					<div
						class="p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg"
					>
						<p class="text-xs text-red-800 dark:text-red-200">{error}</p>
					</div>
				{:else if searchQuery && searchResults.length === 0}
					<div class="text-center py-6 text-sm text-gray-500 dark:text-gray-400">
						No assets found
					</div>
				{:else if searchResults.length > 0}
					<div class="space-y-1">
						{#each searchResults as asset, index (asset.id)}
							<button
								bind:this={resultButtons[index]}
								onclick={() => handleSelectAsset(asset)}
								disabled={isAdding}
								class="w-full p-2.5 flex items-center gap-2 hover:bg-gray-50 dark:hover:bg-gray-700 border rounded-lg transition-colors text-left disabled:opacity-50 disabled:cursor-not-allowed {selectedIndex ===
								index
									? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 border-earthy-terracotta-300 dark:border-earthy-terracotta-800'
									: 'border-transparent hover:border-gray-200 dark:hover:border-gray-600'}"
							>
								<div class="flex-shrink-0">
									<Icon
										name={asset.providers?.length === 1 ? asset.providers[0] : asset.type}
										showLabel={false}
										size="sm"
									/>
								</div>
								<div class="flex-1 min-w-0">
									<div class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
										{asset.name}
									</div>
									<div class="text-xs text-gray-500 dark:text-gray-400 truncate">
										{asset.type}
									</div>
								</div>
								{#if isAdding}
									<div
										class="animate-spin rounded-full h-4 w-4 border-b-2 border-earthy-terracotta-700"
									></div>
								{:else}
									<IconifyIcon
										icon="material-symbols:add-circle-outline"
										class="w-5 h-5 text-earthy-terracotta-700 dark:text-earthy-terracotta-700"
									/>
								{/if}
							</button>
						{/each}
					</div>
				{:else}
					<div class="text-center py-6 text-sm text-gray-500 dark:text-gray-400">
						Start typing to search
					</div>
				{/if}
			</div>
		</div>
	</div>
{/if}
