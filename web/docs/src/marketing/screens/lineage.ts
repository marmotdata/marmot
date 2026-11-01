/**
 * Full-screen lineage of orders_daily, as the product draws it once "Full screen" is pressed on the
 * asset's Lineage tab: the view covers the sidebar and top bar, and the selected asset opens in a
 * blade beside the canvas rather than in the shell's blade slot. Observed edges are off, as in the
 * product, so chatty agents do not crowd pipeline lineage.
 */
import { html, nothing, type TemplateResult } from 'lit';
import type {
	LineageGraph,
	LineageGraphEdge,
	LineageGraphNode
} from '@marmotdata/elements/classes/lineage-canvas.js';
import { NOW, ago, assets, owner } from './data/catalog';
import { ordersGraph, orderResults, n, providerName } from './data/asset';

/* ------------------------------------------------------------------ graph */

// In the product only agent lookups and queries are recorded as observed edges. Plugins and
// OpenLineage runs both write declared edges, so the graph here declares everything else.
const declare = (e: LineageGraphEdge): LineageGraphEdge =>
	e.kind === 'AGENT_LOOKUP' || e.origin === undefined || e.origin === 'declared'
		? e
		: { source: e.source, target: e.target, kind: e.kind, origin: 'declared', ...(e.job ? { job: e.job } : {}) };

/**
 * orders_daily's lineage with two more downstream assets: a Spark job that reported itself through
 * OpenLineage, reading orders_daily and writing a table no plugin has catalogued yet, and a
 * Metabase dashboard built on refund_rates.
 */
const graph: LineageGraph = {
	currentId: ordersGraph.currentId,
	nodes: [
		// A dashboard has nothing downstream of it, so it carries no "+3 downstream" handle.
		...ordersGraph.nodes.map(({ hiddenDownstream, ...x }) =>
			x.id === 'revenue' ? x : { ...x, ...(hiddenDownstream ? { hiddenDownstream } : {}) }
		),
		{
			id: 'enrich_orders',
			mrn: 'mrn://job/spark/lake.enrich_orders',
			name: 'enrich_orders',
			type: 'job',
			provider: 'spark'
		},
		{
			id: 'refunds_by_category',
			mrn: 'mrn://dashboard/metabase/finance.refunds_by_category',
			name: 'Refunds by category',
			type: 'dashboard',
			provider: 'metabase'
		}
	],
	edges: [
		...ordersGraph.edges.filter((e) => e.target !== 'ol_stub').map(declare),
		{ source: 'orders_daily', target: 'enrich_orders', kind: 'FEEDS', origin: 'declared' },
		{ source: 'enrich_orders', target: 'ol_stub', kind: 'PRODUCES', origin: 'declared' },
		{ source: 'refund_rates', target: 'refunds_by_category', kind: 'FEEDS', origin: 'declared' }
	]
};

/** Who declared each edge: the pipeline whose run wrote it, or the person who added it by hand. OpenLineage edges name no one. */
const declaredBy: Record<string, string> = {
	'shop_db>orders': 'postgres-shop',
	'orders>orders_hourly_load': 'airflow-prod',
	'shop.orders.v2>orders_hourly_load': 'airflow-prod',
	'orders_hourly_load>stg_orders': 'airflow-prod',
	'stg_orders>orders_daily': 'dbt-analytics',
	'customers_dim>orders_daily': 'dbt-analytics',
	'orders_daily>refund_rates': 'dbt-analytics',
	'orders_daily>revenue': 'Jane Doe',
	'refund_rates>refunds_by_category': 'metabase-prod'
};

/** The outcome of each asset's latest run, from plugin run history or OpenLineage run events. */
const lastRun: Record<string, 'succeeded' | 'failed' | 'running'> = {
	orders_hourly_load: 'succeeded',
	stg_orders: 'succeeded',
	refund_rates: 'failed',
	enrich_orders: 'succeeded'
};

