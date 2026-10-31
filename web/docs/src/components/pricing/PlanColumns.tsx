import React from "react";
import { PLAN_SUMMARIES } from "./plans";

/**
 * The four plans side by side as open columns divided by hairlines: the name and who it is for,
 * the price and what it buys, the button, then what it adds over the plan before it as quiet
 * lines. The comparison below carries every detail, so this stays short. Only the plan the
 * catalog highlights gets the primary button. Stacks on narrow screens.
 */
export default function PlanColumns(): JSX.Element {
  return (
    <>
      <ul className="mkt-tiers" aria-label="Plans">
        {PLAN_SUMMARIES.map((p) => (
          <li key={p.plan.id} className="mkt-tier">
            <h2 className="mkt-tier__name">{p.plan.name}</h2>
            <p className="mkt-tier__for">{p.plan.tagline}</p>
            <p className="mkt-tier__price">
              {p.from && <span className="mkt-tier__from">From </span>}
              <span className="mkt-tier__amount">{p.price}</span>
              {p.from && <span className="mkt-tier__per"> a month</span>}
            </p>
            <p className="mkt-tier__basis">{p.basis}</p>
            <a className={`mm-btn ${p.plan.highlighted ? "mm-btn--primary" : "mm-btn--secondary"} mkt-tier__action`} href={p.action.href}>
              {p.action.label}
            </a>
            <p className="mkt-tier__lead">{p.builds ? `Everything in ${p.builds}, plus:` : "Includes:"}</p>
            <ul className="mkt-tier__adds">
              {p.adds.map((f) => (
                <li key={f}>{f}</li>
              ))}
            </ul>
          </li>
        ))}
      </ul>
      <p className="mkt-tiers__foot">Prices are in USD, excluding tax. Units are billed monthly and prorated when you add or remove them.</p>
    </>
  );
}
