/**
 * Shared sample data for every screen, so Discover, Asset, Lineage, Query and Settings all describe
 * the same catalog. Realistic and specific: never lorem ipsum or Acme. Copied from the product
 * mockups and trimmed to what the landing page shows.
 * "Now" is fixed so the screens never drift: Tuesday 27 October 2026, 09:30 UTC.
 */
export const NOW = new Date('2026-10-27T09:30:00Z');
export const ago = (minutes: number) => new Date(NOW.getTime() - minutes * 60_000).toISOString();

export type Kind = 'person' | 'agent' | 'team';
export interface Owner {
	id: string;
	name: string;
	kind: Kind;
	email?: string;
	team?: string;
	runsAs?: string;
	title?: string;
}

export const people: Owner[] = [
	{
		id: 'jane',
		name: 'Jane Doe',
		kind: 'person',
		email: 'jane.doe@shopwell.io',
		team: 'Analytics Engineering',
		title: 'Staff analytics engineer'
	},
	{
		id: 'tomas',
		name: 'John Smith',
		kind: 'person',
		email: 'john.smith@shopwell.io',
		team: 'Data Platform',
		title: 'Platform engineer'
	},
	{
		id: 'hannah',
		name: 'Alex Taylor',
		kind: 'person',
		email: 'alex.taylor@shopwell.io',
		team: 'Governance',
		title: 'Data governance lead'
	},
	{
		id: 'dev',
		name: 'Sam Jones',
		kind: 'person',
		email: 'sam.jones@shopwell.io',
		team: 'Finance Analytics',
		title: 'Analyst'
	},
	{
		id: 'aoife',
		name: 'Chris Brown',
		kind: 'person',
		email: 'chris.brown@shopwell.io',
		team: 'Data Platform',
		title: 'Engineering manager'
	},
	{
		id: 'kenji',
		name: 'Pat Wilson',
		kind: 'person',
		email: 'pat.wilson@shopwell.io',
		team: 'Customer Insights',
		title: 'Data scientist'
	}
];
export const teams: Owner[] = [
	{ id: 'ae', name: 'Analytics Engineering', kind: 'team' },
	{ id: 'dp', name: 'Data Platform', kind: 'team' },
	{ id: 'fin', name: 'Finance Analytics', kind: 'team' },
	{ id: 'gov', name: 'Governance', kind: 'team' },
	{ id: 'ci', name: 'Customer Insights', kind: 'team' }
];
export const agents: Owner[] = [
	{
		id: 'support-triage-agent',
		name: 'support-triage-agent',
		kind: 'agent',
		runsAs: 'svc-support'
	},
	{ id: 'revenue-agent', name: 'revenue-agent', kind: 'agent', runsAs: 'svc-finance-agent' },
	{ id: 'catalog-steward', name: 'catalog-steward', kind: 'agent', runsAs: 'svc-governance' },
	{ id: 'cli-agent-jane', name: 'cli-agent (Jane Doe)', kind: 'agent', runsAs: 'jane.doe' }
];
export const owner = (id: string) => [...people, ...teams, ...agents].find((o) => o.id === id)!;

export interface Asset {
	mrn: string;
	name: string;
	type: string;
	provider: string;
	description: string;
	generated?: boolean;
	owners: string[];
	tags: string[];
	rows?: number;
	columns?: number;
	updated: string;
	views7d?: number;
	certified?: boolean;
	pii?: boolean;
	stale?: boolean;
}

