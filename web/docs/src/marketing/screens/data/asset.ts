/**
 * Sample data for the Discover, Asset, Lineage and agent screens. Builds on `catalog.ts` (same
 * catalog, same "now": Tuesday 27 October 2026, 09:30 UTC) and adds what only these screens need:
 * search results, the orders_daily schema, the revenue-agent runs and the lineage graph in the
 * shape `mm-lineage-canvas` takes.
 */
import type { SchemaColumn } from '@marmotdata/elements/classes/schema-table.js';
import type {
	LineageGraph,
	LineageGraphNode,
	LineageGraphEdge
} from '@marmotdata/elements/classes/lineage-canvas.js';
import { NOW, ago, ordersLineage, type Asset } from './catalog';

/* ------------------------------------------------------------------ formatting */

const nf = new Intl.NumberFormat('en-GB');
export const n = (v: number) => nf.format(v);
/** Compact counts: 1.28M, 418K. */
export const compact = (v: number) =>
	v >= 1_000_000
		? `${(v / 1_000_000).toFixed(v >= 10_000_000 ? 1 : 2).replace(/\.?0+$/, '')}M`
		: v >= 10_000
			? `${Math.round(v / 1_000)}K`
			: nf.format(v);
/** "04:04 UTC" for today, "26 Oct" before that. */
export const when = (iso: string) => {
	const d = new Date(iso);
	const sameDay = d.toISOString().slice(0, 10) === NOW.toISOString().slice(0, 10);
	return sameDay
		? `${d.toISOString().slice(11, 16)} UTC`
		: d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', timeZone: 'UTC' });
};
/** "12 min ago", "5 h ago", "3 days ago". */
export const rel = (iso: string) => {
	const m = Math.round((NOW.getTime() - new Date(iso).getTime()) / 60_000);
	if (m < 1) return 'just now';
	if (m < 60) return `${m} min ago`;
	const h = Math.round(m / 60);
	if (h < 24) return `${h} h ago`;
	const d = Math.round(h / 24);
	return d === 1 ? '1 day ago' : `${d} days ago`;
};
/** "27 October 2026, 08:12 UTC", as the blades' More details write it. */
export const longStamp = (iso: string) => {
	const d = new Date(iso);
	return `${d.toLocaleDateString('en-GB', { day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' })}, ${d.toISOString().slice(11, 16)} UTC`;
};
export const ms = (v: number) =>
	v < 1_000
		? `${v} ms`
		: v < 60_000
			? `${(v / 1_000).toFixed(1)} s`
			: `${Math.floor(v / 60_000)} min ${Math.round((v % 60_000) / 1_000)} s`;

export const providerName: Record<string, string> = {
	postgresql: 'PostgreSQL',
	snowflake: 'Snowflake',
	dbt: 'dbt',
	kafka: 'Kafka',
	bigquery: 'BigQuery',
	looker: 'Looker',
	airflow: 'Airflow',
	agent: 'Agent',
	clickhouse: 'ClickHouse',
	spark: 'Spark'
};

/* ------------------------------------------------------------------ Discover */

