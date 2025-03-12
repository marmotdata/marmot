<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount } from 'svelte';
	import { writable, type Writable } from 'svelte/store';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import type { Asset, AvailableFilters } from '$lib/assets/types';
	import QueryInput from '../../components/QueryInput.svelte';
	import AssetBlade from '../../components/AssetBlade.svelte';
	import Icon from '../../components/Icon.svelte';

	interface SearchResponse {
		assets: Asset[];
		total: number;
		limit: number;
		offset: number;
		filters: AvailableFilters;
	}

	interface Filters {
		types: string[];
		providers: string[];
		tags: string[];
		updatedAfter?: string;
	}

	let placeholder = 'Search assets...';

	// State management
	const assets: Writable<Asset[]> = writable([]);
	const totalAssets: Writable<number> = writable(0);
	const isLoading: Writable<boolean> = writable(true);
	const error: Writable<{ status: number; message: string } | null> = writable(null);

	// Pagination
	let currentPage = 1;
	const itemsPerPage = 20;

	// Selected asset for blade
	let selectedAsset: Asset | null = null;

	// Search query from URL
	let searchQuery = $page.url.searchParams.get('q') || '';
	let searchTimeout: NodeJS.Timeout;

	// Filters state
	let selectedFilters: Filters = {
		types: [],
		providers: [],
		tags: []
	};

	// Available filters from API
	let availableFilters: AvailableFilters = {
		types: {},
		providers: {},
		tags: {}
	};

	let typesCounts: Record<string, number> = {};
	let providersCounts: Record<string, number> = {};
	let tagsCounts: Record<string, number> = {};
	let isLoadingFilters = true;

	// Initialize filters from URL
	$: {
		const searchParams = new URLSearchParams($page.url.search);
		selectedFilters = {
			types: searchParams.get('types')?.split(',').filter(Boolean) || [],
			providers: searchParams.get('providers')?.split(',').filter(Boolean) || [],
			tags: searchParams.get('tags')?.split(',').filter(Boolean) || [],
			updatedAfter: searchParams.get('updatedAfter') || undefined
		};
		currentPage = parseInt(searchParams.get('page') || '1', 10);
	}

	function getIconType(asset: Asset): string {
		if (asset.providers && Array.isArray(asset.providers) && asset.providers.length === 1) {
			return asset.providers[0];
		}
		return asset.type;
	}

	async function fetchAssets() {
		isLoading.set(true);
		error.set(null);

		try {
			const queryParams = new URLSearchParams({
				limit: itemsPerPage.toString(),
				offset: ((currentPage - 1) * itemsPerPage).toString(),
				calculateCounts: 'true'
			});

			if (searchQuery) {
				queryParams.append('q', searchQuery);
			}

			if (selectedFilters.types.length)
				queryParams.append('types', selectedFilters.types.join(','));
			if (selectedFilters.providers.length)
				queryParams.append('providers', selectedFilters.providers.join(','));
			if (selectedFilters.tags.length) queryParams.append('tags', selectedFilters.tags.join(','));

			const response = await fetchApi(`/assets/search?${queryParams}`);

			if (!response.ok) {
				const errorData = await response.json();
				throw {
					status: response.status,
					message: errorData.error || 'Unable to complete your request'
				};
			}

			const data: SearchResponse = await response.json();

			assets.set(data.assets);
			totalAssets.set(data.total);

			if (data.filters) {
				availableFilters = data.filters;
				typesCounts = data.filters.types || {};
				providersCounts = data.filters.providers || {};
				tagsCounts = data.filters.tags || {};
			}
			isLoadingFilters = false;
		} catch (e) {
			const errorStatus = e.status || 500;
			error.set({ status: errorStatus, message: e.message });
			console.error('Error fetching assets:', e);
		} finally {
			isLoading.set(false);
		}
	}

	function handleSearch(query: string) {
		if (searchTimeout) {
			clearTimeout(searchTimeout);
		}

		searchQuery = query;

		searchTimeout = setTimeout(() => {
			currentPage = 1;
			updateURL();
			fetchAssets();
		}, 300);
	}

	function handleSearchSubmit() {
		if (searchTimeout) {
			clearTimeout(searchTimeout);
		}

		currentPage = 1;
		updateURL();
		fetchAssets();
	}

	function updateURL() {
		const params = new URLSearchParams();

		if (searchQuery) params.append('q', searchQuery);
		if (selectedFilters.types.length) params.append('types', selectedFilters.types.join(','));
		if (selectedFilters.providers.length)
			params.append('providers', selectedFilters.providers.join(','));
		if (selectedFilters.tags.length) params.append('tags', selectedFilters.tags.join(','));
		if (selectedFilters.updatedAfter) params.append('updatedAfter', selectedFilters.updatedAfter);
		if (currentPage > 1) params.append('page', currentPage.toString());

		goto(`?${params.toString()}`, { replaceState: true, noScroll: true, keepFocus: true });
	}

	function handleFilterChange() {
		currentPage = 1;
		updateURL();
		fetchAssets();
	}

	function handlePageChange(newPage: number) {
		currentPage = newPage;
		updateURL();
		fetchAssets();
	}

	function handleTagClick(tag: string, event: MouseEvent) {
		event.preventDefault();
		event.stopPropagation();

		if (!selectedFilters.tags.includes(tag)) {
			selectedFilters.tags = [...selectedFilters.tags, tag];
			handleFilterChange();
		}
	}

	function handleTypeClick(type: string, event: MouseEvent) {
		event.preventDefault();
		event.stopPropagation();

		if (!selectedFilters.types.includes(type)) {
			selectedFilters.types = [...selectedFilters.types, type];
			handleFilterChange();
		}
	}

	function removeFilter(filterType: keyof Filters, value: string) {
		selectedFilters[filterType] = selectedFilters[filterType].filter((v) => v !== value);
		handleFilterChange();
	}

	function clearAllFilters() {
		selectedFilters = {
			types: [],
			providers: [],
			tags: []
		};
		searchQuery = '';
		handleFilterChange();
	}

	function handleAssetClick(asset: Asset) {
		selectedAsset = asset;
	}

	function getTagColor(type: string): string {
		const colors: Record<string, string> = {
			KAFKA_TOPIC: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-100',
			KAFKA_CONSUMER_GROUP: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-100',
			KAFKA_CLUSTER: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-100',
			default: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-100'
		};
		return colors[type] || colors.default;
	}

	$: hasActiveFilters =
		selectedFilters.types.length > 0 ||
		selectedFilters.providers.length > 0 ||
		selectedFilters.tags.length > 0;

	onMount(() => {
		fetchAssets();
	});
