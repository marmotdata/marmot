<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount } from 'svelte';
	import { writable, type Writable } from 'svelte/store';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import type { Asset, AvailableFilters } from '$lib/assets/types';
	import QueryInput from '../../components/QueryInput.svelte';
	import AssetBlade from '../../components/AssetBlade.svelte';
	import Icon from '../../components/Icon.svelte';
	import IconifyIcon from '@iconify/svelte';
	import TagsInput from '../../components/TagsInput.svelte';
	import Button from '../../components/Button.svelte';
	import { auth } from '$lib/stores/auth';
	import { providerIconMap, typeIconMap } from '$lib/iconloader';

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
	let searchQuery = '';
	let searchTimeout: NodeJS.Timeout;

	// Create asset modal state
	let showCreateModal = false;
	let isCreating = false;
	let createError = '';
	let newAssetName = '';
	let newAssetType = '';
	let newAssetProviders: string[] = [];
	let newAssetUserDescription = '';
	let newAssetTags: string[] = [];
	let typeSearch = '';
	let providerSearch = '';
	let showTypeDropdown = false;
	let showProviderDropdown = false;
	let tagInput = '';
	let selectedTypeIndex = -1;
	let selectedProviderIndex = -1;
	let typeDropdownElement: HTMLDivElement;
	let providerDropdownElement: HTMLDivElement;

	// Update search query and fetch when URL changes
	$: {
		const urlQuery = $page.url.searchParams.get('q') || '';
		if (urlQuery !== searchQuery) {
			searchQuery = urlQuery;
			// Only fetch if we're not on initial mount (fetchAssets will be called in onMount)
			if (browser) {
				fetchAssets();
			}
		}
	}

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

	// Permission check
	let canManageAssets = false;
	$: canManageAssets = auth.hasPermission('assets', 'manage');

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
			KAFKA_TOPIC:
				'bg-earthy-terracotta-100 text-earthy-terracotta-700 dark:bg-earthy-terracotta-900 dark:text-earthy-terracotta-100',
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

	// Helper function to get display name or capitalize
	function getDisplayName(key: string, map: typeof typeIconMap | typeof providerIconMap): string {
		const normalizedKey = key.toLowerCase().replace(/_/g, '-');
		if (map[normalizedKey]?.displayName) {
			return map[normalizedKey].displayName;
		}
		// For items not in the map, return as-is (from API)
		return key;
	}

	// Get unique suggestions with proper casing
	$: filteredTypes = Array.from(
		new Map([
			...Object.keys(typeIconMap)
				.filter((type) =>
					typeIconMap[type].displayName.toLowerCase().includes(typeSearch.toLowerCase())
				)
				.map((type) => [type.toLowerCase(), { key: type, display: typeIconMap[type].displayName }]),
			...Object.keys(availableFilters.types || {})
				.filter((type) => type.toLowerCase().includes(typeSearch.toLowerCase()))
				.map((type) => {
					const normalizedKey = type.toLowerCase().replace(/_/g, '-');
					return [
						normalizedKey,
						{ key: normalizedKey, display: getDisplayName(type, typeIconMap) }
					];
				})
		]).values()
	);

	$: filteredProviders = Array.from(
		new Map([
			...Object.keys(providerIconMap)
				.filter((provider) =>
					providerIconMap[provider].displayName.toLowerCase().includes(providerSearch.toLowerCase())
				)
				.map((provider) => [
					provider.toLowerCase(),
					{ key: provider, display: providerIconMap[provider].displayName }
				]),
			...Object.keys(availableFilters.providers || {})
				.filter((provider) => provider.toLowerCase().includes(providerSearch.toLowerCase()))
				.map((provider) => {
					const normalizedKey = provider.toLowerCase().replace(/_/g, '-');
					return [
						normalizedKey,
						{ key: normalizedKey, display: getDisplayName(provider, providerIconMap) }
					];
				})
		]).values()
	);

	// Reset selected index when filtered list changes
	$: if (filteredTypes) selectedTypeIndex = -1;
	$: if (filteredProviders) selectedProviderIndex = -1;

	function handleNewAsset() {
		newAssetName = '';
		newAssetType = '';
		newAssetProviders = [];
		newAssetUserDescription = '';
		newAssetTags = [];
		tagInput = '';
		typeSearch = '';
		providerSearch = '';
		createError = '';
		showTypeDropdown = false;
		showProviderDropdown = false;
		showCreateModal = true;
	}

	function selectType(typeObj: { key: string; display: string }) {
		newAssetType = typeObj.display;
		typeSearch = typeObj.display;
		showTypeDropdown = false;
	}

	function handleTypeKeydown(event: KeyboardEvent) {
		if (!showTypeDropdown && event.key !== 'Escape') {
			showTypeDropdown = true;
		}

		if (event.key === 'ArrowDown') {
			event.preventDefault();
			selectedTypeIndex = Math.min(selectedTypeIndex + 1, filteredTypes.length - 1);
			scrollToSelectedType();
		} else if (event.key === 'ArrowUp') {
			event.preventDefault();
			selectedTypeIndex = Math.max(selectedTypeIndex - 1, -1);
			scrollToSelectedType();
		} else if (event.key === 'Enter') {
			event.preventDefault();
			if (selectedTypeIndex >= 0 && filteredTypes[selectedTypeIndex]) {
				selectType(filteredTypes[selectedTypeIndex]);
			} else if (typeSearch.trim()) {
				newAssetType = typeSearch.trim();
				showTypeDropdown = false;
			}
		} else if (event.key === 'Escape') {
			event.preventDefault();
			showTypeDropdown = false;
			selectedTypeIndex = -1;
		}
	}

	function scrollToSelectedType() {
		if (typeDropdownElement && selectedTypeIndex >= 0) {
			const buttons = typeDropdownElement.querySelectorAll('button');
			if (buttons[selectedTypeIndex]) {
				buttons[selectedTypeIndex].scrollIntoView({ block: 'nearest', behavior: 'smooth' });
			}
		}
	}

	function handleProviderKeydown(event: KeyboardEvent) {
		if (!showProviderDropdown && event.key !== 'Escape') {
			showProviderDropdown = true;
		}

		if (event.key === 'ArrowDown') {
			event.preventDefault();
			selectedProviderIndex = Math.min(selectedProviderIndex + 1, filteredProviders.length - 1);
			scrollToSelectedProvider();
		} else if (event.key === 'ArrowUp') {
			event.preventDefault();
			selectedProviderIndex = Math.max(selectedProviderIndex - 1, -1);
			scrollToSelectedProvider();
		} else if (event.key === 'Enter') {
			event.preventDefault();
			if (selectedProviderIndex >= 0 && filteredProviders[selectedProviderIndex]) {
				toggleProvider(filteredProviders[selectedProviderIndex]);
				selectedProviderIndex = -1;
			} else if (providerSearch.trim()) {
				if (!newAssetProviders.includes(providerSearch.trim())) {
					newAssetProviders = [...newAssetProviders, providerSearch.trim()];
				}
				providerSearch = '';
			}
		} else if (event.key === 'Escape') {
			event.preventDefault();
			showProviderDropdown = false;
			selectedProviderIndex = -1;
		}
	}

	function scrollToSelectedProvider() {
		if (providerDropdownElement && selectedProviderIndex >= 0) {
			const buttons = providerDropdownElement.querySelectorAll('button');
			if (buttons[selectedProviderIndex]) {
				buttons[selectedProviderIndex].scrollIntoView({ block: 'nearest', behavior: 'smooth' });
			}
		}
	}

	function toggleProvider(providerObj: { key: string; display: string }) {
		const displayName = providerObj.display;
		if (newAssetProviders.includes(displayName)) {
			newAssetProviders = newAssetProviders.filter((p) => p !== displayName);
		} else {
			newAssetProviders = [...newAssetProviders, displayName];
		}
		providerSearch = '';
	}

	function removeProvider(provider: string) {
		newAssetProviders = newAssetProviders.filter((p) => p !== provider);
	}

	async function createAsset() {
		if (!newAssetName || !newAssetType || newAssetProviders.length === 0) {
			createError = 'Name, type, and at least one provider are required';
			return;
		}

		isCreating = true;
		createError = '';

		try {
			const payload: any = {
				name: newAssetName,
				type: newAssetType,
				providers: newAssetProviders
			};

			if (newAssetUserDescription) {
				payload.user_description = newAssetUserDescription;
			}
			if (newAssetTags.length > 0) {
				payload.tags = newAssetTags;
			}

			const response = await fetchApi('/assets/', {
				method: 'POST',
				body: JSON.stringify(payload)
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to create asset');
			}

			showCreateModal = false;
			fetchAssets();
		} catch (err) {
			createError = err instanceof Error ? err.message : 'Failed to create asset';
		} finally {
			isCreating = false;
		}
	}

	onMount(() => {
		fetchAssets();
	});
</script>

<div class="h-full flex flex-col">
	<!-- Main Content -->
	<div class="flex-1 overflow-hidden">
		<div class="max-w-[1600px] mx-auto px-6 py-6 h-full flex gap-6">
			<!-- Filters Sidebar -->
			<div class="w-64 flex-shrink-0">
				<div
					class="sticky top-6 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 shadow-sm"
				>
					<h2
						class="font-semibold text-sm text-gray-900 dark:text-gray-100 mb-4 uppercase tracking-wider"
					>
						Filters
					</h2>

					{#if isLoadingFilters}
						<div class="animate-pulse space-y-4">
							{#each Array(3) as _}
								<div class="h-4 bg-gray-200 dark:bg-gray-600 rounded w-3/4" />
							{/each}
						</div>
					{:else}
						<div class="mb-5">
							<h3
								class="text-xs font-semibold text-gray-600 dark:text-gray-400 mb-3 uppercase tracking-wider"
							>
								Types
							</h3>
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
											class="rounded border-gray-300 dark:border-gray-600 text-earthy-terracotta-700 focus:ring-earthy-terracotta-600 dark:bg-gray-800"
										/>
										<span class="ml-2 text-sm text-gray-700 dark:text-gray-300">{type}</span>
									</div>
									<span class="text-xs text-gray-500 dark:text-gray-400">({typesCounts[type]})</span
									>
								</label>
							{/each}
						</div>

						<div class="mb-5">
							<h3
								class="text-xs font-semibold text-gray-600 dark:text-gray-400 mb-3 uppercase tracking-wider"
							>
								Providers
							</h3>
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
											class="rounded border-gray-300 dark:border-gray-600 text-earthy-terracotta-700 focus:ring-earthy-terracotta-600 dark:bg-gray-800"
										/>
										<span class="ml-2 text-sm text-gray-700 dark:text-gray-300">{provider}</span>
									</div>
									<span class="text-xs text-gray-500 dark:text-gray-400"
										>({providersCounts[provider]})</span
									>
								</label>
							{/each}
						</div>

						<div>
							<h3
								class="text-xs font-semibold text-gray-600 dark:text-gray-400 mb-3 uppercase tracking-wider"
							>
								Tags
							</h3>
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
											class="flex-shrink-0 rounded border-gray-300 dark:border-gray-600 text-earthy-terracotta-700 focus:ring-earthy-terracotta-600 dark:bg-gray-800"
										/>
										<span
											class="ml-2 text-sm text-gray-700 dark:text-gray-300 truncate"
											title={tag}
										>
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

			<!-- Assets Grid -->
			<div class="flex-1 flex flex-col min-h-0">
				<!-- Header with Filters -->
				<div class="mb-6">
					<div class="flex items-center justify-between mb-4">
						<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Assets</h1>
						<div class="flex items-center gap-4">
							{#if canManageAssets}
								<Button
									click={handleNewAsset}
									icon="material-symbols:add"
									text="New Asset"
									variant="filled"
								/>
							{/if}
							<div class="text-sm text-gray-500 dark:text-gray-400">
								{$totalAssets}
								{$totalAssets === 1 ? 'asset' : 'assets'} total
							</div>
						</div>
					</div>
					{#if hasActiveFilters}
						<div
							class="bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 border border-earthy-terracotta-200 dark:border-earthy-terracotta-800 rounded-lg p-3"
						>
							<div class="flex items-center justify-between mb-2">
								<h2
									class="text-xs font-semibold text-earthy-terracotta-700 dark:text-earthy-terracotta-200 uppercase tracking-wider"
								>
									Active Filters
								</h2>
								<button
									on:click={clearAllFilters}
									class="text-xs text-earthy-terracotta-700 dark:text-earthy-terracotta-400 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-100 font-medium"
								>
									Clear all
								</button>
							</div>
							<div class="flex flex-wrap gap-2">
								{#each selectedFilters.types as type}
									<span
										class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium bg-white dark:bg-earthy-terracotta-900/40 text-earthy-terracotta-700 dark:text-earthy-terracotta-100 border border-earthy-terracotta-300 dark:border-earthy-terracotta-800"
									>
										<span class="text-earthy-terracotta-700 dark:text-earthy-terracotta-700"
											>Type:</span
										>
										{type}
										<button
											on:click={() => removeFilter('types', type)}
											class="ml-0.5 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-200"
											aria-label={`Remove ${type} filter`}
										>
											<svg
												class="w-3.5 h-3.5"
												fill="none"
												stroke="currentColor"
												viewBox="0 0 24 24"
											>
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
										class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium bg-white dark:bg-earthy-terracotta-900/40 text-earthy-terracotta-700 dark:text-earthy-terracotta-100 border border-earthy-terracotta-300 dark:border-earthy-terracotta-800"
									>
										<span class="text-earthy-terracotta-700 dark:text-earthy-terracotta-700"
											>Provider:</span
										>
										{provider}
										<button
											on:click={() => removeFilter('providers', provider)}
											class="ml-0.5 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-200"
											aria-label={`Remove ${provider} filter`}
										>
											<svg
												class="w-3.5 h-3.5"
												fill="none"
												stroke="currentColor"
												viewBox="0 0 24 24"
											>
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
										class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium bg-white dark:bg-earthy-terracotta-900/40 text-earthy-terracotta-700 dark:text-earthy-terracotta-100 border border-earthy-terracotta-300 dark:border-earthy-terracotta-800"
									>
										<span class="text-earthy-terracotta-700 dark:text-earthy-terracotta-700"
											>Tag:</span
										>
										{tag}
										<button
											on:click={() => removeFilter('tags', tag)}
											class="ml-0.5 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-200"
											aria-label={`Remove ${tag} filter`}
										>
											<svg
												class="w-3.5 h-3.5"
												fill="none"
												stroke="currentColor"
												viewBox="0 0 24 24"
											>
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
				</div>

				{#if $error}
					{#if String($error.status).startsWith('5')}
						<div
							class="bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-700 text-red-700 dark:text-red-100 px-4 py-3 rounded-lg"
						>
							Something went wrong on our end. Please try again later.
						</div>
					{:else}
						<div
							class="bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900 border border-earthy-terracotta-200 dark:border-earthy-terracotta-800 px-4 py-3 rounded-lg flex items-center gap-3"
						>
							<svg
								class="w-5 h-5 text-earthy-terracotta-700 dark:text-earthy-terracotta-400 flex-shrink-0"
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
							<span class="text-earthy-terracotta-700 dark:text-earthy-terracotta-100">
								{#if $error.status === 400}
									Your search query appears to be incomplete or invalid. Please check your syntax
									and try again.
								{:else}
									{$error.message}
								{/if}
							</span>
						</div>
					{/if}
				{:else}
					<!-- Pagination Header -->
					<div class="flex justify-between items-center mb-4">
						<p class="text-sm text-gray-600 dark:text-gray-400">
							Showing {(currentPage - 1) * itemsPerPage + 1}-{Math.min(
								currentPage * itemsPerPage,
								$totalAssets
							)} of {$totalAssets}
						</p>
						<div class="flex gap-2">
							<button
								on:click={() => handlePageChange(currentPage - 1)}
								disabled={currentPage === 1}
								class="px-4 py-2 text-sm rounded-lg border border-gray-300 dark:border-gray-600 disabled:opacity-50 disabled:cursor-not-allowed text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
							>
								Previous
							</button>
							<button
								on:click={() => handlePageChange(currentPage + 1)}
								disabled={currentPage * itemsPerPage >= $totalAssets}
								class="px-4 py-2 text-sm rounded-lg border border-gray-300 dark:border-gray-600 disabled:opacity-50 disabled:cursor-not-allowed text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
							>
								Next
							</button>
						</div>
					</div>

					<!-- Assets List -->
					<div class="flex-1 overflow-y-auto space-y-3">
						{#if $isLoading}
							{#each Array(itemsPerPage) as _}
								<div
									class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5 animate-pulse shadow-sm"
								>
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
									class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5 hover:shadow-md hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 transition-all cursor-pointer group"
									on:click={() => handleAssetClick(asset)}
								>
									<div class="flex justify-between items-start mb-3">
										<div class="flex items-center gap-3 flex-1 min-w-0">
											<div class="flex-shrink-0">
												<Icon name={getIconType(asset)} showLabel={false} size="md" />
											</div>
											<div class="min-w-0 flex-1">
												<h3
													class="font-semibold text-base text-gray-900 dark:text-gray-100 truncate group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-700 transition-colors"
												>
													{asset.name}
												</h3>
												<p class="text-xs text-gray-500 dark:text-gray-400 truncate font-mono">
													{asset.mrn}
												</p>
											</div>
										</div>
										<button
											on:click={(e) => handleTypeClick(asset.type, e)}
											class="flex-shrink-0 text-xs {getTagColor(
												asset.type
											)} px-2.5 py-1 rounded-md hover:opacity-80 transition-opacity font-medium"
										>
											{asset.type.replace(/_/g, ' ')}
										</button>
									</div>

									{#if asset.description}
										<p class="text-sm text-gray-600 dark:text-gray-400 mb-3 line-clamp-2">
											{asset.description}
										</p>
									{/if}

									{#if asset.tags && asset.tags.length > 0}
										<div class="flex flex-wrap gap-1.5 mb-3">
											{#each asset.tags.slice(0, 5) as tag}
												<button
													on:click={(e) => handleTagClick(tag, e)}
													class="inline-flex items-center gap-1 text-xs bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 px-2 py-1 rounded hover:bg-earthy-terracotta-100 dark:hover:bg-earthy-terracotta-900/30 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-700 transition-colors"
												>
													<IconifyIcon icon="material-symbols:label-outline" class="w-3.5 h-3.5" />
													{tag}
												</button>
											{/each}
											{#if asset.tags.length > 5}
												<span class="text-xs text-gray-500 dark:text-gray-400 px-2 py-1">
													+{asset.tags.length - 5} more
												</span>
											{/if}
										</div>
									{/if}

									<div
										class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400 pt-2 border-t border-gray-100 dark:border-gray-700"
									>
										<span>{asset.created_by}</span>
										<span>•</span>
										<span>{new Date(asset.created_at).toLocaleDateString()}</span>
									</div>
								</div>
							{/each}
						{/if}
					</div>
				{/if}
			</div>
		</div>
	</div>
</div>

<!-- Create Asset Modal -->
{#if showCreateModal}
	<div class="fixed inset-0 z-50 flex items-start justify-center pt-[10vh] px-4 overflow-y-auto">
		<div
			class="fixed inset-0 bg-black/50 dark:bg-black/70 backdrop-blur-sm transition-opacity"
			on:click={() => !isCreating && (showCreateModal = false)}
			on:keypress={(e) => e.key === 'Enter' && !isCreating && (showCreateModal = false)}
			role="button"
			tabindex="0"
		></div>

		<div
			class="relative bg-white dark:bg-gray-800 rounded-xl shadow-2xl max-w-3xl w-full z-10 border border-gray-200 dark:border-gray-700 mb-10"
		>
			<!-- Header -->
			<div
				class="flex items-center justify-between px-6 py-5 border-b border-gray-200 dark:border-gray-700"
			>
				<div>
					<h3 class="text-xl font-semibold text-gray-900 dark:text-gray-100">Create New Asset</h3>
					<p class="text-sm text-gray-500 dark:text-gray-400 mt-1">
						Add a new asset to your data catalog
					</p>
				</div>
				<button
					on:click={() => (showCreateModal = false)}
					disabled={isCreating}
					class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors disabled:opacity-50"
				>
					<IconifyIcon icon="material-symbols:close" class="w-6 h-6" />
				</button>
			</div>

			{#if createError}
				<div
					class="mx-6 mt-5 rounded-lg bg-red-50 dark:bg-red-900/20 p-4 border border-red-200 dark:border-red-800"
				>
					<div class="flex items-start gap-3">
						<IconifyIcon
							icon="material-symbols:error"
							class="w-6 h-6 text-red-600 dark:text-red-400"
						/>
						<p class="text-sm text-red-800 dark:text-red-200">{createError}</p>
					</div>
				</div>
			{/if}

			<form on:submit|preventDefault={createAsset} class="p-6">
				<div class="space-y-6">
					<!-- Basic Information Section -->
					<div class="space-y-4">
						<!-- Name -->
						<div>
							<label
								for="asset-name"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								Name <span class="text-red-500">*</span>
							</label>
							<input
								id="asset-name"
								type="text"
								bind:value={newAssetName}
								disabled={isCreating}
								placeholder="e.g., user_events"
								class="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:bg-gray-700 dark:text-gray-100 disabled:opacity-50 transition-all"
								required
							/>
						</div>

						<!-- Type - Searchable with Free Text -->
						<div>
							<label
								for="asset-type"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								Type <span class="text-red-500">*</span>
							</label>
							<div class="relative">
								<input
									type="text"
									bind:value={typeSearch}
									on:input={() => {
										newAssetType = typeSearch;
										showTypeDropdown = true;
									}}
									on:focus={() => (showTypeDropdown = true)}
									on:blur={() => setTimeout(() => (showTypeDropdown = false), 200)}
									on:keydown={handleTypeKeydown}
									disabled={isCreating}
									placeholder="e.g., Table, Queue, Topic, Database..."
									class="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:bg-gray-700 dark:text-gray-100 disabled:opacity-50 transition-all font-mono"
									required
								/>
								<!-- Dropdown -->
								{#if showTypeDropdown && filteredTypes.length > 0 && typeSearch}
									<div
										bind:this={typeDropdownElement}
										class="absolute z-10 w-full mt-1 bg-white dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg shadow-lg max-h-60 overflow-y-auto"
									>
										{#each filteredTypes as typeObj, index}
											<button
												type="button"
												on:click={() => selectType(typeObj)}
												class="w-full px-4 py-3 flex items-center gap-3 hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors text-left {index ===
												selectedTypeIndex
													? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/30'
													: ''}"
											>
												<Icon name={typeObj.key} showLabel={false} size="md" />
												<span class="text-gray-900 dark:text-gray-100">{typeObj.display}</span>
											</button>
										{/each}
									</div>
								{/if}
							</div>
							<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
								The type of asset. Table, Queue, Topic, Database, View, DAG, etc.
							</p>
						</div>

						<!-- Providers - Multi-select with Free Text -->
						<div>
							<label
								for="asset-providers"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								Providers <span class="text-red-500">*</span>
							</label>

							<!-- Selected Providers -->
							{#if newAssetProviders.length > 0}
								<div class="flex flex-wrap gap-2 mb-2">
									{#each newAssetProviders as provider}
										<span
											class="inline-flex items-center gap-2 px-3 py-1.5 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/30 text-earthy-terracotta-700 dark:text-earthy-terracotta-100 rounded-lg border border-earthy-terracotta-200 dark:border-earthy-terracotta-800"
										>
											<span class="text-sm font-medium">{provider}</span>
											<button
												type="button"
												on:click={() => removeProvider(provider)}
												disabled={isCreating}
												class="text-earthy-terracotta-700 dark:text-earthy-terracotta-700 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-200 transition-colors disabled:opacity-50"
											>
												<IconifyIcon icon="material-symbols:close" class="w-4 h-4" />
											</button>
										</span>
									{/each}
								</div>
							{/if}

							<!-- Search Input -->
							<div class="relative">
								<input
									type="text"
									bind:value={providerSearch}
									on:focus={() => (showProviderDropdown = true)}
									on:blur={() => setTimeout(() => (showProviderDropdown = false), 200)}
									on:keydown={handleProviderKeydown}
									disabled={isCreating}
									placeholder="e.g., Kafka, Snowflake, PostgreSQL, Airflow..."
									class="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:bg-gray-700 dark:text-gray-100 disabled:opacity-50 transition-all font-mono"
								/>
								<!-- Dropdown -->
								{#if showProviderDropdown && filteredProviders.length > 0 && providerSearch}
									<div
										bind:this={providerDropdownElement}
										class="absolute z-10 w-full mt-1 bg-white dark:bg-gray-700 border border-gray-200 dark:border-gray-600 rounded-lg shadow-lg max-h-60 overflow-y-auto"
									>
										{#each filteredProviders as providerObj, index}
											<button
												type="button"
												on:click={() => {
													toggleProvider(providerObj);
												}}
												class="w-full px-4 py-3 flex items-center gap-3 hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors text-left {index ===
												selectedProviderIndex
													? 'bg-blue-50 dark:bg-blue-900/30'
													: newAssetProviders.includes(providerObj.display)
														? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20'
														: ''}"
											>
												<Icon name={providerObj.key} showLabel={false} size="md" />
												<span class="text-gray-900 dark:text-gray-100 flex-1"
													>{providerObj.display}</span
												>
												{#if newAssetProviders.includes(providerObj.display)}
													<IconifyIcon
														icon="material-symbols:check"
														class="w-5 h-5 text-earthy-terracotta-700 dark:text-earthy-terracotta-700"
													/>
												{/if}
											</button>
										{/each}
									</div>
								{/if}
							</div>
							<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
								The asset Provider. Kafka, Snowflake, PostgreSQL, Airflow, dbt, S3, etc.
							</p>
						</div>
					</div>

					<!-- Description Section -->
					<div class="space-y-4 pt-4 border-t border-gray-200 dark:border-gray-700">
						<h4
							class="text-sm font-semibold text-gray-900 dark:text-gray-100 uppercase tracking-wider"
						>
							Description
						</h4>

						<!-- User Description -->
						<div>
							<label
								for="asset-user-description"
								class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
							>
								Description
							</label>
							<textarea
								id="asset-user-description"
								bind:value={newAssetUserDescription}
								disabled={isCreating}
								placeholder="Add your own description to provide context..."
								rows="3"
								class="w-full px-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg shadow-sm focus:ring-2 focus:ring-earthy-terracotta-600 focus:border-earthy-terracotta-700 dark:bg-gray-700 dark:text-gray-100 disabled:opacity-50 transition-all resize-none"
							></textarea>
							<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
								Your custom description for this asset
							</p>
						</div>
					</div>

					<!-- Tags Section -->
					<div class="space-y-4 pt-4 border-t border-gray-200 dark:border-gray-700">
						<h4
							class="text-sm font-semibold text-gray-900 dark:text-gray-100 uppercase tracking-wider"
						>
							Tags
						</h4>

						<div>
							<TagsInput
								bind:tags={newAssetTags}
								disabled={isCreating}
								placeholder="Type a tag and press Enter..."
							/>
							<p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
								Press Enter to add tags for better organization
							</p>
						</div>
					</div>
				</div>

				<!-- Footer Actions -->
				<div class="flex justify-end gap-3 pt-6 mt-6 border-t border-gray-200 dark:border-gray-700">
					<Button
						type="button"
						click={() => (showCreateModal = false)}
						disabled={isCreating}
						text="Cancel"
						variant="clear"
					/>
					<Button
						type="submit"
						disabled={isCreating ||
							!newAssetName ||
							!newAssetType ||
							newAssetProviders.length === 0}
						loading={isCreating}
						icon="material-symbols:add"
						text={isCreating ? 'Creating...' : 'Create Asset'}
						variant="filled"
					/>
				</div>
			</form>
		</div>
	</div>
{/if}

{#if selectedAsset}
	<AssetBlade asset={selectedAsset} onClose={() => (selectedAsset = null)} />
{/if}