/** More search results around "orders", beyond the catalog list. */
export const orderResults: Asset[] = [
	{
		mrn: 'mrn://table/postgresql/shop.public.orders',
		name: 'orders',
		type: 'Table',
		provider: 'postgresql',
		description:
			'One row per customer order placed through the web shop, written by the checkout service and loaded hourly.',
		owners: ['ae', 'jane'],
		tags: ['commerce'],
		rows: 1_284_019,
		columns: 14,
		updated: ago(78),
		views7d: 412,
		pii: true
	},
	{
		mrn: 'mrn://table/snowflake/analytics.marts.orders_daily',
		name: 'ORDERS_DAILY',
		type: 'Table',
		provider: 'snowflake',
		description:
			'Snowflake materialisation of the orders_daily dbt model. Daily order totals, deduplicated and joined to customers_dim.',
		generated: true,
		owners: ['ae', 'jane'],
		tags: ['commerce', 'finance'],
		rows: 1_284,
		columns: 9,
		updated: ago(326),
		views7d: 988,
		certified: true
	},
	{
		mrn: 'mrn://table/postgresql/shop.public.order_items',
		name: 'order_items',
		type: 'Table',
		provider: 'postgresql',
		description: 'Line items for each order: SKU, quantity and unit price in pence.',
		owners: ['ae'],
		tags: ['commerce'],
		rows: 3_918_440,
		columns: 8,
		updated: ago(78),
		views7d: 156
	},
	{
		mrn: 'mrn://table/snowflake/analytics.raw.order_events',
		name: 'ORDER_EVENTS',
		type: 'Table',
		provider: 'snowflake',
		description: 'Order events landed from shop.orders.v2 by the Flink enrichment job.',
		owners: ['dp', 'tomas'],
		tags: ['commerce', 'events'],
		rows: 48_201_377,
		columns: 12,
		updated: ago(6),
		views7d: 73
	},
	{
		mrn: 'mrn://table/postgresql/shop.public.order_status_history',
		name: 'order_status_history',
		type: 'Table',
		provider: 'postgresql',
		description: 'Every status change of an order with the actor that made it.',
		owners: ['ae'],
		tags: ['commerce'],
		rows: 5_102_874,
		columns: 6,
		updated: ago(78),
		views7d: 41
	},
	{
		mrn: 'mrn://table/postgresql/shop.public.refunds',
		name: 'refunds',
		type: 'Table',
		provider: 'postgresql',
		description: 'Refunds issued by customer support, linked to orders.',
		owners: ['ae'],
		tags: ['commerce'],
		rows: 61_802,
		columns: 11,
		updated: ago(4_500),
		views7d: 12,
		stale: true
	},
	{
		mrn: 'mrn://table/snowflake/finance.payments.order_payments',
		name: 'ORDER_PAYMENTS',
		type: 'Table',
		provider: 'snowflake',
		description: 'Payment captures matched to orders, one row per capture.',
		owners: ['fin', 'dev'],
		tags: ['finance'],
		rows: 1_301_552,
		columns: 15,
		updated: ago(65),
		views7d: 220,
		pii: true
	},
	{
		mrn: 'mrn://table/snowflake/analytics.marts.customers_dim',
		name: 'customers_dim',
		type: 'Table',
		provider: 'snowflake',
		description:
			'One row per customer with their current address, segment and lifetime value. Matched on the column last_order_id.',
		owners: ['ci', 'kenji'],
		tags: ['customer'],
		rows: 418_230,
		columns: 22,
		updated: ago(1_440),
		views7d: 640,
		certified: true,
		pii: true
	},
	{
		mrn: 'mrn://table/postgresql/shop.public.orders_archive_2023',
		name: 'orders_archive_2023',
		type: 'Table',
		provider: 'postgresql',
		description: 'Orders from 2023, moved out of shop.public.orders in January.',
		owners: ['dp'],
		tags: ['commerce', 'archive'],
		rows: 402_118,
		columns: 14,
		updated: ago(40_000),
		views7d: 3,
		stale: true
	},
	{
		mrn: 'mrn://table/snowflake/analytics.marts.refund_rates',
		name: 'REFUND_RATES',
		type: 'Table',
		provider: 'snowflake',
		description: 'Weekly refund rate by product category, built from orders_daily.',
		owners: ['fin'],
		tags: ['finance'],
		rows: 6_240,
		columns: 7,
		updated: ago(326),
		views7d: 88
	}
];

/** Popularity level from 1 to 4, from 30 days of views, queries and subscribers. */
export const popularity = (a: Asset): 1 | 2 | 3 | 4 => {
	const views = a.views7d ?? 0;
	return views >= 600 ? 4 : views >= 200 ? 3 : views >= 50 ? 2 : 1;
};

export const facetTypes = [
	{ value: 'table', label: 'Table', icon: 'table', count: 23 },
	{ value: 'view', label: 'View', icon: 'table-view', count: 6 },
	{ value: 'pipeline', label: 'Pipeline', icon: 'conversion-path', count: 3 },
	{ value: 'topic', label: 'Topic', icon: 'stream', count: 2 },
	{ value: 'dashboard', label: 'Dashboard', icon: 'dashboard', count: 2 },
	{ value: 'agent', label: 'Agent', icon: 'smart-toy', count: 1 }
];
export const facetProviders = [
	{ value: 'snowflake', label: 'Snowflake', provider: 'snowflake', count: 12 },
	{ value: 'postgresql', label: 'PostgreSQL', provider: 'postgresql', count: 11 },
	{ value: 'dbt', label: 'dbt', provider: 'dbt', count: 8 },
	{ value: 'kafka', label: 'Kafka', provider: 'kafka', count: 2 },
	{ value: 'airflow', label: 'Airflow', provider: 'airflow', count: 3 },
	{ value: 'looker', label: 'Looker', provider: 'looker', count: 2 }
];
export const facetTags = [
	{ value: 'commerce', label: 'commerce', count: 18 },
	{ value: 'finance', label: 'finance', count: 9 },
	{ value: 'pii', label: 'pii', count: 7 },
	{ value: 'events', label: 'events', count: 4 },
	{ value: 'customer', label: 'customer', count: 3 },
	{ value: 'archive', label: 'archive', count: 1 }
];
export const facetOwners = [
	{ value: 'ae', label: 'Analytics Engineering', icon: 'groups', count: 14 },
	{ value: 'dp', label: 'Data Platform', icon: 'groups', count: 5 },
	{ value: 'fin', label: 'Finance Analytics', icon: 'groups', count: 3 },
	{ value: 'ci', label: 'Customer Insights', icon: 'groups', count: 1 }
];

