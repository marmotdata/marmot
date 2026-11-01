/**
 * Sample data for the Query workspace and the Query audit log. Builds on catalog.ts so the same
 * pipelines, people, agents and tables appear everywhere.
 * "Now" is catalog.ts's fixed NOW: Tuesday 27 October 2026, 09:30 UTC.
 */
import { NOW, ago } from './catalog';
import { ordersDailySchema } from './asset';

/** Relative time from the fixed NOW, so the screens never drift ("31 min ago", "7 h ago", "3 d ago"). */
export function rel(iso: string): string {
	if (!iso) return '';
	const m = Math.round((NOW.getTime() - new Date(iso).getTime()) / 60_000);
	if (m < 1) return 'just now';
	if (m < 60) return `${m} min ago`;
	if (m < 1_440) return `${Math.round(m / 60)} h ago`;
	return `${Math.round(m / 1_440)} d ago`;
}

/** "3 min 12 s", "412 ms", "41 s". */
export function dur(ms: number): string {
	if (ms < 1_000) return `${ms} ms`;
	const s = Math.round(ms / 1_000);
	if (s < 60) return `${s} s`;
	const m = Math.floor(s / 60);
	const r = s % 60;
	return r ? `${m} min ${r} s` : `${m} min`;
}

/** A UTC clock time for logs and tables: "02:03:41". */
export const clock = (iso: string) => new Date(iso).toISOString().slice(11, 19);

/* ------------------------------------------------------------------ Query workspace */

/**
 * The source explorer tree, built the way the app builds it: each source shows its table count, a
 * source with more than one schema gets a level per schema, tables show the row count the catalog
 * last saw (with a PII badge when the table is tagged PII) and columns show their type in mono.
 * The demo queries orders_daily, which lives in snowflake-finance.
 */
const ORDERS_DAILY = 'mrn://table/snowflake/analytics.marts.orders_daily';
const col = (table: string, name: string, type: string) => ({
	id: `c:snowflake-finance:${table}:${name}`,
	label: name,
	icon: 'view-column',
	secondary: type,
	secondaryMono: true
});
const table = (mrn: string, name: string, rows: string, pii = false, children?: unknown[]) => ({
	id: `t:snowflake-finance:${mrn}`,
	label: name,
	icon: 'table',
	secondary: rows,
	secondaryMono: false,
	hasChildren: true,
	badges: pii ? [{ label: 'PII', variant: 'pii' as const }] : undefined,
	...(children ? { children } : {})
});
export const sourceTree = [
	{ id: 's:postgres-shop', label: 'postgres-shop', provider: 'postgresql', secondary: '12', hasChildren: true },
	{
		id: 's:snowflake-finance',
		label: 'snowflake-finance',
		provider: 'snowflake',
		secondary: '5',
		children: [
			{
				id: 'sc:snowflake-finance:analytics.marts',
				label: 'analytics.marts',
				icon: 'schema',
				secondary: '4',
				children: [
					table(
						ORDERS_DAILY,
						'orders_daily',
						'1,284',
						false,
						ordersDailySchema.map((c) => col(ORDERS_DAILY, c.name, c.type))
					),
					table('mrn://table/snowflake/analytics.marts.customers_dim', 'customers_dim', '418K', true),
					table('mrn://table/snowflake/analytics.marts.refund_rates', 'refund_rates', '6,240'),
					table('mrn://table/snowflake/analytics.marts.order_payments', 'order_payments', '1.28M')
				]
			},
			{
				id: 'sc:snowflake-finance:finance.payments',
				label: 'finance.payments',
				icon: 'schema',
				secondary: '1',
				children: [table('mrn://table/snowflake/finance.payments.charges', 'charges', '2.41M')]
			}
		]
	},
	{ id: 's:bigquery-web', label: 'bigquery-web', provider: 'bigquery', secondary: '8', hasChildren: true }
];
/** Open down to the tables, so the explorer lists them with their row counts rather than a column of truncated field names. */
export const treeExpanded = ['s:snowflake-finance', 'sc:snowflake-finance:analytics.marts'];
/** The app selects the source queries run against, not a table. */
export const treeSelected = 's:snowflake-finance';

/** Tables offered under the empty results, each with the sample query the app would open. */
export const starters = [
	{ name: 'orders_daily', description: 'Daily order totals, deduplicated and joined to customers_dim.' },
	{
		name: 'customers_dim',
		description:
			'One row per customer with their current address, segment and lifetime value. Slowly changing, type 2.'
	},
	{ name: 'refund_rates', description: '' },
	{ name: 'order_payments', description: '' }
];

