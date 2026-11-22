<script lang="ts">
	import { untrack } from 'svelte';
	import { SvelteSet, SvelteURLSearchParams } from 'svelte/reactivity';
	import '@marmotdata/elements/facet-sidebar.js';
	import '@marmotdata/elements/link-list.js';
	import '@marmotdata/elements/toolbar.js';
	import type { MmFacetOption } from '@marmotdata/elements/facet-sidebar.js';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { browser } from '$app/env';
	import { setCrumbs } from '#lib/mm/index.ts';
	import { auth } from '#lib/stores/auth.ts';
	import { m } from '#lib/paraglide/messages.js';
	import { formatRelativeTime } from '#lib/utils/format.ts';
	import {
		knowledgeKinds,
		loadKinds,
		kindById,
		type KnowledgeState
	} from '#lib/knowledge/kinds.ts';
	import {
		getCounts,
		listEntries,
		type KnowledgeCounts,
		type KnowledgeSummary
	} from '#lib/knowledge/api.ts';
	import StatusPill from '#components/knowledge/StatusPill.svelte';

	type Sort = 'updated' | 'name' | 'review';

	interface Filters {
		q: string;
		kinds: string[];
		states: string[];
		teams: string[];
		attention: string[];
		sort: Sort;
		page: number;
	}

	const PAGE = 25;
	const SORTS: Sort[] = ['updated', 'name', 'review'];
	const STATES: KnowledgeState[] = ['current', 'pending', 'historical'];

	setCrumbs([
		{ label: m.knowledge_heading(), href: resolve('/knowledge') },
		{ label: m.knowledge_browse_heading() }
	]);

	const canManage = auth.hasPermission('assets', 'manage');

	let rows = $state<KnowledgeSummary[]>([]);
	let total = $state(0);
	let counts = $state<KnowledgeCounts | null>(null);
	let loading = $state(true);
	let failed = $state(false);
	let search = $state('');
	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	let seq = 0;

	// Filters live in the URL, so a filtered view can be shared and the breadcrumb on an entry page can link back to its kind.
	let filters = $derived.by((): Filters => {
		const p = page.url.searchParams;
		const list = (k: string) => (p.get(k) ?? '').split(',').filter(Boolean);
		const sort = p.get('sort') as Sort;
		return {
			q: p.get('q') ?? '',
			kinds: list('kind'),
			states: list('state'),
			teams: list('team'),
			attention: list('show'),
			sort: SORTS.includes(sort) ? sort : 'updated',
			page: Math.max(1, Number(p.get('page')) || 1)
		};
	});

	let filtering = $derived(
		!!filters.q ||
			filters.kinds.length > 0 ||
			filters.states.length > 0 ||
			filters.teams.length > 0 ||
			filters.attention.length > 0
	);
	let onlyKind = $derived(
		filters.kinds.length === 1 ? kindById(filters.kinds[0], $knowledgeKinds) : undefined
	);

	function apply(next: Partial<Filters>) {
		const f = { ...filters, page: 1, ...next };
		const p = new SvelteURLSearchParams();
		if (f.q) p.set('q', f.q);
		if (f.kinds.length) p.set('kind', f.kinds.join(','));
		if (f.states.length) p.set('state', f.states.join(','));
		if (f.teams.length) p.set('team', f.teams.join(','));
		if (f.attention.length) p.set('show', f.attention.join(','));
		if (f.sort !== 'updated') p.set('sort', f.sort);
		if (f.page > 1) p.set('page', String(f.page));
		const qs = p.toString();
		goto(resolve(`/knowledge/browse${qs ? `?${qs}` : ''}` as '/knowledge/browse'), {
			replace: true,
			reset: false
		});
	}

	$effect(() => {
		if (!browser) return;
		loadKinds().catch(() => {});
		getCounts()
			.then((c) => (counts = c))
			.catch(() => {});
	});

	$effect(() => {
		if (!browser) return;
		const f = filters;
		untrack(() => {
			if (search !== f.q && document.activeElement?.tagName !== 'MM-SEARCH-FIELD') search = f.q;
			load(f);
		});
	});

	// A state facet is turned into the statuses that carry it, across every kind, since the API filters on statuses.
	function statusesFor(states: string[]): string[] {
		const out = new SvelteSet<string>();
		for (const k of $knowledgeKinds)
			for (const s of k.statuses) if (states.includes(s.state)) out.add(s.value);
		return [...out];
	}

	async function load(f: Filters) {
		const mine = ++seq;
		loading = true;
		failed = false;
		try {
			// Statuses come from the kinds, so they must have loaded before a state filter can be sent.
			if (f.states.length) await loadKinds();
			const res = await listEntries({
				q: f.q,
				kinds: f.kinds,
				statuses: f.states.length ? statusesFor(f.states) : undefined,
				reviewDue: f.attention.includes('review'),
				mine: f.attention.includes('mine'),
				teams: f.teams,
				sort: f.sort,
				limit: PAGE,
				offset: (f.page - 1) * PAGE
			});
			if (mine !== seq) return;
			rows = res.entries;
			total = res.total;
		} catch {
			if (mine === seq) failed = true;
		} finally {
			if (mine === seq) loading = false;
		}
	}

	function onSearch(value: string) {
		search = value;
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => apply({ q: search.trim() }), 250);
	}

	function onFacets(facets: Record<string, string[]>) {
		apply({
			kinds: facets.kind ?? [],
			states: facets.state ?? [],
			teams: facets.team ?? [],
			attention: facets.show ?? []
		});
	}

	// Counts are across every entry, so they would contradict a filtered list; they show only while nothing is filtered.
	function shownCount(n: number): number | undefined {
		return filtering ? undefined : n;
	}

	let kindOptions = $derived<MmFacetOption[]>(
		$knowledgeKinds.map((k) => ({
			value: k.id,
			label: k.label,
			icon: k.icon,
			count: shownCount(counts?.by_kind[k.id] ?? 0)
		}))
	);
	const stateLabel: Record<KnowledgeState, () => string> = {
		current: m.knowledge_state_current,
		pending: m.knowledge_state_pending,
		historical: m.knowledge_state_historical
	};
	let stateOptions = $derived<MmFacetOption[]>(
		STATES.map((s) => ({
			value: s,
			label: stateLabel[s](),
			count: shownCount(counts?.by_state[s] ?? 0)
		}))
	);
	// Teams that own entries, busiest first, so "what does payments know" is one click.
	let teamOptions = $derived<MmFacetOption[]>(
		(counts?.by_team ?? []).map((t) => ({
			value: t.id,
			label: t.name,
			icon: 'groups',
			count: shownCount(t.count)
		}))
	);
	let showOptions = $derived<MmFacetOption[]>([
		{
			value: 'review',
			label: m.knowledge_show_review_due(),
			icon: 'schedule',
			count: shownCount(counts?.review_due ?? 0)
		},
		{ value: 'mine', label: m.knowledge_show_mine(), icon: 'person' }
	]);

	// One quiet line, as Discover writes it: what it says, then its kind, owners and when it last changed.
	function caption(e: KnowledgeSummary): string {
		const kind = kindById(e.kind, $knowledgeKinds)?.label ?? '';
		return [
			e.summary,
			kind,
			e.owners.map((o) => o.name).join(', '),
			m.knowledge_updated({ date: formatRelativeTime(e.updated_at) })
		]
			.filter(Boolean)
			.join(' · ');
	}
