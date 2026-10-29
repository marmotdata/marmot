/**
 * Full-screen lineage of orders_daily, with the selected asset previewed in the blade. Observed
 * edges are off, as in the product, so chatty agents do not crowd pipeline lineage.
 */
import { html, nothing, type TemplateResult } from 'lit';
import type {
	LineageGraph,
	LineageGraphEdge,
	LineageGraphNode
} from '@marmotdata/elements/classes/lineage-canvas.js';
import { shell } from './shell';
import { assets, owner } from './data/catalog';
import { ordersGraph, n, providerName } from './data/asset';

/* ------------------------------------------------------------------ parts */

const crumbs = (name = 'orders_daily') => [
	{ label: 'Discover', href: '#discover' },
	{ label: 'Tables', href: '#discover' },
	{ label: name, href: `#asset/${name}` },
	{ label: 'Lineage' }
];

const canvas = (o: { graph: LineageGraph; selected?: string }) =>
	html`<mm-lineage-canvas
		.graph=${o.graph}
		label="Lineage of orders_daily"
		depth="10"
		max-depth="20"
		selected=${o.selected ?? ''}
		editable
		style="--mm-lineage-height:100%;flex:1;min-height:0;border-inline:0;border-block-end:0;border-radius:0"
	></mm-lineage-canvas>`;

const page = (title: string, caption: string, body: TemplateResult, back = 'orders_daily') =>
	html`<div style="display:flex;flex-direction:column;height:100%">
		<div style="padding:var(--space-4) var(--space-8)">
			<mm-page-header compact heading=${title} caption=${caption}>
				<mm-button slot="actions" variant="secondary" icon="arrow-back" href="#asset/${back}"
					>Back to ${back}</mm-button
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
		${body}
	</div>`;

const relationship: Record<string, string> = {
	FEEDS: 'Feeds',
	PRODUCES: 'Produces',
	AGENT_LOOKUP: 'Looked up by an agent',
	CONTAINS: 'Contains'
};

/** How a node connects to orders_daily: the edge between them, or the first edge on its side. */
const connection = (id: string): LineageGraphEdge | undefined =>
	ordersGraph.edges.find(
		(e) =>
			(e.source === id && e.target === 'orders_daily') ||
			(e.source === 'orders_daily' && e.target === id)
	) ?? ordersGraph.edges.find((e) => e.source === id || e.target === id);

/** The pipeline that declares each provider's lineage. */
const declaredBy: Record<string, string> = {
	postgresql: 'postgres-shop',
	snowflake: 'snowflake-finance',
	dbt: 'dbt-analytics',
	kafka: 'kafka-orders',
	looker: 'looker-prod',
	airflow: 'OpenLineage',
	spark: 'OpenLineage'
};

/**
 * Preview pattern for a node: at a glance, about, who to ask, how it connects, more details, next
 * steps. The mockup previews customers_dim only; nodes the catalog list does not hold (stg_orders,
 * refund_rates, the agents) fall back to what the graph knows.
 */
const nodeBlade = (node: LineageGraphNode) => {
	const a = assets.find((x) => x.mrn === node.mrn || x.name === node.name);
	const edge = connection(node.id);
	const type = a?.type ?? node.type.replace(/^\w/, (c) => c.toUpperCase());
	return html`<mm-details-blade closable>
		<span slot="heading" class="mm-cluster" style="--mm-gap:var(--space-2);flex-wrap:nowrap">
			<mm-provider-logo provider=${node.provider} size="20" decorative></mm-provider-logo
			>${node.name}
		</span>
		<mm-blade-section heading="At a glance">
			<mm-description-list>
				<mm-description term="Type"
					>${type} · ${providerName[node.provider] ?? node.provider}</mm-description
				>
				<mm-description term="Personal data"
					>${a?.pii ? html`<mm-tag variant="pii"></mm-tag>` : 'None'}</mm-description
				>
				${a?.rows || a?.columns ? html`<mm-description term="Size">${a.rows ? `${n(a.rows)} rows` : ''}${a.columns ? ` · ${a.columns} columns` : ''}</mm-description>` : nothing}
				${node.state === 'failed' ? html`<mm-description term="Last run"><mm-status-pill state="failed"></mm-status-pill></mm-description>` : nothing}
				${node.state === 'stub' ? html`<mm-description term="In the catalog">Not catalogued yet</mm-description>` : nothing}
			</mm-description-list>
		</mm-blade-section>
		${
			a
				? html`<mm-blade-section heading="About">
							<mm-text variant="dense" block>${a.description}</mm-text>
						</mm-blade-section>
						<mm-blade-section heading="Who to ask">
							<div class="mm-stack" style="--mm-gap:var(--space-3)">
								${a.owners.map((o) => html`<mm-owner-chip name=${owner(o).name} kind=${owner(o).kind}></mm-owner-chip>`)}
							</div>
						</mm-blade-section>`
				: nothing
		}
		${
			edge && node.id !== 'orders_daily'
				? html`<mm-blade-section heading="How it connects to orders_daily">
						<mm-description-list>
							<mm-description term="Relationship"
								>${relationship[edge.kind ?? 'FEEDS'] ?? 'Feeds'}</mm-description
							>
							${edge.origin === 'observed' ? nothing : html`<mm-description term="Declared by">${declaredBy[node.provider] ?? 'dbt-analytics'}</mm-description>`}
							${edge.observations ? html`<mm-description term="Observed in">${n(edge.observations)} OpenLineage runs</mm-description>` : nothing}
						</mm-description-list>
					</mm-blade-section>`
				: nothing
		}
		<mm-blade-section heading="More details" collapsible>
			<mm-description-list>
				<mm-description term="MRN" mono>${node.mrn}</mm-description>
				${a ? html`<mm-description term="Tags">${a.tags.filter((t) => t !== 'pii').join(', ')}</mm-description>` : nothing}
				<mm-description term="Edge last seen">27 October 2026, 09:10 UTC</mm-description>
			</mm-description-list>
		</mm-blade-section>
		<div slot="footer" class="mm-cluster" style="--mm-gap:var(--space-2)">
			<mm-button variant="primary" icon="open-in-full" href="#asset/${node.mrn}"
				>Open asset</mm-button
			>
		</div>
	</mm-details-blade>`;
};

const graphSize = (g: LineageGraph) =>
	`${n(g.nodes.length)} assets · ${n(g.edges.length)} edges · Depth 10`;

/* ------------------------------------------------------------------ screen */

/** From the shop's orders table, through the load and the dbt models, to the finance dashboard. */
export const LINEAGE_PATH: string[] = [
	'orders',
	'orders_hourly_load',
	'stg_orders',
	'orders_daily',
	'revenue'
];

export interface LineageState {
	/** Id of the selected node, such as one of LINEAGE_PATH. */
	selected?: string;
}

export const lineageScreen = (s: LineageState): TemplateResult => {
	const node = s.selected ? ordersGraph.nodes.find((x) => x.id === s.selected) : undefined;
	return shell({
		current: 'discover',
		crumbs: crumbs(),
		bleed: true,
		content: page(
			'Lineage of orders_daily',
			graphSize(ordersGraph),
			canvas({ graph: ordersGraph, selected: node?.id })
		),
		...(node ? { blade: nodeBlade(node) } : {})
	});
};