/** Completion schema for the SQL editor. */
export const editorSchema = {
	'analytics.marts.orders_daily': ordersDailySchema.map((c) => c.name),
	'analytics.marts.customers_dim': [
		'customer_id',
		'email',
		'phone',
		'segment',
		'country_code',
		'last_order_at',
		'lifetime_value_gbp'
	],
	'finance.payments.charges': [
		'charge_id',
		'customer_id',
		'amount',
		'currency',
		'region',
		'status',
		'created'
	]
};

/** Last week's revenue from the daily table: Monday 19 to Sunday 25 October, 7 rows. */
export const QUERY_SQL = `-- Revenue last week, by day
select
  order_date,
  orders,
  customers,
  gross_gbp,
  net_revenue_gbp
from analytics.marts.orders_daily
where order_date >= dateadd(week, -1, date_trunc('week', current_date))
  and order_date <  date_trunc('week', current_date)
order by order_date;`;

/** Reads customers_dim, a PII table the caller holds no asset:query grant on, so Marmot refuses it before it reaches Snowflake. */
export const DENIED_SQL = `-- Last week's best customers, to thank them by email
select
  c.customer_id,
  c.email,
  c.segment,
  c.lifetime_value_gbp
from analytics.marts.customers_dim c
where c.last_order_at >= dateadd(week, -1, date_trunc('week', current_date))
order by c.lifetime_value_gbp desc
limit 20;`;

export const resultColumns = [
	{ name: 'order_date', type: 'date' },
	{ name: 'orders', type: 'number' },
	{ name: 'customers', type: 'number' },
	{ name: 'gross_gbp', type: 'number(14,2)' },
	{ name: 'net_revenue_gbp', type: 'number(14,2)' }
];

/** Seven days that add up to the agent's answer: 8,615 orders and £413,180.00 net revenue. */
export const resultRows: unknown[][] = [
	['2026-10-19', 1_302, 1_214, '79262.14', '62410.35'],
	['2026-10-20', 1_248, 1_163, '76047.75', '59880.12'],
	['2026-10-21', 1_190, 1_109, '72520.15', '57102.48'],
	['2026-10-22', 1_236, 1_151, '75326.11', '59311.90'],
	['2026-10-23', 1_311, 1_220, '80066.86', '63044.77'],
	['2026-10-24', 1_197, 1_118, '73101.47', '57560.21'],
	['2026-10-25', 1_131, 1_057, '68415.12', '53870.17']
];

/** What the catalog knew about each table the query read. */
export const tablesRead = [
	{
		id: 'orders_daily',
		name: 'analytics.marts.orders_daily',
		mrn: 'mrn://table/snowflake/analytics.marts.orders_daily',
		owner: 'Analytics Engineering',
		rows: 1_284,
		columnsRead: 5,
		updated: ago(326),
		pii: '',
		certified: true
	}
];
/** Query history for the workspace's History tab. */
export const history = [
	{
		id: 'h1',
		sql: QUERY_SQL,
		status: 'succeeded' as const,
		source: { name: 'snowflake-finance', provider: 'snowflake' },
		rows: 7,
		durationMs: 412,
		startedAt: ago(1)
	},
	{
		id: 'h2',
		sql: DENIED_SQL,
		status: 'denied' as const,
		source: { name: 'snowflake-finance', provider: 'snowflake' },
		startedAt: ago(6),
		reason: 'you do not have asset:query on mrn://table/snowflake/analytics.marts.customers_dim'
	},
	{
		id: 'h3',
		sql: 'select status, count(*) from shop.public.orders group by status',
		status: 'succeeded' as const,
		source: { name: 'postgres-shop', provider: 'postgresql' },
		rows: 5,
		durationMs: 220,
		startedAt: ago(19)
	},
	{
		id: 'h4',
		sql: "select date_trunc('day', order_date) as day, sum(net_revenue_gbp)\nfrom analytics.marts.orders_daily\ngroup by 1 order by 1 desc",
		status: 'succeeded' as const,
		source: { name: 'snowflake-finance', provider: 'snowflake' },
		rows: 90,
		durationMs: 388,
		startedAt: ago(140)
	},
	{
		id: 'h5',
		sql: 'select region, sum(amount) from finance.payments.charges where created >= current_date - 7 group by region',
		status: 'succeeded' as const,
		source: { name: 'snowflake-finance', provider: 'snowflake' },
		rows: 6,
		durationMs: 1_840,
		startedAt: ago(1_500)
	}
];

/* ------------------------------------------------------------------ Query audit */

