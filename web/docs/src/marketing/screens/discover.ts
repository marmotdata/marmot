/**
 * Discover: every kind in one result list, the search bar (search and filters), facets,
 * pagination and the asset preview blade. The mockup draws fixed results; here the list is the
 * catalog filtered by whatever the demo has typed.
 */
import { html, nothing, type TemplateResult } from 'lit';
import type { SearchBarField } from '@marmotdata/elements/classes/search-bar.js';
import { shell } from './shell';
import { assets, glossary, ordersLineage, owner, products, type Asset } from './data/catalog';
import {
	orderResults,
	facetTypes,
	facetProviders,
	facetTags,
	facetOwners,
	compact,
	rel,
	n,
	longStamp,
	providerName,
	popularity
} from './data/asset';

/* ------------------------------------------------------------------ parts */

const crumbs = [{ label: 'Discover' }];

const header = (count?: number) =>
	html`<mm-page-header compact heading="Discover" .count=${count}>
		<mm-button slot="actions" variant="secondary" icon="add" href="#discover/new"
			>New asset</mm-button
		>
	</mm-page-header>`;

const searchFields: SearchBarField[] = [
	{
		value: 'type',
		label: 'Type',
		values: [
			{ value: 'table', label: 'Table' },
			{ value: 'view', label: 'View' },
			{ value: 'topic', label: 'Topic' },
			{ value: 'dashboard', label: 'Dashboard' }
		]
	},
	{
		value: 'provider',
		label: 'Provider',
		values: [
			{ value: 'snowflake', label: 'Snowflake' },
			{ value: 'postgresql', label: 'PostgreSQL' },
			{ value: 'bigquery', label: 'BigQuery' },
			{ value: 'kafka', label: 'Kafka' }
		]
	},
	{ value: 'name', label: 'Name' },
	{
		value: 'term',
		label: 'Glossary term',
		kind: 'term',
		values: ['Gross merchandise value', 'Net revenue', 'Active customer']
	},
	{
		value: 'kind',
		label: 'Kind',
		values: [
			{ value: 'asset', label: 'Asset' },
			{ value: 'glossary', label: 'Glossary term' },
			{ value: 'team', label: 'Team' },
			{ value: 'product', label: 'Data product' }
		]
	},
	{ value: 'metadata.team', label: 'Team' },
	{ value: 'metadata.row_count', label: 'Row count', kind: 'number' }
];

/** The search bar is the query builder: conditions show as tokens, and the facets edit the same query. */
const searchBar = (query: string) =>
	html`<mm-search-bar
		label="Search the catalog"
		placeholder="Search assets, glossary, teams and data products, or type @ to filter"
		value=${query}
		.fields=${searchFields}
	></mm-search-bar>`;

const facetKinds = [
	{ value: 'asset', label: 'Assets', icon: 'database', count: 23 },
	{ value: 'glossary', label: 'Glossary terms', icon: 'menu-book', count: 2 },
	{ value: 'product', label: 'Data products', icon: 'package-2', count: 1 },
	{ value: 'team', label: 'Teams', icon: 'groups', count: 1 }
];

type Selected = {
	kind?: string[];
	type?: string[];
	provider?: string[];
	tags?: string[];
	owner?: string[];
};
const facets = (selected: Selected = {}) =>
	html`<mm-facet-sidebar heading="Filters" hide-applied>
		<mm-facet
			name="kind"
			label="Kind"
			.options=${facetKinds}
			.value=${selected.kind ?? []}
		></mm-facet>
		<mm-facet
			name="type"
			label="Type"
			.options=${facetTypes}
			.value=${selected.type ?? []}
		></mm-facet>
		<mm-facet
			name="provider"
			label="Provider"
			.options=${facetProviders}
			.value=${selected.provider ?? []}
		></mm-facet>
		<mm-facet
			name="tags"
			label="Tags"
			.options=${facetTags}
			.value=${selected.tags ?? []}
		></mm-facet>
		<mm-facet
			name="owner"
			label="Owner"
			.options=${facetOwners}
			.value=${selected.owner ?? []}
			collapsed
		></mm-facet>
	</mm-facet-sidebar>`;

/** Plain caption text, not chips, so rows stay calm. */
const captionFor = (a: Asset) =>
	[
		a.type,
		providerName[a.provider] ?? a.provider,
		a.columns ? `${a.columns} columns` : '',
		a.rows ? `${compact(a.rows)} rows` : '',
		a.owners.map((o) => owner(o).name).join(', '),
		...a.tags.filter((t) => t !== 'pii'),
		`Updated ${rel(a.updated)}`
	]
		.filter(Boolean)
		.join(' · ');

