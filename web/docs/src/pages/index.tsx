import React, { useEffect, useState } from "react";
import Layout from "@theme/Layout";
import Link from "@docusaurus/Link";
import { CONSOLE_URL, useCloudSignedIn } from "@site/src/hooks/useCloudSignedIn";
import MapHero from "@site/src/components/home/MapHero";
import ScreenDemo from "@site/src/components/home/ScreenDemo";
import Beats from "@site/src/components/home/Beats";
import type { Player } from "@site/src/marketing/demo";
import Faq from "@site/src/components/home/Faq";
import BeforeAfter from "@site/src/components/home/BeforeAfter";
import { findDemo, queryDemo, traceDemo, understandDemo, type DemoConfig } from "@site/src/marketing/demos";
import Search from "~icons/material-symbols/search";
import FilterList from "~icons/material-symbols/filter-list";
import Extension from "~icons/material-symbols/extension-outline";
import Group from "~icons/material-symbols/group-outline";
import MenuBook from "~icons/material-symbols/menu-book-outline";
import Verified from "~icons/material-symbols/verified-outline";
import AccountTree from "~icons/material-symbols/account-tree-outline";
import Sync from "~icons/material-symbols/sync";
import Dashboard from "~icons/material-symbols/dashboard-outline";
import Terminal from "~icons/material-symbols/terminal";
import Block from "~icons/material-symbols/block";
import History from "~icons/material-symbols/history";
import Check from "~icons/material-symbols/check";

type IconType = React.ComponentType<{ "aria-hidden"?: string }>;
type ChapterData = {
  id: string;
  title: string;
  lede: string;
  points: { Icon: IconType; text: string }[];
  /** Where to read more: the docs for what the chapter shows. */
  more: { to: string; label: string };
  demo: DemoConfig<{ focus?: string; beat?: number }>;
};
// Each chapter's demo drives a different screen; the list only has to hand them on.
const any = <S extends { focus?: string; beat?: number }>(d: DemoConfig<S>) => d as unknown as DemoConfig<{ focus?: string; beat?: number }>;

// Each chapter opens with the problem it solves, then shows the product solving it. They run in
// the order an agent's question does: find the table, learn what it means, query it, and then
// what a change to it would break.
const chapters: ChapterData[] = [
  {
    id: "find",
    title: "Find the right table without asking in Slack",
    lede: "Search every connected source in plain words, from the browser, the CLI or an agent over MCP.",
    points: [
      { Icon: Search, text: "Tables, topics, dashboards and models in one list" },
      { Icon: FilterList, text: "Filter by type, owner, tag or any metadata" },
      { Icon: Extension, text: "Kept current by 70+ plugins" },
    ],
    more: { to: "/docs/Plugins", label: "See the plugins" },
    demo: any(findDemo),
  },
  {
    id: "understand",
    title: "Know what a column means before anyone trusts it",
    lede: "Owners, descriptions and glossary terms sit with every asset, so humans and agents answer from the same facts.",
    points: [
      { Icon: Group, text: "Owners and teams on every asset" },
      { Icon: MenuBook, text: "Glossary terms that explain each field" },
      { Icon: Verified, text: "Certified fields marked as the ones to use" },
    ],
    more: { to: "/docs/glossary", label: "How the glossary works" },
    demo: any(understandDemo),
  },
  {
    id: "query",
    title: "Let humans and agents query your data without sharing passwords",
    lede: "Humans and agents run queries through Marmot under the same rules. Each query is checked against the caller’s permissions before it reaches the database, and logged with who ran it and whether it was allowed. Access rules per table, data product and glossary term come with Marmot Cloud.",
    points: [
      { Icon: Terminal, text: "Read-only queries against the sources you connect" },
      { Icon: Block, text: "Refused before it reaches the database" },
      { Icon: History, text: "Who ran what, when and whether it was allowed" },
    ],
    more: { to: "/docs/Cloud/access-control", label: "How access control works" },
    demo: any(queryDemo),
  },
  {
    id: "trace",
    title: "See what breaks before you change it",
    lede: "Lineage follows data from source tables through every model to the dashboards that use it.",
    points: [
      { Icon: AccountTree, text: "Upstream and downstream, across tools" },
      { Icon: Sync, text: "From plugins and OpenLineage runs" },
      { Icon: Dashboard, text: "The dashboards a change would break" },
    ],
    more: { to: "/docs/open-lineage", label: "How lineage is collected" },
    demo: any(traceDemo),
  },
];

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

