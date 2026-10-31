<script lang="ts">
	import { m } from '$lib/paraglide/messages';
	import '@marmotdata/elements/results-grid.js';

	interface Props {
		columnNames: string[];
		rows: unknown[][];
		loading?: boolean;
		error?: string | null;
		onretry?: () => void;
	}

	let { columnNames = [], rows = [], loading = false, error = null, onretry }: Props = $props();

	let columns = $derived(columnNames.map((name) => ({ name, type: '' })));
</script>

{#if loading}
	<mm-loading active caption={m.asset_page_preview_loading()}></mm-loading>
{:else if error}
	<mm-callout tone="danger" heading={m.asset_page_preview_error_heading()}>
		{error}
		{#if onretry}
			<mm-button slot="actions" size="sm" variant="secondary" icon="refresh" onclick={onretry}
				>{m.common_retry()}</mm-button
			>
		{/if}
	</mm-callout>
{:else if columnNames.length === 0 || rows.length === 0}
	<mm-empty-state
		heading={m.asset_page_preview_empty_heading()}
		description={m.asset_page_preview_empty_body()}
		compact
	></mm-empty-state>
{:else}
	<mm-results-grid
		label={m.asset_page_preview_label()}
		{columns}
		{rows}
		no-export
		style="--mm-results-grid-height: 32rem"
	></mm-results-grid>
{/if}