/** The MRN and description live in the blade, not the row. */
const resultRow = (a: Asset, selected = false) =>
	html`<li
		style="align-items:flex-start;gap:var(--space-3);padding-block:var(--space-3)"
		aria-current=${selected ? 'true' : nothing}
	>
		<mm-provider-logo
			provider=${a.provider}
			size="20"
			decorative
			style="margin-block-start:2px"
		></mm-provider-logo>
		<div class="mm-stack" style="--mm-gap:var(--space-1);flex:1;min-width:0">
			<div class="mm-cluster">
				<a class="mm-link mm-link--quiet" href="#asset/${a.mrn}" title=${a.description}
					>${a.name}</a
				>
				${a.pii ? html`<mm-tag variant="pii"></mm-tag>` : nothing}
				${a.certified ? html`<mm-tag variant="certified"></mm-tag>` : nothing}
				${a.stale ? html`<mm-status-pill state="stale"></mm-status-pill>` : nothing}
			</div>
			<mm-text variant="caption" tone="secondary" block>${captionFor(a)}</mm-text>
		</div>
		<mm-popularity level=${popularity(a)} style="margin-block-start:4px"></mm-popularity>
	</li>`;

type Term = (typeof glossary)[number];
type Product = (typeof products)[number];

/** Rows for the other kinds share the asset row's shape: a lead icon, the name, one caption. */
const otherRow = (
	icon: string,
	href: string,
	name: string,
	title: string,
	caption: string,
	extra: TemplateResult | typeof nothing = nothing
) =>
	html`<li style="align-items:flex-start;gap:var(--space-3);padding-block:var(--space-3)">
		<mm-icon
			name=${icon}
			size="20"
			aria-hidden="true"
			style="color:var(--mm-text-2);margin-block-start:2px"
		></mm-icon>
		<div class="mm-stack" style="--mm-gap:var(--space-1);flex:1;min-width:0">
			<div class="mm-cluster">
				<a class="mm-link mm-link--quiet" href=${href} title=${title}>${name}</a>${extra}
			</div>
			<mm-text variant="caption" tone="secondary" block>${caption}</mm-text>
		</div>
	</li>`;

const termRow = (t: Term) =>
	otherRow(
		'menu-book',
		`#glossary/${t.id}`,
		t.name,
		t.definition,
		[
			'Glossary term',
			t.domain,
			t.owners.map((o) => owner(o).name).join(', '),
			`${t.linked} linked assets`
		].join(' · '),
		'stale' in t && t.stale ? html`<mm-status-pill state="stale"></mm-status-pill>` : nothing
	);
const productRow = (p: Product) =>
	otherRow(
		'package-2',
		`#products/${p.id}`,
		p.name,
		p.description,
		['Data product', p.owners.map((o) => owner(o).name).join(', '), `${p.assets} assets`].join(
			' · '
		)
	);
const teamRow = (id: string, members: number, assets: number) =>
	otherRow(
		'groups',
		`#teams/${id}`,
		owner(id).name,
		'',
		['Team', `${members} members`, `${assets} assets`].join(' · ')
	);

type Result =
	| { kind: 'asset'; a: Asset }
	| { kind: 'term'; t: Term }
	| { kind: 'product'; p: Product }
	| { kind: 'team'; id: string; members: number; assets: number };

const resultItem = (r: Result, selectedMrn: string) =>
	r.kind === 'asset'
		? resultRow(r.a, r.a.mrn === selectedMrn)
		: r.kind === 'term'
			? termRow(r.t)
			: r.kind === 'product'
				? productRow(r.p)
				: teamRow(r.id, r.members, r.assets);

const results = (rows: Result[], selectedMrn = '') =>
	html`<ul
		class="mm-list"
		aria-label="Search results"
		style="border-block:1px solid var(--mm-rule)"
	>
		${rows.map((r) => resultItem(r, selectedMrn))}
	</ul>`;

const resultsHead = (count: number, noun = 'results') =>
	html`<div class="mm-cluster mm-cluster--between">
		<mm-text variant="dense" tone="secondary" role="status">${n(count)} ${noun}</mm-text>
		<mm-select label="Sort by" hide-label value="relevance" style="width:12rem">
			<mm-option value="relevance">Most relevant</mm-option>
			<mm-option value="popular">Most popular</mm-option>
			<mm-option value="updated">Recently updated</mm-option>
			<mm-option value="name">Name</mm-option>
		</mm-select>
	</div>`;

const layout = (side: TemplateResult, main: TemplateResult) =>
	html`<div class="mm-split mm-split--sidebar">
		${side}
		<section aria-label="Results" class="mm-stack" style="--mm-gap:var(--space-4);min-width:0">
			${main}
		</section>
	</div>`;

