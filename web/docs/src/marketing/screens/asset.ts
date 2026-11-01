/**
 * The asset page for orders_daily: Documentation and Schema tabs. Tabs are links so each view
 * deep-links; the blade folds to a rail. Links are `#…` so they never leave the landing page.
 */
import { html, nothing, type TemplateResult } from 'lit';
import type { SchemaColumn } from '@marmotdata/elements/classes/schema-table.js';
import { shell } from './shell';
import { ordersDailySchema } from './data/asset';

/* ------------------------------------------------------------------ shared parts */

export type Tab = { id: string; label: string; icon: string; count?: number };
// The app's order for a table: Query appears only for assets that carry one, such as a dbt model, and Contents only for drive providers.
const tableTabs: Tab[] = [
	{ id: 'documentation', label: 'Documentation', icon: 'description' },
	{ id: 'metadata', label: 'Metadata', icon: 'data-object' },
	{ id: 'schema', label: 'Schema', icon: 'view-column', count: 9 },
	{ id: 'lineage', label: 'Lineage', icon: 'account-tree', count: 13 },
	{ id: 'run-history', label: 'Run history', icon: 'history', count: 30 },
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

/** Subscribe is a quiet icon button; once subscribed it becomes a menu of what to be notified about, as in the app. */
export const subscribe = (on = false) =>
	on
		? html`<mm-menu-button
				slot="actions"
				variant="ghost"
				icon="notifications-active"
				icon-only
				label="Manage subscription"
			>
				<mm-menu>
					<mm-menu-group label="Notify me about">
						<mm-menu-item type="checkbox" checked>Asset Changes</mm-menu-item>
						<mm-menu-item type="checkbox" checked>Schema Changes</mm-menu-item>
						<mm-menu-item type="checkbox" checked>Upstream Schema</mm-menu-item>
						<mm-menu-item type="checkbox">Downstream Schema</mm-menu-item>
						<mm-menu-item type="checkbox">Lineage Changes</mm-menu-item>
						<mm-menu-item type="checkbox" checked>Asset Deletions</mm-menu-item>
					</mm-menu-group>
					<mm-menu-separator></mm-menu-separator>
					<mm-menu-item icon="notifications-off">Unsubscribe</mm-menu-item>
				</mm-menu>
			</mm-menu-button>`
		: html`<mm-button
				slot="actions"
				variant="ghost"
				icon="notifications"
				label="Subscribe to notifications"
				pressed="false"
			></mm-button>`;

/** A link as a small button with a Material Symbol, the link's own icon or `link` when it has none. */
const linkButton = (label: string, href: string, symbol = 'link') =>
	html`<mm-button slot="links" size="sm" href=${href} external
		><mm-icon slot="icon" name=${symbol} aria-hidden="true"></mm-icon>${label}</mm-button
	>`;

export const assetPage = (header: TemplateResult, strip: TemplateResult, body: TemplateResult) =>
	html`<div class="mm-stack" style="--mm-gap:var(--space-5)">
		${header}
		<div class="mm-stack" style="--mm-gap:var(--space-6)">${strip}${body}</div>
	</div>`;

/** The blade's nearest lineage. A stub, an asset seen in lineage but not catalogued yet, has no page to link to. */
export const lineageList = (
	label: string,
	items: { name: string; provider: string; what: string; stub?: boolean }[]
) =>
	html`<mm-list dense aria-label=${label}>
		${items.map((i) => html`<mm-list-item href=${i.stub ? nothing : `#asset/${i.name}`} secondary=${i.what}><mm-provider-logo slot="start" provider=${i.provider} size="20" decorative></mm-provider-logo>${i.name}</mm-list-item>`)}
	</mm-list>`;

/** The app's crumbs: Discover, the asset type from the URL with its first letter capitalised, then the asset. */
export const crumbsFor = (name: string, type = 'Table') => [
	{ label: 'Discover', href: '#discover' },
	{ label: type, href: `#discover?types=${type.toLowerCase()}` },
	{ label: name }
];

/* ------------------------------------------------------------------ orders_daily */

// The dbt plugin catalogs the table it builds under the warehouse's provider, so orders_daily is a Snowflake table and its dbt model is a separate asset.
const ORDERS_DAILY_MRN = 'mrn://table/snowflake/analytics.marts.orders_daily';

// The links point at the shop's own dbt docs, Snowflake and wiki; here they are `#` links so a click inside the screen never leaves the landing page.
const ordersHeader = () =>
	html`<mm-page-header
		heading="orders_daily"
		provider="snowflake"
		caption="1,284 rows · Updated 5 hours ago"
		mrn=${ORDERS_DAILY_MRN}
		description="Finance's source of truth for daily revenue. Report net_revenue_gbp; gross_gbp is for demand trends only."
	>
		<mm-tag slot="tags" href="#discover?tags=commerce" removable>commerce</mm-tag>
		<mm-tag slot="tags" href="#discover?tags=finance" removable>finance</mm-tag>
		<mm-button slot="tags" variant="ghost" size="sm" icon="add">Add tag</mm-button>
		<mm-owner-chip
			slot="owners"
			name="Analytics Engineering"
			kind="team"
			href="#settings/teams/analytics-engineering"
			removable
		></mm-owner-chip>
		<mm-owner-chip
			slot="owners"
			name="Jane Doe"
			kind="person"
			email="jane.doe@shopwell.io"
			removable
		></mm-owner-chip>
		<mm-button slot="owners" variant="ghost" size="sm" icon="person-add">Add owner</mm-button>
		${linkButton('dbt docs', '#link/dbt-docs')}
		${linkButton('Snowflake', '#link/snowflake')}
		${linkButton('Revenue reporting guide', '#link/revenue-reporting', 'menu-book')}
		<mm-button slot="links" variant="ghost" size="sm" icon="add">Add link</mm-button>
		${subscribe(true)} ${moreMenu('orders_daily')}
		<mm-button slot="actions" variant="primary" icon="terminal">Query table</mm-button>
	</mm-page-header>`;

/** Owners, tags and freshness are in the page header, so the blade does not repeat them, and the notes stay in the header too. */
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
				<mm-description term="Last run">Succeeded · 5 hours ago</mm-description>
				<mm-description term="Last synced">5 hours ago</mm-description>
			</mm-description-list>
		</mm-blade-section>
		<mm-blade-section heading="About">
			<div class="mm-stack" style="--mm-gap:var(--space-3)">
				<div class="mm-stack" style="--mm-gap:var(--space-1);align-items:flex-start">
					<mm-tag variant="plugin"></mm-tag>
					<mm-text variant="dense" block
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
				pressed="false"
			></mm-button>
			<div class="mm-stack" style="--mm-gap:var(--space-3)">
				<div class="mm-cluster" style="--mm-gap:var(--space-2)">
					<mm-tag href="#glossary/gmv" removable>Gross merchandise value</mm-tag
					><mm-tag href="#glossary/active-customer" removable>Active customer</mm-tag>
				</div>
			</div>
		</mm-blade-section>
		<mm-blade-section heading="Where it flows" flush>
			<mm-link slot="actions" href="#asset/orders_daily?tab=lineage">Open lineage</mm-link>
			${lineageList('Nearest lineage', [
				{ name: 'stg_orders', provider: 'dbt', what: 'Upstream · View' },
				{ name: 'customers_dim', provider: 'snowflake', what: 'Upstream · Table' },
				{ name: 'Revenue overview', provider: 'looker', what: 'Downstream · Dashboard' },
				{ name: 'refund_rates', provider: 'dbt', what: 'Downstream · Table, last run failed' },
				{
					name: 'orders_enriched',
					provider: 'spark',
					what: 'Downstream · not catalogued yet',
					stub: true
				}
			])}
		</mm-blade-section>
		<mm-blade-section heading="More details" collapsible>
			<mm-description-list>
				<mm-description term="Created by">dbt-analytics</mm-description>
				<mm-description term="Created">22 Apr 2023, 06:10</mm-description>
				<mm-description term="Updated">27 Oct 2026, 04:04</mm-description>
				<mm-description term="Parent" mono>mrn://schema/snowflake/analytics.marts</mm-description>
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
	html`<div
		style="display:grid;grid-template-columns:13rem minmax(0,1fr);gap:var(--space-6);align-items:start"
	>
		<div class="mm-stack" style="--mm-gap:var(--space-2);align-items:flex-start">
			<mm-tree
				label="Documentation pages"
				selection="single"
				style="--mm-tree-height:60vh;inline-size:100%"
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
				.actions=${[
					{ id: 'add', label: 'Add sub-page', icon: 'note-add' },
					{ id: 'delete', label: 'Delete page', icon: 'delete' }
				]}
			></mm-tree>
			<mm-button variant="ghost" size="sm" icon="note-add">Add page</mm-button>
		</div>
		<article class="mm-stack" style="--mm-gap:var(--space-4);min-width:0">
			<div class="mm-cluster mm-cluster--between">
				<mm-text variant="caption" tone="secondary">Updated 21 Oct 2026</mm-text>
				<mm-button variant="secondary" size="sm" icon="edit">Edit page</mm-button>
			</div>
			<div class="mm-prose">
				<h1>Overview</h1>
				<p>
					orders_daily has one row per calendar day in UTC since the shop moved to the current
					checkout on 22 April 2023. It is built by the <code>orders_daily</code> dbt model from
					<a href="#asset/stg_orders">stg_orders</a> and joined to
					<a href="#asset/customers_dim">customers_dim</a> to count new customers.
				</p>
				<h2>Which figure to use</h2>
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
				<h2>Restatements</h2>
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

// The app builds the outline from the schema document and the markers people set, so it has no profile statistics, semantic types or key markers; they are dropped here, and the Certified marker on net_revenue_gbp follows the screen's state.
const outline = (certified: boolean): SchemaColumn[] =>
	ordersDailySchema.map(({ stats: _s, semantic: _t, primaryKey: _k, ...c }) =>
		c.name === 'net_revenue_gbp' ? { ...c, certified } : c
	);

const termHref = (term: string) => `#glossary/${term.toLowerCase().replace(/\s+/g, '-')}`;

// One schema, so there is no schema picker: the row keeps its left slot empty and puts the schema actions on the right, as the app does.
const schema = (field: string, certified: boolean) => {
	const columns = outline(certified);
	const open = columns.find((c) => c.name === field);
	return html`<div class="mm-stack" style="--mm-gap:var(--space-4)">
		<div class="mm-cluster mm-cluster--between">
			<span></span>
			<div class="mm-cluster" style="--mm-gap:var(--space-2)">
				<mm-button variant="ghost" size="sm" icon="add">Add schema</mm-button>
				<mm-menu-button
					variant="ghost"
					size="sm"
					icon="more-horiz"
					icon-only
					label="More actions for the dbt schema"
				>
					<mm-menu>
						<mm-menu-item icon="edit">Edit schema</mm-menu-item>
						<mm-menu-separator></mm-menu-separator>
						<mm-menu-item icon="delete" danger>Delete schema</mm-menu-item>
					</mm-menu>
				</mm-menu-button>
			</div>
		</div>
		<div class="mm-cluster" style="--mm-gap:var(--space-2)" data-demo="field-actions">
			${open
				? html`<mm-text variant="caption" tone="secondary">Field ${open.name}</mm-text>
						<mm-button variant="ghost" size="sm" icon="shield" pressed=${open.pii ? 'true' : 'false'}
							>Personal data</mm-button
						>
						<mm-button
							variant="ghost"
							size="sm"
							icon="verified"
							pressed=${open.certified ? 'true' : 'false'}
							>Certified</mm-button
						>
						<mm-button variant="ghost" size="sm" icon="edit">Edit description</mm-button>
						<mm-button variant="ghost" size="sm" icon="menu-book">Glossary terms</mm-button>`
				: html`<mm-text variant="caption" tone="secondary"
						>Select a field to mark it as personal data, certify it, edit its description or link
						glossary terms.</mm-text
					>`}
		</div>
		${open?.glossaryTerms?.length
			? html`<div class="mm-cluster" style="--mm-gap:var(--space-2)" data-demo="field-terms">
					<mm-text variant="caption" tone="secondary">Glossary terms for ${open.name}</mm-text>
					${open.glossaryTerms.map((t) => html`<mm-tag href=${termHref(t)} removable>${t}</mm-tag>`)}
				</div>`
			: nothing}
		<mm-schema-table
			label="Schema of orders_daily"
			editable
			.columns=${columns}
			.selected=${field}
		></mm-schema-table>
	</div>`;
};

/* ------------------------------------------------------------------ screen */

export interface AssetState {
	tab: 'documentation' | 'schema';
	/** Path of the field open in the schema's field panel, such as `net_revenue_gbp`. */
	field?: string;
	/** Whether net_revenue_gbp carries the Certified marker. It does unless a demo is about to certify it. */
	certified?: boolean;
}

/* The Schema tab needs the width, so the blade starts folded there, as in the app. */
export const assetScreen = (s: AssetState): TemplateResult =>
	s.tab === 'schema'
		? ordersShell('schema', schema(s.field ?? '', s.certified ?? true), { collapsed: true })
		: ordersShell('documentation', documentation());
