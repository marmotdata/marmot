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

/** The source explorer tree. The mockup opens postgres-shop; the demo queries orders_daily, which lives in Snowflake. */
const col = (id: string, name: string, type: string, pii = false) => ({
	id: `${id}.${name}`,
	label: name,
	icon: 'view-column',
	secondary: type,
	badges: pii ? [{ label: 'PII', variant: 'pii' as const }] : undefined
});
const MARTS = 'snowflake-finance.analytics.marts';
export const sourceTree = [
	{
		id: 'postgres-shop',
		label: 'postgres-shop',
		provider: 'postgresql',
		secondary: 'Read only',
		secondaryMono: false,
		hasChildren: true
	},
	{
		id: 'snowflake-finance',
		label: 'snowflake-finance',
		provider: 'snowflake',
		secondary: 'Read only',
		secondaryMono: false,
		children: [
			{
				id: 'snowflake-finance.analytics',
				label: 'analytics',
				icon: 'database',
				children: [
					{
						id: MARTS,
						label: 'marts',
						icon: 'schema',
						secondary: '4 tables',
						secondaryMono: false,
						children: [
							{
								id: `${MARTS}.orders_daily`,
								label: 'orders_daily',
								icon: 'table',
								secondary: '1,284',
								secondaryMono: false,
								children: ordersDailySchema.map((c) => col(`${MARTS}.orders_daily`, c.name, c.type))
							},
							{
								id: `${MARTS}.customers_dim`,
								label: 'customers_dim',
								icon: 'table',
								secondary: '418K',
								secondaryMono: false,
								children: [
									col(`${MARTS}.customers_dim`, 'customer_id', 'number'),
									col(`${MARTS}.customers_dim`, 'email', 'varchar', true),
									col(`${MARTS}.customers_dim`, 'phone', 'varchar', true),
									col(`${MARTS}.customers_dim`, 'segment', 'varchar'),
									col(`${MARTS}.customers_dim`, 'country_code', 'char(2)'),
									col(`${MARTS}.customers_dim`, 'last_order_at', 'timestamp_ntz'),
									col(`${MARTS}.customers_dim`, 'lifetime_value_gbp', 'number(14,2)')
								]
							},
							{
								id: `${MARTS}.refund_rates`,
								label: 'refund_rates',
								icon: 'table',
								secondary: '6,240',
								secondaryMono: false,
								hasChildren: true
							},
							{
								id: `${MARTS}.order_payments`,
								label: 'order_payments',
								icon: 'table',
								secondary: '1.3M',
								secondaryMono: false,
								hasChildren: true
							}
						]
					}
				]
			},
			{
				id: 'snowflake-finance.finance',
				label: 'finance',
				icon: 'database',
				hasChildren: true
			}
		]
	},
	{
		id: 'bigquery-web',
		label: 'bigquery-web',
		provider: 'bigquery',
		secondary: 'Read only',
		secondaryMono: false,
		hasChildren: true
	}
];
export const treeExpanded = [
	'snowflake-finance',
	'snowflake-finance.analytics',
	MARTS,
	`${MARTS}.orders_daily`
];
export const treeSelected = `${MARTS}.orders_daily`;

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

/** Last week's revenue from the certified daily table: Monday 19 to Sunday 25 October, 7 rows. */
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