const page = (o: { query: string; side: TemplateResult; main: TemplateResult; count?: number }) =>
	html`<div class="mm-stack" style="--mm-gap:var(--space-5)">
		${header(o.count)} ${searchBar(o.query)} ${layout(o.side, o.main)}
	</div>`;

/* ------------------------------------------------------------------ search */

/** Everything a search can find: the catalog, the extra "orders" results, glossary, products and a team. */
const pool: Result[] = [
	...[...assets, ...orderResults.filter((r) => !assets.some((a) => a.mrn === r.mrn))].map(
		(a): Result => ({ kind: 'asset', a })
	),
	...glossary.map((t): Result => ({ kind: 'term', t })),
	...products.map((p): Result => ({ kind: 'product', p })),
	{ kind: 'team', id: 'ae', members: 6, assets: 9 }
];

const kindOf = (r: Result) => (r.kind === 'term' ? 'glossary' : r.kind);
const nameOf = (r: Result) =>
	r.kind === 'asset'
		? r.a.name
		: r.kind === 'term'
			? r.t.name
			: r.kind === 'product'
				? r.p.name
				: owner(r.id).name;
const textOf = (r: Result) =>
	r.kind === 'asset'
		? r.a.description
		: r.kind === 'term'
			? r.t.definition
			: r.kind === 'product'
				? r.p.description
				: '';

/** `@field = value` or `@field: value`, with an optional `*` wildcard in the value. */
const condition = /^@([\w.]+)\s*(?:=|:)\s*"?([^"]*)"?$/;
const glob = (pattern: string, value: string) =>
	new RegExp(
		`^${pattern
			.toLowerCase()
			.replace(/[.+?^${}()|[\]\\]/g, '\\$&')
			.replace(/\*/g, '.*')}$`
	).test(value.toLowerCase());

/** Whether one condition holds. Metadata and term conditions are not modelled, so they pass. */
const holds = (r: Result, field: string, value: string) => {
	const v = value.toLowerCase();
	if (field === 'kind') return kindOf(r) === v;
	if (field === 'type' || field === 'provider' || field === 'name' || field === 'tags') {
		if (r.kind !== 'asset') return false;
		if (field === 'type') return r.a.type.toLowerCase() === v;
		if (field === 'provider') return r.a.provider === v;
		if (field === 'tags') return r.a.tags.includes(v);
		return glob(value, r.a.name);
	}
	return true;
};

/**
 * A small client filter: `AND` joins parts, a part may be `(a OR b)` conditions, and every other
 * word is free text that must appear in the name or description. Name matches rank first.
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
	return {
		rows: matches
			.map((r, i) => ({ r, i, s: score(r) }))
			.sort((a, b) => b.s - a.s || a.i - b.i)
			.map((x) => x.r),
		facets: Object.fromEntries(
			['kind', 'type', 'provider', 'tags'].map((f) => [
				f,
				groups
					.flat()
					.filter((c) => c.field === f)
					.map((c) => c.value.toLowerCase())
			])
		) as Selected
	};
};

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

/** The pipeline that catalogues each provider's assets. */
const createdBy: Record<string, string> = {
	postgresql: 'postgres-shop',
	snowflake: 'snowflake-finance',
	dbt: 'dbt-analytics',
	kafka: 'kafka-orders',
	looker: 'looker-prod',
	bigquery: 'bigquery-web'
};

/** The asset's neighbours in the orders_daily lineage, for "Where it flows". */
const neighbours = (a: Asset) => {
	const node = ordersLineage.nodes.find((x) => x.mrn === a.mrn || x.name === a.name);
	if (!node) return [];
	const byId = (id: string) => ordersLineage.nodes.find((x) => x.id === id)!;
	const what = (dir: string, x: { type: string; provider: string }) =>
		`${dir} · ${providerName[x.provider] ?? x.provider} ${x.type.toLowerCase()}`;
	return [
		...ordersLineage.edges
			.filter((e) => e.target === node.id)
			.map((e) => byId(e.source))
			.map((x) => ({ name: x.name, provider: x.provider, what: what('Upstream', x) })),
		...ordersLineage.edges
			.filter((e) => e.source === node.id)
			.map((e) => byId(e.target))
			.map((x) => ({ name: x.name, provider: x.provider, what: what('Downstream', x) }))
	].slice(0, 5);
};

/**
 * The preview answers, in order: what is it, can I trust it, who do I ask, what does it mean,
 * where does it flow. More details fold away, and the next steps sit in the footer.
 */
