<script lang="ts">
	import { untrack } from 'svelte';
	import { SvelteURLSearchParams } from 'svelte/reactivity';
	import '@marmotdata/elements/split-pane.js';
	import '@marmotdata/elements/tree.js';
	import type {
		MmTreeSelectDetail,
		MmTreeExpandDetail
	} from '@marmotdata/elements/classes/tree.js';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { setCrumbs } from '$lib/mm';
	import { toasts, parseApiError, isLimitExceeded } from '$lib/stores/toast';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import { getTerm, listTerms, searchTerms } from '$lib/glossary/api';
	import { ancestorIds, flatNodes, termTree } from '$lib/glossary/tree';
	import type { CreateTermInput, GlossaryTerm } from '$lib/glossary/types';
	import GlossaryTermView from '$components/glossary/GlossaryTermView.svelte';
	import TermForm from '$components/glossary/TermForm.svelte';

	// The list endpoint caps a page at 100, so a glossary is read in pages up to this many terms for the tree and pickers
	const PAGE_SIZE = 100;
	const MAX_TERMS = 1000;

	const canManage = auth.hasPermission('glossary', 'manage');

	let allTerms = $state<GlossaryTerm[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let loaded = $state(false);
	let error = $state('');

	let searchResults = $state<GlossaryTerm[]>([]);
	let searching = $state(false);
	let searchInput = $state(page.url.searchParams.get('q') ?? '');
	let searchTimer: ReturnType<typeof setTimeout> | undefined;

	let selectedTerm = $state<GlossaryTerm | null>(null);
	let expanded = $state<string[]>([]);

	let query = $derived(page.url.searchParams.get('q') ?? '');
	let creating = $derived(page.params.id === 'new');
	let termId = $derived(page.params.id && page.params.id !== 'new' ? page.params.id : null);

	let nodes = $derived(query ? flatNodes(searchResults) : termTree(allTerms));
	// A brand-new glossary gets the welcome page in place of the split workspace
	let empty = $derived(loaded && !error && total === 0 && !query && !creating);

	const glossaryHref = resolve('/glossary/[[id]]', {});

	$effect(() => {
		if (creating) {
			setCrumbs([
				{ label: m.glossary_heading(), href: glossaryHref },
				{ label: m.glossary_new_term_heading() }
			]);
		} else if (selectedTerm) {
			setCrumbs([
				{ label: m.glossary_heading(), href: glossaryHref },
				{ label: selectedTerm.name }
			]);
		} else {
			setCrumbs([{ label: m.glossary_heading() }]);
		}
	});

	function normalise(t: GlossaryTerm): GlossaryTerm {
		return { ...t, tags: t.tags ?? [], metadata: t.metadata ?? {}, owners: t.owners ?? [] };
	}

	async function loadAll() {
		loading = true;
		error = '';
		try {
			const first = await listTerms(0, PAGE_SIZE);
			const terms = [...(first.terms ?? [])];
			const want = Math.min(first.total ?? terms.length, MAX_TERMS);
			const offsets: number[] = [];
			for (let o = PAGE_SIZE; o < want; o += PAGE_SIZE) offsets.push(o);
			const rest = await Promise.all(offsets.map((o) => listTerms(o, PAGE_SIZE)));
			for (const r of rest) terms.push(...(r.terms ?? []));
			allTerms = terms.map(normalise);
			total = first.total ?? terms.length;
			// Open the top level the first time the tree appears, so people see the shape of the glossary
			if (!loaded) expanded = allTerms.filter((t) => !t.parent_term_id).map((t) => t.id);
			loaded = true;
		} catch {
			error = m.glossary_fetch_error();
		} finally {
			loading = false;
		}
	}

	async function runSearch(q: string) {
		if (!q) {
			searchResults = [];
			return;
		}
		searching = true;
		try {
			const res = await searchTerms(q, null, 0, PAGE_SIZE);
			if (q === query) searchResults = (res.terms ?? []).map(normalise);
		} catch {
			if (q === query) error = m.glossary_fetch_error();
		} finally {
			searching = false;
		}
	}

	async function loadSelected(id: string | null) {
		if (!id) {
			selectedTerm = null;
			return;
		}
		// The list copy shows at once; the full term follows with what only a single read carries, such as related terms
		const known = allTerms.find((t) => t.id === id);
		if (known) selectedTerm = known;
		try {
			const t = await getTerm(id);
			if (id === termId) selectedTerm = normalise(t);
		} catch {
			if (id === termId && !known) selectedTerm = null;
		}
	}

	loadAll();

	$effect(() => {
		const q = query;
		untrack(() => {
			searchInput = q;
			runSearch(q);
		});
	});

	$effect(() => {
		const id = termId;
		untrack(() => {
			loadSelected(id);
			if (id) expanded = [...new Set([...expanded, ...ancestorIds(allTerms, id)])];
		});
	});

	// Once the full list arrives, a term opened by URL picks up the loaded copy and its path in the tree opens
	$effect(() => {
		const terms = allTerms;
		untrack(() => {
			if (!termId) return;
			const known = terms.find((t) => t.id === termId);
			if (known && selectedTerm?.id !== known.id) selectedTerm = known;
			expanded = [...new Set([...expanded, ...ancestorIds(terms, termId)])];
		});
	});

	// Keeps the search when moving between terms, and drops the create-only parameters
	function termQuery(): string {
		const params = new SvelteURLSearchParams(page.url.searchParams);
		params.delete('name');
		params.delete('parent');
		return params.toString();
	}

	function onSearchInput(value: string) {
		searchInput = value;
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => commitSearch(value), 300);
	}

	function commitSearch(value: string) {
		clearTimeout(searchTimer);
		const params = new SvelteURLSearchParams(page.url.searchParams);
		if (value.trim()) params.set('q', value.trim());
		else params.delete('q');
		const qs = params.toString();
		goto(resolve(`/glossary/[[id]]?${qs}`, { id: page.params.id }), {
			replaceState: true,
			noScroll: true,
			keepFocus: true
		});
	}

	function onTreeSelect(e: CustomEvent<MmTreeSelectDetail>) {
		const id = e.detail.selected[0];
		// Selecting the current term again would clear the selection; the article stays put instead
		if (!id || id === termId) return;
		goto(resolve(`/glossary/[[id]]?${termQuery()}`, { id }), { noScroll: true, keepFocus: true });
	}

	function onTreeExpand(e: CustomEvent<MmTreeExpandDetail>) {
		const { node, expanded: open } = e.detail;
		expanded = open ? [...expanded, node.id] : expanded.filter((id) => id !== node.id);
	}

	function newTermHref(name = '') {
		const qs = name ? new SvelteURLSearchParams({ name }).toString() : '';
		return resolve(`/glossary/[[id]]?${qs}`, { id: 'new' });
	}

	async function create(input: CreateTermInput): Promise<string | void> {
		try {
			const res = await fetchApi('/glossary/', { method: 'POST', body: JSON.stringify(input) });
			if (!res.ok) {
				const info = await parseApiError(res);
				if (isLimitExceeded(info)) toasts.warning(info.message);
				return info.message || m.glossary_create_error();
			}
			const created = normalise(await res.json());
			allTerms = [...allTerms, created];
			total += 1;
			selectedTerm = created;
			await goto(resolve(`/glossary/[[id]]?${termQuery()}`, { id: created.id }), {
				replaceState: true
			});
			loadAll();
		} catch (e) {
			return e instanceof Error ? e.message : m.glossary_create_error();
		}
	}

	function onUpdated(updated: GlossaryTerm) {
		selectedTerm = updated;
		allTerms = allTerms.map((t) => (t.id === updated.id ? updated : t));
		searchResults = searchResults.map((t) => (t.id === updated.id ? updated : t));
	}

	async function onDeleted() {
		selectedTerm = null;
		await goto(resolve(`/glossary/[[id]]?${termQuery()}`, {}), { replaceState: true });
		await loadAll();
		if (query) runSearch(query);
	}

	const starters = [
		{
			name: m.glossary_starter_active_customer,
			summary: m.glossary_starter_active_customer_summary
		},
		{ name: m.glossary_starter_gmv, summary: m.glossary_starter_gmv_summary },
		{ name: m.glossary_starter_net_revenue, summary: m.glossary_starter_net_revenue_summary },
		{ name: m.glossary_starter_pii, summary: m.glossary_starter_pii_summary },
		{ name: m.glossary_starter_churned, summary: m.glossary_starter_churned_summary }
	];
