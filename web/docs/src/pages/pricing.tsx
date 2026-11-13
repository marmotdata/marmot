import React, { useEffect } from "react";
import Layout from "@theme/Layout";
import { CONSOLE_URL, useCloudSignedIn } from "@site/src/hooks/useCloudSignedIn";
import PlanColumns, { SelfHostedColumns } from "@site/src/components/pricing/PlanColumns";
import PlanCompare from "@site/src/components/pricing/PlanCompare";
import UnitEstimator from "@site/src/components/home/UnitEstimator";
import Faq, { pricingFaq } from "@site/src/components/home/Faq";
import { ENTERPRISE_MAIL } from "@site/src/components/pricing/plans";


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
      description="Marmot is the knowledgehouse for your company. Run it on Marmot Cloud, free to start and priced per compute unit with unlimited users, or on your own infrastructure with an Enterprise licence."
    >
      <div className="mkt-site">
        <section className="mkt-page-head mkt-page-head--field">
          <mm-topography seed="5" density="sparse" fade="start"></mm-topography>
          <div className="mm-container">
            <h1 className="mkt-page-head__title">Pricing</h1>
            <p className="mkt-page-head__lede">
              Start free on Marmot Cloud and pay for the work Marmot does, never for seats. Enterprise runs it on your terms, in our cloud or
              on your own infrastructure.
            </p>
          </div>
        </section>

        <section className="mkt-block mkt-block--tight mkt-plans-sheet" aria-labelledby="cloud">
          <div className="mm-container">
            <div className="mkt-block__head">
              <h2 className="mkt-teams__heading" id="cloud">
                Marmot Cloud
              </h2>
              <p className="mkt-block__lede">
                We run it for you. Start free with no card, then pay per compute unit for the size of your catalog and how busy your agents
                are.
              </p>
            </div>
            <PlanColumns />
          </div>
        </section>

        <section className="mkt-block mkt-block--rule" id="estimate" aria-labelledby="estimate-title">
          <div className="mm-container">
            <div className="mkt-block__head">
              <h2 className="mkt-teams__heading" id="estimate-title">
                Work out your Cloud price
              </h2>
              <p className="mkt-block__lede">
                You need enough units for your catalog or for your agents’ traffic, whichever is bigger. Set both and see the plan that
                fits.
              </p>
            </div>
            <UnitEstimator />
          </div>
        </section>

        <section className="mkt-block mkt-tour-band" aria-labelledby="compare">
          <mm-topography seed="9" density="sparse" fade="end"></mm-topography>
          <div className="mm-container">
            <div className="mkt-block__head">
              <h2 className="mkt-teams__heading" id="compare">
                Compare Cloud plans
              </h2>
              <p className="mkt-block__lede">Every difference between the Marmot Cloud plans, in one place.</p>
            </div>
            <PlanCompare />
          </div>
        </section>

        <section className="mkt-block mkt-plans-sheet" aria-labelledby="self-hosted">
          <div className="mm-container mkt-block--aside">
            <div className="mkt-block__head">
              <h2 className="mkt-teams__heading" id="self-hosted">
                On your own infrastructure
              </h2>
              <p className="mkt-block__lede">
                An Enterprise licence gives you everything in Marmot Cloud on infrastructure you run, with our help to deploy it and support with
                an SLA.
              </p>
              <p className="mkt-deploy__oss">
                Want to try it on your own first? The open source edition is MIT-licensed.{" "}
                <a className="mm-link" href="https://github.com/marmotdata/marmot">
                  View on GitHub
                </a>
              </p>
            </div>
            <SelfHostedColumns only={["licence"]} />
          </div>
        </section>

        <section className="mkt-block mkt-block--rule" aria-labelledby="questions">
          <div className="mm-container mkt-block--aside">
            <div className="mkt-block__head">
              <h2 className="mkt-teams__heading" id="questions">
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
              Get started with Marmot
            </h2>
            <p className="mkt-band__lede">Start free on Marmot Cloud, or talk to us about Enterprise to run it on your terms.</p>
            <div className="mkt-hero__actions">
              <SignUp />
              <a className="mm-btn mm-btn--secondary" href={ENTERPRISE_MAIL}>
                Talk to us
              </a>
            </div>
          </div>
        </section>
      </div>
    </Layout>
  );
}
