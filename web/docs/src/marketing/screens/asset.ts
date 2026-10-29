/**
 * The asset page for orders_daily: Documentation and Schema tabs. Tabs are links so each view
 * deep-links; the blade folds to a rail. Links are `#…` so they never leave the landing page.
 */
import { html, nothing, type TemplateResult } from 'lit';
import { shell } from './shell';
import { asset } from './data/catalog';
import { ordersDailySchema } from './data/asset';

/* ------------------------------------------------------------------ shared parts */

export type Tab = { id: string; label: string; icon: string; count?: number };
const tableTabs: Tab[] = [
	{ id: 'documentation', label: 'Documentation', icon: 'description' },
	{ id: 'metadata', label: 'Metadata', icon: 'data-object' },
	{ id: 'schema', label: 'Schema', icon: 'view-column', count: 9 },
	{ id: 'lineage', label: 'Lineage', icon: 'account-tree', count: 13 },
	{ id: 'runs', label: 'Run history', icon: 'history', count: 30 },
	{ id: 'queries', label: 'Queries', icon: 'query-stats', count: 1_204 },
	{ id: 'preview', label: 'Preview', icon: 'table-view' },
	{ id: 'environments', label: 'Environments', icon: 'layers', count: 2 }
];

export const tabs = (current: string, set: Tab[] = tableTabs, base = 'orders_daily') =>
	html`<mm-tabs label="Views of ${base}" value=${current}>
		${set.map((t) => html`<mm-tab value=${t.id} href="#asset/${base}?tab=${t.id}" icon=${t.icon} .count=${t.count}>${t.label}</mm-tab>`)}
	</mm-tabs>`;

/** More holds only occasional and destructive actions: places to go are in the Links row, and the MRN has its own copy button. */
export const moreMenu = (name: string) =>
	html`<mm-menu-button
		slot="actions"
		variant="secondary"
		icon="more-horiz"
		icon-only
		label="More actions for ${name}"
	>
		<mm-menu>
			<mm-menu-item icon="edit-note">Edit notes</mm-menu-item>
			<mm-menu-separator></mm-menu-separator>
			<mm-menu-item icon="delete" danger>Delete asset</mm-menu-item>
		</mm-menu>
	</mm-menu-button>`;

/** Subscribe is a personal toggle, so it is a quiet icon button that shows its state, not a labelled secondary. */
export const subscribe = (on = false) =>
	html`<mm-button
		slot="actions"
		variant="ghost"
		icon=${on ? 'notifications-active' : 'notifications'}
		label="Subscribe to changes"
		pressed=${on ? 'true' : 'false'}
	></mm-button>`;

const addLink = html`<mm-button slot="links" variant="ghost" size="sm" icon="add"
	>Add link</mm-button
>`;

/** A link as a small button with its own icon: a provider logo, or a Material Symbol for anything else. */
const linkButton = (label: string, href: string, icon: { provider?: string; symbol?: string }) =>
	html`<mm-button slot="links" size="sm" href=${href} external
		>${
			icon.provider
				? html`<mm-provider-logo
						slot="icon"
						provider=${icon.provider}
						size="16"
						decorative
					></mm-provider-logo>`
				: html`<mm-icon slot="icon" name=${icon.symbol ?? 'link'} aria-hidden="true"></mm-icon>`
		}${label}</mm-button
	>`;

export const assetPage = (header: TemplateResult, strip: TemplateResult, body: TemplateResult) =>
	html`<div class="mm-stack" style="--mm-gap:var(--space-5)">
		${header}
		<div class="mm-stack" style="--mm-gap:var(--space-6)">${strip}${body}</div>
	</div>`;

export const lineageList = (
	label: string,
	items: { name: string; provider: string; what: string }[]
) =>
	html`<mm-list dense aria-label=${label}>
		${items.map((i) => html`<mm-list-item href="#asset/${i.name}" secondary=${i.what}><mm-provider-logo slot="start" provider=${i.provider} decorative></mm-provider-logo>${i.name}</mm-list-item>`)}
	</mm-list>`;

export const crumbsFor = (name: string, type = 'Tables') => [
	{ label: 'Discover', href: '#discover' },
	{ label: type, href: '#discover' },
	{ label: name }
];

/* ------------------------------------------------------------------ orders_daily */

const ordersDaily = asset('orders_daily');

