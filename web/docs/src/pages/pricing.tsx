import React, { useEffect } from "react";
import Layout from "@theme/Layout";
import Link from "@docusaurus/Link";
import { CONSOLE_URL, useCloudSignedIn } from "@site/src/hooks/useCloudSignedIn";
import PlanColumns from "@site/src/components/pricing/PlanColumns";
import PlanCompare from "@site/src/components/pricing/PlanCompare";
import UnitEstimator from "@site/src/components/home/UnitEstimator";
import Faq, { pricingFaq } from "@site/src/components/home/Faq";


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
              Start free with no card. Paid plans are priced per compute unit, so you pay for the size of your catalog and how busy your
              agents are, never for seats.
            </p>
          </div>
        </section>

        <section className="mkt-block mkt-block--tight">
          <div className="mm-container">
            <PlanColumns />
          </div>
        </section>

        <section className="mkt-block mkt-block--rule" id="estimate" aria-labelledby="estimate-title">
          <div className="mm-container">
            <div className="mkt-block__head">
              <h2 className="mkt-block__title" id="estimate-title">
                Work out your price
              </h2>
              <p className="mkt-block__lede">
                You need enough units for your catalog or for your agents’ traffic, whichever is bigger. Set both and see the plan that
                fits.
              </p>
            </div>
            <UnitEstimator />
          </div>
        </section>

        <section className="mkt-block mkt-block--rule" aria-labelledby="compare">
          <div className="mm-container">
            <div className="mkt-block__head">
              <h2 className="mkt-block__title" id="compare">
                Compare plans
              </h2>
              <p className="mkt-block__lede">Every difference between the plans, in one place.</p>
            </div>
            <PlanCompare />
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
          <mm-topography seed="17" fade="start"></mm-topography>
          <div className="mm-container">
            <h2 className="mkt-band__title" id="start">
              Start free, and size it when you need to
            </h2>
            <p className="mkt-band__lede">Start on Marmot Cloud, or run it on your own infrastructure.</p>
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
