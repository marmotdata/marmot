<script lang="ts">
	import type { Environment } from '$lib/assets/types';
	import { m } from '$lib/paraglide/messages';
	import '@marmotdata/elements/metadata-viewer.js';

	// One folded row per environment; the first opens so the tab never lands on a wall of closed rows.
	let { environments }: { environments: Record<string, Environment> } = $props();

	let entries = $derived(Object.entries(environments ?? {}));
</script>

{#if entries.length === 0}
	<mm-empty-state
		heading={m.asset_page_env_empty_heading()}
		description={m.asset_environments_empty()}
		compact
	></mm-empty-state>
{:else}
	<ul class="envs" aria-label={m.discover_tab_environments()}>
		{#each entries as [key, env], i (key)}
			<li>
				<mm-disclosure summary={env.name || key} open={i === 0} heading-level={3}>
					<div class="mm-stack" style="--mm-gap: var(--space-3)">
						{#if env.path}
							<mm-text variant="mono-dense" tone="secondary" block>{env.path}</mm-text>
						{/if}
						<mm-metadata-viewer
							label={m.asset_page_env_metadata_label({ name: env.name || key })}
							metadata={env.metadata ?? {}}
							expand-depth={1}
						></mm-metadata-viewer>
					</div>
				</mm-disclosure>
			</li>
		{/each}
	</ul>
{/if}

<style>
	.envs {
		list-style: none;
		margin: 0;
		padding: 0;
		border-block-start: 1px solid var(--mm-rule);
	}
	.envs > li {
		padding-block: var(--space-3);
		border-block-end: 1px solid var(--mm-rule);
	}
</style>
