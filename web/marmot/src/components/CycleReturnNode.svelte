<script lang="ts">
	import { Handle, Position } from '@xyflow/svelte';
	import Icon from './Icon.svelte';

	let { data } = $props<{
		data: {
			targetName: string;
			targetType: string;
			targetId: string;
			onReturnClick?: (targetId: string) => void;
		};
	}>();

	function handleClick() {
		if (data.onReturnClick) {
			data.onReturnClick(data.targetId);
		}
	}
</script>

<Handle type="target" position={Position.Left} style="background: #f59e0b;" />

<div class="cycle-return-node" onclick={handleClick}>
	<div
		class="text-xs text-gray-500 dark:text-gray-400 font-bold text-center pb-2 border-b border-earthy-terracotta-300 dark:border-earthy-terracotta-700 flex items-center justify-center gap-1"
	>
		<div class="flex items-center justify-center">
			<div class="text-gray-500 dark:text-gray-400" style="filter: grayscale(1) opacity(0.6);">
				<Icon name={data.targetType} size="xs" showLabel={false} />
			</div>
		</div>
		<span class="uppercase">{data.targetType}</span>
	</div>
	<div class="flex items-center justify-center mt-2 mb-1">
		<div class="text-earthy-terracotta-700 dark:text-earthy-terracotta-700 text-lg">↻</div>
	</div>
	<div class="text-xs text-gray-600 dark:text-gray-400 text-center leading-tight">
		Returns to<br />
		<span class="font-medium text-gray-800 dark:text-gray-200">{data.targetName}</span>
	</div>
</div>

<style>
	.cycle-return-node {
		padding: 0.75rem;
		border-radius: 0.5rem;
		border: 2px dashed #f59e0b;
		background: #fffbeb;
		min-width: 100px;
		text-align: center;
		cursor: pointer;
		transition: all 150ms;
	}

	.cycle-return-node:hover {
		border-color: #d97706;
		background: #fef3c7;
		transform: scale(1.02);
	}

	:global(.dark) .cycle-return-node {
		background: #451a03;
		border-color: #d97706;
	}

	:global(.dark) .cycle-return-node:hover {
		background: #78350f;
		border-color: #f59e0b;
	}

	:global(.svelte-flow__handle) {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		border: 2px solid #fffbeb;
	}

	:global(.dark .svelte-flow__handle) {
		border-color: #451a03;
	}
</style>