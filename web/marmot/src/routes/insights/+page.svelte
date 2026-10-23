<script lang="ts">
	import { m } from '$lib/paraglide/messages';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import CatalogInsights from '$components/insights/CatalogInsights.svelte';
</script>

<svelte:head>
	<title>{m.pagetitle_insights()}</title>
</svelte:head>

<div class="w-full px-4 py-6 sm:px-6 lg:px-8">
	<PageHeader title={m.insights_heading()} />
	<CatalogInsights />
</div>
