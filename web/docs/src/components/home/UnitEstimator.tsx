import React, { useEffect, useId, useState } from "react";
import Check from "~icons/material-symbols/check";
import { PLANS, formatMoney } from "@site/src/marketing/plans";
import {
  ACTIVITY,
  ASSETS_PER_UNIT,
  SLIDER_MAX,
  UNIT_PLANS,
  UNIT_PRICE,
  formatAssets,
  quote,
  unitSpecLine,
  unitsForAgents,
  unitsForAssets,
  type ActivityId,
} from "@site/src/marketing/units";

const MAX_ASSETS = SLIDER_MAX * ASSETS_PER_UNIT;
const MAX_AGENTS = 200;
const ENTERPRISE_MAIL = "mailto:support@marmotdata.io?subject=Marmot%20Cloud%20Enterprise";

const valueOf = (e: Event) => (e.currentTarget as HTMLInputElement).value;

// How hard each activity level works its agents, drawn as one to three bars.
const level: Record<ActivityId, number> = { light: 1, steady: 2, heavy: 3 };

function Bars({ n }: { n: number }): JSX.Element {
  return (
    <span className="mkt-calc__bars" aria-hidden="true">
      {[1, 2, 3].map((b) => (
        <span key={b} className={b <= n ? "is-on" : undefined} style={{ height: `${b * 5 + 1}px` }} />
      ))}
    </span>
  );
}

/**
 * Sizes a plan from a catalog and its agent traffic, whichever needs more units. The inputs show
 * their values large as they move; the result is an ink panel on a contour field with the price,
 * how it is made up, the compute it buys and the plan's own selling points, then one button.
 */
export default function UnitEstimator(): JSX.Element {
  const [assets, setAssets] = useState(ASSETS_PER_UNIT);
  const [agents, setAgents] = useState(10);
  const [activity, setActivity] = useState<ActivityId>("steady");
  const name = useId();

  // Defining an element needs the DOM; the server-rendered page only carries the tags.
  useEffect(() => {
    void import("@marmotdata/elements/slider.js");
    void import("@marmotdata/elements/topography.js");
  }, []);

  const forCatalog = unitsForAssets(assets);
  const forAgents = unitsForAgents(agents, activity);
  const needed = Math.max(forCatalog, forAgents);
  const enterprise = needed > SLIDER_MAX;
  const q = quote(Math.min(needed, SLIDER_MAX));
  const units = Math.max(q.units, q.plan.included);
  const plan = PLANS[enterprise ? "enterprise" : q.plan.id];
  const sizedBy = forCatalog === forAgents ? "your catalog and agents" : forCatalog > forAgents ? "your catalog" : "your agents";
  const highlights = plan.features.filter((f) => !f.startsWith("Everything in")).slice(0, 3).map((f) => f.replace(/ & /g, " and "));

  return (
    <div className="mkt-calc">
      <div className="mkt-calc__inputs">
        <div className="mkt-calc__field">
          <span className="mkt-calc__label">Catalog size</span>
          <span className="mkt-calc__value">
            {formatAssets(assets)} <small>assets</small>
          </span>
          <mm-slider
            label="Catalog size"
            unit="assets"
            min={ASSETS_PER_UNIT}
            max={MAX_ASSETS}
            step={ASSETS_PER_UNIT}
            value={assets}
            onmm-input={(e: Event) => setAssets(Number(valueOf(e)))}
          ></mm-slider>
        </div>
        <div className="mkt-calc__field">
          <span className="mkt-calc__label">AI agents</span>
          <span className="mkt-calc__value">
            {agents} <small>agents</small>
          </span>
          <mm-slider
            label="AI agents"
            unit="agents"
            min={0}
            max={MAX_AGENTS}
            step={5}
            value={agents}
            onmm-input={(e: Event) => setAgents(Number(valueOf(e)))}
          ></mm-slider>
        </div>
        <fieldset className="mkt-calc__activity">
          <legend className="mkt-calc__label">How busy are your agents?</legend>
          <div className="mkt-calc__choices">
            {ACTIVITY.map((a) => (
              <label key={a.id} className="mkt-calc__choice">
                <input type="radio" name={name} value={a.id} checked={activity === a.id} onChange={() => setActivity(a.id)} />
                <Bars n={level[a.id]} />
                <span>
                  <strong>{a.label}</strong>
                  {a.hint}
                </span>
              </label>
            ))}
          </div>
        </fieldset>
      </div>

      <div className="mkt-calc__result" aria-live="polite">
        <mm-topography seed="5" density="sparse" fade="start"></mm-topography>
        <p className="mkt-calc__plan">{plan.name}</p>
        {enterprise ? (
          <>
            <p className="mkt-calc__price">Talk to us</p>
            <p className="mkt-calc__sum">That is more than the self-serve plans hold, so we will size it with you.</p>
          </>
        ) : (
          <>
            <p className="mkt-calc__price">
              {formatMoney(q.total)}
              <small> a month</small>
            </p>
            <p className="mkt-calc__sum">
              {units} {units === 1 ? "unit" : "units"} × {formatMoney(UNIT_PRICE)}, sized by {sizedBy}
            </p>
            <p className="mkt-calc__spec">{unitSpecLine(units)}, bursting to twice that</p>
            {/* Always there, empty unless the estimate has moved to Scale, so the panel never jumps. */}
            <p className="mkt-calc__spec mkt-calc__spec--note">
              {q.plan.id === "scale" ? `Team holds up to ${UNIT_PLANS.team.max} units, so this is Scale.` : ""}
            </p>
          </>
        )}
        <ul className="mkt-calc__highlights">
          {highlights.map((h) => (
            <li key={h}>
              <Check aria-hidden="true" />
              {h}
            </li>
          ))}
        </ul>
        <a
          className="mm-btn mkt-calc__action"
          href={enterprise ? ENTERPRISE_MAIL : `https://cloud.marmotdata.io/signup?plan=${q.plan.id}&units=${q.units}`}
        >
          {enterprise ? "Talk to us" : `Start on ${plan.name}`}
        </a>
      </div>
    </div>
  );
}
