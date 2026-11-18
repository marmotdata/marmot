<script lang="ts">
	import { untrack } from 'svelte';
	import '@marmotdata/elements/facet-sidebar.js';
	import type { MmFacetOption } from '@marmotdata/elements/facet-sidebar.js';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { browser } from '$app/env';
	import { fetchApi } from '#lib/api.ts';
	import { setCrumbs } from '#lib/mm/index.ts';
	import { m } from '#lib/paraglide/messages.js';
	import { discoverPath, discoverPathname } from '#lib/mrn.ts';
	import { typeLabel } from '#components/query/catalogSearch.ts';
	import {
		kindById,
		knowledgeKinds,
		loadKinds,
		type KnowledgeState
	} from '#lib/knowledge/kinds.ts';
	import {
		knowledgeAbout,
		type KnowledgeAboutItem,
		type KnowledgeAboutPage,
		type KnowledgeSource
	} from '#lib/knowledge/api.ts';
	import { ruleLabel } from '#lib/knowledge/rules.ts';
	import StatusPill from '#components/knowledge/StatusPill.svelte';

	// Everything the company knows about one asset, for when the asset page's panel shows only the first few. Entries in use come first; the rest page in as needed, so a table with tens of thousands of linked decisions reads like one with ten.
	const PAGE = 50;
	const STATES: KnowledgeState[] = ['current', 'pending', 'historical'];
	const SOURCES: KnowledgeSource[] = ['direct', 'rule', 'inherited'];

	interface AssetInfo {
		id: string;
		name: string;
		type: string;
		mrn?: string;
	}

	let id = $derived(page.params.id ?? '');
	let asset = $state<AssetInfo | null>(null);
	let all = $state<KnowledgeAboutPage | null>(null);
	let items = $state<KnowledgeAboutItem[]>([]);
	let total = $state(0);
	let next = $state('');
	let loading = $state(true);
	let loadingMore = $state(false);
	let failed = $state(false);
	let kinds = $state<string[]>([]);
	let states = $state<string[]>([]);
	let sources = $state<string[]>([]);
	let seq = 0;

	$effect(() => {
		setCrumbs([
			{ label: m.knowledge_heading(), href: resolve('/knowledge') },
			{ label: asset ? m.knowledge_about_crumb({ name: asset.name }) : m.knowledge_entry_loading() }
		]);
	});

	// The asset and the unfiltered totals load once per asset; the totals label the filters, so they do not shrink as filters are applied.
	$effect(() => {
		if (!browser) return;
		const current = id;
		untrack(() => {
			asset = null;
			all = null;
			kinds = [];
			states = [];
			sources = [];
			loadKinds().catch(() => {});
			fetchApi(`/assets/${encodeURIComponent(current)}`)
				.then(async (r) => (r.ok ? (asset = await r.json()) : null))
				.catch(() => {});
			knowledgeAbout(current, { limit: 1 })
				.then((p) => (all = p))
				.catch(() => {});
		});
	});

	$effect(() => {
		if (!browser) return;
		const params = { id, kinds: [...kinds], states: [...states], sources: [...sources] };
		untrack(() => load(params, ''));
	});

	async function load(
		p: { id: string; kinds: string[]; states: string[]; sources: string[] },
		cursor: string
	) {
		const mine = ++seq;
		if (cursor) loadingMore = true;
		else loading = true;
		failed = false;
		try {
			const res = await knowledgeAbout(p.id, {
				kinds: p.kinds,
				states: p.states as KnowledgeState[],
				sources: p.sources as KnowledgeSource[],
				cursor,
				limit: PAGE
			});
			if (mine !== seq) return;
			items = cursor ? [...items, ...res.entries] : res.entries;
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

	const stateLabel: Record<KnowledgeState, () => string> = {
		current: m.knowledge_state_current,
		pending: m.knowledge_state_pending,
		historical: m.knowledge_state_historical
	};
	const sourceLabel: Record<KnowledgeSource, () => string> = {
		direct: m.knowledge_source_direct,
		rule: m.knowledge_source_rule,
		inherited: m.knowledge_source_inherited
	};

	let kindOptions = $derived<MmFacetOption[]>(
		$knowledgeKinds
			.filter((k) => (all?.by_kind[k.id] ?? 0) > 0)
			.map((k) => ({ value: k.id, label: k.label, icon: k.icon, count: all?.by_kind[k.id] ?? 0 }))
	);
	let stateOptions = $derived<MmFacetOption[]>(
		STATES.map((s) => ({ value: s, label: stateLabel[s]() }))
	);
	let sourceOptions = $derived<MmFacetOption[]>(
		SOURCES.filter((s) => (all?.by_source[s] ?? 0) > 0).map((s) => ({
			value: s,
			label: sourceLabel[s](),
			count: all?.by_source[s] ?? 0
		}))
	);

	function note(item: KnowledgeAboutItem): string {
		const kind = kindById(item.kind, $knowledgeKinds)?.label ?? '';
		if (item.source === 'inherited' && item.via)
			return [kind, m.knowledge_about_via({ name: item.via.name })].join(' · ');
		if (item.source === 'rule' && item.rule) return [kind, ruleLabel(item.rule)].join(' · ');
		return [kind, item.summary].filter(Boolean).join(' · ');
	}

	let assetHref = $derived.by(() => {
		const path = discoverPath(asset?.mrn);
		return path ? resolve(discoverPathname(path)) : '';
	});
</script>

<svelte:head>
	<title
		>{asset ? m.knowledge_about_page_title({ name: asset.name }) : m.knowledge_page_title()}</title
	>
</svelte:head>

<div class="mm-stack" style="--mm-gap: var(--space-6)">
	<mm-page-header
		heading={asset ? m.knowledge_about_heading({ name: asset.name }) : m.knowledge_heading()}
		caption={asset ? typeLabel(asset.type) : undefined}
		description={m.knowledge_about_description()}
	>
		{#if assetHref}
			<mm-button slot="actions" variant="secondary" href={assetHref}
				>{m.knowledge_about_open_asset()}</mm-button
			>
		{/if}
	</mm-page-header>

	{#if all && all.total === 0}
		<mm-empty-state
			heading={m.knowledge_about_empty_title()}
			description={m.knowledge_about_empty_description()}
			seed={4}
		></mm-empty-state>
	{:else}
		<div class="mm-split mm-split--sidebar">
			<mm-facet-sidebar
				heading={m.knowledge_filters_heading()}
				onmm-filter-change={(e: CustomEvent<{ facets: Record<string, string[]> }>) => {
					kinds = e.detail.facets.kind ?? [];
					states = e.detail.facets.state ?? [];
					sources = e.detail.facets.source ?? [];
				}}
			>
				<mm-facet name="kind" label={m.knowledge_facet_kind()} options={kindOptions} value={kinds}
				></mm-facet>
				<mm-facet
					name="state"
					label={m.knowledge_facet_state()}
					options={stateOptions}
					value={states}
				></mm-facet>
				{#if sourceOptions.length > 1}
					<mm-facet
						name="source"
						label={m.knowledge_facet_source()}
						options={sourceOptions}
						value={sources}
					></mm-facet>
				{/if}
			</mm-facet-sidebar>

			<section
				class="mm-stack"
				style="--mm-gap: var(--space-4)"
				aria-label={m.knowledge_entries_label()}
			>
				{#if failed}
					<mm-callout tone="danger" heading={m.knowledge_load_failed()}>
						<mm-button
							slot="actions"
							size="sm"
							icon="refresh"
							onclick={() => load({ id, kinds, states, sources }, '')}
							>{m.knowledge_try_again()}</mm-button
						>
					</mm-callout>
				{:else if loading && items.length === 0}
					<mm-skeleton loading count={6}></mm-skeleton>
				{:else if total === 0}
					<mm-empty-state
						heading={m.knowledge_no_match_title()}
						description={m.knowledge_no_match_description()}
						seed={3}
					></mm-empty-state>
				{:else}
					<mm-text variant="dense" tone="secondary" role="status"
						>{m.knowledge_results({ count: total })}</mm-text
					>
					<ul class="mm-list entries" aria-label={m.knowledge_entries_label()} aria-busy={loading}>
						{#each items as e (e.asset_id)}
							{@const kind = kindById(e.kind, $knowledgeKinds)}
							<li class:historical={e.state === 'historical'}>
								<mm-icon name={kind?.icon ?? 'description'} size={20} aria-hidden="true"></mm-icon>
								<div class="body">
									<div class="mm-cluster" style="--mm-gap: var(--space-2)">
										<a
											class="mm-link mm-link--quiet name"
											href={resolve('/knowledge/[id]', { id: e.asset_id })}>{e.name}</a
										>
										<StatusPill {kind} status={e.status} state={e.state} reviewDue={e.review_due} />
									</div>
									<mm-text variant="dense" tone="secondary" block class="truncate"
										>{note(e)}</mm-text
									>
								</div>
							</li>
						{/each}
					</ul>
					{#if next}
						<div>
							<mm-button
								loading={loadingMore}
								onclick={() => load({ id, kinds, states, sources }, next)}
								>{m.knowledge_about_load_more({ shown: items.length, total })}</mm-button
							>
						</div>
					{/if}
				{/if}
			</section>
		</div>
	{/if}
</div>

<style>
	.entries {
		border-block: 1px solid var(--mm-rule);
	}
	.entries > li {
		display: flex;
		align-items: center;
		gap: var(--space-4);
		padding-block: var(--space-3);
	}
	.entries > li.historical .name {
		color: var(--mm-text-2);
	}
	.body {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
		flex: 1;
		min-width: 0;
	}
	.name {
		font: var(--text-body-strong);
	}
	.truncate {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
