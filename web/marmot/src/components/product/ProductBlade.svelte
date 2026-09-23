<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount, afterUpdate } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { SvelteMap } from 'svelte/reactivity';
	import type { DataProduct, ResolvedAssetsResponse } from '$lib/dataproducts/types';
	import type { Asset } from '$lib/assets/types';
	import Button from '$components/ui/Button.svelte';
	import OwnerSelector from '$components/shared/OwnerSelector.svelte';
	import AssetIcon from '$components/ui/Icon.svelte';
	import IconifyIcon from '@iconify/svelte';
	import AuthenticatedImage from '$components/ui/AuthenticatedImage.svelte';
	import ProductGovernedFieldsSummary from './ProductGovernedFieldsSummary.svelte';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import { formatDateTime } from '$lib/utils';

	export let product: DataProduct | null = null;
	export let onClose: () => void;
	export let staticPlacement = false;
	export let collapsed = false;
	export let onToggleCollapse: (() => void) | undefined = undefined;

	let currentProductId: string | null = null;
	let mounted = false;
	let showDeleteModal = false;
	let isDeleting = false;
	let deleteError = '';

	// Asset preview
	let resolvedAssets: ResolvedAssetsResponse | null = null;
	let assetDetails = new SvelteMap<string, Asset>();
	let loadingAssets = false;

	const canManage = auth.hasPermission('assets', 'manage');
	$: isVisible = product != null;
	$: fullViewUrl = product ? `/products/${product.id}` : '';

	function syncProductChange(productId: string | null) {
		if (productId === currentProductId) return;
		currentProductId = productId;
		resolvedAssets = null;
		assetDetails = new SvelteMap();
		if (productId) {
			fetchResolvedAssets();
		}
	}

	$: syncProductChange(product?.id ?? null);

	async function fetchResolvedAssets() {
		if (!product?.id || loadingAssets) return;

		loadingAssets = true;
		try {
			const response = await fetchApi(`/products/resolved-assets/${product.id}?limit=10`);
			if (response.ok) {
				resolvedAssets = await response.json();
				// Load first 5 asset details for preview
				if (resolvedAssets && resolvedAssets.all_assets.length > 0) {
					await loadAssetDetails(resolvedAssets.all_assets.slice(0, 5));
				}
			}
		} catch (error) {
			console.error('Failed to load resolved assets:', error);
		} finally {
			loadingAssets = false;
		}
	}

	async function loadAssetDetails(assetIds: string[]) {
		const newDetails = new SvelteMap<string, Asset>();
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

	onMount(() => {
		mounted = true;
		if (product?.id) {
			fetchResolvedAssets();
		}
	});

	afterUpdate(() => {
		if (mounted && product?.id && !resolvedAssets && !loadingAssets) {
			fetchResolvedAssets();
		}
	});

	async function handleDelete() {
		if (!product?.id) return;

		isDeleting = true;
		deleteError = '';

		try {
			const response = await fetchApi(`/products/${product.id}`, {
				method: 'DELETE'
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || m.products_error_delete());
			}

			showDeleteModal = false;

			if (staticPlacement) {
				goto(resolve('/products'));
			} else {
				onClose();
				window.location.reload();
			}
		} catch (err) {
			deleteError = err instanceof Error ? err.message : m.products_error_delete();
		} finally {
			isDeleting = false;
		}
	}
</script>

{#if isVisible && product}
	{#if !staticPlacement}
		<div
			role="button"
			tabindex="0"
			class="fixed inset-0 bg-black bg-opacity-30 z-40"
			onclick={onClose}
			onkeydown={(e) => e.key === 'Enter' && onClose()}
			transition:fade={{ duration: 200 }}
		></div>
	{/if}

	<div
		class={staticPlacement
			? 'h-full w-full bg-earthy-brown-50 dark:bg-gray-900 flex'
			: 'fixed right-0 top-0 h-full w-full max-w-2xl bg-earthy-brown-50 dark:bg-gray-900 shadow-lg dark:shadow-2xl z-50 flex flex-col'}
		transition:fly={{ x: staticPlacement ? 0 : 400, duration: staticPlacement ? 0 : 200 }}
	>
		{#if staticPlacement && onToggleCollapse}
			<button
				type="button"
				onclick={onToggleCollapse}
				class="flex-shrink-0 w-8 flex items-center justify-center transition-colors hover:bg-gray-100 dark:hover:bg-gray-800"
				aria-label={collapsed
					? m.asset_blade_expand_sidebar_aria()
					: m.asset_blade_collapse_sidebar_aria()}
			>
				<svg
					class="w-4 h-4 text-gray-600 dark:text-gray-400 transition-transform {collapsed
						? 'rotate-180'
						: ''}"
					fill="none"
					stroke="currentColor"
					viewBox="0 0 24 24"
				>
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
				</svg>
			</button>
		{/if}

		{#if !collapsed}
			<div class="flex-1 flex flex-col min-w-0 min-h-0">
				<div
					class="flex-none bg-earthy-brown-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700 px-6 py-4 flex justify-between items-center"
				>
					<h2 class="text-2xl font-bold text-gray-900 dark:text-gray-100">
						{m.products_blade_details_heading()}
					</h2>
					{#if !staticPlacement}
						<div class="flex items-center space-x-4">
							<Button
								icon="material-symbols:screenshot-monitor-outline"
								href={fullViewUrl}
								text={m.asset_blade_full_view()}
								variant="filled"
							/>
							<button
								type="button"
								onclick={onClose}
								class="p-2 text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
							>
								<IconifyIcon icon="material-symbols:close" class="w-5 h-5" />
							</button>
						</div>
					{/if}
				</div>

				<div class="flex-1 overflow-y-auto min-h-0 {staticPlacement ? 'pr-6 py-6' : 'p-6'}">
					<div class="space-y-4">
						<!-- Product Header -->
						{#if !staticPlacement}
							<a
								href={fullViewUrl ? resolve(fullViewUrl) : '#'}
								class="block bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5 hover:shadow-md hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 transition-all"
							>
								<div class="flex items-start gap-3 mb-3">
									<div class="flex-shrink-0">
										<div
											class="w-10 h-10 rounded-lg bg-earthy-terracotta-100 dark:bg-earthy-terracotta-900/30 flex items-center justify-center overflow-hidden"
										>
											{#if product.icon_url}
												<AuthenticatedImage
													src={product.icon_url}
													alt={m.products_icon_alt({ name: product.name })}
													class="w-full h-full object-cover"
												/>
											{:else}
												<IconifyIcon
													icon="mdi:package-variant-closed"
													class="w-5 h-5 text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
												/>
											{/if}
										</div>
									</div>
									<div class="flex-1 min-w-0">
										<h3 class="font-semibold text-base text-gray-900 dark:text-gray-100 truncate">
											{product.name || ''}
										</h3>
										{#if product.description}
											<p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5 line-clamp-2">
												{product.description}
											</p>
										{/if}
									</div>
								</div>
								<div class="space-y-4">
									<div>
										<div class="flex items-center gap-2 mb-2">
											<IconifyIcon
												icon="material-symbols:label-outline"
												class="w-4 h-4 text-gray-500 dark:text-gray-400"
											/>
											<h4
												class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider"
											>
												{m.common_tags()}
											</h4>
										</div>
										<div class="flex flex-wrap gap-1">
											{#if product.tags && product.tags.length > 0}
												{#each product.tags.slice(0, 5) as tag (tag)}
													<span
														class="inline-flex items-center gap-0.5 text-xs bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 px-1.5 py-0.5 rounded"
													>
														{tag}
													</span>
												{/each}
												{#if product.tags.length > 5}
													<span class="text-xs text-gray-500">+{product.tags.length - 5}</span>
												{/if}
											{:else}
												<span class="text-xs text-gray-400 dark:text-gray-500 italic"
													>{m.products_no_tags()}</span
												>
											{/if}
										</div>
									</div>
									<div>
										<div class="flex items-center gap-2 mb-2">
											<IconifyIcon
												icon="material-symbols:person-outline"
												class="w-4 h-4 text-gray-500 dark:text-gray-400"
											/>
											<h4
												class="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider"
											>
												{m.common_owners()}
											</h4>
										</div>
										<OwnerSelector
											selectedOwners={product.owners || []}
											onChange={() => {}}
											disabled={true}
										/>
									</div>
								</div>
							</a>
						{/if}

						<!-- Governed metadata fields: only in the popup from Discover. In static mode the
						     full product page already has its own Metadatos tab for this. -->
						{#if !staticPlacement}
							<ProductGovernedFieldsSummary {product} />
						{/if}

						<!-- Description Section - only show in non-static mode -->
						{#if !staticPlacement && product.description}
							<div
								class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5"
							>
								<div class="flex items-center justify-between mb-2">
									<div class="flex items-center gap-2">
										<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">
											{m.common_description()}
										</h3>
									</div>
								</div>
								<p
									class="text-sm text-gray-600 dark:text-gray-400 whitespace-pre-wrap leading-relaxed"
								>
									{product.description}
								</p>
							</div>
						{/if}

						<!-- Assets Preview -->
						{#if resolvedAssets && resolvedAssets.all_assets.length > 0}
							<div
								class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5"
							>
								<div class="flex items-center justify-between mb-3">
									<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">
										{m.products_assets()}
									</h3>
									<span class="text-xs text-gray-500 dark:text-gray-400">
										{m.products_total_count({ count: resolvedAssets.total })}
									</span>
								</div>

								{#if loadingAssets}
									<div class="flex items-center justify-center py-4">
										<div
											class="animate-spin h-5 w-5 border-b-2 border-earthy-terracotta-700 rounded-full"
										></div>
									</div>
								{:else}
									<div class="space-y-2">
										{#each resolvedAssets.all_assets.slice(0, 5) as assetId (assetId)}
											{@const asset = assetDetails.get(assetId)}
											{#if asset}
												{@const assetUrl = getAssetUrl(asset)}
												<a
													href={assetUrl === '#' ? '#' : resolve(assetUrl)}
													class="flex items-center gap-2 p-2 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
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
												<div class="p-2 text-xs text-gray-500">{assetId}</div>
											{/if}
										{/each}
									</div>
									{#if resolvedAssets.total > 5}
										<a
											href={resolve(`${fullViewUrl}?tab=assets`)}
											class="mt-3 inline-flex items-center gap-1 text-xs text-earthy-terracotta-600 hover:text-earthy-terracotta-700 dark:text-earthy-terracotta-400"
										>
											{m.products_view_all_assets({ count: resolvedAssets.total })}
											<IconifyIcon icon="material-symbols:arrow-forward" class="w-3 h-3" />
										</a>
									{/if}
								{/if}
							</div>
						{/if}

						<!-- Rules Preview -->
						{#if product.rules && product.rules.length > 0}
							<div
								class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5"
							>
								<div class="flex items-center justify-between mb-3">
									<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100">
										{m.products_tab_rules()}
									</h3>
									<span class="text-xs text-gray-500 dark:text-gray-400">
										{m.products_rule_count({ count: product.rules.length })}
									</span>
								</div>
								<div class="space-y-2">
									{#each product.rules.slice(0, 3) as rule (rule.name)}
										<div
											class="p-2 rounded-lg bg-gray-50 dark:bg-gray-700/50 flex items-center justify-between"
										>
											<div class="flex items-center gap-2">
												<IconifyIcon icon="mdi:database-search" class="w-4 h-4 text-blue-500" />
												<span class="text-sm text-gray-900 dark:text-gray-100">
													{rule.name}
												</span>
											</div>
											<span
												class="text-xs px-2 py-0.5 rounded-full {rule.is_enabled
													? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400'
													: 'bg-gray-100 dark:bg-gray-600 text-gray-500'}"
											>
												{rule.is_enabled ? m.common_active() : m.common_disabled()}
											</span>
										</div>
									{/each}
								</div>
								{#if product.rules.length > 3}
									<a
										href={resolve(`${fullViewUrl}?tab=rules`)}
										class="mt-3 inline-flex items-center gap-1 text-xs text-earthy-terracotta-600 hover:text-earthy-terracotta-700 dark:text-earthy-terracotta-400"
									>
										{m.products_view_all_rules({ count: product.rules.length })}
										<IconifyIcon icon="material-symbols:arrow-forward" class="w-3 h-3" />
									</a>
								{/if}
							</div>
						{/if}

						<!-- Additional Details -->
						<div
							class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5"
						>
							<h4 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-3">
								{m.common_details()}
							</h4>
							<dl class="space-y-3">
								{#if product.created_by}
									<div>
										<dt class="text-xs text-gray-500 dark:text-gray-400">{m.asset_created_by()}</dt>
										<dd class="text-sm text-gray-900 dark:text-gray-100 mt-0.5">
											{product.created_by}
										</dd>
									</div>
								{/if}
								<div>
									<dt class="text-xs text-gray-500 dark:text-gray-400">{m.asset_created_at()}</dt>
									<dd class="text-sm text-gray-900 dark:text-gray-100 mt-0.5">
										{product.created_at ? formatDateTime(product.created_at) : m.common_unknown()}
									</dd>
								</div>
								<div>
									<dt class="text-xs text-gray-500 dark:text-gray-400">{m.asset_last_updated()}</dt>
									<dd class="text-sm text-gray-900 dark:text-gray-100 mt-0.5">
										{product.updated_at ? formatDateTime(product.updated_at) : m.common_unknown()}
									</dd>
								</div>
								<div>
									<dt class="text-xs text-gray-500 dark:text-gray-400">{m.products_assets()}</dt>
									<dd class="text-sm text-gray-900 dark:text-gray-100 mt-0.5">
										{m.products_total_count({ count: product.asset_count || 0 })}
										{#if product.manual_asset_count || product.rule_asset_count}
											<span class="text-xs text-gray-500">
												{m.products_asset_breakdown({
													manual: product.manual_asset_count || 0,
													rules: product.rule_asset_count || 0
												})}
											</span>
										{/if}
									</dd>
								</div>
							</dl>
						</div>
					</div>
				</div>

				<!-- Delete Button Footer -->
				{#if canManage}
					<div
						class="flex-none border-t border-gray-200 dark:border-gray-700 bg-earthy-brown-50 dark:bg-gray-900 px-6 py-4 flex justify-end"
					>
						<button
							onclick={() => (showDeleteModal = true)}
							class="inline-flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-950/30 rounded-lg transition-colors"
						>
							<IconifyIcon icon="material-symbols:delete-outline-rounded" class="w-4 h-4" />
							{m.products_delete_product()}
						</button>
					</div>
				{/if}
			</div>
		{/if}
	</div>
{/if}

<!-- Delete Confirmation Modal -->
{#if showDeleteModal && product}
	<div
		class="fixed inset-0 bg-black/50 dark:bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center px-4"
		onclick={() => !isDeleting && (showDeleteModal = false)}
		onkeydown={(e) => e.key === 'Escape' && !isDeleting && (showDeleteModal = false)}
		role="button"
		tabindex="-1"
	>
		<div
			class="bg-white dark:bg-gray-800 rounded-xl shadow-2xl max-w-md w-full border border-gray-200 dark:border-gray-700"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => e.stopPropagation()}
			role="dialog"
			tabindex="-1"
		>
			<div class="p-6">
				<div class="flex items-start gap-4">
					<div class="flex-shrink-0">
						<IconifyIcon
							icon="material-symbols:warning-rounded"
							class="w-12 h-12 text-red-600 dark:text-red-400"
						/>
					</div>
					<div class="flex-1">
						<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-2">
							{m.products_delete_title()}
						</h3>
						<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
							{m.products_delete_confirm_message({ name: product.name })}
						</p>
						{#if deleteError}
							<div
								class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg"
							>
								<p class="text-sm text-red-800 dark:text-red-200">{deleteError}</p>
							</div>
						{/if}
						<div class="flex gap-3 justify-end">
							<button
								onclick={() => (showDeleteModal = false)}
								disabled={isDeleting}
								class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
							>
								{m.common_cancel()}
							</button>
							<button
								onclick={handleDelete}
								disabled={isDeleting}
								class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-red-600 hover:bg-red-700 dark:bg-red-500 dark:hover:bg-red-600 rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
							>
								{#if isDeleting}
									<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white"></div>
									{m.products_deleting()}
								{:else}
									<IconifyIcon icon="material-symbols:delete-outline" class="w-5 h-5" />
									{m.products_delete_product()}
								{/if}
							</button>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
{/if}
