<script lang="ts">
	import { formatDate } from '#lib/utils/format.ts';
	import type { KnowledgeField } from '#lib/knowledge/kinds.ts';
	import type { FieldRow, FieldValue } from '#lib/knowledge/api.ts';

	// One typed field's value, read-only. Select values show their label, links open in a new tab and tables keep their columns.
	let { field, value }: { field: KnowledgeField; value: FieldValue | undefined } = $props();

	function label(f: KnowledgeField, v: string): string {
		if (f.type === 'select') return f.options?.find((o) => o.value === v)?.label ?? v;
		if (f.type === 'date') return formatDate(v);
		return v;
	}

	// A link reads as its site and its last path part, such as github.com/…/03628f03, so it stays on one line; the full address shows on hover.
	function shortUrl(v: string): string {
		try {
			const u = new URL(v);
			const host = u.hostname.replace(/^www\./, '');
			const parts = u.pathname.split('/').filter(Boolean);
			if (parts.length === 0) return host;
			if (parts.length === 1) return `${host}/${parts[0]}`;
			return `${host}/…/${parts[parts.length - 1]}`;
		} catch {
			return v.replace(/^https?:\/\//, '');
		}
	}

	let rows = $derived(
		Array.isArray(value)
			? (value as unknown[]).filter((r): r is FieldRow => typeof r === 'object' && r !== null)
			: []
	);
	let items = $derived(
		Array.isArray(value)
			? (value as unknown[]).filter((v): v is string => typeof v === 'string')
			: []
	);
	let columns = $derived(field.columns ?? []);
</script>

{#snippet cell(c: KnowledgeField, v: string | undefined)}
	{#if !v}
		<mm-text tone="secondary">—</mm-text>
	{:else if c.type === 'url'}
		<mm-link href={v} title={v} external>{shortUrl(v)}</mm-link>
	{:else if c.type === 'long_text'}
		<span class="prose">{v}</span>
	{:else}
		{label(c, v)}
	{/if}
{/snippet}

{#if field.type === 'table'}
	<div class="mm-table-wrap">
		<table class="mm-table mm-table--compact">
			<thead>
				<tr>
					{#each columns as c (c.key)}
						<th scope="col">{c.label}</th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#each rows as r, i (i)}
					<tr>
						{#each columns as c (c.key)}
							<td>{@render cell(c, r[c.key])}</td>
						{/each}
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{:else if field.type === 'list'}
	<div class="mm-cluster" style="--mm-gap: var(--space-1)">
		{#each items as item, i (i)}
			<mm-tag>{item}</mm-tag>
		{/each}
	</div>
{:else if typeof value === 'string'}
	{#if field.type === 'url'}
		<mm-link href={value} title={value} external>{shortUrl(value)}</mm-link>
	{:else if field.type === 'long_text'}
		<span class="prose">{value}</span>
	{:else}
		{label(field, value)}
	{/if}
{/if}

<style>
	.prose {
		white-space: pre-line;
	}
</style>
