<script lang="ts">
	import { m } from '$lib/paraglide/messages';
	import type { PluginInfo } from './pipeline';

	let { plugin }: { plugin: PluginInfo } = $props();
</script>

<!-- The chosen plugin beside the form, leading with querying because it is what people connect sources for -->
<div class="mm-cluster" style="--mm-gap: var(--space-2)">
	<mm-provider-logo provider={plugin.icon || plugin.id} size="24" decorative></mm-provider-logo>
	<mm-heading level={2} variant="heading-sm">{plugin.name}</mm-heading>
</div>
{#if plugin.description}
	<mm-text block variant="dense" tone="secondary">{plugin.description}</mm-text>
{/if}
{#if plugin.supports_query}
	<div
		class="mm-cluster"
		style="--mm-gap: var(--space-2); align-items: flex-start; flex-wrap: nowrap"
	>
		<mm-icon name="terminal" aria-hidden="true" style="margin-block-start: 2px"></mm-icon>
		<mm-text block variant="dense"
			><strong>{m.pipelines_answers_queries_lead()}</strong>
			{m.pipelines_answers_queries_body()}</mm-text
		>
	</div>
{/if}
