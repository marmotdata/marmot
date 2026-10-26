<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import IconifyIcon from '@iconify/svelte';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import { listQuerySources } from '$lib/dataquery/api';
	import QueryWorkspace from '$components/dataquery/QueryWorkspace.svelte';

	const canRun = auth.hasPermission('assets', 'query');

	// Links from asset pages and the overview open the workspace on a source and statement
	const initialSource = $page.url.searchParams.get('source') ?? '';
	const initialStatement = $page.url.searchParams.get('q') ?? '';
	const autoRun = $page.url.searchParams.get('run') === '1';

	let sourcesTotal = $state(0);
	let ready = $state(false);

	onMount(async () => {
		if (!canRun) return;
		const sources = await listQuerySources({ limit: 1 }).catch(() => null);
		sourcesTotal = sources?.total ?? 0;
		ready = true;
	});
</script>

<svelte:head>
	<title>{m.dataquery_page_title()}</title>
</svelte:head>

{#if !canRun}
	<div class="mx-auto max-w-lg px-4 py-24 text-center">
		<IconifyIcon
			icon="material-symbols:shield-lock-outline"
			class="mx-auto h-12 w-12 text-gray-400"
		/>
		<h1 class="mt-4 text-xl font-semibold text-gray-900 dark:text-gray-100">
			{m.dataquery_no_access_title()}
		</h1>
		<p class="mt-2 text-sm text-gray-600 dark:text-gray-400">{m.dataquery_no_access_body()}</p>
	</div>
{:else if !ready}
	<div class="flex h-[calc(100dvh-4rem)] min-h-0">
		<div class="hidden w-72 border-r border-gray-200 md:block dark:border-gray-800"></div>
		<div class="flex-1">
			<div class="h-12 border-b border-gray-200 dark:border-gray-800"></div>
			<div class="h-64 animate-pulse bg-gray-100 dark:bg-gray-900"></div>
		</div>
	</div>
{:else}
	<h1 class="sr-only">{m.nav_query()}</h1>
	<!-- The workspace takes the viewport below the 4rem navigation bar, so editor and results scroll on their own -->
	<div class="flex h-[calc(100dvh-4rem)] min-h-0 flex-col">
		<QueryWorkspace hasSources={sourcesTotal > 0} {initialSource} {initialStatement} {autoRun} />
	</div>
{/if}
