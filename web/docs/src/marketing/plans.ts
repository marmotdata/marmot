// The plan catalog as Marmot Cloud sells it. The console reads it from the Cloud API; this is the
// static copy the marketing pages are built from, so keep it in step with cloud.marmotdata.io.

export type PlanId = 'free' | 'team' | 'scale' | 'enterprise';

export interface Plan {
	id: PlanId;
	name: string;
	tagline: string;
	/** Per compute unit per month: 0 on Free, null where it is negotiated. */
	unit_price: number | null;
	/** Both 0 on a plan that sells no units; null max_units has no ceiling. */
	min_units: number;
	max_units: number | null;
	max_instances: number | null;
	/** A fixed cap on a plan without units; on the others it follows the units. */
	max_assets: number | null;
	features: string[];
	sso: boolean;
	custom_domains: boolean;
	highlighted: boolean;
}

export function formatMoney(amount: number, currency = 'USD'): string {
	return new Intl.NumberFormat('en-US', {
		style: 'currency',
		currency,
		maximumFractionDigits: amount % 1 === 0 ? 0 : 2
	}).format(amount);
}

/** Static copy of the catalog for the prerendered landing page; the console reads it from the API. */
export const PLAN_LIST: Plan[] = [
	{
		id: 'free',
		name: 'Free',
		tagline: 'A small catalog to try Marmot, on us.',
		unit_price: 0,
		min_units: 0,
		max_units: 0,
		max_instances: 1,
		max_assets: 500,
		features: [
			'All plugins included',
			'Federated secret stores',
			'Shared MCP endpoint for AI agents',
			'Community support'
		],
		sso: false,
		custom_domains: false,
		highlighted: false
	},
	{
		id: 'team',
		name: 'Team',
		tagline: 'One instance for a team that runs Marmot as its shared source of truth.',
		unit_price: 595,
		min_units: 1,
		max_units: 3,
		max_instances: 1,
		max_assets: null,
		features: [
			'Everything in Free',
			'Dedicated compute units',
			'Private plugin registry',
			'SSO (OIDC) with role mapping & JIT provisioning',
			'Email support, 99.5% uptime SLA'
		],
		sso: true,
		custom_domains: false,
		highlighted: false
	},
	{
		id: 'scale',
		name: 'Scale',
		tagline: 'As many units as your catalog and agents need, on your own domain.',
		unit_price: 595,
		min_units: 4,
		max_units: null,
		max_instances: 1,
		max_assets: null,
		features: [
			'Everything in Team',
			'Four or more compute units',
			'Bring your own domain',
			'Priority support & shared Slack, 99.9% uptime SLA'
		],
		sso: true,
		custom_domains: true,
		highlighted: true
	},
	{
		id: 'enterprise',
		name: 'Enterprise',
		tagline: 'Multiple instances, custom terms and a direct line to us.',
		unit_price: null,
		min_units: 0,
		max_units: 0,
		max_instances: 3,
		max_assets: null,
		features: [
			'Everything in Scale',
			'Per-agent audit trail & SIEM export',
			'Plugin development on request',
			'VPC peering & data residency',
			'Custom backup retention',
			'Named support contact, 99.95% uptime SLA'
		],
		sso: true,
		custom_domains: true,
		highlighted: false
	}
];

export const PLANS = Object.fromEntries(PLAN_LIST.map((p) => [p.id, p])) as Record<PlanId, Plan>;

/** Whether the plan is billed per compute unit. */
export function sellsUnits(plan: Plan): boolean {
	return plan.min_units > 0;
}

/** "1 to 3 units", "4+ units". */
export function unitRange(plan: Plan): string {
	return plan.max_units === null
		? `${plan.min_units}+ units`
		: `${plan.min_units} to ${plan.max_units} units`;
}

export interface PlanLimit {
	label: string;
	value: string;
}

export function planLimits(plan: Plan, assetsPerUnit = 5000): PlanLimit[] {
	const count = (v: number | null, unit: string, one = unit) =>
		v === null ? 'Unlimited' : `${v.toLocaleString('en-US')} ${v === 1 ? one : unit}`;
	return [
		{ label: 'Instances', value: count(plan.max_instances, 'instances', 'instance') },
		{
			label: 'Compute units',
			value: sellsUnits(plan) ? unitRange(plan) : plan.id === 'free' ? 'Shared' : 'Custom'
		},
		{
			label: 'Assets',
			value: sellsUnits(plan)
				? `${assetsPerUnit.toLocaleString('en-US')} per unit`
				: plan.max_assets === null
					? 'Custom'
					: count(plan.max_assets, 'assets')
		},
		{ label: 'Users', value: 'Unlimited' }
	];
}
