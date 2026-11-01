/**
 * Discover, drawn as web/marmot/src/routes/discover/+page.svelte draws it: the search bar (search
 * and filters), the facets, one result list for every kind, pagination and the asset preview blade
 * (AssetBlade.svelte). The list is the catalog filtered by whatever the demo has put in the bar,
 * and the facet counts follow the results, as they do for a listing in the app.
 */
import { html, nothing, type TemplateResult } from 'lit';
import type { SearchBarField } from '@marmotdata/elements/classes/search-bar.js';
import { shell } from './shell';
import {
	NOW,
	ago,
	assets,
	glossary,
	ordersLineage,
	owner,
	products,
	type Asset
} from './data/catalog';
import { orderResults, compact, n, providerName, popularity } from './data/asset';

/* ------------------------------------------------------------------ formatting */

const providerLabel = (p: string) => providerName[p] ?? p;
const plural = (count: number, one: string, other: string) =>
	`${n(count)} ${count === 1 ? one : other}`;

const rtf = new Intl.RelativeTimeFormat('en', { numeric: 'always' });
/** As formatRelativeTime in the app: minutes, hours and days, then the date after a week. */
const relTime = (iso: string) => {
	const diff = NOW.getTime() - new Date(iso).getTime();
	const mins = Math.floor(diff / 60_000);
	const hours = Math.floor(diff / 3_600_000);
	const days = Math.floor(diff / 86_400_000);
	if (mins < 1) return 'Just now';
	if (mins < 60) return rtf.format(-mins, 'minute');
	if (hours < 24) return rtf.format(-hours, 'hour');
	if (days < 7) return rtf.format(-days, 'day');
	return new Date(iso).toLocaleDateString('en', {
		year: 'numeric',
		month: 'short',
		day: 'numeric',
		timeZone: 'UTC'
	});
};
/** As formatDateTime in the app, which the blade's More details uses. */
const dateTime = (iso: string) =>
	new Date(iso).toLocaleString('en', {
		year: 'numeric',
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
		timeZone: 'UTC'
	});

/** The tags the app stores: personal data is a tag there and a flag in the sample data. The demos leave certification out. */
const tagsOf = (a: Asset) => [...new Set([...a.tags, ...(a.pii ? ['pii'] : [])])];

/* ------------------------------------------------------------------ results */

type Term = (typeof glossary)[number];
type Product = (typeof products)[number];

type Result =
	| { kind: 'asset'; a: Asset }
	| { kind: 'glossary'; t: Term }
	| { kind: 'data_product'; p: Product }
	| { kind: 'team'; id: string; members: number; assets: number };

/** Everything Discover can list: the catalog, the extra "orders" results, glossary, products and a team. */
const pool: Result[] = [
	...[...assets, ...orderResults.filter((r) => !assets.some((a) => a.mrn === r.mrn))].map(
		(a): Result => ({ kind: 'asset', a })
	),
	...glossary.map((t): Result => ({ kind: 'glossary', t })),
	...products.map((p): Result => ({ kind: 'data_product', p })),
	{ kind: 'team', id: 'ae', members: 6, assets: 9 }
];

const nameOf = (r: Result) =>
	r.kind === 'asset'
		? r.a.name
		: r.kind === 'glossary'
			? r.t.name
			: r.kind === 'data_product'
				? r.p.name
				: owner(r.id).name;
const textOf = (r: Result) =>
	r.kind === 'asset'
		? r.a.description
		: r.kind === 'glossary'
			? r.t.definition
			: r.kind === 'data_product'
				? r.p.description
				: '';
const ownersOf = (r: Result) =>
	r.kind === 'asset'
		? r.a.owners
		: r.kind === 'glossary'
			? r.t.owners
			: r.kind === 'data_product'
				? r.p.owners
				: [];
const ownerNames = (r: Result) =>
	ownersOf(r)
		.map((o) => owner(o).name)
		.join(', ');

