<script lang="ts">
	import { BaseEdge, getBezierPath } from '@xyflow/svelte';
	import IconifyIcon from '@iconify/svelte';

	let {
		id,
		sourceX,
		sourceY,
		targetX,
		targetY,
		sourcePosition,
		targetPosition,
		style = {},
		markerEnd,
		label,
		data
	} = $props();

	let isObserved = $derived(data?.edgeOrigin === 'observed');
	let observationCount = $derived(Number(data?.observationCount ?? 0));
	// Cluster edges (agent ↔ AgentClusterNode) suppress the chip — the cluster
	// card already shows the lookup count, so two readouts side-by-side is noise.
	let showObservedChip = $derived(isObserved && !data?.suppressLabel);

	let isHovered = $state(false);

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
</style>
