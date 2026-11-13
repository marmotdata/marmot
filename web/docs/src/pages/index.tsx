import React, { useEffect } from "react";
import Layout from "@theme/Layout";
import Link from "@docusaurus/Link";
import { CONSOLE_URL, useCloudSignedIn } from "@site/src/hooks/useCloudSignedIn";
import { ENTERPRISE_MAIL } from "@site/src/components/pricing/plans";
import MapHero from "@site/src/components/home/MapHero";
import Faq from "@site/src/components/home/Faq";
import BeforeAfter from "@site/src/components/home/BeforeAfter";
import Teams from "@site/src/components/home/Teams";
import Problem from "@site/src/components/home/Problem";
import Lock from "~icons/material-symbols/lock-outline";
import Badge from "~icons/material-symbols/badge-outline";
import Key from "~icons/material-symbols/key-outline";
import Database from "~icons/material-symbols/database-outline";
import Check from "~icons/material-symbols/check";

// Some of the sources Marmot has a plugin for, named beside the quote.
const sources = [
  { provider: "postgresql", name: "PostgreSQL" },
  { provider: "bigquery", name: "BigQuery" },
  { provider: "kafka", name: "Kafka" },
  { provider: "dbt", name: "dbt" },
  { provider: "airflow", name: "Airflow" },
  { provider: "s3", name: "S3" },
  { provider: "clickhouse", name: "ClickHouse" },
  { provider: "trino", name: "Trino" },
  { provider: "mongodb", name: "MongoDB" },
];

// What a security reviewer asks first, each answered by something Marmot Cloud does today.
const trust = [
  { Icon: Lock, title: "Your own instance", text: "A dedicated database and encryption key for every customer." },
  { Icon: Badge, title: "Your identity provider", text: "Single sign-on for people, a service account for every agent." },
  { Icon: Key, title: "No stored passwords", text: "Credentials stay in your secret store, or aren't needed at all." },
  { Icon: Database, title: "Rows are never stored", text: "Results go straight to whoever asked. Marmot keeps the record." },
];

const deploy = {
  cloud: ["Access control per table, product and term", "Governed queries, with an audit log of every one", "Upgrades, nightly backups and support from the team"],
  enterprise: ["A per-agent audit trail, exported to your SIEM", "Data residency, VPC peering or your own infrastructure", "A named contact, a shared Slack channel and an SLA"],
};

// A pause after the tour: one line and the two ways in, so nobody scrolls the demos with nothing to act on.
function TourAction(): JSX.Element {
  return (
    <aside className="mkt-cta-band" aria-labelledby="tour-action">
      <mm-topography seed="3" density="sparse" fade="start"></mm-topography>
      <div className="mm-container mkt-tour-action">
      <div>
        <h2 className="mkt-tour-action__title" id="tour-action">
          Try it on your own data
        </h2>
        <p className="mkt-tour-action__lede">Launch an instance in about a minute and connect your first source.</p>
      </div>
      <div className="mkt-hero__actions">
        <SignUp />
        <Link className="mm-btn mm-btn--secondary" to="/docs/Deploy">
          Deploy it yourself
        </Link>
      </div>
      </div>
    </aside>
  );
}

function SignUp({ label = "Start for free" }: { label?: string }): JSX.Element {
  const signedIn = useCloudSignedIn();
  return signedIn ? (
    <a className="mm-btn mm-btn--primary" href={CONSOLE_URL}>
      Open console
    </a>
  ) : (
    <a className="mm-btn mm-btn--primary" href="https://cloud.marmotdata.io/signup">
      {label}
    </a>
  );
}

