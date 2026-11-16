<script lang="ts">
	import '@marmotdata/elements/combobox.js';
	import '@marmotdata/elements/fieldset.js';
	import { SvelteMap, SvelteURLSearchParams } from 'svelte/reactivity';
	import type { MmOptionData } from '@marmotdata/elements/select.js';
	import { fetchApi } from '#lib/api.ts';
	import { m } from '#lib/paraglide/messages.js';
	import { typeLabel } from '#components/query/catalogSearch.ts';
	import type { KnowledgeAssetRef } from '#lib/knowledge/api.ts';

	// Links from an entry to other assets: the tables a policy governs, or the older entry a decision replaces. Chosen assets are listed by name with a remove button, and a search field adds more, so nobody has to know an ID.
	let {
		label,
		help = '',
		value,
		max = 100,
		assetType = '',
		exclude = '',
		onchange
	}: {
		label: string;
		help?: string;
		value: KnowledgeAssetRef[];
		/** At most this many links; 1 makes the field pick a single asset. */
		max?: number;
		/** Limits the search to one asset type, such as Decision. */
		assetType?: string;
		/** An asset ID never offered, usually the entry itself. */
		exclude?: string;
		onchange: (value: KnowledgeAssetRef[]) => void;
	} = $props();

	interface SearchResult {
		id: string;
		name: string;
		metadata?: { type?: string; mrn?: string; primary_provider?: string };
	}

	const found = new SvelteMap<string, KnowledgeAssetRef>();
	let picker = $state<HTMLElement & { value: string }>();

	async function load(query: string, signal: AbortSignal): Promise<MmOptionData[]> {
		const params = new SvelteURLSearchParams({ limit: '10', offset: '0' });
		params.append('types[]', 'asset');
		if (query.trim()) params.set('q', query.trim());
		if (assetType) params.set('asset_types', assetType);
		const res = await fetchApi(`/search?${params}`, { signal });
		if (!res.ok) return [];
		const data: { results?: SearchResult[] } = await res.json();
		const taken = new Set(value.map((v) => v.id));
		return (data.results ?? [])
			.filter((r) => r.id !== exclude && !taken.has(r.id))
			.map((r) => {
				const ref = {
					id: r.id,
					name: r.name,
					type: r.metadata?.type ?? '',
					mrn: r.metadata?.mrn ?? ''
				};
				found.set(r.id, ref);
				return {
					value: r.id,
					label: r.name,
					meta: ref.type ? typeLabel(ref.type) : undefined,
					provider: r.metadata?.primary_provider?.toLowerCase()
				};
			});
	}

	function add(id: string) {
		const ref = found.get(id);
		if (!ref || value.some((v) => v.id === id)) return;
		onchange(max === 1 ? [ref] : [...value, ref]);
		if (picker) picker.value = '';
	}

	function remove(id: string) {
		onchange(value.filter((v) => v.id !== id));
	}
</script>

<mm-fieldset {label} description={help} optional>
	<div class="mm-stack" style="--mm-gap: var(--space-2)">
		{#if value.length}
			<ul class="links" aria-label={label}>
				{#each value as v (v.id)}
					<li>
						<span class="name">{v.name}</span>
						{#if v.type}
							<mm-text variant="caption" tone="secondary">{typeLabel(v.type)}</mm-text>
						{/if}
						<mm-button
							variant="ghost"
							size="sm"
							icon="close"
							label={m.knowledge_links_remove({ name: v.name })}
							onclick={() => remove(v.id)}
						></mm-button>
					</li>
				{/each}
			</ul>
		{/if}
		{#if value.length < max || max === 1}
			<mm-combobox
				bind:this={picker}
				label={max === 1 && value.length ? m.knowledge_links_change() : m.knowledge_links_add()}
				hide-label
				placeholder={max === 1 && value.length
					? m.knowledge_links_change()
					: m.knowledge_links_search()}
				loader={load}
				onmm-change={(e: CustomEvent<{ value: string }>) => add(e.detail.value)}
			></mm-combobox>
		{/if}
	</div>
</mm-fieldset>

<style>
	.links {
		list-style: none;
		margin: 0;
		padding: 0;
		border-block-start: 1px solid var(--mm-rule);
	}
	.links li {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding-block: var(--space-1);
		border-block-end: 1px solid var(--mm-rule);
	}
	.name {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
