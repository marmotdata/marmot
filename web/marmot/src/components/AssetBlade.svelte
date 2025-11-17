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
	import AssetTagsGlossary from './AssetTagsGlossary.svelte';
	import AssetDescriptions from './AssetDescriptions.svelte';

	export let asset: Asset | null = null;
	export let lineage: LineageResponse | null = null;
	export let onClose: () => void;
	export let staticPlacement = false;
	export let collapsed = false;
	export let onToggleCollapse: (() => void) | undefined = undefined;
	export let assetUrl: string | undefined = undefined;
	let currentAssetId: string | null = null;
	let mounted = false;

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

		<div class="flex-1 overflow-y-auto min-h-0">
			<div class="divide-y divide-gray-200 dark:divide-gray-700">
				<!-- Asset Header -->
				{#if staticPlacement}
					<div class="p-4">
						<div class="flex items-start space-x-3">
							<div class="flex-shrink-0 mt-0.5">
								<Icon name={currentIconName} iconSize="md" />
							</div>
							<div class="flex-1 min-w-0">
								<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 truncate">
									{asset.name || ''}
								</h3>
								<p class="text-xs text-gray-500 dark:text-gray-400 truncate mt-0.5">
									{asset.mrn || ''}
								</p>
							</div>
						</div>
					</div>
				{:else}
					<a href={`${fullViewUrl}`} class="block p-4 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors">
						<div class="flex items-start space-x-3">
							<div class="flex-shrink-0 mt-0.5">
								<Icon name={currentIconName} iconSize="md" />
							</div>
							<div class="flex-1 min-w-0">
								<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 truncate">
									{asset.name || ''}
								</h3>
								<p class="text-xs text-gray-500 dark:text-gray-400 truncate mt-0.5">
									{asset.mrn || ''}
								</p>
							</div>
						</div>
					</a>
				{/if}

				<!-- Descriptions -->
				<div class="p-5">
					<AssetDescriptions {asset} editable={staticPlacement} />
				</div>

				<!-- Tags and Glossary Terms -->
				<div class="p-5">
					<AssetTagsGlossary {asset} editable={staticPlacement} />
				</div>

				<!-- Run History -->
				{#if !shouldHideRunHistory && asset.has_run_history}
					<div class="p-5">
						<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-3">
							Run History
						</h3>
						<RunHistory assetId={asset.id} minimal={true} {asset} />
					</div>
				{/if}

				<!-- Data Lineage -->
				{#if hasNonCurrentNodes}
					<div class="p-5">
						<h3 class="text-base font-semibold text-gray-900 dark:text-gray-100 mb-3">
							Data Lineage
						</h3>

						{#if loadingLineage}
							<div class="flex items-center justify-center py-8">
								<div class="animate-spin h-6 w-6 border-b-2 border-orange-600 rounded-full"></div>
							</div>
						{:else if lineageError}
							<div class="text-sm text-red-600 dark:text-red-400">{lineageError}</div>
						{:else if lineage}
							{#if lineage.nodes.filter((n) => n.depth < 0).length > 0}
								<div class="mb-4">
									<h4 class="text-sm font-medium text-gray-600 dark:text-gray-400 uppercase tracking-wide mb-2">
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
									<h4 class="text-sm font-medium text-gray-600 dark:text-gray-400 uppercase tracking-wide mb-2">
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
				<div class="p-4">
					<h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-3">
						Details
					</h4>
					<dl class="space-y-2">
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
		</div>
		{/if}
	</div>
{/if}
