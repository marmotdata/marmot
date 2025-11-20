<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import type { Asset } from '$lib/assets/types';
	import AssetCard from './AssetCard.svelte';
	import QueryInput from './QueryInput.svelte';
	import Icon from '@iconify/svelte';

	export let onSearch: ((query: string) => void) | undefined = undefined;
	export let placeholder = 'Search assets...';
	export let initialQuery = '';
	export let autofocus = false;
	export let onNavigate: (() => void) | undefined = undefined;
	export let showSuggestions = true;
	export let headerStyle = false;

	let searchQuery = initialQuery;
	let searchResults: Asset[] = [];
	let isLoading = false;
	let showResults = false;
	let selectedSearchIndex = -1;
	let debounceTimer: NodeJS.Timeout;
	let inputElement: HTMLDivElement;
	let previousInitialQuery = initialQuery;

	const isMac = browser && navigator.platform.toUpperCase().indexOf('MAC') >= 0;

	// Only sync when initialQuery prop actually changes (not when searchQuery changes)
	$: if (initialQuery !== previousInitialQuery) {
		searchQuery = initialQuery;
		previousInitialQuery = initialQuery;
	}

	onMount(() => {
		if (initialQuery) {
			fetchResults(initialQuery);
		}
	});

	async function fetchResults(query: string) {
		if (!showSuggestions) {
			searchResults = [];
			showResults = false;
			return;
		}

		if (!query.trim()) {
			searchResults = [];
			showResults = false;
			return;
		}

		try {
			isLoading = true;
			const response = await fetchApi(`/assets/search?q=${encodeURIComponent(query)}&limit=5`);
			const data = await response.json();
			searchResults = data.assets;
			showResults = true;
			selectedSearchIndex = -1;
		} catch (error) {
			console.error('Search error:', error);
			searchResults = [];
		} finally {
			isLoading = false;
		}
	}

	function isIncompleteMetadataQuery(query: string): boolean {
		if (!query.includes('@metadata.')) return false;

		const lastMetadataIndex = query.lastIndexOf('@metadata.');
		const restOfQuery = query.slice(lastMetadataIndex);

		if (restOfQuery.match(/@metadata\.[a-zA-Z0-9_.]+\s*[:<>=]+\s*[^:\s]+/)) {
			return false;
		}

		if (restOfQuery.startsWith('@metadata.')) {
			return (
				!restOfQuery.includes(':') ||
				/:\s*$/.test(restOfQuery) ||
				restOfQuery === '@metadata.' ||
				restOfQuery.split(':')[1]?.trim() === ''
			);
		}

		return false;
	}

	function handleKeydown(event: KeyboardEvent) {
		// Only handle if the event target is within our component
		if (!inputElement || !inputElement.contains(event.target as Node)) {
			return;
		}

		// Handle keyboard navigation when suggestions are shown
		if (showSuggestions && showResults && searchResults && searchResults.length > 0) {
			if (event.key === 'ArrowDown') {
				event.preventDefault();
				selectedSearchIndex = (selectedSearchIndex + 1) % searchResults.length;
				return;
			} else if (event.key === 'ArrowUp') {
				event.preventDefault();
				selectedSearchIndex =
					selectedSearchIndex <= 0 ? searchResults.length - 1 : selectedSearchIndex - 1;
				return;
			} else if (event.key === 'Enter') {
				event.preventDefault();
				// Navigate directly to selected asset
				if (selectedSearchIndex >= 0 && searchResults[selectedSearchIndex]) {
					handleAssetClick(searchResults[selectedSearchIndex]);
				} else {
					handleSubmit();
				}
				return;
			}
		} else if (event.key === 'Enter') {
			event.preventDefault();
			handleSubmit();
			return;
		}
	}

	function handleSubmit() {
		if (searchQuery.trim() && !isIncompleteMetadataQuery(searchQuery)) {
			showResults = false;
			if (onNavigate) {
				onNavigate();
			}
			goto(`/assets?q=${encodeURIComponent(searchQuery)}`);
		}
	}

	function handleQueryChange(newQuery: string) {
		searchQuery = newQuery;
		clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			fetchResults(searchQuery);
			if (onSearch) {
				onSearch(searchQuery);
			}
		}, 300);
	}

	function handleAssetClick(asset: Asset) {
		goto(`/assets/${asset.type.toLowerCase()}/${encodeURIComponent(asset.name)}`);
		showResults = false;
		if (onNavigate) {
			onNavigate();
		}
	}

	function handleClickOutside(event: MouseEvent) {
		if (inputElement && !inputElement.contains(event.target as Node)) {
			showResults = false;
		}
	}

	onMount(() => {
		if (initialQuery) {
			fetchResults(initialQuery);
		}
	});
</script>

<svelte:window on:click={handleClickOutside} on:keydown={handleKeydown} />

<div class="relative w-full" bind:this={inputElement}>
	{#if headerStyle}
		<div class="flex items-center gap-2 px-4 py-2 text-sm text-gray-600 dark:text-gray-400 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-lg hover:border-gray-400 dark:hover:border-gray-500 transition-colors header-search-wrapper">
			<Icon icon="material-symbols:search" class="w-4 h-4 flex-shrink-0" />
			<div class="flex-1 min-w-0">
				<QueryInput
					bind:value={searchQuery}
					{placeholder}
					{isLoading}
					{autofocus}
					plain={true}
					onQueryChange={handleQueryChange}
					onSubmit={handleSubmit}
				/>
			</div>
			<kbd class="px-2 py-0.5 text-xs font-semibold text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded flex-shrink-0">
				{isMac ? '⌘' : 'Ctrl'}K
			</kbd>
		</div>
	{:else}
		<QueryInput
			bind:value={searchQuery}
			{placeholder}
			{isLoading}
			{autofocus}
			onQueryChange={handleQueryChange}
			onSubmit={handleSubmit}
		/>
	{/if}

	{#if showResults && searchResults && searchResults.length > 0}
		<div
			class="absolute z-40 w-full mt-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg overflow-hidden divide-y divide-gray-200 dark:divide-gray-700 max-h-[400px] overflow-y-auto"
		>
			{#each searchResults as asset, i}
				<AssetCard
					{asset}
					compact={true}
					selected={i === selectedSearchIndex}
					onClick={() => handleAssetClick(asset)}
				/>
			{/each}
		</div>
	{/if}
</div>

<style>
	/* Focus styles for header search wrapper */
	:global(.header-search-wrapper:focus-within) {
		border-color: rgb(249 115 22 / 0.5);
		outline: 2px solid rgb(249 115 22 / 0.2);
		outline-offset: 0;
	}

	:global(.dark .header-search-wrapper:focus-within) {
		border-color: rgb(251 146 60 / 0.5);
		outline-color: rgb(251 146 60 / 0.2);
	}
</style>
