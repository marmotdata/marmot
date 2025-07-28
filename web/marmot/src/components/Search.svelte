<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import type { Asset } from '$lib/assets/types';
	import AssetCard from './AssetCard.svelte';
	import QueryInput from './QueryInput.svelte';

	export let onSearch: ((query: string) => void) | undefined = undefined;
	export let placeholder = 'Search assets...';
	export let initialQuery = '';

	let searchQuery = initialQuery;
	let searchResults: Asset[] = [];
	let isLoading = false;
	let showResults = false;
	let selectedSearchIndex = -1;
	let debounceTimer: NodeJS.Timeout;
	let inputElement: HTMLDivElement;
	let queryInput: QueryInput;

	async function fetchResults(query: string) {
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
		if (queryInput && 'showDropdown' in queryInput && queryInput.showDropdown) {
			return;
		}

		if (event.key === 'Enter') {
			event.preventDefault();
			handleSubmit();
			return;
		}

		if (showResults && searchResults && searchResults.length > 0) {
			if (event.key === 'ArrowDown') {
				event.preventDefault();
				selectedSearchIndex = (selectedSearchIndex + 1) % searchResults.length;
			} else if (event.key === 'ArrowUp') {
				event.preventDefault();
				selectedSearchIndex =
					selectedSearchIndex <= 0 ? searchResults.length - 1 : selectedSearchIndex - 1;
			} else if (event.key === 'Enter' && selectedSearchIndex >= 0) {
				event.preventDefault();
				handleAssetClick(searchResults[selectedSearchIndex]);
			}
		}
	}

	function handleSubmit() {
		if (searchQuery.trim() && !isIncompleteMetadataQuery(searchQuery)) {
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
	<QueryInput
		bind:value={searchQuery}
		bind:this={queryInput}
		{placeholder}
		{isLoading}
		onQueryChange={handleQueryChange}
		onSubmit={handleSubmit}
	/>

	{#if showResults && searchResults && searchResults.length > 0}
		<div
			class="absolute z-40 w-full mt-2 bg-white dark:bg-gray-800 dark:bg-gray-800 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 dark:border-gray-700 rounded-lg shadow-lg dark:shadow-lg-white overflow-hidden divide-y divide-gray-200 dark:divide-gray-700 dark:divide-gray-700 dark:divide-gray-700"
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
