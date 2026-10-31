<script lang="ts">
	import { resolve } from '$app/paths';
	import { loadTerms } from '$lib/assetrules/terms';
	import type { ExternalLink } from '$lib/assetrules/types';
	import type { GlossaryTerm } from '$lib/glossary/types';
	import { m } from '$lib/paraglide/messages';

	// What a rule adds, by name: glossary terms as tags and links with their own external mark.
	let { termIds = [], links = [] }: { termIds?: string[]; links?: ExternalLink[] } = $props();

	let terms = $state<GlossaryTerm[]>([]);

	$effect(() => {
		const ids = termIds;
		let stale = false;
		loadTerms(ids).then((t) => {
			if (!stale) terms = t;
		});
		return () => {
			stale = true;
		};
	});
</script>

{#if terms.length || links.length}
	<span class="mm-cluster" style="--mm-gap: var(--space-2)">
		{#each terms as term (term.id)}
			<mm-tag href={resolve('/glossary/[[id]]', { id: term.id })}>{term.name}</mm-tag>
		{/each}
		{#each links as link (link.url)}
			<mm-link href={link.url} external>{link.name}</mm-link>
		{/each}
	</span>
{:else if !termIds.length}
	<mm-text variant="dense" tone="tertiary">{m.assetrules_adds_nothing()}</mm-text>
{/if}