export const assets: Asset[] = [
	{
		mrn: 'mrn://table/postgresql/shop.public.orders',
		name: 'orders',
		type: 'Table',
		provider: 'postgresql',
		description:
			'One row per customer order placed through the web shop, written by the checkout service and loaded hourly.',
		owners: ['ae', 'jane'],
		tags: ['commerce', 'pii'],
		rows: 1_284_019,
		columns: 14,
		updated: ago(78),
		views7d: 412,
		pii: true
	},
	{
		mrn: 'mrn://table/dbt/analytics.marts.orders_daily',
		name: 'orders_daily',
		type: 'Table',
		provider: 'dbt',
		description: 'Daily order totals, deduplicated and joined to customers_dim.',
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
		mrn: 'mrn://table/snowflake/analytics.marts.customers_dim',
		name: 'customers_dim',
		type: 'Table',
		provider: 'snowflake',
		description:
			'One row per customer with their current address, segment and lifetime value. Slowly changing, type 2.',
		owners: ['ci', 'kenji'],
		tags: ['customer', 'pii'],
		rows: 418_230,
		columns: 22,
		updated: ago(1_440),
		views7d: 640,
		certified: true,
		pii: true
	},
	{
		mrn: 'mrn://topic/kafka/shop.orders.v2',
		name: 'shop.orders.v2',
		type: 'Topic',
		provider: 'kafka',
		description: 'Order events from the checkout service. Avro, keyed by order_id, 12 partitions.',
		owners: ['dp', 'tomas'],
		tags: ['commerce', 'events'],
		updated: ago(4),
		views7d: 133
	},
	{
		mrn: 'mrn://table/bigquery/marmot-prod.analytics.sessions',
		name: 'sessions',
		type: 'Table',
		provider: 'bigquery',
		description: 'Web sessions stitched from clickstream, partitioned by day.',
		owners: ['ci'],
		tags: ['web'],
		rows: 92_400_118,
		columns: 31,
		updated: ago(190),
		views7d: 77
	},
	{
		mrn: 'mrn://dashboard/looker/finance.revenue_overview',
		name: 'Revenue overview',
		type: 'Dashboard',
		provider: 'looker',
		description:
			'Weekly revenue, refunds and gross margin by region for the finance leadership review.',
		owners: ['fin', 'dev'],
		tags: ['finance'],
		updated: ago(2_880),
		views7d: 301
	},
	{
		mrn: 'mrn://table/snowflake/finance.payments.charges',
		name: 'charges',
		type: 'Table',
		provider: 'snowflake',
		description: 'Card charges from Stripe, one row per charge attempt.',
		owners: ['fin'],
		tags: ['finance', 'pii'],
		rows: 3_902_551,
		columns: 18,
		updated: ago(65),
		views7d: 220,
		pii: true
	},
	{
		mrn: 'mrn://table/dbt/analytics.staging.stg_stripe__charges',
		name: 'stg_stripe__charges',
		type: 'View',
		provider: 'dbt',
		description: 'Renamed and typed Stripe charges.',
		generated: true,
		owners: ['ae'],
		tags: ['finance'],
		columns: 18,
		updated: ago(326),
		views7d: 41
	},
	{
		mrn: 'mrn://pipeline/airflow/orders_hourly_load',
		name: 'orders_hourly_load',
		type: 'Pipeline',
		provider: 'airflow',
		description: 'Loads checkout orders from PostgreSQL into Snowflake every hour.',
		owners: ['dp'],
		tags: ['commerce'],
		updated: ago(31),
		views7d: 58
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
		mrn: 'mrn://agent/mcp/support-triage-agent',
		name: 'support-triage-agent',
		type: 'Agent',
		provider: 'agent',
		description: 'Answers order-status questions in the support inbox using orders and refunds.',
		owners: ['ci'],
		tags: ['support'],
		updated: ago(12),
		views7d: 64
	},
	// Not in the mockups' catalog list, though the lineage and audit log already name it: the agent demo opens its page.
	{
		mrn: 'mrn://agent/mcp/revenue-agent',
		name: 'revenue-agent',
		type: 'Agent',
		provider: 'agent',
		description:
			'Answers revenue questions from the finance team in Slack, using certified finance tables.',
		owners: ['fin', 'dev'],
		tags: ['finance'],
		updated: ago(0),
		views7d: 118
	},
	{
		mrn: 'mrn://table/clickhouse/events.product_events',
		name: 'product_events',
		type: 'Table',
		provider: 'clickhouse',
		description: 'Product analytics events from the mobile apps.',
		owners: ['ci'],
		tags: ['web', 'events'],
		rows: 1_840_220_412,
		columns: 26,
		updated: ago(2),
		views7d: 95
	}
];
export const asset = (name: string) => assets.find((a) => a.name === name)!;

export const glossary = [
	{
		id: 'gmv',
		name: 'Gross merchandise value',
		domain: 'Commerce',
		owners: ['fin', 'dev'],
		definition:
			'The total value of orders placed in a period before refunds, discounts and VAT are removed. Use it to compare demand across periods; use net revenue for anything reported to finance.',
		linked: 4,
		updated: ago(2_000)
	},
	{
		id: 'nrr',
		name: 'Net revenue retention',
		domain: 'Commerce',
		owners: ['fin'],
		definition:
			'Revenue from customers who were active twelve months ago, as a percentage of their revenue then.',
		linked: 2,
		updated: ago(9_000),
		stale: true
	},
	{
		id: 'arr',
		name: 'Annual recurring revenue',
		domain: 'Finance',
		owners: ['fin'],
		definition: 'Subscription revenue normalised to a year.',
		linked: 3,
		updated: ago(14_000)
	},
	{
		id: 'active-customer',
		name: 'Active customer',
		domain: 'Customer',
		owners: ['ci', 'kenji'],
		definition: 'A customer with at least one paid order in the last 90 days.',
		linked: 6,
		updated: ago(400)
	},
	{
		id: 'pii',
		name: 'Personal data (PII)',
		domain: 'Governance',
		owners: ['gov', 'hannah'],
		definition:
			'Data that identifies a person directly or in combination: name, email, phone, address, payment details. Agents may not read PII columns without an explicit grant.',
		linked: 41,
		updated: ago(700)
	}
];