/** One quiet caption per row, as plain text separated by dots, as the app writes it for each kind. */
const caption = (r: Result) => {
	const parts: string[] = [];
	if (r.kind === 'asset') {
		const a = r.a;
		parts.push(a.type, providerLabel(a.provider));
		if (a.columns) parts.push(plural(a.columns, 'column', 'columns'));
		if (a.rows) parts.push(`${compact(a.rows)} ${a.rows === 1 ? 'row' : 'rows'}`);
		parts.push(ownerNames(r));
		parts.push(...tagsOf(a).filter((t) => t !== 'pii'));
		parts.push(`Updated ${relTime(a.updated)}`);
	} else if (r.kind === 'glossary') {
		parts.push('Glossary term', ownerNames(r), plural(r.t.linked, 'linked asset', 'linked assets'));
	} else if (r.kind === 'data_product') {
		parts.push('Data product', ownerNames(r), plural(r.p.assets, 'asset', 'assets'));
	} else {
		parts.push('Team', plural(r.members, 'member', 'members'), plural(r.assets, 'asset', 'assets'));
	}
	return parts.filter(Boolean).join(' · ');
};

const kindIcons: Record<string, string> = {
	glossary: 'menu-book',
	team: 'groups',
	data_product: 'package-2'
};

const hrefOf = (r: Result) =>
	r.kind === 'asset'
		? `#asset/${r.a.mrn}`
		: r.kind === 'glossary'
			? `#glossary/${r.t.id}`
			: r.kind === 'data_product'
				? `#products/${r.p.id}`
				: `#teams/${r.id}`;

const staleOf = (r: Result) =>
	r.kind === 'asset' ? !!r.a.stale : r.kind === 'glossary' && 'stale' in r.t && !!r.t.stale;

/** A row: the provider logo or the kind's icon, the name with its badges, one caption, and popularity for assets. */
const resultRow = (r: Result, selected: boolean) => {
	const tags = r.kind === 'asset' ? tagsOf(r.a) : [];
	const lead = 'flex:none;margin-block-start:2px;color:var(--mm-text-2)';
	return html`<li
		style="position:relative;align-items:flex-start;gap:var(--space-3);padding-block:var(--space-3)${selected
			? ';background:var(--mm-selected)'
			: ''}"
		aria-current=${selected ? 'true' : nothing}
	>
		${r.kind === 'asset'
			? html`<mm-provider-logo
					provider=${r.a.provider}
					size="20"
					decorative
					style=${lead}
				></mm-provider-logo>`
			: html`<mm-icon
					name=${kindIcons[r.kind] ?? 'database'}
					size="20"
					aria-hidden="true"
					style=${lead}
				></mm-icon>`}
		<div class="mm-stack" style="--mm-gap:var(--space-1);flex:1;min-width:0">
			<div class="mm-cluster" style="--mm-gap:var(--space-2)">
				<a
					class="mm-link mm-link--quiet"
					style="overflow-wrap:anywhere"
					href=${hrefOf(r)}
					title=${textOf(r)}
					>${nameOf(r)}</a
				>
				${tags.includes('pii') ? html`<mm-tag variant="pii"></mm-tag>` : nothing}
				${staleOf(r) ? html`<mm-status-pill state="stale"></mm-status-pill>` : nothing}
			</div>
			<mm-text variant="caption" tone="secondary" block>${caption(r)}</mm-text>
		</div>
		${r.kind === 'asset'
			? html`<mm-popularity
					level=${popularity(r.a)}
					style="flex:none;margin-block-start:4px"
				></mm-popularity>`
			: nothing}
	</li>`;
};

/* ------------------------------------------------------------------ search */

/** `@field = value` or `@field: value`, with an optional `*` wildcard in the value. */
const condition = /^@([\w.]+)\s*(?:=|:)\s*"?([^"]*)"?$/;
const glob = (pattern: string, value: string) =>
	new RegExp(
		`^${pattern
			.toLowerCase()
			.replace(/[.+?^${}()|[\]\\]/g, '\\$&')
			.replace(/\*/g, '.*')}$`
	).test(value.toLowerCase());

/** Whether one condition holds. Asset fields leave the other kinds out; metadata and term conditions are not modelled, so they pass. */
const holds = (r: Result, field: string, value: string) => {
	const v = value.toLowerCase();
	if (field === 'kind') return r.kind === v;
	if (['type', 'provider', 'name', 'tag', 'tags'].includes(field)) {
		if (r.kind !== 'asset') return false;
		if (field === 'type') return r.a.type.toLowerCase() === v;
		if (field === 'provider') return r.a.provider === v;
		if (field === 'name') return glob(value, r.a.name);
		return tagsOf(r.a).includes(v);
	}
	return true;
};

/** The facet each bar field ticks, as FACET_FIELDS in the app. */
const facetOf: Record<string, keyof Selected> = {
	kind: 'kind',
	type: 'type',
	provider: 'provider',
	tag: 'tags',
	tags: 'tags'
};