export default function Home(): JSX.Element {
  useEffect(() => {
    void import("@marmotdata/elements/topography.js");
    void import("@marmotdata/icons/define.js");
  }, []);
  return (
    <Layout
      title="The knowledgehouse for your company"
      description="Marmot is the knowledgehouse for your company: it knows what your data means, governs who may use it and runs every query your people and agents make, checked and recorded. Open source, or hosted on Marmot Cloud."
    >
      <div className="mkt-site">
        <section className="mkt-hero mkt-hero--map">
          <div className="mm-container mkt-hero__grid">
            <div>
              <h1 className="mkt-hero__title">Let every person and every agent know your company as well as your best people do.</h1>
              <p className="mkt-hero__lede">
                Marmot is the knowledgehouse for your company. It knows what your data means, governs who may use it and runs every query your
                people and agents make, checked and recorded.
              </p>
              <div className="mkt-hero__actions">
                <SignUp />
                <Link className="mm-btn mm-btn--secondary" to="/docs/Deploy">
                  Deploy it yourself
                </Link>
              </div>
            </div>
            <MapHero />
          </div>
        </section>

        <section className="mkt-proof" aria-label="Who uses Marmot">
          <div className="mm-container mkt-proof__inner">
            <figure className="mkt-quote">
              <blockquote>
                <p>
                  “Marmot brings metadata, ownership and lineage into one searchable catalog, making data products easier to discover and
                  understand.”
                </p>
              </blockquote>
              <figcaption>
                <img src="/img/testimonials/manuel-janez-garcia.jpg" alt="" width={40} height={40} loading="lazy" />
                <span>
                  <strong>Manuel Jáñez García</strong>
                  Senior Specialist of Data Governance Services
                </span>
              </figcaption>
            </figure>
            <div className="mkt-proof__sources">
              <div className="mkt-proof__group">
                <p className="mkt-proof__label">Reads metadata from the tools you already run, with 70+ plugins</p>
                <ul className="mkt-proof__providers">
                  {sources.map((s) => (
                    <li key={s.provider}>
                      <mm-provider-logo provider={s.provider} size={16} decorative></mm-provider-logo>
                      {s.name}
                    </li>
                  ))}
                  <li className="mkt-proof__more">and dozens more</li>
                </ul>
                <a className="mm-link" href="https://plugins.marmotdata.io">
                  Browse plugins
                </a>
              </div>
              <div className="mkt-proof__group">
                <p className="mkt-proof__label">Built in the open by engineers from</p>
                <p className="mkt-proof__value">HashiCorp, Adidas, Just Eat Takeaway.com and Traefik</p>
                <a className="mm-link" href="https://github.com/marmotdata/marmot">
                  More than 600 stars on GitHub
                </a>
              </div>
            </div>
          </div>
        </section>

        <Problem />


        <section className="mkt-block mkt-tour-band" aria-labelledby="teams">
          <mm-topography seed="7" density="sparse" fade="end"></mm-topography>
          <div className="mm-container">
            <div className="mkt-block__head">
              <h2 className="mkt-teams__heading" id="teams">
                How each team works with Marmot
              </h2>
            </div>
            <Teams />
          </div>
        </section>

        <section className="mkt-block" aria-labelledby="change">
          <div className="mm-container">
            <BeforeAfter
              head={
                <div>
                  <h2 className="mkt-teams__heading" id="change">
                    Why a database password isn’t enough
                  </h2>
                  <p className="mkt-chapter__lede">The same finance agent, looking for last week’s revenue in the same six tables.</p>
                </div>
              }
            />
          </div>
        </section>

        <TourAction />

        <section className="mkt-block mkt-block--rule mkt-trust" aria-labelledby="trust">
          <div className="mm-container">
            <div className="mkt-trust__head">
              <h2 className="mkt-trust__title" id="trust">
                Ready for your security review.{" "}
                <span>Your data stays where it lives.</span>
              </h2>
              <Link className="mm-link" to="/docs/Cloud/security">
                How Marmot Cloud keeps your knowledge safe
              </Link>
            </div>
            <ul className="mkt-trust__points">
              {trust.map(({ Icon, title, text }) => (
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

        <section className="mkt-block mkt-block--rule" aria-labelledby="questions">
          <div className="mm-container mkt-block--aside">
            <div className="mkt-block__head">
              <h2 className="mkt-teams__heading" id="questions">
                Questions
              </h2>
              <p className="mkt-block__lede">
                Anything else, email{" "}
                <a className="mm-link" href="mailto:support@marmotdata.io">
                  support@marmotdata.io
                </a>
                .
              </p>
            </div>
            <Faq />
          </div>
        </section>

        <section className="mkt-band mkt-band--deploy" aria-labelledby="deploy">
          <mm-topography seed="11" fade="start"></mm-topography>
          <div className="mm-container">
            <h2 className="mkt-band__title" id="deploy">
              Get started with Marmot
            </h2>
            <p className="mkt-band__lede">Start free on Marmot Cloud, or talk to us about Enterprise to run it on your terms.</p>
            <div className="mkt-deploy">
              <section className="mkt-deploy__option" aria-labelledby="deploy-cloud">
                <h3 id="deploy-cloud">Marmot Cloud</h3>
                <p>We run it for you, with access control and an audit of every query.</p>
                <ul>
                  {deploy.cloud.map((t) => (
                    <li key={t}>
                      <Check aria-hidden="true" />
                      {t}
                    </li>
                  ))}
                </ul>
                <p className="mkt-deploy__price">Free to start with no card, and unlimited users on every plan.</p>
                <div className="mkt-deploy__actions">
                  <SignUp />
                  <Link className="mm-link" to="/pricing">
                    See pricing
                  </Link>
                </div>
              </section>
              <section className="mkt-deploy__option" aria-labelledby="deploy-enterprise">
                <h3 id="deploy-enterprise">Enterprise</h3>
                <p>For large organisations with their own security and compliance needs.</p>
                <ul>
                  {deploy.enterprise.map((t) => (
                    <li key={t}>
                      <Check aria-hidden="true" />
                      {t}
                    </li>
                  ))}
                </ul>
                <div className="mkt-deploy__actions">
                  <a className="mm-btn mm-btn--secondary" href={ENTERPRISE_MAIL}>
                    Talk to us
                  </a>
                  <Link className="mm-link" to="/pricing">
                    Compare plans
                  </Link>
                </div>
              </section>
            </div>
            <p className="mkt-deploy__oss">
              Want to try it on your own first? The open source edition is MIT-licensed.{" "}
              <a className="mm-link" href="https://github.com/marmotdata/marmot">
                View on GitHub
              </a>
            </p>
          </div>
        </section>

      </div>
    </Layout>
  );
}