/* ------------------------------------------------------------------ orders_daily */

export const ordersDailySchema: SchemaColumn[] = [
	{
		name: 'order_date',
		type: 'date',
		nullable: false,
		primaryKey: true,
		description: 'Calendar day in UTC on which the orders were placed.',
		descriptionSource: 'plugin',
		stats: { distinct: 1_284, nullPercent: 0, min: '2023-04-22', max: '2026-10-26' }
	},
	{
		name: 'orders',
		type: 'integer',
		nullable: false,
		description: 'Orders placed that day, excluding cancelled and test orders.',
		descriptionSource: 'plugin',
		stats: { distinct: 1_102, nullPercent: 0, min: 212, max: 9_840 }
	},
	{
		name: 'customers',
		type: 'integer',
		nullable: false,
		description: 'Distinct customers who placed at least one order.',
		descriptionSource: 'plugin',
		stats: { distinct: 1_087, nullPercent: 0, min: 198, max: 8_911 }
	},
	{
		name: 'new_customers',
		type: 'integer',
		nullable: false,
		description: 'Customers whose first order fell on this day.',
		descriptionSource: 'user',
		glossaryTerms: ['Active customer'],
		stats: { distinct: 640, nullPercent: 0, min: 12, max: 2_204 }
	},
	{
		name: 'gross_gbp',
		type: 'numeric(14,2)',
		semantic: 'Money (GBP)',
		nullable: false,
		description: 'Order totals including VAT before refunds and discounts.',
		descriptionSource: 'user',
		glossaryTerms: ['Gross merchandise value'],
		certified: true,
		stats: { distinct: 1_284, nullPercent: 0, min: '4,118.20', max: '612,904.55' }
	},
	{
		name: 'discounts_gbp',
		type: 'numeric(14,2)',
		semantic: 'Money (GBP)',
		nullable: false,
		description: 'Promotion value applied at checkout.',
		descriptionSource: 'plugin',
		stats: { distinct: 1_270, nullPercent: 0, min: '0.00', max: '88,410.00' }
	},
	{
		name: 'refunds_gbp',
		type: 'numeric(14,2)',
		semantic: 'Money (GBP)',
		nullable: true,
		description:
			'Refunds settled against orders from this day. Null until the refund window closes after 30 days.',
		descriptionSource: 'user',
		stats: { distinct: 1_251, nullPercent: 2.3, min: '0.00', max: '41,002.90' }
	},
	{
		name: 'net_revenue_gbp',
		type: 'numeric(14,2)',
		semantic: 'Money (GBP)',
		nullable: true,
		description: 'Gross less discounts, refunds and VAT. The figure finance reports.',
		descriptionSource: 'user',
		certified: true,
		stats: { distinct: 1_252, nullPercent: 2.3, min: '3,020.11', max: '470,118.06' }
	},
	{
		name: 'avg_order_gbp',
		type: 'numeric(10,2)',
		semantic: 'Money (GBP)',
		nullable: false,
		description: 'gross_gbp divided by orders.',
		descriptionSource: 'plugin',
		stats: { distinct: 1_190, nullPercent: 0, min: '19.42', max: '71.88' }
	}
];

/* ------------------------------------------------------------------ revenue-agent */

export interface AgentRun {
	id: string;
	started: string;
	trigger: string;
	status: 'succeeded' | 'failed' | 'running';
	durationMs: number;
	toolCalls: number;
	tokensIn: number;
	tokensOut: number;
}

