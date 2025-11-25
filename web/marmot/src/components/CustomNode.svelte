<script lang="ts">
	import { Handle, Position } from '@xyflow/svelte';
	import Icon from './Icon.svelte';
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
			nodeClickHandler: (id: string) => void;
			onAddUpstream?: (nodeMrn: string, event: MouseEvent) => void;
			onAddDownstream?: (nodeMrn: string, event: MouseEvent) => void;
		};
	}>();

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
</script>

<!-- Add Upstream Button -->
{#if data.onAddUpstream && !data.isStub}
	<button
		class="add-lineage-btn add-upstream"
		onclick={handleAddUpstream}
		title="Add upstream dependency"
	>
		<IconifyIcon icon="material-symbols:add-rounded" class="w-4 h-4" />
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
</div>

{#if data.hasDownstream}
	<Handle type="source" position={Position.Right} style="background: #696969;" />
{/if}

<!-- Add Downstream Button -->
{#if data.onAddDownstream && !data.isStub}
	<button
		class="add-lineage-btn add-downstream"
		onclick={handleAddDownstream}
		title="Add downstream dependency"
	>
		<IconifyIcon icon="material-symbols:add-rounded" class="w-4 h-4" />
	</button>
{/if}

<style>
	.node {
		padding: 1rem;
		border-radius: 0.5rem;
		border: 1px solid #e5e7eb;
		background: #ffffff;
		cursor: pointer;
		min-width: 150px;
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
</style>
