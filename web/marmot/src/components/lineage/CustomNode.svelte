<script lang="ts">
	import { Handle, Position } from '@xyflow/svelte';
	import Icon from '$components/ui/Icon.svelte';
	import IconifyIcon from '@iconify/svelte';

	let { data } = $props<{
		data: {
			name: string;
			type: string;
			iconType: string;
			service: string;
			isCurrent: boolean;
			id: string;
			mrn: string;
			hasUpstream: boolean;
			hasDownstream: boolean;
			isStub?: boolean;
			schema?: Record<string, string>;
			nodeClickHandler: (id: string) => void;
			onAddUpstream?: (nodeMrn: string, event: MouseEvent) => void;
			onAddDownstream?: (nodeMrn: string, event: MouseEvent) => void;
		};
	}>();

	// Columns collapse by default — the node stays compact until the user
	// asks to see fields. Keeps a 30-column fact table from dominating the
	// graph on first render.
	let columnsExpanded = $state(false);

	// Marmot's schema shape is loose: postgres-like plugins bundle full
	// column detail as a JSON blob under schema.columns, while lighter
	// plugins emit a flat {colName: type} map. Normalise here so the UI
	// doesn't care.
	type Column = { name: string; type: string; primaryKey?: boolean };

	let columns = $derived.by<Column[]>(() => {
		const s = data.schema;
		if (!s) return [];
		const raw = s['columns'];
		if (typeof raw === 'string' && raw.length > 0) {
			try {
				const parsed = JSON.parse(raw) as Array<Record<string, unknown>>;
				return parsed.map((c) => ({
					name: String(c.column_name ?? c.name ?? ''),
					type: String(c.data_type ?? c.type ?? ''),
					primaryKey: Boolean(c.is_primary_key)
				}));
			} catch {
				return [];
			}
		}
		// Flat map fallback: iterate the schema entries directly.
		return Object.entries(s)
			.filter(([k]) => k !== 'columns')
			.map(([name, type]) => ({ name, type: String(type) }));
	});

	let hasColumns = $derived(columns.length > 0);

	function handleClick() {
		if (!data.isStub) {
			data.nodeClickHandler(data.id);
		}
	}

	function handleAddUpstream(e: MouseEvent) {
		e.stopPropagation();
		data.onAddUpstream?.(data.mrn, e);
	}

	function handleAddDownstream(e: MouseEvent) {
		e.stopPropagation();
		data.onAddDownstream?.(data.mrn, e);
	}

	function toggleColumns(e: MouseEvent) {
		e.stopPropagation();
		columnsExpanded = !columnsExpanded;
	}
</script>

