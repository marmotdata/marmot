<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { writable, type Writable } from 'svelte/store';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import type { Asset } from '$lib/assets/types';
	import type { DataProduct } from '$lib/dataproducts/types';
	import AssetBlade from '$components/asset/AssetBlade.svelte';
	import ProductBlade from '$components/product/ProductBlade.svelte';
	import Icon from '$components/ui/Icon.svelte';
	import IconifyIcon from '@iconify/svelte';
	import Button from '$components/ui/Button.svelte';
	import QueryBuilder from '$components/query/QueryBuilder.svelte';
	import { auth } from '$lib/stores/auth';
	import AuthenticatedImage from '$components/ui/AuthenticatedImage.svelte';

	interface SearchResult {
		type: 'asset' | 'glossary' | 'team' | 'data_product';
		id: string;
		name: string;
		description?: string;
		metadata?: Record<string, any>;
		url: string;
		rank: number;
		updated_at?: string;
	}

	interface FacetValue {
		value: string;
		count: number;
	}

	interface Facets {
		types: Record<string, number>;
		asset_types: FacetValue[];
		providers: FacetValue[];
		tags: FacetValue[];
	}

	interface SearchResponse {
		results: SearchResult[];
		total: number;
		facets: Facets;
		limit: number;
		offset: number;
	}

	const results: Writable<SearchResult[]> = writable([]);
	const totalResults: Writable<number> = writable(0);
	const facets: Writable<Facets> = writable({
		types: {},
		asset_types: [],
		providers: [],
		tags: []
	});
	const isLoading: Writable<boolean> = writable(true);
	const error: Writable<{ status: number; message: string } | null> = writable(null);

	let currentPage = $state(1);
	const itemsPerPage = 20;
	let selectedAsset = $state<Asset | null>(null);
	let selectedProduct = $state<DataProduct | null>(null);
	let searchQuery = $state('');
	let searchTimeout: ReturnType<typeof setTimeout>;
	let selectedKinds = $state<string[]>(['asset', 'glossary', 'team', 'data_product']);
	let selectedTypes = $state<string[]>([]);
	let selectedProviders = $state<string[]>([]);
	let selectedTags = $state<string[]>([]);
	let canManageAssets = $derived(auth.hasPermission('assets', 'manage'));
	let filtersExpanded = $state(true);
	let queryBuilderExpanded = $state(false);
	let previousUrl = $state<string | null>(null);
	let skipNextUrlEffect = false;

	// Initialize filters from URL
	$effect(() => {
		const currentUrl = $page.url.search;

		// Only process if URL actually changed (but allow first load when previousUrl is null)
		if (previousUrl !== null && previousUrl === currentUrl) {
			return;
		}

		previousUrl = currentUrl;

		if (skipNextUrlEffect) {
			skipNextUrlEffect = false;
			return;
		}

		const searchParams = new URLSearchParams(currentUrl);
		searchQuery = searchParams.get('q') || '';
		selectedKinds = searchParams.get('kind')?.split(',').filter(Boolean) || [
			'asset',
			'glossary',
			'team',
			'data_product'
		];
		selectedTypes = searchParams.get('types')?.split(',').filter(Boolean) || [];
		selectedProviders = searchParams.get('providers')?.split(',').filter(Boolean) || [];
		selectedTags = searchParams.get('tags')?.split(',').filter(Boolean) || [];
		currentPage = parseInt(searchParams.get('page') || '1', 10);

		if (browser) {
			fetchResults();
		}
	});

	function getIconType(asset: any): string {
		if (asset.providers && Array.isArray(asset.providers) && asset.providers.length === 1) {
			return asset.providers[0];
		}
		return asset.type;
	}

	async function fetchResults() {
		$isLoading = true;
		$error = null;

		try {
			const queryParams = new URLSearchParams({
				limit: itemsPerPage.toString(),
				offset: ((currentPage - 1) * itemsPerPage).toString()
			});

			if (searchQuery) {
				queryParams.append('q', searchQuery);
			}

			// Add kind filters
			selectedKinds.forEach((kind) => {
				queryParams.append('types[]', kind);
			});

			// Add asset-specific filters (only if asset kind is selected)
			if (selectedKinds.includes('asset')) {
				if (selectedTypes.length) queryParams.append('asset_types', selectedTypes.join(','));
				if (selectedProviders.length) queryParams.append('providers', selectedProviders.join(','));
				if (selectedTags.length) queryParams.append('tags', selectedTags.join(','));
			}

			const response = await fetchApi(`/search?${queryParams}`);

			if (!response.ok) {
				const errorData = await response.json();
				throw {
					status: response.status,
					message: errorData.error || 'Unable to complete your request'
				};
			}

			const data: SearchResponse = await response.json();

			$results = data.results || [];
			$totalResults = data.total || 0;
			$facets = {
				types: data.facets?.types || {},
				asset_types: data.facets?.asset_types || [],
				providers: data.facets?.providers || [],
				tags: data.facets?.tags || []
			};
		} catch (e: any) {
			const errorStatus = e.status || 500;
			$error = { status: errorStatus, message: e.message };
			console.error('Error fetching results:', e);
		} finally {
			$isLoading = false;
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
			fetchResults();
		}, 300);
	}

	function handleSearchSubmit() {
		if (searchTimeout) {
			clearTimeout(searchTimeout);
		}

		currentPage = 1;
		updateURL();
		fetchResults();
	}

	function handleRunQuery(query: string) {
		if (searchTimeout) {
			clearTimeout(searchTimeout);
		}

		searchQuery = query;
		currentPage = 1;
		updateURL();
		fetchResults();
	}

	function updateURL() {
		const params = new URLSearchParams();

		if (searchQuery) params.append('q', searchQuery);
		if (selectedKinds.length) params.append('kind', selectedKinds.join(','));
		if (selectedTypes.length) params.append('types', selectedTypes.join(','));
		if (selectedProviders.length) params.append('providers', selectedProviders.join(','));
		if (selectedTags.length) params.append('tags', selectedTags.join(','));
		if (currentPage > 1) params.append('page', currentPage.toString());

		skipNextUrlEffect = true;
		goto(`?${params.toString()}`, { replaceState: true, noScroll: true, keepFocus: true });
	}

	function handleFilterChange() {
		currentPage = 1;
		updateURL();
		fetchResults();
	}

	function handlePageChange(newPage: number) {
		currentPage = newPage;
		updateURL();
		fetchResults();
	}

	function handleTagClick(tag: string, event: MouseEvent) {
		event.preventDefault();
		event.stopPropagation();

		if (!selectedTags.includes(tag)) {
			selectedTags = [...selectedTags, tag];
			handleFilterChange();
		}
	}

	function handleTypeClick(type: string, event: MouseEvent) {
		event.preventDefault();
		event.stopPropagation();

		if (!selectedTypes.includes(type)) {
			selectedTypes = [...selectedTypes, type];
			handleFilterChange();
		}
	}

	function removeFilter(filterType: 'kind' | 'types' | 'providers' | 'tags', value: string) {
		if (filterType === 'kind') {
			selectedKinds = selectedKinds.filter((v) => v !== value);
		} else if (filterType === 'types') {
			selectedTypes = selectedTypes.filter((v) => v !== value);
		} else if (filterType === 'providers') {
			selectedProviders = selectedProviders.filter((v) => v !== value);
		} else if (filterType === 'tags') {
			selectedTags = selectedTags.filter((v) => v !== value);
		}
		handleFilterChange();
	}

	function clearAllFilters() {
		selectedKinds = ['asset', 'glossary', 'team', 'data_product'];
		selectedTypes = [];
		selectedProviders = [];
		selectedTags = [];
		searchQuery = '';
		handleFilterChange();
	}

	async function handleAssetClick(assetId: string) {
		try {
			// Fetch the full asset data from the assets API
			const response = await fetchApi(`/assets/${assetId}`);
			if (response.ok) {
				const asset: Asset = await response.json();
				selectedAsset = asset;
			} else {
				console.error('Failed to load asset');
			}
		} catch (err) {
			console.error('Error loading asset:', err);
		}
	}

	async function handleDataProductClick(productId: string) {
		try {
			const response = await fetchApi(`/products/${productId}`);
			if (response.ok) {
				const product: DataProduct = await response.json();
				// Ensure arrays are initialized
				product.tags = product.tags || [];
				product.owners = product.owners || [];
				product.rules = product.rules || [];
				selectedProduct = product;
			} else {
				console.error('Failed to load data product');
			}
		} catch (err) {
			console.error('Error loading data product:', err);
		}
	}

	function navigateToResult(result: SearchResult) {
		goto(result.url);
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

	function getResultTypeIcon(type: string): string {
		const iconMap: Record<string, string> = {
			asset: 'mdi:database',
			glossary: 'mdi:book-open-variant',
			team: 'mdi:account-group',
			data_product: 'mdi:package-variant-closed'
		};
		return iconMap[type] || 'mdi:file-document';
	}

	function getResultTypeColor(type: string): string {
		const colorMap: Record<string, string> = {
			asset: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300',
			glossary: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-300',
			team: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300',
			data_product:
				'bg-earthy-terracotta-100 text-earthy-terracotta-800 dark:bg-earthy-terracotta-900 dark:text-earthy-terracotta-300'
		};
		return colorMap[type] || 'bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-300';
	}

	function getKindLabel(kind: string): string {
		const labels: Record<string, string> = {
			asset: 'Asset',
			glossary: 'Glossary',
			team: 'Team',
			data_product: 'Product'
		};
		return labels[kind] || kind;
	}

	function formatDate(dateString: string): string {
		return new Date(dateString).toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric'
		});
	}

	function getResultSubtitle(result: SearchResult): string {
		if (result.type === 'asset' && result.metadata?.type) {
			return result.metadata.type;
		}
		if (result.description) {
			return result.description;
		}
		return '';
	}

	let hasActiveFilters = $derived(
		// Check if kinds differ from default (asset, glossary, team, data_product)
		!(
			selectedKinds.length === 4 &&
			selectedKinds.includes('asset') &&
			selectedKinds.includes('glossary') &&
			selectedKinds.includes('team') &&
			selectedKinds.includes('data_product')
		) ||
			selectedTypes.length > 0 ||
			selectedProviders.length > 0 ||
			selectedTags.length > 0
	);

	let showAssetFilters = $derived(selectedKinds.includes('asset'));
