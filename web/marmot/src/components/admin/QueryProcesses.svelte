<script lang="ts">
	import { onMount } from 'svelte';
	import { html } from 'lit';
	import type { DataTableColumn } from '@marmotdata/elements/classes/data-table';
	import { m } from '$lib/paraglide/messages';
	import { getErrorMessage } from '$lib/stores/toast';
	import { getQueryStatus } from '$lib/dataquery/api';
	import type { QueryInstanceStatus } from '$lib/dataquery/types';

	// The plugin processes answering data queries right now; Marmot starts them on first use and stops them once idle
	let instances = $state<QueryInstanceStatus[]>([]);
	let loading = $state(true);
	let error = $state('');

	const columns: DataTableColumn<QueryInstanceStatus>[] = [
		{
			key: 'plugin_id',
			header: m.dataquery_status_col_plugin(),
			width: 220,
			render: (i) =>
				html`<span class="inline-flex items-center gap-2 whitespace-nowrap"
					><mm-provider-logo provider=${i.plugin_id} size="16" decorative></mm-provider-logo
					>${i.plugin_id}</span
				>`
		},
		{ key: 'started_at', header: m.dataquery_status_col_started(), type: 'date', width: 140 },
		{ key: 'last_used_at', header: m.dataquery_status_col_last_used(), type: 'date', width: 140 },
		{
			key: 'active_queries',
			header: m.dataquery_status_col_active(),
			type: 'number',
			width: 140
		},
		{ key: 'restarts', header: m.dataquery_status_col_restarts(), type: 'number', width: 110 }
	];

	async function load() {
		loading = true;
		error = '';
		try {
			instances = await getQueryStatus();
		} catch (e) {
			error = getErrorMessage(e);
		} finally {
			loading = false;
		}
	}

	onMount(load);
</script>

<mm-section heading={m.system_processes_heading()} description={m.system_processes_description()}>
	<mm-button
		slot="actions"
		size="sm"
		variant="ghost"
		icon="refresh"
		disabled={loading}
		onclick={load}>{m.system_processes_refresh()}</mm-button
	>
	<mm-data-table
		label={m.system_processes_heading()}
		row-key="plugin_id"
		{columns}
		rows={instances}
		{loading}
		{error}
		empty-heading={m.system_processes_empty_heading()}
		empty-text={m.system_processes_empty_text()}
		onmm-retry={load}
	></mm-data-table>
</mm-section>