type Selected = { kind?: string[]; type?: string[]; provider?: string[]; tags?: string[] };

/**
 * A small client filter: `AND` joins parts, a part may be `(a OR b)` conditions, and every other
 * word is free text that must appear in the name or description. With no words it is a listing,
 * most popular first; otherwise name matches rank first.
 */
const search = (query: string) => {
	const parts = query
		.split(/\s+AND\s+/i)
		.map((p) => p.trim().replace(/^\(|\)$/g, ''))
		.filter(Boolean);
	const groups: { field: string; value: string }[][] = [];
	const words: string[] = [];
	for (const part of parts) {
		const alternatives = part.split(/\s+OR\s+/i).map((x) => condition.exec(x.trim()));
		if (alternatives.every(Boolean))
			groups.push(alternatives.map((m) => ({ field: m![1]!, value: m![2]! })));
		else
			words.push(
				...part
					.replace(/^NOT\s+/i, '')
					.toLowerCase()
					.split(/\s+/)
					.filter((w) => w && !w.startsWith('@'))
			);
	}
	const matches = pool.filter((r) => {
		const name = nameOf(r).toLowerCase();
		const text = textOf(r).toLowerCase();
		return (
			groups.every((g) => g.some((c) => holds(r, c.field, c.value))) &&
			words.every((w) => name.includes(w) || text.includes(w))
		);
	});
	const score = (r: Result) => {
		const name = nameOf(r).toLowerCase();
		const named = words.filter((w) => name.includes(w)).length;
		const exact = words.length > 0 && words.join(' ') === name ? 10 : 0;
		return exact + named * 2 + (r.kind === 'asset' ? (r.a.views7d ?? 0) / 1_000 : 0);
	};
	const selected: Selected = {};
	for (const c of groups.flat()) {
		const key = facetOf[c.field];
		if (key) (selected[key] ??= []).push(c.value.toLowerCase());
	}
	return {
		rows: matches
			.map((r, i) => ({ r, i, s: score(r) }))
			.sort((a, b) => b.s - a.s || a.i - b.i)
			.map((x) => x.r),
		selected
	};
};

/* ------------------------------------------------------------------ facets */

type Option = { value: string; label: string; count: number; icon?: string; provider?: string };

/** Counts per value, most first, as the app's facets list them. */
const tally = (values: { value: string; label: string; icon?: string; provider?: string }[]) => {
	const out = new Map<string, Option>();
	for (const v of values) {
		const o = out.get(v.value) ?? { ...v, count: 0 };
		o.count++;
		out.set(v.value, o);
	}
	return [...out.values()].sort((a, b) => b.count - a.count);
};

const facetsFor = (rows: Result[]) => {
	const found = rows.flatMap((r) => (r.kind === 'asset' ? [r.a] : []));
	const kinds = (k: string) => rows.filter((r) => r.kind === k).length;
	return {
		kind: [
			{ value: 'asset', label: 'Assets', icon: 'database', count: kinds('asset') },
			{ value: 'data_product', label: 'Data products', icon: 'package-2', count: kinds('data_product') },
			{ value: 'glossary', label: 'Glossary terms', icon: 'menu-book', count: kinds('glossary') },
			{ value: 'team', label: 'Teams', icon: 'groups', count: kinds('team') }
		],
		type: tally(found.map((a) => ({ value: a.type.toLowerCase(), label: a.type }))),
		provider: tally(
			found.map((a) => ({ value: a.provider, label: providerLabel(a.provider), provider: a.provider }))
		),
		tags: tally(found.flatMap((a) => tagsOf(a).map((t) => ({ value: t, label: t })))),
		owner: tally(
			rows.flatMap((r) =>
				ownersOf(r).map((o) => ({
					value: o,
					label: owner(o).name,
					icon: owner(o).kind === 'team' ? 'groups' : 'person'
				}))
			)
		)
	};
};

type Facets = ReturnType<typeof facetsFor>;

const ALL_KINDS = 4;