const assetBlade = (a: Asset) => {
	const flows = neighbours(a);
	return html`<mm-details-blade mode="overlay" open resizable>
		<span slot="heading" class="mm-cluster" style="--mm-gap:var(--space-2);flex-wrap:nowrap">
			<mm-provider-logo provider=${a.provider} size="20" decorative></mm-provider-logo>${a.name}
		</span>
		<mm-button
			slot="actions"
			variant="ghost"
			size="sm"
			icon="notifications"
			label="Subscribe to changes"
		></mm-button>
		<mm-blade-section heading="At a glance">
			<mm-description-list>
				<mm-description term="Type"
					>${a.type} · ${providerName[a.provider] ?? a.provider}</mm-description
				>
				<mm-description term="Freshness"
					>${a.stale ? html`<mm-status-pill state="stale"></mm-status-pill>` : `Updated ${rel(a.updated)}`}</mm-description
				>
				<mm-description term="Personal data"
					>${a.pii ? html`<mm-tag variant="pii"></mm-tag>` : 'None'}</mm-description
				>
				<mm-description term="Popularity"
					><mm-popularity level=${popularity(a)}></mm-popularity
				></mm-description>
				${a.rows ? html`<mm-description term="Size">${compact(a.rows)} rows · ${a.columns} columns</mm-description>` : nothing}
			</mm-description-list>
		</mm-blade-section>
		<mm-blade-section heading="About">
			<mm-text variant="dense" block>${a.description}</mm-text>
		</mm-blade-section>
		<mm-blade-section heading="Who to ask">
			<div class="mm-stack" style="--mm-gap:var(--space-3)">
				${a.owners.map((o) => html`<mm-owner-chip name=${owner(o).name} kind=${owner(o).kind}></mm-owner-chip>`)}
			</div>
		</mm-blade-section>
		<mm-blade-section heading="What it means">
			<div class="mm-cluster">
				${(meaningOf[a.name] ?? []).map((t) => html`<mm-tag href="#glossary/${t.id}">${t.name}</mm-tag>`)}
				${a.tags.filter((t) => t !== 'pii').map((t) => html`<mm-tag>${t}</mm-tag>`)}
			</div>
		</mm-blade-section>
		${
			flows.length
				? html`<mm-blade-section heading="Where it flows" flush>
						<mm-link slot="actions" href="#lineage/${a.mrn}">Open lineage</mm-link>
						<mm-list dense aria-label="Nearest lineage">
							${flows.map((f) => html`<mm-list-item href="#asset/${f.name}" secondary=${f.what}><mm-provider-logo slot="start" provider=${f.provider} decorative></mm-provider-logo>${f.name}</mm-list-item>`)}
						</mm-list>
					</mm-blade-section>`
				: nothing
		}
		<mm-blade-section heading="More details" collapsible>
			<mm-description-list>
				<mm-description term="MRN" mono>${a.mrn}</mm-description>
				${createdBy[a.provider] ? html`<mm-description term="Created by">${createdBy[a.provider]}</mm-description>` : nothing}
				<mm-description term="Updated">${longStamp(a.updated)}</mm-description>
			</mm-description-list>
		</mm-blade-section>
		<div slot="footer" class="mm-cluster" style="--mm-gap:var(--space-2)">
			<mm-button variant="primary" icon="open-in-full" href="#asset/${a.mrn}">Open asset</mm-button>
			<mm-button icon="terminal" href="#query?asset=${encodeURIComponent(a.mrn)}">Query</mm-button>
		</div>
	</mm-details-blade>`;
};

/* ------------------------------------------------------------------ screen */

export interface DiscoverState {
	/** What is in the search bar, such as `revenue` or `orders AND @type = table`. */
	query: string;
	/** Name of the asset to select and preview in the blade. */
	selected?: string;
}

const PAGE = 10;

export const discoverScreen = (s: DiscoverState): TemplateResult => {
	const found = search(s.query);
	const pick = s.selected
		? (found.rows.find((r) => r.kind === 'asset' && r.a.name === s.selected) ??
			pool.find((r) => r.kind === 'asset' && r.a.name === s.selected))
		: undefined;
	const selected = pick?.kind === 'asset' ? pick.a : undefined;
	const count = found.rows.length;
	const main = count
		? html`${resultsHead(count)} ${results(found.rows.slice(0, PAGE), selected?.mrn)}
			${count > PAGE ? html`<mm-pagination total=${count} page-size=${PAGE} page="1" label="Search results pages"></mm-pagination>` : nothing}`
		: html`<mm-empty-state
				heading="No matches"
				description="Check the spelling or search by column name."
				seed="5"
			></mm-empty-state>`;
	return shell({
		current: 'discover',
		crumbs,
		content: html`${page({ query: s.query, side: facets(found.facets), main })}${selected ? assetBlade(selected) : nothing}`
	});
};
