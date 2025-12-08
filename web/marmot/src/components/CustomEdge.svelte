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
		data
	} = $props();

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