/** Steps from orders_daily, negative upstream, as the lineage response gives each node's depth. */
const depthOf = (() => {
	const depth = new Map<string, number>([[graph.currentId!, 0]]);
	const walk = (dir: 1 | -1) => {
		let frontier = [graph.currentId!];
		for (let step = 1; frontier.length; step++) {
			const next: string[] = [];
			for (const id of frontier)
				for (const e of graph.edges) {
					const other = dir === 1 ? (e.source === id ? e.target : '') : e.target === id ? e.source : '';
					if (other && !depth.has(other)) {
						depth.set(other, step * dir);
						next.push(other);
					}
				}
			frontier = next;
		}
	};
	walk(-1);
	walk(1);
	return depth;
})();

/* ------------------------------------------------------------------ assets */

interface NodeAsset {
	type: string;
	description?: string;
	owners: string[];
	tags: string[];
	rows?: number;
	columns?: number;
	updated: string;
	pii?: boolean;
	createdBy?: string;
}

/** Assets the catalog lists do not hold, as their plugins or OpenLineage would record them. */
const extra: Record<string, NodeAsset> = {
	stg_orders: {
		type: 'View',
		description: 'Orders renamed, typed and deduplicated on order_id, one row per order.',
		owners: ['ae'],
		tags: ['commerce'],
		columns: 14,
		updated: ago(326)
	},
	refund_rates: {
		type: 'Table',
		description: 'Weekly refund rate by product category, built from orders_daily.',
		owners: ['fin'],
		tags: ['finance'],
		rows: 6_240,
		columns: 7,
		updated: ago(326)
	},
	enrich_orders: {
		type: 'Job',
		description: 'Scores each day’s orders for fraud risk and writes them to the lake.',
		owners: [],
		tags: ['openlineage', 'job', 'spark'],
		updated: ago(20),
		createdBy: 'openlineage'
	},
	refunds_by_category: {
		type: 'Dashboard',
		description: 'Refund rate by product category and week, for the monthly returns review.',
		owners: ['fin', 'dev'],
		tags: ['finance'],
		updated: ago(1_900)
	}
};

const assetOf = (node: LineageGraphNode): NodeAsset | undefined => {
	if (extra[node.id]) return extra[node.id];
	return [...assets, ...orderResults].find((x) => x.mrn === node.mrn);
};

const providerLabel = (p: string) => (p === 'metabase' ? 'Metabase' : (providerName[p] ?? p));

/* ------------------------------------------------------------------ formatting */

/** "31 minutes ago", "2 days ago", then the date, as the product's relative time reads. */
const relative = (iso: string) => {
	const ms = NOW.getTime() - new Date(iso).getTime();
	const mins = Math.floor(ms / 60_000);
	const hours = Math.floor(ms / 3_600_000);
	const days = Math.floor(ms / 86_400_000);
	if (mins < 1) return 'just now';
	const rtf = new Intl.RelativeTimeFormat('en-GB', { numeric: 'always' });
	if (mins < 60) return rtf.format(-mins, 'minute');
	if (hours < 24) return rtf.format(-hours, 'hour');
	if (days < 7) return rtf.format(-days, 'day');
	return new Date(iso).toLocaleDateString('en-GB', {
		year: 'numeric',
		month: 'short',
		day: 'numeric',
		timeZone: 'UTC'
	});
};

const dateTime = (iso: string) =>
	new Date(iso).toLocaleString('en-GB', {
		year: 'numeric',
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
		timeZone: 'UTC'
	});

const plural = (count: number, one: string) => `${n(count)} ${one}${count === 1 ? '' : 's'}`;

/* ------------------------------------------------------------------ parts */

const canvas = (o: { selected?: string }) =>
	html`<mm-lineage-canvas
		.graph=${graph}
		label="Lineage of orders_daily"
		depth="10"
		max-depth="20"
		fullscreen
		editable
		selected=${o.selected ?? ''}
		style="position:relative;inset:auto;z-index:auto;height:auto;flex:1;min-width:0;border-inline:0;border-block-end:0;border-radius:0"
	></mm-lineage-canvas>`;