const deploy = {
  oss: ["The catalog, lineage, glossary and MCP server", "One binary, or on Kubernetes with the Helm chart", "Help from the community on Discord and GitHub"],
  cloud: ["Access control per table, product and term", "Governed queries, with an audit log of every one", "Upgrades, nightly backups and support from the team"],
};

// One chapter of the tour: the problem, then its points as captions that follow the demo beside it.
function Chapter({ c, flip = false }: { c: ChapterData; flip?: boolean }): JSX.Element {
  const [demo, setDemo] = useState<{ player: Player; bounds: number[] }>();
  return (
    <section className={`mkt-chapter mkt-chapter--split${flip ? " mkt-chapter--flip" : ""}`} id={c.id} aria-labelledby={`${c.id}-title`}>
      <div className="mkt-chapter__text">
        <div>
          <h2 className="mkt-chapter__title" id={`${c.id}-title`}>
            {c.title}
          </h2>
          <p className="mkt-chapter__lede">{c.lede}</p>
        </div>
        <Beats points={c.points} player={demo?.player} bounds={demo?.bounds} />
        <Link className="mm-link mkt-chapter__more" to={c.more.to}>
          {c.more.label}
        </Link>
      </div>
      <ScreenDemo {...c.demo} onPlayer={(player, bounds) => setDemo({ player, bounds })} />
    </section>
  );
}

// A pause in the tour: one line and the two ways in, so nobody scrolls three demos with nothing to act on.
function TourAction(): JSX.Element {
  return (
    <aside className="mkt-tour-action" aria-labelledby="tour-action">
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
      title="Data catalog, query gateway and MCP server for humans and agents"
      description="Marmot is a data catalog, query gateway and MCP server in one. Your team and your agents look up what a table means and who owns it, then query it through Marmot, which checks each query against your access rules and keeps a record of every query. Open source, or hosted on Marmot Cloud."
    >
      <div className="mkt-site">
        <section className="mkt-hero mkt-hero--map">
          <div className="mm-container mkt-hero__grid">
            <div>
              <h1 className="mkt-hero__title">Give humans and agents safe access to the right data.</h1>
              <p className="mkt-hero__lede">
                Marmot is a data catalog, query gateway and MCP server in one. Your team and your agents read the same definitions and owners, and
                every query they run is checked and recorded.
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

        <section className="mkt-block mkt-block--rule" aria-labelledby="change">
          <div className="mm-container">
            <div className="mkt-block__head">
              <h2 className="mkt-block__title" id="change">
                Why a database password isn’t enough
              </h2>
              <p className="mkt-block__lede">Both maps show a finance agent looking for last week’s revenue in the same six tables.</p>
            </div>
            <BeforeAfter />
            <div className="mkt-hero__actions mkt-block__actions">
              <SignUp />
              <Link className="mm-btn mm-btn--secondary" to="/docs/Deploy">
                Deploy it yourself
              </Link>
            </div>
          </div>
        </section>

        <div className="mm-container mkt-tour">
          {chapters.map((c, i) => (
            <React.Fragment key={c.id}>
              <Chapter c={c} flip={i % 2 === 1} />
              {c.id === "query" && <TourAction />}
            </React.Fragment>
          ))}
        </div>

        <section className="mkt-block mkt-block--rule" aria-labelledby="questions">
          <div className="mm-container mkt-block--aside">
            <div className="mkt-block__head">
              <h2 className="mkt-block__title" id="questions">
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
              Give your team and your agents one safe way in to your data
            </h2>
            <p className="mkt-band__lede">Run it yourself or let us run it. It’s the same catalog and the same MCP server either way, so you can move between them.</p>
            <div className="mkt-deploy">
              <section className="mkt-deploy__option" aria-labelledby="deploy-cloud">
                <h3 id="deploy-cloud">Marmot Cloud</h3>
                <p>We run it for you and add access control on top.</p>
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
              <section className="mkt-deploy__option" aria-labelledby="deploy-oss">
                <h3 id="deploy-oss">Open source</h3>
                <p>MIT-licensed and free, on your own infrastructure.</p>
                <ul>
                  {deploy.oss.map((t) => (
                    <li key={t}>
                      <Check aria-hidden="true" />
                      {t}
                    </li>
                  ))}
                </ul>
                <div className="mkt-deploy__actions">
                  <Link className="mm-btn mm-btn--secondary" to="/docs/Deploy">
                    Deploy it yourself
                  </Link>
                  <a className="mm-link" href="https://github.com/marmotdata/marmot">
                    View on GitHub
                  </a>
                </div>
              </section>
            </div>
          </div>
        </section>

      </div>
    </Layout>
  );
}
