<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount, afterUpdate } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import type { Asset } from '$lib/assets/types';
	import type { LineageResponse } from '$lib/lineage/types';
	import Button from './Button.svelte';
	import LineageViewNode from './LineageViewNode.svelte';
	import Icon from './Icon.svelte';

	export let asset: Asset | null = null;
	export let lineage: LineageResponse | null = null;
	export let onClose: () => void;
	export let staticPlacement = false;
	export let assetUrl: string | undefined = undefined;
	let currentAssetId: string | null = null;
	let mounted = false;

	// Reset state when asset changes
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
			const response = await fetchApi(`/lineage/assets/${asset.id}`);
			if (!response.ok) throw new Error('Failed to fetch lineage');
			lineage = await response.json();
		} catch (error) {
			lineageError = error instanceof Error ? error.message : 'Failed to load lineage';
		} finally {
			loadingLineage = false;
		}
	}

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
			role="link"
			class="fixed inset-0 bg-black bg-opacity-30 z-40"
			on:click={onClose}
			transition:fade={{ duration: 200 }}
		></div>
	{/if}

	<div
		class={staticPlacement
			? 'h-full w-full bg-earthy-brown-50 dark:bg-gray-900 flex flex-col'
			: 'fixed right-0 top-0 h-full w-full max-w-2xl bg-earthy-brown-50 dark:bg-gray-900 shadow-lg dark:shadow-2xl z-50 flex flex-col'}
		transition:fly={{ x: staticPlacement ? 0 : 400, duration: staticPlacement ? 0 : 200 }}
	>
		<!-- Header -->
		<div
			class="flex-none bg-earthy-brown-50 dark:bg-gray-900 border-b border-gray-200 dark:border-gray-700 px-6 py-4 flex justify-between items-center"
		>
			<h2 class="text-2xl font-bold text-gray-900 dark:text-gray-100">Asset Details</h2>
			{#if !staticPlacement}
				<div class="flex items-center space-x-4">
					<Button icon="aspect-ratio" href={fullViewUrl} text="Full View" variant="filled" />
					<Button click={onClose} icon="x-lg" variant="clear" />
				</div>
			{/if}
		</div>

		<!-- Scrollable content -->
		<div class="flex-1 overflow-y-auto min-h-0 pb-16">
			<div class="px-6 py-4 space-y-6">
				<!-- Asset Overview Card -->
				{#if staticPlacement}
					<div
						class="bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700 p-4"
					>
						<div class="flex items-center space-x-4">
							<div class="flex-shrink-0">
								<Icon name={currentIconName} iconSize="lg" />
							</div>
							<div class="flex-1 min-w-0">
								<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 truncate">
									{asset.name || ''}
								</h3>
								<p class="text-sm text-gray-500 dark:text-gray-400 truncate">{asset.mrn || ''}</p>
								{#if asset.tags?.length > 0}
									<div class="mt-2 flex flex-wrap gap-2">
										{#each asset.tags as tag}
											<span
												class="px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200"
											>
												{tag}
											</span>
										{/each}
									</div>
								{/if}
							</div>
						</div>
					</div>
				{:else}
					<a href={`${fullViewUrl}`} class="block">
						<div
							class="bg-earthy-brown-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700 p-4 hover:border-orange-400 transition-colors"
						>
							<div class="flex items-center space-x-4">
								<div class="flex-shrink-0">
									<Icon name={currentIconName} iconSize="lg" />
								</div>
								<div class="flex-1 min-w-0">
									<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100 truncate">
										{asset.name || ''}
									</h3>
									<p class="text-sm text-gray-500 dark:text-gray-400 truncate">{asset.mrn || ''}</p>
									{#if asset.tags?.length > 0}
										<div class="mt-2 flex flex-wrap gap-2">
											{#each asset.tags as tag}
												<span
													class="px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200"
												>
													{tag}
												</span>
											{/each}
										</div>
									{/if}
								</div>
							</div>
						</div>
					</a>
				{/if}

				{#if asset.description}
					<div class="bg-earthy-brown-50 dark:bg-gray-900 p-4">
						<h4 class="text-sm font-medium text-gray-900 dark:text-gray-100 mb-2">Description</h4>
						<p class="text-gray-600 dark:text-gray-300">{asset.description}</p>
					</div>
				{/if}

				<!-- Lineage Section -->
				<div class="border-t border-gray-200 dark:border-gray-700 pt-4">
					<h3 class="text-xl font-bold text-gray-900 dark:text-gray-100 mb-4">Data Lineage</h3>

					{#if loadingLineage}
						<div class="flex items-center justify-center p-8">
							<div class="animate-spin h-8 w-8 border-b-2 border-orange-600 rounded-full" />
						</div>
					{:else if lineageError}
						<div class="p-4 text-red-600 dark:text-red-400">{lineageError}</div>
					{:else if lineage}
						<!-- Upstream Assets -->
						{#if lineage.nodes.filter((n) => n.depth < 0).length > 0}
							<div class="mt-6">
								<h4 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-3">
									Upstream Assets
								</h4>
								<div class="space-y-3">
									{#each lineage.nodes.filter((n) => n.depth < 0) as node}
										<LineageViewNode
											{node}
											expanded={expandedAssets.has(node.id)}
											onClick={() => toggleAssetExpansion(node.id)}
											maxMetadataDepth={1}
										/>
									{/each}
								</div>
							</div>
						{/if}

						<!-- Downstream Assets -->
						{#if lineage.nodes.filter((n) => n.depth > 0).length > 0}
							<div class="mt-6">
								<h4 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-3">
									Downstream Assets
								</h4>
								<div class="space-y-3">
									{#each lineage.nodes.filter((n) => n.depth > 0) as node}
										<LineageViewNode
											{node}
											expanded={expandedAssets.has(node.id)}
											onClick={() => toggleAssetExpansion(node.id)}
											maxMetadataDepth={1}
										/>
									{/each}
								</div>
							</div>
						{/if}
					{/if}
				</div>

				<!-- Asset Details Section -->
				<div class="border-t border-gray-200 dark:border-gray-700 pt-4">
					<h4 class="text-lg font-semibold text-gray-800 dark:text-gray-200 mb-3">
						Additional Details
					</h4>
					<dl class="grid grid-cols-2 gap-4">
						<div>
							<dt class="text-sm text-gray-500 dark:text-gray-400">Created By</dt>
							<dd class="text-sm font-medium text-gray-900 dark:text-gray-100">
								{asset.created_by || ''}
							</dd>
						</div>
						<div>
							<dt class="text-sm text-gray-500 dark:text-gray-400">Created At</dt>
							<dd class="text-sm font-medium text-gray-900 dark:text-gray-100">
								{asset.created_at ? formatDate(asset.created_at) : ''}
							</dd>
						</div>
						<div>
							<dt class="text-sm text-gray-500 dark:text-gray-400">Last Updated</dt>
							<dd class="text-sm font-medium text-gray-900 dark:text-gray-100">
								{asset.updated_at ? formatDate(asset.updated_at) : ''}
							</dd>
						</div>
						{#if asset.parent_mrn}
							<div class="col-span-2">
								<dt class="text-sm text-gray-500 dark:text-gray-400">Parent Asset</dt>
								<dd class="text-sm font-medium text-gray-900 dark:text-gray-100">
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