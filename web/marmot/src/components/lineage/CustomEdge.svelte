<script lang="ts">
	import { BaseEdge, EdgeLabel, getBezierPath } from '@xyflow/svelte';
	import IconifyIcon from '@iconify/svelte';

	let {
		sourceX,
		sourceY,
		targetX,
		targetY,
		sourcePosition,
		targetPosition,
		style = {},
		markerEnd,
		data
	} = $props();

	let isObserved = $derived(data?.edgeOrigin === 'observed');
	let observationCount = $derived(Number(data?.observationCount ?? 0));
	// Cluster edges (agent ↔ AgentClusterNode) suppress the chip — the cluster
	// card already shows the lookup count, so two readouts side-by-side is noise.
	let showObservedChip = $derived(isObserved && !data?.suppressLabel);

	// Column-level lineage: list of {from_columns, to_column, transform?}
	// entries the parser produced for this edge. Empty when the source didn't
	// emit any column info (which is the default for most sources today).
	let columnLineage = $derived<
		Array<{ from_columns: string[]; to_column: string; transform?: string }>
	>(data?.columnLineage ?? []);
	let hasColumnLineage = $derived(columnLineage.length > 0);

	let isHovered = $state(false);
	let columnsOpen = $state(false);

	function toggleColumns(event: MouseEvent) {
		event.stopPropagation();
		columnsOpen = !columnsOpen;
	}

	const [edgePath, labelX, labelY] = $derived(
		getBezierPath({
			sourceX,
			sourceY,
			sourcePosition,
			targetX,
			targetY,
			targetPosition
		})
	);

	function handleDeleteClick(event: MouseEvent) {
		event.stopPropagation();
		const target = event.currentTarget as HTMLElement;
		const rect = target.getBoundingClientRect();
		const position = {
			x: rect.left + rect.width / 2,
			y: rect.top + rect.height
		};
		data?.onDelete?.(data.edgeId, position);
	}
</script>

