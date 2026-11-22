<script lang="ts">
	import { untrack } from 'svelte';
	import '@marmotdata/elements/toolbar.js';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { browser } from '$app/env';
	import { fetchApi } from '#lib/api.ts';
	import { setCrumbs } from '#lib/mm/index.ts';
	import { m } from '#lib/paraglide/messages.js';
	import { discoverPath } from '#lib/mrn.ts';
	import { typeLabel } from '#components/query/catalogSearch.ts';
	import {
		kindOfType,
		knowledgeKinds,
		knowledgeRelations,
		loadKinds,
		relationOf
	} from '#lib/knowledge/kinds.ts';
	import { entryLinks, type KnowledgeAssetRef, type KnowledgeLink } from '#lib/knowledge/api.ts';
	import AssetRefIcon from '#components/knowledge/AssetRefIcon.svelte';

	// Every asset one entry is linked to, for entries that cover more than the side column can show. Links page in by name, a hundred at a time.
	const PAGE = 100;

	let id = $derived(page.params.id ?? '');
	let name = $state('');
	let relation = $state('');
	let links = $state<KnowledgeLink[]>([]);
	let total = $state(0);
	let next = $state('');
	let loading = $state(true);
	let loadingMore = $state(false);
	let failed = $state(false);
	let seq = 0;

	$effect(() => {
		setCrumbs([
			{ label: m.knowledge_heading(), href: resolve('/knowledge') },
			{ label: name || m.knowledge_entry_loading(), href: resolve('/knowledge/[id]', { id }) },
			{ label: m.knowledge_group_links() }
		]);
	});

	$effect(() => {
		if (!browser) return;
		const current = id;
		untrack(() => {
			loadKinds().catch(() => {});
			fetchApi(`/assets/${encodeURIComponent(current)}`)
				.then(async (r) => (r.ok ? (name = (await r.json()).name ?? '') : null))
				.catch(() => {});
		});
	});

	$effect(() => {
		if (!browser) return;
		const params = { id, relation };
		untrack(() => load(params.id, params.relation, ''));
	});

	async function load(entry: string, rel: string, cursor: string) {
		const mine = ++seq;
		if (cursor) loadingMore = true;
		else loading = true;
		failed = false;
		try {
			const res = await entryLinks(entry, cursor, PAGE, rel);
			if (mine !== seq) return;
			links = cursor ? [...links, ...res.links] : res.links;
			total = res.total;
			next = res.next ?? '';
		} catch {
			if (mine === seq) failed = true;
		} finally {
			if (mine === seq) {
				loading = false;
				loadingMore = false;
			}
		}
	}

	// A path for a linked asset: its knowledge page for an entry, its catalog page otherwise. Callers resolve it.
	function pathOf(r: KnowledgeAssetRef): string {
		if (kindOfType(r.type, $knowledgeKinds)) return `/knowledge/${encodeURIComponent(r.id)}`;
		return discoverPath(r.mrn) ?? '/discover';
	}
</script>

<svelte:head>
	<title>{m.knowledge_links_page_title({ name })}</title>
</svelte:head>

<div class="mm-stack" style="--mm-gap: var(--space-5)">
	<mm-page-header
		heading={m.knowledge_links_page_heading({ name })}
		compact
		count={loading ? undefined : total}
		back-href={resolve('/knowledge/[id]', { id })}
		back-label={name}
	></mm-page-header>

	<mm-toolbar label={m.knowledge_filter_label()}>
		<mm-select
			class="relation"
			label={m.knowledge_link_relation_new()}
			hide-label
			value={relation}
			onmm-change={(e: CustomEvent<{ value: string }>) => (relation = e.detail.value)}
		>
			<mm-option value="">{m.knowledge_links_all_relations()}</mm-option>
			{#each $knowledgeRelations as r (r.id)}
				<mm-option value={r.id}>{r.label}</mm-option>
			{/each}
		</mm-select>
	</mm-toolbar>

	{#if failed}
		<mm-callout tone="danger" heading={m.knowledge_load_failed()}>
			<mm-button slot="actions" size="sm" icon="refresh" onclick={() => load(id, relation, '')}
				>{m.knowledge_try_again()}</mm-button
			>
		</mm-callout>
	{:else if loading && links.length === 0}
		<mm-skeleton loading count={8}></mm-skeleton>
	{:else if total === 0}
		<mm-empty-state
			heading={m.knowledge_links_none_title()}
			description={m.knowledge_links_empty()}
			seed={2}
		></mm-empty-state>
	{:else}
		<ul class="mm-list rows" aria-label={m.knowledge_group_links()}>
			{#each links as l (l.relation + l.target.id)}
				<li>
					<AssetRefIcon asset={l.target} size={20} />
					<a class="mm-link mm-link--quiet name" href={resolve(pathOf(l.target) as '/')}
						>{l.target.name}</a
					>
					<mm-text variant="caption" tone="secondary">{typeLabel(l.target.type)}</mm-text>
					<span class="relation-label">
						<mm-text variant="caption" tone="secondary"
							>{relationOf(l.relation, $knowledgeRelations)?.label ?? l.relation}</mm-text
						>
					</span>
				</li>
			{/each}
		</ul>
		{#if next}
			<div>
				<mm-button loading={loadingMore} onclick={() => load(id, relation, next)}
					>{m.knowledge_about_load_more({ shown: links.length, total })}</mm-button
				>
			</div>
		{/if}
	{/if}
</div>

<style>
	.relation {
		width: 14rem;
	}
	.rows {
		border-block: 1px solid var(--mm-rule);
	}
	.rows > li {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding-block: var(--space-2);
	}
	.name {
		font: var(--text-body-strong);
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.relation-label {
		margin-inline-start: auto;
	}
</style>
