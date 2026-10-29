import React, { useEffect } from "react";
import Layout from "@theme/Layout";
import Link from "@docusaurus/Link";
import { CONSOLE_URL, useCloudSignedIn } from "@site/src/hooks/useCloudSignedIn";
import PlanTable from "@site/src/components/home/PlanTable";
import UnitEstimator from "@site/src/components/home/UnitEstimator";
import Faq, { pricingFaq } from "@site/src/components/home/Faq";
import { PLANS } from "@site/src/marketing/plans";
import { ASSETS_PER_UNIT, UNIT_SPEC, formatAssets } from "@site/src/marketing/units";
import Inventory from "~icons/material-symbols/inventory-2-outline";
import Memory from "~icons/material-symbols/memory";
import Speed from "~icons/material-symbols/speed";
import Storage from "~icons/material-symbols/storage";
import Bolt from "~icons/material-symbols/bolt-outline";
import Group from "~icons/material-symbols/group-outline";
import Check from "~icons/material-symbols/check";

// What one unit is, from the same constants the console sizes instances with.
const unit = [
  { Icon: Inventory, title: `${formatAssets(ASSETS_PER_UNIT)} assets`, text: "Tables, topics, dashboards and models in the catalog." },
  { Icon: Memory, title: "Dedicated compute", text: `${UNIT_SPEC.vcpu} vCPU and ${UNIT_SPEC.memoryGiB} GiB of memory for your instance.` },
  { Icon: Speed, title: `${UNIT_SPEC.requestsPerSecond} requests a second`, text: "For people, agents and plugins together." },
  { Icon: Storage, title: `${UNIT_SPEC.storageGiB} GiB of storage`, text: "For the catalog's database, with nightly backups." },
  { Icon: Bolt, title: "Room to burst", text: "Compute and requests burst to twice the unit, so a spike lands on headroom." },
  { Icon: Group, title: "Unlimited users", text: "People browsing the catalog are never what you pay for." },
];

// What Enterprise adds, as the plan catalog lists it.
const enterprise = PLANS.enterprise.features.filter((f) => !f.startsWith("Everything in"));

function SignUp(): JSX.Element {
  const signedIn = useCloudSignedIn();
  return signedIn ? (
    <a className="mm-btn mm-btn--primary" href={CONSOLE_URL}>
      Open console
    </a>
  ) : (
    <a className="mm-btn mm-btn--primary" href="https://cloud.marmotdata.io/signup">
      Start for free
    </a>
  );
}

export default function Pricing(): JSX.Element {
  useEffect(() => {
    void import("@marmotdata/elements/topography.js");
  }, []);
  return (
    <Layout
      title="Pricing"
      description="Marmot Cloud is free to start with no card. Paid plans are sized in compute units for your catalog and your agents, with unlimited users on every plan."
    >
      <div className="mkt-site">
        <section className="mkt-page-head">
          <div className="mm-container">
            <h1 className="mkt-page-head__title">Pricing</h1>
            <p className="mkt-page-head__lede">
              Start free with no card. Paid plans are sized for your catalog and your agents, and every plan has unlimited users.
            </p>
          </div>
        </section>

        <section className="mkt-block mkt-block--tight" aria-label="Plans">
          <div className="mm-container">
            <PlanTable />
          </div>
        </section>

        <section className="mkt-block mkt-block--rule" aria-labelledby="unit">
          <div className="mm-container">
            <div className="mkt-block__head">
              <h2 className="mkt-block__title" id="unit">
                What a compute unit is
              </h2>
              <p className="mkt-block__lede">
                Paid plans are sold in units. You need enough of them for your catalog or for your agent traffic, whichever is bigger.
              </p>
            </div>
            <ul className="mkt-points">
              {unit.map(({ Icon, title, text }) => (
                <li key={title}>
                  <h3>
                    <Icon aria-hidden="true" />
                    {title}
                  </h3>
                  <p>{text}</p>
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section className="mkt-block mkt-block--rule" id="estimate" aria-labelledby="estimate-title">
          <div className="mm-container">
            <div className="mkt-block__head">
              <h2 className="mkt-block__title" id="estimate-title">
                Estimate your units
              </h2>
              <p className="mkt-block__lede">Move the sliders to the size of your catalog and how busy your agents are.</p>
            </div>
            <UnitEstimator />
          </div>
        </section>

        <section className="mkt-block mkt-block--rule" aria-labelledby="enterprise">
          <div className="mm-container mkt-block--aside">
            <div className="mkt-block__head">
              <h2 className="mkt-block__title" id="enterprise">
                Enterprise
              </h2>
              <p className="mkt-block__lede">For several instances, custom terms and a direct line to the team.</p>
              <p className="mkt-block__lede">
                <a className="mm-btn mm-btn--secondary" href="mailto:support@marmotdata.io?subject=Marmot%20Cloud%20Enterprise">
                  Talk to us
                </a>
              </p>
            </div>
            <ul className="mkt-checklist">
              <li>
                <Check aria-hidden="true" />
                Everything in Scale
              </li>
              {enterprise.map((f) => (
                <li key={f}>
                  <Check aria-hidden="true" />
                  {f}
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section className="mkt-block mkt-block--rule" aria-labelledby="questions">
          <div className="mm-container mkt-block--aside">
            <div className="mkt-block__head">
              <h2 className="mkt-block__title" id="questions">
                Questions about pricing
              </h2>
              <p className="mkt-block__lede">
                Anything else, email{" "}
                <a className="mm-link" href="mailto:support@marmotdata.io">
                  support@marmotdata.io
                </a>
                .
              </p>
            </div>
            <Faq items={pricingFaq} />
          </div>
        </section>

        <section className="mkt-band mkt-band--closing" aria-labelledby="start">
          <mm-topography seed="17" fade="edges"></mm-topography>
          <div className="mm-container">
            <h2 className="mkt-band__title" id="start">
              Start free, and size it when you need to.
            </h2>
            <p className="mkt-band__lede">No card for the free plan, or run Marmot on your own infrastructure.</p>
            <div className="mkt-hero__actions">
              <SignUp />
              <Link className="mm-btn mm-btn--secondary" to="/docs/Deploy">
                Deploy it yourself
              </Link>
            </div>
          </div>
        </section>
      </div>
    </Layout>
  );
}
