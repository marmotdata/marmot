// What the pricing page says about each plan, built from the plan catalog (marketing/plans.ts) and
// the unit model (marketing/units.ts) so the page never drifts from what Marmot Cloud sells.
import { PLAN_LIST, formatMoney, type Plan, type PlanId } from "@site/src/marketing/plans";
import { ASSETS_PER_UNIT, UNIT_PLANS, UNIT_PRICE, isUnitPlan } from "@site/src/marketing/units";

const SIGNUP = "https://cloud.marmotdata.io/signup";
export const ENTERPRISE_MAIL = "mailto:support@marmotdata.io?subject=Marmot%20Cloud%20Enterprise";

export interface PlanSummary {
  plan: Plan;
  /** "$595", "$0", "Custom". */
  price: string;
  /** Whether the price is where the plan starts, so it reads "From $595". */
  from: boolean;
  /** One line under the price saying what it buys. */
  basis: string;
  action: { label: string; href: string };
  /** The plan the "Everything in …" line points back to, if any. */
  builds: string | null;
  /** What the plan adds over the one before it. */
  adds: string[];
}

const basis: Record<PlanId, string> = {
  free: "500 assets on shared compute",
  team: `${formatMoney(UNIT_PRICE)} per unit, 1 to 3 units`,
  scale: `${formatMoney(UNIT_PRICE)} per unit, 4 or more`,
  enterprise: "Several instances and custom terms",
};

const action: Record<PlanId, { label: string; href: string }> = {
  free: { label: "Start for free", href: `${SIGNUP}?plan=free` },
  team: { label: "Start on Team", href: `${SIGNUP}?plan=team` },
  scale: { label: "Start on Scale", href: `${SIGNUP}?plan=scale` },
  enterprise: { label: "Talk to us", href: ENTERPRISE_MAIL },
};

// The catalog's feature lines say "Everything in Team" first; that becomes the lead-in line.
const builds = (plan: Plan) => plan.features.find((f) => f.startsWith("Everything in"))?.replace("Everything in ", "") ?? null;

// "SSO (OIDC) with role mapping & JIT provisioning" reads as "and" in our copy.
const plain = (f: string) => f.replace(/ & /g, " and ");

export const PLAN_SUMMARIES: PlanSummary[] = PLAN_LIST.map((plan) => ({
  plan,
  price: isUnitPlan(plan.id) ? formatMoney(UNIT_PLANS[plan.id].included * UNIT_PRICE) : plan.id === "free" ? formatMoney(0) : "Custom",
  from: isUnitPlan(plan.id),
  basis: basis[plan.id],
  action: action[plan.id],
  builds: builds(plan),
  adds: plan.features.filter((f) => !f.startsWith("Everything in")).map(plain),
}));

/** A cell in the comparison: a value, or whether the plan includes it. */
export type Cell = string | boolean;

export interface CompareGroup {
  title: string;
  rows: { label: string; cells: Record<PlanId, Cell> }[];
}

const has = (ids: PlanId[]): Record<PlanId, Cell> => ({
  free: ids.includes("free"),
  team: ids.includes("team"),
  scale: ids.includes("scale"),
  enterprise: ids.includes("enterprise"),
});
const all = has(["free", "team", "scale", "enterprise"]);
const by = (pick: (p: Plan) => boolean) => has(PLAN_LIST.filter(pick).map((p) => p.id));

// Every difference spelled out, grouped the way a buyer checks them.
export const COMPARE: CompareGroup[] = [
  {
    title: "Capacity",
    rows: [
      { label: "Instances", cells: { free: "1", team: "1", scale: "1", enterprise: "3" } },
      { label: "Compute", cells: { free: "Shared, pauses when idle", team: "1 to 3 units", scale: "4 or more units", enterprise: "Custom" } },
      {
        label: "Assets",
        cells: {
          free: "500",
          team: `${ASSETS_PER_UNIT.toLocaleString("en-US")} per unit`,
          scale: `${ASSETS_PER_UNIT.toLocaleString("en-US")} per unit`,
          enterprise: "Custom",
        },
      },
      { label: "Users", cells: { free: "Unlimited", team: "Unlimited", scale: "Unlimited", enterprise: "Unlimited" } },
    ],
  },
  {
    title: "Features",
    rows: [
      { label: "All plugins", cells: all },
      { label: "Federated secret stores", cells: all },
      { label: "MCP endpoint for agents", cells: all },
      { label: "Private plugin registry", cells: has(["team", "scale", "enterprise"]) },
      { label: "Single sign-on with role mapping", cells: by((p) => p.sso) },
      { label: "Your own domain", cells: by((p) => p.custom_domains) },
      { label: "Per-agent audit trail and SIEM export", cells: has(["enterprise"]) },
      { label: "VPC peering and data residency", cells: has(["enterprise"]) },
      { label: "Custom backup retention", cells: has(["enterprise"]) },
      { label: "Plugin development on request", cells: has(["enterprise"]) },
    ],
  },
  {
    title: "Support",
    rows: [
      { label: "Support", cells: { free: "Community", team: "Email", scale: "Priority and shared Slack", enterprise: "Named contact" } },
      { label: "Uptime SLA", cells: { free: false, team: "99.5%", scale: "99.9%", enterprise: "99.95%" } },
    ],
  },
];
