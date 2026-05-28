<script lang="ts">
	import { Handle, Position } from '@xyflow/svelte';
	import Icon from '$components/ui/Icon.svelte';
	import IconifyIcon from '@iconify/svelte';

	let { data } = $props<{
		data: {
			provider: string;
			assetType: string;
			count: number;
			totalObservations: number;
			originKind?: 'declared' | 'observed' | 'mixed';
			clusterKey: string;
			onCollapse: (clusterKey: string) => void;
		};
	}>();

	function handleCollapse(e: MouseEvent) {
		e.stopPropagation();
		data.onCollapse(data.clusterKey);
	}

	let originPrefix = $derived(
		data.originKind === 'declared' ? 'DECLARED' : data.originKind === 'mixed' ? 'MIXED' : 'OBSERVED'
	);
	let originIcon = $derived(
		data.originKind === 'declared'
			? 'material-symbols:link-rounded'
			: 'material-symbols:visibility-outline'
	);
</script>

<!-- Source handle so the synthetic single edge to the focal agent emerges
	from the right edge of the container. -->
<Handle type="source" position={Position.Right} style="background: #607b60;" />

<div class="frame">
	<div class="header">
		<div class="header-left">
			<div class="provider-icon">
				<Icon name={data.provider} size="xs" showLabel={false} />
			</div>
			<div class="meta">
				<div class="title">
					<IconifyIcon icon={originIcon} class="w-3 h-3" />
					<span>{originPrefix} · {data.provider}</span>
				</div>
				<div class="subtitle">
					{data.count}
					{data.assetType}{data.count === 1 ? '' : 's'}
					{#if data.totalObservations > 0}
						· {data.totalObservations} lookups
					{/if}
				</div>
			</div>
		</div>
		<button
			class="collapse-btn"
			onclick={handleCollapse}
			title="Collapse this group"
			aria-label="Collapse group"
		>
			<IconifyIcon icon="material-symbols:unfold-less-rounded" class="w-4 h-4" />
			<span>Collapse</span>
		</button>
	</div>
</div>

<style>
	.frame {
		position: absolute;
		inset: 0;
		border-radius: 0.75rem;
		border: 2px dashed #607b60;
		background: rgba(96, 123, 96, 0.04);
		pointer-events: none; /* let click events fall through to children */
	}

	:global(.dark) .frame {
		background: rgba(96, 123, 96, 0.12);
	}

	.header {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		height: 40px;
		padding: 0 0.5rem 0 0.625rem;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
		background: #607b60;
		color: #ffffff;
		border-radius: 0.625rem 0.625rem 0 0;
		pointer-events: auto;
	}

	.header-left {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		min-width: 0;
	}

	.provider-icon {
		flex-shrink: 0;
		width: 1.5rem;
		height: 1.5rem;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(255, 255, 255, 0.18);
		border-radius: 0.25rem;
		padding: 0.125rem;
	}

	.meta {
		min-width: 0;
		display: flex;
		flex-direction: column;
		justify-content: center;
		line-height: 1.1;
	}

	.title {
		display: flex;
		align-items: center;
		gap: 0.25rem;
		font-size: 0.625rem;
		font-weight: 700;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	.subtitle {
		font-size: 0.625rem;
		opacity: 0.85;
		margin-top: 0.125rem;
		font-variant-numeric: tabular-nums;
	}

	.collapse-btn {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		padding: 0.25rem 0.5rem;
		font-size: 0.625rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: #ffffff;
		background: rgba(255, 255, 255, 0.18);
		border: 1px solid rgba(255, 255, 255, 0.3);
		border-radius: 999px;
		cursor: pointer;
		transition: background 150ms;
		flex-shrink: 0;
	}

	.collapse-btn:hover {
		background: rgba(255, 255, 255, 0.32);
	}

	:global(.svelte-flow__handle) {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		border: 2px solid #ffffff;
	}
</style>
