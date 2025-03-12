<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { onMount } from 'svelte';
	import AssetCardGrid from './AssetCardGrid.svelte';
	import TagsGrid from './TagsGrid.svelte';

	interface AssetTypeSummary {
		count: number;
		service: string;
	}

	interface AssetSummaryResponse {
		types: { [key: string]: AssetTypeSummary };
		providers: { [key: string]: number };
		tags: { [key: string]: number };
	}

	let summary: AssetSummaryResponse = {
		types: {},
		providers: {},
		tags: {}
	};
	let isLoading = true;

	async function fetchAssetSummary() {
		try {
			const response = await fetchApi('/assets/summary');
			summary = await response.json();
		} finally {
			isLoading = false;
		}
	}

	onMount(fetchAssetSummary);
</script>

<div class="flex flex-col gap-8 mt-8 w-full">
	<AssetCardGrid title="Asset Types" {isLoading} items={summary.types} filterType="types" />

	<AssetCardGrid title="Providers" {isLoading} items={summary.providers} filterType="providers" />

	<TagsGrid {isLoading} tags={summary.tags} />
</div>
