<script lang="ts">
	import type { Asset } from '$lib/assets/types';
	import { m } from '$lib/paraglide/messages';
	import { html } from 'lit';
	import type { DataTableColumn } from '@marmotdata/elements/classes/data-table.js';
	import { assetHref, providerKey } from './assetFormat';

	// Containment is stored as CONTAINS lineage, so both directions come from the same edges: what this asset holds, and what holds it.
	let { children, parents, loading }: { children: Asset[]; parents: Asset[]; loading: boolean } =
		$props();

	const columns: DataTableColumn<Asset>[] = [
		{
			key: 'name',
			header: m.common_name(),
			sortable: true,
			width: 280,
			href: (a) => assetHref(a),
			render: (a) =>
				html`<span class="mm-cluster" style="--mm-gap: var(--space-2); flex-wrap: nowrap"
					><mm-provider-logo provider=${providerKey(a)} size="16" decorative></mm-provider-logo
					>${a.name}</span
				>`
		},
		{ key: 'type', header: m.common_type(), sortable: true, width: 140 },
		{
			key: 'description',
			header: m.common_description(),
			value: (a) => a.user_description || a.description || '',
			empty: ''
		}
	];
</script>

<div class="mm-stack" style="--mm-gap: var(--space-4)">
	{#if parents.length > 0}
		<div class="mm-cluster" style="--mm-gap: var(--space-2)">
			<mm-text variant="dense" tone="secondary">{m.asset_contents_inside_label()}</mm-text>
			{#each parents as parent (parent.mrn)}
				<mm-link href={assetHref(parent)}>{parent.name}</mm-link>
			{/each}
		</div>
	{/if}

	<mm-data-table
		label={m.discover_tab_contents()}
		row-key="mrn"
		{columns}
		rows={children}
		{loading}
		sort={{ key: 'name', direction: 'asc' }}
		empty-heading={m.asset_page_contents_empty_heading()}
		empty-text={m.asset_contents_empty()}
	></mm-data-table>
</div>