/** Type, provider and tags only show while assets are among the kinds; Owner stays folded until it is used. */
const facetSidebar = (f: Facets, selected: Selected) => {
	const kinds = selected.kind ?? [];
	const narrowed = kinds.length > 0 && kinds.length < ALL_KINDS;
	const showAssetFacets = !kinds.length || kinds.includes('asset');
	return html`<mm-facet-sidebar heading="Filters" hide-applied>
		<mm-facet
			name="kind"
			label="Kind"
			.options=${f.kind}
			.value=${narrowed ? kinds : []}
		></mm-facet>
		${showAssetFacets
			? html`<mm-facet
						name="type"
						label="Type"
						.options=${f.type}
						.value=${selected.type ?? []}
					></mm-facet>
					<mm-facet
						name="provider"
						label="Provider"
						.options=${f.provider}
						.value=${selected.provider ?? []}
					></mm-facet>
					<mm-facet
						name="tags"
						label="Tags"
						.options=${f.tags}
						.value=${selected.tags ?? []}
					></mm-facet>`
			: nothing}
		<mm-facet name="owner" label="Owner" .options=${f.owner} .value=${[]} collapsed></mm-facet>
	</mm-facet-sidebar>`;
};

/* ------------------------------------------------------------------ page */

const crumbs = [{ label: 'Discover' }];

const header = html`<mm-page-header compact heading="Discover">
	<mm-button slot="actions" variant="secondary" icon="add" href="#assets/new">New asset</mm-button>
</mm-page-header>`;

/** The fields the backend query parser accepts, as catalogFields in the app, with the values the facets know. */
const fieldsFor = (f: Facets): SearchBarField[] => [
	{
		value: 'kind',
		label: 'Kind',
		operators: ['='],
		values: [
			{ value: 'asset', label: 'Asset' },
			{ value: 'data_product', label: 'Data product' },
			{ value: 'glossary', label: 'Glossary term' },
			{ value: 'team', label: 'Team' }
		]
	},
	{ value: 'type', label: 'Type', values: f.type.map((t) => ({ value: t.value, label: t.label })) },
	{
		value: 'provider',
		label: 'Provider',
		values: f.provider.map((p) => ({ value: p.value, label: p.label }))
	},
	{ value: 'name', label: 'Name' },
	{ value: 'term', label: 'Glossary term', kind: 'term' },
	{ value: 'tag', label: 'Tag', values: f.tags.map((t) => t.value) }
];

const searchBar = (query: string, f: Facets) =>
	html`<mm-search-bar
		label="Search the catalog"
		placeholder="Search assets, glossary, teams and data products, or type @ to filter"
		value=${query}
		.fields=${fieldsFor(f)}
	></mm-search-bar>`;

const resultsHead = (count: number) =>
	html`<div class="mm-cluster mm-cluster--between">
		<mm-text variant="dense" tone="secondary" role="status"
			>${plural(count, 'result', 'results')}</mm-text
		>
		<mm-select label="Sort by" hide-label value="relevance" style="inline-size:12rem">
			<mm-option value="relevance">Most relevant</mm-option>
			<mm-option value="popular">Most popular</mm-option>
			<mm-option value="updated">Recently updated</mm-option>
			<mm-option value="name">Name</mm-option>
		</mm-select>
	</div>`;

const PAGE_SIZE = 20;

const resultsSection = (rows: Result[], selectedMrn: string, hasFilters: boolean) =>
	html`<section
		aria-label="Search results"
		class="mm-stack"
		style="--mm-gap:var(--space-4);min-width:0"
	>
		${rows.length
			? html`${resultsHead(rows.length)}
					<ul
						class="mm-list"
						aria-label="Search results"
						style="border-block:1px solid var(--mm-rule)"
					>
						${rows
							.slice(0, PAGE_SIZE)
							.map((r) => resultRow(r, r.kind === 'asset' && r.a.mrn === selectedMrn))}
					</ul>
					${rows.length > PAGE_SIZE
						? html`<mm-pagination
								total=${rows.length}
								page-size=${PAGE_SIZE}
								page="1"
								label="Search results pages"
							></mm-pagination>`
						: nothing}`
			: html`<mm-empty-state
					heading="No matches"
					description="Check the spelling, or search by column name."
					seed="5"
				>
					${hasFilters
						? html`<mm-button slot="action" variant="secondary" icon="filter-list"
								>Clear filters</mm-button
							>`
						: nothing}
				</mm-empty-state>`}
	</section>`;

/* ------------------------------------------------------------------ blade */

