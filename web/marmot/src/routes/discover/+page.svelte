<script lang="ts">
	import '@marmotdata/elements/search-bar.js';
	import '@marmotdata/elements/facet-sidebar.js';
	import '@marmotdata/elements/popularity.js';
	import {
		buildSearch,
		parseSearch,
		type SearchBarField,
		type SearchCondition
	} from '@marmotdata/elements/search-bar.js';
	import type { MmFacetOption } from '@marmotdata/elements/classes/facet-sidebar.js';
	import { untrack } from 'svelte';
	import { SvelteURLSearchParams } from 'svelte/reactivity';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { browser } from '$app/environment';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { setCrumbs } from '$lib/mm';
	import { m } from '$lib/paraglide/messages';
	import { formatRelativeTime } from '$lib/utils/format';
	import type { Asset } from '$lib/assets/types';
	import type { DataProduct } from '$lib/dataproducts/types';
	import AssetBlade from '$components/asset/AssetBlade.svelte';
	import ProductBlade from '$components/product/ProductBlade.svelte';
	import {
		RESULT_KINDS,
		catalogFields,
		compactNumber,
		kindLabel,
		loadFieldValues,
		loadMetadataFields,
		providerLabel,
		typeLabel
	} from '$components/query/catalogSearch';

	interface SearchResultMetadata {
		type?: string;
		mrn?: string;
		tags?: string[];
		primary_provider?: string;
		providers?: string[];
		asset_count?: number;
		[key: string]: unknown;
	}

	interface ResultOwner {
		id: string;
		name: string;
		kind: 'user' | 'team';
	}

	interface SearchResult {
		type: 'asset' | 'glossary' | 'team' | 'data_product';
		id: string;
		name: string;
		description?: string;
		metadata?: SearchResultMetadata;
		url: string;
		rank: number;
		updated_at?: string;
		owners?: ResultOwner[];
		popularity?: number;
		row_count?: number;
		column_count?: number;
		stale?: boolean;
		linked_asset_count?: number;
		asset_count?: number;
		member_count?: number;
	}

	interface FacetValue {
		value: string;
		count: number;
	}

	interface OwnerFacetValue extends FacetValue {
		label: string;
		kind: 'user' | 'team';
	}

	interface Facets {
		types: Record<string, number>;
		asset_types: FacetValue[];
		providers: FacetValue[];
		tags: FacetValue[];
		owners: OwnerFacetValue[];
	}

	type Sort = 'relevance' | 'popular' | 'updated' | 'name';
	const SORTS: Sort[] = ['relevance', 'popular', 'updated', 'name'];

	interface SearchResponse {
		results: SearchResult[];
		total: number;
		facets: Facets;
	}

	/** The filters Discover keeps in its own URL params, read and written by the facets as well as the search bar. */
	interface Filters {
		q: string;
		kinds: string[];
		types: string[];
		providers: string[];
		tags: string[];
		/** User and team IDs; owners match assets, glossary terms and data products, so they never narrow the kinds. */
		owners: string[];
	}

	const ALL_KINDS: string[] = [...RESULT_KINDS];
	const PAGE_SIZE = 20;

	setCrumbs([{ label: m.discover_heading() }]);

	const canManageAssets = auth.hasPermission('assets', 'manage');
	const canAddPlugins = auth.hasPermission('ingestion', 'manage');

	let filters = $state<Filters>({
		q: '',
		kinds: [],
		types: [],
		providers: [],
		tags: [],
		owners: []
	});
	let sort = $state<Sort>('relevance');
	let currentPage = $state(1);
	let barValue = $state('');
	let results = $state<SearchResult[]>([]);
	let total = $state(0);
	let facets = $state<Facets>({
		types: {},
		asset_types: [],
		providers: [],
		tags: [],
		owners: []
	});
	let loading = $state(true);
	let loaded = $state(false);
	let error = $state<{ status: number; message: string } | null>(null);
	let draftCount = $state<number | undefined>(undefined);
	let selectedAsset = $state<Asset | null>(null);
	let selectedProduct = $state<DataProduct | null>(null);

	let lastSearch: string | null = null;
	let request = 0;
	let typingTimer: ReturnType<typeof setTimeout> | undefined;
	let draftTimer: ReturnType<typeof setTimeout> | undefined;

	/** Kinds narrowed below "everything", which is what the backend does when none are given. */
	const narrowed = (kinds: string[]) => kinds.length > 0 && kinds.length < ALL_KINDS.length;

	// Facet-backed conditions in the bar: plain "is" conditions on these fields live in the URL params rather than in q, so the tokens and the facets edit one set of filters.
	const FACET_FIELDS = {
		kind: 'kinds',
		type: 'types',
		provider: 'providers',
		tag: 'tags'
	} as const;

	function facetConditions(f: Filters): SearchCondition[] {
		const out: SearchCondition[] = [];
		if (narrowed(f.kinds)) out.push({ field: 'kind', operator: '=', values: f.kinds });
		if (f.types.length) out.push({ field: 'type', operator: '=', values: f.types });
		if (f.providers.length) out.push({ field: 'provider', operator: '=', values: f.providers });
		if (f.tags.length) out.push({ field: 'tag', operator: '=', values: f.tags });
		return out;
	}

	/** The search bar's query: q plus the facet filters as tokens. */
	function compose(f: Filters): string {
		const extra = facetConditions(f);
		const parsed = parseSearch(f.q);
		if (parsed)
			return buildSearch({ text: parsed.text, conditions: [...parsed.conditions, ...extra] });
		const rest = buildSearch({ text: '', conditions: extra });
		return rest ? `(${f.q}) AND ${rest}` : f.q;
	}

	/** Splits the bar's query back into q and the facet filters. A query the tokens cannot show stays whole in q, since the backend reads kind, type, provider and tag there too, as do wildcard values the facets cannot hold. Owners have no token, so they carry over. */
	function decompose(value: string): Filters {
		const f: Filters = {
			q: value.trim(),
			kinds: [],
			types: [],
			providers: [],
			tags: [],
			owners: [...filters.owners]
		};
		const parsed = parseSearch(value);
		if (!parsed) return f;
		const rest: SearchCondition[] = [];
		for (const c of parsed.conditions) {
			const key = FACET_FIELDS[c.field as keyof typeof FACET_FIELDS];
			if (key && c.operator === '=' && !c.values.some((v) => v.includes('*'))) {
				for (const v of c.values) if (!f[key].includes(v)) f[key].push(v);
			} else rest.push(c);
		}
		f.q = buildSearch({ text: parsed.text, conditions: rest });
		return f;
	}

	function readParams(params: URLSearchParams) {
		const list = (key: string) => params.get(key)?.split(',').filter(Boolean) ?? [];
		filters = {
			q: params.get('q') ?? '',
			kinds: list('kind'),
			types: list('types'),
			providers: list('providers'),
			tags: list('tags'),
			owners: list('owner')
		};
		const s = params.get('sort') as Sort | null;
		sort = s && SORTS.includes(s) ? s : 'relevance';
		currentPage = Math.max(1, parseInt(params.get('page') ?? '1', 10) || 1);
	}

	function searchParams(f: Filters, offset: number, limit: number): URLSearchParams {
		const params = new SvelteURLSearchParams({ limit: String(limit), offset: String(offset) });
		if (f.q) params.append('q', f.q);
		for (const kind of f.kinds.length ? f.kinds : ALL_KINDS) params.append('types[]', kind);
		// Asset filters only apply while assets are among the kinds, as before.
		if (!f.kinds.length || f.kinds.includes('asset')) {
			if (f.types.length) params.append('asset_types', f.types.join(','));
			if (f.providers.length) params.append('providers', f.providers.join(','));
			if (f.tags.length) params.append('tags', f.tags.join(','));
		}
		if (f.owners.length) params.append('owner', f.owners.join(','));
		if (sort !== 'relevance') params.append('sort', sort);
		return params;
	}

	// Follow the URL: the first load, back and forward, and links into Discover from elsewhere.
	$effect(() => {
		const search = page.url.search;
		untrack(() => {
			if (search === lastSearch) return;
			lastSearch = search;
			readParams(page.url.searchParams);
			barValue = compose(filters);
			if (browser) fetchResults();
		});
	});

	async function fetchResults() {
		const id = ++request;
		loading = true;
		error = null;
		try {
			const response = await fetchApi(
				`/search?${searchParams(filters, (currentPage - 1) * PAGE_SIZE, PAGE_SIZE)}`
			);
			if (id !== request) return;
			if (!response.ok) {
				const data = await response.json().catch(() => ({}));
				error = { status: response.status, message: data.error || m.discover_error_generic() };
				return;
			}
			const data: SearchResponse = await response.json();
			if (id !== request) return;
			results = data.results ?? [];
			total = data.total ?? 0;
			facets = {
				types: data.facets?.types ?? {},
				asset_types: data.facets?.asset_types ?? [],
				providers: data.facets?.providers ?? [],
				tags: data.facets?.tags ?? [],
				owners: data.facets?.owners ?? []
			};
		} catch (e) {
			if (id !== request) return;
			error = { status: 0, message: e instanceof Error ? e.message : m.discover_error_unknown() };
		} finally {
			if (id === request) {
				loading = false;
				loaded = true;
			}
		}
	}

	function updateURL() {
		const params = new SvelteURLSearchParams();
		if (filters.q) params.set('q', filters.q);
		if (filters.kinds.length) params.set('kind', filters.kinds.join(','));
		if (filters.types.length) params.set('types', filters.types.join(','));
		if (filters.providers.length) params.set('providers', filters.providers.join(','));
		if (filters.tags.length) params.set('tags', filters.tags.join(','));
		if (filters.owners.length) params.set('owner', filters.owners.join(','));
		if (sort !== 'relevance') params.set('sort', sort);
		if (currentPage > 1) params.set('page', String(currentPage));
		const qs = params.toString();
		lastSearch = qs ? `?${qs}` : '';
		goto(resolve(qs ? `/discover?${qs}` : '/discover'), {
			replaceState: true,
			noScroll: true,
			keepFocus: true
		});
	}

	function apply(next: Filters, page = 1) {
		filters = next;
		currentPage = page;
		updateURL();
		fetchResults();
	}

	// Typing fires on every key, so wait for a pause; tokens, facets and Enter apply at once.
	function onBarInput(value: string) {
		barValue = value;
		clearTimeout(typingTimer);
		typingTimer = setTimeout(() => apply(decompose(value)), 300);
	}

	function onBarSearch() {
		clearTimeout(typingTimer);
		apply(decompose(barValue));
	}

	function onDraft(value: string) {
		clearTimeout(draftTimer);
		draftTimer = setTimeout(async () => {
			try {
				const res = await fetchApi(`/search?${searchParams(decompose(value), 0, 1)}`);
				draftCount = res.ok ? ((await res.json()) as SearchResponse).total : undefined;
			} catch {
				draftCount = undefined;
			}
		}, 300);
	}

	function onFacets(selected: Record<string, string[]>) {
		const next: Filters = {
			q: filters.q,
			kinds: selected.kind ?? [],
			types: selected.type ?? filters.types,
			providers: selected.provider ?? filters.providers,
			tags: selected.tags ?? filters.tags,
			owners: selected.owner ?? filters.owners
		};
		// Picking an asset filter narrows to assets, as the old filters did, so other kinds do not pass through unfiltered.
		const addedAssetFilter =
			next.types.length > filters.types.length ||
			next.providers.length > filters.providers.length ||
			next.tags.length > filters.tags.length;
		if (addedAssetFilter && !narrowed(next.kinds)) next.kinds = ['asset'];
		if (narrowed(next.kinds) === false) next.kinds = [];
		barValue = compose(next);
		apply(next);
	}

	function clearFilters() {
		const next: Filters = { q: '', kinds: [], types: [], providers: [], tags: [], owners: [] };
		barValue = '';
		apply(next);
	}

	function onSort(value: string) {
		if (!SORTS.includes(value as Sort) || value === sort) return;
		sort = value as Sort;
		apply(filters);
	}

	async function openAsset(id: string) {
		try {
			const response = await fetchApi(`/assets/${id}`);
			if (response.ok) {
				selectedProduct = null;
				selectedAsset = await response.json();
			}
		} catch (err) {
			console.error('Error loading asset:', err);
		}
	}

	async function openProduct(id: string) {
		try {
			const response = await fetchApi(`/products/${id}`);
			if (response.ok) {
				const product: DataProduct = await response.json();
				product.tags = product.tags || [];
				product.owners = product.owners || [];
				product.rules = product.rules || [];
				selectedAsset = null;
				selectedProduct = product;
			}
		} catch (err) {
			console.error('Error loading data product:', err);
		}
	}

	/** Plain clicks open the preview; modified clicks keep the link's own behaviour, such as a new tab. */
	function preview(e: MouseEvent, open: () => void) {
		if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
		e.preventDefault();
		open();
	}

	function providerOf(r: SearchResult): string {
		return r.metadata?.primary_provider || r.metadata?.providers?.[0] || '';
	}

	function ownerNames(r: SearchResult): string {
		return (r.owners ?? [])
			.map((o) => o.name)
			.filter(Boolean)
			.join(', ');
	}

	/** One quiet caption per row, as plain text separated by dots: what it is, its size, who owns it and its tags for assets, and the count that matters for each other kind. */
	function caption(r: SearchResult): string {
		const parts: string[] = [];
		if (r.type === 'asset') {
			if (r.metadata?.type) parts.push(typeLabel(r.metadata.type));
			const providers = r.metadata?.providers ?? [];
			if (providers.length) parts.push(providers.map(providerLabel).join(', '));
			if (r.column_count) parts.push(m.discover_result_columns({ count: r.column_count }));
			if (r.row_count)
				parts.push(
					m.discover_result_rows({ count: r.row_count, rows: compactNumber(r.row_count) })
				);
			parts.push(ownerNames(r));
			parts.push(...(r.metadata?.tags ?? []).filter((t) => t !== 'pii' && t !== 'certified'));
			if (r.updated_at)
				parts.push(m.discover_result_updated({ when: formatRelativeTime(r.updated_at) }));
		} else {
			parts.push(kindLabel(r.type));
			if (r.type === 'glossary') {
				parts.push(ownerNames(r));
				if (r.linked_asset_count !== undefined)
					parts.push(m.discover_result_linked_assets({ count: r.linked_asset_count }));
			} else if (r.type === 'data_product') {
				parts.push(ownerNames(r));
				const count = r.asset_count ?? r.metadata?.asset_count;
				if (count !== undefined) parts.push(m.discover_result_assets({ count }));
			} else if (r.type === 'team') {
				if (r.member_count !== undefined)
					parts.push(m.discover_result_members({ count: r.member_count }));
				if (r.asset_count !== undefined)
					parts.push(m.discover_result_assets({ count: r.asset_count }));
			}
		}
		return parts.filter(Boolean).join(' · ');
	}

	/** mm-popularity draws one to four bars; an asset nobody viewed recently shows none. */
	function popularityLevel(r: SearchResult): 1 | 2 | 3 | 4 | null {
		const level = Math.round(r.popularity ?? 0);
		return level >= 1 ? (Math.min(level, 4) as 1 | 2 | 3 | 4) : null;
	}

	const kindIcons: Record<string, string> = {
		glossary: 'menu-book',
		team: 'groups',
		data_product: 'package-2'
	};

	let showAssetFacets = $derived(!filters.kinds.length || filters.kinds.includes('asset'));
	let hasFilters = $derived(
		narrowed(filters.kinds) ||
			filters.types.length > 0 ||
			filters.providers.length > 0 ||
			filters.tags.length > 0 ||
			filters.owners.length > 0
	);
	let firstRun = $derived(loaded && !loading && !error && total === 0 && !filters.q && !hasFilters);

	let fields = $derived<SearchBarField[]>(
		catalogFields({
			types: facets.asset_types.map((f) => f.value),
			providers: facets.providers.map((f) => f.value),
			tags: facets.tags.map((f) => f.value)
		})
	);

	let kindOptions = $derived<MmFacetOption[]>([
		{
			value: 'asset',
			label: m.discover_facet_assets(),
			icon: 'database',
			count: facets.types.asset ?? 0
		},
		{
			value: 'data_product',
			label: m.discover_facet_products(),
			icon: 'package-2',
			count: facets.types.data_product ?? 0
		},
		{
			value: 'glossary',
			label: m.discover_facet_terms(),
			icon: 'menu-book',
			count: facets.types.glossary ?? 0
		},
		{
			value: 'team',
			label: m.discover_facet_teams(),
			icon: 'groups',
			count: facets.types.team ?? 0
		}
	]);
	let typeOptions = $derived<MmFacetOption[]>(
		facets.asset_types.map((f) => ({ value: f.value, label: typeLabel(f.value), count: f.count }))
	);
	let providerOptions = $derived<MmFacetOption[]>(
		facets.providers.map((f) => ({
			value: f.value,
			label: providerLabel(f.value),
			provider: f.value.toLowerCase(),
			count: f.count
		}))
	);
	let tagOptions = $derived<MmFacetOption[]>(
		facets.tags.map((f) => ({ value: f.value, label: f.value, count: f.count }))
	);
	// Owners already picked stay listed even when the counts no longer return them, so they can be cleared.
	let ownerOptions = $derived<MmFacetOption[]>([
		...facets.owners.map((f) => ({
			value: f.value,
			label: f.label || f.value,
			icon: f.kind === 'team' ? 'groups' : 'person',
			count: f.count
		})),
		...filters.owners
			.filter((id) => !facets.owners.some((f) => f.value === id))
			.map((id) => ({ value: id, label: id }))
	]);
