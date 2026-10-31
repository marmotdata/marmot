<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages';
	import MetadataView from '$components/shared/MetadataView.svelte';
	import { assetHref } from './graph';

	interface LineageNode {
		id: string;
		type: string;
		asset: {
			mrn?: string;
			description?: string;
			tags?: string[];
			providers?: string[];
			provider?: string;
			metadata?: Record<string, unknown>;
		};
	}

	let {
		node,
		expanded,
		onClick,
		maxMetadataDepth = 1,
		compact = false
	}: {
		node: LineageNode;
		expanded: boolean;
		onClick: () => void;
		maxMetadataDepth?: number;
		compact?: boolean;
	} = $props();

	let name = $derived(node.id.split('/').pop() ?? node.id);
	let provider = $derived(node.asset.providers?.[0] ?? node.asset.provider ?? '');
	let path = $derived(assetHref({ mrn: node.asset.mrn ?? '', type: node.type }));
	let href = $derived(path ? resolve(path as `/discover/${string}/${string}/${string}`) : '');
	let hasMetadata = $derived(Object.keys(node.asset.metadata ?? {}).length > 0);
</script>

{#if compact}
	<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- href is resolved above -->
	<a {href} class="row compact">
		{#if provider}
			<mm-provider-logo {provider} size="16" decorative></mm-provider-logo>
		{/if}
		<span class="name">{name}</span>
	</a>
{:else}
	<div class="node">
		<div class="row">
			<button
				type="button"
				class="toggle"
				aria-expanded={expanded}
				onclick={onClick}
				disabled={!node.asset.description && !node.asset.tags?.length && !hasMetadata}
			>
				{#if provider}
					<mm-provider-logo {provider} size="20" decorative></mm-provider-logo>
				{/if}
				<span class="text">
					<span class="name">{name}</span>
					<span class="caption">{node.asset.providers?.join(', ') || node.asset.provider}</span>
				</span>
				<mm-icon name={expanded ? 'expand-less' : 'expand-more'} aria-hidden="true"></mm-icon>
			</button>
			{#if href}
				<mm-button variant="ghost" size="sm" icon="open-in-full" {href}
					>{m.lineage_view_full()}</mm-button
				>
			{/if}
		</div>
		{#if expanded}
			<div class="mm-stack details" style="--mm-gap: var(--space-3)">
				{#if node.asset.description}
					<div>
						<div class="label">{m.common_description()}</div>
						<mm-text variant="dense" tone="secondary" block>{node.asset.description}</mm-text>
					</div>
				{/if}
				{#if node.asset.tags?.length}
					<div>
						<div class="label">{m.common_tags()}</div>
						<div class="mm-cluster" style="--mm-gap: var(--space-2)">
							{#each node.asset.tags as tag (tag)}
								<mm-tag>{tag}</mm-tag>
							{/each}
						</div>
					</div>
				{/if}
				{#if hasMetadata}
					<div>
						<div class="label">{m.lineage_metadata_heading()}</div>
						<MetadataView
							metadata={node.asset.metadata ?? {}}
							maxDepth={maxMetadataDepth}
							maxCharLength={50}
							showDetailsLink={href}
						/>
					</div>
				{/if}
			</div>
		{/if}
	</div>
{/if}

<style>
	.row {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		min-width: 0;
	}
	.compact {
		padding: var(--space-2) var(--space-3);
		border: 1px solid var(--mm-rule);
		border-radius: var(--radius-sm);
		color: var(--mm-text);
		font: var(--text-dense);
		text-decoration: none;
		transition: border-color var(--mm-dur-press) var(--mm-ease);
	}
	.compact:hover {
		border-color: var(--mm-control-border);
	}
	.compact:hover .name {
		text-decoration: underline;
	}
	.name {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-weight: 600;
	}
	.node {
		border: 1px solid var(--mm-rule);
		border-radius: var(--radius-md);
		background: var(--mm-surface);
	}
	.node > .row {
		padding: var(--space-3) var(--space-4);
	}
	.toggle {
		all: unset;
		box-sizing: border-box;
		flex: 1;
		min-width: 0;
		display: flex;
		align-items: center;
		gap: var(--space-3);
		cursor: pointer;
		color: var(--mm-text-2);
	}
	.toggle:disabled {
		cursor: default;
	}
	.toggle:disabled mm-icon {
		visibility: hidden;
	}
	.toggle:focus-visible {
		outline: 2px solid var(--mm-text);
		outline-offset: 2px;
	}
	.text {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
	}
	.text .name {
		color: var(--mm-text);
		font: var(--text-dense);
		font-weight: 600;
	}
	.caption {
		font: var(--text-caption);
		color: var(--mm-text-2);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.details {
		padding: var(--space-3) var(--space-4) var(--space-4);
		border-top: 1px solid var(--mm-rule);
	}
	.label {
		font: var(--text-label);
		color: var(--mm-text-2);
		margin-bottom: var(--space-1);
	}
</style>
