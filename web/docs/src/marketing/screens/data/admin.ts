/**
 * Sample data for Settings › Access. Builds on catalog.ts so the same people, teams and assets
 * appear everywhere. "Now" is catalog.ts's NOW: Tuesday 27 October 2026, 09:30 UTC.
 */
import { NOW, ago } from './catalog';

export interface ApiKey {
	id: string;
	prefix: string;
	name: string;
	created: string;
	lastUsed: string;
	expires: string;
	state: 'active' | 'expiring' | 'expired';
}

export const serviceAccounts = [
	{
		id: 'svc-support',
		name: 'svc-support',
		description: 'Runs support-triage-agent in the support inbox.',
		roles: ['Viewer'],
		keys: 2,
		lastUsed: ago(3),
		agent: 'support-triage-agent',
		created: ago(140_000)
	},
	{
		id: 'svc-finance-agent',
		name: 'svc-finance-agent',
		description: 'Runs revenue-agent for the finance team.',
		roles: ['Viewer'],
		keys: 1,
		lastUsed: ago(1),
		agent: 'revenue-agent',
		created: ago(90_000)
	},
	{
		id: 'svc-governance',
		name: 'svc-governance',
		description: 'Runs catalog-steward, which proposes tags and owners.',
		roles: ['Editor', 'Governance'],
		keys: 1,
		lastUsed: ago(45),
		agent: 'catalog-steward',
		created: ago(60_000)
	},
	{
		id: 'svc-ci',
		name: 'svc-ci',
		description: 'Pushes dbt manifests from CI after each merge.',
		roles: ['Editor'],
		keys: 3,
		lastUsed: ago(326),
		agent: '',
		created: ago(200_000)
	}
];

export const financeKeys: ApiKey[] = [
	{
		id: 'k1',
		prefix: 'mk_live_Fa31',
		name: 'revenue-agent-prod',
		created: '2026-08-26T11:40:00Z',
		lastUsed: ago(1),
		expires: '2027-02-26T11:40:00Z',
		state: 'active'
	}
];

/** A role held on the whole catalog or on one asset. */
export interface Grant {
	id: string;
	role: string;
	on: string;
	provider?: string;
	by: string;
	at: string;
}

/** svc-finance-agent's grants before the demo. */
export const financeGrants: Grant[] = [
	{ id: 'g1', role: 'catalog.viewer', on: 'Whole catalog', by: 'Chris Brown', at: ago(90_000) },
	{
		id: 'g2',
		role: 'asset.dataViewer',
		on: 'charges',
		provider: 'snowflake',
		by: 'Sam Jones',
		at: ago(30_000)
	},
	{
		id: 'g3',
		role: 'asset.viewer',
		on: 'Revenue overview',
		provider: 'looker',
		by: 'Sam Jones',
		at: ago(30_000)
	}
];

/** The grant the access demo adds. */
export const newGrant: Grant = {
	id: 'g-orders-daily',
	role: 'asset.dataViewer',
	on: 'orders_daily',
	provider: 'dbt',
	by: 'Jane Doe',
	at: NOW.toISOString()
};
