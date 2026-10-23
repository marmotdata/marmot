<script lang="ts">
	import SearchManagement from '$components/admin/SearchManagement.svelte';
	import QueryPluginStatus from '$components/dataquery/QueryPluginStatus.svelte';
	import PageHeader from '$components/ui/PageHeader.svelte';
	import { m } from '$lib/paraglide/messages';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';

	// Plugin processes that answer data queries are system health, shown to whoever may read the query log
	let canViewQueryStatus = $derived(holdsAtRoot($myAccess, 'access:view'));
</script>

<svelte:head>
	<title>{m.settings_system()} · {m.settings_title()}</title>
</svelte:head>

<PageHeader title={m.settings_system()} description={m.settings_system_description()} />
<div class="space-y-6">
	{#if $myAccess.admin}
		<SearchManagement />
	{/if}
	{#if canViewQueryStatus}
		<QueryPluginStatus />
	{/if}
</div>