const relationship: Record<string, string> = {
	FEEDS: 'Feeds',
	CONTAINS: 'Contains',
	PRODUCES: 'Produces',
	REFERENCES: 'References',
	AGENT_LOOKUP: 'Looked up'
};

/**
 * The asset preview, section for section as the product's lineage blade: at a glance, about, who
 * to ask, how it connects to orders_daily and more details. Relationship and who declared it come
 * only from edges straight between the asset and orders_daily, so assets further away show their
 * direction and steps alone.
 */
const nodeBlade = (node: LineageGraphNode, details: boolean) => {
	const a = assetOf(node);
	const current = graph.currentId!;
	const edges = graph.edges.filter(
		(e) =>
			(e.source === node.id && e.target === current) ||
			(e.target === node.id && e.source === current)
	);
	const depth = depthOf.get(node.id) ?? 1;
	const kinds = [...new Set(edges.map((e) => e.kind ?? 'FEEDS'))].map((k) => relationship[k]);
	const origins = new Set(
		edges.flatMap((e) => (e.origin === 'mixed' ? ['declared', 'observed'] : [e.origin ?? 'declared']))
	);
	const recorded =
		origins.size === 0
			? ''
			: origins.size > 1
				? 'Declared and observed'
				: origins.has('observed')
					? 'Observed'
					: 'Declared';
	const declarers = [
		...new Set(edges.map((e) => declaredBy[`${e.source}>${e.target}`]).filter(Boolean))
	].join(', ');
	const observations = edges.reduce((s, e) => s + (e.observations ?? 0), 0);
	const lastSeen = edges.map((e) => e.lastSeen ?? '').sort().pop() ?? '';
	const job = edges.find((e) => e.job)?.job ?? '';
	const size = [
		a?.rows != null ? plural(a.rows, 'row') : '',
		a?.columns != null ? plural(a.columns, 'column') : ''
	]
		.filter(Boolean)
		.join(' · ');
	const run = lastRun[node.id];
	const tags = (a?.tags ?? []).filter((t) => t !== 'pii');
	return html`<mm-details-blade closable>
		<span slot="heading" class="mm-cluster" style="--mm-gap:var(--space-2);flex-wrap:nowrap">
			<mm-provider-logo provider=${node.provider} size="20" decorative></mm-provider-logo
			>${node.name}
		</span>
		<mm-blade-section heading="At a glance">
			<mm-description-list>
				<mm-description term="Type">${a?.type ?? node.type}</mm-description>
				<mm-description term="Provider">${providerLabel(node.provider)}</mm-description>
				${a?.pii ? html`<mm-description term="Personal data"><mm-tag variant="pii"></mm-tag></mm-description>` : nothing}
				${size ? html`<mm-description term="Size">${size}</mm-description>` : nothing}
				${
					run
						? html`<mm-description term="Last run"
								>${run === 'failed' ? html`<mm-status-pill state="failed"></mm-status-pill>` : run === 'running' ? 'Running' : 'Succeeded'}</mm-description
							>`
						: nothing
				}
				${a ? html`<mm-description term="Updated">${relative(a.updated)}</mm-description>` : nothing}
			</mm-description-list>
		</mm-blade-section>
		${
			a?.description
				? html`<mm-blade-section heading="About">
						<mm-text variant="dense" block>${a.description}</mm-text>
					</mm-blade-section>`
				: nothing
		}
		<mm-blade-section heading="Who to ask">
			${
				a?.owners.length
					? html`<div class="mm-stack" style="--mm-gap:var(--space-3)">
							${a.owners.map((o) => html`<mm-owner-chip name=${owner(o).name} kind=${owner(o).kind === 'team' ? 'team' : 'person'}></mm-owner-chip>`)}
						</div>`
					: html`<mm-text variant="dense" tone="secondary" block>No owners have been added yet.</mm-text>`
			}
		</mm-blade-section>
		<mm-blade-section heading="How it connects to orders_daily">
			<mm-description-list>
				<mm-description term="Direction">${depth < 0 ? 'Upstream' : 'Downstream'}</mm-description>
				${Math.abs(depth) > 1 ? html`<mm-description term="Steps away"><mm-text numeric>${n(Math.abs(depth))}</mm-text></mm-description>` : nothing}
				${kinds.length ? html`<mm-description term="Relationship">${kinds.join(', ')}</mm-description>` : nothing}
				${recorded ? html`<mm-description term="Recorded">${recorded}</mm-description>` : nothing}
				${declarers ? html`<mm-description term="Declared by">${declarers}</mm-description>` : nothing}
				${observations ? html`<mm-description term="Observed in">${plural(observations, 'run')}</mm-description>` : nothing}
			</mm-description-list>
		</mm-blade-section>
		<mm-blade-section heading="More details" collapsible ?open=${details}>
			<mm-description-list>
				<mm-description term="MRN" mono>${node.mrn}</mm-description>
				${tags.length ? html`<mm-description term="Tags">${tags.join(', ')}</mm-description>` : nothing}
				${a?.createdBy ? html`<mm-description term="Created by">${a.createdBy}</mm-description>` : nothing}
				${job ? html`<mm-description term="Job" mono>${job}</mm-description>` : nothing}
				${lastSeen ? html`<mm-description term="Edge last seen">${dateTime(lastSeen)}</mm-description>` : nothing}
			</mm-description-list>
		</mm-blade-section>
		<div slot="footer" class="mm-cluster" style="--mm-gap:var(--space-2)">
			<mm-button variant="primary" icon="open-in-full" href="#asset/${node.mrn}"
				>Open asset</mm-button
			>
		</div>
	</mm-details-blade>`;
};

