<script lang="ts">
	import { m } from '$lib/paraglide/messages';
	import CatalogInsights from '$components/insights/CatalogInsights.svelte';
</script>

<svelte:head>
	<title>{m.pagetitle_insights()}</title>
</svelte:head>

<div class="w-full px-4 py-6 sm:px-6 lg:px-8">
	<h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-gray-100">
		{m.insights_heading()}
	</h1>
	<div class="mt-5">
		<CatalogInsights />
	</div>
</div>
