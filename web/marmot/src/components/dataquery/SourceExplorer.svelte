<script lang="ts">
	import '@marmotdata/elements/tree.js';
	import type { TreeAction, TreeNode } from '@marmotdata/elements/classes/tree';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import { formatNumber } from '$lib/utils/format';
	import { getTableColumns, listQuerySourceTables } from '$lib/dataquery/api';
	import { assetRouteParams, formatRowCount, qualifiedName } from '$lib/dataquery/format';
	import type { QuerySource, QuerySourceTable } from '$lib/dataquery/types';

	let {
		sources,
		total,
		loading,
		error,
		active,
		onMore,
		onRetry,
		onSelect,
		onInsert,
		onSample,
		onTables,
		onColumns
	}: {
		sources: QuerySource[];
		total: number;
		loading: boolean;
		error: string;
		active: string;
		onMore: () => void;
		onRetry: () => void;
		onSelect: (source: string) => void;
		onInsert: (text: string) => void;
		onSample: (source: string, statement: string) => void;
		onTables: (source: string, tables: QuerySourceTable[]) => void;
		onColumns: (source: string, table: QuerySourceTable, columns: { name: string }[]) => void;
	} = $props();

	const TABLE_LIMIT = 200;

	type Entry =
		| { kind: 'source'; source: string }
		| { kind: 'schema'; source: string }
		| { kind: 'table'; source: string; table: QuerySourceTable }
		| { kind: 'column'; source: string; name: string };

	// The tree drops loaded children whenever its roots change (more sources paged in), so loads are cached here and come back at once
	const cache: Record<string, TreeNode[]> = {};

	let expandedIds = $state<string[]>([]);
	let tree = $state<HTMLElement & { expandedIds: string[] }>();

	let nodes = $derived<TreeNode[]>(
		sources.map((s) => ({
			id: `s:${s.name}`,
			label: s.name,
			provider: s.plugin_id || undefined,
			icon: s.plugin_id ? undefined : 'database',
			secondary: formatNumber(s.table_count),
			hasChildren: true,
			data: { kind: 'source', source: s.name } satisfies Entry
		}))
	);

	function tableNode(source: string, table: QuerySourceTable): TreeNode {
		const pii = table.tags?.some((t) => t.toLowerCase() === 'pii');
		return {
			id: `t:${source}:${table.mrn}`,
			label: table.name,
			icon: table.type === 'View' ? 'visibility' : 'table',
			// The size the catalog last saw, so the biggest tables stand out before anyone queries them
			secondary: table.row_count != null ? formatRowCount(table.row_count) : undefined,
			secondaryMono: false,
			hasChildren: !!table.asset_id,
			badges: pii ? [{ label: 'PII', variant: 'pii' }] : undefined,
			data: { kind: 'table', source, table } satisfies Entry
		};
	}

	async function loadChildren(node: TreeNode): Promise<TreeNode[]> {
		const cached = cache[node.id];
		if (cached) return cached;
		const entry = node.data as Entry;
		let children: TreeNode[] = [];
		if (entry.kind === 'source') {
			const tables = await listQuerySourceTables(entry.source, { limit: TABLE_LIMIT });
			onTables(entry.source, tables);
			const schemas = [...new Set(tables.map((t) => t.schema ?? ''))];
			// Schemas only add a level when there is more than one to tell apart
			children =
				schemas.length > 1
					? schemas.map((schema) => ({
							id: `sc:${entry.source}:${schema}`,
							label: schema || m.dataquery_explorer_no_schema(),
							icon: 'schema',
							secondary: formatNumber(tables.filter((t) => (t.schema ?? '') === schema).length),
							children: tables
								.filter((t) => (t.schema ?? '') === schema)
								.map((t) => tableNode(entry.source, t)),
							data: { kind: 'schema', source: entry.source } satisfies Entry
						}))
					: tables.map((t) => tableNode(entry.source, t));
		} else if (entry.kind === 'table' && entry.table.asset_id) {
			const columns = await getTableColumns(entry.table.asset_id);
			onColumns(entry.source, entry.table, columns);
			children = columns.map((c) => ({
				id: `c:${entry.source}:${entry.table.mrn}:${c.name}`,
				label: c.name,
				icon: 'view-column',
				secondary: c.type,
				secondaryMono: true,
				data: { kind: 'column', source: entry.source, name: c.name } satisfies Entry
			}));
		}
		cache[node.id] = children;
		return children;
	}

	function actionsFor(node: TreeNode): TreeAction[] {
		const entry = node.data as Entry;
		const insert = { id: 'insert', label: m.dataquery_explorer_insert_action(), icon: 'input' };
		if (entry.kind === 'column') return [insert];
		if (entry.kind !== 'table') return [];
		const actions = [insert];
		if (entry.table.sample_statement)
			actions.push({ id: 'sample', label: m.dataquery_explorer_preview(), icon: 'code' });
		if (entry.table.asset_id && assetRouteParams(entry.table.mrn))
			actions.push({
				id: 'catalog',
				label: m.dataquery_explorer_open_catalog(),
				icon: 'open-in-new'
			});
		return actions;
	}

	function act(action: string, node: TreeNode) {
		const entry = node.data as Entry;
		if (entry.kind === 'column') {
			if (action === 'insert') onInsert(entry.name);
			return;
		}
		if (entry.kind !== 'table') return;
		if (action === 'insert') onInsert(qualifiedName(entry.table));
		else if (action === 'sample' && entry.table.sample_statement)
			onSample(entry.source, entry.table.sample_statement);
		else if (action === 'catalog') {
			const route = assetRouteParams(entry.table.mrn);
			if (route) goto(resolve('/discover/[type]/[service]/[name]', route));
		}
	}

	// Selecting anything inside a source makes that source the one queries run against
	function select(node: TreeNode) {
		const entry = node.data as Entry;
		if (entry.source !== active) onSelect(entry.source);
	}

	// Enter or a double-click does the obvious thing: a table opens its sample query, a column goes into the editor
	function activate(node: TreeNode) {
		const entry = node.data as Entry;
		if (entry.kind === 'table') {
			if (entry.table.sample_statement) onSample(entry.source, entry.table.sample_statement);
			else onInsert(qualifiedName(entry.table));
		} else if (entry.kind === 'column') onInsert(entry.name);
	}

	// A newly active source opens so the tables you are querying are in view; collapsing it afterwards sticks
	let opened = '';
	$effect(() => {
		if (!active || active === opened) return;
		opened = active;
		const id = `s:${active}`;
		if (!expandedIds.includes(id)) expandedIds = [...expandedIds, id];
	});
