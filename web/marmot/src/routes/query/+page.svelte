<script lang="ts">
	import { page } from '$app/state';
	import { setCrumbs } from '$lib/mm';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import QueryWorkspace from '$components/dataquery/QueryWorkspace.svelte';

	const canRun = auth.hasPermission('assets', 'query');

	// Links from asset pages, the audit log and Home open the workspace on a source and statement
	const initialSource = page.url.searchParams.get('source') ?? '';
	const initialStatement = page.url.searchParams.get('q') ?? '';
	const autoRun = page.url.searchParams.get('run') === '1';

	setCrumbs([{ label: m.nav_query() }]);
</script>

<svelte:head>
	<title>{m.dataquery_page_title()}</title>
</svelte:head>

{#if canRun}
	<QueryWorkspace {initialSource} {initialStatement} {autoRun} />
{:else}
	<div class="no-access">
		<mm-empty-state
			level={1}
			heading={m.dataquery_no_access_title()}
			description={m.dataquery_no_access_body()}
		></mm-empty-state>
	</div>
{/if}

<style>
	.no-access {
		display: grid;
		place-items: center;
		min-block-size: 100%;
		padding: var(--space-8) var(--space-4);
		box-sizing: border-box;
	}
</style>
