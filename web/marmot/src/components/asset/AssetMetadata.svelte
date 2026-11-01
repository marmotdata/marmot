<script lang="ts">
	import type { Asset } from '$lib/assets/types';
	import { m } from '$lib/paraglide/messages';
	import { formatRelativeTime } from '$lib/utils';
	import { SvelteSet } from 'svelte/reactivity';
	import '@marmotdata/elements/metadata-viewer.js';
	import '@marmotdata/elements/code-sample.js';

	// The Metadata tab: an agent's spec first (framework, model, tools, prompt), then the metadata itself, then the plugins that wrote it.
	let { asset }: { asset: Asset } = $props();

	let isAgent = $derived((asset.type || '').toLowerCase() === 'agent');
	let framework = $derived(asset.metadata?.framework as string | undefined);
	let model = $derived(asset.metadata?.model as string | undefined);
	let toolNames = $derived<string[]>(
		Array.isArray(asset.metadata?.tool_names) ? (asset.metadata.tool_names as string[]) : []
	);
	let prompt = $derived(
		(asset.metadata?.system_prompt || asset.metadata?.prompt || asset.metadata?.instructions) as
			string | undefined
	);
	let sources = $derived(Array.isArray(asset.sources) ? asset.sources : []);
	const expanded = new SvelteSet<string>();

	function toggle(name: string) {
		if (expanded.has(name)) expanded.delete(name);
		else expanded.add(name);
	}
</script>

<div class="mm-stack" style="--mm-gap: var(--space-6)">
	{#if isAgent}
		<mm-section heading={m.asset_page_agent_spec_heading()}>
			<div class="mm-stack" style="--mm-gap: var(--space-4)">
				<mm-description-list layout="inline-grid" columns={3}>
					<mm-description term={m.asset_agent_spec_framework()}
						>{framework || m.asset_page_not_set()}</mm-description
					>
					<mm-description term={m.asset_agent_spec_model()} mono={!!model}
						>{model || m.asset_page_not_set()}</mm-description
					>
					<mm-description term={m.asset_page_agent_tools()}
						>{toolNames.length > 0 ? toolNames.join(', ') : m.asset_page_not_set()}</mm-description
					>
				</mm-description-list>
				{#if prompt}
					<mm-disclosure summary={m.asset_page_show_system_prompt()}>
						<mm-code-sample
							language="text"
							code={prompt}
							label={m.asset_agent_spec_system_prompt()}
							wrap
						></mm-code-sample>
					</mm-disclosure>
				{/if}
			</div>
		</mm-section>
	{/if}

	<mm-metadata-viewer
		label={m.asset_page_metadata_label({ name: asset.name })}
		metadata={asset.metadata ?? {}}
		expand-depth={1}
		empty-text={m.asset_page_metadata_empty()}
	></mm-metadata-viewer>

	{#if sources.length > 0}
		<mm-section
			heading={m.asset_page_sources_heading()}
			description={m.asset_page_sources_description()}
		>
			<ul class="sources" aria-label={m.asset_page_sources_heading()}>
				{#each sources as source (source.name)}
					{@const hasProps = source.properties && Object.keys(source.properties).length > 0}
					<li class="mm-stack" style="--mm-gap: var(--space-3)">
						<div class="row">
							<mm-provider-logo provider={source.name.toLowerCase()} size="20" decorative
							></mm-provider-logo>
							<div class="mm-stack" style="--mm-gap: 0; min-width: 0">
								<mm-text variant="dense">{source.name}</mm-text>
								{#if source.last_sync_at}
									<mm-text variant="caption" tone="secondary"
										>{m.asset_page_source_synced({
											when: formatRelativeTime(source.last_sync_at)
										})}</mm-text
									>
								{/if}
							</div>
							<mm-tag variant="plugin"></mm-tag>
							{#if hasProps}
								<mm-button
									class="mm-push"
									variant="ghost"
									size="sm"
									expanded={expanded.has(source.name)}
									onclick={() => toggle(source.name)}
									>{expanded.has(source.name)
										? m.asset_page_hide_properties()
										: m.asset_page_show_properties()}</mm-button
								>
							{/if}
						</div>
						{#if hasProps && expanded.has(source.name)}
							<mm-metadata-viewer
								label={m.asset_page_source_properties({ name: source.name })}
								metadata={source.properties}
								hide-toolbar
							></mm-metadata-viewer>
						{/if}
					</li>
				{/each}
			</ul>
		</mm-section>
	{/if}
</div>

<style>
	.sources {
		list-style: none;
		margin: 0;
		padding: 0;
		border-block-start: 1px solid var(--mm-rule);
	}
	.sources > li {
		padding-block: var(--space-3);
		border-block-end: 1px solid var(--mm-rule);
	}
	.row {
		display: flex;
		align-items: center;
		gap: var(--space-3);
	}
</style>