/** The Query audit log: newest first. `client` uses the audit-row keys. */
export interface AuditEvent {
	id: string;
	at: string;
	actor: string;
	kind: 'person' | 'agent';
	sa?: string;
	client: 'mcp' | 'cli' | 'api' | 'editor';
	source: string;
	provider: string;
	sql: string;
	rows?: number;
	truncated?: boolean;
	outcome: 'allowed' | 'denied' | 'failed';
	reason?: string;
	ms: number;
	tables: string[];
	decision: string;
}

/** What revenue-agent runs during the demo, oldest first: the revenue query, then a reach for customers_dim. */
export const agentAudit: AuditEvent[] = [
	{
		id: '9ec02ef9-9693-47fa-9464-3a5ee1c19f47',
		at: '2026-10-27T09:29:12Z',
		actor: 'revenue-agent',
		kind: 'agent',
		sa: 'svc-finance-agent',
		client: 'mcp',
		source: 'snowflake-finance',
		provider: 'snowflake',
		sql: QUERY_SQL,
		rows: 7,
		outcome: 'allowed',
		ms: 412,
		tables: ['mrn://table/snowflake/analytics.marts.orders_daily'],
		decision: 'Allowed by role asset.dataViewer on orders_daily'
	},
	{
		id: '62bacc3c-3b2d-4f55-ae98-4603ca18016a',
		at: '2026-10-27T09:29:48Z',
		actor: 'revenue-agent',
		kind: 'agent',
		sa: 'svc-finance-agent',
		client: 'mcp',
		source: 'snowflake-finance',
		provider: 'snowflake',
		sql: DENIED_SQL,
		outcome: 'denied',
		reason: 'you do not have asset:query on mrn://table/snowflake/analytics.marts.customers_dim',
		ms: 3,
		tables: ['mrn://table/snowflake/analytics.marts.customers_dim'],
		decision: 'Denied by permission'
	}
];

