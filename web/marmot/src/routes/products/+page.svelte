<script lang="ts">
	import { untrack } from 'svelte';
	import '@marmotdata/elements/multiselect.js';
	import '@marmotdata/elements/popularity.js';
	import '@marmotdata/elements/toolbar.js';
	import { resolve } from '$app/paths';
	import { browser } from '$app/environment';
	import { fetchApi } from '$lib/api';
	import { setCrumbs } from '$lib/mm';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import type { DataProduct, ProductFacets, ProductSort } from '$lib/dataproducts/types';
	import { getProductFacets, queryDataProducts } from '$lib/dataproducts/api';
	import { normaliseProduct, popularityLevel } from '$lib/dataproducts/assets';
	import ProductBlade from '$components/product/ProductBlade.svelte';
	import ProductIcon from '$components/product/ProductIcon.svelte';

	const PAGE = 25;
	const YOURS_MAX = 5;

	setCrumbs([{ label: m.products_heading() }]);

	// Searching, filtering, sorting and paging all happen on the server, so the page never reads the whole catalog
	let yours = $state<DataProduct[]>([]);
	let yoursTotal = $state(0);
	let rows = $state<DataProduct[]>([]);
	let rowsTotal = $state(0);
	let catalogTotal = $state<number | null>(null);
	let facets = $state<ProductFacets>({ owners: [], tags: [] });
	let loading = $state(true);
	let failed = $state<string | null>(null);
	let selected = $state<DataProduct | null>(null);

	let search = $state('');
	let query = $state('');
	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	let ownerFilter = $state<string[]>([]);
	let tagFilter = $state<string[]>([]);
	let mineOnly = $state(false);
	let sort = $state<ProductSort>('popular');
	let page = $state(1);
	let seq = 0;

	const canManage = auth.hasPermission('assets', 'manage');
	const me = auth.getCurrentUserId();

	let filtering = $derived(!!query || ownerFilter.length > 0 || tagFilter.length > 0 || mineOnly);

	$effect(() => {
		if (!browser) return;
		getProductFacets()
			.then((f) => (facets = f))
			.catch(() => {});
	});

	$effect(() => {
		if (!browser) return;
		const params = {
			q: query,
			owners: [...ownerFilter],
			tags: [...tagFilter],
			mine: mineOnly,
			sort,
			page
		};
		untrack(() => load(params));
	});

	async function load(p: {
		q: string;
		owners: string[];
		tags: string[];
		mine: boolean;
		sort: ProductSort;
		page: number;
	}) {
		const mine = ++seq;
		loading = true;
		failed = null;
		const offset = (p.page - 1) * PAGE;
		try {
			if (p.q || p.owners.length || p.tags.length || p.mine) {
				const res = await queryDataProducts({
					q: p.q,
					owners: p.owners,
					tags: p.tags,
					mine: p.mine ? 'only' : undefined,
					sort: p.sort,
					offset,
					limit: PAGE
				});
				if (mine !== seq) return;
				rows = (res.data_products ?? []).map(normaliseProduct);
				rowsTotal = res.total ?? rows.length;
			} else if (me) {
				// Products the person's teams own come first, so each product appears once
				const [own, rest] = await Promise.all([
					queryDataProducts({ mine: 'only', sort: p.sort, limit: YOURS_MAX }),
					queryDataProducts({ mine: 'exclude', sort: p.sort, offset, limit: PAGE })
				]);
				if (mine !== seq) return;
				yours = (own.data_products ?? []).map(normaliseProduct);
				yoursTotal = own.total ?? yours.length;
				rows = (rest.data_products ?? []).map(normaliseProduct);
				rowsTotal = rest.total ?? rows.length;
				catalogTotal = yoursTotal + rowsTotal;
			} else {
				const res = await queryDataProducts({ sort: p.sort, offset, limit: PAGE });
				if (mine !== seq) return;
				yours = [];
				yoursTotal = 0;
				rows = (res.data_products ?? []).map(normaliseProduct);
				rowsTotal = res.total ?? rows.length;
				catalogTotal = rowsTotal;
			}
		} catch (e) {
			if (mine !== seq) return;
			const status = (e as { status?: number }).status ?? 0;
			failed =
				status >= 500
					? m.products_error_server()
					: e instanceof Error
						? e.message
						: m.products_error_unknown();
		} finally {
			if (mine === seq) loading = false;
		}
	}

	function retry() {
		load({
			q: query,
			owners: ownerFilter,
			tags: tagFilter,
			mine: mineOnly,
			sort,
			page
		});
	}

	function onSearch(value: string) {
		search = value;
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => {
			query = search.trim();
			page = 1;
		}, 250);
	}

	function plain(text?: string): string {
		return (text ?? '').replace(/<[^>]*>/g, '');
	}

	function ownerNames(p: DataProduct): string[] {
		return p.owners.map((o) => o.name || o.username || o.id);
	}

	async function preview(event: MouseEvent, product: DataProduct) {
		// Modified clicks keep the browser's own behaviour, so the page can still open in a new tab.
		if (event.metaKey || event.ctrlKey || event.shiftKey || event.button !== 0) return;
		event.preventDefault();
		selected = product;
		try {
			const res = await fetchApi(`/products/${product.id}`);
			if (res.ok && selected?.id === product.id) selected = normaliseProduct(await res.json());
		} catch {
			// The list entry is enough to preview while the full record is unavailable.
		}
	}

	function clearFilters() {
		search = '';
		query = '';
		ownerFilter = [];
		tagFilter = [];
		mineOnly = false;
		page = 1;
	}

	function showAllYours() {
		mineOnly = true;
		page = 1;
	}