// The mockup's links point at the shop's own dbt docs, Snowflake and wiki; here they are `#` links
// so a click inside the screen never leaves the landing page.
const ordersHeader = () =>
	html`<mm-page-header
		heading="orders_daily"
		provider="dbt"
		caption="1,284 rows · Updated 5 hours ago"
		mrn=${ordersDaily.mrn}
		description="Finance's source of truth for daily revenue. Report net_revenue_gbp; gross_gbp is for demand trends only."
	>
		<mm-tag slot="tags" variant="certified"></mm-tag>
		<mm-tag slot="tags" href="#discover?tags=commerce">commerce</mm-tag>
		<mm-tag slot="tags" href="#discover?tags=finance">finance</mm-tag>
		<mm-button slot="tags" variant="ghost" size="sm" icon="add">Add tag</mm-button>
		<mm-owner-chip slot="owners" name="Analytics Engineering" kind="team"></mm-owner-chip>
		<mm-owner-chip slot="owners" name="Jane Doe"></mm-owner-chip>
		<mm-button
			slot="owners"
			variant="ghost"
			size="sm"
			icon="person-add"
			label="Add owner"
		></mm-button>
		${linkButton('dbt docs', '#link/dbt-docs', { provider: 'dbt' })}
		${linkButton('Snowflake', '#link/snowflake', { provider: 'snowflake' })}
		${linkButton('Revenue reporting guide', '#link/revenue-reporting', { symbol: 'menu-book' })}
		${addLink} ${subscribe(true)} ${moreMenu('orders_daily')}
		<mm-button slot="actions" variant="primary" icon="terminal">Query table</mm-button>
	</mm-page-header>`;

/** Type, size, freshness, owners and tags are in the page header, so the blade does not repeat them; run history is its own tab. */
const ordersBlade = (o: { collapsed?: boolean } = {}) =>
	html`<mm-details-blade
		heading="About this asset"
		collapsible
		?collapsed=${o.collapsed}
		resizable
		storage-key=""
	>
		<mm-blade-section heading="At a glance">
			<mm-description-list>
				<mm-description term="Personal data">None</mm-description>
				<mm-description term="Popularity"><mm-popularity level="4"></mm-popularity></mm-description>
				<mm-description term="Last run">Succeeded 5 h ago</mm-description>
			</mm-description-list>
		</mm-blade-section>
		<mm-blade-section heading="About">
			<mm-button
				slot="actions"
				variant="ghost"
				size="sm"
				icon="edit"
				label="Edit description"
			></mm-button>
			<div class="mm-stack" style="--mm-gap:var(--space-3)">
				<mm-text variant="dense" block
					>Daily order totals, deduplicated and joined to customers_dim. Refunds settle 30 days
					after the order date, so the last month restates nightly.</mm-text
				>
				<div class="mm-stack" style="--mm-gap:var(--space-1);align-items:flex-start">
					<mm-tag variant="plugin"></mm-tag>
					<mm-text variant="dense" tone="secondary" block
						>Daily order totals, deduplicated and joined to customers_dim.</mm-text
					>
				</div>
			</div>
		</mm-blade-section>
		<mm-blade-section heading="What it means">
			<mm-button
				slot="actions"
				variant="ghost"
				size="sm"
				icon="add"
				label="Link a glossary term"
			></mm-button>
			<div class="mm-cluster">
				<mm-tag href="#glossary/gmv">Gross merchandise value</mm-tag
				><mm-tag href="#glossary/active-customer">Active customer</mm-tag>
			</div>
		</mm-blade-section>
		<mm-blade-section heading="Where it flows" flush>
			<mm-link slot="actions" href="#lineage/orders_daily">Open lineage</mm-link>
			${lineageList('Nearest lineage', [
				{ name: 'stg_orders', provider: 'dbt', what: 'Upstream · dbt view' },
				{ name: 'customers_dim', provider: 'snowflake', what: 'Upstream · Snowflake table' },
				{ name: 'Revenue overview', provider: 'looker', what: 'Downstream · Looker dashboard' },
				{ name: 'refund_rates', provider: 'dbt', what: 'Downstream · dbt table, last run failed' },
				{ name: 'orders_enriched', provider: 'spark', what: 'Downstream · not catalogued yet' }
			])}
		</mm-blade-section>
		<mm-blade-section heading="More details" collapsible>
			<mm-description-list>
				<mm-description term="Created by">dbt-analytics</mm-description>
				<mm-description term="Created">22 April 2023, 06:10 UTC</mm-description>
				<mm-description term="Updated">27 October 2026, 04:04 UTC</mm-description>
				<mm-description term="Parent"
					><a class="mm-link mm-link--quiet" href="#asset/mrn://schema/snowflake/analytics.marts"
						>analytics.marts</a
					>
					schema</mm-description
				>
			</mm-description-list>
		</mm-blade-section>
	</mm-details-blade>`;