/** Earlier runs of revenue-agent, newest first; the run the demo plays goes on top. */
export const revenueRuns: AgentRun[] = [
	{
		id: 'arun_7d2a40',
		started: ago(26),
		trigger: 'Slack #finance-questions: Card revenue by region, last 7 days',
		status: 'succeeded',
		durationMs: 9_840,
		toolCalls: 4,
		tokensIn: 7_204,
		tokensOut: 388
	},
	{
		id: 'arun_7d2911',
		started: ago(188),
		trigger: 'Slack #finance-questions: Who are our top customers by lifetime value?',
		status: 'failed',
		durationMs: 3_410,
		toolCalls: 2,
		tokensIn: 3_812,
		tokensOut: 140
	},
	{
		id: 'arun_7d27c2',
		started: ago(1_440),
		trigger: 'Weekly digest: Refund rate by category',
		status: 'succeeded',
		durationMs: 12_120,
		toolCalls: 5,
		tokensIn: 9_930,
		tokensOut: 702
	},
	{
		id: 'arun_7d2603',
		started: ago(1_610),
		trigger: 'Slack #finance-questions: GMV month to date',
		status: 'succeeded',
		durationMs: 6_480,
		toolCalls: 3,
		tokensIn: 5_118,
		tokensOut: 296
	},
	{
		id: 'arun_7d24b8',
		started: ago(2_950),
		trigger: 'Slack #finance-questions: Average order value in September',
		status: 'succeeded',
		durationMs: 7_015,
		toolCalls: 3,
		tokensIn: 5_407,
		tokensOut: 318
	},
	{
		id: 'arun_7d2390',
		started: ago(4_320),
		trigger: 'Weekly digest: Revenue against forecast',
		status: 'succeeded',
		durationMs: 11_870,
		toolCalls: 6,
		tokensIn: 9_218,
		tokensOut: 655
	},
	{
		id: 'arun_7d2201',
		started: ago(5_800),
		trigger: 'Slack #finance-questions: Discounts given last month',
		status: 'succeeded',
		durationMs: 5_980,
		toolCalls: 3,
		tokensIn: 4_906,
		tokensOut: 274
	}
];

/* ------------------------------------------------------------------ lineage */

const toNode = (x: (typeof ordersLineage.nodes)[number]): LineageGraphNode => ({
	id: x.id,
	mrn: x.mrn,
	name: x.name,
	type: x.type.toLowerCase(),
	provider: x.provider,
	...(x.state ? { state: x.state as 'stub' | 'failed' } : {})
});

/** orders_daily lineage, with three more agents that looked it up so the agent cluster has members. */
export const ordersGraph: LineageGraph = {
	currentId: 'orders_daily',
	nodes: [
		{
			id: 'shop_db',
			mrn: 'mrn://database/postgresql/shop',
			name: 'shop',
			type: 'database',
			provider: 'postgresql'
		},
		...ordersLineage.nodes
			.map(toNode)
			.map((x) =>
				x.id === 'orders'
					? { ...x, hiddenUpstream: 2 }
					: x.id === 'revenue'
						? { ...x, hiddenDownstream: 3 }
						: x
			),
		{
			id: 'revenue-agent',
			mrn: 'mrn://agent/mcp/revenue-agent',
			name: 'revenue-agent',
			type: 'agent',
			provider: 'agent'
		},
		{
			id: 'catalog-steward',
			mrn: 'mrn://agent/mcp/catalog-steward',
			name: 'catalog-steward',
			type: 'agent',
			provider: 'agent'
		},
		{
			id: 'cli-agent-jane',
			mrn: 'mrn://agent/mcp/cli-agent-jane',
			name: 'cli-agent (Jane Doe)',
			type: 'agent',
			provider: 'agent'
		}
	],
	edges: [
		{ source: 'shop_db', target: 'orders', kind: 'CONTAINS' },
		...ordersLineage.edges.map((e) => ({
			source: e.source,
			target: e.target,
			kind: e.kind as LineageGraphEdge['kind'],
			origin: e.origin as LineageGraphEdge['origin'],
			...(e.observations ? { observations: e.observations, lastSeen: ago(20) } : {}),
			...(e.target === 'orders_daily' && e.source === 'stg_orders'
				? { job: 'dbt run orders_daily' }
				: {})
		})),
		{
			source: 'orders_daily',
			target: 'revenue-agent',
			kind: 'AGENT_LOOKUP',
			origin: 'observed',
			observations: 88,
			lastSeen: ago(26)
		},
		{
			source: 'orders_daily',
			target: 'catalog-steward',
			kind: 'AGENT_LOOKUP',
			origin: 'observed',
			observations: 12,
			lastSeen: ago(400)
		},
		{
			source: 'orders_daily',
			target: 'cli-agent-jane',
			kind: 'AGENT_LOOKUP',
			origin: 'observed',
			observations: 5,
			lastSeen: ago(58)
		}
	]
};