</script>

<div class="container mx-auto px-4 py-8">
	<div class="mb-8">
		<h1 class="text-3xl font-bold text-gray-900 dark:text-gray-100 mb-4">Assets</h1>
		<QueryInput
			value={searchQuery}
			onQueryChange={handleSearch}
			onSubmit={handleSearchSubmit}
			{placeholder}
		/>
	</div>
	{#if hasActiveFilters}
		<div class="mb-6 rounded-lg dark:shadow-white/20 p-4">
			<div class="flex items-center justify-between mb-2">
				<h2 class="font-medium text-gray-700 dark:text-gray-300">Active Filters</h2>
				<button
					on:click={clearAllFilters}
					class="text-sm text-orange-600 dark:text-orange-400 hover:text-orange-800 dark:hover:text-orange-300 font-medium"
				>
					Clear all filters
				</button>
			</div>
			<div class="flex flex-wrap gap-2">
				{#each selectedFilters.types as type}
					<span
						class="inline-flex items-center px-3 py-1 rounded-full text-sm bg-orange-100 dark:bg-orange-900 text-orange-800 dark:text-orange-100"
					>
						Type: {type}
						<button
							on:click={() => removeFilter('types', type)}
							class="ml-2 text-orange-500 dark:text-orange-300 hover:text-orange-700 dark:hover:text-orange-200"
							aria-label={`Remove ${type} filter`}
						>
							<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path
									stroke-linecap="round"
									stroke-linejoin="round"
									stroke-width="2"
									d="M6 18L18 6M6 6l12 12"
								/>
							</svg>
						</button>
					</span>
				{/each}

				{#each selectedFilters.providers as provider}
					<span
						class="inline-flex items-center px-3 py-1 rounded-full text-sm bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-100"
					>
						Service: {provider}
						<button
							on:click={() => removeFilter('providers', provider)}
							class="ml-2 text-green-500 dark:text-green-300 hover:text-green-700 dark:hover:text-green-200"
							aria-label={`Remove ${provider} filter`}
						>
							<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path
									stroke-linecap="round"
									stroke-linejoin="round"
									stroke-width="2"
									d="M6 18L18 6M6 6l12 12"
								/>
							</svg>
						</button>
					</span>
				{/each}

				{#each selectedFilters.tags as tag}
					<span
						class="inline-flex items-center px-3 py-1 rounded-full text-sm bg-purple-100 dark:bg-purple-900 text-purple-800 dark:text-purple-100"
					>
						Tag: {tag}
						<button
							on:click={() => removeFilter('tags', tag)}
							class="ml-2 text-purple-500 dark:text-purple-300 hover:text-purple-700 dark:hover:text-purple-200"
							aria-label={`Remove ${tag} filter`}
						>
							<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path
									stroke-linecap="round"
									stroke-linejoin="round"
									stroke-width="2"
									d="M6 18L18 6M6 6l12 12"
								/>
							</svg>
						</button>
					</span>
				{/each}
			</div>
		</div>
	{/if}

	<div class="flex gap-8">
		<div class="w-80 flex-shrink-0">
			<div class="rounded-lg p-4">
				<h2 class="font-semibold text-lg text-gray-900 dark:text-gray-100 mb-4">Filters</h2>

				{#if isLoadingFilters}
					<div class="animate-pulse space-y-4">
						{#each Array(3) as _}
							<div class="h-4 bg-gray-200 dark:bg-gray-600 rounded w-3/4" />
						{/each}
					</div>
				{:else}
					<div class="mb-6">
						<h3 class="font-medium text-gray-800 dark:text-gray-200 mb-2">Types</h3>
						{#each Object.keys(availableFilters.types) as type}
							<label class="flex items-center justify-between mb-2">
								<div class="flex items-center">
									<input
										type="checkbox"
										checked={selectedFilters.types.includes(type)}
										on:change={(e) => {
											if (e.target.checked) {
												selectedFilters.types = [...selectedFilters.types, type];
											} else {
												selectedFilters.types = selectedFilters.types.filter((t) => t !== type);
											}
											handleFilterChange();
										}}
										class="rounded border-gray-300 dark:border-gray-600 text-orange-600 focus:ring-orange-500 dark:bg-gray-800"
									/>
									<span class="ml-2 text-sm text-gray-700 dark:text-gray-300">{type}</span>
								</div>
								<span class="text-xs text-gray-500 dark:text-gray-400">({typesCounts[type]})</span>
							</label>
						{/each}
					</div>

					<div class="mb-6">
						<h3 class="font-medium text-gray-800 dark:text-gray-200 mb-2">Providers</h3>
						{#each Object.keys(availableFilters.providers) as provider}
							<label class="flex items-center justify-between mb-2">
								<div class="flex items-center">
									<input
										type="checkbox"
										checked={selectedFilters.providers.includes(provider)}
										on:change={(e) => {
											if (e.target.checked) {
												selectedFilters.providers = [...selectedFilters.providers, provider];
											} else {
												selectedFilters.providers = selectedFilters.providers.filter(
													(s) => s !== provider
												);
											}
											handleFilterChange();
										}}
										class="rounded border-gray-300 dark:border-gray-600 text-orange-600 focus:ring-orange-500 dark:bg-gray-800"
									/>
									<span class="ml-2 text-sm text-gray-700 dark:text-gray-300">{provider}</span>
								</div>
								<span class="text-xs text-gray-500 dark:text-gray-400"
									>({providersCounts[provider]})</span
								>
							</label>
						{/each}
					</div>

					<div class="mb-6">
						<h3 class="font-medium text-gray-800 dark:text-gray-200 mb-2">Tags</h3>
						{#each Object.keys(availableFilters.tags) as tag}
							<label class="flex items-center justify-between mb-2 gap-2">
								<div class="flex items-center min-w-0">
									<input
										type="checkbox"
										checked={selectedFilters.tags.includes(tag)}
										on:change={(e) => {
											if (e.target.checked) {
												selectedFilters.tags = [...selectedFilters.tags, tag];
											} else {
												selectedFilters.tags = selectedFilters.tags.filter((t) => t !== tag);
											}
											handleFilterChange();
										}}
										class="flex-shrink-0 rounded border-gray-300 dark:border-gray-600 text-orange-600 focus:ring-orange-500 dark:bg-gray-800"
									/>
									<span class="ml-2 text-sm text-gray-700 dark:text-gray-300 truncate" title={tag}>
										{tag.length > 100 ? tag.slice(0, 100) + '...' : tag}
									</span>
								</div>
								<span class="flex-shrink-0 text-xs text-gray-500 dark:text-gray-400"
									>({tagsCounts[tag]})</span
								>
							</label>
						{/each}
					</div>
				{/if}
			</div>
		</div>

		<div class="flex-1">
			{#if $error}
				{#if String($error.status).startsWith('5')}
					<div
						class="bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-700 text-red-700 dark:text-red-100 px-4 py-3 rounded-lg"
					>
						Something went wrong on our end. Please try again later.
					</div>
				{:else}
					<div
						class="bg-orange-50 dark:bg-orange-900 border border-orange-200 dark:border-orange-700 px-4 py-3 rounded-lg flex items-center gap-3"
					>
						<svg
							class="w-5 h-5 text-orange-500 dark:text-orange-300 flex-shrink-0"
							xmlns="http://www.w3.org/2000/svg"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
						>
							<path d="M12 8v4m0 4h.01M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
						</svg>
						<span class="text-orange-800 dark:text-orange-100">
							{#if $error.status === 400}
								Your search query appears to be incomplete or invalid. Please check your syntax and
								try again.
							{:else}
								{$error.message}
							{/if}
						</span>
					</div>
				{/if}
			{:else}
				<div class="flex justify-between items-center mb-4">
					<p class="text-gray-600 dark:text-gray-400">
						Showing {(currentPage - 1) * itemsPerPage + 1} to {Math.min(
							currentPage * itemsPerPage,
							$totalAssets
						)} of {$totalAssets} assets
					</p>

					<div class="flex gap-2">
						<button
							on:click={() => handlePageChange(currentPage - 1)}
							disabled={currentPage === 1}
							class="px-3 py-1 rounded border border-gray-300 dark:border-gray-600 disabled:opacity-50 text-gray-700 dark:text-gray-300"
						>
							Previous
						</button>
						<button
							on:click={() => handlePageChange(currentPage + 1)}
							disabled={currentPage * itemsPerPage >= $totalAssets}
							class="px-3 py-1 rounded border border-gray-300 dark:border-gray-600 disabled:opacity-50 text-gray-700 dark:text-gray-300"
						>
							Next
						</button>
					</div>
				</div>

				<div class="rounded-lg shadow dark:shadow-white/20 overflow-hidden">
					{#if $isLoading}
						{#each Array(itemsPerPage) as _}
							<div class="p-4 border-b border-gray-200 dark:border-gray-600 animate-pulse">
								<div class="flex justify-between items-start mb-2">
									<div class="space-y-2">
										<div class="h-5 bg-gray-200 dark:bg-gray-600 rounded w-48" />
										<div class="h-4 bg-gray-200 dark:bg-gray-600 rounded w-64" />
									</div>
									<div class="h-6 bg-gray-200 dark:bg-gray-600 rounded w-24" />
								</div>
								<div class="h-4 bg-gray-200 dark:bg-gray-600 rounded w-3/4 mb-2" />
								<div class="flex gap-2">
									{#each Array(3) as _}
										<div class="h-6 bg-gray-200 dark:bg-gray-600 rounded w-16" />
									{/each}
								</div>
							</div>
						{/each}
					{:else}
						{#each $assets as asset (asset.id)}
							<div
								class="p-4 border-b border-gray-200 dark:border-gray-600 hover:bg-gray-100 dark:hover:bg-gray-600 cursor-pointer"
								on:click={() => handleAssetClick(asset)}
							>
								<div class="flex justify-between items-start mb-2">
									<div class="flex items-center gap-3">
										<Icon name={getIconType(asset)} showLabel={false} iconSize="md" />
										<div>
											<h3 class="font-medium text-lg text-gray-900 dark:text-gray-100">
												{asset.name}
											</h3>
											<p class="text-sm text-gray-600 dark:text-gray-400">{asset.mrn}</p>
										</div>
									</div>
									<button
										on:click={(e) => handleTypeClick(asset.type, e)}
										class="text-sm {getTagColor(
											asset.type
										)} px-3 py-1 rounded-full hover:opacity-80 transition-opacity"
									>
										{asset.type.replace(/_/g, ' ')}
									</button>
								</div>

								{#if asset.description}
									<p class="text-gray-600 dark:text-gray-400 mb-2">{asset.description}</p>
								{/if}

								<div class="flex flex-wrap gap-2">
									{#each asset.tags as tag}
										<button
											on:click={(e) => handleTagClick(tag, e)}
											class="text-xs bg-gray-100 dark:bg-gray-600 text-gray-600 dark:text-gray-300 px-2 py-1 rounded-full hover:bg-gray-200 dark:hover:bg-gray-500"
										>
											{tag}
										</button>
									{/each}
								</div>

								<div class="mt-2 text-sm text-gray-500 dark:text-gray-400">
									Created by {asset.created_by} on {new Date(asset.created_at).toLocaleDateString()}
								</div>
							</div>
						{/each}
					{/if}
				</div>
			{/if}
		</div>
	</div>
</div>

{#if selectedAsset}
	<AssetBlade asset={selectedAsset} onClose={() => (selectedAsset = null)} />
{/if}
