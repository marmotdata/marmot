<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount, afterUpdate } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import { page } from '$app/stores';
	import type { Asset } from '$lib/assets/types';
	import type { LineageResponse } from '$lib/lineage/types';
	import Button from './Button.svelte';
	import LineageViewNode from './LineageViewNode.svelte';
	import Icon from './Icon.svelte';
	import RunHistory from './RunHistory.svelte';
	import AssetTags from './AssetTags.svelte';
	import AssetGlossaryTerms from './AssetGlossaryTerms.svelte';
	import AssetDescriptions from './AssetDescriptions.svelte';
	import IconifyIcon from '@iconify/svelte';
	import { auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';

	export let asset: Asset | null = null;
	export let lineage: LineageResponse | null = null;
	export let onClose: () => void;
	export let staticPlacement = false;
	export let collapsed = false;
	export let onToggleCollapse: (() => void) | undefined = undefined;
	export let assetUrl: string | undefined = undefined;
	let currentAssetId: string | null = null;
	let mounted = false;
	let showDeleteModal = false;
	let isDeleting = false;
	let deleteError = '';

	$: canManageAssets = auth.hasPermission('assets', 'manage');

	$: isRunHistoryTab = $page.url.searchParams.get('tab') === 'run-history';
	$: shouldHideRunHistory = staticPlacement && isRunHistoryTab;

	$: if (asset?.id !== currentAssetId) {
		currentAssetId = asset?.id || null;
		lineage = null;
		expandedAssets = new Set<string>();
	}

	$: currentIconName = asset
		? Array.isArray(asset.providers) && asset.providers.length === 1
			? asset.providers[0]
			: asset.type
		: '';

	$: isVisible = asset != null;
	$: fullViewUrl =
		assetUrl || `/assets/${asset?.type.toLowerCase()}/${encodeURIComponent(asset?.name)}`;

	let loadingLineage = false;
	let lineageError: string | null = null;
	let expandedAssets = new Set<string>();

	function formatDate(dateString: string): string {
		return new Date(dateString).toLocaleString();
	}

	function toggleAssetExpansion(id: string) {
		if (expandedAssets.has(id)) {
			expandedAssets.delete(id);
		} else {
			expandedAssets.add(id);
		}
		expandedAssets = expandedAssets;
	}

	function getNodeName(mrn: string): string {
		return mrn.split('/').pop() || mrn;
	}

	async function fetchLineage() {
		if (!asset?.id || loadingLineage) return;

		loadingLineage = true;
		try {
			const response = await fetchApi(`/lineage/assets/${asset.id}?depth=1`);
			if (!response.ok) throw new Error('Failed to fetch lineage');
			lineage = await response.json();
		} catch (error) {
			lineageError = error instanceof Error ? error.message : 'Failed to load lineage';
		} finally {
			loadingLineage = false;
		}
	}
	$: filteredLineage = lineage
		? {
				...lineage,
				nodes: lineage.nodes.filter((node) => !node.asset?.is_stub)
			}
		: null;

	$: hasNonCurrentNodes =
		filteredLineage?.nodes && filteredLineage.nodes.filter((n) => n.depth !== 0).length > 0;

	$: hasDescriptionsOrTerms = asset?.description || asset?.user_description || staticPlacement;

	onMount(() => {
		mounted = true;
		if (asset?.id) {
			fetchLineage();
		}
	});

	afterUpdate(() => {
		if (mounted && asset?.id && !lineage) {
			fetchLineage();
		}
	});

	async function handleDelete() {
		if (!asset?.id) return;

		isDeleting = true;
		deleteError = '';

		try {
			const response = await fetchApi(`/assets/${asset.id}`, {
				method: 'DELETE'
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to delete asset');
			}

			showDeleteModal = false;

			// Close the blade and redirect to assets list
			if (staticPlacement) {
				goto('/assets');
			} else {
				onClose();
				// Trigger a page refresh to update the asset list
				window.location.reload();
			}
		} catch (err) {
			deleteError = err instanceof Error ? err.message : 'Failed to delete asset';
		} finally {
			isDeleting = false;
		}
	}
</script>

{#if isVisible && asset}
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
			<a
				href="#"
				onclick={(e) => {
					e.preventDefault();
					onToggleCollapse();
				}}
				class="flex-shrink-0 w-8 flex items-center justify-center transition-colors hover:bg-gray-100 dark:hover:bg-gray-800"
				aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
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
			</a>
		{/if}

		{#if !collapsed}
		<div class="flex-1 flex flex-col min-w-0">
		<div
			class="flex-none bg-earthy-brown-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700 px-6 py-4 flex justify-between items-center"
		>
			<h2 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Asset Details</h2>
			{#if !staticPlacement}
				<div class="flex items-center space-x-4">
					<Button
						icon="material-symbols:screenshot-monitor-outline"
						href={fullViewUrl}
						text="Full View"
						variant="filled"
					/>
					<Button click={onClose} variant="clear" icon="material-symbols:close" />
				</div>
			{/if}
		</div>

		<div class="flex-1 overflow-y-auto min-h-0 {staticPlacement ? 'pr-6 py-6' : 'p-6'}">
			<div class="space-y-4">
				<!-- Asset Header -->
				{#if staticPlacement}
					<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5">
						<div class="flex items-start gap-3 mb-3">
							<div class="flex-shrink-0">
								<Icon name={currentIconName} iconSize="md" />
							</div>
							<div class="flex-1 min-w-0">
								<h3 class="font-semibold text-base text-gray-900 dark:text-gray-100 truncate">
									{asset.name || ''}
								</h3>
								<p class="text-xs text-gray-500 dark:text-gray-400 truncate font-mono mt-0.5">
									{asset.mrn || ''}
								</p>
							</div>
						</div>
						<AssetTags {asset} editable={staticPlacement} />
					</div>
				{:else}
					<a href={`${fullViewUrl}`} class="block bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5 hover:shadow-md hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 transition-all">
						<div class="flex items-start gap-3 mb-3">
							<div class="flex-shrink-0">
								<Icon name={currentIconName} iconSize="md" />
							</div>
							<div class="flex-1 min-w-0">
								<h3 class="font-semibold text-base text-gray-900 dark:text-gray-100 truncate">
									{asset.name || ''}
								</h3>
								<p class="text-xs text-gray-500 dark:text-gray-400 truncate font-mono mt-0.5">
									{asset.mrn || ''}
								</p>
							</div>
						</div>
						<AssetTags {asset} editable={false} />
					</a>
				{/if}

				<!-- Descriptions and Glossary Terms -->
				{#if hasDescriptionsOrTerms}
					<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5">
						<div class="space-y-5">
							<AssetDescriptions {asset} editable={staticPlacement} />
							<AssetGlossaryTerms {asset} editable={staticPlacement} />
						</div>
					</div>
				{/if}

				<!-- Run History -->
				{#if !shouldHideRunHistory && asset.has_run_history}
					<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5">
						<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-3">
							Run History
						</h3>
						<RunHistory assetId={asset.id} minimal={true} {asset} />
					</div>
				{/if}

				<!-- Data Lineage -->
				{#if hasNonCurrentNodes}
					<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5">
						<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-3">
							Data Lineage
						</h3>

						{#if loadingLineage}
							<div class="flex items-center justify-center py-8">
								<div class="animate-spin h-6 w-6 border-b-2 border-earthy-terracotta-700 rounded-full"></div>
							</div>
						{:else if lineageError}
							<div class="text-sm text-red-600 dark:text-red-400">{lineageError}</div>
						{:else if lineage}
							{#if lineage.nodes.filter((n) => n.depth < 0).length > 0}
								<div class="mb-4">
									<h4 class="text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wider mb-2">
										Upstream
									</h4>
									<div class="space-y-2">
										{#each filteredLineage.nodes.filter((n) => n.depth < 0) as node}
											<LineageViewNode
												{node}
												expanded={expandedAssets.has(node.id)}
												onClick={() => toggleAssetExpansion(node.id)}
												maxMetadataDepth={0}
												compact={true}
											/>
										{/each}
									</div>
								</div>
							{/if}

							{#if lineage.nodes.filter((n) => n.depth > 0).length > 0}
								<div>
									<h4 class="text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wider mb-2">
										Downstream
									</h4>
									<div class="space-y-2">
										{#each filteredLineage.nodes.filter((n) => n.depth > 0) as node}
											<LineageViewNode
												{node}
												expanded={expandedAssets.has(node.id)}
												onClick={() => toggleAssetExpansion(node.id)}
												maxMetadataDepth={0}
												compact={true}
											/>
										{/each}
									</div>
								</div>
							{/if}
						{/if}
					</div>
				{/if}

				<!-- Additional Details -->
				<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5">
					<h4 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-3">
						Details
					</h4>
					<dl class="space-y-3">
						<div>
							<dt class="text-xs text-gray-500 dark:text-gray-400">Created By</dt>
							<dd class="text-sm text-gray-900 dark:text-gray-100 mt-0.5">
								{asset.created_by || 'Unknown'}
							</dd>
						</div>
						<div>
							<dt class="text-xs text-gray-500 dark:text-gray-400">Created At</dt>
							<dd class="text-sm text-gray-900 dark:text-gray-100 mt-0.5">
								{asset.created_at ? formatDate(asset.created_at) : 'Unknown'}
							</dd>
						</div>
						<div>
							<dt class="text-xs text-gray-500 dark:text-gray-400">Last Updated</dt>
							<dd class="text-sm text-gray-900 dark:text-gray-100 mt-0.5">
								{asset.updated_at ? formatDate(asset.updated_at) : 'Unknown'}
							</dd>
						</div>
						{#if asset.parent_mrn}
							<div>
								<dt class="text-xs text-gray-500 dark:text-gray-400">Parent Asset</dt>
								<dd class="text-sm text-gray-900 dark:text-gray-100 mt-0.5 font-mono text-xs break-all">
									{asset.parent_mrn}
								</dd>
							</div>
						{/if}
					</dl>
				</div>
			</div>
		</div>

		<!-- Delete Button Footer -->
		{#if canManageAssets}
			<div class="flex-none border-t border-gray-200 dark:border-gray-700 bg-earthy-brown-50 dark:bg-gray-900 px-6 py-4 flex justify-end">
				<button
					onclick={() => (showDeleteModal = true)}
					class="inline-flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-950/30 rounded-lg transition-colors"
				>
					<IconifyIcon icon="material-symbols:delete-outline-rounded" class="w-4 h-4" />
					Delete Asset
				</button>
			</div>
		{/if}
		</div>
		{/if}
	</div>
{/if}

<!-- Delete Confirmation Modal -->
{#if showDeleteModal && asset}
	<div
		class="fixed inset-0 bg-black/50 dark:bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center px-4"
		onclick={() => !isDeleting && (showDeleteModal = false)}
		role="button"
		tabindex="-1"
	>
		<div
			class="bg-white dark:bg-gray-800 rounded-xl shadow-2xl max-w-md w-full border border-gray-200 dark:border-gray-700"
			onclick={(e) => e.stopPropagation()}
			role="dialog"
			tabindex="-1"
		>
			<div class="p-6">
				<div class="flex items-start gap-4">
					<div class="flex-shrink-0">
						<IconifyIcon icon="material-symbols:warning-rounded" class="w-12 h-12 text-red-600 dark:text-red-400" />
					</div>
					<div class="flex-1">
						<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-2">
							Delete Asset
						</h3>
						<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
							Are you sure you want to delete <span class="font-semibold text-gray-900 dark:text-gray-100">"{asset.name}"</span>? This action cannot be undone.
						</p>
						{#if deleteError}
							<div class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
								<p class="text-sm text-red-800 dark:text-red-200">{deleteError}</p>
							</div>
						{/if}
						<div class="flex gap-3 justify-end">
							<button
								onclick={() => (showDeleteModal = false)}
								disabled={isDeleting}
								class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
							>
								Cancel
							</button>
							<button
								onclick={handleDelete}
								disabled={isDeleting}
								class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-red-600 hover:bg-red-700 dark:bg-red-500 dark:hover:bg-red-600 rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
							>
								{#if isDeleting}
									<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white"></div>
									Deleting...
								{:else}
									<IconifyIcon icon="material-symbols:delete-outline" class="w-5 h-5" />
									Delete Asset
								{/if}
							</button>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
{/if}