</script>

<svelte:head>
	<title>{m.discover_page_title()}</title>
</svelte:head>

<div class="mm-stack" style="--mm-gap: var(--space-5)">
	<mm-page-header compact heading={m.discover_heading()}>
		{#if canManageAssets}
			<mm-button slot="actions" variant="secondary" icon="add" href={resolve('/assets/new')}
				>{m.discover_new_asset_button()}</mm-button
			>
		{/if}
	</mm-page-header>

	{#if firstRun}
		<mm-empty-state
			heading={m.discover_firstrun_heading()}
			description={m.discover_firstrun_description()}
			seed={2}
		>
			{#if canAddPlugins}
				<mm-button slot="action" variant="primary" icon="extension" href={resolve('/pipelines/new')}
					>{m.discover_firstrun_add_plugin()}</mm-button
				>
			{/if}
		</mm-empty-state>
		<section
			class="mm-stack mm-measure"
			style="--mm-gap: var(--space-2); margin-inline: auto; width: 100%"
			aria-labelledby="other-ways"
		>
			<mm-heading id="other-ways" level={2} variant="panel-title"
				>{m.discover_firstrun_other_ways()}</mm-heading
			>
			<mm-list aria-label={m.discover_firstrun_other_ways()}>
				<mm-list-item
					href="https://marmotdata.io/docs/open-lineage"
					target="_blank"
					secondary={m.discover_firstrun_openlineage_hint()}
					><mm-icon slot="start" name="conversion-path" aria-hidden="true"
					></mm-icon>{m.discover_firstrun_openlineage()}</mm-list-item
				>
				<mm-list-item
					href="https://marmotdata.io/docs/Populating/CLI"
					target="_blank"
					secondary={m.discover_firstrun_cli_hint()}
					><mm-icon slot="start" name="terminal" aria-hidden="true"
					></mm-icon>{m.discover_firstrun_cli()}</mm-list-item
				>
				{#if canManageAssets}
					<mm-list-item href={resolve('/assets/new')} secondary={m.discover_firstrun_manual_hint()}
						><mm-icon slot="start" name="edit-note" aria-hidden="true"
						></mm-icon>{m.discover_firstrun_manual()}</mm-list-item
					>
				{/if}
			</mm-list>
		</section>
	{:else}
		<mm-search-bar
			label={m.discover_search_label()}
			placeholder={m.discover_search_placeholder()}
			value={barValue}
			{fields}
			loadFields={loadMetadataFields}
			loadValues={loadFieldValues}
			draft-count={draftCount}
			onmm-input={(e: CustomEvent<{ value: string }>) => onBarInput(e.detail.value)}
			onmm-search={onBarSearch}
			onmm-draft={(e: CustomEvent<{ value: string }>) => onDraft(e.detail.value)}
		></mm-search-bar>

		<div class="mm-split mm-split--sidebar">
			<mm-facet-sidebar
				heading={m.discover_filters_heading()}
				hide-applied
				onmm-filter-change={(e: CustomEvent<{ facets: Record<string, string[]> }>) =>
					onFacets(e.detail.facets)}
			>
				<mm-facet
					name="kind"
					label={m.discover_kind_heading()}
					options={kindOptions}
					value={narrowed(filters.kinds) ? filters.kinds : []}
				></mm-facet>
				{#if showAssetFacets}
					<mm-facet name="type" label={m.common_type()} options={typeOptions} value={filters.types}
					></mm-facet>
					<mm-facet
						name="provider"
						label={m.discover_field_provider()}
						options={providerOptions}
						value={filters.providers}
					></mm-facet>
					<mm-facet name="tags" label={m.common_tags()} options={tagOptions} value={filters.tags}
					></mm-facet>
				{/if}
				<mm-facet
					name="owner"
					label={m.discover_facet_owner()}
					options={ownerOptions}
					value={filters.owners}
					collapsed={filters.owners.length === 0}
				></mm-facet>
			</mm-facet-sidebar>

			<section
				aria-label={m.discover_results_label()}
				class="mm-stack"
				style="--mm-gap: var(--space-4); min-width: 0"
			>
				{#if error}
					{#if error.status === 400}
						<mm-callout tone="warning" heading={m.discover_error_query_heading()}>
							{m.discover_error_invalid_query()}
						</mm-callout>
					{:else}
						<mm-callout tone="danger" heading={m.discover_error_heading()}>
							{error.status >= 500 ? m.discover_error_server() : error.message}
							<mm-button
								slot="actions"
								size="sm"
								variant="secondary"
								icon="refresh"
								onclick={fetchResults}>{m.discover_try_again()}</mm-button
							>
						</mm-callout>
					{/if}
				{:else if loaded && !loading && results.length === 0}
					<mm-empty-state
						heading={m.discover_empty_heading()}
						description={m.discover_empty_description()}
						seed={5}
					>
						{#if hasFilters}
							<mm-button slot="action" variant="secondary" icon="filter-list" onclick={clearFilters}
								>{m.discover_clear_filters()}</mm-button
							>
						{/if}
					</mm-empty-state>
				{:else}
					<div class="mm-cluster mm-cluster--between">
						<mm-text variant="dense" tone="secondary" role="status"
							>{loaded ? m.discover_results_count({ count: total }) : ''}</mm-text
						>
						<mm-select
							label={m.discover_sort_label()}
							hide-label
							value={sort}
							style="inline-size: 12rem"
							onmm-change={(e: CustomEvent<{ value: string }>) => onSort(e.detail.value)}
						>
							<mm-option value="relevance">{m.discover_sort_relevance()}</mm-option>
							<mm-option value="popular">{m.discover_sort_popular()}</mm-option>
							<mm-option value="updated">{m.discover_sort_updated()}</mm-option>
							<mm-option value="name">{m.discover_sort_name()}</mm-option>
						</mm-select>
					</div>
					<mm-skeleton loading={loading && !results.length} shape="table-row" count={8}>
						<ul class="mm-list results" aria-label={m.discover_results_label()} aria-busy={loading}>
							{#each results as r (r.type + r.id)}
								<li
									aria-current={selectedAsset?.id === r.id || selectedProduct?.id === r.id
										? 'true'
										: undefined}
								>
									{#if r.type === 'asset' && providerOf(r)}
										<mm-provider-logo
											class="lead"
											provider={providerOf(r).toLowerCase()}
											size={20}
											decorative
										></mm-provider-logo>
									{:else}
										<mm-icon
											class="lead"
											name={kindIcons[r.type] ?? 'database'}
											size={20}
											aria-hidden="true"
										></mm-icon>
									{/if}
									<div class="mm-stack" style="--mm-gap: var(--space-1); flex: 1; min-width: 0">
										<div class="mm-cluster" style="--mm-gap: var(--space-2)">
											<a
												class="mm-link mm-link--quiet name"
												href={resolve(r.url as '/')}
												title={r.description ?? ''}
												onclick={(e) =>
													r.type === 'asset'
														? preview(e, () => openAsset(r.id))
														: r.type === 'data_product'
															? preview(e, () => openProduct(r.id))
															: undefined}>{r.name}</a
											>
											{#if r.metadata?.tags?.includes('pii')}
												<mm-tag variant="pii"></mm-tag>
											{/if}
											{#if r.metadata?.tags?.includes('certified')}
												<mm-tag variant="certified"></mm-tag>
											{/if}
											{#if r.stale}
												<mm-status-pill state="stale"></mm-status-pill>
											{/if}
										</div>
										<mm-text variant="caption" tone="secondary" block>{caption(r)}</mm-text>
									</div>
									{#if r.type === 'asset' && popularityLevel(r)}
										<mm-popularity class="popularity" level={popularityLevel(r)}></mm-popularity>
									{/if}
								</li>
							{/each}
						</ul>
					</mm-skeleton>
					{#if total > PAGE_SIZE}
						<mm-pagination
							{total}
							page-size={PAGE_SIZE}
							page={currentPage}
							label={m.discover_pagination_label()}
							onmm-page-change={(e: CustomEvent<{ page: number }>) => apply(filters, e.detail.page)}
						></mm-pagination>
					{/if}
				{/if}
			</section>
		</div>
	{/if}
</div>

{#if selectedAsset}
	<AssetBlade bind:asset={selectedAsset} onClose={() => (selectedAsset = null)} />
{/if}

{#if selectedProduct}
	<ProductBlade bind:product={selectedProduct} onClose={() => (selectedProduct = null)} />
{/if}

<style>
	.results {
		border-block: 1px solid var(--mm-rule);
	}
	/* The whole row is the target: the name's link stretches over it, so modified clicks and keyboard focus still go to one real link */
	.results > li {
		position: relative;
		align-items: flex-start;
		gap: var(--space-3);
		padding-block: var(--space-3);
		transition: background-color var(--mm-dur-press) var(--mm-ease);
	}
	.results > li:hover {
		background: var(--mm-sunken);
	}
	.name::after {
		content: '';
		position: absolute;
		inset: 0;
	}
	.results mm-tag {
		position: relative;
	}
	.popularity {
		flex: none;
		margin-block-start: 4px;
	}
	.results > li[aria-current='true'] {
		background: var(--mm-selected);
	}
	.lead {
		flex: none;
		margin-block-start: 2px;
		color: var(--mm-text-2);
	}
	.name {
		overflow-wrap: anywhere;
	}
</style>