/** Reaches for the email column of customers_dim, which is tagged PII, so Marmot refuses it. */
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
		mrn: 'mrn://table/dbt/analytics.marts.orders_daily',
		owner: 'Analytics Engineering',
		rows: 1_284,
		columnsRead: 5,
		updated: ago(326),
		pii: '',
		certified: true
	}
];
export const deniedTablesRead = [
	{
		id: 'customers_dim',
		name: 'analytics.marts.customers_dim',
		mrn: 'mrn://table/snowflake/analytics.marts.customers_dim',
		owner: 'Customer Insights',
		rows: 418_230,
		columnsRead: 4,
		updated: ago(1_440),
		pii: 'email',
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
		reason: 'Your role analyst has no grant on PII columns of analytics.marts.customers_dim.'
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

/** What revenue-agent runs during the demo, oldest first: the revenue query, then a reach for PII. */
export const agentAudit: AuditEvent[] = [
	{
		id: 'q_01JBK83F6N2W7D',
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
		id: 'q_01JBK84R1C9ZPA',
		at: '2026-10-27T09:29:48Z',
		actor: 'revenue-agent',
		kind: 'agent',
		sa: 'svc-finance-agent',
		client: 'mcp',
		source: 'snowflake-finance',
		provider: 'snowflake',
		sql: DENIED_SQL,
		outcome: 'denied',
		reason:
			'Denied because svc-finance-agent has no grant on the PII column email of analytics.marts.customers_dim. The PII policy requires the pii-read grant for agents.',
		ms: 3,
		tables: ['mrn://table/snowflake/analytics.marts.customers_dim'],
		decision: 'Denied by PII policy'
	}
];

export const audit: AuditEvent[] = [
	{
		id: 'q_01JBK7Z3D9W4R6',
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
		id: 'q_01JBK7XQ2M8C1T',
		at: ago(4),
		actor: 'support-triage-agent',
		kind: 'agent',
		sa: 'svc-support',
		client: 'mcp',
		source: 'postgres-shop',
		provider: 'postgresql',
		sql: 'select c.email, c.phone, o.total_gbp\nfrom shop.public.orders o\njoin shop.analytics.customers_dim c using (customer_id)\nwhere o.order_id = 5521907',
		outcome: 'denied',
		reason:
			'Denied because svc-support has no grant on the PII columns email and phone of shop.analytics.customers_dim. The PII policy requires the pii-read grant for agents.',
		ms: 4,
		tables: [
			'mrn://table/postgresql/shop.public.orders',
			'mrn://table/postgresql/shop.analytics.customers_dim'
		],
		decision: 'Denied by PII policy'
	},
	{
		id: 'q_01JBK7S0H6V3NA',
		at: ago(12),
		actor: 'Jane Doe',
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
		id: 'q_01JBK7NB0T4XWE',
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
		id: 'q_01JBK7J7RZ2KQ5',
		at: ago(41),
		actor: 'Sam Jones',
		kind: 'person',
		client: 'cli',
		source: 'bigquery-web',
		provider: 'bigquery',
		sql: 'select count(*)\nfrom analytics.sessions\nwhere session_start >= timestamp_sub(current_timestamp(), interval 1 day)',
		outcome: 'failed',
		reason:
			'The query exceeded the 30 s timeout for bigquery-web. Filter on the partition column session_date so BigQuery can prune partitions.',
		ms: 30_000,
		tables: ['mrn://table/bigquery/marmot-prod.analytics.sessions'],
		decision: 'Allowed by role analyst'
	},
	{
		id: 'q_01JBK7F3C0PRM9',
		at: ago(58),
		actor: 'cli-agent (Jane Doe)',
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
		id: 'q_01JBK79V4HDN0S',
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
		id: 'q_01JBK72AQ7G5LB',
		at: ago(96),
		actor: 'Alex Taylor',
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
		id: 'q_01JBK6WZ8EYFJ3',
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
		id: 'q_01JBK6P1KS3MTV',
		at: ago(188),
		actor: 'revenue-agent',
		kind: 'agent',
		sa: 'svc-finance-agent',
		client: 'api',
		source: 'snowflake-finance',
		provider: 'snowflake',
		sql: 'select * from analytics.marts.customers_dim limit 500',
		outcome: 'denied',
		reason:
			'Denied because select * on shop customers_dim reads 3 PII columns, and svc-finance-agent is limited to aggregates on PII tables.',
		ms: 3,
		tables: ['mrn://table/snowflake/analytics.marts.customers_dim'],
		decision: 'Denied by aggregates-only grant'
	},
	{
		id: 'q_01JBK6GH5W0ZDC',
		at: ago(240),
		actor: 'Pat Wilson',
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
		id: 'q_01JBK69MT2RA8F',
		at: ago(302),
		actor: 'John Smith',
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