</script>

<svelte:head>
	<title>Discover - Marmot</title>
</svelte:head>

<div class="h-full flex flex-col">
	<!-- Main Content -->
	<div class="flex-1 overflow-hidden">
		<div class="max-w-[1600px] mx-auto px-4 py-4 h-full flex gap-4">
			<!-- Filters Sidebar -->
			<div class="{filtersExpanded ? 'w-56' : 'w-10'} flex-shrink-0 transition-all duration-300">
				<div
					class="sticky top-4 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm overflow-hidden"
				>
					<button
						onclick={() => (filtersExpanded = !filtersExpanded)}
						class="w-full px-3 py-2 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
						title={filtersExpanded ? 'Collapse filters' : 'Expand filters'}
					>
						{#if filtersExpanded}
							<h2
								class="font-semibold text-xs text-gray-900 dark:text-gray-100 uppercase tracking-wider"
							>
								Filters
							</h2>
							<IconifyIcon icon="mdi:chevron-left" class="text-gray-500" />
						{:else}
							<IconifyIcon icon="mdi:filter-outline" class="text-gray-500 mx-auto" />
						{/if}
					</button>

					{#if filtersExpanded}
						<div class="p-3">
							<!-- Clear Filters Link -->
							<div class="mb-3">
								<button
									onclick={clearAllFilters}
									class="text-xs text-earthy-terracotta-700 dark:text-earthy-terracotta-400 hover:text-earthy-terracotta-800 dark:hover:text-earthy-terracotta-300 font-medium"
								>
									Clear filters
								</button>
							</div>

							<!-- Kinds -->
							<div class="mb-4">
								<h3
									class="text-xs font-semibold text-gray-600 dark:text-gray-400 mb-2 uppercase tracking-wider"
								>
									Kind
								</h3>
								{#each ['asset', 'data_product', 'glossary', 'team'] as kind}
									<label class="flex items-center justify-between mb-2">
										<div class="flex items-center">
											<input
												type="checkbox"
												checked={selectedKinds.includes(kind)}
												onchange={(e) => {
													if (e.target.checked) {
														selectedKinds = [...selectedKinds, kind];
													} else {
														selectedKinds = selectedKinds.filter((k) => k !== kind);
													}
													handleFilterChange();
												}}
												class="rounded border-gray-300 dark:border-gray-600 text-earthy-terracotta-700 focus:ring-earthy-terracotta-600 dark:bg-gray-800"
											/>
											<span class="ml-2 text-sm text-gray-700 dark:text-gray-300"
												>{getKindLabel(kind)}</span
											>
										</div>
										<span class="text-xs text-gray-500 dark:text-gray-400"
											>({$facets.types[kind] || 0})</span
										>
									</label>
								{/each}
							</div>

							<!-- Asset-specific filters (only show when Asset is selected) -->
							{#if showAssetFilters}
								{#if $facets.asset_types.length > 0}
									<div class="mb-4">
										<h3
											class="text-xs font-semibold text-gray-600 dark:text-gray-400 mb-2 uppercase tracking-wider"
										>
											Type
										</h3>
										{#each $facets.asset_types as { value, count }}
											<label class="flex items-center justify-between mb-2">
												<div class="flex items-center">
													<input
														type="checkbox"
														checked={selectedTypes.includes(value)}
														onchange={(e) => {
															if (e.target.checked) {
																selectedTypes = [...selectedTypes, value];
																selectedKinds = ['asset'];
															} else {
																selectedTypes = selectedTypes.filter((t) => t !== value);
															}
															handleFilterChange();
														}}
														class="rounded border-gray-300 dark:border-gray-600 text-earthy-terracotta-700 focus:ring-earthy-terracotta-600 dark:bg-gray-800"
													/>
													<span class="ml-2 text-sm text-gray-700 dark:text-gray-300">{value}</span>
												</div>
												<span class="text-xs text-gray-500 dark:text-gray-400">({count})</span>
											</label>
										{/each}
									</div>
								{/if}

								{#if $facets.providers.length > 0}
									<div class="mb-4">
										<h3
											class="text-xs font-semibold text-gray-600 dark:text-gray-400 mb-2 uppercase tracking-wider"
										>
											Providers
										</h3>
										{#each $facets.providers as { value, count }}
											<label class="flex items-center justify-between mb-2">
												<div class="flex items-center">
													<input
														type="checkbox"
														checked={selectedProviders.includes(value)}
														onchange={(e) => {
															if (e.target.checked) {
																selectedProviders = [...selectedProviders, value];
																selectedKinds = ['asset'];
															} else {
																selectedProviders = selectedProviders.filter((s) => s !== value);
															}
															handleFilterChange();
														}}
														class="rounded border-gray-300 dark:border-gray-600 text-earthy-terracotta-700 focus:ring-earthy-terracotta-600 dark:bg-gray-800"
													/>
													<span class="ml-2 text-sm text-gray-700 dark:text-gray-300">{value}</span>
												</div>
												<span class="text-xs text-gray-500 dark:text-gray-400">({count})</span>
											</label>
										{/each}
									</div>
								{/if}

								{#if $facets.tags.length > 0}
									<div>
										<h3
											class="text-xs font-semibold text-gray-600 dark:text-gray-400 mb-2 uppercase tracking-wider"
										>
											Tags
										</h3>
										{#each $facets.tags as { value, count }}
											<label class="flex items-center justify-between mb-2 gap-2">
												<div class="flex items-center min-w-0">
													<input
														type="checkbox"
														checked={selectedTags.includes(value)}
														onchange={(e) => {
															if (e.target.checked) {
																selectedTags = [...selectedTags, value];
																selectedKinds = ['asset'];
															} else {
																selectedTags = selectedTags.filter((t) => t !== value);
															}
															handleFilterChange();
														}}
														class="flex-shrink-0 rounded border-gray-300 dark:border-gray-600 text-earthy-terracotta-700 focus:ring-earthy-terracotta-600 dark:bg-gray-800"
													/>
													<span
														class="ml-2 text-sm text-gray-700 dark:text-gray-300 truncate"
														title={value}
													>
														{value.length > 100 ? value.slice(0, 100) + '...' : value}
													</span>
												</div>
												<span class="flex-shrink-0 text-xs text-gray-500 dark:text-gray-400"
													>({count})</span
												>
											</label>
										{/each}
									</div>
								{/if}
							{/if}
						</div>
					{/if}
				</div>
			</div>

			<!-- Results Grid -->
			<div class="flex-1 flex flex-col min-h-0">
				<!-- Header with Filters -->
				<div class="mb-4">
					<div class="flex items-center justify-between mb-3">
						<h1 class="text-xl font-bold text-gray-900 dark:text-gray-100">Discover</h1>
						<div class="flex items-center gap-3">
							{#if canManageAssets}
								<Button
									href="/assets/new"
									icon="material-symbols:add"
									text="New Asset"
									variant="filled"
								/>
							{/if}
							<div class="text-xs text-gray-500 dark:text-gray-400">
								{$totalResults}
								{$totalResults === 1 ? 'result' : 'results'}
							</div>
						</div>
					</div>

					<!-- Query Builder (Collapsible) -->
					<div class="mb-3">
						<QueryBuilder
							query={searchQuery}
							onQueryChange={handleSearch}
							onRunClick={handleRunQuery}
							initiallyExpanded={queryBuilderExpanded}
						/>
					</div>

					{#if hasActiveFilters}
						<div
							class="bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 border border-earthy-terracotta-200 dark:border-earthy-terracotta-800 rounded-lg p-2"
						>
							<div class="flex items-center justify-between mb-1.5">
								<h2
									class="text-xs font-semibold text-earthy-terracotta-700 dark:text-earthy-terracotta-200 uppercase tracking-wider"
								>
									Active Filters
								</h2>
								<button
									onclick={clearAllFilters}
									class="text-xs text-earthy-terracotta-700 dark:text-earthy-terracotta-400 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-100 font-medium"
								>
									Clear all
								</button>
							</div>
							<div class="flex flex-wrap gap-1.5">
								{#each selectedTypes as type}
									<span
										class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium bg-white dark:bg-earthy-terracotta-900/40 text-earthy-terracotta-700 dark:text-earthy-terracotta-100 border border-earthy-terracotta-300 dark:border-earthy-terracotta-800"
									>
										<span class="text-earthy-terracotta-700 dark:text-earthy-terracotta-700"
											>Type:</span
										>
										{type}
										<button
											onclick={() => removeFilter('types', type)}
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

								{#each selectedProviders as provider}
									<span
										class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium bg-white dark:bg-earthy-terracotta-900/40 text-earthy-terracotta-700 dark:text-earthy-terracotta-100 border border-earthy-terracotta-300 dark:border-earthy-terracotta-800"
									>
										<span class="text-earthy-terracotta-700 dark:text-earthy-terracotta-700"
											>Provider:</span
										>
										{provider}
										<button
											onclick={() => removeFilter('providers', provider)}
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

								{#each selectedTags as tag}
									<span
										class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium bg-white dark:bg-earthy-terracotta-900/40 text-earthy-terracotta-700 dark:text-earthy-terracotta-100 border border-earthy-terracotta-300 dark:border-earthy-terracotta-800"
									>
										<span class="text-earthy-terracotta-700 dark:text-earthy-terracotta-700"
											>Tag:</span
										>
										{tag}
										<button
											onclick={() => removeFilter('tags', tag)}
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
					<div class="flex justify-between items-center mb-3">
						<p class="text-xs text-gray-600 dark:text-gray-400">
							{#if $totalResults > 0}
								{(currentPage - 1) * itemsPerPage + 1}-{Math.min(
									currentPage * itemsPerPage,
									$totalResults
								)} of {$totalResults}
							{/if}
						</p>
						<div class="flex gap-1.5">
							<button
								onclick={() => handlePageChange(currentPage - 1)}
								disabled={currentPage === 1}
								class="px-3 py-1.5 text-xs rounded-lg border border-gray-300 dark:border-gray-600 disabled:opacity-50 disabled:cursor-not-allowed text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
							>
								Previous
							</button>
							<button
								onclick={() => handlePageChange(currentPage + 1)}
								disabled={currentPage * itemsPerPage >= $totalResults}
								class="px-3 py-1.5 text-xs rounded-lg border border-gray-300 dark:border-gray-600 disabled:opacity-50 disabled:cursor-not-allowed text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
							>
								Next
							</button>
						</div>
					</div>

					<!-- Results List -->
					<div class="flex-1 overflow-y-auto space-y-2">
						{#if $isLoading}
							{#each Array(itemsPerPage) as _}
								<div
									class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-3 animate-pulse shadow-sm"
								>
									<div class="flex justify-between items-start mb-2">
										<div class="space-y-1.5">
											<div class="h-4 bg-gray-200 dark:bg-gray-600 rounded w-48"></div>
											<div class="h-3 bg-gray-200 dark:bg-gray-600 rounded w-64"></div>
										</div>
										<div class="h-5 bg-gray-200 dark:bg-gray-600 rounded w-20"></div>
									</div>
									<div class="h-3 bg-gray-200 dark:bg-gray-600 rounded w-3/4 mb-1.5"></div>
									<div class="flex gap-1.5">
										{#each Array(3) as _}
											<div class="h-5 bg-gray-200 dark:bg-gray-600 rounded w-14"></div>
										{/each}
									</div>
								</div>
							{/each}
						{:else if $results.length === 0}
							<div class="flex-1 flex items-center justify-center py-12">
								<div class="text-center">
									<div class="flex justify-center mb-4">
										<IconifyIcon icon="mdi:magnify" class="text-6xl text-gray-400" />
									</div>
									<p class="text-gray-600 dark:text-gray-400 text-lg">No results found</p>
									<p class="text-gray-500 dark:text-gray-500 text-sm mt-2">
										Try adjusting your search or filters
									</p>
								</div>
							</div>
						{:else}
							{#each $results as result}
								{#if result.type === 'asset'}
									<!-- Asset card -->
									<div
										role="button"
										tabindex="0"
										class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-3 hover:shadow-md hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 transition-all cursor-pointer group"
										onclick={() => handleAssetClick(result.id)}
										onkeydown={(e) => e.key === 'Enter' && handleAssetClick(result.id)}
									>
										<div class="flex justify-between items-start mb-2">
											<div class="flex items-center gap-2 flex-1 min-w-0">
												<div class="flex-shrink-0">
													<Icon name={getIconType(result.metadata)} showLabel={false} size="sm" />
												</div>
												<div class="min-w-0 flex-1">
													<h3
														class="font-semibold text-sm text-gray-900 dark:text-gray-100 truncate group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-700 transition-colors"
													>
														{result.name}
													</h3>
													<p class="text-xs text-gray-500 dark:text-gray-400 truncate font-mono">
														{result.metadata?.mrn}
													</p>
												</div>
											</div>
											<button
												onclick={(e) => handleTypeClick(result.metadata?.type, e)}
												class="flex-shrink-0 text-xs {getTagColor(
													result.metadata?.type
												)} px-2 py-0.5 rounded hover:opacity-80 transition-opacity font-medium"
											>
												{result.metadata?.type?.replace(/_/g, ' ')}
											</button>
										</div>

										{#if result.description}
											<p class="text-xs text-gray-600 dark:text-gray-400 mb-2 line-clamp-1">
												{result.description}
											</p>
										{/if}

										{#if result.metadata?.tags && result.metadata.tags.length > 0}
											<div class="flex flex-wrap gap-1 mb-2">
												{#each result.metadata.tags.slice(0, 3) as tag}
													<button
														onclick={(e) => handleTagClick(tag, e)}
														class="inline-flex items-center gap-0.5 text-xs bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 px-1.5 py-0.5 rounded hover:bg-earthy-terracotta-100 dark:hover:bg-earthy-terracotta-900/30 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-700 transition-colors"
													>
														<IconifyIcon icon="material-symbols:label-outline" class="w-3 h-3" />
														{tag}
													</button>
												{/each}
												{#if result.metadata.tags.length > 3}
													<span class="text-xs text-gray-500 dark:text-gray-400 px-1.5 py-0.5">
														+{result.metadata.tags.length - 3}
													</span>
												{/if}
											</div>
										{/if}

										<div
											class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400 pt-1.5 border-t border-gray-100 dark:border-gray-700"
										>
											{#if result.metadata?.created_by}
												<span>{result.metadata.created_by}</span>
												<span>•</span>
											{/if}
											<span>
												{#if result.metadata?.created_at}
													{formatDate(result.metadata.created_at)}
												{/if}
											</span>
										</div>
									</div>
								{:else if result.type === 'data_product'}
									<!-- Data Product card -->
									<div
										role="button"
										tabindex="0"
										class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-3 hover:shadow-md hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 transition-all cursor-pointer group"
										onclick={() => handleDataProductClick(result.id)}
										onkeydown={(e) => e.key === 'Enter' && handleDataProductClick(result.id)}
									>
										<div class="flex justify-between items-start mb-2">
											<div class="flex items-center gap-2 flex-1 min-w-0">
												<div class="flex-shrink-0">
													<div
														class="w-8 h-8 rounded-lg bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 flex items-center justify-center overflow-hidden"
													>
														{#if result.metadata?.icon_url}
															<AuthenticatedImage
																src={result.metadata.icon_url}
																alt="{result.name} icon"
																class="w-full h-full object-cover"
															/>
														{:else}
															<IconifyIcon
																icon="mdi:package-variant-closed"
																class="w-4 h-4 text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
															/>
														{/if}
													</div>
												</div>
												<div class="min-w-0 flex-1">
													<h3
														class="font-semibold text-sm text-gray-900 dark:text-gray-100 truncate group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-700 transition-colors"
													>
														{result.name}
													</h3>
													<div
														class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400"
													>
														{#if result.metadata?.asset_count !== undefined}
															<span class="flex items-center gap-1">
																<IconifyIcon icon="material-symbols:database" class="w-3 h-3" />
																{result.metadata.asset_count} assets
															</span>
														{/if}
														{#if result.metadata?.owner_count}
															<span class="flex items-center gap-1">
																<IconifyIcon icon="material-symbols:person" class="w-3 h-3" />
																{result.metadata.owner_count}
															</span>
														{/if}
													</div>
												</div>
											</div>
										</div>

										{#if result.description}
											<p class="text-xs text-gray-600 dark:text-gray-400 mb-2 line-clamp-1">
												{result.description}
											</p>
										{/if}

										{#if result.metadata?.tags && result.metadata.tags.length > 0}
											<div class="flex flex-wrap gap-1 mb-2">
												{#each result.metadata.tags.slice(0, 3) as tag}
													<span
														class="inline-flex items-center gap-0.5 text-xs bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 px-1.5 py-0.5 rounded"
													>
														<IconifyIcon icon="material-symbols:label-outline" class="w-3 h-3" />
														{tag}
													</span>
												{/each}
												{#if result.metadata.tags.length > 3}
													<span class="text-xs text-gray-500 dark:text-gray-400 px-1.5 py-0.5">
														+{result.metadata.tags.length - 3}
													</span>
												{/if}
											</div>
										{/if}

										<div
											class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400 pt-1.5 border-t border-gray-100 dark:border-gray-700"
										>
											{#if result.metadata?.created_by}
												<span>{result.metadata.created_by}</span>
												<span>•</span>
											{/if}
											<span>
												{#if result.updated_at}
													Updated {formatDate(result.updated_at)}
												{/if}
											</span>
										</div>
									</div>
								{:else}
									<!-- Other non-asset card (glossary, team) -->
									<div
										role="button"
										tabindex="0"
										class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-3 hover:shadow-md hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 transition-all cursor-pointer group"
										onclick={() => navigateToResult(result)}
										onkeydown={(e) => e.key === 'Enter' && navigateToResult(result)}
									>
										<div class="flex justify-between items-start mb-2">
											<div class="flex items-center gap-2 flex-1 min-w-0">
												<div class="flex-shrink-0">
													<IconifyIcon icon={getResultTypeIcon(result.type)} class="text-lg" />
												</div>
												<div class="min-w-0 flex-1">
													<h3
														class="font-semibold text-sm text-gray-900 dark:text-gray-100 truncate group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-700 transition-colors"
													>
														{result.name}
													</h3>
												</div>
											</div>
											<span
												class="flex-shrink-0 text-xs {getResultTypeColor(
													result.type
												)} px-2 py-0.5 rounded font-medium"
											>
												{getKindLabel(result.type)}
											</span>
										</div>

										{#if getResultSubtitle(result)}
											<p class="text-xs text-gray-600 dark:text-gray-400 line-clamp-1">
												{getResultSubtitle(result)}
											</p>
										{/if}
									</div>
								{/if}
							{/each}
						{/if}
					</div>
				{/if}
			</div>
		</div>
	</div>
</div>

{#if selectedAsset}
	<AssetBlade bind:asset={selectedAsset} onClose={() => (selectedAsset = null)} />
{/if}

{#if selectedProduct}
	<ProductBlade bind:product={selectedProduct} onClose={() => (selectedProduct = null)} />
{/if}
