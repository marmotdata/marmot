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
			memberMRNs: string[];
			recencyMs?: number;
			clusterKey?: string;
			onToggleExpand?: (clusterKey: string) => void;
		};
	}>();

	function handleExpand(e: MouseEvent) {
		e.stopPropagation();
		if (data.onToggleExpand && data.clusterKey) {
			data.onToggleExpand(data.clusterKey);
		}
	}

	function formatCount(n: number): string {
		if (n < 1000) return String(n);
		return `${(n / 1000).toFixed(1)}k`;
	}

	function relativeTime(ms?: number): string | null {
		if (!ms) return null;
		const diff = Date.now() - ms;
		if (diff < 60_000) return 'just now';
		if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`;
		if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`;
		return `${Math.floor(diff / 86_400_000)}d ago`;
	}

	let lastSeen = $derived(relativeTime(data.recencyMs));
</script>

<Handle type="source" position={Position.Right} style="background: #607b60;" />

<div
	class="cluster"
	class:clickable={!!data.onToggleExpand}
	title={data.memberMRNs.slice(0, 6).join('\n') +
		(data.memberMRNs.length > 6 ? `\n…+${data.memberMRNs.length - 6} more` : '') +
		'\n\nClick to expand'}
	onclick={handleExpand}
	onkeydown={(e) =>
		(e.key === 'Enter' || e.key === ' ') && handleExpand(e as unknown as MouseEvent)}
	role="button"
	tabindex="0"
>
	<div class="stack-shadow stack-shadow-2"></div>
	<div class="stack-shadow stack-shadow-1"></div>

	<div class="card">
		<div class="observed-pill">
			<IconifyIcon icon="material-symbols:visibility-outline" class="w-3 h-3" />
			<span>OBSERVED · runtime</span>
		</div>

		<div class="header">
			<div class="provider-icon">
				<Icon name={data.provider} size="sm" showLabel={false} />
			</div>
			<div class="meta">
				<div class="provider-label">{data.provider}</div>
				<div class="type-label">{data.assetType}</div>
			</div>
			<div class="count-badge">{formatCount(data.count)}</div>
		</div>

		<div class="footer">
			<span>{formatCount(data.totalObservations)} lookups</span>
			{#if lastSeen}
				<span class="last-seen">· {lastSeen}</span>
			{/if}
			{#if data.onToggleExpand}
				<span class="expand-hint">
					<IconifyIcon icon="material-symbols:unfold-more-rounded" class="w-3 h-3" />
					expand
				</span>
			{/if}
		</div>
	</div>
</div>

<style>
	.cluster {
		position: relative;
		width: 200px;
		outline: none;
	}

	.cluster.clickable {
		cursor: pointer;
	}

	.cluster.clickable:hover .card {
		border-color: #607b60;
		box-shadow: 0 2px 8px rgba(96, 123, 96, 0.18);
	}

	.cluster:focus-visible .card {
		border-color: #607b60;
		box-shadow: 0 0 0 2px rgba(96, 123, 96, 0.35);
	}

	.expand-hint {
		margin-left: auto;
		display: inline-flex;
		align-items: center;
		gap: 0.125rem;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		font-weight: 600;
		font-size: 0.563rem;
		color: #607b60;
	}

	/* Stack effect: two ghost cards behind the main one */
	.stack-shadow {
		position: absolute;
		inset: 0;
		border-radius: 0.5rem;
		border: 1px solid #e5e7eb;
		background: #ffffff;
	}
	.stack-shadow-1 {
		transform: translate(4px, 4px);
		opacity: 0.6;
	}
	.stack-shadow-2 {
		transform: translate(8px, 8px);
		opacity: 0.3;
	}

	:global(.dark) .stack-shadow {
		background: #1f2937;
		border-color: #374151;
	}

	.card {
		position: relative;
		padding: 0.75rem;
		padding-top: 1.75rem; /* leave space for the OBSERVED pill */
		border-radius: 0.5rem;
		border: 1px solid #d1d5db;
		background: #ffffff;
		box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
	}

	:global(.dark) .card {
		background: #1f2937;
		border-color: #4b5563;
	}

	.observed-pill {
		position: absolute;
		top: -0.5rem;
		left: 0.75rem;
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		padding: 0.125rem 0.5rem;
		font-size: 0.563rem;
		font-weight: 600;
		letter-spacing: 0.06em;
		color: #ffffff;
		background: #607b60;
		border-radius: 999px;
		box-shadow: 0 1px 0 rgba(0, 0, 0, 0.05);
	}

	.header {
		display: flex;
		align-items: center;
		gap: 0.625rem;
	}

	.provider-icon {
		flex-shrink: 0;
		width: 2rem;
		height: 2rem;
		display: flex;
		align-items: center;
		justify-content: center;
		background: #f9fafb;
		border-radius: 0.375rem;
		border: 1px solid #e5e7eb;
	}

	:global(.dark) .provider-icon {
		background: #111827;
		border-color: #374151;
	}

	.meta {
		flex: 1;
		min-width: 0;
	}

	.provider-label {
		font-size: 0.75rem;
		font-weight: 600;
		color: #111827;
		line-height: 1.1;
	}

	:global(.dark) .provider-label {
		color: #f3f4f6;
	}

	.type-label {
		font-size: 0.625rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: #6b7280;
		margin-top: 0.125rem;
	}

	:global(.dark) .type-label {
		color: #9ca3af;
	}

	.count-badge {
		flex-shrink: 0;
		min-width: 1.75rem;
		height: 1.75rem;
		padding: 0 0.5rem;
		display: flex;
		align-items: center;
		justify-content: center;
		background: #607b60;
		color: white;
		border-radius: 0.875rem;
		font-size: 0.75rem;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}

	.footer {
		margin-top: 0.625rem;
		padding-top: 0.5rem;
		border-top: 1px dashed #e5e7eb;
		display: flex;
		align-items: center;
		gap: 0.375rem;
		font-size: 0.625rem;
		color: #6b7280;
		font-variant-numeric: tabular-nums;
	}

	:global(.dark) .footer {
		border-top-color: #374151;
		color: #9ca3af;
	}

	.last-seen {
		opacity: 0.7;
	}
</style>