const caption = `${plural(graph.nodes.length, 'asset')} · ${plural(graph.edges.length, 'edge')} · Depth 10`;

/* ------------------------------------------------------------------ screen */

/** From the shop's orders table, through the hourly load and the dbt staging model, past orders_daily to the finance dashboard. */
export const LINEAGE_PATH: string[] = ['orders', 'orders_hourly_load', 'stg_orders', 'revenue'];

/** An edge a plugin declared: dbt-analytics wrote customers_dim → orders_daily on its last run. */
export const LINEAGE_PLUGIN_EDGE = 'customers_dim';

/** A job that reported itself through OpenLineage, with its last run and an output no plugin has catalogued. */
export const LINEAGE_OPENLINEAGE_JOB = 'enrich_orders';

/** The dashboards downstream of orders_daily, one step and two steps away. */
export const LINEAGE_DASHBOARDS: string[] = ['revenue', 'refunds_by_category'];

export interface LineageState {
	/** Id of the selected node, such as one of LINEAGE_PATH. orders_daily itself opens no blade. */
	selected?: string;
	/** Whether the blade's More details section is open. */
	details?: boolean;
}

export const lineageScreen = (s: LineageState): TemplateResult => {
	const node =
		s.selected && s.selected !== graph.currentId
			? graph.nodes.find((x) => x.id === s.selected && x.state !== 'stub')
			: undefined;
	return html`<div
		style="height:var(--screen-h, 100vh);display:flex;flex-direction:column;background:var(--mm-paper);color:var(--mm-text)"
	>
		<div style="padding:var(--space-4) var(--space-8)">
			<mm-page-header compact heading="Lineage of orders_daily" caption=${caption}>
				<mm-button slot="actions" variant="secondary" icon="arrow-back"
					>Back to orders_daily</mm-button
				>
				<mm-menu-button
					slot="actions"
					variant="ghost"
					icon="more-horiz"
					icon-only
					label="More actions for this lineage view"
				>
					<mm-menu>
						<mm-menu-item icon="link">Copy link</mm-menu-item>
					</mm-menu>
				</mm-menu-button>
			</mm-page-header>
		</div>
		<div style="display:flex;flex:1;min-height:0;min-width:0">
			${canvas({ selected: node?.id })} ${node ? nodeBlade(node, !!s.details) : nothing}
		</div>
	</div>`;
};
