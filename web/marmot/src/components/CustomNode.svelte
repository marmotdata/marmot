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
			hasUpstream: boolean;
			hasDownstream: boolean;
			isStub?: boolean;
			nodeClickHandler: (id: string) => void;
		};
	}>();

	function handleClick() {
		if (!data.isStub) {
			data.nodeClickHandler(data.id);
		}
	}
</script>

{#if data.hasUpstream}
	<Handle type="target" position={Position.Left} style="background: #696969;" />
{/if}

<div
	class="node {data.isCurrent ? 'current' : ''} {data.isStub ? 'stub' : ''}"
	on:click={handleClick}
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

<style>
	.node {
		padding: 1rem;
		border-radius: 0.5rem;
		border: 1px solid #dfdfdf;
		background: #fefdf8;
		cursor: pointer;
		min-width: 150px;
		transition: all 150ms;
		position: relative;
		overflow: hidden;
	}

	:global(.dark) .node {
		background: #2e2e2e;
		border-color: #4d4d4d;
	}

	.node:not(.current):not(.stub):hover {
		border-color: #fb923c;
		background: #fff7ed;
	}

	:global(.dark) .node:not(.current):not(.stub):hover {
		background: #4d4d4d;
	}

	.node.current {
		background: #fff7ed;
		border: 2px solid #ea580c;
	}

	:global(.dark) .node.current {
		background: #4d4d4d;
	}

	.node.stub {
		cursor: default;
		background: #f5f4ef;
		border-color: #c9c9c9;
	}

	:global(.dark) .node.stub {
		background: #252525;
		border-color: #3d3d3d;
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
		border: 2px solid #fefdf8;
	}

	:global(.dark .svelte-flow__handle) {
		border-color: #2e2e2e;
	}
</style>