{#if data.onAddUpstream && !data.isStub}
	<button
		class="add-lineage-btn add-upstream"
		onclick={handleAddUpstream}
		title="Add upstream dependency"
		aria-label="Add upstream dependency to {data.name}"
	>
		<IconifyIcon icon="material-symbols:add-rounded" class="w-4 h-4" aria-hidden="true" />
	</button>
{/if}

{#if data.hasUpstream}
	<Handle type="target" position={Position.Left} style="background: #696969;" />
{/if}

<div
	class="node {data.isCurrent ? 'current' : ''} {data.isStub ? 'stub' : ''}"
	onclick={handleClick}
	title={data.isStub ? 'Stub asset created by OpenLineage' : ''}
>
	{#if data.isStub}
		<div class="stub-corner" title="Stub asset created by OpenLineage">
			<IconifyIcon
				icon="bi:ticket-perforated-fill"
				class="w-4 h-4 text-white absolute"
				style="transform: rotate(-45deg); top: -33px; left: 5px;"
			/>
		</div>
	{/if}

	<div
		class="text-xs text-gray-500 dark:text-gray-400 font-bold text-center pb-2 border-b border-gray-200 dark:border-gray-600 flex items-center justify-center gap-1"
	>
		<div class="flex items-center justify-center">
			<div class="text-gray-500 dark:text-gray-400" style="filter: grayscale(1) opacity(0.6);">
				<Icon name={data.type} size="s" showLabel={false} />
			</div>
		</div>
		<span class="uppercase">{data.type}</span>
	</div>
	<div class="name text-gray-900 dark:text-gray-100 text-center mt-2">{data.name}</div>
	<div class="text-xs text-gray-500 dark:text-gray-400 mt-1 text-center">{data.provider}</div>
	<div class="flex justify-center mt-2">
		<div class="icon-wrapper p-2">
			<Icon name={data.iconType} size="sm" />
		</div>
	</div>

	{#if hasColumns}
		<div class="columns-section">
			<button
				type="button"
				class="columns-heading"
				onclick={toggleColumns}
				aria-expanded={columnsExpanded}
			>
				<span class="chev" class:open={columnsExpanded} aria-hidden="true">▸</span>
				<span class="columns-label">Columns</span>
				<span class="columns-count">{columns.length}</span>
			</button>
			{#if columnsExpanded}
				<ul class="columns-list">
					{#each columns as col}
						<li class="column-row" title="{col.name}: {col.type}">
							<span class="col-name">
								{#if col.primaryKey}
									<span class="pk-marker" title="Primary key">🔑</span>
								{/if}
								{col.name}
							</span>
							<span class="col-type">{col.type}</span>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}
</div>

{#if data.hasDownstream}
	<Handle type="source" position={Position.Right} style="background: #696969;" />
{/if}

{#if data.onAddDownstream && !data.isStub}
	<button
		class="add-lineage-btn add-downstream"
		onclick={handleAddDownstream}
		title="Add downstream dependency"
		aria-label="Add downstream dependency to {data.name}"
	>
		<IconifyIcon icon="material-symbols:add-rounded" class="w-4 h-4" aria-hidden="true" />
	</button>
{/if}

<style>
	.node {
		padding: 1rem;
		border-radius: 0.5rem;
		border: 1px solid #e5e7eb;
		background: #ffffff;
		cursor: pointer;
		min-width: 180px;
		max-width: 240px;
		transition: all 150ms;
		position: relative;
		overflow: hidden;
	}

	:global(.dark) .node {
		background: #1f2937;
		border-color: #374151;
	}

	.node:not(.current):not(.stub):hover {
		border-color: #fb923c;
		background: #f9fafb;
	}

	:global(.dark) .node:not(.current):not(.stub):hover {
		background: #374151;
	}

	.node.current {
		background: #fff7ed;
		border: 2px solid #ea580c;
	}

	:global(.dark) .node.current {
		background: #374151;
	}

	.node.stub {
		cursor: default;
		background: #f9fafb;
		border-color: #d1d5db;
	}

	:global(.dark) .node.stub {
		background: #111827;
		border-color: #374151;
	}

	.name {
		font-weight: 500;
	}

	.stub-corner {
		position: absolute;
		top: -1px;
		left: -1px;
		width: 0;
		height: 0;
		border-top: 40px solid #f97316;
		border-right: 40px solid transparent;
		z-index: 10;
		opacity: 0.7;
	}

	:global(.dark) .stub-corner {
		border-top-color: #fb923c;
	}

	:global(.svelte-flow__handle) {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		border: 2px solid #ffffff;
	}

	:global(.dark .svelte-flow__handle) {
		border-color: #1f2937;
	}

	.add-lineage-btn {
		position: absolute;
		top: 50%;
		transform: translateY(-50%);
		width: 24px;
		height: 24px;
		border-radius: 50%;
		background: #e55633;
		border: 2px solid #fefdf8;
		color: white;
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
		transition: all 150ms;
		z-index: 20;
		opacity: 0;
	}

	:global(.dark) .add-lineage-btn {
		border-color: #1f2937;
		background: #e55633;
	}

	.node:hover .add-lineage-btn,
	:global(.svelte-flow__node:hover) .add-lineage-btn {
		opacity: 1;
	}

	.add-lineage-btn:hover {
		background: #d25a30;
		transform: translateY(-50%) scale(1.1);
	}

	.add-upstream {
		left: -12px;
	}

	.add-downstream {
		right: -12px;
	}

	.columns-section {
		margin-top: 0.75rem;
		padding-top: 0.5rem;
		border-top: 1px solid #f1f5f9;
	}
	:global(.dark) .columns-section {
		border-top-color: #334155;
	}
	.columns-heading {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		background: none;
		border: none;
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: #64748b;
		padding: 3px 4px;
		cursor: pointer;
		border-radius: 4px;
		transition: color 120ms ease, background-color 120ms ease;
	}
	.columns-heading:hover {
		background: #f8fafc;
		color: #334155;
	}
	:global(.dark) .columns-heading {
		color: #94a3b8;
	}
	:global(.dark) .columns-heading:hover {
		background: #111827;
		color: #e2e8f0;
	}
	.chev {
		display: inline-block;
		font-size: 10px;
		line-height: 1;
		color: #cbd5e1;
		transition: transform 140ms ease, color 120ms ease;
		flex-shrink: 0;
	}
	.chev.open {
		transform: rotate(90deg);
		color: #ea580c;
	}
	.columns-heading:hover .chev {
		color: #64748b;
	}
	.columns-heading:hover .chev.open {
		color: #ea580c;
	}
	.columns-label {
		flex: 1;
		text-align: left;
	}
	.columns-count {
		font-weight: 600;
		color: #cbd5e1;
		font-variant-numeric: tabular-nums;
	}
	:global(.dark) .columns-count {
		color: #475569;
	}
	.columns-list {
		list-style: none;
		padding: 4px 0 0 0;
		margin: 0;
		display: flex;
		flex-direction: column;
	}
	.column-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
		padding: 2px 4px;
		font-size: 11px;
		border-radius: 3px;
		min-width: 0;
	}
	.column-row:hover {
		background: #f8fafc;
	}
	:global(.dark) .column-row:hover {
		background: #111827;
	}
	.col-name {
		font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, monospace;
		color: #0f172a;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		display: inline-flex;
		align-items: center;
		gap: 0.3rem;
		min-width: 0;
	}
	:global(.dark) .col-name {
		color: #e2e8f0;
	}
	.pk-marker {
		font-size: 8px;
		line-height: 1;
		flex-shrink: 0;
	}
	.col-type {
		font-size: 10px;
		color: #94a3b8;
		font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, monospace;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		max-width: 90px;
		text-align: right;
	}
	:global(.dark) .col-type {
		color: #64748b;
	}
</style>