export const audit: AuditEvent[] = [
	{
		id: '45071117-bc32-49cf-b1ee-32061d98db84',
		at: ago(3),
		actor: 'support-triage-agent',
		kind: 'agent',
		sa: 'svc-support',
		client: 'mcp',
		source: 'postgres-shop',
		provider: 'postgresql',
		sql: 'select order_id, status, placed_at\nfrom shop.public.orders\nwhere customer_id = 81723\norder by placed_at desc\nlimit 5',
		rows: 5,
		outcome: 'allowed',
		ms: 38,
		tables: ['mrn://table/postgresql/shop.public.orders'],
		decision: 'Allowed by grant support-orders-read'
	},
	{
		id: 'b233335b-18e1-4801-9546-871831b60fe9',
		at: ago(4),
		actor: 'support-triage-agent',
		kind: 'agent',
		sa: 'svc-support',
		client: 'mcp',
		source: 'postgres-shop',
		provider: 'postgresql',
		sql: 'select c.email, c.phone, o.total_gbp\nfrom shop.public.orders o\njoin shop.analytics.customers_dim c using (customer_id)\nwhere o.order_id = 5521907',
		outcome: 'denied',
		reason: 'you do not have asset:query on mrn://table/postgresql/shop.analytics.customers_dim',
		ms: 4,
		tables: [
			'mrn://table/postgresql/shop.public.orders',
			'mrn://table/postgresql/shop.analytics.customers_dim'
		],
		decision: 'Denied by permission'
	},
	{
		id: '227e79c7-fdff-4ec9-8533-4809bc1b93e8',
		at: ago(12),
		actor: 'jane.doe',
		kind: 'person',
		client: 'editor',
		source: 'snowflake-finance',
		provider: 'snowflake',
		sql: "select date_trunc('day', order_date) as day, sum(net_revenue_gbp)\nfrom analytics.marts.orders_daily\ngroup by 1 order by 1 desc",
		rows: 90,
		outcome: 'allowed',
		ms: 412,
		tables: ['mrn://table/snowflake/analytics.marts.orders_daily'],
		decision: 'Allowed by role analyst'
	},
	{
		id: '73dd7bad-a271-47bf-bdc4-0bbc6b9224b5',
		at: ago(26),
		actor: 'revenue-agent',
		kind: 'agent',
		sa: 'svc-finance-agent',
		client: 'api',
		source: 'snowflake-finance',
		provider: 'snowflake',
		sql: 'select region, sum(amount) as amount_gbp\nfrom finance.payments.charges\nwhere created >= current_date - 7\ngroup by region',
		rows: 6,
		outcome: 'allowed',
		ms: 1_840,
		tables: ['mrn://table/snowflake/finance.payments.charges'],
		decision: 'Allowed by grant finance-aggregates'
	},
	{
		id: '38160a09-7aa4-4304-be52-c3fcec4f02fb',
		at: ago(41),
		actor: 'sam.jones',
		kind: 'person',
		client: 'cli',
		source: 'bigquery-web',
		provider: 'bigquery',
		sql: 'select count(*)\nfrom analytics.sessions\nwhere session_start >= timestamp_sub(current_timestamp(), interval 1 day)',
		outcome: 'failed',
		reason: 'context deadline exceeded',
		ms: 30_000,
		tables: ['mrn://table/bigquery/marmot-prod.analytics.sessions'],
		decision: 'Allowed by role analyst'
	},
	{
		id: '213354c9-dc68-4395-a0c2-77f29a6a3a27',
		at: ago(58),
		actor: 'claude-code',
		kind: 'agent',
		sa: 'jane.doe',
		client: 'mcp',
		source: 'postgres-shop',
		provider: 'postgresql',
		sql: 'select status, count(*)\nfrom shop.public.orders\ngroup by status',
		rows: 5,
		outcome: 'allowed',
		ms: 220,
		tables: ['mrn://table/postgresql/shop.public.orders'],
		decision: 'Allowed by role analyst (on behalf of Jane Doe)'
	},
	{
		id: '54851cf1-5d75-4827-941e-7fa10fc84a16',
		at: ago(74),
		actor: 'catalog-steward',
		kind: 'agent',
		sa: 'svc-governance',
		client: 'mcp',
		source: 'snowflake-finance',
		provider: 'snowflake',
		sql: "select table_name, last_altered\nfrom information_schema.tables\nwhere table_schema = 'PAYMENTS'",
		rows: 18,
		outcome: 'allowed',
		ms: 640,
		tables: ['mrn://table/snowflake/finance.information_schema.tables'],
		decision: 'Allowed by grant metadata-read'
	},
	{
		id: '8c44b6b8-9eb9-45cf-9e73-2d7b9e9d875e',
		at: ago(96),
		actor: 'alex.taylor',
		kind: 'person',
		client: 'editor',
		source: 'snowflake-finance',
		provider: 'snowflake',
		sql: "select customer_id, card_last4, amount\nfrom finance.payments.charges\nwhere status = 'disputed'",
		rows: 212,
		outcome: 'allowed',
		ms: 905,
		tables: ['mrn://table/snowflake/finance.payments.charges'],
		decision: 'Allowed by role governance'
	},
	{
		id: '19009545-f970-4c48-8fd5-505de4d741a1',
		at: ago(131),
		actor: 'support-triage-agent',
		kind: 'agent',
		sa: 'svc-support',
		client: 'mcp',
		source: 'postgres-shop',
		provider: 'postgresql',
		sql: 'select refund_id, amount_gbp, reason\nfrom shop.public.refunds\nwhere order_id = 5519033',
		rows: 1,
		outcome: 'allowed',
		ms: 29,
		tables: ['mrn://table/postgresql/shop.public.refunds'],
		decision: 'Allowed by grant support-orders-read'
	},
	{
		id: 'e3cac74e-5faa-4327-a810-9ee506675cb2',
		at: ago(188),
		actor: 'revenue-agent',
		kind: 'agent',
		sa: 'svc-finance-agent',
		client: 'api',
		source: 'snowflake-finance',
		provider: 'snowflake',
		sql: 'select * from analytics.marts.customers_dim limit 500',
		outcome: 'denied',
		reason: 'you do not have asset:query on mrn://table/snowflake/analytics.marts.customers_dim',
		ms: 3,
		tables: ['mrn://table/snowflake/analytics.marts.customers_dim'],
		decision: 'Denied by permission'
	},
	{
		id: '3f0fd2c8-cbb1-417d-9bb6-82c4f0a803c9',
		at: ago(240),
		actor: 'pat.wilson',
		kind: 'person',
		client: 'cli',
		source: 'bigquery-web',
		provider: 'bigquery',
		sql: "select device, count(*) as sessions\nfrom analytics.sessions\nwhere session_date = '2026-10-26'\ngroup by device",
		rows: 3,
		outcome: 'allowed',
		ms: 2_210,
		tables: ['mrn://table/bigquery/marmot-prod.analytics.sessions'],
		decision: 'Allowed by role analyst'
	},
	{
		id: '0787b90e-0892-44f0-bd8d-aa755058076e',
		at: ago(302),
		actor: 'john.smith',
		kind: 'person',
		client: 'editor',
		source: 'postgres-shop',
		provider: 'postgresql',
		sql: 'select count(*) from shop.public.order_items',
		rows: 1,
		outcome: 'allowed',
		ms: 1_130,
		tables: ['mrn://table/postgresql/shop.public.order_items'],
		decision: 'Allowed by role admin'
	}
];