</script>

<svelte:head>
	<title>{m.products_page_title()}</title>
</svelte:head>

{#snippet row(p: DataProduct)}
	<li aria-current={selected?.id === p.id ? 'true' : undefined}>
		<ProductIcon iconUrl={p.icon_url} />
		<div class="mm-stack body">
			<a
				class="mm-link mm-link--quiet name"
				href={resolve('/products/[id]', { id: p.id })}
				onclick={(e) => preview(e, p)}>{p.name}</a
			>
			{#if p.description}
				<mm-text variant="dense" tone="secondary" block class="truncate"
					>{plain(p.description)}</mm-text
				>
			{/if}
			{#if p.owners.length > 0 || p.tags.length > 0}
				<mm-text variant="caption" tone="secondary" block class="truncate"
					>{[...ownerNames(p), ...p.tags].join(' · ')}</mm-text
				>
			{/if}
		</div>
		<div class="mm-stack side">
			{#if popularityLevel(p.popularity)}
				<mm-popularity level={popularityLevel(p.popularity)}></mm-popularity>
			{/if}
			<mm-text variant="caption" tone="secondary" numeric
				>{m.products_assets_count({ count: p.asset_count ?? 0 })}</mm-text
			>
		</div>
	</li>
{/snippet}

{#snippet list(label: string, rows: DataProduct[])}
	<ul class="mm-list products" aria-label={label}>
		{#each rows as p (p.id)}
			{@render row(p)}
		{/each}
	</ul>
{/snippet}

{#snippet allRows()}
	<div class="mm-stack" style="--mm-gap: var(--space-3)">
		{@render list(m.products_all(), rows)}
		{@render pager(rowsTotal)}
	</div>
{/snippet}

{#snippet pager(count: number)}
	{#if count > PAGE}
		<mm-pagination
			total={count}
			page-size={PAGE}
			{page}
			label={m.products_pages_label()}
			onmm-page-change={(e: CustomEvent<{ page: number }>) => (page = e.detail.page)}
		></mm-pagination>
	{/if}
{/snippet}

<div class="mm-stack" style="--mm-gap: var(--space-5)">
	<mm-page-header
		heading={m.products_heading()}
		compact
		count={failed || catalogTotal === null ? undefined : catalogTotal}
	>
		{#if canManage && catalogTotal !== 0}
			<mm-button slot="actions" variant="primary" icon="add" href={resolve('/products/new')}
				>{m.products_create()}</mm-button
			>
		{/if}
	</mm-page-header>

	{#if !loading && !failed && catalogTotal === 0 && !filtering}
		<mm-empty-state
			heading={m.products_empty_title()}
			description={m.products_empty_description()}
			seed={5}
		>
			{#if canManage}
				<mm-button slot="action" variant="primary" icon="add" href={resolve('/products/new')}
					>{m.products_create()}</mm-button
				>
			{/if}
		</mm-empty-state>
	{:else}
		<mm-toolbar label={m.products_filter_label()}>
			<mm-search-field
				class="search"
				label={m.products_search_placeholder()}
				hide-label
				placeholder={m.products_search_hint()}
				shortcut="/"
				value={search}
				onmm-input={(e: CustomEvent<{ value: string }>) => onSearch(e.detail.value)}
			></mm-search-field>
			<mm-multiselect
				class="filter"
				label={m.common_owners()}
				hide-label
				placeholder={m.products_any_owner()}
				max-tags={1}
				value={ownerFilter}
				onmm-change={(e: CustomEvent<{ value: string[] }>) => {
					ownerFilter = e.detail.value;
					page = 1;
				}}
			>
				{#each facets.owners as o (o.id)}
					<mm-option
						value={o.id}
						label={o.name || o.id}
						icon={o.type === 'team' ? 'groups' : 'person'}
					></mm-option>
				{/each}
			</mm-multiselect>
			<mm-multiselect
				class="filter"
				label={m.common_tags()}
				hide-label
				placeholder={m.products_any_tag()}
				max-tags={1}
				value={tagFilter}
				onmm-change={(e: CustomEvent<{ value: string[] }>) => {
					tagFilter = e.detail.value;
					page = 1;
				}}
			>
				{#each facets.tags as t (t.tag)}
					<mm-option value={t.tag} label={t.tag}></mm-option>
				{/each}
			</mm-multiselect>
			<mm-select
				slot="end"
				class="sort"
				label={m.products_sort_label()}
				hide-label
				value={sort}
				onmm-change={(e: CustomEvent<{ value: string }>) => {
					sort =
						(['popular', 'updated', 'name'] as const).find((v) => v === e.detail.value) ??
						'popular';
					page = 1;
				}}
			>
				<mm-option value="popular">{m.products_sort_popular()}</mm-option>
				<mm-option value="updated">{m.products_sort_updated()}</mm-option>
				<mm-option value="name">{m.products_sort_name()}</mm-option>
			</mm-select>
		</mm-toolbar>

		<div>
			{#if failed}
				<mm-callout tone="danger" heading={m.products_load_failed()}>
					{failed}
					<mm-button slot="actions" size="sm" icon="refresh" onclick={retry}
						>{m.products_try_again()}</mm-button
					>
				</mm-callout>
			{:else if loading && rows.length === 0 && yours.length === 0}
				<mm-skeleton loading count={6}></mm-skeleton>
			{:else if filtering && rowsTotal === 0}
				<mm-empty-state
					heading={m.products_no_match_title()}
					description={m.products_no_match_description()}
					seed={3}
				>
					<mm-button slot="action" onclick={clearFilters}>{m.products_clear_search()}</mm-button>
				</mm-empty-state>
			{:else if filtering}
				<!-- Searching or filtering shows one ranked list, so results are not split across sections. -->
				<div class="mm-stack" style="--mm-gap: var(--space-3)">
					<div class="mm-cluster mm-cluster--between">
						<mm-text variant="dense" tone="secondary" role="status"
							>{mineOnly
								? m.products_results_yours({ count: rowsTotal })
								: m.products_results({ count: rowsTotal })}</mm-text
						>
						{#if mineOnly}
							<mm-button size="sm" variant="ghost" onclick={clearFilters}
								>{m.products_show_every_product()}</mm-button
							>
						{/if}
					</div>
					{@render list(m.products_search_results(), rows)}
					{@render pager(rowsTotal)}
				</div>
			{:else}
				{#if yours.length > 0}
					<mm-section heading={m.products_yours()}>
						{#if yoursTotal > YOURS_MAX}
							<mm-button slot="actions" variant="ghost" size="sm" onclick={showAllYours}
								>{m.products_show_all_count({ count: yoursTotal })}</mm-button
							>
						{/if}
						{@render list(m.products_yours(), yours)}
					</mm-section>
				{/if}
				{#if yours.length > 0}
					<mm-section heading={m.products_all()}>
						{@render allRows()}
					</mm-section>
				{:else}
					{@render allRows()}
				{/if}
			{/if}
		</div>
	{/if}
</div>

{#if selected}
	<ProductBlade bind:product={selected} onClose={() => (selected = null)} />
{/if}

<style>
	.search {
		width: 20rem;
		max-width: 100%;
	}
	.filter {
		width: 14rem;
	}
	.sort {
		width: 12rem;
	}
	.products {
		border-block: 1px solid var(--mm-rule);
	}
	.products > li {
		gap: var(--space-4);
		padding-block: var(--space-4);
	}
	.products > li[aria-current='true'] {
		background: var(--mm-selected);
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
	.side {
		--mm-gap: var(--space-2);
		align-items: flex-end;
		flex: none;
	}
</style>