/** Glossary terms the mockups link to these assets. */
const meaningOf: Record<string, { id: string; name: string }[]> = {
	orders: [{ id: 'gmv', name: 'Gross merchandise value' }],
	orders_daily: [
		{ id: 'gmv', name: 'Gross merchandise value' },
		{ id: 'active-customer', name: 'Active customer' }
	],
	ORDERS_DAILY: [
		{ id: 'gmv', name: 'Gross merchandise value' },
		{ id: 'active-customer', name: 'Active customer' }
	],
	customers_dim: [
		{ id: 'active-customer', name: 'Active customer' },
		{ id: 'pii', name: 'Personal data (PII)' }
	],
	'Revenue overview': [{ id: 'gmv', name: 'Gross merchandise value' }]
};

/** When each source's scheduled pipeline last synced it. Freshness shows the later of this and the asset's own update, as last_sync_at does in the app. */
const lastSync: Record<string, string> = {
	snowflake: ago(9),
	postgresql: ago(17),
	dbt: ago(326),
	kafka: ago(4),
	bigquery: ago(47),
	looker: ago(52),
	airflow: ago(31),
	clickhouse: ago(2)
};
const freshness = (a: Asset) => {
	const synced = lastSync[a.provider];
	return synced && synced > a.updated ? synced : a.updated;
};

/** Assets a pipeline catalogues record the run that created them, which for a scheduled pipeline is the scheduler; agents are registered by a person. */
const createdBy = (a: Asset) => (a.provider === 'agent' ? 'Jane Doe' : 'scheduler');
const CATALOGUED = '2025-09-16T08:12:00Z';

/** The asset's nearest neighbours in the orders_daily lineage, stubs left out, upstream first. */
const neighbours = (a: Asset) => {
	const node = ordersLineage.nodes.find(
		(x) => x.mrn === a.mrn || x.name.toLowerCase() === a.name.toLowerCase()
	);
	if (!node) return [];
	const byId = (id: string) => ordersLineage.nodes.find((x) => x.id === id)!;
	const real = (x: (typeof ordersLineage.nodes)[number]) => !('state' in x && x.state === 'stub');
	const what = (dir: string, x: { type: string; provider: string }) =>
		`${dir} · ${providerLabel(x.provider)} ${x.type}`;
	return [
		...ordersLineage.edges
			.filter((e) => e.target === node.id)
			.map((e) => byId(e.source))
			.filter(real)
			.map((x) => ({ name: x.name, provider: x.provider, what: what('Upstream', x) })),
		...ordersLineage.edges
			.filter((e) => e.source === node.id)
			.map((e) => byId(e.target))
			.filter(real)
			.map((x) => ({ name: x.name, provider: x.provider, what: what('Downstream', x) }))
	].slice(0, 8);
};

/**
 * The preview answers, in order: what is it and how fresh, what it is about, who to ask, what it
 * means and where it flows. More details fold away, and the next steps sit in the footer.
 */
