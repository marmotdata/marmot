<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { fetchApi } from '$lib/api';
	import type { Asset } from '$lib/assets/types';
	import AssetBlade from '$lib/../components/AssetBlade.svelte';
	import Button from '$lib/../components/Button.svelte';
	import AssetDocumentation from '$lib/../components/AssetDocumentation.svelte';
	import AssetSources from '$lib/../components/AssetSources.svelte';
	import MetadataView from '$lib/../components/MetadataView.svelte';
	import Lineage from '$lib/../components/Lineage.svelte';
	import SchemaSummary from '$lib/../components/SchemaSummary.svelte';
	import AssetEnvironmentsView from '$lib/../components/AssetEnvironmentsView.svelte';
	import QueryPreview from '$lib/../components/QueryPreview.svelte';
	import RunHistory from '$lib/../components/RunHistory.svelte';

	let asset: Asset | null = $state(null);
	let loading = $state(true);
	let error: string | null = $state(null);
	let bladeCollapsed = $state(false);

	let activeTab = $derived($page.url.searchParams.get('tab') || 'metadata');
	let assetType = $derived($page.params.type);
	let assetName = $derived($page.params.name);

	async function fetchAsset() {
		try {
			loading = true;
			error = null;
			const response = await fetchApi(
				`/assets/lookup/${assetType}/${encodeURIComponent(assetName)}`
			);
			if (!response.ok) {
				throw new Error('Failed to fetch asset');
			}
			asset = await response.json();
		} catch (err) {
			console.error('Error fetching asset:', err);
			error = err instanceof Error ? err.message : 'Failed to load asset';
		} finally {
			loading = false;
		}
	}

	function setActiveTab(tab: string) {
		const url = new URL(window.location.href);
		url.searchParams.set('tab', tab);
		goto(url.toString(), { replaceState: true });
	}

	function handleBack() {
		goto('/assets');
	}

	let visibleTabs = $derived(
		[
			'metadata',
			'query',
			'environments',
			'schema',
			'documentation',
			'run-history',
			'lineage'
		].filter((tab) => {
			if (
				tab === 'environments' &&
				(!asset?.environments || Object.keys(asset.environments).length === 0)
			)
				return false;
			if (tab === 'documentation' && !asset?.documentation) return false;
			if (tab === 'query' && (!asset?.query || !asset.query.trim())) return false;
			if (tab === 'run-history' && !asset?.has_run_history) return false;
			return true;
		})
	);

	$effect(() => {
		if (assetType && assetName) {
			fetchAsset();
		}
	});
</script>

<div class="h-full flex">
	{#if loading}
		<div class="flex items-center justify-center w-full">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-orange-600"></div>
		</div>
	{:else if error}
		<div class="flex items-center justify-center w-full">
			<div
				class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/50 rounded-lg p-4 text-red-600 dark:text-red-400"
			>
				{error}
			</div>
		</div>
	{:else}
		<div class="flex-1 flex flex-col min-w-0">
			<div class="flex-none p-8">
				<div class="mb-6">
					<button
						on:click={handleBack}
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
						Back to Assets
					</button>
				</div>

				{#if asset}
					<div class="mb-6">
						<div class="flex flex-col gap-4">
							<div>
								<h1 class="text-2xl font-semibold text-gray-900 dark:text-gray-100">
									{asset.name}
								</h1>
								<p class="text-sm text-gray-500 dark:text-gray-400 mt-1">{asset.mrn}</p>
							</div>
							<div class="flex gap-2">
								{#if asset.external_links}
									{#each asset.external_links as link}
										<Button
											icon={link.icon}
											text={link.name}
											variant="clear"
											href={link.url}
											target="_blank"
											rel="noopener noreferrer"
											class="bg-gray-50 dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700"
										/>
									{/each}
								{/if}
							</div>
						</div>
					</div>

					<div class="border-b border-gray-200 dark:border-gray-700">
						{#each visibleTabs as tab}
							<button
								class="py-3 px-2 border-b-2 font-medium text-sm {activeTab === tab
									? 'border-orange-600 text-orange-600'
									: 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'}"
								aria-selected={activeTab === tab}
								on:click={() => setActiveTab(tab)}
							>
								{tab === 'run-history' ? 'Run History' : tab.charAt(0).toUpperCase() + tab.slice(1)}
							</button>
						{/each}
					</div>
				{/if}
			</div>

			<div class="flex-1 overflow-y-auto overflow-x-auto px-8">
				<div class="pb-16">
					<div class="rounded-lg max-w-full overflow-x-auto">
						{#if !asset}
							<div class="bg-gray-50 dark:bg-gray-800 rounded-lg p-4">
								<p class="text-gray-500 dark:text-gray-400">Loading asset information...</p>
							</div>
						{:else if activeTab === 'metadata'}
							<div class="mt-6">
								{#if Object.keys(asset.metadata || {}).length > 0}
									<MetadataView metadata={asset.metadata} />
								{:else}
									<div class="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
										<p class="text-gray-500 dark:text-gray-400 italic">No metadata available</p>
									</div>
								{/if}
								<h3 class="pt-4 text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">
									Asset Sources
								</h3>
								<AssetSources sources={asset.sources || []} />
							</div>
						{:else if activeTab === 'query'}
							<div class="mt-6">
								<QueryPreview
									query={asset.query || ''}
									queryLanguage={asset.query_language || 'sql'}
								/>
							</div>
						{:else if activeTab === 'environments'}
							<div class="mt-6">
								{#if asset.environments && Object.keys(asset.environments).length > 0}
									<AssetEnvironmentsView environments={asset.environments} />
								{:else}
									<div class="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
										<p class="text-gray-500 dark:text-gray-400 italic">No environments available</p>
									</div>
								{/if}
							</div>
						{:else if activeTab === 'schema'}
							<div class="mt-6">
								<SchemaSummary schema={asset.schema} />
							</div>
						{:else if activeTab === 'documentation'}
							<div class="mt-6">
								<AssetDocumentation mrn={asset.mrn} />
							</div>
						{:else if activeTab === 'run-history'}
							<div class="mt-6">
								<RunHistory assetId={asset.id} />
							</div>
						{:else if activeTab === 'lineage'}
							<div class="mt-6">
								<Lineage currentAsset={asset} />
							</div>
						{:else}
							<div class="mt-6">
								<p class="text-gray-500 dark:text-gray-400">
									{activeTab.charAt(0).toUpperCase() + activeTab.slice(1)} coming soon.
								</p>
							</div>
						{/if}
					</div>
				</div>
			</div>
		</div>

		<div
			class="border-l border-gray-200 dark:border-gray-700 overflow-hidden transition-all duration-300 {bladeCollapsed
				? 'w-12'
				: 'w-[32rem]'}"
		>
			<AssetBlade
				{asset}
				staticPlacement={true}
				collapsed={bladeCollapsed}
				onToggleCollapse={() => (bladeCollapsed = !bladeCollapsed)}
				onClose={() => {}}
			/>
		</div>
	{/if}
</div>