</script>

<svelte:head>
	<title>{m.knowledge_browse_page_title()}</title>
</svelte:head>

{#snippet starters()}
	<mm-link-list columns="2">
		{#each $knowledgeKinds as k (k.id)}
			<mm-link-list-item
				href={`${resolve('/knowledge/new')}?kind=${k.id}`}
				icon={k.icon}
				description={k.description}>{k.label}</mm-link-list-item
			>
		{/each}
	</mm-link-list>
{/snippet}

<div class="mm-stack" style="--mm-gap: var(--space-6)">
	<mm-page-header heading={m.knowledge_browse_heading()} compact count={counts?.total}>
		{#if canManage && counts?.total}
			<mm-menu-button slot="actions" variant="primary" icon="add" label={m.knowledge_new()}>
				<mm-menu
					label={m.knowledge_new()}
					onmm-select={(e: CustomEvent<{ value: string }>) =>
						goto(resolve(`/knowledge/new?kind=${e.detail.value}` as '/knowledge/new'))}
				>
					{#each $knowledgeKinds as k (k.id)}
						<mm-menu-item value={k.id} icon={k.icon}>{k.label}</mm-menu-item>
					{/each}
				</mm-menu>
			</mm-menu-button>
		{/if}
	</mm-page-header>

	{#if counts && counts.total === 0}
		<mm-empty-state
			heading={m.knowledge_empty_title()}
			description={m.knowledge_empty_description()}
			seed={7}
		></mm-empty-state>
		{#if canManage}
			<mm-section heading={m.knowledge_start_heading()}>
				{@render starters()}
			</mm-section>
		{/if}
	{:else}
		<mm-search-field
			class="search"
			label={m.knowledge_search_label()}
			hide-label
			placeholder={m.knowledge_search_hint()}
			shortcut="/"
			value={search}
			onmm-input={(e: CustomEvent<{ value: string }>) => onSearch(e.detail.value)}
		></mm-search-field>

		<div class="mm-split mm-split--sidebar">
			<!-- Each facet already says how many are selected, and a long team name in the applied-filters row ran into the results. -->
			<mm-facet-sidebar
				hide-applied
				heading={m.knowledge_filters_heading()}
				onmm-filter-change={(e: CustomEvent<{ facets: Record<string, string[]> }>) =>
					onFacets(e.detail.facets)}
			>
				<mm-facet
					name="kind"
					label={m.knowledge_facet_kind()}
					options={kindOptions}
					value={filters.kinds}
				></mm-facet>
				<mm-facet
					name="state"
					label={m.knowledge_facet_state()}
					options={stateOptions}
					value={filters.states}
				></mm-facet>
				{#if teamOptions.length}
					<mm-facet
						name="team"
						label={m.knowledge_facet_team()}
						options={teamOptions}
						value={filters.teams}
						limit={6}
						filter-threshold={10}
					></mm-facet>
				{/if}
				<mm-facet
					name="show"
					label={m.knowledge_facet_show()}
					options={showOptions}
					value={filters.attention}
				></mm-facet>
			</mm-facet-sidebar>

			<section
				class="mm-stack"
				style="--mm-gap: var(--space-4)"
				aria-label={m.knowledge_entries_label()}
			>
				{#if onlyKind}
					<mm-text variant="dense" tone="secondary" block>{onlyKind.description}</mm-text>
				{/if}

				{#if failed}
					<mm-callout tone="danger" heading={m.knowledge_load_failed()}>
						<mm-button slot="actions" size="sm" icon="refresh" onclick={() => load(filters)}
							>{m.knowledge_try_again()}</mm-button
						>
					</mm-callout>
				{:else if loading && rows.length === 0}
					<mm-skeleton loading count={6}></mm-skeleton>
				{:else if total === 0}
					<mm-empty-state
						heading={m.knowledge_no_match_title()}
						description={m.knowledge_no_match_description()}
						seed={3}
					>
						{#if filtering}
							<mm-button
								slot="action"
								onclick={() => apply({ q: '', kinds: [], states: [], teams: [], attention: [] })}
								>{m.knowledge_clear_filters()}</mm-button
							>
						{/if}
					</mm-empty-state>
				{:else}
					<div class="mm-cluster mm-cluster--between">
						<mm-text variant="dense" tone="secondary" role="status"
							>{m.knowledge_results({ count: total })}</mm-text
						>
						<mm-select
							style="inline-size: 12rem"
							label={m.knowledge_sort_label()}
							hide-label
							value={filters.sort}
							onmm-change={(e: CustomEvent<{ value: string }>) =>
								apply({ sort: e.detail.value as Sort })}
						>
							<mm-option value="updated">{m.knowledge_sort_updated()}</mm-option>
							<mm-option value="name">{m.knowledge_sort_name()}</mm-option>
							<mm-option value="review">{m.knowledge_sort_review()}</mm-option>
						</mm-select>
					</div>
					<mm-list class="results" aria-label={m.knowledge_entries_label()} aria-busy={loading}>
						{#each rows as e (e.asset_id)}
							{@const kind = kindById(e.kind, $knowledgeKinds)}
							<mm-list-item
								href={resolve('/knowledge/[id]', { id: e.asset_id })}
								title={e.summary || undefined}
								secondary={caption(e)}
								muted={e.state === 'historical'}
							>
								<mm-icon
									slot="start"
									name={kind?.icon ?? 'description'}
									size={20}
									aria-hidden="true"
								></mm-icon>
								{e.name}
								<span slot="badges" class="pills"
									><StatusPill
										{kind}
										status={e.status}
										state={e.state}
										reviewDue={e.review_due}
									/></span
								>
							</mm-list-item>
						{/each}
					</mm-list>
					{#if total > PAGE}
						<mm-pagination
							{total}
							page-size={PAGE}
							page={filters.page}
							label={m.knowledge_pages_label()}
							onmm-page-change={(e: CustomEvent<{ page: number }>) =>
								apply({ page: e.detail.page })}
						></mm-pagination>
					{/if}
				{/if}
			</section>
		</div>
	{/if}
</div>

<style>
	.results {
		border-block: 1px solid var(--mm-rule);
	}
	.pills {
		display: inline-flex;
		flex-wrap: wrap;
		gap: var(--space-2);
	}
	.search {
		width: 100%;
	}
</style>
