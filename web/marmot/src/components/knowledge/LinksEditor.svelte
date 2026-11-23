<script lang="ts">
	import '@marmotdata/elements/combobox.js';
	import '@marmotdata/elements/fieldset.js';
	import { SvelteMap, SvelteURLSearchParams } from 'svelte/reactivity';
	import type { MmOptionData } from '@marmotdata/elements/select.js';
	import { fetchApi } from '#lib/api.ts';
	import { m } from '#lib/paraglide/messages.js';
	import { typeLabel } from '#components/query/catalogSearch.ts';
	import { knowledgeRelations } from '#lib/knowledge/kinds.ts';
	import type { KnowledgeAssetRef, KnowledgeLink } from '#lib/knowledge/api.ts';

	// The assets an entry is linked to, each with how it relates: a policy applies to a table, a decision documents a pipeline. Anything in the catalog can be linked, found by name rather than ID.
	let {
		value,
		exclude = '',
		onchange
	}: {
		value: KnowledgeLink[];
		/** The entry's own asset ID, never offered. */
		exclude?: string;
		onchange: (value: KnowledgeLink[]) => void;
	} = $props();

	interface SearchResult {
		id: string;
		name: string;
		metadata?: { type?: string; mrn?: string; primary_provider?: string };
	}

	const found = new SvelteMap<string, KnowledgeAssetRef>();
	let picker = $state<HTMLElement & { value: string }>();
	let relation = $state('applies_to');

	async function load(query: string, signal: AbortSignal): Promise<MmOptionData[]> {
		const params = new SvelteURLSearchParams({ limit: '10', offset: '0' });
		params.append('types[]', 'asset');
		if (query.trim()) params.set('q', query.trim());
		const res = await fetchApi(`/search?${params}`, { signal });
		if (!res.ok) return [];
		const data: { results?: SearchResult[] } = await res.json();
		const taken = new Set(value.filter((v) => v.relation === relation).map((v) => v.target.id));
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
		const target = found.get(id);
		if (!target || value.some((v) => v.target.id === id && v.relation === relation)) return;
		onchange([...value, { relation, target }]);
		if (picker) picker.value = '';
	}

	function setRelation(i: number, rel: string) {
		const next = value.map((v) => ({ ...v }));
		next[i].relation = rel;
		onchange(
			next.filter(
				(v, j) =>
					next.findIndex((w) => w.relation === v.relation && w.target.id === v.target.id) === j
			)
		);
	}

	function remove(i: number) {
		onchange(value.filter((_, j) => j !== i));
	}
</script>

<mm-fieldset label={m.knowledge_group_links()} description={m.knowledge_links_help()} optional>
	<div class="mm-stack" style="--mm-gap: var(--space-2)">
		{#if value.length}
			<ul class="links" aria-label={m.knowledge_group_links()}>
				{#each value as v, i (v.relation + v.target.id)}
					<li>
						<mm-select
							class="relation"
							label={m.knowledge_link_relation_for({ name: v.target.name })}
							hide-label
							value={v.relation}
							onmm-change={(e: CustomEvent<{ value: string }>) => setRelation(i, e.detail.value)}
						>
							{#each $knowledgeRelations as r (r.id)}
								<mm-option value={r.id} label={r.label}></mm-option>
							{/each}
						</mm-select>
						<span class="name">{v.target.name}</span>
						{#if v.target.type}
							<mm-text variant="caption" tone="secondary">{typeLabel(v.target.type)}</mm-text>
						{/if}
						<mm-button
							variant="ghost"
							size="sm"
							icon="close"
							label={m.knowledge_links_remove({ name: v.target.name })}
							onclick={() => remove(i)}
						></mm-button>
					</li>
				{/each}
			</ul>
		{/if}
		<div class="add">
			<mm-select
				class="relation"
				label={m.knowledge_link_relation_new()}
				hide-label
				value={relation}
				onmm-change={(e: CustomEvent<{ value: string }>) => (relation = e.detail.value)}
			>
				{#each $knowledgeRelations as r (r.id)}
					<mm-option value={r.id} label={r.label} description={r.description}></mm-option>
				{/each}
			</mm-select>
			<mm-combobox
				bind:this={picker}
				label={m.knowledge_links_add()}
				hide-label
				placeholder={m.knowledge_links_search()}
				loader={load}
				onmm-change={(e: CustomEvent<{ value: string }>) => add(e.detail.value)}
			></mm-combobox>
		</div>
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
	.relation {
		width: 10rem;
		flex: none;
	}
	.name {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.add {
		display: grid;
		grid-template-columns: 10rem minmax(0, 1fr);
		gap: var(--space-2);
	}
</style>
