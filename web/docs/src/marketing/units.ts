import type { PlanId } from './plans';

// The compute unit model behind pricing and the plan picker. What a unit is
// made of is decided by marmot-instance-operator (internal/controller/units.go)
// and only mirrored here for display; the console's usage page still runs on
// mock-usage.ts.
//
// A unit is capacity, and an account needs enough of it for whichever is
// bigger: its catalog, or the traffic its agents send. People are not a
// dimension: browsing a catalog costs next to nothing, so users are unlimited.

/** USD per unit per month, the same on every plan that sells units. */
export const UNIT_PRICE = 595;
/** Catalog capacity each unit adds: the hard limit. */
export const ASSETS_PER_UNIT = 5000;
/** How far above the sustained rate a short burst may go, as a percentage of it. */
export const BURST_CEILING = 200;

/**
 * What one unit runs on, sustained: dedicated Marmot compute (its database
 * grows alongside), requests the gateway admits, egress and database storage.
 * Compute, memory and requests burst to BURST_CEILING.
 */
export const UNIT_SPEC = {
	vcpu: 1,
	memoryGiB: 2,
	requestsPerSecond: 100,
	egressMbps: 40,
	storageGiB: 25
} as const;

/** "4 vCPU · 8 GiB · 400 req/s": the compute n units run on. */
export function unitSpecLine(units: number): string {
	const n = Math.max(1, Math.round(units));
	return `${n * UNIT_SPEC.vcpu} vCPU · ${n * UNIT_SPEC.memoryGiB} GiB · ${(n * UNIT_SPEC.requestsPerSecond).toLocaleString('en-US')} req/s`;
}
/** The most units the self-serve estimator sizes; past it is Enterprise. */
export const SLIDER_MAX = 20;

/**
 * How many agents one unit's sustained rate carries, by how hard they work.
 * Placeholders: set these from a load test of one unit before publishing.
 */
export const ACTIVITY = [
	{ id: 'light', label: 'Light', hint: 'Occasional questions', agentsPerUnit: 40 },
	{ id: 'steady', label: 'Steady', hint: 'Regular daily use', agentsPerUnit: 20 },
	{ id: 'heavy', label: 'Heavy', hint: 'Running non-stop', agentsPerUnit: 8 }
] as const;
export type ActivityId = (typeof ACTIVITY)[number]['id'];

export type UnitPlanId = Extract<PlanId, 'team' | 'scale'>;
export type EstimatePlanId = UnitPlanId | 'enterprise';

export interface UnitPlan {
	id: UnitPlanId;
	/** Units the plan price pays for; also the fewest it runs on. */
	included: number;
	/** The most units the plan holds. Team stops below Scale's included units:
	 * past that Team would cost what Scale does without Scale's features. */
	max: number | null;
}

export const UNIT_PLANS: Record<UnitPlanId, UnitPlan> = {
	team: { id: 'team', included: 1, max: 3 },
	scale: { id: 'scale', included: 4, max: null }
};

export function isUnitPlan(id: string): id is UnitPlanId {
	return id === 'team' || id === 'scale';
}

/** The plan a unit count lands on: Team until it outgrows it, then Scale. */
export function planForUnits(units: number): UnitPlanId {
	return units > (UNIT_PLANS.team.max ?? Infinity) ? 'scale' : 'team';
}

export function unitsForAssets(assets: number): number {
	return Math.max(1, Math.ceil(assets / ASSETS_PER_UNIT));
}

export function unitsForAgents(agents: number, activity: ActivityId): number {
	const per = ACTIVITY.find((a) => a.id === activity)?.agentsPerUnit ?? 20;
	return Math.max(1, Math.ceil(agents / per));
}

export interface UnitQuote {
	plan: UnitPlan;
	units: number;
	/** The plan price: its included units at the unit price. */
	base: number;
	extra: number;
	extraCost: number;
	total: number;
	assets: number;
}

/** Prices a unit count on the plan it lands on. */
export function quote(units: number): UnitQuote {
	const n = Math.max(1, Math.round(units));
	const plan = UNIT_PLANS[planForUnits(n)];
	const extra = Math.max(0, n - plan.included);
	return {
		plan,
		units: n,
		base: plan.included * UNIT_PRICE,
		extra,
		extraCost: extra * UNIT_PRICE,
		total: Math.max(n, plan.included) * UNIT_PRICE,
		assets: n * ASSETS_PER_UNIT
	};
}

export function formatAssets(n: number): string {
	return n.toLocaleString('en-US');
}

/** 100000 as 100K, 1200000 as 1.2M. */
export function compact(n: number): string {
	return new Intl.NumberFormat('en-US', { notation: 'compact', maximumFractionDigits: 1 }).format(
		n
	);
}

export function unitsLabel(n: number): string {
	return `${n} unit${n === 1 ? '' : 's'}`;
}
