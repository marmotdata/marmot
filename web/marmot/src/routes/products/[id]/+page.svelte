<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import type {
		DataProduct,
		ResolvedAssetsResponse,
		Owner,
		Rule,
		RuleInput
	} from '$lib/dataproducts/types';
	import type { Asset } from '$lib/assets/types';
	import ProductBlade from '$components/product/ProductBlade.svelte';
	import Button from '$components/ui/Button.svelte';
	import IconifyIcon from '@iconify/svelte';
	import MetadataView from '$components/shared/MetadataView.svelte';
	import AssetIcon from '$components/ui/Icon.svelte';
	import DocumentationSystem from '$components/docs/DocumentationSystem.svelte';
	import Tabs, { type Tab } from '$components/ui/Tabs.svelte';
	import QueryBuilder from '$components/query/QueryBuilder.svelte';
	import ConfirmModal from '$components/ui/ConfirmModal.svelte';
	import { auth } from '$lib/stores/auth';

	let productId = $derived($page.params.id);
	let activeTab = $derived($page.url.searchParams.get('tab') || 'documentation');

	let product = $state<DataProduct | null>(null);
	let resolvedAssets = $state<ResolvedAssetsResponse | null>(null);
	let assetDetails = $state<Map<string, Asset>>(new Map());

	let isLoading = $state(true);
	let loadError = $state<string | null>(null);
	let isLoadingAssets = $state(false);

	let bladeCollapsed = $state(false);

	let showRuleForm = $state(false);
	let editingRule = $state<Rule | null>(null);
	let ruleForm = $state<RuleInput>({
		name: '',
		rule_type: 'query',
		query_expression: '',
		is_enabled: true
	});
	let savingRule = $state(false);
	let ruleError = $state<string | null>(null);
	let deletingRuleId = $state<string | null>(null);
	let isPreviewLoading = $state(false);
	let rulePreviewResults = $state<any[]>([]);
	let rulePreviewTotal = $state(0);

	let showDeleteRuleModal = $state(false);
	let ruleToDelete = $state<Rule | null>(null);

	let assetSearchQuery = $state('');
	let assetSearchResults = $state<Asset[]>([]);
	let isSearchingAssets = $state(false);
	let isAddingAsset = $state(false);
	let removingAssetId = $state<string | null>(null);

	let canManage = $derived(auth.hasPermission('assets', 'manage'));

	const tabs: Tab[] = [
		{ id: 'documentation', label: 'Documentation', icon: 'material-symbols:description' },
		{ id: 'metadata', label: 'Metadata', icon: 'material-symbols:data-object' },
		{ id: 'assets', label: 'Assets', icon: 'material-symbols:database' },
		{ id: 'rules', label: 'Rules', icon: 'material-symbols:filter-list' }
	];

	// All tabs are always visible
	let visibleTabs = $derived(tabs);

	function setActiveTab(tab: string) {
		const url = new URL(window.location.href);
		url.searchParams.set('tab', tab);
		goto(url.toString(), { replaceState: true });
	}

	function handleBack() {
		window.history.back();
	}

	async function loadDataProduct(showLoading = true) {
		if (showLoading) {
			isLoading = true;
		}
		loadError = null;

		try {
			const response = await fetchApi(`/products/${productId}`);
			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to load data product');
			}

			const data = await response.json();
			// Ensure fields have default values for binding
			data.tags = data.tags || [];
			data.owners = data.owners || [];
			data.rules = data.rules || [];
			product = data;

			// Load resolved assets
			await loadResolvedAssets();
		} catch (err) {
			loadError = err instanceof Error ? err.message : 'Failed to load data product';
		} finally {
			if (showLoading) {
				isLoading = false;
			}
		}
	}

	async function loadResolvedAssets() {
		isLoadingAssets = true;
		try {
			const response = await fetchApi(`/products/resolved-assets/${productId}?limit=100`);
			if (response.ok) {
				resolvedAssets = await response.json();

				// Load asset details for display
				if (resolvedAssets && resolvedAssets.all_assets.length > 0) {
					await loadAssetDetails(resolvedAssets.all_assets.slice(0, 50));
				}
			}
		} catch (err) {
			console.error('Failed to load resolved assets:', err);
		} finally {
			isLoadingAssets = false;
		}
	}

	async function loadAssetDetails(assetIds: string[]) {
		const newDetails = new Map<string, Asset>();
		for (const assetId of assetIds) {
			try {
				const response = await fetchApi(`/assets/${assetId}`);
				if (response.ok) {
					const asset = await response.json();
					newDetails.set(assetId, asset);
				}
			} catch (err) {
				console.error(`Failed to load asset ${assetId}:`, err);
			}
		}
		assetDetails = newDetails;
	}

	function getAssetUrl(asset: Asset): string {
		if (!asset.mrn) return '#';
		const mrnParts = asset.mrn.replace('mrn://', '').split('/');
		if (mrnParts.length < 3) return '#';
		const type = mrnParts[0];
		const service = mrnParts[1];
		const fullName = mrnParts.slice(2).join('/');
		return `/discover/${encodeURIComponent(type)}/${encodeURIComponent(service)}/${encodeURIComponent(fullName)}`;
	}

	function getIconType(asset: Asset): string {
		if (asset.providers && Array.isArray(asset.providers) && asset.providers.length === 1) {
			return asset.providers[0];
		}
		return asset.type || 'unknown';
	}

	function formatDate(dateString: string): string {
		return new Date(dateString).toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	function openAddRuleForm() {
		editingRule = null;
		ruleForm = { name: '', rule_type: 'query', query_expression: '', is_enabled: true };
		ruleError = null;
		showRuleForm = true;
	}

	function openEditRuleForm(rule: Rule) {
		editingRule = rule;
		ruleForm = {
			name: rule.name,
			description: rule.description,
			rule_type: rule.rule_type,
			query_expression: rule.query_expression,
			is_enabled: rule.is_enabled
		};
		ruleError = null;
		showRuleForm = true;
	}

	function cancelRuleForm() {
		showRuleForm = false;
		editingRule = null;
		ruleError = null;
		rulePreviewResults = [];
		rulePreviewTotal = 0;
	}

	async function previewRule(queryExpression?: string) {
		isPreviewLoading = true;
		rulePreviewResults = [];
		rulePreviewTotal = 0;

		const query = queryExpression || ruleForm.query_expression || '';
		if (!query.trim()) {
			isPreviewLoading = false;
			return;
		}

		try {
			const params = new URLSearchParams({
				q: query,
				limit: '10'
			});
			params.append('types[]', 'asset');

			const response = await fetchApi(`/search?${params}`);

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to preview rule');
			}

			const data = await response.json();
			rulePreviewResults = (data.results || []).filter((r: any) => r.type === 'asset');
			rulePreviewTotal = data.total || rulePreviewResults.length;
		} catch (err) {
			console.error('Failed to preview rule:', err);
		} finally {
			isPreviewLoading = false;
		}
	}

	function getPreviewIconType(result: any): string {
		const metadata = result.metadata;
		if (
			metadata?.providers &&
			Array.isArray(metadata.providers) &&
			metadata.providers.length === 1
		) {
			return metadata.providers[0];
		}
		return metadata?.type || 'unknown';
	}

	function getPreviewAssetUrl(result: any): string {
		const mrn = result.metadata?.mrn || '';
		if (!mrn) return '#';
		const mrnParts = mrn.replace('mrn://', '').split('/');
		if (mrnParts.length < 3) return '#';
		const type = mrnParts[0];
		const service = mrnParts[1];
		const fullName = mrnParts.slice(2).join('/');
		return `/discover/${encodeURIComponent(type)}/${encodeURIComponent(service)}/${encodeURIComponent(fullName)}`;
	}

	async function saveRule() {
		if (!ruleForm.name.trim()) {
			ruleError = 'Rule name is required';
			return;
		}
		if (!ruleForm.query_expression?.trim()) {
			ruleError = 'Query expression is required';
			return;
		}

		savingRule = true;
		ruleError = null;

		try {
			const payload = {
				name: ruleForm.name.trim(),
				description: ruleForm.description?.trim() || undefined,
				rule_type: ruleForm.rule_type,
				query_expression: ruleForm.query_expression.trim(),
				is_enabled: ruleForm.is_enabled
			};

			let response: Response;
			if (editingRule) {
				response = await fetchApi(`/products/rules/${productId}/${editingRule.id}`, {
					method: 'PUT',
					body: JSON.stringify(payload)
				});
			} else {
				response = await fetchApi(`/products/rules/${productId}`, {
					method: 'POST',
					body: JSON.stringify(payload)
				});
			}

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to save rule');
			}

			// Reload product to get updated rules and resolved assets (without full page spinner)
			await loadDataProduct(false);
			showRuleForm = false;
			editingRule = null;
			rulePreviewResults = [];
			rulePreviewTotal = 0;
		} catch (err) {
			ruleError = err instanceof Error ? err.message : 'Failed to save rule';
		} finally {
			savingRule = false;
		}
	}

	function confirmDeleteRule(rule: Rule) {
		ruleToDelete = rule;
		showDeleteRuleModal = true;
	}

	async function deleteRule() {
		if (!ruleToDelete) return;

		const ruleId = ruleToDelete.id;
		showDeleteRuleModal = false;
		deletingRuleId = ruleId;

		try {
			const response = await fetchApi(`/products/rules/${productId}/${ruleId}`, {
				method: 'DELETE'
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to delete rule');
			}

			// Reload product to get updated rules and resolved assets (without full page spinner)
			await loadDataProduct(false);
		} catch (err) {
			console.error('Failed to delete rule:', err);
		} finally {
			deletingRuleId = null;
			ruleToDelete = null;
		}
	}

	async function toggleRuleEnabled(rule: Rule) {
		try {
			const response = await fetchApi(`/products/rules/${productId}/${rule.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					name: rule.name,
					description: rule.description,
					rule_type: rule.rule_type,
					query_expression: rule.query_expression,
					is_enabled: !rule.is_enabled
				})
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to update rule');
			}

			// Reload product to get updated rules and resolved assets (without full page spinner)
			await loadDataProduct(false);
		} catch (err) {
			console.error('Failed to toggle rule:', err);
		}
	}

	// Asset management functions
	let searchTimeout: ReturnType<typeof setTimeout>;

	function handleAssetSearch(query: string) {
		assetSearchQuery = query;
		if (searchTimeout) clearTimeout(searchTimeout);

		if (!query.trim()) {
			assetSearchResults = [];
			return;
		}

		searchTimeout = setTimeout(async () => {
			isSearchingAssets = true;
			try {
				const response = await fetchApi(
					`/search?q=${encodeURIComponent(query)}&kinds=asset&limit=10`
				);
				if (response.ok) {
					const data = await response.json();
					// Filter out assets already in the product
					const existingIds = new Set(resolvedAssets?.all_assets || []);
					assetSearchResults = (data.results || [])
						.filter((r: any) => r.type === 'asset' && !existingIds.has(r.id))
						.map((r: any) => ({
							id: r.id,
							name: r.name,
							mrn:
								r.metadata?.mrn || r.url?.replace('/discover/', 'mrn://').replace(/\//g, '/') || '',
							type: r.metadata?.type || 'unknown',
							providers: r.metadata?.providers || [],
							description: r.description
						}));
				}
			} catch (err) {
				console.error('Failed to search assets:', err);
			} finally {
				isSearchingAssets = false;
			}
		}, 300);
	}

	async function addAssetToProduct(assetId: string) {
		isAddingAsset = true;
		try {
			const response = await fetchApi(`/products/assets/${productId}`, {
				method: 'POST',
				body: JSON.stringify({ asset_id: assetId })
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to add asset');
			}

			// Clear search and reload
			assetSearchQuery = '';
			assetSearchResults = [];
			await loadDataProduct();
		} catch (err) {
			console.error('Failed to add asset:', err);
		} finally {
			isAddingAsset = false;
		}
	}

	async function removeAssetFromProduct(assetId: string) {
		removingAssetId = assetId;
		try {
			const response = await fetchApi(`/products/assets/${productId}/${assetId}`, {
				method: 'DELETE'
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to remove asset');
			}

			await loadDataProduct();
		} catch (err) {
			console.error('Failed to remove asset:', err);
		} finally {
			removingAssetId = null;
		}
	}

	onMount(() => {
		loadDataProduct();
	});
</script>

<div class="h-full flex">
	{#if isLoading}
		<div class="flex items-center justify-center w-full">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"></div>
		</div>
	{:else if loadError}
		<div class="flex items-center justify-center w-full">
			<div
				class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-6 text-center max-w-md"
			>
				<IconifyIcon icon="material-symbols:error" class="h-12 w-12 text-red-400 mx-auto mb-3" />
				<h3 class="text-lg font-medium text-red-800 dark:text-red-200 mb-2">Failed to Load</h3>
				<p class="text-sm text-red-600 dark:text-red-300 mb-4">{loadError}</p>
				<Button click={() => goto('/products')} text="Back to Data Products" variant="clear" />
			</div>
		</div>
	{:else if product}
		<div class="flex-1 flex flex-col min-w-0">
			<div class="flex-none p-8">
				<div class="mb-6">
					<button
						onclick={handleBack}
						class="inline-flex items-center text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300"
					>
						<svg class="w-5 h-5 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M10 19l-7-7m0 0l7-7m-7 7h18"
							/>
						</svg>
						Back
					</button>
				</div>

				<div class="mb-6">
					<div class="flex flex-col gap-4">
						<div>
							<h1 class="text-2xl font-semibold text-gray-900 dark:text-gray-100">
								{product.name}
							</h1>
							{#if product.description}
								<p class="text-sm text-gray-500 dark:text-gray-400 mt-1 max-w-2xl">
									{product.description}
								</p>
							{/if}
						</div>
						<div class="flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
							<span class="flex items-center gap-1.5">
								<IconifyIcon icon="material-symbols:database" class="h-4 w-4" />
								{product.asset_count || 0} assets
							</span>
							{#if product.rules && product.rules.length > 0}
								<span class="flex items-center gap-1.5">
									<IconifyIcon icon="material-symbols:filter-list" class="h-4 w-4" />
									{product.rules.length} rule{product.rules.length === 1 ? '' : 's'}
								</span>
							{/if}
						</div>
					</div>
				</div>

				<Tabs tabs={visibleTabs} bind:activeTab onTabChange={setActiveTab} />
			</div>

			<div class="flex-1 overflow-y-auto overflow-x-auto px-8">
				<div class="pb-16 max-w-7xl mx-auto">
					<div class="rounded-lg max-w-full overflow-x-auto">
						<!-- Metadata Tab -->
						{#if activeTab === 'metadata'}
							<div class="mt-6">
								<MetadataView
									metadata={product.metadata || {}}
									endpoint="/products"
									id={productId}
									readOnly={!canManage}
									permissionResource="assets"
									permissionAction="manage"
								/>
							</div>
						{/if}

						<!-- Documentation Tab -->
						{#if activeTab === 'documentation'}
							<div class="mt-6" style="height: calc(100vh - 320px); min-height: 400px;">
								<DocumentationSystem entityType="data_product" entityId={product.id} />
							</div>
						{/if}

						<!-- Assets Tab -->
						{#if activeTab === 'assets'}
							<div class="mt-6">
								<div
									class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
								>
									<div class="flex items-center justify-between mb-4">
										<h3
											class="text-base font-semibold text-gray-900 dark:text-gray-100 flex items-center gap-2"
										>
											<IconifyIcon icon="material-symbols:database" class="h-5 w-5" />
											Assets
											{#if resolvedAssets}
												<span class="text-sm font-normal text-gray-500">
													({resolvedAssets.total} total)
												</span>
											{/if}
										</h3>
									</div>

									<!-- Add Asset Search -->
									{#if canManage}
										<div class="mb-4">
											<label
												class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
											>
												Add Assets
											</label>
											<div class="relative">
												<input
													type="text"
													value={assetSearchQuery}
													oninput={(e) => handleAssetSearch(e.currentTarget.value)}
													placeholder="Search for assets to add..."
													class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-earthy-terracotta-500"
												/>
												{#if isSearchingAssets}
													<div class="absolute right-3 top-1/2 -translate-y-1/2">
														<div
															class="h-4 w-4 border-2 border-earthy-terracotta-500 border-t-transparent rounded-full animate-spin"
														></div>
													</div>
												{/if}
											</div>

											<!-- Search Results -->
											{#if assetSearchResults.length > 0}
												<div
													class="mt-2 border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden"
												>
													{#each assetSearchResults as asset}
														<div
															class="flex items-center gap-3 p-3 hover:bg-gray-50 dark:hover:bg-gray-700/50 border-b border-gray-100 dark:border-gray-700 last:border-b-0"
														>
															<AssetIcon name={getIconType(asset)} size="sm" showLabel={false} />
															<div class="flex-1 min-w-0">
																<div
																	class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate"
																>
																	{asset.name}
																</div>
																<div
																	class="text-xs text-gray-500 dark:text-gray-400 truncate font-mono"
																>
																	{asset.mrn}
																</div>
															</div>
															<button
																onclick={() => addAssetToProduct(asset.id)}
																disabled={isAddingAsset}
																class="px-3 py-1 text-xs font-medium text-white bg-earthy-terracotta-600 hover:bg-earthy-terracotta-700 rounded-lg disabled:opacity-50 transition-colors"
															>
																{isAddingAsset ? 'Adding...' : 'Add'}
															</button>
														</div>
													{/each}
												</div>
											{:else if assetSearchQuery && !isSearchingAssets}
												<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
													No assets found matching "{assetSearchQuery}"
												</p>
											{/if}
										</div>
									{/if}

									{#if isLoadingAssets}
										<div class="flex justify-center py-8">
											<div
												class="animate-spin rounded-full h-8 w-8 border-b-2 border-earthy-terracotta-700"
											></div>
										</div>
									{:else if resolvedAssets && resolvedAssets.all_assets.length > 0}
										<!-- Asset counts by type -->
										<div class="flex gap-4 mb-4 text-sm">
											{#if resolvedAssets.manual_assets.length > 0}
												<span
													class="px-3 py-1 rounded-full bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300"
												>
													{resolvedAssets.manual_assets.length} manual
												</span>
											{/if}
											{#if resolvedAssets.dynamic_assets.length > 0}
												<span
													class="px-3 py-1 rounded-full bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300"
												>
													{resolvedAssets.dynamic_assets.length} from rules
												</span>
											{/if}
										</div>

										<div class="space-y-2">
											{#each resolvedAssets.all_assets as assetId}
												{@const asset = assetDetails.get(assetId)}
												{@const isManual = resolvedAssets.manual_assets.includes(assetId)}
												<div
													class="flex items-center gap-3 p-3 rounded-lg border border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-all group"
												>
													{#if asset}
														<a
															href={getAssetUrl(asset)}
															class="flex items-center gap-3 flex-1 min-w-0"
														>
															<AssetIcon name={getIconType(asset)} size="sm" showLabel={false} />
															<div class="flex-1 min-w-0">
																<div
																	class="text-sm font-medium text-gray-900 dark:text-gray-100 truncate group-hover:text-earthy-terracotta-700"
																>
																	{asset.name}
																</div>
																<div
																	class="text-xs text-gray-500 dark:text-gray-400 truncate font-mono"
																>
																	{asset.mrn}
																</div>
															</div>
														</a>
														<span
															class="text-xs px-2 py-0.5 rounded-full {isManual
																? 'bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400'
																: 'bg-purple-100 dark:bg-purple-900/30 text-purple-600 dark:text-purple-400'}"
														>
															{isManual ? 'manual' : 'rule'}
														</span>
														{#if canManage && isManual}
															<button
																onclick={() => removeAssetFromProduct(assetId)}
																disabled={removingAssetId === assetId}
																class="p-1 text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors disabled:opacity-50"
																title="Remove from product"
															>
																{#if removingAssetId === assetId}
																	<div
																		class="h-4 w-4 border-2 border-gray-400 border-t-transparent rounded-full animate-spin"
																	></div>
																{:else}
																	<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
																{/if}
															</button>
														{/if}
													{:else}
														<div class="text-sm text-gray-500">{assetId}</div>
													{/if}
												</div>
											{/each}
										</div>
									{:else}
										<div class="text-center py-8 text-gray-500 dark:text-gray-400">
											<IconifyIcon
												icon="material-symbols:database"
												class="h-12 w-12 mx-auto mb-3 opacity-50"
											/>
											<p>No assets in this data product</p>
											<p class="text-sm mt-1">
												{#if canManage}
													Search above to add assets manually, or create rules to include assets
													dynamically
												{:else}
													Assets can be added manually or matched by rules
												{/if}
											</p>
										</div>
									{/if}
								</div>
							</div>
						{/if}

						<!-- Rules Tab -->
						{#if activeTab === 'rules'}
							<div class="mt-6">
								<div
									class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
								>
									<div class="flex items-center justify-between mb-4">
										<h3
											class="text-base font-semibold text-gray-900 dark:text-gray-100 flex items-center gap-2"
										>
											<IconifyIcon icon="material-symbols:filter-list" class="h-5 w-5" />
											Dynamic Rules
											{#if product.rules}
												<span class="text-sm font-normal text-gray-500">
													({product.rules.length})
												</span>
											{/if}
										</h3>
										{#if canManage && !showRuleForm}
											<Button
												click={openAddRuleForm}
												icon="material-symbols:add"
												text="Add Rule"
												variant="filled"
											/>
										{/if}
									</div>

									<!-- Rule Form -->
									{#if showRuleForm && canManage}
										<div
											class="mb-6 p-4 rounded-lg border border-earthy-terracotta-200 dark:border-earthy-terracotta-800 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20"
										>
											<h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-4">
												{editingRule ? 'Edit Rule' : 'Add New Rule'}
											</h4>

											{#if ruleError}
												<div
													class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg"
												>
													<p class="text-sm text-red-800 dark:text-red-200">{ruleError}</p>
												</div>
											{/if}

											<div class="space-y-4">
												<div>
													<label
														class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
													>
														Rule Name *
													</label>
													<input
														type="text"
														bind:value={ruleForm.name}
														placeholder="e.g., All staging tables"
														class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-earthy-terracotta-500"
													/>
												</div>

												<div>
													<label
														class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
													>
														Description
													</label>
													<input
														type="text"
														bind:value={ruleForm.description}
														placeholder="Optional description"
														class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-earthy-terracotta-500 focus:border-earthy-terracotta-500"
													/>
												</div>

												<div>
													<label
														class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
													>
														Query Expression *
													</label>
													<QueryBuilder
														query={ruleForm.query_expression || ''}
														onQueryChange={(q) => (ruleForm.query_expression = q)}
														initiallyExpanded={true}
														showRunButton={true}
														runButtonText={isPreviewLoading ? 'Searching...' : 'Preview'}
														runButtonIcon={isPreviewLoading
															? 'mdi:loading'
															: 'material-symbols:visibility'}
														onRunClick={(q) => previewRule(q)}
													/>
												</div>

												<!-- Preview Results -->
												{#if isPreviewLoading}
													<div class="space-y-2">
														{#each Array(3) as _}
															<div
																class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-600 p-3 animate-pulse"
															>
																<div class="flex items-center gap-2">
																	<div class="h-6 w-6 bg-gray-200 dark:bg-gray-600 rounded"></div>
																	<div class="space-y-1.5 flex-1">
																		<div
																			class="h-4 bg-gray-200 dark:bg-gray-600 rounded w-48"
																		></div>
																		<div
																			class="h-3 bg-gray-200 dark:bg-gray-600 rounded w-64"
																		></div>
																	</div>
																</div>
															</div>
														{/each}
													</div>
												{:else if rulePreviewResults.length > 0}
													<div class="space-y-3">
														<div class="flex items-center justify-between">
															<div
																class="flex items-center gap-2 text-sm text-green-700 dark:text-green-400"
															>
																<IconifyIcon icon="material-symbols:check-circle" class="h-4 w-4" />
																<span class="font-medium"
																	>{rulePreviewTotal} asset{rulePreviewTotal !== 1 ? 's' : ''} matched</span
																>
															</div>
															{#if rulePreviewTotal > rulePreviewResults.length}
																<span class="text-xs text-gray-500 dark:text-gray-400">
																	Showing first {rulePreviewResults.length}
																</span>
															{/if}
														</div>
														<div class="space-y-2 max-h-64 overflow-y-auto">
															{#each rulePreviewResults as result}
																<a
																	href={getPreviewAssetUrl(result)}
																	target="_blank"
																	class="block bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-600 p-3 hover:shadow-md hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 transition-all group"
																>
																	<div class="flex items-start gap-3">
																		<div class="flex-shrink-0">
																			<AssetIcon
																				name={getPreviewIconType(result)}
																				showLabel={false}
																				size="sm"
																			/>
																		</div>
																		<div class="flex-1 min-w-0">
																			<div class="flex items-center justify-between gap-2">
																				<h4
																					class="font-medium text-sm text-gray-900 dark:text-gray-100 truncate group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-400 transition-colors"
																				>
																					{result.name}
																				</h4>
																				<span
																					class="flex-shrink-0 text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 px-2 py-0.5 rounded"
																				>
																					{result.metadata?.type?.replace(/_/g, ' ')}
																				</span>
																			</div>
																			<p
																				class="text-xs text-gray-500 dark:text-gray-400 truncate font-mono mt-0.5"
																			>
																				{result.metadata?.mrn}
																			</p>
																			{#if result.description}
																				<p
																					class="text-xs text-gray-600 dark:text-gray-400 mt-1 line-clamp-1"
																				>
																					{result.description}
																				</p>
																			{/if}
																		</div>
																	</div>
																</a>
															{/each}
														</div>
													</div>
												{:else if ruleForm.query_expression}
													<div class="text-center py-6 text-gray-500 dark:text-gray-400">
														<IconifyIcon
															icon="material-symbols:search"
															class="h-8 w-8 mx-auto mb-2 opacity-40"
														/>
														<p class="text-sm">
															Click "Preview" to see which assets match this rule
														</p>
													</div>
												{/if}

												<div class="flex items-center gap-2">
													<input
														type="checkbox"
														id="rule-enabled"
														bind:checked={ruleForm.is_enabled}
														class="rounded border-gray-300 dark:border-gray-600 text-earthy-terracotta-600 focus:ring-earthy-terracotta-500"
													/>
													<label
														for="rule-enabled"
														class="text-sm text-gray-700 dark:text-gray-300"
													>
														Enable this rule
													</label>
												</div>

												<div class="flex gap-2 pt-2">
													<Button
														click={saveRule}
														icon={savingRule ? '' : 'material-symbols:save'}
														text={savingRule ? 'Saving...' : 'Save Rule'}
														variant="filled"
														disabled={savingRule}
													/>
													<Button
														click={cancelRuleForm}
														text="Cancel"
														variant="clear"
														disabled={savingRule}
													/>
												</div>
											</div>
										</div>
									{/if}

									{#if product.rules && product.rules.length > 0}
										<div class="space-y-3">
											{#each product.rules as rule}
												<div
													class="p-4 rounded-lg border border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-gray-700/30"
												>
													<div class="flex items-start justify-between">
														<div class="flex items-center gap-3">
															<div
																class="w-8 h-8 rounded-lg flex items-center justify-center bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400"
															>
																<IconifyIcon icon="mdi:database-search" class="h-4 w-4" />
															</div>
															<div>
																<div class="text-sm font-semibold text-gray-900 dark:text-gray-100">
																	{rule.name}
																</div>
																{#if rule.description}
																	<div class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
																		{rule.description}
																	</div>
																{/if}
															</div>
														</div>
														<div class="flex items-center gap-2">
															{#if rule.matched_asset_count !== undefined}
																<span
																	class="text-xs px-2 py-0.5 rounded-full bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400"
																>
																	{rule.matched_asset_count} matched
																</span>
															{/if}
															<button
																onclick={() => toggleRuleEnabled(rule)}
																class="text-xs px-2 py-0.5 rounded-full cursor-pointer transition-colors {rule.is_enabled
																	? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 hover:bg-green-200 dark:hover:bg-green-900/50'
																	: 'bg-gray-100 dark:bg-gray-700 text-gray-500 hover:bg-gray-200 dark:hover:bg-gray-600'}"
																title={rule.is_enabled ? 'Click to disable' : 'Click to enable'}
															>
																{rule.is_enabled ? 'Active' : 'Disabled'}
															</button>
															{#if canManage}
																<button
																	onclick={() => openEditRuleForm(rule)}
																	class="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
																	title="Edit rule"
																>
																	<IconifyIcon icon="material-symbols:edit" class="h-4 w-4" />
																</button>
																<button
																	onclick={() => confirmDeleteRule(rule)}
																	disabled={deletingRuleId === rule.id}
																	class="p-1 text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors disabled:opacity-50"
																	title="Delete rule"
																>
																	{#if deletingRuleId === rule.id}
																		<div
																			class="h-4 w-4 border-2 border-gray-400 border-t-transparent rounded-full animate-spin"
																		></div>
																	{:else}
																		<IconifyIcon icon="material-symbols:delete" class="h-4 w-4" />
																	{/if}
																</button>
															{/if}
														</div>
													</div>
													{#if rule.query_expression}
														<div
															class="mt-3 px-3 py-2 bg-white dark:bg-gray-800 rounded-lg font-mono text-xs text-gray-600 dark:text-gray-400 border border-gray-200 dark:border-gray-600"
														>
															{rule.query_expression}
														</div>
													{/if}
												</div>
											{/each}
										</div>
									{:else if !showRuleForm}
										<div class="text-center py-8 text-gray-500 dark:text-gray-400">
											<IconifyIcon
												icon="material-symbols:filter-list"
												class="h-12 w-12 mx-auto mb-3 opacity-50"
											/>
											<p>No rules defined</p>
											<p class="text-sm mt-1">
												Rules automatically include assets matching specific criteria
											</p>
											{#if canManage}
												<div class="mt-4">
													<Button
														click={openAddRuleForm}
														icon="material-symbols:add"
														text="Add Your First Rule"
														variant="filled"
													/>
												</div>
											{/if}
										</div>
									{/if}
								</div>
							</div>
						{/if}
					</div>
				</div>
			</div>
		</div>

		<div
			class="border-l border-gray-200 dark:border-gray-700 overflow-hidden transition-all duration-300 {bladeCollapsed
				? 'w-12'
				: 'w-[36rem]'}"
		>
			<ProductBlade
				{product}
				staticPlacement={true}
				collapsed={bladeCollapsed}
				onToggleCollapse={() => (bladeCollapsed = !bladeCollapsed)}
				onClose={() => {}}
			/>
		</div>
	{/if}
</div>

<!-- Delete Rule Confirmation Modal -->
<ConfirmModal
	bind:show={showDeleteRuleModal}
	title="Delete Rule"
	message={ruleToDelete
		? `Are you sure you want to delete the rule "${ruleToDelete.name}"? This will remove any dynamically matched assets from this data product.`
		: ''}
	confirmText="Delete"
	cancelText="Cancel"
	variant="danger"
	onConfirm={deleteRule}
	onCancel={() => {
		showDeleteRuleModal = false;
		ruleToDelete = null;
	}}
/>