const assetBlade = (a: Asset) => {
	const flows = neighbours(a);
	const terms = meaningOf[a.name] ?? [];
	const tags = tagsOf(a).filter((t) => t !== 'pii');
	const size = [
		a.rows ? `${compact(a.rows)} ${a.rows === 1 ? 'row' : 'rows'}` : '',
		a.columns ? plural(a.columns, 'column', 'columns') : ''
	]
		.filter(Boolean)
		.join(' · ');
	const queryable = ['table', 'view'].includes(a.type.toLowerCase());
	return html`<mm-details-blade mode="overlay" open resizable>
		<span slot="heading" class="mm-cluster" style="--mm-gap:var(--space-2);flex-wrap:nowrap">
			<mm-provider-logo provider=${a.provider} size="20" decorative></mm-provider-logo>${a.name}
		</span>
		<span slot="actions" class="mm-cluster" style="--mm-gap:var(--space-1)">
			<mm-button
				variant="ghost"
				icon="notifications"
				label="Subscribe to notifications"
				pressed="false"
			></mm-button>
		</span>
		<mm-menu-button
			slot="actions"
			variant="ghost"
			size="sm"
			icon="more-horiz"
			icon-only
			label="More actions for ${a.name}"
		>
			<mm-menu>
				<mm-menu-item icon="delete" danger value="delete">Delete asset</mm-menu-item>
			</mm-menu>
		</mm-menu-button>
		<mm-blade-section heading="At a glance">
			<mm-description-list>
				<mm-description term="Type">${a.type} · ${providerLabel(a.provider)}</mm-description>
				<mm-description term="Freshness"
					>${a.stale
						? html`<mm-status-pill state="stale"></mm-status-pill>`
						: `Updated ${relTime(freshness(a))}`}</mm-description
				>
				<mm-description term="Personal data"
					>${a.pii ? html`<mm-tag variant="pii"></mm-tag>` : 'None'}</mm-description
				>
				<mm-description term="Popularity"
					><mm-popularity level=${popularity(a)}></mm-popularity
				></mm-description>
				${size ? html`<mm-description term="Size">${size}</mm-description>` : nothing}
			</mm-description-list>
		</mm-blade-section>
		${a.description
			? html`<mm-blade-section heading="About">
					<mm-text variant="dense" block style="white-space:pre-wrap">${a.description}</mm-text>
				</mm-blade-section>`
			: nothing}
		${a.owners.length
			? html`<mm-blade-section heading="Who to ask">
					<div class="mm-stack" style="--mm-gap:var(--space-3)">
						${a.owners.map(
							(o) =>
								html`<mm-owner-chip
									name=${owner(o).name}
									kind=${owner(o).kind === 'team' ? 'team' : 'person'}
									email=${owner(o).email ?? ''}
								></mm-owner-chip>`
						)}
					</div>
				</mm-blade-section>`
			: nothing}
		${terms.length || tags.length
			? html`<mm-blade-section heading="What it means">
					<div class="mm-cluster" style="--mm-gap:var(--space-2)">
						${terms.map((t) => html`<mm-tag href="#glossary/${t.id}">${t.name}</mm-tag>`)}
						${tags.map((t) => html`<mm-tag>${t}</mm-tag>`)}
					</div>
				</mm-blade-section>`
			: nothing}
		${flows.length
			? html`<mm-blade-section heading="Where it flows" flush>
					<mm-link slot="actions" href="#lineage/${a.mrn}">Open lineage</mm-link>
					<mm-list dense aria-label="Nearest lineage">
						${flows.map(
							(f) =>
								html`<mm-list-item href="#asset/${f.name}" secondary=${f.what}
									><mm-provider-logo
										slot="start"
										provider=${f.provider}
										decorative
									></mm-provider-logo
									>${f.name}</mm-list-item
								>`
						)}
					</mm-list>
				</mm-blade-section>`
			: nothing}
		<mm-blade-section heading="More details" collapsible>
			<mm-description-list>
				<mm-description term="MRN" mono>${a.mrn}</mm-description>
				<mm-description term="Created by">${createdBy(a)}</mm-description>
				<mm-description term="Created">${dateTime(CATALOGUED)}</mm-description>
				<mm-description term="Updated">${dateTime(a.updated)}</mm-description>
			</mm-description-list>
		</mm-blade-section>
		<div slot="footer" class="mm-cluster" style="--mm-gap:var(--space-2)">
			<mm-button variant="primary" icon="open-in-full" href="#asset/${a.mrn}"
				>Open asset</mm-button
			>
			${queryable
				? html`<mm-button
						icon="terminal"
						href="#query?q=${encodeURIComponent(`SELECT *\nFROM ${a.name}\nLIMIT 10`)}"
						>Query</mm-button
					>`
				: nothing}
		</div>
	</mm-details-blade>`;
};

/* ------------------------------------------------------------------ screen */

export interface DiscoverState {
	/**
	 * What is in the search bar, written as the app composes it: free text, then the facet filters
	 * as conditions, such as `@kind = asset AND @type = table`. Empty lists the whole catalog.
	 */
	query: string;
	/** Name of the asset to select and preview in the blade. */
	selected?: string;
}

export const discoverScreen = (s: DiscoverState): TemplateResult => {
	const found = search(s.query);
	const pick = s.selected
		? (found.rows.find((r) => r.kind === 'asset' && r.a.name === s.selected) ??
			pool.find((r) => r.kind === 'asset' && r.a.name === s.selected))
		: undefined;
	const selected = pick?.kind === 'asset' ? pick.a : undefined;
	const facets = facetsFor(found.rows);
	const hasFilters = Object.values(found.selected).some((v) => v && v.length > 0);
	return shell({
		current: 'discover',
		crumbs,
		content: html`<div class="mm-stack" style="--mm-gap:var(--space-5)">
			${header} ${searchBar(s.query, facets)}
			<div class="mm-split mm-split--sidebar">
				${facetSidebar(facets, found.selected)}
				${resultsSection(found.rows, selected?.mrn ?? '', hasFilters)}
			</div>
		</div>`,
		blade: selected ? assetBlade(selected) : undefined
	});
};
