<script lang="ts">
	import { untrack } from 'svelte';
	import { SvelteURLSearchParams } from 'svelte/reactivity';
	import '@marmotdata/elements/toolbar.js';
	import { resolve } from '$app/paths';
	import { browser } from '$app/env';
	import { fetchApi } from '#lib/api.ts';
	import { setCrumbs } from '#lib/mm/index.ts';
	import { auth } from '#lib/stores/auth.ts';
	import { m } from '#lib/paraglide/messages.js';
	import { formatRelativeTime } from '#lib/utils/format.ts';
	import { knowledgeKindOf, knowledgeKinds } from '#lib/knowledge/kinds.ts';

	interface Entry {
		id: string;
		name: string;
		description?: string;
		url: string;
		updated_at?: string;
		metadata?: { type?: string };
		owners?: { id: string; name: string }[];
	}

	const PAGE = 25;

	setCrumbs([{ label: m.knowledge_heading() }]);

	const canManage = auth.hasPermission('assets', 'manage');

	let rows = $state<Entry[]>([]);
	let total = $state(0);
	/** How many entries exist with no filter applied, so an empty catalog reads differently from an empty search. */
	let allTotal = $state<number | null>(null);
	let loading = $state(true);
	let failed = $state(false);

	let search = $state('');
	let query = $state('');
	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	let kindFilter = $state('');
	let page = $state(1);
	let seq = 0;

	let filtering = $derived(!!query || !!kindFilter);

	$effect(() => {
		if (!browser) return;
		const params = { q: query, kind: kindFilter, page };
		untrack(() => load(params));
	});

	// Entries are assets of the knowledge types, so the list is the catalog search narrowed to those types.
	async function load(p: { q: string; kind: string; page: number }) {
		const mine = ++seq;
		loading = true;
		failed = false;
		const types = knowledgeKinds.filter((k) => !p.kind || k.id === p.kind).map((k) => k.type);
		const params = new SvelteURLSearchParams({
			limit: String(PAGE),
			offset: String((p.page - 1) * PAGE),
			asset_types: types.join(',')
		});
		params.append('types[]', 'asset');
		if (p.q) params.append('q', p.q);
		else params.append('sort', 'updated');
		try {
			const res = await fetchApi(`/search?${params}`);
			if (mine !== seq) return;
			if (!res.ok) {
				failed = true;
				return;
			}
			const data: { results?: Entry[]; total?: number } = await res.json();
			if (mine !== seq) return;
			rows = data.results ?? [];
			total = data.total ?? rows.length;
			if (!p.q && !p.kind) allTotal = total;
		} catch {
			if (mine === seq) failed = true;
		} finally {
			if (mine === seq) loading = false;
		}
	}

	function onSearch(value: string) {
		search = value;
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => {
			query = search.trim();
			page = 1;
		}, 250);
	}

	function clearFilters() {
		search = '';
		query = '';
		kindFilter = '';
		page = 1;
	}

	function caption(e: Entry): string {
		const parts = [knowledgeKindOf(e.metadata?.type)?.label() ?? e.metadata?.type ?? ''];
		for (const o of e.owners ?? []) parts.push(o.name);
		if (e.updated_at) parts.push(m.knowledge_updated({ date: formatRelativeTime(e.updated_at) }));
		return parts.filter(Boolean).join(' · ');
	}
</script>

<svelte:head>
	<title>{m.knowledge_page_title()}</title>
</svelte:head>

{#snippet starters()}
	<mm-list label={m.knowledge_kinds_label()}>
		{#each knowledgeKinds as k (k.id)}
			<mm-list-item href={`${resolve('/knowledge/new')}?kind=${k.id}`} secondary={k.description()}
				><mm-icon slot="start" name={k.icon} aria-hidden="true"></mm-icon>{k.label()}</mm-list-item
			>
		{/each}
	</mm-list>
{/snippet}

<div class="mm-stack" style="--mm-gap: var(--space-5)">
	<mm-page-header
		heading={m.knowledge_heading()}
		description={m.knowledge_description()}
		count={failed || allTotal === null ? undefined : allTotal}
	>
		{#if canManage && allTotal !== 0}
			<mm-button slot="actions" variant="primary" icon="add" href={resolve('/knowledge/new')}
				>{m.knowledge_new()}</mm-button
			>
		{/if}
	</mm-page-header>

	{#if !loading && !failed && allTotal === 0 && !filtering}
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
		<mm-toolbar label={m.knowledge_filter_label()}>
			<mm-search-field
				class="search"
				label={m.knowledge_search_label()}
				hide-label
				placeholder={m.knowledge_search_hint()}
				shortcut="/"
				value={search}
				onmm-input={(e: CustomEvent<{ value: string }>) => onSearch(e.detail.value)}
			></mm-search-field>
			<mm-select
				class="kind"
				label={m.knowledge_new_kind_label()}
				hide-label
				value={kindFilter}
				onmm-change={(e: CustomEvent<{ value: string }>) => {
					kindFilter = e.detail.value;
					page = 1;
				}}
			>
				<mm-option value="">{m.knowledge_any_kind()}</mm-option>
				{#each knowledgeKinds as k (k.id)}
					<mm-option value={k.id} icon={k.icon}>{k.label()}</mm-option>
				{/each}
			</mm-select>
		</mm-toolbar>

		<div>
			{#if failed}
				<mm-callout tone="danger" heading={m.knowledge_load_failed()}>
					<mm-button
						slot="actions"
						size="sm"
						icon="refresh"
						onclick={() => load({ q: query, kind: kindFilter, page })}
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
					<mm-button slot="action" onclick={clearFilters}>{m.knowledge_clear_filters()}</mm-button>
				</mm-empty-state>
			{:else}
				<div class="mm-stack" style="--mm-gap: var(--space-3)">
					<ul class="mm-list entries" aria-label={m.knowledge_entries_label()}>
						{#each rows as e (e.id)}
							<li>
								<mm-icon
									name={knowledgeKindOf(e.metadata?.type)?.icon ?? 'description'}
									size={20}
									aria-hidden="true"
								></mm-icon>
								<div class="mm-stack body">
									<a class="mm-link mm-link--quiet name" href={resolve(e.url as '/')}>{e.name}</a>
									{#if e.description}
										<mm-text variant="dense" tone="secondary" block class="truncate"
											>{e.description}</mm-text
										>
									{/if}
									<mm-text variant="caption" tone="secondary" block class="truncate"
										>{caption(e)}</mm-text
									>
								</div>
							</li>
						{/each}
					</ul>
					{#if total > PAGE}
						<mm-pagination
							{total}
							page-size={PAGE}
							{page}
							label={m.knowledge_pages_label()}
							onmm-page-change={(e: CustomEvent<{ page: number }>) => (page = e.detail.page)}
						></mm-pagination>
					{/if}
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.search {
		width: 20rem;
		max-width: 100%;
	}
	.kind {
		width: 14rem;
	}
	.entries {
		border-block: 1px solid var(--mm-rule);
	}
	.entries > li {
		gap: var(--space-4);
		padding-block: var(--space-4);
	}
	.body {
		--mm-gap: var(--space-1);
		flex: 1;
		min-width: 0;
	}
	.name {
		font: var(--text-heading-sm);
	}
	.truncate {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
