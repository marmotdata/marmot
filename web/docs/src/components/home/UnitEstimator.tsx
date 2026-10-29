import React, { useEffect, useState } from "react";
import { PLANS, formatMoney } from "@site/src/marketing/plans";
import {
  ACTIVITY,
  ASSETS_PER_UNIT,
  SLIDER_MAX,
  formatAssets,
  quote,
  unitsForAgents,
  unitsForAssets,
  type ActivityId,
} from "@site/src/marketing/units";

const MAX_ASSETS = SLIDER_MAX * ASSETS_PER_UNIT;
const MAX_AGENTS = 200;

const valueOf = (e: Event) => (e.currentTarget as HTMLInputElement).value;

/** Sizes a plan from a catalog and its agent traffic, whichever needs more units. */
export default function UnitEstimator(): JSX.Element {
  const [assets, setAssets] = useState(ASSETS_PER_UNIT);
  const [agents, setAgents] = useState(10);
  const [activity, setActivity] = useState<ActivityId>("steady");

  // Defining an element needs the DOM; the server-rendered page only carries the tags.
  useEffect(() => {
    void import("@marmotdata/elements/slider.js");
    void import("@marmotdata/elements/radio-group.js");
    void import("@marmotdata/elements/kpi-tile.js");
    void import("@marmotdata/elements/description-list.js");
  }, []);

  const forCatalog = unitsForAssets(assets);
  const forAgents = unitsForAgents(agents, activity);
  const needed = Math.max(forCatalog, forAgents);
  const enterprise = needed > SLIDER_MAX;
  const q = quote(Math.min(needed, SLIDER_MAX));
  const units = Math.max(q.units, q.plan.included);
  const sizedBy = forCatalog === forAgents ? "Catalog and agents" : forCatalog > forAgents ? "Catalog" : "Agents";

  return (
    <div className="mm-panel mkt-estimator">
      <div className="mkt-estimator__inputs">
        <mm-slider
          label="Catalog size"
          unit="assets"
          min={ASSETS_PER_UNIT}
          max={MAX_ASSETS}
          step={ASSETS_PER_UNIT}
          value={assets}
          onmm-input={(e: Event) => setAssets(Number(valueOf(e)))}
        ></mm-slider>
        <mm-slider
          label="AI agents"
          unit="agents"
          min={0}
          max={MAX_AGENTS}
          step={5}
          value={agents}
          onmm-input={(e: Event) => setAgents(Number(valueOf(e)))}
        ></mm-slider>
        <mm-radio-group
          label="How busy are your agents?"
          orientation="horizontal"
          value={activity}
          onchange={(e: Event) => setActivity(valueOf(e) as ActivityId)}
        >
          {ACTIVITY.map((a) => (
            <mm-radio key={a.id} value={a.id} description={a.hint}>
              {a.label}
            </mm-radio>
          ))}
        </mm-radio-group>
        <p className="mkt-estimator__limit">
          Larger than {formatAssets(MAX_ASSETS)} assets or {MAX_AGENTS} agents? Enterprise covers it.
        </p>
      </div>
      <div className="mkt-estimator__result" aria-live="polite">
        {enterprise ? (
          <>
            <mm-kpi-tile label="Estimated cost" value="Custom" context="Enterprise pricing"></mm-kpi-tile>
            <p className="mkt-estimator__note">
              This is more than the self-serve plans hold. Email{" "}
              <a className="mm-link" href="mailto:support@marmotdata.io?subject=Marmot%20Cloud%20Enterprise">
                support@marmotdata.io
              </a>{" "}
              and we will size it with you.
            </p>
          </>
        ) : (
          <>
            <mm-kpi-tile label="Estimated cost" value={formatMoney(q.total)} context="Per month, excluding tax"></mm-kpi-tile>
            <mm-description-list>
              <mm-description term="Plan">{PLANS[q.plan.id].name}</mm-description>
              <mm-description term="Compute units">{units}</mm-description>
              <mm-description term="Sized by">{sizedBy}</mm-description>
            </mm-description-list>
            <div>
              <a className="mm-btn mm-btn--secondary" href={`https://cloud.marmotdata.io/signup?plan=${q.plan.id}&units=${q.units}`}>
                Start with this estimate
              </a>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