export const products = [
	{
		id: 'dp-commerce',
		name: 'Commerce core',
		description: 'Orders, refunds and customers for anyone reporting on shop performance.',
		owners: ['ae'],
		assets: 14,
		rules: 2,
		updated: ago(1_200)
	},
	{
		id: 'dp-finance',
		name: 'Finance reporting',
		description: 'Certified finance tables and the revenue dashboards built on them.',
		owners: ['fin'],
		assets: 9,
		rules: 1,
		updated: ago(3_000)
	},
	{
		id: 'dp-web',
		name: 'Web analytics',
		description: 'Sessions, product events and attribution.',
		owners: ['ci'],
		assets: 22,
		rules: 3,
		updated: ago(600)
	}
];

/** The lineage around orders_daily. */
export const ordersLineage = {
	currentId: 'orders_daily',
	nodes: [
		{
			id: 'orders',
			mrn: 'mrn://table/postgresql/shop.public.orders',
			name: 'orders',
			type: 'Table',
			provider: 'postgresql'
		},
		{
			id: 'shop.orders.v2',
			mrn: 'mrn://topic/kafka/shop.orders.v2',
			name: 'shop.orders.v2',
			type: 'Topic',
			provider: 'kafka'
		},
		{
			id: 'orders_hourly_load',
			mrn: 'mrn://pipeline/airflow/orders_hourly_load',
			name: 'orders_hourly_load',
			type: 'Pipeline',
			provider: 'airflow'
		},
		{
			id: 'stg_orders',
			mrn: 'mrn://table/dbt/analytics.staging.stg_orders',
			name: 'stg_orders',
			type: 'View',
			provider: 'dbt'
		},
		{
			id: 'customers_dim',
			mrn: 'mrn://table/snowflake/analytics.marts.customers_dim',
			name: 'customers_dim',
			type: 'Table',
			provider: 'snowflake'
		},
		{
			id: 'orders_daily',
			mrn: 'mrn://table/dbt/analytics.marts.orders_daily',
			name: 'orders_daily',
			type: 'Table',
			provider: 'dbt'
		},
		{
			id: 'revenue',
			mrn: 'mrn://dashboard/looker/finance.revenue_overview',
			name: 'Revenue overview',
			type: 'Dashboard',
			provider: 'looker'
		},
		{
			id: 'refund_rates',
			mrn: 'mrn://table/dbt/analytics.marts.refund_rates',
			name: 'refund_rates',
			type: 'Table',
			provider: 'dbt',
			state: 'failed'
		},
		{
			id: 'ol_stub',
			mrn: 'mrn://table/spark/lake.orders_enriched',
			name: 'orders_enriched',
			type: 'Table',
			provider: 'spark',
			state: 'stub'
		},
		{
			id: 'support-triage-agent',
			mrn: 'mrn://agent/mcp/support-triage-agent',
			name: 'support-triage-agent',
			type: 'Agent',
			provider: 'agent'
		}
	],
	edges: [
		{ source: 'orders', target: 'orders_hourly_load', kind: 'FEEDS', origin: 'declared' },
		{
			source: 'shop.orders.v2',
			target: 'orders_hourly_load',
			kind: 'FEEDS',
			origin: 'observed',
			observations: 128
		},
		{ source: 'orders_hourly_load', target: 'stg_orders', kind: 'PRODUCES', origin: 'declared' },
		{ source: 'stg_orders', target: 'orders_daily', kind: 'FEEDS', origin: 'declared' },
		{
			source: 'customers_dim',
			target: 'orders_daily',
			kind: 'FEEDS',
			origin: 'mixed',
			observations: 40
		},
		{ source: 'orders_daily', target: 'revenue', kind: 'FEEDS', origin: 'declared' },
		{ source: 'orders_daily', target: 'refund_rates', kind: 'FEEDS', origin: 'declared' },
		{
			source: 'orders_daily',
			target: 'ol_stub',
			kind: 'FEEDS',
			origin: 'observed',
			observations: 6
		},
		{
			source: 'orders_daily',
			target: 'support-triage-agent',
			kind: 'AGENT_LOOKUP',
			origin: 'observed',
			observations: 311
		}
	]
};