</script>

<svelte:head>
	<title
		>{selectedTerm?.name
			? m.glossary_term_page_title({ name: selectedTerm.name })
			: m.glossary_page_title()}</title
	>
</svelte:head>

{#if empty}
	<div class="mm-page mm-stack glossary-empty">
		<mm-page-header heading={m.glossary_heading()} compact count={0}>
			{#if canManage}
				<mm-button slot="actions" variant="primary" icon="add" href={newTermHref()}
					>{m.glossary_create_term_button()}</mm-button
				>
			{/if}
		</mm-page-header>
		<mm-empty-state
			heading={m.glossary_empty_heading()}
			description={m.glossary_empty_description()}
			seed={11}
		></mm-empty-state>
		{#if canManage}
			<mm-panel heading={m.glossary_starter_heading()} icon="menu-book">
				<mm-list>
					{#each starters as s (s.name())}
						<mm-list-item secondary={s.summary()}>
							{s.name()}
							<mm-button
								slot="end"
								size="sm"
								variant="ghost"
								icon="add"
								label={m.glossary_starter_add_label({ name: s.name() })}
								href={newTermHref(s.name())}>{m.glossary_starter_add()}</mm-button
							>
						</mm-list-item>
					{/each}
				</mm-list>
			</mm-panel>
		{/if}
	</div>
{:else}
	<mm-split-pane
		label={m.glossary_split_label()}
		default-position={320}
		min-start={240}
		min-end={480}
		storage-key="marmot.glossary.split"
		style="height: 100%"
	>
		<div slot="start" class="mm-stack glossary-list" style="--mm-gap: var(--space-3)">
			<div class="mm-cluster mm-cluster--between">
				<h2 class="mm-heading-sm">
					{m.glossary_terms_heading()}
					{#if !error && loaded}<span class="mm-count">{total}</span>{/if}
				</h2>
				{#if canManage}
					<mm-button variant="primary" size="sm" icon="add" href={newTermHref()}
						>{m.glossary_create_term_button()}</mm-button
					>
				{/if}
			</div>
			<mm-search-field
				label={m.glossary_search_label()}
				hide-label
				placeholder={m.glossary_search_label()}
				shortcut="/"
				value={searchInput}
				onmm-input={(e: CustomEvent<{ value: string }>) => onSearchInput(e.detail.value)}
				onmm-search={(e: CustomEvent<{ value: string }>) => commitSearch(e.detail.value)}
			></mm-search-field>
			<mm-tree
				style="--mm-tree-height: 100%; flex: 1; min-height: 0"
				label={m.glossary_tree_label()}
				{nodes}
				expandedIds={expanded}
				selectedIds={termId ? [termId] : []}
				loading={(loading && !loaded) || searching}
				{error}
				empty-text={query ? m.glossary_no_terms_found() : m.glossary_no_terms_yet()}
				onmm-select={onTreeSelect}
				onmm-expand={onTreeExpand}
				onmm-retry={() => (query ? runSearch(query) : loadAll())}
			></mm-tree>
		</div>

		<div slot="end" class="glossary-pane">
			{#if creating}
				{#if canManage}
					{#key page.url.search}
						<TermForm
							{allTerms}
							initialName={page.url.searchParams.get('name') ?? ''}
							initialParent={page.url.searchParams.get('parent') ?? ''}
							cancelHref={glossaryHref}
							onSubmit={create}
						/>
					{/key}
				{:else}
					<div class="mm-page">
						<mm-empty-state
							heading={m.glossary_no_permission_heading()}
							description={m.glossary_no_permission()}
							seed={7}
						></mm-empty-state>
					</div>
				{/if}
			{:else if selectedTerm}
				<GlossaryTermView term={selectedTerm} {allTerms} {canManage} {onUpdated} {onDeleted} />
			{:else}
				<div class="mm-page">
					<mm-empty-state
						heading={m.glossary_choose_heading()}
						description={m.glossary_choose_hint()}
						seed={7}
					></mm-empty-state>
				</div>
			{/if}
		</div>
	</mm-split-pane>
{/if}

<style>
	.glossary-list {
		padding: var(--space-4);
		height: 100%;
		box-sizing: border-box;
	}
	.glossary-pane {
		height: 100%;
		overflow-y: auto;
	}
	.glossary-empty {
		max-width: 72rem;
	}
</style>