</script>

<div class="explorer mm-stack">
	<div class="mm-cluster mm-cluster--between">
		<mm-heading level={2} variant="panel-title">{m.dataquery_explorer_heading()}</mm-heading>
		{#if total > 0}
			<mm-text variant="caption" tone="secondary" numeric>{formatNumber(total)}</mm-text>
		{/if}
	</div>
	<mm-tree
		bind:this={tree}
		label={m.dataquery_explorer_tree_label()}
		filterable
		filter-label={m.dataquery_explorer_filter()}
		filter-placeholder={m.dataquery_explorer_filter()}
		empty-text={m.dataquery_empty_title()}
		{nodes}
		{loading}
		{error}
		{expandedIds}
		selectedIds={active ? [`s:${active}`] : []}
		{loadChildren}
		actions={actionsFor}
		onmm-expand={() => (expandedIds = [...(tree?.expandedIds ?? [])])}
		onmm-select={(e: CustomEvent<{ node: TreeNode }>) => select(e.detail.node)}
		onmm-activate={(e: CustomEvent<{ node: TreeNode }>) => activate(e.detail.node)}
		onmm-action={(e: CustomEvent<{ action: string; node: TreeNode }>) =>
			act(e.detail.action, e.detail.node)}
		onmm-retry={onRetry}
		style="--mm-tree-height: 100%; flex: 1; min-block-size: 0"
	></mm-tree>
	{#if sources.length < total}
		<mm-button size="sm" variant="ghost" icon="expand-more" onclick={onMore}>
			{m.dataquery_explorer_more({ count: formatNumber(total - sources.length) })}
		</mm-button>
	{/if}
</div>

<style>
	.explorer {
		--mm-gap: var(--space-3);
		block-size: 100%;
		box-sizing: border-box;
		padding: var(--space-4) var(--space-3) var(--space-4) var(--space-4);
	}
</style>
