<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { fetchApi } from '$lib/api';
	import type { DataProduct, ResolvedAssetsResponse, Owner } from '$lib/dataproducts/types';
	import type { Asset } from '$lib/assets/types';
	import Button from '../../../components/Button.svelte';
	import IconifyIcon from '@iconify/svelte';
	import Tags from '../../../components/Tags.svelte';
	import MetadataView from '../../../components/MetadataView.svelte';
	import AssetIcon from '../../../components/Icon.svelte';
	import OwnerSelector from '../../../components/OwnerSelector.svelte';
	import Documentation from '../../../components/Documentation.svelte';
	import Tabs, { type Tab } from '../../../components/Tabs.svelte';
	import { auth } from '$lib/stores/auth';
	import { marked } from 'marked';

	let productId = $derived($page.params.id);
	let activeTab = $derived($page.url.searchParams.get('tab') || 'overview');

	// Data
	let product = $state<DataProduct | null>(null);
	let resolvedAssets = $state<ResolvedAssetsResponse | null>(null);
	let assetDetails = $state<Map<string, Asset>>(new Map());

	// Loading states
	let isLoading = $state(true);
	let loadError = $state<string | null>(null);
	let isLoadingAssets = $state(false);

	// Permissions
	let canManage = $derived(auth.hasPermission('assets', 'manage'));

	// Tabs configuration
	const tabs: Tab[] = [
		{ id: 'overview', label: 'Overview', icon: 'material-symbols:info-outline' },
		{ id: 'documentation', label: 'Documentation', icon: 'material-symbols:description' },
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

	async function loadDataProduct() {
		isLoading = true;
		loadError = null;

		try {
			const response = await fetchApi(`/products/${productId}`);
			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to load data product');
			}

			const data = await response.json();
			// Ensure tags is always an array for binding
			data.tags = data.tags || [];
			product = data;

			// Load resolved assets
			await loadResolvedAssets();
		} catch (err) {
			loadError = err instanceof Error ? err.message : 'Failed to load data product';
		} finally {
			isLoading = false;
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

	function renderDocumentation(content: string): string {
		return marked(content) as string;
	}

	async function handleOwnersChange(newOwners: Owner[]) {
		if (!product || !canManage) return;

		try {
			const response = await fetchApi(`/products/${productId}`, {
				method: 'PUT',
				body: JSON.stringify({
					owners: newOwners.map((o) => ({ id: o.id, type: o.type }))
				})
			});

			if (response.ok) {
				const updated = await response.json();
				product = updated;
			}
		} catch (err) {
			console.error('Failed to update owners:', err);
		}
	}

	onMount(() => {
		loadDataProduct();
	});
</script>

{#if isLoading}
	<div class="min-h-screen flex justify-center items-center py-32">
		<div
			class="animate-spin rounded-full h-10 w-10 border-b-2 border-earthy-terracotta-700 dark:border-earthy-terracotta-500"
		></div>
	</div>
{:else if loadError}
	<div class="min-h-screen">
		<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
			<div
				class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-6 text-center"
			>
				<IconifyIcon icon="material-symbols:error" class="h-12 w-12 text-red-400 mx-auto mb-3" />
				<h3 class="text-lg font-medium text-red-800 dark:text-red-200 mb-2">Failed to Load</h3>
				<p class="text-sm text-red-600 dark:text-red-300 mb-4">{loadError}</p>
				<Button
					click={() => goto('/products')}
					text="Back to Data Products"
					variant="clear"
				/>
			</div>
		</div>
	</div>
{:else if product}
	<div class="min-h-screen">
		<!-- Header -->
		<div class="border-b border-gray-200 dark:border-gray-700">
			<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
				<div class="flex items-start justify-between">
					<div class="flex items-start gap-4">
						<button
							onclick={() => goto('/products')}
							class="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors mt-1"
						>
							<IconifyIcon
								icon="material-symbols:arrow-back"
								class="h-5 w-5 text-gray-600 dark:text-gray-400"
							/>
						</button>
						<div>
							<h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">
								{product.name}
							</h1>
							{#if product.description}
								<p class="text-sm text-gray-600 dark:text-gray-400 mt-1 max-w-2xl">
									{product.description}
								</p>
							{/if}
							<div class="flex items-center gap-4 mt-3 text-sm text-gray-500 dark:text-gray-400">
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
								<span class="flex items-center gap-1.5">
									<IconifyIcon icon="material-symbols:schedule" class="h-4 w-4" />
									Updated {formatDate(product.updated_at)}
								</span>
							</div>
						</div>
					</div>
				</div>
			</div>
		</div>

		<!-- Tab Navigation -->
		<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
			<Tabs
				tabs={visibleTabs}
				activeTab={activeTab}
				onTabChange={setActiveTab}
			/>
		</div>

		<!-- Tab Content -->
		<div class="container max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
			<!-- Overview Tab -->
			{#if activeTab === 'overview'}
				<div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
					<!-- Main Info -->
					<div class="lg:col-span-2 space-y-6">
						<!-- Documentation Preview -->
						{#if product.documentation && product.documentation.trim().length > 0}
							{@const isLongDoc = product.documentation.split('\n').length > 60 || product.documentation.length > 3000}
							<div
								class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
							>
								<div class="flex items-center justify-between mb-3">
									<h3
										class="text-sm font-semibold text-gray-900 dark:text-gray-100 flex items-center gap-2"
									>
										<IconifyIcon icon="material-symbols:description" class="h-4 w-4" />
										Documentation
									</h3>
								</div>
								<div class="relative">
									<div
										class="prose prose-sm prose-gray dark:prose-invert max-w-none prose-headings:text-gray-900 dark:prose-headings:text-gray-100 prose-p:text-gray-600 dark:prose-p:text-gray-300 prose-a:text-earthy-terracotta-600 dark:prose-a:text-earthy-terracotta-400 {isLongDoc ? 'max-h-96 overflow-hidden' : ''}"
									>
										{@html renderDocumentation(product.documentation)}
									</div>
									{#if isLongDoc}
										<div class="absolute bottom-0 left-0 right-0 h-24 bg-gradient-to-t from-white dark:from-gray-800 to-transparent pointer-events-none"></div>
									{/if}
								</div>
								{#if isLongDoc}
									<div class="mt-4 text-center">
										<button
											onclick={() => setActiveTab('documentation')}
											class="text-sm text-earthy-terracotta-600 hover:text-earthy-terracotta-700 dark:text-earthy-terracotta-400 inline-flex items-center gap-1"
										>
											View full documentation
											<IconifyIcon icon="material-symbols:arrow-forward" class="h-4 w-4" />
										</button>
									</div>
								{/if}
							</div>
						{/if}

						<!-- Metadata -->
						<div
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
						>
							<h3
								class="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-3 flex items-center gap-2"
							>
								<IconifyIcon icon="material-symbols:data-object" class="h-4 w-4" />
								Metadata
							</h3>
							<MetadataView
								metadata={product.metadata || {}}
								endpoint="/products"
								id={productId}
								readOnly={!canManage}
								permissionResource="assets"
								permissionAction="manage"
							/>
						</div>

						<!-- Recent Assets Preview -->
						{#if resolvedAssets && resolvedAssets.all_assets.length > 0}
							<div
								class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
							>
								<div class="flex items-center justify-between mb-4">
									<h3
										class="text-sm font-semibold text-gray-900 dark:text-gray-100 flex items-center gap-2"
									>
										<IconifyIcon icon="material-symbols:database" class="h-4 w-4" />
										Assets
									</h3>
									<button
										onclick={() => setActiveTab('assets')}
										class="text-xs text-earthy-terracotta-600 hover:text-earthy-terracotta-700 dark:text-earthy-terracotta-400"
									>
										View all
									</button>
								</div>
								<div class="space-y-2">
									{#each resolvedAssets.all_assets.slice(0, 5) as assetId}
										{@const asset = assetDetails.get(assetId)}
										{#if asset}
											<a
												href={getAssetUrl(asset)}
												class="flex items-center gap-3 p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
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
											</a>
										{:else}
											<div class="p-3 text-sm text-gray-500">{assetId}</div>
										{/if}
									{/each}
								</div>
							</div>
						{/if}
					</div>

					<!-- Sidebar -->
					<div class="space-y-6">
						<!-- Tags -->
						<div
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
						>
							<h3
								class="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-3 flex items-center gap-2"
							>
								<IconifyIcon icon="material-symbols:label" class="h-4 w-4" />
								Tags
							</h3>
							<Tags
								bind:tags={product.tags}
								endpoint="/products"
								id={productId}
								canEdit={canManage}
							/>
						</div>

						<!-- Owners -->
						<div
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
						>
							<h3
								class="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-3 flex items-center gap-2"
							>
								<IconifyIcon icon="material-symbols:group" class="h-4 w-4" />
								Owners
							</h3>
							<OwnerSelector
								selectedOwners={product.owners || []}
								onChange={handleOwnersChange}
								disabled={!canManage}
							/>
						</div>

						<!-- Details -->
						<div
							class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
						>
							<h3
								class="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-3 flex items-center gap-2"
							>
								<IconifyIcon icon="material-symbols:info" class="h-4 w-4" />
								Details
							</h3>
							<dl class="space-y-3 text-sm">
								<div>
									<dt class="text-gray-500 dark:text-gray-400">Created</dt>
									<dd class="text-gray-900 dark:text-gray-100 mt-0.5">
										{formatDate(product.created_at)}
									</dd>
								</div>
								<div>
									<dt class="text-gray-500 dark:text-gray-400">Last Updated</dt>
									<dd class="text-gray-900 dark:text-gray-100 mt-0.5">
										{formatDate(product.updated_at)}
									</dd>
								</div>
								{#if product.created_by}
									<div>
										<dt class="text-gray-500 dark:text-gray-400">Created By</dt>
										<dd class="text-gray-900 dark:text-gray-100 mt-0.5">
											{product.created_by}
										</dd>
									</div>
								{/if}
							</dl>
						</div>
					</div>
				</div>
			{/if}

			<!-- Documentation Tab -->
			{#if activeTab === 'documentation'}
				<div
					class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
				>
					<Documentation
						bind:content={product.documentation}
						endpoint="/products"
						id={productId}
						readOnly={!canManage}
					/>
				</div>
			{/if}

			<!-- Assets Tab -->
			{#if activeTab === 'assets'}
				<div
					class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
				>
					<h3
						class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2"
					>
						<IconifyIcon icon="material-symbols:database" class="h-5 w-5" />
						Assets
						{#if resolvedAssets}
							<span class="text-sm font-normal text-gray-500">
								({resolvedAssets.total} total)
							</span>
						{/if}
					</h3>

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
								<a
									href={asset ? getAssetUrl(asset) : '#'}
									class="flex items-center gap-3 p-3 rounded-lg border border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50 hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 transition-all"
								>
									{#if asset}
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
										<span
											class="text-xs px-2 py-0.5 rounded-full {isManual
												? 'bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400'
												: 'bg-purple-100 dark:bg-purple-900/30 text-purple-600 dark:text-purple-400'}"
										>
											{isManual ? 'manual' : 'rule'}
										</span>
									{:else}
										<div class="text-sm text-gray-500">{assetId}</div>
									{/if}
								</a>
							{/each}
						</div>
					{:else}
						<div class="text-center py-8 text-gray-500 dark:text-gray-400">
							<IconifyIcon
								icon="material-symbols:database"
								class="h-12 w-12 mx-auto mb-3 opacity-50"
							/>
							<p>No assets in this data product</p>
						</div>
					{/if}
				</div>
			{/if}

			<!-- Rules Tab -->
			{#if activeTab === 'rules'}
				<div
					class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-6"
				>
					<h3
						class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2"
					>
						<IconifyIcon icon="material-symbols:filter-list" class="h-5 w-5" />
						Dynamic Rules
						{#if product.rules}
							<span class="text-sm font-normal text-gray-500">
								({product.rules.length})
							</span>
						{/if}
					</h3>

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
											<span
												class="text-xs px-2 py-0.5 rounded-full {rule.is_enabled
													? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400'
													: 'bg-gray-100 dark:bg-gray-700 text-gray-500'}"
											>
												{rule.is_enabled ? 'Active' : 'Disabled'}
											</span>
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
					{:else}
						<div class="text-center py-8 text-gray-500 dark:text-gray-400">
							<IconifyIcon
								icon="material-symbols:filter-list"
								class="h-12 w-12 mx-auto mb-3 opacity-50"
							/>
							<p>No rules defined</p>
							<p class="text-sm mt-1">
								Rules automatically include assets matching specific criteria
							</p>
						</div>
					{/if}
				</div>
			{/if}

		</div>
	</div>
{/if}