<g onmouseenter={() => (isHovered = true)} onmouseleave={() => (isHovered = false)}>
	<BaseEdge path={edgePath} {markerEnd} {style} />

	{#if showObservedChip}
		<foreignObject
			x={labelX - 70}
			y={labelY - 12}
			width="140"
			height="24"
			style="overflow: visible; pointer-events: none;"
		>
			<div class="observed-chip">
				<span>observed</span>
				{#if observationCount > 1}
					<span class="count">· {observationCount}×</span>
				{/if}
			</div>
		</foreignObject>
	{/if}


	{#if isHovered && data?.onDelete}
		<foreignObject
			x={labelX - 16}
			y={labelY - 16}
			width="32"
			height="32"
			style="overflow: visible;"
		>
			<button
				onclick={handleDeleteClick}
				class="flex items-center justify-center w-8 h-8 bg-red-600 hover:bg-red-700 dark:bg-red-500 dark:hover:bg-red-600 text-white rounded-full shadow-xl border-2 border-white dark:border-gray-900 transition-all hover:scale-110"
				title="Delete lineage connection"
			>
				<IconifyIcon icon="material-symbols:delete-outline-rounded" class="w-4.5 h-4.5" />
			</button>
		</foreignObject>
	{/if}
</g>

{#if hasColumnLineage}
	<EdgeLabel x={labelX} y={labelY + 40} transparent={true}>
		<div class="cll-anchor" class:chip-dim={isHovered && !columnsOpen}>
			<button
				type="button"
				class="columns-chip"
				onclick={toggleColumns}
				aria-expanded={columnsOpen}
				aria-label="{columnLineage.length} column mapping{columnLineage.length === 1
					? ''
					: 's'}"
				title="Show column lineage"
			>
				<span class="chip-icon" aria-hidden="true">
					<svg viewBox="0 0 12 12" width="10" height="10" fill="currentColor">
						<rect x="1" y="1" width="3" height="10" rx="0.6" />
						<rect x="5" y="1" width="2.5" height="10" rx="0.6" opacity="0.6" />
						<rect x="8.5" y="1" width="2.5" height="10" rx="0.6" opacity="0.35" />
					</svg>
				</span>
				<span class="chip-count">{columnLineage.length}</span>
				<span class="chip-label">column{columnLineage.length === 1 ? '' : 's'}</span>
			</button>
			{#if columnsOpen}
				<div class="columns-panel">
					<div class="columns-header">
						<span class="header-title">
							<svg viewBox="0 0 12 12" width="11" height="11" fill="currentColor" aria-hidden="true">
								<rect x="1" y="1" width="3" height="10" rx="0.6" />
								<rect x="5" y="1" width="2.5" height="10" rx="0.6" opacity="0.6" />
								<rect x="8.5" y="1" width="2.5" height="10" rx="0.6" opacity="0.35" />
							</svg>
							Column lineage
						</span>
						<button class="close" onclick={toggleColumns} aria-label="Close">×</button>
					</div>
					<div class="columns-body">
						{#each columnLineage as ce}
							<div class="row">
								<span class="from" title={ce.from_columns.join(', ')}
									>{ce.from_columns.join(', ') || '—'}</span
								>
								<span class="arrow">→</span>
								<span class="to" title={ce.to_column}>{ce.to_column}</span>
								{#if ce.transform}
									<span class="transform" title="derived by an expression">fx</span>
								{:else}
									<span class="transform-placeholder" aria-hidden="true"></span>
								{/if}
							</div>
						{/each}
					</div>
				</div>
			{/if}
		</div>
	</EdgeLabel>
{/if}

<style>
	.observed-chip {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		padding: 0.125rem 0.5rem;
		background: white;
		border: 1px solid #607b60;
		border-radius: 999px;
		font-size: 10px;
		font-weight: 600;
		letter-spacing: 0.02em;
		color: #607b60;
		box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
		white-space: nowrap;
		font-variant-numeric: tabular-nums;
		width: fit-content;
		margin: 0 auto;
	}

	:global(.dark) .observed-chip {
		background: #1f2937;
		border-color: #607b60;
	}

	.count {
		opacity: 0.8;
	}

	/* EdgeLabel already centers this container on (labelX, labelY+40) via
	   transform:translate(-50%,-50%), so we just lay out children as a
	   centered column. The z-index has to beat xyflow's node layer
	   (nodes carry z-index 1000+ once selected), so we go big. */
	.cll-anchor {
		display: flex;
		flex-direction: column;
		align-items: center;
		pointer-events: all;
		transition: opacity 120ms ease;
		z-index: 9999;
	}
	/* EdgeLabel writes its own z-index from the edge's store value. Override
	   at the label-container level so the whole popup floats above nodes. */
	:global(.svelte-flow__edge-label:has(> .cll-anchor)) {
		z-index: 9999 !important;
	}
	/* When the edge itself is being hovered (delete button in play) the chip
	   dims so the two controls don't fight for the same cursor real-estate.
	   Panel stays fully visible once opened. */
	.cll-anchor.chip-dim {
		opacity: 0.35;
	}

	/* Chip: matches the warm off-white node cards with the app's orange
	   accent. Sits above the edge, hovers with a small lift. */
	.columns-chip {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 4px 10px 4px 9px;
		background: #ffffff;
		border: 1px solid #e5e7eb;
		border-radius: 999px;
		font-size: 11px;
		font-weight: 500;
		color: #334155;
		box-shadow:
			0 1px 2px rgba(15, 23, 42, 0.06),
			0 2px 8px rgba(15, 23, 42, 0.08);
		white-space: nowrap;
		cursor: pointer;
		line-height: 1.4;
		font-variant-numeric: tabular-nums;
		transition:
			transform 120ms ease,
			box-shadow 120ms ease,
			border-color 120ms ease,
			color 120ms ease;
	}
	.columns-chip:hover {
		transform: translateY(-1px);
		border-color: #fb923c;
		color: #0f172a;
		box-shadow:
			0 2px 4px rgba(15, 23, 42, 0.08),
			0 6px 16px rgba(234, 88, 12, 0.18);
	}
	.chip-icon {
		display: inline-flex;
		align-items: center;
		color: #ea580c;
	}
	.chip-count {
		font-weight: 700;
		color: #0f172a;
	}
	.chip-label {
		color: #64748b;
		font-weight: 500;
	}
	:global(.dark) .columns-chip {
		background: #1f2937;
		border-color: #374151;
		color: #cbd5e1;
	}
	:global(.dark) .columns-chip:hover {
		border-color: #fb923c;
		color: #f1f5f9;
	}
	:global(.dark) .chip-icon {
		color: #fb923c;
	}
	:global(.dark) .chip-count {
		color: #f1f5f9;
	}
	:global(.dark) .chip-label {
		color: #94a3b8;
	}

	/* Panel: same rhythm as the node cards — rounded-lg, hairline border,
	   soft two-layer shadow. */
	.columns-panel {
		margin-top: 10px;
		position: relative;
		background: #ffffff;
		border: 1px solid #e5e7eb;
		border-radius: 8px;
		box-shadow:
			0 1px 2px rgba(15, 23, 42, 0.04),
			0 12px 32px rgba(15, 23, 42, 0.14);
		max-height: 320px;
		overflow: hidden;
		display: flex;
		flex-direction: column;
		min-width: 320px;
		z-index: 20;
		animation: panel-in 140ms ease-out;
	}
	@keyframes panel-in {
		from {
			opacity: 0;
			transform: translateY(-4px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}
	:global(.dark) .columns-panel {
		background: #1f2937;
		border-color: #374151;
	}
	.columns-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 9px 12px;
		border-bottom: 1px solid #f1f5f9;
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: #64748b;
		background: #fefdf8;
	}
	.header-title {
		display: inline-flex;
		align-items: center;
		gap: 6px;
	}
	.header-title > svg {
		flex-shrink: 0;
		color: #ea580c;
	}
	:global(.dark) .columns-header {
		border-bottom-color: #374151;
		color: #94a3b8;
		background: #111827;
	}
	:global(.dark) .header-title > svg {
		color: #fb923c;
	}
	.close {
		background: none;
		border: none;
		font-size: 16px;
		line-height: 1;
		cursor: pointer;
		color: #94a3b8;
		padding: 0 4px;
		border-radius: 4px;
		transition: color 120ms ease, background-color 120ms ease;
	}
	.close:hover {
		color: #334155;
		background: #f1f5f9;
	}
	:global(.dark) .close:hover {
		color: #cbd5e1;
		background: #374151;
	}
	.columns-body {
		overflow-y: auto;
		padding: 6px 12px 10px;
	}
	.row {
		display: grid;
		grid-template-columns: 1fr 14px 1fr 22px;
		align-items: center;
		gap: 8px;
		padding: 5px 4px;
		font-size: 12px;
		border-radius: 4px;
	}
	.row:hover {
		background: #f8fafc;
	}
	:global(.dark) .row:hover {
		background: #111827;
	}
	.from,
	.to {
		font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, monospace;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 11.5px;
	}
	.from {
		color: #64748b;
		text-align: right;
	}
	.to {
		color: #0f172a;
		font-weight: 600;
	}
	:global(.dark) .from {
		color: #94a3b8;
	}
	:global(.dark) .to {
		color: #f1f5f9;
	}
	.arrow {
		color: #cbd5e1;
		font-size: 12px;
		line-height: 1;
		text-align: center;
	}
	:global(.dark) .arrow {
		color: #64748b;
	}
	.transform {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		height: 16px;
		padding: 0 5px;
		font-size: 9px;
		font-weight: 700;
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
		text-transform: lowercase;
		color: #ea580c;
		background: #fff7ed;
		border: 1px solid #fed7aa;
		border-radius: 4px;
		cursor: help;
	}
	:global(.dark) .transform {
		color: #fb923c;
		background: rgba(251, 146, 60, 0.12);
		border-color: rgba(251, 146, 60, 0.3);
	}
	.transform-placeholder {
		width: 22px;
		height: 16px;
	}
</style>
