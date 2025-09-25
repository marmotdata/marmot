<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount } from 'svelte';
	import AssetCardGrid from './AssetCardGrid.svelte';
	import TagsGrid from './TagsGrid.svelte';
	import GettingStarted from './GettingStarted.svelte';

	interface AssetTypeSummary {
		count: number;
		service: string;
	}

	interface AssetSummaryResponse {
		types: { [key: string]: AssetTypeSummary };
		providers: { [key: string]: number };
		tags: { [key: string]: number };
	}

	let summary = $state<AssetSummaryResponse>({
		types: {},
		providers: {},
		tags: {}
	});
	let isLoading = $state(true);
	let hasLoadedOnce = $state(false);

	let hasAssets = $derived(
		hasLoadedOnce &&
			!isLoading &&
			(Object.keys(summary.types).length > 0 ||
				Object.keys(summary.providers).length > 0 ||
				Object.keys(summary.tags).length > 0)
	);

	let showGettingStarted = $derived(
		hasLoadedOnce &&
			!isLoading &&
			Object.keys(summary.types).length === 0 &&
			Object.keys(summary.providers).length === 0 &&
			Object.keys(summary.tags).length === 0
	);

	async function fetchAssetSummary() {
		try {
			const response = await fetchApi('/assets/summary');
			summary = await response.json();
		} finally {
			isLoading = false;
			hasLoadedOnce = true;
		}
	}

	onMount(fetchAssetSummary);
</script>

{#if hasAssets}
	<div class="flex flex-col gap-8 mt-8 w-full">
		<AssetCardGrid title="Asset Types" {isLoading} items={summary.types} filterType="types" />

		<AssetCardGrid title="Providers" {isLoading} items={summary.providers} filterType="providers" />

		<TagsGrid {isLoading} tags={summary.tags} />
	</div>
{:else if showGettingStarted}
	<div class="mt-8">
		<GettingStarted
			condensed={true}
			title="Get Started with Marmot"
			description="Start populating your data catalog with assets from your data sources."
		/>
	</div>
{:else}
	<div class="flex flex-col gap-8 mt-8 w-full">
		<AssetCardGrid title="Asset Types" {isLoading} items={summary.types} filterType="types" />

		<AssetCardGrid title="Providers" {isLoading} items={summary.providers} filterType="providers" />

		<TagsGrid {isLoading} tags={summary.tags} />
	</div>
{/if}
