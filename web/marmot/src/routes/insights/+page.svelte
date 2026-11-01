<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { m } from '$lib/paraglide/messages';
	import { setCrumbs } from '$lib/mm';
	import { myAccess, loadMyAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import CatalogInsights from '$components/insights/CatalogInsights.svelte';
	import QueryInsights from '$components/insights/QueryInsights.svelte';
	import { isRange, type InsightsRange } from '$components/insights/range';

	type View = 'queries' | 'catalog';

	setCrumbs([{ label: m.insights_heading() }]);

	onMount(() => {
		loadMyAccess();
	});

	// Query stats answer callers who may query data or read the query log; everyone else only has the catalog view
	let canViewLog = $derived(holdsAtRoot($myAccess, 'access:view'));
	let canSeeQueries = $derived(canViewLog || holdsAtRoot($myAccess, 'assets:query'));

	// The view and window live in the URL so a shared link opens the same dashboard
	let requestedView = $derived<View>(
		page.url.searchParams.get('view') === 'catalog' ? 'catalog' : 'queries'
	);
	let view = $derived<View>(canSeeQueries ? requestedView : 'catalog');
	let range = $derived<InsightsRange>(
		isRange(page.url.searchParams.get('range'))
			? (page.url.searchParams.get('range') as InsightsRange)
			: '7d'
	);

	function show(next: { view?: View; range?: InsightsRange }) {
		const query = `view=${next.view ?? view}&range=${next.range ?? range}`;
		goto(resolve(`/insights?${query}`), { replaceState: true, noScroll: true, keepFocus: true });
	}
</script>

<svelte:head>
	<title>{m.pagetitle_insights()}</title>
</svelte:head>

<div class="mm-stack">
	<mm-page-header heading={m.insights_heading()} description={m.insights_description()}>
		{#if canSeeQueries}
			<mm-segmented-control
				slot="actions"
				name="view"
				label={m.insights_view_label()}
				hide-label
				value={view}
				onmm-change={(e: CustomEvent<{ value: string }>) => show({ view: e.detail.value as View })}
			>
				<mm-segment value="queries">{m.insights_view_queries()}</mm-segment>
				<mm-segment value="catalog">{m.insights_view_catalog()}</mm-segment>
			</mm-segmented-control>
		{/if}
		<mm-select
			slot="actions"
			name="range"
			label={m.dataquery_range_label()}
			hide-label
			value={range}
			style="inline-size: 10rem"
			onmm-change={(e: CustomEvent<{ value: string }>) =>
				isRange(e.detail.value) && show({ range: e.detail.value })}
		>
			<mm-option value="24h">{m.insights_range_24h()}</mm-option>
			<mm-option value="7d">{m.dataquery_range_7d()}</mm-option>
			<mm-option value="30d">{m.dataquery_range_30d()}</mm-option>
		</mm-select>
	</mm-page-header>

	{#if !$myAccess.checked}
		<!-- Waits for the token's permissions so the default view does not flash from catalog to queries -->
	{:else if view === 'queries'}
		<QueryInsights {range} {canViewLog} />
	{:else}
		<CatalogInsights {range} />
	{/if}
</div>
