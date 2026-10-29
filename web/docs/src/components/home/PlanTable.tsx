import React from "react";
import Check from "~icons/material-symbols/check";
import { PLAN_LIST, formatMoney, planLimits, type PlanId } from "@site/src/marketing/plans";
import { ASSETS_PER_UNIT, UNIT_PLANS, UNIT_PRICE, isUnitPlan } from "@site/src/marketing/units";

const SIGNUP = "https://cloud.marmotdata.io/signup";

// Each plan's support line from the catalog, without the feature-list wording.
const support: Record<PlanId, string> = {
  free: "Community",
  team: "Email, 99.5% uptime SLA",
  scale: "Priority and shared Slack, 99.9% uptime SLA",
  enterprise: "Named contact, 99.95% uptime SLA",
};

const action: Record<PlanId, { label: string; href: string }> = {
  free: { label: "Start for free", href: `${SIGNUP}?plan=free` },
  team: { label: "Start on Team", href: `${SIGNUP}?plan=team` },
  scale: { label: "Start on Scale", href: `${SIGNUP}?plan=scale` },
  enterprise: { label: "Talk to us", href: "mailto:support@marmotdata.io" },
};

const plans = PLAN_LIST.map((plan) => {
  const limits = Object.fromEntries(planLimits(plan, ASSETS_PER_UNIT).map((l) => [l.label, l.value]));
  return {
    plan,
    from: isUnitPlan(plan.id),
    price: isUnitPlan(plan.id)
      ? formatMoney(UNIT_PLANS[plan.id].included * UNIT_PRICE)
      : plan.id === "free"
        ? formatMoney(0)
        : "Custom",
    // How long Free waits before pausing is ours to tune, so it is not promised here.
    compute: plan.id === "free" ? "Shared, pauses when idle" : limits["Compute units"],
    limits,
  };
});

type Row = (typeof plans)[number];
const rows: { label: string; value: (p: Row) => string | boolean }[] = [
  { label: "Instances", value: (p) => p.limits["Instances"] },
  { label: "Compute", value: (p) => p.compute },
  { label: "Assets", value: (p) => p.limits["Assets"] },
  { label: "Single sign-on", value: (p) => p.plan.sso },
  { label: "Your own domain", value: (p) => p.plan.custom_domains },
  { label: "Support", value: (p) => support[p.plan.id] },
];

export const Included = () => (
  <span className="mkt-plans__yes">
    <Check aria-hidden="true" />
    Included
  </span>
);
export const NotIncluded = () => <span className="mkt-plans__no">Not included</span>;

export default function PlanTable(): JSX.Element {
  return (
    <>
      {/* It scrolls sideways on narrow screens, so the keyboard must be able to reach it. */}
      <div className="mkt-plans" role="region" aria-label="Plans compared" tabIndex={0}>
        <table>
          <colgroup>
            <col />
            {plans.map((p) => (
              <col key={p.plan.id} />
            ))}
          </colgroup>
          <thead>
            <tr>
              <td />
              {plans.map((p) => (
                <th scope="col" key={p.plan.id}>
                  <span className="mkt-plans__name">{p.plan.name}</span>
                  <span className="mkt-plans__price">
                    {p.from && <small>From </small>}
                    {p.price}
                  </span>
                  <span className="mkt-plans__per">{p.from ? "per month" : ""}</span>
                  <span className="mkt-plans__note">{p.plan.tagline}</span>
                </th>
              ))}
            </tr>
            <tr className="mkt-plans__actions">
              <td />
              {plans.map((p) => (
                <td key={p.plan.id}>
                  <a className={`mm-btn ${p.plan.highlighted ? "mm-btn--primary" : "mm-btn--secondary"}`} href={action[p.plan.id].href}>
                    {action[p.plan.id].label}
                  </a>
                </td>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.label}>
                <th scope="row">{row.label}</th>
                {plans.map((p) => {
                  const v = row.value(p);
                  return <td key={p.plan.id}>{v === true ? <Included /> : v === false ? <NotIncluded /> : v}</td>;
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="mkt-plans__foot">
        Prices are in USD, excluding tax. Units are billed monthly and prorated when you add or remove them.
      </p>
    </>
  );
}