const ordersShell = (
	tab: string,
	body: TemplateResult,
	o: { collapsed?: boolean; extra?: TemplateResult } = {}
) =>
	shell({
		current: 'discover',
		crumbs: crumbsFor('orders_daily'),
		content: html`${assetPage(ordersHeader(), tabs(tab), body)}${o.extra ?? nothing}`,
		blade: ordersBlade({ collapsed: o.collapsed })
	});

/* ------------------------------------------------------------------ tab bodies */

const documentation = () =>
	html`<div class="mm-split mm-split--sidebar" style="grid-template-columns:13rem minmax(0,1fr)">
		<div class="mm-stack" style="--mm-gap:var(--space-2)">
			<mm-tree
				label="Documentation pages"
				selection="single"
				style="--mm-tree-height:12rem"
				.nodes=${[
					{
						id: 'overview',
						label: 'Overview',
						icon: 'article',
						children: [
							{ id: 'definitions', label: 'Definitions', icon: 'article' },
							{ id: 'restatements', label: 'Restatements', icon: 'article' }
						]
					},
					{ id: 'known-issues', label: 'Known issues', icon: 'article' },
					{ id: 'changelog', label: 'Changelog', icon: 'article' }
				]}
				.expandedIds=${['overview']}
				.selectedIds=${['overview']}
			></mm-tree>
			<mm-button variant="ghost" size="sm" icon="note-add">Add page</mm-button>
		</div>
		<article class="mm-stack" style="--mm-gap:var(--space-4)">
			<div class="mm-cluster mm-cluster--between">
				<mm-text variant="caption" tone="secondary">Edited by Jane Doe on 21 Oct</mm-text>
				<mm-button variant="secondary" size="sm" icon="edit">Edit page</mm-button>
			</div>
			<div class="mm-prose">
				<h2 style="margin-block-start:0">Overview</h2>
				<p>
					orders_daily has one row per calendar day in UTC since the shop moved to the current
					checkout on 22 April 2023. It is built by the <code>orders_daily</code> dbt model from
					<a href="#asset/stg_orders">stg_orders</a> and joined to
					<a href="#asset/customers_dim">customers_dim</a> to count new customers.
				</p>
				<h3>Which figure to use</h3>
				<ul>
					<li>
						<strong>net_revenue_gbp</strong> for anything reported to finance or the board. It
						removes discounts, refunds and VAT.
					</li>
					<li>
						<strong>gross_gbp</strong> for demand trends. It matches the glossary term Gross
						merchandise value.
					</li>
					<li>
						<strong>orders</strong> excludes cancelled and test orders, so it will not match a raw
						count of shop.public.orders.
					</li>
				</ul>
				<h3>Restatements</h3>
				<p>
					Refunds settle up to 30 days after an order. Until then <code>refunds_gbp</code> and
					<code>net_revenue_gbp</code> are null for that day, and the nightly run restates the last
					30 days. Snapshot the table if you need a figure that never changes.
				</p>
				<blockquote>
					Questions go to #analytics-engineering. For access to the PII join, ask Governance.
				</blockquote>
			</div>
		</article>
	</div>`;

const schema = (field: string) =>
	html`<mm-schema-table
		label="Schema of orders_daily"
		editable
		.columns=${ordersDailySchema}
		.selected=${field}
	></mm-schema-table>`;

/* ------------------------------------------------------------------ screen */

export interface AssetState {
	tab: 'documentation' | 'schema';
	/** Path of the field open in the schema's field panel, such as `net_revenue_gbp`. */
	field?: string;
}

/* Data tabs need the width, so the blade starts folded, as in the mockup's Schema story. */
export const assetScreen = (s: AssetState): TemplateResult =>
	s.tab === 'schema'
		? ordersShell('schema', schema(s.field ?? ''), { collapsed: true })
		: ordersShell('documentation', documentation());
